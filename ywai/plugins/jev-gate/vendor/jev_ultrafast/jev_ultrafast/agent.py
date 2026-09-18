"""The complete agent loop. Typed choices, observable state, bounded execution."""

import base64
import time
from pathlib import Path

from .browser import Browser, StalePage
from .model import action_space, choose
from .questions import MAX_STEPS


class Agent:
    def __init__(self, url, goals, *, record_dir=None, screenshots=False, values=None):
        task = goals.strip() if isinstance(goals, str) else "\n".join(goals).strip()
        if not task:
            raise ValueError("Supply a task")
        # ywai patch A/B: caller-supplied field values never reach a model; checked before any network call.
        self.values = {str(k).strip().lower(): str(v) for k, v in (values or {}).items() if v}
        refuse_leak(task, self.values, "goal")
        plan = [task]
        self.browser = Browser(url)
        self.record_dir = Path(record_dir) if record_dir else None
        self.screenshots = screenshots or bool(record_dir)
        try:
            page = self.browser.observe(screenshot=self.screenshots)
        except Exception:
            self.browser.close()
            raise
        self.state = dict(
            browser=self.browser,
            goal="\n".join(plan),
            page=page,
            decision=None,
            history=[],
            status="ready",
            plan=plan,
            plan_index=0,
            decisions=[],
            elapsed_ms=0,
            started_at=None,
            record=bool(self.record_dir),
        )
        if self.record_dir:
            self.record_dir.mkdir(parents=True, exist_ok=True)
            (self.record_dir / "000000.jpg").write_bytes(base64.b64decode(page["screenshot"]))

    def snapshot(self):
        return {
            **{k: v for k, v in self.state.items() if k != "browser"},
            "elements": action_space(self.state["page"]["actions"])[0],
        }

    def command(self, name, body=None):
        body = body or {}
        state = self.state
        if name == "tick":
            try:
                self.command("predict", {})
                return self.command("act", {"fingerprint": state["page"]["fingerprint"]})
            except StalePage:
                state["decision"] = None
                state["status"] = "ready"
                state["page"] = state["browser"].observe(screenshot=self.screenshots)
                state["elapsed_ms"] = round((time.perf_counter() - state["started_at"]) * 1000)
                return self.snapshot()
        elif name == "predict":
            if not state["browser"]:
                raise ValueError("Start a demo first")
            if state["started_at"] is None:
                state["started_at"] = time.perf_counter()
            if not state["browser"].fresh(state["page"]):
                state["page"] = state["browser"].observe(screenshot=self.screenshots)
            state["decision"] = None
            if state["status"] in {"done", "blocked"}:
                raise ValueError("This run has stopped. Start a fresh demo.")
            if len(state["decisions"]) >= MAX_STEPS * 2:
                raise ValueError("Reached the demo's model-call budget")
            state["decision"] = choose(state["page"], state["goal"], state["history"])
            state["decisions"].append(
                {
                    **state["decision"],
                    "fingerprint": state["page"]["fingerprint"],
                    "elapsed_ms": round((time.perf_counter() - state["started_at"]) * 1000),
                }
            )
            state["status"] = "predicted"
        elif name == "act":
            decision, page = state["decision"], state["page"]
            if not decision or body.get("fingerprint") != page["fingerprint"]:
                raise ValueError("Observe and choose before acting")
            # Consume once, before any mutation or model call. A retry cannot double-click.
            state["decision"] = None
            selected = decision["choice"]
            if selected in {"DONE", "BLOCKED"}:
                if not state["browser"].fresh(page):
                    state["status"] = "ready"
                    raise StalePage("Page changed since the decision. Choose again.")
                state["status"] = "done" if selected == "DONE" else "blocked"
                if selected == "BLOCKED":
                    # ywai patch: keep why, as the runner-up choices; TypeSafe returns no rationale.
                    labels = {a["id"]: a["label"] for a in page["actions"]}
                    ranked = decision.get("operation_probabilities") or decision["probabilities"]
                    probs = sorted(ranked.items(), key=lambda kv: -kv[1])
                    others = ", ".join(f"{labels.get(k, k)} {v:.0%}" for k, v in probs if k != "BLOCKED")[:300]
                    p = decision["probabilities"]["BLOCKED"]
                    state["blocked_reason"] = f"Jev chose BLOCKED ({p:.0%}); next: {others}"
                state["plan_index"] = int(selected == "DONE")
                state["elapsed_ms"] = round((time.perf_counter() - state["started_at"]) * 1000)
                return self.snapshot()
            action = next(a for a in page["actions"] if a["id"] == selected)
            if len(state["history"]) >= MAX_STEPS:
                state["status"] = "blocked"
                raise ValueError(f"Stopped at the {MAX_STEPS}-action demo budget")
            text = None
            if action["kind"] == "fill":
                if not state["browser"].fresh(page):
                    raise StalePage("Page changed before typing. Choose again.")
                text = supplied_value(action, getattr(self, "values", {}))
                if text is None:
                    # ywai patch A: the caller writes every typed string; no text LLM.
                    state["status"] = "blocked"
                    usable = ", ".join(repr(k) for k in field_keys(action))
                    state["blocked_reason"] = f"missing value: {action['label']!r} (use any key: {usable})"
                    return self.snapshot()
            # Browser.act checks freshness immediately before input, including after text generation.
            state["browser"].act(action, page, text=text)
            state["elapsed_ms"] = round((time.perf_counter() - state["started_at"]) * 1000)
            # Record execution before observing. A stale post-action observation must not erase the action.
            state["history"].append(
                {
                    "step": len(state["history"]) + 1,
                    "action": action["label"],
                    "kind": action["kind"],
                    "choice": selected,
                    "probability": decision["probabilities"][selected],
                    "confidence": decision["confidence"],
                    "latency_ms": decision["latency_ms"],
                    "text": None if text is None else "***",
                    "operation": decision["operation"],
                    "target": decision["target"],
                    "page_changed": None,
                    "url": page["url"],
                    "usage": decision["usage"],
                    "executed_ms": round((time.perf_counter() - state["started_at"]) * 1000),
                    "elapsed_ms": state["elapsed_ms"],
                }
            )
            state["page"] = state["browser"].observe(screenshot=self.screenshots)
            state["elapsed_ms"] = round((time.perf_counter() - state["started_at"]) * 1000)
            state["history"][-1].update(
                page_changed=state["page"]["fingerprint"] != page["fingerprint"],
                url=state["page"]["url"],
                elapsed_ms=state["elapsed_ms"],
            )
            if state["record"]:
                (self.record_dir / f"{state['elapsed_ms']:06d}.jpg").write_bytes(
                    base64.b64decode(state["page"]["screenshot"])
                )
            repeated = state["history"][-3:]
            stuck = len(repeated) == 3 and all(h["page_changed"] is False and h["kind"] != "wait" for h in repeated)
            if stuck:  # ywai patch: say why the loop stopped
                last = repeated[-1]["action"]
                state["blocked_reason"] = f"3 actions in a row did not change the page (last: {last!r})"
            state["status"] = "blocked" if stuck else "ready"
        else:
            raise ValueError("Unknown command")
        return self.snapshot()

    def run(self):
        while self.state["status"] not in {"done", "blocked"}:
            yield self.command("tick")

    def close(self):
        self.browser.close()

    def __enter__(self):
        return self

    def __exit__(self, *_args):
        self.close()


def supplied_value(action, values):
    """ywai patch A: match a fill target by label, name, placeholder, <label> text, or id; case-insensitive."""
    for key in field_keys(action):
        if key.strip().lower() in values:
            return values[key.strip().lower()]
    return None


def field_keys(action):
    """Every name a caller can use as a `values` key for this field, first seen first."""
    keys = (action.get("label"), action.get("name"), action.get("placeholder"),
            *action.get("labels", []), action.get("elid"))
    return list(dict.fromkeys(k for k in keys if k))


def refuse_leak(text, values, where):
    """ywai patch B: a supplied value must never be sent to TypeSafe."""
    if any(v in text for v in values.values()):
        raise ValueError(f"A supplied value appears in the {where}; refusing before any model call.")

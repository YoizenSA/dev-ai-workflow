"""ywai entry: one JSON request on stdin, JSON lines on stdout (states, then one result).

Request: {"url": str, "goal": str, "values": {label_or_name: text}, "profile_dir": str?}
"""

import json
import os
import shutil
import subprocess
import sys
import tempfile
import time
from pathlib import Path

CHROMES = ("google-chrome-stable", "google-chrome", "chromium-browser", "chromium")


def launch_chrome(profile):
    """A Chrome of our own on its own profile, so the run never touches the user's browser."""
    binary = os.environ.get("BH_CHROME_PATH") or os.environ.get("CHROME_PATH")
    binary = binary or next(filter(None, map(shutil.which, CHROMES)), None)
    if not binary:
        raise RuntimeError("No Chrome/Chromium found; set BH_CHROME_PATH.")
    port_file = Path(profile) / "DevToolsActivePort"
    port_file.unlink(missing_ok=True)
    args = [binary, f"--user-data-dir={profile}", "--remote-debugging-port=0", "--no-first-run",
            "--no-default-browser-check", "about:blank"]
    if os.environ.get("JEV_HEADED") != "1":
        args.insert(1, "--headless=new")
    proc = subprocess.Popen(args, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    deadline = time.monotonic() + 20
    while time.monotonic() < deadline:
        lines = port_file.read_text().split() if port_file.exists() else []
        if lines:
            return proc, f"http://127.0.0.1:{lines[0]}"
        if proc.poll() is not None:
            raise RuntimeError("Chrome exited before DevTools was ready.")
        time.sleep(0.1)
    proc.kill()
    raise RuntimeError("Chrome DevTools did not come up within 20s.")


def final_page(page):
    """ywai patch: the last observed page as an aria-like snapshot, for jev_check_page (the Then)."""
    lines = [line for line in (page.get("text") or "").splitlines() if line.strip()][:200]
    seen = set()
    for a in page.get("actions", []):
        if a.get("kind") not in {"click", "fill", "select"} or not a.get("role"):
            continue
        if a["kind"] == "select":  # one line per dropdown, not one per option
            if a.get("node") in seen:
                continue
            seen.add(a.get("node"))
            a = {**a, "label": a.get("label", "").split(" → ")[0]}
        value = a.get("current_value", a.get("value")) if a.get("kind") != "click" else None
        state = {"true": " [checked]", "false": " [unchecked]"}.get(a.get("checked"), "")
        lines.append(f'- {a["role"]} "{a.get("label", "")}"{state}' + (f": {value}" if value else ""))
    return {"url": page.get("url"), "title": page.get("title"), "snapshot": "\n".join(lines)}


def main():
    request = json.loads(sys.stdin.read())
    values = {str(k): str(v) for k, v in (request.get("values") or {}).items() if v}
    secrets = [s for v in values.values() for s in (v, json.dumps(v)[1:-1])]

    def emit(obj):
        line = json.dumps(obj)
        for s in secrets:
            line = line.replace(s, "***")
        print(line, flush=True)

    started = time.perf_counter()
    result = {"type": "result", "status": "error", "history": [], "final_url": request.get("url")}
    owned = not request.get("profile_dir")
    profile = request.get("profile_dir") or tempfile.mkdtemp(prefix="jev-profile-")
    proc = agent = None
    name = f"jev{os.getpid()}"
    try:
        os.environ["BU_NAME"] = name  # browser_harness reads it at import time
        from .agent import Agent, refuse_leak

        refuse_leak(request["goal"], values, "goal")  # patch B, before Chrome or any network call
        proc, cdp_url = launch_chrome(profile)
        os.environ["BU_CDP_URL"] = cdp_url
        agent = Agent(request["url"], request["goal"], values=values)
        for snap in agent.run():
            last = snap["history"][-1] if snap["history"] else {}
            emit({"type": "state", "status": snap["status"], "step": len(snap["history"]),
                  "action": last.get("action"), "kind": last.get("kind"), "url": snap["page"]["url"]})
        result["status"] = agent.state["status"]
        if agent.state.get("blocked_reason"):
            result["reason"] = agent.state["blocked_reason"]
    except Exception as err:  # noqa: BLE001 - every failure must reach the caller as a result line
        result["error"] = str(err)
        if agent and agent.state["status"] == "blocked":
            result["status"] = "blocked"
    finally:
        if agent:
            s = agent.state
            result["history"] = [{k: h.get(k) for k in ("step", "action", "kind", "text", "url", "page_changed")}
                                 for h in s["history"]]
            result["final_url"] = s["page"]["url"]
            try:
                # The Then is scored on a settled page: DONE often follows a submit by ~50 ms.
                time.sleep(2)
                s["page"] = s["browser"].observe(screenshot=False)
                result["final_url"] = s["page"]["url"]
            except Exception:  # noqa: BLE001 - keep the last observation
                pass
            result["final_page"] = final_page(s["page"])
            try:
                agent.close()
            except Exception:  # noqa: BLE001
                pass
        if proc:
            try:
                from browser_harness.admin import restart_daemon

                restart_daemon(name)
            except Exception:  # noqa: BLE001
                pass
            proc.terminate()
            proc.wait(timeout=10)
        if owned:
            shutil.rmtree(profile, ignore_errors=True)
        result["elapsed_ms"] = round((time.perf_counter() - started) * 1000)
        emit(result)


if __name__ == "__main__":
    main()

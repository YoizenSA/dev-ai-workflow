"""ywai patches: the caller supplies every typed value; none reaches a model."""

import time
from unittest.mock import Mock

import pytest

from jev_ultrafast import agent as loop
from jev_ultrafast.browser import fingerprint


def login_page():
    state = {
        "url": "https://example.test/login",
        "title": "Login",
        "text": "Login",
        "scroll": {"y": 0},
        "actions": [
            {"id": "e1", "kind": "fill", "label": "Email", "role": "textbox", "value": "", "node": 1},
            {"id": "e2", "kind": "fill", "label": "Password", "name": "pw", "role": "textbox",
             "value": "", "node": 2, "secret": True},
            {"id": "wait", "kind": "wait", "label": "Wait"},
        ],
    }
    state["fingerprint"] = fingerprint(state)
    return state


def runner(values, choice="e2"):
    a = loop.Agent.__new__(loop.Agent)
    a.screenshots = False
    a.pending_text = None
    a.values = {k.strip().lower(): v for k, v in values.items()}
    p = login_page()
    a.state = {
        "browser": Mock(fresh=Mock(return_value=True), observe=Mock(return_value=p)),
        "page": p, "goal": "Log in", "history": [], "decisions": [], "status": "predicted",
        "started_at": time.perf_counter(), "record": False, "text_calls": [],
        "decision": {"choice": choice, "operation": "TYPE_TEXT", "target": "1", "confidence": 1.0,
                     "probabilities": {choice: 1.0}, "latency_ms": 1, "usage": {}},
    }
    return a


def act(a):
    return a.command("act", {"fingerprint": a.state["page"]["fingerprint"]})


def test_matched_value_skips_llm_and_is_redacted(monkeypatch):
    a = runner({" PW ": "hunter2-secret"})
    snap = act(a)
    assert a.state["browser"].act.call_args.kwargs["text"] == "hunter2-secret"
    assert snap["history"][0]["text"] == "***"
    assert "hunter2-secret" not in repr(snap)


def test_label_match_is_case_insensitive(monkeypatch):
    a = runner({"email": "me@example.test"}, choice="e1")
    act(a)
    assert a.state["browser"].act.call_args.kwargs["text"] == "me@example.test"


def test_unmatched_password_blocks_without_llm(monkeypatch):
    a = runner({"email": "me@example.test"})
    snap = act(a)
    assert snap["status"] == "blocked"
    a.state["browser"].act.assert_not_called()


def test_goal_with_a_value_is_refused_before_any_network(monkeypatch):
    monkeypatch.setattr(loop, "Browser", Mock(side_effect=AssertionError("no browser")))
    monkeypatch.setattr(loop, "choose", Mock(side_effect=AssertionError("no model")))
    with pytest.raises(ValueError, match="goal"):
        loop.Agent("https://example.test", "Log in with hunter2-secret", values={"pw": "hunter2-secret"})


def test_unmatched_field_blocks_without_llm(monkeypatch):
    a = runner({"pw": "hunter2-secret"}, choice="e1")
    snap = act(a)
    assert snap["status"] == "blocked"
    assert snap["blocked_reason"] == "missing value: 'Email' (use any key: 'Email')"
    a.state["browser"].act.assert_not_called()


def test_cli_refuses_leaky_goal_without_launching_chrome(monkeypatch, capsys):
    import io
    import json

    from jev_ultrafast import cli

    monkeypatch.setattr(cli, "launch_chrome", Mock(side_effect=AssertionError("no chrome")))
    request = {"url": "https://example.test", "goal": "Use hunter2-secret", "values": {"pw": "hunter2-secret"}}
    monkeypatch.setattr("sys.stdin", io.StringIO(json.dumps(request)))
    cli.main()
    out = capsys.readouterr().out
    result = json.loads(out.strip().splitlines()[-1])
    assert result["type"] == "result" and result["status"] == "error"
    assert "goal" in result["error"]
    assert "hunter2-secret" not in out


def test_navigation_during_observe_is_stale_not_fatal(monkeypatch):
    from jev_ultrafast import browser

    def gone(*_a, **_k):
        raise RuntimeError("evaluate: Execution context was destroyed, most likely because of a navigation")

    monkeypatch.setattr(browser, "cdp", gone)
    with pytest.raises(browser.StalePage):
        browser.browser_operation({"operation": "observe", "session": "s", "screenshot": False})


def test_navigation_during_act_is_not_retried(monkeypatch):
    from jev_ultrafast import browser

    def gone(*_a, **_k):
        raise RuntimeError("evaluate: Execution context was destroyed")

    monkeypatch.setattr(browser, "cdp", gone)
    action = {"id": "e1", "kind": "click", "node": 1}
    with pytest.raises(RuntimeError) as err:
        browser.browser_operation({"operation": "act", "session": "s", "action": action, "page": {}})
    assert not isinstance(err.value, browser.StalePage)


def test_secret_field_reports_filled_without_its_value():
    from pathlib import Path

    js = (Path(loop.__file__).parent / "snapshot.js").read_text()
    assert "secret ? (e.value ? '(filled)' : '')" in js


def test_final_page_snapshot_lists_text_and_controls_without_secrets():
    from jev_ultrafast.cli import final_page

    page = login_page()
    page["text"] = "Login\nInvalid credentials"
    page["actions"][0]["value"] = "me@example.test"
    snap = final_page(page)
    assert snap["url"] == "https://example.test/login"
    assert "Invalid credentials" in snap["snapshot"]
    assert '- textbox "Email": me@example.test' in snap["snapshot"]
    assert '- textbox "Password"' in snap["snapshot"]
    assert "wait" not in snap["snapshot"].lower()


def test_value_matches_by_placeholder():
    action = {"label": "New Todo Input", "placeholder": "What needs to be done?"}
    assert loop.supplied_value(action, {"what needs to be done?": "Buy milk"}) == "Buy milk"


def test_snapshot_js_exposes_placeholder():
    from pathlib import Path

    js = (Path(loop.__file__).parent / "snapshot.js").read_text()
    assert "base.placeholder=e.getAttribute('placeholder')" in js


def test_final_page_shows_checked_state():
    from jev_ultrafast.cli import final_page

    page = {"url": "u", "title": "t", "text": "", "actions": [
        {"kind": "click", "role": "checkbox", "label": "checkbox 1", "checked": "true"},
        {"kind": "click", "role": "checkbox", "label": "checkbox 2", "checked": "false"},
    ]}
    snap = final_page(page)["snapshot"]
    assert '- checkbox "checkbox 1" [checked]' in snap
    assert '- checkbox "checkbox 2" [unchecked]' in snap


def test_trailing_newline_types_text_then_presses_enter(monkeypatch):
    from jev_ultrafast import browser

    calls = []

    def fake_cdp(method, **params):
        calls.append((method, params))
        if method == "Runtime.evaluate":
            return {"result": {"value": {"x": 10, "y": 10}}}
        return {}

    monkeypatch.setattr(browser, "cdp", fake_cdp)
    action = {"id": "e1", "kind": "fill", "node": 1}
    browser.browser_operation({"operation": "act", "session": "s", "action": action, "page": {}, "text": "Buy milk\n"})
    typed = [p["text"] for m, p in calls if m == "Input.insertText"]
    enter = [p["type"] for m, p in calls if m == "Input.dispatchKeyEvent" and p.get("key") == "Enter"]
    assert typed == ["Buy milk"]
    assert enter == ["keyDown", "keyUp"]


def test_value_matches_by_associated_label_or_id():
    action = {"label": "name@example.com", "labels": ["Email"], "elid": "userEmail"}
    assert loop.supplied_value(action, {"email": "a@b.test"}) == "a@b.test"
    assert loop.supplied_value(action, {"useremail": "c@d.test"}) == "c@d.test"


def test_snapshot_js_exposes_labels_and_id():
    from pathlib import Path

    js = (Path(loop.__file__).parent / "snapshot.js").read_text()
    assert "base.labels=" in js and "base.elid=e.id" in js


def test_final_page_collapses_a_dropdown_to_one_line():
    from jev_ultrafast.cli import final_page

    opts = [{"kind": "select", "node": 7, "role": "combobox", "label": f"Sort → {o}",
             "current_value": "Price (low to high)"} for o in ("Name (A to Z)", "Name (Z to A)")]
    snap = final_page({"url": "u", "text": "", "actions": opts})["snapshot"]
    assert snap.splitlines() == ['- combobox "Sort": Price (low to high)']


def test_jev_blocked_records_why(monkeypatch):
    a = runner({}, choice="BLOCKED")
    a.state["decision"]["probabilities"] = {"BLOCKED": 0.6, "e1": 0.3, "wait": 0.1}
    snap = act(a)
    assert snap["status"] == "blocked"
    assert snap["blocked_reason"].startswith("Jev chose BLOCKED (60%)")
    assert "Email 30%" in snap["blocked_reason"]


def test_missing_value_lists_every_usable_key():
    a = runner({"pw": "x"}, choice="e1")
    a.state["page"]["actions"][0].update(label="name@example.com", placeholder="name@example.com", elid="userEmail")
    snap = act(a)
    assert snap["blocked_reason"] == "missing value: 'name@example.com' (use any key: 'name@example.com', 'userEmail')"


def test_jev_blocked_lists_runner_up_operations():
    a = runner({}, choice="BLOCKED")
    a.state["decision"]["probabilities"] = {"BLOCKED": 0.55}
    a.state["decision"]["operation_probabilities"] = {"BLOCKED": 0.55, "CLICK": 0.3, "TYPE_TEXT": 0.15}
    assert act(a)["blocked_reason"] == "Jev chose BLOCKED (55%); next: CLICK 30%, TYPE_TEXT 15%"

"""Jev chooses an observed action. Code owns execution.

ywai patch: no eager imports. browser_harness reads BU_NAME at import time, and cli.py must set it first.
Import `jev_ultrafast.agent.Agent` / `jev_ultrafast.browser.Browser` directly.
"""

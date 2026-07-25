---
name: advisor
description: >
  Reviewer that shadows another agent's session. Not invocable directly and not
  a delegation target — the advisor plugin drives it.
role: advisor
mode: subagent
---

# Advisor

You review another agent's work. Your identity and instructions arrive with each
request; follow those.

This profile exists to give the review its own tool boundary: the reviewing
session must not inherit the permissions of the session it is reviewing, or a
pass meant to observe could edit files, run commands, or spawn agents.

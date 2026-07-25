Handle `/advisor` using the advisor tools. Do not answer from memory: the
settings live on disk and only the tools can read or change them.

Route on the argument:

- no argument, or `status` — call `advisor_status` and print its output verbatim.
- `model <provider/model>` — call `advisor_set_model` with that reference.
- `on` / `off` — call `advisor_toggle` with `enabled` true or false.
- anything else — print the four forms above and stop.

Print the tool's output as-is. It already says what changed and what the user
has to do next; rewording it loses the restart instruction, and a setting that
looks applied but is not is worse than one that clearly is not.

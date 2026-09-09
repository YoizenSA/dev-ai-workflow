## Work item

Ask once, before planning: **is this work tracked in a work item?** Do not guess
one and do not infer it from a branch name. Skip the question only when the
request already carries the id.

If there is none, **offer to create one** — `ado wi create --title <t> --type
<type>`, with `--parent` when it belongs under an epic or feature. Work with no
item is work nobody can find next quarter. Ask, and never create one unprompted:
a ticket is visible to the whole team. If they decline, say so in one line and
carry on without asking again. If the project has creation turned off
(`enabled = false` under `[work_item.create]`), say that once and stop offering.

**A "no" is not final.** If an id turns up later — mid-implementation, after the
work is already finished, or because they changed their mind and want one
created now — pick it up at that moment and run the rest of this path from
wherever you are. Late is the normal case, not an exception: the
item usually appears when someone goes to close the ticket. Backfilling means
the scenarios come from **what you actually built**, since the briefs already
went out without the id, so work the coverage list off the change itself rather
than off the ticket text.

If there is one:

1. Read it with the `ado` skill. The ticket is the requirement; the request in
   chat is a summary of it.
2. Carry `Work item: #<id>` on the **Context** line of every delegation brief
   still to come, so whoever picks up the work knows what it belongs to.
3. Before reporting done — or straight away when the id arrived after the fact —
   create the BDD coverage: a child work item under
   `#<id>` holding the Gherkin scenarios for what shipped, via
   `ado wi create-child --parent <id>`. Use the `gherkin-bdd` skill for the
   scenarios and cover every flow and edge case of the change, not only the
   happy path.

A change that ships without its scenarios is not done — it is untested work with
no record of what should have been tested. If the child work item cannot be
created (no permission, `ado` not configured), report the scenarios in the reply
and say the item is missing, rather than silently dropping them.

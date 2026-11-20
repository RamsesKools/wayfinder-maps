# Map format

## The format contract

The contract is the skill's local-markdown adapter, [`TRACKER-MARKDOWN.md`](https://github.com/RamsesKools/wayfinder-maps/blob/main/skills/wayfinder-maps/TRACKER-MARKDOWN.md); the skill itself is tracker-agnostic method.
This tool implements that adapter, and any other reader should target the same file.

In summary: structure the facts, leave the prose alone.
A ticket's type and edges have exactly one correct value a machine can check.
Its **Question** and **Answer**, the map's **Destination** and **Notes**, and each one-line gist in Decisions-so-far are lossy prose, never parsed.

A ticket is `.plan/<effort>/tickets/NN-<slug>.md`:

```markdown
---
type: research | prototype | grilling | task | implement
blocked_by: [02, 03] # [] when none
claimed_by: <session id> # while a session holds it
claimed_at: 2026-07-10T09:00:00Z # RFC 3339, alongside claimed_by
undermined_by: [08] # optional, resolved on a premise 08 later broke
assets: [../assets/x.html.approved] # optional
pr: https://github.com/o/r/pull/12 # optional, implement only
afk: true # optional, task only: an agent can drive it alone
---

# <Ticket title>

## Question

...

## Answer # its presence _is_ the resolution

...
```

An `implement` ticket swaps **Question** for **What to build** plus `- [ ]` acceptance criteria, targets one repo and usually one PR, and resolves when the work has shipped.
Its **Answer** records what shipped and links the PR or commit when there is one.
The map indexes it under **Implemented so far**, the counterpart of Decisions-so-far, see [Implement tickets](../features/implement-tickets.md).

Research and implement tickets are AFK (an agent can take them), grilling and prototype are HITL, and a task is HITL unless it sets `afk: true`.

## Status is derived

There is no `status:` field: every value it could hold is already written in the file, and a second copy is free to go stale.

| Derived status | When                                                |
| -------------- | --------------------------------------------------- |
| `resolved`     | the body has an `## Answer` **with prose under it** |
| `out_of_scope` | the body has a `## Ruled out` with prose under it   |
| `claimed`      | neither, and `claimed_by` is set                    |
| `open`         | none of the above                                   |

Closure is read first, so a `claimed_by` left on a closed ticket is inert litter.
And it is the prose, not the heading, that closes a ticket: a session that types `## Answer` and dies has resolved nothing.
The ticket stays claimed, and its claim goes stale on schedule.
`lint` errors on the empty heading outright.

- **`undermined_by`** marks a decision whose premise a later ticket broke; without it a renderer paints the node green and launders a live problem into a checkmark.
- **`claimed_at`** separates a dead session from live work.
- **`out_of_scope`** stays distinct from `resolved`: a scope boundary is not a step on the route, and a parser that lumps them in over-counts.

## Fenced code blocks are not structure

Every scan (headings, titles, bullets, links) ignores what sits inside a ` ``` ` or `~~~` fence.
A ticket that quotes the ticket format contains the line `## Answer`, and must not thereby resolve itself.
The rule cuts one way only: an answer whose entire body is a code fence is still an answer.
Any other reader of these files owes the same rule.

## One session at a time

The adapter forbids concurrent sessions: git merges a duplicate ticket number cleanly and neither session can see it.
`lint` still reports duplicates, because a merge can happen anyway.

## Fog

Fog patches in the map's `## Not yet specified` are bullets with a bolded lead title and an optional anchor; the rest stays loose prose:

```markdown
- **Today's row.** Whether today is visually distinguished. <clears-with: 04>
```

## Checks

`lint` reports errors and warnings, and exits `0` clean, `1` errors found, `2` no map at that path.

Errors:

- dangling or self `blocked_by`, and `blocked_by` cycles;
- duplicate ticket numbers;
- a ticket carrying both `## Answer` and `## Ruled out`, or a closing heading with nothing under it;
- a resolved ticket missing from Decisions-so-far (Implemented-so-far for an implement ticket);
- an out-of-scope ticket listed as a decision or implemented, or missing from Out-of-scope;
- Decisions-so-far or Implemented-so-far pointing at an unresolved ticket;
- a fog patch duplicating a live ticket's title, or anchored to a resolved ticket;
- a missing Destination, or an unknown type;
- a leftover `status:` field that disagrees with the body;
- a `pr` that is not a single http(s) URL.

Warnings:

- loose (pre-frontmatter) headers, and a `status:` field that merely duplicates the body;
- a claim with no owner or older than 72h, or a claim left on a closed ticket;
- a ticket blocked by something out of scope, which can never unblock;
- `pr` on a non-implement ticket, or `afk` on a non-task ticket;
- an implement ticket in Decisions-so-far, or any other ticket in Implemented-so-far.

Checks that need judgment (does a gist say what its answer says, does a fog title duplicate a live ticket _in substance_) stay with the skill; no parser can make them.

## Reading legacy maps

Two older shapes still parse, each lints as a warning, and neither is written any more:

- the loose header (`Type:` / `Status:` / `Blocked by:` after the H1), from before frontmatter;
- a stored `status:` field, from before status was derived.
  Where it agrees with the body, `lint` says to delete it; where it disagrees, that is an error and the body wins.

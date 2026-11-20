# Implement tickets

A map no longer stops at the last decision: `implement` tickets put the build on the same graph.

- Each one ships in one repo, usually as one PR, with a `## What to build` body plus `- [ ]` acceptance criteria, and resolves when the work has shipped.
- The optional `pr:` field links the PR when there is one.
- `## Implemented so far` in the map lists shipped work, next to Decisions-so-far.

The [map format](../guide/map-format.md) has the full ticket shape.

## Planning the implementation

A "Plan the implementation of X" grilling ticket breaks the destination into implement tickets, following [`IMPLEMENT-PLAN.md`](https://github.com/RamsesKools/wayfinder-maps/blob/main/skills/wayfinder-maps/IMPLEMENT-PLAN.md).
New questions that turn up while building go on the same map as grilling, research or prototype tickets.

## In the viewer

Implement tickets are hexagonal stations in an outer belt, and the ticket panel links the PR when there is one.

![The star-map with teal implement stations in an outer belt and the HUD counting decisions and implementation apart](../images/demo.png)

## status, lint and graph JSON

With implement tickets, `status` counts decisions and implementation separately and lists in-flight and shipped implement tickets, with their PR when they have one:

```
Decisions:      9 resolved · 0 claimed · 0 open · 0 out of scope
Implementation: 1 resolved · 1 claimed · 2 open · 0 out of scope
```

`lint` checks that resolved implement tickets sit under Implemented so far and that `pr` is a single http(s) URL, see [Checks](../guide/map-format.md#checks).
The graph JSON the viewer reads carries the same split.

[`pull`](pull.md) imports the `wayfinder:implement` label and the linked PRs from GitHub issues.

# AFK or HITL frontier

Every frontier ticket says whether an agent can take it alone (AFK, away from keyboard) or it needs you (HITL, human in the loop).
A hub session can then hand the AFK ones to parallel workers while you grill through the rest.

- Research and implement tickets are AFK.
- Grilling and prototype tickets are HITL.
- A task is HITL unless it sets `afk: true`; on GitHub the `wayfinder:afk` label does the same.

`status` marks every frontier row and counts both kinds:

```
Frontier — ready to claim, first by number wins (1 AFK, 0 HITL):
  14  Add the empty-day card                          implement  AFK
```

In the viewer, frontier tickets carry an agent or person badge, and the HUD counts how many agents can take and how many need you.

![The star-map with agent and person badges on frontier tickets](../images/demo.png)

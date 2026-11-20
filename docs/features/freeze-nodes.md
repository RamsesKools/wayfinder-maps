# Freeze nodes

Stars drift by a couple of pixels so the map feels alive, which some people find distracting.
The gear button at the right end of the top bar opens a small settings popover with one switch, Freeze nodes.
The popover unfolds out of the gear, and folds back into it when closed.

![The gear button at the right end of the top bar](../images/freeze-gear.png)

![The settings popover with the Freeze nodes switch off](../images/freeze-popover.png)

![The Freeze nodes switch on](../images/freeze-on.png)

Each setting has a small `?` next to its name: hover or focus it for a note on what the setting does.

![The hover note for Freeze nodes hanging below the popover](../images/freeze-help.png)

Freezing only removes the idle drift.
The layout is unchanged, and everything else keeps moving: flow particles on edges, the frontier pulse, claimed rings, the undermined halo, status flares, grow-in and the glide to new positions after a live update.
The choice is saved in `settings.json` next to the [compass](compass.md) direction, so it survives restarts.

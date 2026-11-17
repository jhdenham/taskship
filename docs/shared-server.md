# Shared server (planned v0.2)

Taskship will support an optional server, which you run yourself, so your machines share the same tasks. Each project can stay local-only or be connected to a server. A private network such as Tailscale can restrict access.

Every machine keeps its own copy of your tasks and keeps working when the server can't be reached, whether Tailscale is down, the server is off or you have no network. Changes sync when the server is back.

The server stores tasks; the agent runs locally on whichever machine invokes `taskship run`.

## Versions and upgrades

You won't have to upgrade your machines and the server together, or in a set order. A newer `taskship` syncs with an older server, and the other way around:

- A server keeps the fields an older `taskship` doesn't know about. Syncing from an older machine never clears them.
- If a newer `taskship` has fields the server doesn't support yet, they stay on that machine and sync once the server is upgraded. Everything else syncs as usual.
- When a version is too old to sync at all, Taskship says which version you need and keeps working locally until you upgrade.

Upgrading the server first is still a good habit. Before the server updates its database, it saves a copy, so you can go back to the older version by restoring that copy.

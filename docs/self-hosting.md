# Self-hosting (planned v0.2)

Taskship will support an optional central REST API backed by PostgreSQL or SQLite. Configure clients on each machine to point to the same server. A private network such as Tailscale can restrict access.

**Important:** Remote mode is online-only in v0.2. Local SQLite data is not automatically synchronized with server data. Import/export is an explicit operation.

The server stores tasks; the agent runs locally on whichever machine invokes `taskship run`.

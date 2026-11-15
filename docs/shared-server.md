# Shared server (planned v0.2)

Taskship will support an optional server, which you run yourself, so your machines share the same tasks. Each project can stay local-only or be connected to a server. A private network such as Tailscale can restrict access.

Every machine keeps its own copy of your tasks and keeps working when the server can't be reached, whether Tailscale is down, the server is off or you have no network. Changes sync when the server is back.

The server stores tasks; the agent runs locally on whichever machine invokes `taskship run`.

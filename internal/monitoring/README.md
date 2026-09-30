# Monitoring Engine

The v0.7 engine uses one scheduler and a bounded worker pool (4 workers by default). It supports HTTP/HTTPS and TCP checks, persists the latest state in SQLite, and applies progressive failure backoff capped at 300 seconds. Disabled monitors are never scheduled. Application shortcuts read the persisted state through the page runtime response, so rendering a dashboard does not trigger a network check.

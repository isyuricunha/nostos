# Tasks

Tasks are stored in the database and executed by the worker.

Supported schedule modes:

- `manual`
- `one_time`
- `cron`
- `interval`

Schedules use IANA timezone names and calculate next execution using timezone-aware cron parsing.

Task runs move through `queued`, `running`, `succeeded`, `failed`, `cancelled`, and `timed_out` states. Leases allow abandoned work to be recovered by another worker.

System-managed maintenance tasks are visible in the task list and can be enabled, disabled, and rescheduled. They use bounded criteria and do not run destructive unbounded cleanup.

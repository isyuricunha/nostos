# Tasks

Tasks are stored in the database and executed by the worker.

Supported schedule modes:

- `manual`
- `one_time`
- `cron`
- `interval`

Schedules use IANA timezone names and calculate next execution using timezone-aware cron parsing.

Task runs move through `queued`, `running`, `succeeded`, `failed`, `cancelled`, and `timed_out` states. Leases are renewed while work is healthy and allow abandoned work to be recovered by another worker.

Worker concurrency is bounded by `WORKER_CONCURRENCY`. Each worker process starts a fixed number of claim/execution loops and stops claiming new work during graceful shutdown.

Concurrency policies are enforced when runs are enqueued:

- `allow`: overlapping runs are allowed.
- `skip`: a new run is skipped while another run for the same task is active.
- `replace`: active runs are cancelled before the new run is queued.

Scheduled occurrences are claimed with an atomic database update before enqueueing so only one worker advances a specific occurrence. The task-run idempotency key is retained as a second defense.

System-managed maintenance tasks are visible in the task list and can be enabled, disabled, and rescheduled. They use bounded criteria and do not run destructive unbounded cleanup.

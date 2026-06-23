<script lang="ts">
  import EmptyState from '../components/common/EmptyState.svelte';
  import StatusPill from '../components/common/StatusPill.svelte';
  import ModelPicker from '../components/models/ModelPicker.svelte';
  import type { Agent, Provider, ProviderModel, TaskRecord, TaskRun, TaskRunEvent, TaskToolCall } from '../lib/types';
  import { strings } from '../strings';

  export let taskRecords: TaskRecord[] = [];
  export let taskRuns: TaskRun[] = [];
  export let taskRunEvents: TaskRunEvent[] = [];
  export let taskRunToolCalls: TaskToolCall[] = [];
  export let agents: Agent[] = [];
  export let providers: Provider[] = [];
  export let providerModels: ProviderModel[] = [];
  export let editingTaskId = '';
  export let taskName = '';
  export let taskDescription = '';
  export let taskPrompt = '';
  export let taskType = 'agent';
  export let taskState = 'enabled';
  export let taskAgentId = '';
  export let taskProviderId = '';
  export let taskModel = '';
  export let taskScheduleMode = 'manual';
  export let taskCronExpression = '';
  export let taskIntervalSeconds = 3600;
  export let taskRunAt = '';
  export let taskToolPolicy = 'use_preapproved_tools_only';
  export let taskMaxRetries = 3;
  export let taskTimeoutMS = 600000;
  export let taskConcurrencyPolicy = 'skip';
  export let submitting = false;
  export let onSubmit: () => void | Promise<void>;
  export let onCancelEdit: () => void;
  export let onRefresh: () => void | Promise<void>;
  export let onEdit: (record: TaskRecord) => void;
  export let onDelete: (taskId: string) => void | Promise<void>;
  export let onRunTask: (taskId: string) => void | Promise<void>;
  export let onCancelRun: (runId: string) => void | Promise<void>;
  export let onRetryRun: (runId: string) => void | Promise<void>;
  export let onShowEvents: (runId: string) => void | Promise<void>;
  export let taskNameForRun: (run: TaskRun) => string;

  function runTone(state: string): 'success' | 'warning' | 'danger' | 'neutral' | 'accent' {
    if (state === 'succeeded') return 'success';
    if (state === 'failed' || state === 'timed_out') return 'danger';
    if (state === 'running' || state === 'queued' || state === 'waiting') return 'accent';
    return 'neutral';
  }
</script>

<section class="providers-layout">
  <form class="panel form-grid" on:submit|preventDefault={onSubmit}>
    <div class="panel-heading">
      <div>
        <p class="eyebrow">Automation</p>
        <h2>{editingTaskId ? 'Edit task' : strings.tasks.add}</h2>
      </div>
      {#if editingTaskId}
        <button on:click={onCancelEdit} type="button">Cancel edit</button>
      {/if}
    </div>

    <div class="form-section">
      <h3>Identity</h3>
      <label>
        Name
        <input bind:value={taskName} required />
      </label>
      <label>
        Description
        <input bind:value={taskDescription} />
      </label>
      <label>
        Type
        <select bind:value={taskType}>
          <option value="agent">agent</option>
          <option value="system">system</option>
        </select>
      </label>
      <label>
        State
        <select bind:value={taskState}>
          <option value="draft">draft</option>
          <option value="enabled">enabled</option>
          <option value="disabled">disabled</option>
        </select>
      </label>
      <label>
        Prompt
        <textarea bind:value={taskPrompt} required></textarea>
      </label>
    </div>

    {#if taskType === 'agent'}
      <div class="form-section">
        <h3>Agent runtime</h3>
        <label>
          Agent
          <select bind:value={taskAgentId}>
            <option value="">Task default agent behavior</option>
            {#each agents as agent (agent.id)}
              <option value={agent.id}>{agent.name}</option>
            {/each}
          </select>
        </label>
        <ModelPicker
          bind:selectedModelId={taskModel}
          bind:selectedProviderId={taskProviderId}
          label="Task model override"
          models={providerModels}
          {providers}
          role="utility"
        />
      </div>
    {/if}

    <div class="form-section">
      <h3>Schedule</h3>
      <label>
        Schedule
        <select bind:value={taskScheduleMode}>
          <option value="manual">manual</option>
          <option value="one_time">one_time</option>
          <option value="cron">cron</option>
          <option value="interval">interval</option>
        </select>
      </label>
      {#if taskScheduleMode === 'cron'}
        <label>
          Cron expression
          <input bind:value={taskCronExpression} placeholder="0 * * * *" required />
        </label>
      {:else if taskScheduleMode === 'interval'}
        <label>
          Interval seconds
          <input bind:value={taskIntervalSeconds} min="1" type="number" />
        </label>
      {:else if taskScheduleMode === 'one_time'}
        <label>
          Run at
          <input bind:value={taskRunAt} placeholder="2026-06-22T12:00:00Z" required />
        </label>
      {/if}
    </div>

    <div class="form-section">
      <h3>Safety</h3>
      <label>
        Tool policy
        <select bind:value={taskToolPolicy}>
          <option value="use_preapproved_tools_only">use_preapproved_tools_only</option>
          <option value="fail_if_approval_required">fail_if_approval_required</option>
        </select>
      </label>
      <label>
        Maximum retries
        <input bind:value={taskMaxRetries} min="0" max="20" type="number" />
      </label>
      <label>
        Timeout, milliseconds
        <input bind:value={taskTimeoutMS} min="1000" type="number" />
      </label>
      <label>
        Concurrency policy
        <select bind:value={taskConcurrencyPolicy}>
          <option value="allow">allow</option>
          <option value="skip">skip</option>
          <option value="replace">replace</option>
        </select>
      </label>
    </div>

    <button disabled={submitting} type="submit">{editingTaskId ? 'Save task' : strings.tasks.add}</button>
  </form>

  <section class="panel">
    <div class="panel-heading">
      <div>
        <p class="eyebrow">Operations</p>
        <h2>Tasks</h2>
      </div>
      <button on:click={onRefresh} type="button">Refresh</button>
    </div>
    {#if taskRecords.length === 0}
      <EmptyState description="Manual, scheduled, and system tasks will appear here." title={strings.tasks.noTasks} />
    {:else}
      <div class="table-list task-cards">
        {#each taskRecords as record (record.task.id)}
          <article class:system-managed={record.task.system_managed}>
            <div>
              <div class="split">
                <strong>{record.task.name}</strong>
                <div class="cluster">
                  <StatusPill status={record.task.state} tone={record.task.state === 'enabled' ? 'success' : 'neutral'} />
                  {#if record.task.system_managed}
                    <StatusPill status="system" tone="accent" />
                  {/if}
                </div>
              </div>
              <span>{record.task.task_type} task / {record.schedule.mode} schedule</span>
              <span>
                {record.schedule.next_run_at
                  ? `next ${new Date(record.schedule.next_run_at).toLocaleString()}`
                  : 'no next run'}
              </span>
              <span>
                retries {record.task.max_retries} / timeout {Math.round(record.task.timeout_ms / 1000)}s / concurrency
                {record.task.concurrency_policy}
              </span>
              {#if record.task.agent_id || record.task.provider_id || record.task.model}
                <span>
                  {record.task.agent_id ? 'agent configured' : ''}
                  {record.task.provider_id ? ' / provider override' : ''}
                  {record.task.model ? ` / ${record.task.model}` : ''}
                </span>
              {/if}
            </div>
            <div>
              <button on:click={() => onEdit(record)} type="button">Edit</button>
              <button on:click={() => onRunTask(record.task.id)} type="button">{strings.tasks.runNow}</button>
              {#if !record.task.system_managed}
                <button on:click={() => onDelete(record.task.id)} type="button">Delete</button>
              {/if}
            </div>
          </article>
        {/each}
      </div>
    {/if}

    <div class="panel-heading nested-heading">
      <div>
        <p class="eyebrow">Execution history</p>
        <h2>Runs</h2>
      </div>
    </div>
    {#if taskRuns.length === 0}
      <EmptyState description="Task runs and retry history will be shown after execution." title={strings.tasks.noRuns} />
    {:else}
      <div class="table-list">
        {#each taskRuns as run (run.id)}
          <article>
            <div>
              <div class="split">
                <strong>{taskNameForRun(run)}</strong>
                <StatusPill status={run.state} tone={runTone(run.state)} />
              </div>
              <span>attempt {run.attempt + 1} of {run.max_retries + 1}</span>
              <span>Queued {new Date(run.queued_at).toLocaleString()}</span>
              {#if run.result}
                <span>{run.result}</span>
              {/if}
              {#if run.error_message}
                <span class="danger-text">{run.error_message}</span>
              {/if}
            </div>
            <div>
              <button on:click={() => onShowEvents(run.id)} type="button">{strings.tasks.events}</button>
              {#if run.state === 'queued' || run.state === 'running' || run.state === 'waiting'}
                <button on:click={() => onCancelRun(run.id)} type="button">{strings.tasks.cancel}</button>
              {/if}
              {#if run.state === 'failed' || run.state === 'timed_out' || run.state === 'cancelled'}
                <button on:click={() => onRetryRun(run.id)} type="button">{strings.tasks.retry}</button>
              {/if}
            </div>
          </article>
        {/each}
      </div>
    {/if}

    {#if taskRunEvents.length > 0}
      <div class="event-log">
        {#each taskRunEvents as event (event.id)}
          <p><strong>{event.level}</strong> {new Date(event.created_at).toLocaleString()} - {event.message}</p>
        {/each}
      </div>
    {/if}

    {#if taskRunToolCalls.length > 0}
      <div class="event-log">
        {#each taskRunToolCalls as call (call.id)}
          <p>
            <strong>{call.state}</strong>
            {call.tool_name}
            <span>permission {call.permission_decision}</span>
            {#if call.duration_ms}
              <span>{call.duration_ms} ms</span>
            {/if}
            {#if call.result_truncated}
              <span>truncated</span>
            {/if}
            {#if call.error_message}
              <span class="danger-text">{call.error_message}</span>
            {/if}
          </p>
        {/each}
      </div>
    {/if}
  </section>
</section>

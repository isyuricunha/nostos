<script lang="ts">
  import Icon from '../components/common/Icon.svelte';
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
  export let onToggleState: (record: TaskRecord) => void | Promise<void>;
  export let taskNameForRun: (run: TaskRun) => string;

  type Tab = 'tasks' | 'runs' | 'details';

  let activeTab: Tab = 'tasks';
  let query = '';
  let typeFilter: 'all' | 'agent' | 'system' = 'all';
  let stateFilter = 'all';
  let formOpen = false;
  let openMenuId = '';

  $: if (editingTaskId) {
    formOpen = true;
  }
  $: filteredTasks = taskRecords.filter((record) => {
    const haystack = `${record.task.name} ${record.task.description} ${record.task.model ?? ''} ${record.schedule.mode}`.toLowerCase();
    const matchesSearch = haystack.includes(query.trim().toLowerCase());
    const matchesType =
      typeFilter === 'all' ||
      (typeFilter === 'agent' && !record.task.system_managed && record.task.task_type === 'agent') ||
      (typeFilter === 'system' && (record.task.system_managed || record.task.task_type === 'system'));
    const matchesState = stateFilter === 'all' || record.task.state === stateFilter;
    return matchesSearch && matchesType && matchesState;
  });

  function openCreate(): void {
    onCancelEdit();
    formOpen = true;
  }

  function closeForm(): void {
    onCancelEdit();
    formOpen = false;
  }

  function startEdit(record: TaskRecord): void {
    onEdit(record);
    formOpen = true;
    openMenuId = '';
  }

  async function submitForm(): Promise<void> {
    await onSubmit();
    formOpen = false;
  }

  async function showRunDetails(runId: string): Promise<void> {
    await onShowEvents(runId);
    activeTab = 'details';
  }

  function runTone(state: string): string {
    if (state === 'succeeded') return 'healthy';
    if (state === 'failed' || state === 'timed_out') return 'unhealthy';
    if (state === 'running' || state === 'queued' || state === 'waiting') return 'unknown';
    return 'disabled';
  }

  function formatDate(value = ''): string {
    return value ? new Date(value).toLocaleString() : 'Not scheduled';
  }
</script>

<div class="workspace-module-grid" class:editor-open={formOpen}>
  <section class="window-list-pane" aria-label="Tasks">
    <header class="window-panel-toolbar">
      <div>
        <strong>Tasks</strong>
        <span>{taskRecords.length} tasks · {taskRuns.length} runs</span>
      </div>
      <div class="window-toolbar-actions">
        <button aria-label="Refresh tasks" on:click={onRefresh} type="button"><Icon name="refresh" size={13} /></button>
        <button on:click={openCreate} type="button"><Icon name="plus" size={13} /> New task</button>
      </div>
    </header>

    <nav class="segmented-tabs" aria-label="Task tabs">
      <button class:active={activeTab === 'tasks'} on:click={() => (activeTab = 'tasks')} type="button">Tasks</button>
      <button class:active={activeTab === 'runs'} on:click={() => (activeTab = 'runs')} type="button">Runs</button>
      <button class:active={activeTab === 'details'} on:click={() => (activeTab = 'details')} type="button">Details</button>
    </nav>

    {#if activeTab === 'tasks'}
      <div class="window-filter-row">
        <label class="window-search">
          <Icon name="search" size={13} />
          <input bind:value={query} placeholder="Search tasks" />
        </label>
        <select aria-label="Task type" bind:value={typeFilter}>
          <option value="all">All</option>
          <option value="agent">User</option>
          <option value="system">System</option>
        </select>
        <select aria-label="Task state" bind:value={stateFilter}>
          <option value="all">All states</option>
          <option value="enabled">Enabled</option>
          <option value="disabled">Disabled</option>
          <option value="draft">Draft</option>
        </select>
      </div>

      {#if filteredTasks.length === 0}
        <p class="window-empty">{strings.tasks.noTasks}</p>
      {:else}
        <div class="dense-row-list">
          {#each filteredTasks as record (record.task.id)}
            <article class="task-row">
              <span class="row-icon"><Icon name="tasks" size={15} /></span>
              <span class={`status-dot ${record.task.state === 'enabled' ? 'healthy' : 'disabled'}`}></span>
              <div>
                <strong>{record.task.name}</strong>
                <span>{record.task.description || record.task.prompt}</span>
                <small>
                  {record.task.system_managed ? 'system' : 'user'} · {record.schedule.mode} · next {formatDate(record.schedule.next_run_at)}
                </small>
              </div>
              <div class="row-actions compact">
                <button aria-label={`Task menu for ${record.task.name}`} on:click={() => (openMenuId = openMenuId === record.task.id ? '' : record.task.id)} type="button">
                  <Icon name="kebab" size={14} />
                </button>
                {#if openMenuId === record.task.id}
                  <div class="row-menu row-menu-right" role="menu">
                    <button on:click={() => startEdit(record)} type="button"><Icon name="edit" size={13} /> Edit</button>
                    <button on:click={() => { openMenuId = ''; onRunTask(record.task.id); }} type="button">
                      <Icon name="send" size={13} /> {strings.tasks.runNow}
                    </button>
                    <button on:click={() => { openMenuId = ''; onToggleState(record); }} type="button">
                      <Icon name={record.task.state === 'enabled' ? 'minus' : 'check'} size={13} />
                      {record.task.state === 'enabled' ? 'Disable' : 'Enable'}
                    </button>
                    {#if !record.task.system_managed}
                      <button class="danger" on:click={() => { openMenuId = ''; onDelete(record.task.id); }} type="button">
                        <Icon name="trash" size={13} /> Delete
                      </button>
                    {/if}
                  </div>
                {/if}
              </div>
            </article>
          {/each}
        </div>
      {/if}
    {:else if activeTab === 'runs'}
      {#if taskRuns.length === 0}
        <p class="window-empty">{strings.tasks.noRuns}</p>
      {:else}
        <div class="dense-row-list">
          {#each taskRuns as run (run.id)}
            <article class="task-row">
              <span class="row-icon"><Icon name="tasks" size={15} /></span>
              <span class={`status-dot ${runTone(run.state)}`}></span>
              <div>
                <strong>{taskNameForRun(run)}</strong>
                <span>attempt {run.attempt + 1} of {run.max_retries + 1} · queued {formatDate(run.queued_at)}</span>
                <small>{run.result || run.error_message || run.state}</small>
              </div>
              <div class="row-actions">
                <button on:click={() => showRunDetails(run.id)} type="button">{strings.tasks.events}</button>
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
    {:else}
      {#if taskRunEvents.length === 0 && taskRunToolCalls.length === 0}
        <p class="window-empty">Select a run to inspect events and tool calls.</p>
      {:else}
        <div class="event-log compact">
          {#each taskRunEvents as event (event.id)}
            <p><strong>{event.level}</strong> {formatDate(event.created_at)} - {event.message}</p>
          {/each}
          {#each taskRunToolCalls as call (call.id)}
            <p>
              <strong>{call.state}</strong> {call.tool_name}
              <span>permission {call.permission_decision}</span>
              {#if call.duration_ms}<span>{call.duration_ms} ms</span>{/if}
              {#if call.result_truncated}<span>truncated</span>{/if}
              {#if call.error_message}<span class="danger-text">{call.error_message}</span>{/if}
            </p>
          {/each}
        </div>
      {/if}
    {/if}
  </section>

  {#if formOpen}
    <aside class="window-editor-panel" aria-label={editingTaskId ? 'Edit task' : 'Create task'}>
      <header>
        <strong>{editingTaskId ? 'Edit task' : strings.tasks.add}</strong>
        <button aria-label="Close task editor" on:click={closeForm} type="button"><Icon name="close" size={13} /></button>
      </header>
      <form class="compact-editor-form" on:submit|preventDefault={submitForm}>
        <label>
          Name
          <input bind:value={taskName} required />
        </label>
        <label>
          Description
          <input bind:value={taskDescription} />
        </label>
        <div class="two-col">
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
        </div>
        <label>
          Prompt
          <textarea bind:value={taskPrompt} required></textarea>
        </label>
        {#if taskType === 'agent'}
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
        {/if}
        <div class="two-col">
          <label>
            Schedule
            <select bind:value={taskScheduleMode}>
              <option value="manual">manual</option>
              <option value="one_time">one_time</option>
              <option value="cron">cron</option>
              <option value="interval">interval</option>
            </select>
          </label>
          <label>
            Tool policy
            <select bind:value={taskToolPolicy}>
              <option value="use_preapproved_tools_only">use_preapproved_tools_only</option>
              <option value="fail_if_approval_required">fail_if_approval_required</option>
            </select>
          </label>
        </div>
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
        <div class="two-col">
          <label>
            Max retries
            <input bind:value={taskMaxRetries} min="0" max="20" type="number" />
          </label>
          <label>
            Timeout, ms
            <input bind:value={taskTimeoutMS} min="1000" type="number" />
          </label>
        </div>
        <label>
          Concurrency
          <select bind:value={taskConcurrencyPolicy}>
            <option value="allow">allow</option>
            <option value="skip">skip</option>
            <option value="replace">replace</option>
          </select>
        </label>
        <div class="editor-actions">
          <button disabled={submitting} type="submit">{editingTaskId ? 'Save task' : strings.tasks.add}</button>
          <button on:click={closeForm} type="button">Cancel</button>
        </div>
      </form>
    </aside>
  {/if}
</div>

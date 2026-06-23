<script lang="ts" module>
  export type ToastType = 'success' | 'error' | 'warning' | 'info' | 'loading';

  export type AppToast = {
    id: string;
    type: ToastType;
    message: string;
    actionLabel?: string;
    onAction?: () => void | Promise<void>;
    persistent?: boolean;
  };
</script>

<script lang="ts">
  import Icon, { type IconName } from './Icon.svelte';

  export let toasts: AppToast[] = [];
  export let onDismiss: (toastId: string) => void = () => undefined;

  function iconFor(type: ToastType): IconName {
    if (type === 'success') return 'check';
    if (type === 'error') return 'close';
    if (type === 'warning') return 'details';
    if (type === 'loading') return 'refresh';
    return 'details';
  }

  async function runAction(toast: AppToast): Promise<void> {
    await toast.onAction?.();
    onDismiss(toast.id);
  }
</script>

<section class="toast-region" aria-label="Application feedback" aria-live="polite">
  {#each toasts as toast (toast.id)}
    <article class={`toast toast-${toast.type}`} role={toast.type === 'error' ? 'alert' : 'status'}>
      <span class="toast-icon" aria-hidden="true">
        <Icon name={iconFor(toast.type)} size={14} />
      </span>
      <p>{toast.message}</p>
      {#if toast.actionLabel && toast.onAction}
        <button class="toast-action" on:click={() => runAction(toast)} type="button">{toast.actionLabel}</button>
      {/if}
      <button class="toast-dismiss" aria-label="Dismiss notification" on:click={() => onDismiss(toast.id)} type="button">
        <Icon name="close" size={12} />
      </button>
    </article>
  {/each}
</section>

<style>
  .toast-region {
    position: fixed;
    z-index: 500;
    top: 12px;
    right: 14px;
    display: grid;
    width: min(380px, calc(100vw - 28px));
    gap: 8px;
    pointer-events: none;
  }

  .toast {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr) auto auto;
    align-items: center;
    gap: 8px;
    min-height: 38px;
    border: 1px solid var(--workspace-border-strong);
    border-left-color: var(--toast-accent, var(--color-accent));
    border-radius: 8px;
    background: rgba(12, 13, 13, 0.96);
    box-shadow: var(--workspace-window-shadow);
    color: var(--color-text);
    padding: 7px 8px;
    pointer-events: auto;
  }

  .toast p {
    min-width: 0;
    margin: 0;
    color: var(--color-text-soft);
    font-size: 0.8rem;
    line-height: 1.35;
  }

  .toast-icon {
    display: grid;
    width: 22px;
    height: 22px;
    place-items: center;
    border-radius: 5px;
    background: color-mix(in srgb, var(--toast-accent, var(--color-accent)) 14%, transparent);
    color: var(--toast-accent, var(--color-accent));
  }

  .toast-loading .toast-icon :global(svg) {
    animation: toast-spin 900ms linear infinite;
  }

  .toast-success {
    --toast-accent: var(--color-success);
  }

  .toast-error {
    --toast-accent: var(--color-danger);
  }

  .toast-warning {
    --toast-accent: var(--color-warning);
  }

  .toast-info,
  .toast-loading {
    --toast-accent: var(--color-info);
  }

  .toast-action,
  .toast-dismiss {
    border: 1px solid var(--workspace-border);
    border-radius: 6px;
    background: rgba(255, 255, 255, 0.035);
    color: var(--color-text);
    cursor: pointer;
    font: inherit;
  }

  .toast-action {
    min-height: 26px;
    padding: 0 8px;
    font-size: 0.75rem;
  }

  .toast-dismiss {
    display: grid;
    width: 26px;
    height: 26px;
    place-items: center;
    padding: 0;
  }

  .toast-action:hover,
  .toast-action:focus-visible,
  .toast-dismiss:hover,
  .toast-dismiss:focus-visible {
    border-color: var(--workspace-border-strong);
    background: rgba(255, 255, 255, 0.07);
    outline: none;
  }

  @keyframes toast-spin {
    to {
      transform: rotate(360deg);
    }
  }

  @media (max-width: 640px) {
    .toast-region {
      top: 58px;
      right: 9px;
      left: 9px;
      width: auto;
    }

    .toast {
      grid-template-columns: auto minmax(0, 1fr) auto;
    }

    .toast-action {
      grid-column: 2 / 3;
      justify-self: start;
    }
  }
</style>

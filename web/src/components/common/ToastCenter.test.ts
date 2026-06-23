import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import ToastCenter, { type AppToast } from './ToastCenter.svelte';

describe('ToastCenter', () => {
  it('announces feedback and dismisses notifications', async () => {
    const onDismiss = vi.fn();
    const toasts: AppToast[] = [
      {
        id: 'toast_1',
        type: 'success',
        message: 'Provider saved.'
      }
    ];

    render(ToastCenter, { toasts, onDismiss });

    expect(screen.getByRole('status').textContent).toContain('Provider saved.');

    await fireEvent.click(screen.getByRole('button', { name: 'Dismiss notification' }));

    expect(onDismiss).toHaveBeenCalledWith('toast_1');
  });

  it('runs retry actions before removing the toast', async () => {
    const onAction = vi.fn();
    const onDismiss = vi.fn();
    const toasts: AppToast[] = [
      {
        id: 'toast_retry',
        type: 'error',
        message: 'Provider test failed.',
        actionLabel: 'Retry',
        onAction,
        persistent: true
      }
    ];

    render(ToastCenter, { toasts, onDismiss });

    expect(screen.getByRole('alert').textContent).toContain('Provider test failed.');

    await fireEvent.click(screen.getByRole('button', { name: 'Retry' }));

    await waitFor(() => expect(onAction).toHaveBeenCalledTimes(1));
    expect(onDismiss).toHaveBeenCalledWith('toast_retry');
  });
});

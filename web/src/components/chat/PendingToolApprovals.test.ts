import { fireEvent, render, screen } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import PendingToolApprovals from './PendingToolApprovals.svelte';
import type { ToolCall } from '../../lib/types';

const toolCall: ToolCall = {
  id: 'call_1',
  chat_run_id: 'run_1',
  name: 'lookup_status',
  input: '{"service":"api"}',
  state: 'waiting_for_approval',
  approval_state: 'pending',
  created_at: '2026-06-22T12:00:00Z'
};

describe('PendingToolApprovals', () => {
  it('renders pending approval actions and dispatches decisions', async () => {
    const onApprove = vi.fn();
    const onDeny = vi.fn();
    const onRefresh = vi.fn();

    render(PendingToolApprovals, { toolCalls: [toolCall], onApprove, onDeny, onRefresh });

    expect(screen.getByText('lookup_status')).toBeTruthy();
    expect(screen.getByText('{"service":"api"}')).toBeTruthy();

    await fireEvent.click(screen.getByRole('button', { name: 'Refresh' }));
    await fireEvent.click(screen.getByRole('button', { name: 'Approve once' }));
    await fireEvent.click(screen.getByRole('button', { name: 'Deny and disable' }));

    expect(onRefresh).toHaveBeenCalledOnce();
    expect(onApprove).toHaveBeenCalledWith(toolCall, 'approve_once');
    expect(onDeny).toHaveBeenCalledWith(toolCall, 'deny_disable_tool');
  });

  it('does not render the approval section without pending calls', () => {
    render(PendingToolApprovals, { toolCalls: [], onApprove: vi.fn(), onDeny: vi.fn(), onRefresh: vi.fn() });

    expect(screen.queryByLabelText('Pending tool approvals')).toBeNull();
  });
});

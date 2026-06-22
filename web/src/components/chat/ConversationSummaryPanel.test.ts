import { fireEvent, render, screen } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import ConversationSummaryPanel from './ConversationSummaryPanel.svelte';
import type { Conversation } from '../../lib/types';

const conversation: Conversation = {
  id: 'conv_1',
  title: 'Conversation',
  summary: 'Yuri prefers concise Go backend answers.',
  summary_status: 'completed',
  summary_updated_at: '2026-06-22T12:00:00Z',
  updated_at: '2026-06-22T12:00:00Z'
};

describe('ConversationSummaryPanel', () => {
  it('renders summary status and runs summary actions', async () => {
    const onRegenerate = vi.fn();
    const onClear = vi.fn();

    render(ConversationSummaryPanel, { conversation, onRegenerate, onClear });

    expect(screen.getByText('Conversation summary')).toBeTruthy();
    expect(screen.getByText('completed')).toBeTruthy();
    expect(screen.getByText(conversation.summary ?? '')).toBeTruthy();

    await fireEvent.click(screen.getByRole('button', { name: 'Regenerate summary' }));
    await fireEvent.click(screen.getByRole('button', { name: 'Clear summary' }));

    expect(onRegenerate).toHaveBeenCalledOnce();
    expect(onClear).toHaveBeenCalledOnce();
  });

  it('shows an empty state when no summary exists', () => {
    render(ConversationSummaryPanel, {
      conversation: { ...conversation, summary: '', summary_status: 'idle', summary_updated_at: undefined },
      onRegenerate: vi.fn(),
      onClear: vi.fn()
    });

    expect(screen.getByText('No summary stored for this conversation.')).toBeTruthy();
  });
});

import { fireEvent, render, screen, within } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import AppShell from './AppShell.svelte';
import type { Conversation, User } from '../../lib/types';
import { strings } from '../../strings';

const user: User = {
  id: 'user_1',
  email: 'owner@example.com',
  display_name: 'Owner User',
  role: 'owner',
  workspace_id: 'workspace_1'
};

const conversations: Conversation[] = [
  {
    id: 'conversation_alpha',
    title: 'Alpha planning',
    updated_at: '2026-06-23T12:00:00Z'
  },
  {
    id: 'conversation_beta',
    title: 'Beta launch',
    summary: 'Launch notes are available.',
    updated_at: '2026-06-23T12:05:00Z'
  }
];

const callbacks = {
  onLogout: vi.fn(),
  onCreateConversation: vi.fn(),
  onSelectConversation: vi.fn(),
  onRenameConversation: vi.fn(),
  onArchiveConversation: vi.fn(),
  onUnarchiveConversation: vi.fn(),
  onDeleteConversation: vi.fn(),
  onRestoreWindow: vi.fn()
};

describe('AppShell', () => {
  it('filters conversations and keeps row actions behind the overflow menu', async () => {
    render(AppShell, {
      activeView: strings.nav.chat,
      navItems: [strings.nav.chat, strings.nav.memories, strings.nav.tasks],
      user,
      conversations,
      selectedConversationId: 'conversation_beta',
      ...callbacks
    });

    await fireEvent.input(screen.getByPlaceholderText('Search'), { target: { value: 'Beta' } });

    expect(screen.queryByText('Alpha planning')).toBeNull();
    expect(screen.getAllByText('Beta launch').length).toBeGreaterThan(0);

    await fireEvent.click(screen.getByRole('button', { name: 'Conversation menu for Beta launch' }));

    const menu = screen.getByRole('menu');
    expect(within(menu).getByRole('button', { name: /Rename/ })).toBeTruthy();
    expect(within(menu).getByRole('button', { name: /Archive/ })).toBeTruthy();
    expect(within(menu).getByRole('button', { name: /Delete/ })).toBeTruthy();
  });

  it('closes the conversation menu on outside click and Escape', async () => {
    render(AppShell, {
      activeView: strings.nav.chat,
      navItems: [strings.nav.chat, strings.nav.memories, strings.nav.tasks],
      user,
      conversations,
      selectedConversationId: 'conversation_beta',
      ...callbacks
    });

    await fireEvent.click(screen.getByRole('button', { name: 'Conversation menu for Beta launch' }));
    expect(screen.getByRole('menu')).toBeTruthy();

    await fireEvent.click(document.body);
    expect(screen.queryByRole('menu')).toBeNull();

    await fireEvent.click(screen.getByRole('button', { name: 'Conversation menu for Beta launch' }));
    expect(screen.getByRole('menu')).toBeTruthy();

    await fireEvent.keyDown(screen.getByRole('option', { name: /Beta launch/ }), { key: 'Escape' });
    expect(screen.queryByRole('menu')).toBeNull();
  });

  it('opens the mobile drawer state from the navigation button', async () => {
    const { container } = render(AppShell, {
      activeView: strings.nav.chat,
      navItems: [strings.nav.chat],
      user,
      conversations: [],
      selectedConversationId: '',
      ...callbacks
    });

    await fireEvent.click(screen.getByRole('button', { name: 'Open navigation' }));

    expect(container.querySelector('.workspace-shell.sidebar-open')).toBeTruthy();
  });
});

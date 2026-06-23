import { fireEvent, render, screen } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import WorkspaceWindow from './WorkspaceWindow.svelte';

describe('WorkspaceWindow', () => {
  it('closes with Escape and exposes titlebar actions', async () => {
    const onClose = vi.fn();
    const onMinimize = vi.fn();

    render(WorkspaceWindow, {
      id: 'settings',
      title: 'Settings',
      icon: 'gear',
      onClose,
      onMinimize
    });

    const dialog = screen.getByRole('dialog', { name: 'Settings' });
    await fireEvent.keyDown(dialog, { key: 'Escape' });
    await fireEvent.click(screen.getByRole('button', { name: 'Minimize Settings' }));

    expect(onClose).toHaveBeenCalledOnce();
    expect(onMinimize).toHaveBeenCalledOnce();
  });
});

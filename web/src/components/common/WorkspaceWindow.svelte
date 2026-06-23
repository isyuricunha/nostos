<script lang="ts">
  import { onMount } from 'svelte';
  import Icon from './Icon.svelte';
  import type { IconName } from './Icon.svelte';

  export let id: string;
  export let title: string;
  export let icon: IconName = 'window';
  export let width = 760;
  export let height = 620;
  export let onClose: () => void = () => undefined;
  export let onMinimize: () => void = () => undefined;
  export let onActivate: () => void = () => undefined;

  let dialog: HTMLElement;
  let dragging = false;
  let startX = 0;
  let startY = 0;
  let startLeft = 0;
  let startTop = 0;
  let position = loadPosition();
  let previousFocus: Element | null = null;

  $: style = `width:${width}px;height:${height}px;left:${position.left}px;top:${position.top}px;`;

  onMount(() => {
    previousFocus = document.activeElement;
    requestAnimationFrame(() => dialog?.focus());
    return () => {
      if (previousFocus instanceof HTMLElement) {
        previousFocus.focus();
      }
      stopDrag();
    };
  });

  function loadPosition(): { left: number; top: number } {
    const fallback = centerPosition(width, height);
    try {
      const raw = localStorage.getItem(`nostos-window:${id}`);
      if (!raw) return fallback;
      const parsed = JSON.parse(raw) as Partial<{ left: number; top: number }>;
      if (!Number.isFinite(parsed.left) || !Number.isFinite(parsed.top)) return fallback;
      return clampPosition(Number(parsed.left), Number(parsed.top), width, height);
    } catch {
      return fallback;
    }
  }

  function centerPosition(targetWidth: number, targetHeight: number): { left: number; top: number } {
    if (typeof window === 'undefined') return { left: 260, top: 80 };
    const leftOffset = workspaceLeftOffset();
    const availableWidth = Math.max(targetWidth, window.innerWidth - leftOffset);
    return clampPosition(
      Math.round(leftOffset + (availableWidth - targetWidth) / 2),
      Math.round((window.innerHeight - targetHeight) / 2),
      targetWidth,
      targetHeight
    );
  }

  function clampPosition(left: number, top: number, targetWidth: number, targetHeight: number): { left: number; top: number } {
    if (typeof window === 'undefined') return { left, top };
    const margin = 10;
    const leftOffset = workspaceLeftOffset();
    const minLeft = leftOffset + margin;
    const maxLeft = Math.max(minLeft, window.innerWidth - targetWidth - margin);
    const maxTop = Math.max(margin, window.innerHeight - targetHeight - margin);
    return {
      left: Math.max(minLeft, Math.min(left, maxLeft)),
      top: Math.max(margin, Math.min(top, maxTop))
    };
  }

  function workspaceLeftOffset(): number {
    if (typeof window === 'undefined' || window.innerWidth <= 980) return 0;
    const sidebar = document.querySelector<HTMLElement>('.workspace-sidebar');
    return sidebar?.getBoundingClientRect().width ?? 0;
  }

  function persistPosition(): void {
    try {
      localStorage.setItem(`nostos-window:${id}`, JSON.stringify(position));
    } catch {
      // Non-critical preference persistence.
    }
  }

  function startDrag(event: MouseEvent): void {
    if (window.innerWidth <= 760) return;
    const target = event.target;
    if (target instanceof Element && target.closest('button, input, select, textarea, a')) return;
    dragging = true;
    startX = event.clientX;
    startY = event.clientY;
    startLeft = position.left;
    startTop = position.top;
    onActivate();
    window.addEventListener('mousemove', drag);
    window.addEventListener('mouseup', stopDrag);
  }

  function drag(event: MouseEvent): void {
    if (!dragging) return;
    position = clampPosition(startLeft + event.clientX - startX, startTop + event.clientY - startY, width, height);
  }

  function stopDrag(): void {
    if (dragging) {
      persistPosition();
    }
    dragging = false;
    window.removeEventListener('mousemove', drag);
    window.removeEventListener('mouseup', stopDrag);
  }

  function focusableElements(): HTMLElement[] {
    if (!dialog) return [];
    return Array.from(
      dialog.querySelectorAll<HTMLElement>(
        'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'
      )
    ).filter((element) => !element.hasAttribute('disabled') && element.offsetParent !== null);
  }

  function handleKeydown(event: KeyboardEvent): void {
    if (event.key === 'Escape') {
      event.stopPropagation();
      onClose();
      return;
    }
    if (event.key !== 'Tab') return;
    const items = focusableElements();
    if (items.length === 0) {
      event.preventDefault();
      return;
    }
    const first = items[0];
    const last = items[items.length - 1];
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first.focus();
    }
  }
</script>

<div
  bind:this={dialog}
  aria-labelledby={`${id}-title`}
  aria-modal="true"
  class="workspace-window"
  data-window-id={id}
  on:keydown={handleKeydown}
  on:mousedown={onActivate}
  role="dialog"
  style={style}
  tabindex="-1"
>
  <header class="workspace-window-titlebar" on:mousedown={startDrag} role="presentation">
    <span class="window-title-icon"><Icon name={icon} size={15} /></span>
    <h2 id={`${id}-title`}>{title}</h2>
    <div class="window-title-actions">
      <button aria-label={`Minimize ${title}`} class="icon-only" on:click={onMinimize} type="button">
        <Icon name="minus" size={14} />
      </button>
      <button aria-label={`Close ${title}`} class="icon-only" on:click={onClose} type="button">
        <Icon name="close" size={14} />
      </button>
    </div>
  </header>
  <div class="workspace-window-body">
    <slot />
  </div>
</div>

<style>
  .workspace-window {
    position: fixed;
    z-index: var(--window-z, 80);
    display: grid;
    grid-template-rows: auto minmax(0, 1fr);
    max-width: calc(100vw - 20px);
    max-height: calc(100vh - 20px);
    overflow: hidden;
    border: 1px solid var(--workspace-border-strong);
    border-radius: 8px;
    background: var(--workspace-window);
    box-shadow: var(--workspace-window-shadow);
    color: var(--color-text);
  }

  .workspace-window:focus {
    outline: none;
  }

  .workspace-window-titlebar {
    display: flex;
    align-items: center;
    gap: 8px;
    min-height: 38px;
    border-bottom: 1px solid var(--workspace-border);
    background: var(--workspace-window-title);
    padding: 0 8px 0 11px;
    cursor: move;
    user-select: none;
  }

  .window-title-icon {
    color: var(--color-accent-strong);
  }

  .workspace-window-titlebar h2 {
    flex: 1 1 auto;
    min-width: 0;
    margin: 0;
    overflow: hidden;
    color: var(--color-text);
    font-size: 0.82rem;
    font-weight: 680;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .window-title-actions {
    display: flex;
    gap: 4px;
  }

  .icon-only {
    display: grid;
    width: 27px;
    height: 27px;
    place-items: center;
    padding: 0;
  }

  .workspace-window-body {
    min-height: 0;
    overflow: auto;
    padding: 0;
  }

  @media (max-width: 760px) {
    .workspace-window {
      inset: 0 !important;
      width: 100vw !important;
      height: 100dvh !important;
      max-width: none;
      max-height: none;
      border-width: 0;
      border-radius: 0;
    }

    .workspace-window-titlebar {
      min-height: 44px;
      cursor: default;
      padding-left: max(12px, env(safe-area-inset-left));
      padding-right: max(8px, env(safe-area-inset-right));
    }
  }
</style>

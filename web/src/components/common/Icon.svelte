<script lang="ts" module>
  export type IconName =
    | 'agent'
    | 'archive'
    | 'brain'
    | 'chat'
    | 'check'
    | 'chevron-down'
    | 'close'
    | 'copy'
    | 'details'
    | 'edit'
    | 'eye'
    | 'gear'
    | 'grid'
    | 'kebab'
    | 'menu'
    | 'minus'
    | 'model'
    | 'moon'
    | 'plus'
    | 'refresh'
    | 'search'
    | 'send'
    | 'spark'
    | 'stop'
    | 'tasks'
    | 'tools'
    | 'trash'
    | 'user'
    | 'window';
</script>

<script lang="ts">
  export let name: IconName = 'chat';
  export let size = 16;
  export let stroke = 1.8;
</script>

<svg
  aria-hidden="true"
  class="nostos-icon"
  fill="none"
  height={size}
  viewBox="0 0 24 24"
  width={size}
  stroke="currentColor"
  stroke-linecap="round"
  stroke-linejoin="round"
  stroke-width={stroke}
>
  {#if name === 'agent'}
    <path d="M8 19h8" />
    <path d="M9 15h6a4 4 0 0 0 4-4V8a4 4 0 0 0-4-4H9a4 4 0 0 0-4 4v3a4 4 0 0 0 4 4Z" />
    <path d="M9 9h.01M15 9h.01" />
    <path d="M10 12h4" />
  {:else if name === 'archive'}
    <path d="M4 8h16" />
    <path d="M6 8v10h12V8" />
    <path d="M8 4h8l2 4H6l2-4Z" />
    <path d="M10 12h4" />
  {:else if name === 'brain'}
    <path d="M12 5a3 3 0 0 0-5.7 1.3A4 4 0 0 0 5 14.6 3.5 3.5 0 0 0 12 17Z" />
    <path d="M12 5a3 3 0 0 1 5.7 1.3A4 4 0 0 1 19 14.6 3.5 3.5 0 0 1 12 17Z" />
    <path d="M12 8v9M9 11c1.2-.2 2.1-.9 3-2M15 11c-1.2-.2-2.1-.9-3-2" />
  {:else if name === 'chat'}
    <path d="M5 6.5A3.5 3.5 0 0 1 8.5 3h7A3.5 3.5 0 0 1 19 6.5v5A3.5 3.5 0 0 1 15.5 15H10l-5 4v-5.5A3.5 3.5 0 0 1 5 11.5Z" />
  {:else if name === 'check'}
    <path d="m5 12 4 4L19 6" />
  {:else if name === 'chevron-down'}
    <path d="m7 9 5 5 5-5" />
  {:else if name === 'close'}
    <path d="M7 7l10 10M17 7 7 17" />
  {:else if name === 'copy'}
    <path d="M8 8h10v10H8z" />
    <path d="M6 14H4V4h10v2" />
  {:else if name === 'details'}
    <path d="M12 5h.01M12 12h.01M12 19h.01" />
  {:else if name === 'edit'}
    <path d="M5 19h4l10-10a2.5 2.5 0 0 0-4-4L5 15v4Z" />
    <path d="m13.5 6.5 4 4" />
  {:else if name === 'eye'}
    <path d="M3 12s3.5-6 9-6 9 6 9 6-3.5 6-9 6-9-6-9-6Z" />
    <path d="M12 10a2 2 0 1 1 0 4 2 2 0 0 1 0-4Z" />
  {:else if name === 'gear'}
    <path d="M12 8.5a3.5 3.5 0 1 1 0 7 3.5 3.5 0 0 1 0-7Z" />
    <path d="M4.8 10.2 3.5 8l2-3.4 2.5.8M19.2 13.8l1.3 2.2-2 3.4-2.5-.8M8.3 4.5 9.5 2h5l1.2 2.5M15.7 19.5 14.5 22h-5l-1.2-2.5M4.8 13.8 3.5 16l2 3.4 2.5-.8M19.2 10.2 20.5 8l-2-3.4-2.5.8" />
  {:else if name === 'grid'}
    <path d="M4 4h6v6H4zM14 4h6v6h-6zM4 14h6v6H4zM14 14h6v6h-6z" />
  {:else if name === 'kebab'}
    <path d="M12 5h.01M12 12h.01M12 19h.01" />
  {:else if name === 'menu'}
    <path d="M4 7h16M4 12h16M4 17h16" />
  {:else if name === 'minus'}
    <path d="M6 12h12" />
  {:else if name === 'model'}
    <path d="M12 3 4 7v10l8 4 8-4V7l-8-4Z" />
    <path d="M4 7l8 4 8-4M12 11v10" />
  {:else if name === 'moon'}
    <path d="M18 16.5A8 8 0 0 1 8.5 6 8 8 0 1 0 18 16.5Z" />
  {:else if name === 'plus'}
    <path d="M12 5v14M5 12h14" />
  {:else if name === 'refresh'}
    <path d="M19 7v5h-5" />
    <path d="M5 17v-5h5" />
    <path d="M18 12a6 6 0 0 0-10.4-4M6 12a6 6 0 0 0 10.4 4" />
  {:else if name === 'search'}
    <path d="M10.5 18a7.5 7.5 0 1 1 0-15 7.5 7.5 0 0 1 0 15Z" />
    <path d="m16 16 5 5" />
  {:else if name === 'send'}
    <path d="M4 12 20 4l-5 16-3-7-8-1Z" />
    <path d="m12 13 8-9" />
  {:else if name === 'spark'}
    <path d="M12 3 14 9l6 3-6 3-2 6-2-6-6-3 6-3 2-6Z" />
  {:else if name === 'stop'}
    <path d="M7 7h10v10H7z" />
  {:else if name === 'tasks'}
    <path d="M6 4h12a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2Z" />
    <path d="M8 9h8M8 14h3M14 15l1.5 1.5L19 13" />
  {:else if name === 'tools'}
    <path d="m14 6 4-4 4 4-4 4-4-4Z" />
    <path d="M4 20 13.5 10.5" />
    <path d="m6 14 4 4" />
  {:else if name === 'trash'}
    <path d="M5 7h14" />
    <path d="M9 7V5h6v2" />
    <path d="m7 7 1 13h8l1-13" />
    <path d="M10 11v5M14 11v5" />
  {:else if name === 'user'}
    <path d="M12 12a4 4 0 1 0 0-8 4 4 0 0 0 0 8Z" />
    <path d="M4 21a8 8 0 0 1 16 0" />
  {:else if name === 'window'}
    <path d="M4 5h16v14H4z" />
    <path d="M4 9h16" />
  {/if}
</svg>

<style>
  .nostos-icon {
    display: block;
    flex: 0 0 auto;
  }
</style>

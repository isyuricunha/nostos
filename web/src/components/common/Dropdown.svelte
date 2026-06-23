<script lang="ts">
  export let label: string;
  export let open = false;

  function closeAfterAction(event: MouseEvent): void {
    const target = event.target;
    if (target instanceof Element && target.closest('button')) {
      open = false;
    }
  }

  function handlePanelKeydown(event: KeyboardEvent): void {
    if (event.key === 'Escape') {
      open = false;
    }
  }
</script>

<div class="ui-dropdown">
  <button aria-expanded={open} on:click={() => (open = !open)} type="button">{label}</button>
  {#if open}
    <div class="ui-dropdown-panel" on:click={closeAfterAction} on:keydown={handlePanelKeydown} role="menu" tabindex="-1">
      <slot />
    </div>
  {/if}
</div>

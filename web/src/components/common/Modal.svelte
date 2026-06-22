<script lang="ts">
  export let title: string;
  export let open = false;
  export let onClose: () => void = () => undefined;
</script>

{#if open}
  <div class="ui-modal-layer">
    <button aria-label="Close modal" class="ui-modal-backdrop" on:click={onClose} type="button"></button>
    <div aria-labelledby="modal-title" aria-modal="true" class="ui-modal" role="dialog" tabindex="-1">
      <header>
        <h2 id="modal-title">{title}</h2>
        <button aria-label="Close" on:click={onClose} type="button">Close</button>
      </header>
      <slot />
    </div>
  </div>
{/if}

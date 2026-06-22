<script lang="ts">
  export let label: string;
  export let open = false;
</script>

<div class="ui-dropdown">
  <button aria-expanded={open} on:click={() => (open = !open)} type="button">{label}</button>
  {#if open}
    <div class="ui-dropdown-panel">
      <slot />
    </div>
  {/if}
</div>

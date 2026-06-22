<script lang="ts">
  export let label: string;
  export let active = false;
  export let icon = '';
  export let onSelect: () => void = () => undefined;
</script>

<button class:active class="sidebar-item" on:click={onSelect} type="button">
  <span class="sidebar-item-icon" aria-hidden="true">{icon || label.slice(0, 1)}</span>
  <span>{label}</span>
</button>

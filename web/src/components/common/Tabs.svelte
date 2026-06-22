<script lang="ts">
  type TabItem = {
    id: string;
    label: string;
  };

  export let items: TabItem[] = [];
  export let active = '';
  export let onSelect: (id: string) => void = () => undefined;
</script>

<div class="ui-tabs" role="tablist">
  {#each items as item (item.id)}
    <button
      aria-selected={active === item.id}
      class:active={active === item.id}
      on:click={() => onSelect(item.id)}
      role="tab"
      type="button"
    >
      {item.label}
    </button>
  {/each}
</div>

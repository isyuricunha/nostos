<script lang="ts">
  type SelectOption = {
    value: string;
    label: string;
  };

  export let label: string;
  export let value = '';
  export let options: SelectOption[] = [];
  export let hint = '';
</script>

<label class="field">
  <span>{label}</span>
  <select bind:value>
    {#each options as option (option.value)}
      <option value={option.value}>{option.label}</option>
    {/each}
  </select>
  {#if hint}
    <small>{hint}</small>
  {/if}
</label>

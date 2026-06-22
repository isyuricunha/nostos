<script lang="ts">
  export let label: string;
  export let checked = false;
  export let hint = '';
</script>

<label class="inline-check">
  <input bind:checked type="checkbox" />
  <span>
    {label}
    {#if hint}
      <small>{hint}</small>
    {/if}
  </span>
</label>

<script lang="ts">
  export let label: string;
  export let value = '';
  export let placeholder = '';
  export let required = false;
  export let rows = 5;
  export let hint = '';
</script>

<label class="field">
  <span>{label}</span>
  <textarea bind:value {placeholder} {required} {rows}></textarea>
  {#if hint}
    <small>{hint}</small>
  {/if}
</label>

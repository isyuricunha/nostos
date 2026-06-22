<script lang="ts">
  export let label: string;
  export let value = '';
  export let type = 'text';
  export let placeholder = '';
  export let required = false;
  export let autocomplete: 'off' | 'email' | 'name' | 'new-password' | 'current-password' | '' = '';
  export let hint = '';
</script>

<label class="field">
  <span>{label}</span>
  <input autocomplete={autocomplete || undefined} bind:value {placeholder} {required} {type} />
  {#if hint}
    <small>{hint}</small>
  {/if}
</label>

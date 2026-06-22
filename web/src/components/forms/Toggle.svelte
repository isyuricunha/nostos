<script lang="ts">
  export let label: string;
  export let checked = false;
</script>

<label class="toggle-field">
  <span>{label}</span>
  <input bind:checked type="checkbox" />
  <i aria-hidden="true"></i>
</label>

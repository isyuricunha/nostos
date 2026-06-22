<script lang="ts">
  export let tone: 'success' | 'error' | 'warning' | 'info' = 'info';
  export let role: 'status' | 'alert' = tone === 'error' ? 'alert' : 'status';
</script>

<div class={`notice ${tone}`} {role}>
  <slot />
</div>

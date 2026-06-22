<script lang="ts">
  export let tone: 'neutral' | 'accent' | 'success' | 'warning' | 'danger' = 'neutral';
</script>

<span class={`ui-badge ${tone}`}>
  <slot />
</span>

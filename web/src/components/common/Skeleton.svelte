<script lang="ts">
  export let label = 'Loading';
  export let lines = 1;
</script>

<div aria-label={label} class="skeleton-stack">
  {#each Array.from({ length: lines }, (_line, index) => index) as index (index)}
    <span class="skeleton"></span>
  {/each}
</div>

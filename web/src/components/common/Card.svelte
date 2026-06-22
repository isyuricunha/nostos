<script lang="ts">
  export let elevated = false;
  export let interactive = false;
</script>

<section class:elevated class:interactive class="ui-card">
  <slot />
</section>

<script lang="ts">
  export let elevated = false;
  export let interactive = false;
</script>

<section class="ui-card" class:elevated class:interactive>
  <slot />
</section>

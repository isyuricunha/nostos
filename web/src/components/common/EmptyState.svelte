<script lang="ts">
  export let title: string;
  export let description = '';
</script>

<div class="empty-state">
  <span aria-hidden="true"></span>
  <strong>{title}</strong>
  {#if description}
    <p>{description}</p>
  {/if}
  <slot />
</div>

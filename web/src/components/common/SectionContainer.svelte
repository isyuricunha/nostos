<script lang="ts">
  export let title: string;
  export let subtitle = '';
</script>

<section class="ui-section">
  <header class="ui-section-header">
    <div>
      <p class="ui-kicker">Workspace</p>
      <h2>{title}</h2>
      {#if subtitle}
        <p>{subtitle}</p>
      {/if}
    </div>
    <div class="ui-section-actions">
      <slot name="actions" />
    </div>
  </header>
  <slot />
</section>

<script lang="ts">
  export let label: string;
  export let type: 'button' | 'submit' | 'reset' = 'button';
  export let disabled = false;
  export let active = false;
  export let onClick: ((event: MouseEvent) => void | Promise<void>) | undefined = undefined;
</script>

<button
  aria-label={label}
  class:active
  class="ui-icon-button"
  {disabled}
  on:click={(event) => onClick?.(event)}
  {type}
>
  <slot />
</button>

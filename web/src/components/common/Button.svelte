<script lang="ts">
  export let type: 'button' | 'submit' | 'reset' = 'button';
  export let variant: 'primary' | 'secondary' | 'ghost' | 'danger' = 'secondary';
  export let size: 'sm' | 'md' | 'lg' = 'md';
  export let disabled = false;
  export let pressed = false;
  export let onClick: ((event: MouseEvent) => void | Promise<void>) | undefined = undefined;
</script>

<button
  aria-pressed={pressed || undefined}
  class={`ui-button ${variant} ${size}`}
  {disabled}
  on:click={(event) => onClick?.(event)}
  {type}
>
  <slot />
</button>

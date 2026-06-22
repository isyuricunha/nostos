<script lang="ts">
  export let status: string;
  export let tone: 'neutral' | 'success' | 'warning' | 'danger' | 'accent' = 'neutral';
</script>

<span class={`status-pill ${tone}`}>
  <span aria-hidden="true"></span>
  {status}
</span>

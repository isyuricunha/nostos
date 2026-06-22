<script lang="ts">
  import Button from '../components/common/Button.svelte';
  import Notice from '../components/common/Notice.svelte';
  import Skeleton from '../components/common/Skeleton.svelte';
  import { strings } from '../strings';

  export let mode: 'loading' | 'setup' | 'login';
  export let setupEmail = '';
  export let setupDisplayName = '';
  export let setupPassword = '';
  export let setupConfirmPassword = '';
  export let loginEmail = '';
  export let loginPassword = '';
  export let submitting = false;
  export let notice = '';
  export let errorMessage = '';
  export let onSetup: () => void | Promise<void>;
  export let onLogin: () => void | Promise<void>;
</script>

<main class="auth-screen">
  <section class="auth-panel" aria-label={mode === 'setup' ? strings.auth.setupTitle : strings.auth.loginTitle}>
    <div class="auth-brand">
      <span class="brand-mark" aria-hidden="true">N</span>
      <div>
        <strong>{strings.appName}</strong>
        <small>Self-hosted AI workspace</small>
      </div>
    </div>

    {#if mode === 'loading'}
      <Skeleton label="Loading application state" lines={4} />
    {:else if mode === 'setup'}
      <form class="auth-form" on:submit|preventDefault={onSetup}>
        <div>
          <p class="eyebrow">First run</p>
          <h1>{strings.auth.setupTitle}</h1>
          <p>{strings.auth.setupSubtitle}</p>
        </div>
        <label>
          {strings.auth.email}
          <input bind:value={setupEmail} autocomplete="email" required type="email" />
        </label>
        <label>
          {strings.auth.displayName}
          <input bind:value={setupDisplayName} autocomplete="name" />
        </label>
        <label>
          {strings.auth.password}
          <input bind:value={setupPassword} autocomplete="new-password" minlength="12" required type="password" />
        </label>
        <label>
          {strings.auth.confirmPassword}
          <input
            bind:value={setupConfirmPassword}
            autocomplete="new-password"
            minlength="12"
            required
            type="password"
          />
        </label>
        {#if errorMessage}
          <Notice tone="error">{errorMessage}</Notice>
        {/if}
        <Button disabled={submitting} type="submit" variant="primary">
          {submitting ? 'Creating...' : strings.auth.createOwner}
        </Button>
      </form>
    {:else}
      <form class="auth-form" on:submit|preventDefault={onLogin}>
        <div>
          <p class="eyebrow">Owner access</p>
          <h1>{strings.auth.loginTitle}</h1>
          <p>{strings.auth.loginSubtitle}</p>
        </div>
        <label>
          {strings.auth.email}
          <input bind:value={loginEmail} autocomplete="email" required type="email" />
        </label>
        <label>
          {strings.auth.password}
          <input bind:value={loginPassword} autocomplete="current-password" required type="password" />
        </label>
        {#if notice}
          <Notice tone="success">{notice}</Notice>
        {/if}
        {#if errorMessage}
          <Notice tone="error">{errorMessage}</Notice>
        {/if}
        <Button disabled={submitting} type="submit" variant="primary">
          {submitting ? 'Signing in...' : strings.auth.signIn}
        </Button>
      </form>
    {/if}
  </section>
</main>

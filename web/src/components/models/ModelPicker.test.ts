import { fireEvent, render, screen, within } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import ModelPicker from './ModelPicker.svelte';
import type { Provider, ProviderModel } from '../../lib/types';

const providers: Provider[] = [
  {
    id: 'provider_bifrost',
    name: 'Bifrost',
    base_url: 'http://localhost:8080',
    enabled: true,
    request_timeout_ms: 60000,
    custom_headers: {},
    health_status: 'healthy'
  },
  {
    id: 'provider_direct',
    name: 'Direct NVIDIA',
    base_url: 'http://localhost:8081',
    enabled: true,
    request_timeout_ms: 60000,
    custom_headers: {},
    health_status: 'unknown'
  }
];

const models: ProviderModel[] = [
  {
    id: 'model_1',
    workspace_id: 'workspace_1',
    provider_id: 'provider_bifrost',
    provider_name: 'Bifrost',
    model_id: 'NVIDIA NIM/openai/gpt-oss-120b',
    display_name: 'GPT OSS 120B',
    enabled: true,
    available: true,
    manually_added: false,
    capabilities: ['chat', 'tools']
  },
  {
    id: 'model_2',
    workspace_id: 'workspace_1',
    provider_id: 'provider_bifrost',
    provider_name: 'Bifrost',
    model_id: 'NVIDIA NIM/moonshotai/kimi-k2.6',
    display_name: 'Kimi K2.6',
    enabled: true,
    available: false,
    manually_added: false,
    capabilities: ['chat']
  },
  {
    id: 'model_3',
    workspace_id: 'workspace_1',
    provider_id: 'provider_direct',
    provider_name: 'Direct NVIDIA',
    model_id: 'openai/gpt-oss-120b',
    display_name: 'GPT OSS Direct',
    enabled: true,
    available: true,
    manually_added: true,
    capabilities: ['chat']
  }
];

describe('ModelPicker', () => {
  it('searches cached full model IDs grouped by provider', async () => {
    render(ModelPicker, { label: 'Chat model', providers, models, role: 'chat' });

    await fireEvent.click(screen.getByRole('button', { name: /select model/i }));
    await fireEvent.input(screen.getByPlaceholderText('Search provider, model, capability'), {
      target: { value: 'kimi' }
    });

    const dialog = screen.getByRole('dialog', { name: 'Chat model picker' });
    expect(within(dialog).getByText('Bifrost')).toBeTruthy();
    expect(within(dialog).getByText('Kimi K2.6')).toBeTruthy();
    expect(within(dialog).getByText('NVIDIA NIM/moonshotai/kimi-k2.6')).toBeTruthy();
    expect(within(dialog).getByText(/unavailable/)).toBeTruthy();
  });

  it('accepts a manual full model ID', async () => {
    render(ModelPicker, { label: 'Utility model', providers, models, role: 'utility' });

    await fireEvent.click(screen.getByRole('button', { name: /select model/i }));
    await fireEvent.input(screen.getByPlaceholderText('Type an unlisted full model ID'), {
      target: { value: 'Bifrost/custom/manual-model' }
    });
    await fireEvent.click(screen.getByRole('button', { name: 'Use ID' }));

    expect(screen.getByRole('button', { name: new RegExp('Bifrost/custom/manual-model') })).toBeTruthy();
  });
});

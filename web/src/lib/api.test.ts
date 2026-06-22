import { describe, expect, it, vi } from 'vitest';
import { getJSON } from './api';

describe('getJSON', () => {
  it('returns parsed JSON for successful responses', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => new Response(JSON.stringify({ ok: true }), { status: 200 }))
    );

    await expect(getJSON<{ ok: boolean }>('/api/v1/health')).resolves.toEqual({ ok: true });
  });

  it('uses API error messages for failed responses', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(
        async () =>
          new Response(JSON.stringify({ error: { code: 'bad_request', message: 'Bad request.' } }), {
            status: 400
          })
      )
    );

    await expect(getJSON('/api/v1/missing')).rejects.toThrow('Bad request.');
  });
});

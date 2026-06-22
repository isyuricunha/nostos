import { describe, expect, it, vi } from 'vitest';
import { deleteJSON, getJSON, postJSON } from './api';

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

  it('sends CSRF headers for unsafe methods', async () => {
    Object.defineProperty(document, 'cookie', {
      value: 'nostos_csrf=token-123',
      writable: true
    });
    const fetchMock = vi.fn(async () => new Response(JSON.stringify({ ok: true }), { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);

    await postJSON('/api/v1/auth/logout');
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/auth/logout',
      expect.objectContaining({
        method: 'POST',
        headers: expect.objectContaining({ 'X-CSRF-Token': 'token-123' })
      })
    );
  });

  it('supports DELETE requests', async () => {
    const fetchMock = vi.fn(async () => new Response(JSON.stringify({ ok: true }), { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);

    await deleteJSON('/api/v1/sessions/session-id');
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/sessions/session-id',
      expect.objectContaining({ method: 'DELETE' })
    );
  });
});

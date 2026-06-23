export type APIError = {
  error: {
    code: string;
    message: string;
    request_id?: string;
    details?: Record<string, unknown>;
  };
};

export async function getJSON<T>(path: string): Promise<T> {
  return requestJSON<T>('GET', path);
}

export async function postJSON<T>(path: string, body?: unknown): Promise<T> {
  return requestJSON<T>('POST', path, body);
}

export async function putJSON<T>(path: string, body?: unknown): Promise<T> {
  return requestJSON<T>('PUT', path, body);
}

export async function deleteJSON<T>(path: string): Promise<T> {
  return requestJSON<T>('DELETE', path);
}

export type StreamEventHandler = (event: string, payload: unknown) => void;

export async function postStream(
  path: string,
  body: unknown,
  onEvent: StreamEventHandler
): Promise<void> {
  return requestStream('POST', path, body, onEvent);
}

export async function patchStream(
  path: string,
  body: unknown,
  onEvent: StreamEventHandler
): Promise<void> {
  return requestStream('PATCH', path, body, onEvent);
}

async function requestStream(
  method: 'POST' | 'PATCH',
  path: string,
  body: unknown,
  onEvent: StreamEventHandler
): Promise<void> {
  const csrfToken = readCookie('nostos_csrf');
  const response = await fetch(path, {
    method,
    credentials: 'include',
    headers: {
      Accept: 'text/event-stream',
      'Content-Type': 'application/json',
      ...(csrfToken ? { 'X-CSRF-Token': csrfToken } : {})
    },
    body: JSON.stringify(body)
  });
  if (!response.ok || !response.body) {
    const responseBody = (await response.json()) as APIError;
    throw new Error(responseBody.error?.message ?? 'Streaming request failed.');
  }
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';
  while (true) {
    const { done, value } = await reader.read();
    if (done) {
      break;
    }
    buffer += decoder.decode(value, { stream: true });
    const parts = buffer.split('\n\n');
    buffer = parts.pop() ?? '';
    for (const part of parts) {
      const eventLine = part
        .split('\n')
        .find((line) => line.startsWith('event:'))
        ?.replace('event:', '')
        .trim();
      const dataLine = part
        .split('\n')
        .find((line) => line.startsWith('data:'))
        ?.replace('data:', '')
        .trim();
      if (!eventLine || !dataLine) {
        continue;
      }
      onEvent(eventLine, JSON.parse(dataLine) as unknown);
    }
  }
}

async function requestJSON<T>(method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = {
    Accept: 'application/json'
  };
  if (body !== undefined) {
    headers['Content-Type'] = 'application/json';
  }
  if (method !== 'GET' && method !== 'HEAD') {
    const csrfToken = readCookie('nostos_csrf');
    if (csrfToken) {
      headers['X-CSRF-Token'] = csrfToken;
    }
  }
  const response = await fetch(path, {
    method,
    credentials: 'include',
    headers,
    body: body === undefined ? undefined : JSON.stringify(body)
  });
  const responseBody = (await response.json()) as T | APIError;
  if (!response.ok) {
    const apiError = responseBody as APIError;
    throw new Error(apiError.error?.message ?? 'Request failed.');
  }
  return responseBody as T;
}

function readCookie(name: string): string {
  const prefix = `${name}=`;
  const values = document.cookie.split(';').map((value) => value.trim());
  const match = values.find((value) => value.startsWith(prefix));
  return match ? decodeURIComponent(match.slice(prefix.length)) : '';
}

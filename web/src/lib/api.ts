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

export async function deleteJSON<T>(path: string): Promise<T> {
  return requestJSON<T>('DELETE', path);
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

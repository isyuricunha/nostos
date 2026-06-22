export type APIError = {
  error: {
    code: string;
    message: string;
    request_id?: string;
    details?: Record<string, unknown>;
  };
};

export async function getJSON<T>(path: string): Promise<T> {
  const response = await fetch(path, {
    credentials: 'include',
    headers: {
      Accept: 'application/json'
    }
  });
  const body = (await response.json()) as T | APIError;
  if (!response.ok) {
    const apiError = body as APIError;
    throw new Error(apiError.error?.message ?? 'Request failed.');
  }
  return body as T;
}

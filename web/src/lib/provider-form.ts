export function parseHeaderText(value: string): Record<string, string> {
  const trimmed = value.trim();
  if (!trimmed) {
    return {};
  }
  const parsed: unknown = JSON.parse(trimmed);
  if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
    throw new Error('Custom headers must be a JSON object.');
  }
  const headers: Record<string, string> = {};
  for (const [key, headerValue] of Object.entries(parsed)) {
    const normalizedKey = key.trim();
    if (!normalizedKey) {
      throw new Error('Custom header names cannot be empty.');
    }
    if (typeof headerValue !== 'string') {
      throw new Error('Custom header values must be strings.');
    }
    headers[normalizedKey] = headerValue.trim();
  }
  return headers;
}

export function formatHeaderText(headers: Record<string, string> | undefined): string {
  if (!headers || Object.keys(headers).length === 0) {
    return '';
  }
  return JSON.stringify(headers, null, 2);
}

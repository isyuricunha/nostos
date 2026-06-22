import { describe, expect, it } from 'vitest';
import { formatHeaderText, parseHeaderText } from './provider-form';

describe('provider form helpers', () => {
  it('parses custom header JSON into string headers', () => {
    expect(parseHeaderText('{ "X-Workspace": "nostos" }')).toEqual({ 'X-Workspace': 'nostos' });
  });

  it('rejects non-object custom header input', () => {
    expect(() => parseHeaderText('[]')).toThrow('Custom headers must be a JSON object.');
  });

  it('rejects non-string header values', () => {
    expect(() => parseHeaderText('{ "X-Retry": 3 }')).toThrow('Custom header values must be strings.');
  });

  it('formats headers for editing', () => {
    expect(formatHeaderText({ 'X-Workspace': 'nostos' })).toContain('"X-Workspace": "nostos"');
  });
});

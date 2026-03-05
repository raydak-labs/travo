import { describe, it, expect } from 'vitest';
import { API_VERSION, type HealthResponse, isHealthResponse } from '../index';

describe('shared package', () => {
  it('exports API_VERSION', () => {
    expect(API_VERSION).toBeDefined();
    expect(typeof API_VERSION).toBe('string');
    expect(API_VERSION).toBe('v1');
  });

  it('exports HealthResponse type guard', () => {
    expect(typeof isHealthResponse).toBe('function');
  });

  it('validates a correct HealthResponse', () => {
    const valid: HealthResponse = { status: 'ok' };
    expect(isHealthResponse(valid)).toBe(true);
  });

  it('rejects an invalid HealthResponse', () => {
    expect(isHealthResponse({})).toBe(false);
    expect(isHealthResponse({ status: 'bad' })).toBe(false);
    expect(isHealthResponse(null)).toBe(false);
    expect(isHealthResponse('string')).toBe(false);
  });
});

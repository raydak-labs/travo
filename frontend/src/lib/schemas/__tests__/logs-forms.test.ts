import { describe, it, expect } from 'vitest';
import { logsFilterFormSchema } from '../logs-forms';

describe('logsFilterFormSchema', () => {
  it('accepts default-shaped client filters', () => {
    const r = logsFilterFormSchema.safeParse({
      lineFilter: '',
      serviceFilter: '',
      levelFilter: '',
      customService: '',
      showCustomInput: false,
    });
    expect(r.success).toBe(true);
  });

  it('accepts arbitrary string filters and custom mode', () => {
    const r = logsFilterFormSchema.safeParse({
      lineFilter: 'foo',
      serviceFilter: 'dnsmasq',
      levelFilter: 'err',
      customService: 'mysvc',
      showCustomInput: true,
    });
    expect(r.success).toBe(true);
  });
});

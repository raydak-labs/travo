import { describe, it, expect } from 'vitest';
import { clientAliasFormSchema } from '../clients-forms';

describe('clientAliasFormSchema', () => {
  it('accepts empty alias', () => {
    const r = clientAliasFormSchema.safeParse({ alias: '' });
    expect(r.success).toBe(true);
  });

  it('rejects overly long alias', () => {
    const r = clientAliasFormSchema.safeParse({ alias: 'x'.repeat(129) });
    expect(r.success).toBe(false);
  });
});

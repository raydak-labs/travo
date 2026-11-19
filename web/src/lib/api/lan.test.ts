import { describe, expect, it } from 'vitest';
import { adoptedTxn, LOST_ANSWER_SECONDS, lanRefusal, moveOutcome, moveURL } from './lan';

describe('moveURL', () => {
  it('keeps scheme and port and opens the Network page', () => {
    expect(moveURL('192.168.8.1/24', { protocol: 'http:', port: '' })).toBe(
      'http://192.168.8.1/#/network',
    );
    expect(moveURL('10.0.0.1/30', { protocol: 'https:', port: '8443' })).toBe(
      'https://10.0.0.1:8443/#/network',
    );
  });
});

describe('moveOutcome', () => {
  const txn = {
    id: '0123456789abcdef',
    configs: ['network'],
    started: '',
    summary: '',
    window_seconds: 180,
  };
  it('sends the user to the new address when the answer is lost or pending', () => {
    expect(moveOutcome('lost')).toEqual({ kind: 'moving', seconds: LOST_ANSWER_SECONDS });
    expect(moveOutcome({ data: { ...txn, state: 'pending' } })).toEqual({
      kind: 'moving',
      seconds: 180,
    });
  });
  it('reports a rollback and maps refusals', () => {
    expect(moveOutcome({ data: { ...txn, state: 'rolled_back' } })).toEqual({
      kind: 'rolled_back',
    });
    expect(moveOutcome({ error: { code: 'lan_subnet_overlap' } })).toMatchObject({
      kind: 'refused',
      error: expect.stringContaining('overlaps'),
    });
    expect(lanRefusal('lan_no_change').message).toBeTruthy();
    expect(lanRefusal('nope').error).toBeTruthy();
  });
});

describe('adoptedTxn', () => {
  it('turns the pending change into a pending transaction with the remaining window', () => {
    expect(
      adoptedTxn({ id: '0123456789abcdef', address: '192.168.8.1/24', remaining_seconds: 97 }),
    ).toMatchObject({ id: '0123456789abcdef', state: 'pending', window_seconds: 97 });
  });
});

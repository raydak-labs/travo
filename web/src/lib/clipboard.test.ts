import { describe, expect, it, vi } from 'vitest';
import { copyText } from './clipboard';

describe('copyText', () => {
  it('uses the clipboard API when present', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    const selectAndCopy = vi.fn();
    expect(await copyText('tgt_x', { clipboard: { writeText }, selectAndCopy })).toBe(true);
    expect(writeText).toHaveBeenCalledWith('tgt_x');
    expect(selectAndCopy).not.toHaveBeenCalled();
  });

  it('falls back to the selection on plain http (no clipboard API)', async () => {
    expect(await copyText('tgt_x', { selectAndCopy: () => true })).toBe(true);
  });

  it('falls back when the clipboard API refuses', async () => {
    const writeText = vi.fn().mockRejectedValue(new Error('denied'));
    expect(await copyText('tgt_x', { clipboard: { writeText }, selectAndCopy: () => true })).toBe(
      true,
    );
  });

  it('reports failure when both paths fail', async () => {
    expect(await copyText('tgt_x', { selectAndCopy: () => false })).toBe(false);
    expect(
      await copyText('tgt_x', {
        selectAndCopy: () => {
          throw new Error('no document');
        },
      }),
    ).toBe(false);
  });
});

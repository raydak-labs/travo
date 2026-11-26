import { describe, expect, it } from 'vitest';
import { boardMatches, firmwareError, phraseMatches } from './firmware';
import type { FirmwareImage } from './gen';

const image = (tests: Record<string, boolean>): FirmwareImage => ({
  board: 'glinet,gl-axt1800',
  model: 'GL.iNet GL-AXT1800',
  sha256: 'a'.repeat(64),
  size: 1,
  packages_known: true,
  dropped_packages: [],
  validation: { valid: true, forceable: true, allow_backup: true, tests },
});

describe('firmware', () => {
  it('needs the exact phrase', () => {
    expect(phraseMatches('upgrade firmware')).toBe(true);
    expect(phraseMatches('Upgrade firmware')).toBe(false);
    expect(phraseMatches('upgrade firmware ')).toBe(false);
  });

  it('reads the board from fwtool_device_match only', () => {
    expect(boardMatches(image({ fwtool_device_match: true, fwtool_signature: true }))).toBe(true);
    expect(boardMatches(image({ fwtool_device_match: false }))).toBe(false);
    expect(boardMatches(image({}))).toBe(false);
  });

  it('maps codes, never text', () => {
    expect(firmwareError('firmware_no_room')).toMatch(/free memory/);
    expect(firmwareError('operation_in_progress')).toMatch(/in progress/);
    expect(firmwareError('session_required')).toMatch(/browser session/);
    expect(firmwareError(undefined)).toMatch(/failed/);
  });
});

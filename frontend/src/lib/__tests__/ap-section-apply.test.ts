import { describe, expect, it, vi } from 'vitest';
import type { APConfig } from '@shared/index';
import {
  ApApplyRollbackError,
  apBandLabel,
  describeApApplyRollback,
  describeApSection,
  rollbackApSections,
  snapshotApSections,
} from '../ap-section-apply';

const AP_2G: APConfig = {
  radio: 'radio0',
  band: '2g',
  ssid: 'OpenWrt-Travel',
  encryption: 'psk2',
  key: 'travel12345',
  enabled: true,
  channel: 6,
  section: 'default_radio0',
};

const AP_5G: APConfig = {
  ...AP_2G,
  radio: 'radio1',
  band: '5g',
  channel: 36,
  section: 'default_radio1',
};

describe('ap-section-apply', () => {
  it('labels bands and sections for failure messages', () => {
    expect(apBandLabel('2g')).toBe('2.4 GHz');
    expect(apBandLabel('5g')).toBe('5 GHz');
    expect(apBandLabel('weird')).toBe('weird');
    expect(describeApSection(AP_5G)).toBe('5 GHz radio1 (default_radio1)');
  });

  it('snapshots the credentials of every section before the write loop', () => {
    const snapshots = snapshotApSections([AP_2G, AP_5G]);
    expect(snapshots).toEqual([
      {
        section: 'default_radio0',
        radio: 'radio0',
        band: '2g',
        config: {
          ssid: 'OpenWrt-Travel',
          encryption: 'psk2',
          key: 'travel12345',
          enabled: true,
        },
      },
      {
        section: 'default_radio1',
        radio: 'radio1',
        band: '5g',
        config: {
          ssid: 'OpenWrt-Travel',
          encryption: 'psk2',
          key: 'travel12345',
          enabled: true,
        },
      },
    ]);
  });

  it('restores the written sections newest first', async () => {
    const apply = vi.fn().mockResolvedValue(undefined);
    const result = await rollbackApSections(snapshotApSections([AP_2G, AP_5G]), apply);

    expect(apply.mock.calls.map(([snap]) => snap.section)).toEqual([
      'default_radio1',
      'default_radio0',
    ]);
    expect(result).toEqual({ restored: ['default_radio1', 'default_radio0'], failed: [] });
  });

  it('records a failed restore instead of masking the original failure', async () => {
    const apply = vi.fn(async (snap: { section: string }) => {
      if (snap.section === 'default_radio1') throw new Error('device busy');
    });
    const result = await rollbackApSections(snapshotApSections([AP_2G, AP_5G]), apply);
    expect(result).toEqual({ restored: ['default_radio0'], failed: ['default_radio1'] });
  });

  it('reports the failed section and what was rolled back', () => {
    const error = new ApApplyRollbackError(AP_5G, new Error('uci commit failed'), {
      restored: ['default_radio0'],
      failed: [],
    });
    const message = describeApApplyRollback(error);
    expect(message).toContain('Failed to save 5 GHz radio1 (default_radio1)');
    expect(message).toContain('uci commit failed');
    expect(message).toContain('Rolled back to the previous settings on default_radio0');
  });

  it('names the sections it could not roll back', () => {
    const error = new ApApplyRollbackError(AP_5G, new Error('boom'), {
      restored: [],
      failed: ['default_radio0'],
    });
    expect(describeApApplyRollback(error)).toContain(
      'Could not roll back default_radio0 — it still has the new settings.',
    );
  });

  it('passes plain errors through unchanged', () => {
    expect(describeApApplyRollback(new Error('network down'))).toBe('network down');
  });
});

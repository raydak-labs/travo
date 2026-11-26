import { describe, expect, it } from 'vitest';
import { attendedSysupgradeURL, openWrtCheckError } from './openwrt-updates';

describe('openwrt updates', () => {
  it('links LuCI attended sysupgrade on the moved LuCI port', () => {
    expect(attendedSysupgradeURL({ protocol: 'http:', hostname: '192.168.1.1' })).toBe(
      'http://192.168.1.1:8080/cgi-bin/luci/admin/system/attendedsysupgrade',
    );
    expect(attendedSysupgradeURL({ protocol: 'https:', hostname: 'router.lan' })).toBe(
      'https://router.lan:8443/cgi-bin/luci/admin/system/attendedsysupgrade',
    );
    expect(attendedSysupgradeURL({ protocol: 'http:', hostname: 'fd00::1' })).toBe(
      'http://[fd00::1]:8080/cgi-bin/luci/admin/system/attendedsysupgrade',
    );
  });

  it('maps codes, never text', () => {
    expect(openWrtCheckError('update_check_offline')).toMatch(/No Internet/);
    expect(openWrtCheckError('update_check_failed')).toMatch(/failed/);
    expect(openWrtCheckError(undefined)).toMatch(/failed/);
  });
});

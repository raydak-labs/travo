import { describe, expect, it } from 'vitest';
import { emptyForm, forwardRefusal, requestFrom, targetLabel } from './port-forwards';

describe('port forward request', () => {
  it('trims and omits empty optional fields', () => {
    expect(
      requestFrom({ ...emptyForm(), externalPort: ' 8080 ', targetIP: ' 192.168.1.50 ' }),
    ).toEqual({ proto: 'tcp', external_port: '8080', target_ip: '192.168.1.50' });
    expect(
      requestFrom({
        name: ' NAS ',
        proto: 'both',
        externalPort: '8000-8010',
        targetIP: '192.168.1.50',
        targetPort: '9000-9010',
      }),
    ).toEqual({
      name: 'NAS',
      proto: 'both',
      external_port: '8000-8010',
      target_ip: '192.168.1.50',
      target_port: '9000-9010',
    });
  });

  it('labels the target', () => {
    expect(targetLabel({ target_ip: '192.168.1.5', target_port: '' })).toBe('192.168.1.5');
    expect(targetLabel({ target_ip: '', target_port: '22' })).toBe('router:22');
  });

  it('maps refusal codes', () => {
    expect(forwardRefusal('port_forward_in_use').error).toContain('already uses');
    expect(forwardRefusal('not_found').message).toBeDefined();
    expect(forwardRefusal('bogus').error).toBe('Could not change the port forwards.');
  });
});

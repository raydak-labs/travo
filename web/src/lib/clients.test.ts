import { describe, expect, it } from 'vitest';
import { blockRefusal, clientError, clientName } from './api/clients';
import { connectionStatus, planClientEdit } from './clients';

const client = { mac: '22:4e:76:6c:2d:62', alias: 'Phone' };

describe('planClientEdit', () => {
  it('sends nothing when nothing changed', () => {
    const plan = planClientEdit(
      client,
      { ip: '192.168.1.5', name: 'phone' },
      {
        alias: ' Phone ',
        reserved: true,
        ip: '192.168.1.5',
        name: 'phone',
      },
    );
    expect(plan.steps).toEqual([]);
  });

  it('renames, then reserves a new address', () => {
    const plan = planClientEdit(client, undefined, {
      alias: 'Laptop',
      reserved: true,
      ip: ' 192.168.1.9 ',
      name: '',
    });
    expect(plan).toEqual({
      mac: client.mac,
      steps: [
        { kind: 'alias', alias: 'Laptop' },
        { kind: 'reserve', ip: '192.168.1.9', name: '' },
      ],
    });
  });

  it('clears the alias and drops the reservation', () => {
    const plan = planClientEdit(
      client,
      { ip: '192.168.1.5' },
      {
        alias: '',
        reserved: false,
        ip: '192.168.1.5',
        name: '',
      },
    );
    expect(plan.steps).toEqual([{ kind: 'alias', alias: '' }, { kind: 'unreserve' }]);
  });
});

describe('client labels', () => {
  it('names a client by alias, host name, then MAC', () => {
    expect(clientName({ mac: 'm', alias: 'a', hostname: 'h' })).toBe('a');
    expect(clientName({ mac: 'm', hostname: 'h' })).toBe('h');
    expect(clientName({ mac: 'm' })).toBe('m');
  });

  it('labels connections without claiming Ethernet for lan', () => {
    expect(connectionStatus({ connection: 'wifi', signal: -38 })).toEqual({
      kind: 'ok',
      label: 'WiFi · -38 dBm',
    });
    expect(connectionStatus({ connection: 'lan' }).label).toBe('LAN');
    expect(connectionStatus({ connection: 'offline' }).kind).toBe('neutral');
  });

  it('maps stable codes, and treats no change as information', () => {
    expect(clientError('client_is_caller')).toBe('That is the device you are using.');
    expect(clientError('something_new')).toBe('The change failed.');
    expect(blockRefusal('client_no_change')).toEqual({ message: 'Nothing to change.' });
    expect(blockRefusal('transaction_pending').error).toBeDefined();
  });
});

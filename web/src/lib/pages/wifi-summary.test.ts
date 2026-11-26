import { render } from 'svelte/server';
import { describe, expect, it } from 'vitest';
import type { WiFiSummary } from '../api/gen';
import WiFiSummaryView from './WiFiSummary.svelte';

const empty: WiFiSummary = { radios: [], stas: [], aps: [] };

function summary(data: Partial<WiFiSummary>): string {
  return render(WiFiSummaryView, { props: { data: { ...empty, ...data } } }).body;
}

describe('WiFi summary band', () => {
  it.each([
    { associated: true, text: 'Connected', kind: 'ok' },
    { associated: false, text: 'Not connected', kind: 'warn' },
    { associated: null, text: 'Unknown', kind: 'neutral' },
  ])('renders STA associated=$associated independently', ({ associated, text, kind }) => {
    const body = summary({
      stas: [
        { section: 'wwan', radio: 'radio0', ssid: 'HomeUplink', ifname: 'phy0-sta0', associated },
      ],
    });
    expect(body).toContain('HomeUplink');
    expect(body).toContain(text);
    expect(body).toContain(`status-${kind}`);
  });

  it('renders an absent STA as not configured, never as disconnected', () => {
    const body = summary({});
    expect(body).toContain('No uplink configured');
    expect(body).toContain('status-neutral');
    expect(body).not.toContain('Not connected');
  });

  it.each([
    { up: true, text: 'Up', kind: 'ok' },
    { up: false, text: 'Down', kind: 'danger' },
    { up: null, text: 'Unknown', kind: 'neutral' },
  ])('renders AP up=$up independently', ({ up, text, kind }) => {
    const body = summary({
      aps: [
        { section: 'default_radio1', radio: 'radio1', ssid: 'TravelAP', ifname: 'phy1-ap0', up },
      ],
    });
    expect(body).toContain('TravelAP');
    expect(body).toContain(text);
    expect(body).toContain(`status-${kind}`);
  });

  it('renders no APs as none', () => {
    expect(summary({})).toContain('None');
  });

  it('labels radio state as configured and distinguishes disabled, up and not up', () => {
    const body = summary({
      radios: [
        { name: 'radio0', band: '5g', disabled: false, up: true },
        { name: 'radio1', band: '2g', disabled: false, up: false },
        { name: 'radio2', band: '6g', disabled: true, up: false },
      ],
    });
    expect(body).toContain('Radios (configured)');
    expect(body).toContain('5 GHz');
    expect(body).toContain('2.4 GHz');
    expect(body).toContain('Up');
    expect(body).toContain('Not up');
    expect(body).toContain('Disabled');
    expect(body).toContain('status-ok');
    expect(body).toContain('status-warn');
    expect(body).toContain('status-neutral');
  });
});

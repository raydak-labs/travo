import { useNetworkStatus } from '@/hooks/use-network';
import { useRadios, useWifiConnection, useWifiHealth } from '@/hooks/use-wifi';
import { SummaryBand, SummaryTile } from '@/components/ui/summary-band';
import { StatValue } from '@/components/ui/stat-value';
import { getWifiModeLabel } from '@/components/wifi/wifi-mode-options';
import type { WifiBand } from '@shared/index';

function bandLabel(band: WifiBand | string): string {
  switch (band) {
    case '2.4ghz':
      return '2.4 GHz';
    case '5ghz':
      return '5 GHz';
    case '6ghz':
      return '6 GHz';
    default:
      return band;
  }
}

/**
 * Always-visible summary of the WiFi page.
 *
 * Every tile degrades on its own: one failed request must not cost the operator
 * the four other facts, and a failed request is never turned into a negative
 * claim ("no internet") it cannot support.
 *
 * `stale` is deliberately not passed. With the default staleTime of 0 every
 * tile would read as stale the instant it loaded and dim itself permanently.
 */
export function WifiSummaryBand() {
  const connection = useWifiConnection();
  const network = useNetworkStatus();
  const radios = useRadios();
  const health = useWifiHealth();

  // `conn` is only the association the endpoint actually reported as up: a
  // stale SSID from a past session must never read as a live connection.
  const conn = connection.data?.connected === true ? connection.data : undefined;
  const connected = conn !== undefined;
  const reachable = network.data?.internet_reachable;
  const radioList = radios.data ?? [];
  const healthData = health.data;
  const repeaterConflict = healthData?.repeater_same_radio_ap_sta === true;

  const healthTone = !healthData
    ? undefined
    : repeaterConflict || healthData.status === 'error'
      ? 'danger'
      : healthData.status === 'warning'
        ? 'warn'
        : 'ok';

  return (
    <SummaryBand label="WiFi status">
      <SummaryTile
        title="Connection"
        tone={connected ? 'ok' : 'neutral'}
        isLoading={connection.isLoading}
        isError={connection.isError}
        error={connection.error}
        onRetry={() => void connection.refetch()}
      >
        <div className="space-y-2">
          <StatValue
            label="SSID"
            value={conn ? conn.ssid : 'Not connected'}
            hint={conn ? `${bandLabel(conn.band)} · channel ${conn.channel}` : undefined}
          />
          <StatValue
            label="Signal"
            value={conn ? `${conn.signal_dbm} dBm · ${conn.signal_percent}%` : null}
          />
        </div>
      </SummaryTile>

      <SummaryTile
        title="Internet"
        tone={reachable === true ? 'ok' : reachable === false ? 'danger' : undefined}
        isLoading={network.isLoading}
        isError={network.isError}
        error={network.error}
        onRetry={() => void network.refetch()}
      >
        <StatValue
          label="Reachability"
          value={reachable === true ? 'Reachable' : reachable === false ? 'Not reachable' : null}
          // Same wording the WAN card uses: this is a WAN-carrier check, not a
          // live probe of a name or address.
          hint="WAN carrier check, not a live probe"
        />
      </SummaryTile>

      <SummaryTile
        title="Radios"
        isLoading={radios.isLoading}
        isError={radios.isError}
        error={radios.error}
        onRetry={() => void radios.refetch()}
      >
        <div className="space-y-2">
          {radioList.length === 0 ? (
            <StatValue label="Radios" value={null} />
          ) : (
            radioList.map((radio) => (
              <StatValue
                key={radio.name}
                label={`${radio.name} (${radio.band})`}
                value={`ch ${radio.channel} · ${radio.htmode}`}
                hint={`role ${radio.role} · ${radio.disabled ? 'disabled' : 'enabled'}`}
              />
            ))
          )}
          {/* These facts are read out of UCI/CONFIG, so a channel here is the
              configured one and can differ from what is on the air until the
              next apply. Saying so keeps the tile from being read as live
              radio state. */}
          <p className="text-xs text-gray-500 dark:text-gray-400">
            Configured radios, not live radio state.
          </p>
        </div>
      </SummaryTile>

      <SummaryTile
        title="Mode"
        isLoading={connection.isLoading}
        isError={connection.isError}
        error={connection.error}
        onRetry={() => void connection.refetch()}
      >
        <StatValue
          label="Router mode"
          value={conn?.mode ? getWifiModeLabel(conn.mode) : null}
          hint="How this router uses Wi-Fi"
        />
      </SummaryTile>

      <SummaryTile
        title="Health"
        tone={healthTone}
        isLoading={health.isLoading}
        isError={health.isError}
        error={health.error}
        onRetry={() => void health.refetch()}
      >
        <div className="space-y-2">
          <StatValue
            label="wwan"
            value={
              healthData?.wwan
                ? healthData.wwan.up
                  ? `Up · ${healthData.wwan.device || 'device unknown'}`
                  : 'Down'
                : null
            }
          />
          <StatValue
            label="STA"
            value={
              healthData?.sta
                ? healthData.sta.associated
                  ? `Associated · ${healthData.sta.ssid}`
                  : 'Not associated'
                : null
            }
          />
          {repeaterConflict ? (
            <StatValue
              label="Radio layout"
              value="AP shares the STA radio"
              hint="Fragile: reconcile or move the downlink"
            />
          ) : null}
        </div>
      </SummaryTile>
    </SummaryBand>
  );
}

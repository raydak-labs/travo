import { useNetworkStatus } from '@/hooks/use-network';
import { useWifiConnection } from '@/hooks/use-wifi';
import { SummaryBand, SummaryTile } from '@/components/ui/summary-band';
import { StatValue } from '@/components/ui/stat-value';
import { getWifiModeLabel } from '@/components/wifi/wifi-mode-options';

function bandLabel(band: string): string {
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
 * Three tiles only: what the connection is, whether the internet answers, and
 * how the router uses its radios. Radio details are configuration and belong
 * to WiFi / Advanced; health problems already get the banner below, so a tile
 * for them repeated the same warning and pushed the controls off the screen.
 *
 * Every tile degrades on its own: one failed request must not cost the operator
 * the other two facts, and a failed request is never turned into a negative
 * claim ("no internet") it cannot support.
 *
 * `stale` is deliberately not passed. With the default staleTime of 0 every
 * tile would read as stale the instant it loaded and dim itself permanently.
 */
export function WifiSummaryBand() {
  const connection = useWifiConnection();
  const network = useNetworkStatus();

  // `conn` is only the association the endpoint actually reported as up: a
  // stale SSID from a past session must never read as a live connection.
  const conn = connection.data?.connected === true ? connection.data : undefined;
  const connected = conn !== undefined;
  const reachable = network.data?.internet_reachable;

  return (
    <SummaryBand label="WiFi status" className="lg:grid-cols-3">
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
    </SummaryBand>
  );
}

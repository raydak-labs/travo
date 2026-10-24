import { useCaptivePortal } from '@/hooks/use-captive-portal';
import { useNetworkStatus } from '@/hooks/use-network';
import { useWifiConnection } from '@/hooks/use-wifi';
import { SummaryBand, SummaryTile } from '@/components/ui/summary-band';
import { StatValue } from '@/components/ui/stat-value';

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
 * Two tiles only: what the connection is, and whether the internet answers.
 * The mode selector sits directly below the band as three plain option boxes;
 * a third tile repeated a control the operator can already see. Radio details
 * are configuration and belong to WiFi / Advanced; health problems already get
 * the banner above, so a tile for them repeated the same warning and pushed the
 * controls off the screen.
 *
 * Every tile degrades on its own: one failed request must not cost the operator
 * the other fact, and a failed request is never turned into a negative claim
 * ("no internet") it cannot support. The Internet tile prefers the captive
 * portal probe because it is the only endpoint that distinguishes a working
 * uplink from a login page; `network_status` remains the fallback when the
 * probe is unavailable.
 *
 * `stale` is deliberately not passed. With the default staleTime of 0 every
 * tile would read as stale the instant it loaded and dim itself permanently.
 */
export function WifiSummaryBand() {
  const connection = useWifiConnection();
  const network = useNetworkStatus();
  const portal = useCaptivePortal();

  // `conn` is only the association the endpoint actually reported as up: a
  // stale SSID from a past session must never read as a live connection.
  const conn = connection.data?.connected === true ? connection.data : undefined;
  const connected = conn !== undefined;

  const portalStatus = portal.data;
  const reachable = portalStatus?.can_reach_internet ?? network.data?.internet_reachable;
  const portalDetected = portalStatus?.detected === true;
  const portalLoginRequired = portalDetected && reachable === false;
  const internetSourceKnown = portalStatus !== undefined || network.data !== undefined;
  const internetHint = !internetSourceKnown
    ? undefined
    : portalLoginRequired
      ? 'Captive portal detected'
      : portalStatus
        ? 'Live probe from the router'
        : 'WAN carrier check, not a live probe';

  return (
    <SummaryBand label="WiFi status" className="lg:grid-cols-2">
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
        tone={
          portalLoginRequired
            ? 'warn'
            : reachable === true
              ? 'ok'
              : reachable === false
                ? 'danger'
                : undefined
        }
        isLoading={portal.isLoading && network.isLoading}
        isError={portal.isError && network.isError}
        error={portal.error ?? network.error}
        onRetry={() => {
          void portal.refetch();
          void network.refetch();
        }}
      >
        <StatValue
          label="Reachability"
          value={
            portalLoginRequired
              ? 'Login required'
              : reachable === true
                ? 'Reachable'
                : reachable === false
                  ? 'Not reachable'
                  : null
          }
          hint={internetHint}
        />
      </SummaryTile>
    </SummaryBand>
  );
}

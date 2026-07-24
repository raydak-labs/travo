import {
  Cable,
  Wifi,
  Smartphone,
  Shield,
  Monitor,
  Router,
  CheckCircle2,
  XCircle,
} from 'lucide-react';
import { Link } from '@tanstack/react-router';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { TimezoneAlert } from '@/components/timezone-alert';
import { useTopologyData } from '@/hooks/use-topology-data';
import type { SourceDef } from '@/hooks/use-topology-data';
import { formatUptime } from '@/lib/utils';
import { QuickActions } from '@/pages/dashboard/quick-actions';
import { NetworkChart } from '@/pages/dashboard/network-chart';

// ─── Topology diagram ────────────────────────────────────────────────────────

interface ClientDef {
  label: string;
  icon: typeof Wifi;
  count: number;
}

function TopologyRouterCenter({
  hostname,
  model,
  features,
  loading,
}: {
  hostname: string;
  model: string;
  features: { label: string; active: boolean }[];
  loading: boolean;
}) {
  if (loading) {
    return <Skeleton className="h-20 w-20 rounded-2xl" />;
  }

  return (
    <>
      <div className="relative flex h-20 w-20 items-center justify-center rounded-2xl border border-slate-600 bg-slate-800">
        <Router className="h-10 w-10 text-slate-300" />
        <div className="absolute -right-1 -top-1 h-3 w-3 rounded-full bg-emerald-400 ring-2 ring-slate-900" />
      </div>
      <div className="min-w-0 max-w-full px-1 text-center">
        <div className="text-xs font-semibold leading-tight text-slate-200">
          {hostname || 'OpenWRT'}
        </div>
        {model && (
          <div className="break-words text-[10px] leading-tight text-slate-500">{model}</div>
        )}
      </div>
      <div className="flex max-w-full flex-wrap justify-center gap-1">
        {features.map((f) => (
          <span
            key={f.label}
            className={`shrink-0 rounded border px-1.5 py-0.5 text-[10px] font-medium ${
              f.active
                ? 'border-emerald-600 bg-emerald-900/40 text-emerald-300'
                : 'border-slate-700 bg-slate-800/40 text-slate-500'
            }`}
          >
            {f.label}
          </span>
        ))}
      </div>
    </>
  );
}

function TopologyDiagram({
  sources,
  clients,
  hostname,
  model,
  features,
  loading,
}: {
  sources: SourceDef[];
  clients: ClientDef[];
  hostname: string;
  model: string;
  features: { label: string; active: boolean }[];
  loading: boolean;
}) {
  const ROW_H = 56;
  const DIAGRAM_H = Math.max(sources.length, clients.length * 2) * ROW_H;
  const LEFT_COL = 172;
  const RIGHT_COL = 160;
  const CENTER_W = 160;

  const sourceYs = sources.map((_, i) => ((i + 0.5) / sources.length) * DIAGRAM_H);
  const clientYs =
    clients.length === 1
      ? [DIAGRAM_H / 2]
      : clients.map((_, i) => ((i + 0.5) / clients.length) * DIAGRAM_H);

  const routerCY = DIAGRAM_H / 2;

  return (
    <div className="@container overflow-visible rounded-xl bg-slate-900 p-4 dark:bg-slate-950 sm:p-6">
      {/* Stack when topology card < ~576px (@xl container). Phones only; mid widths keep line graph. */}
      <div data-testid="topology-mobile" className="flex flex-col gap-5 @xl:hidden">
        <div className="space-y-2">
          {sources.map((src) => {
            const Icon = src.icon;
            return (
              <div key={src.label} className="flex min-w-0 items-center gap-2">
                <div
                  className={`h-2 w-2 flex-shrink-0 rounded-full ${
                    src.connected ? 'bg-emerald-400' : 'bg-slate-600'
                  }`}
                />
                <Icon
                  className={`h-4 w-4 flex-shrink-0 ${
                    src.connected ? 'text-emerald-400' : 'text-slate-500'
                  }`}
                />
                <div className="min-w-0">
                  <div className="text-xs font-medium leading-tight text-slate-200">
                    {src.label}
                  </div>
                  {src.detail && (
                    <div className="break-words text-[10px] leading-tight text-slate-500">
                      {src.detail}
                    </div>
                  )}
                </div>
              </div>
            );
          })}
        </div>

        <div className="flex flex-col items-center justify-center gap-2">
          <TopologyRouterCenter
            hostname={hostname}
            model={model}
            features={features}
            loading={loading}
          />
        </div>

        <div className="flex flex-wrap justify-center gap-4">
          {clients.map((client) => {
            const Icon = client.icon;
            return (
              <div key={client.label} className="flex items-center gap-2">
                <div className="rounded-lg border border-slate-700 bg-slate-800 p-1.5">
                  <Icon className="h-4 w-4 text-slate-400" />
                </div>
                <div>
                  {loading ? (
                    <Skeleton className="h-5 w-8" />
                  ) : (
                    <div className="text-2xl font-bold leading-none text-slate-100">
                      {client.count}
                    </div>
                  )}
                  <div className="text-[10px] text-slate-500">{client.label}</div>
                </div>
              </div>
            );
          })}
        </div>
      </div>

      {/* Original horizontal line diagram once the card is wide enough */}
      <div
        data-testid="topology-desktop"
        className="relative hidden min-w-0 items-stretch overflow-x-auto @xl:flex"
        style={{ minHeight: DIAGRAM_H + 48 }}
      >
        <div style={{ width: LEFT_COL, minWidth: LEFT_COL }}>
          <div className="relative" style={{ height: DIAGRAM_H }}>
            {sources.map((src, i) => {
              const Icon = src.icon;
              const y = sourceYs[i];
              return (
                <div
                  key={src.label}
                  className="absolute flex items-center gap-2"
                  style={{ top: y - ROW_H / 2, height: ROW_H, left: 0, right: 0 }}
                >
                  <div
                    className={`h-2 w-2 flex-shrink-0 rounded-full ${
                      src.connected ? 'bg-emerald-400' : 'bg-slate-600'
                    }`}
                  />
                  <Icon
                    className={`h-4 w-4 flex-shrink-0 ${
                      src.connected ? 'text-emerald-400' : 'text-slate-500'
                    }`}
                  />
                  <div className="min-w-0">
                    <div className="text-xs font-medium leading-tight text-slate-200">
                      {src.label}
                    </div>
                    {src.detail && (
                      <div className="truncate text-[10px] leading-tight text-slate-500">
                        {src.detail}
                      </div>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        </div>

        <div className="relative min-w-[40px] flex-1">
          <svg
            className="absolute inset-0 h-full w-full"
            preserveAspectRatio="none"
            viewBox={`0 0 100 ${DIAGRAM_H}`}
          >
            {sources.map((src, i) => {
              const y = sourceYs[i];
              const midX = 50;
              const color = src.connected ? '#10b981' : '#334155';
              return (
                <path
                  key={src.label}
                  d={`M 0 ${y} C ${midX} ${y}, ${midX} ${routerCY}, 100 ${routerCY}`}
                  fill="none"
                  stroke={color}
                  strokeWidth="1.5"
                  strokeDasharray={src.connected ? '0' : '4 3'}
                  opacity={src.connected ? 0.7 : 0.35}
                />
              );
            })}
          </svg>
        </div>

        <div
          className="flex w-[160px] flex-shrink-0 flex-col items-center justify-center gap-2"
          style={{ width: CENTER_W }}
        >
          <TopologyRouterCenter
            hostname={hostname}
            model={model}
            features={features}
            loading={loading}
          />
        </div>

        <div className="relative min-w-[40px] flex-1">
          <svg
            className="absolute inset-0 h-full w-full"
            preserveAspectRatio="none"
            viewBox={`0 0 100 ${DIAGRAM_H}`}
          >
            {clients.map((client, i) => {
              const y = clientYs[i];
              const midX = 50;
              const hasClients = client.count > 0;
              const color = hasClients ? '#10b981' : '#334155';
              return (
                <path
                  key={client.label}
                  d={`M 0 ${routerCY} C ${midX} ${routerCY}, ${midX} ${y}, 100 ${y}`}
                  fill="none"
                  stroke={color}
                  strokeWidth="1.5"
                  strokeDasharray={hasClients ? '0' : '4 3'}
                  opacity={hasClients ? 0.7 : 0.35}
                />
              );
            })}
          </svg>
        </div>

        <div style={{ width: RIGHT_COL, minWidth: RIGHT_COL }}>
          <div className="relative" style={{ height: DIAGRAM_H }}>
            {clients.map((client, i) => {
              const Icon = client.icon;
              const y = clientYs[i];
              return (
                <div
                  key={client.label}
                  className="absolute flex items-center gap-3"
                  style={{ top: y - ROW_H / 2, height: ROW_H, left: 0, right: 0 }}
                >
                  <div className="rounded-lg border border-slate-700 bg-slate-800 p-1.5">
                    <Icon className="h-4 w-4 text-slate-400" />
                  </div>
                  <div>
                    {loading ? (
                      <Skeleton className="h-5 w-8" />
                    ) : (
                      <div className="text-2xl font-bold leading-none text-slate-100">
                        {client.count}
                      </div>
                    )}
                    <div className="text-[10px] text-slate-500">{client.label}</div>
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      </div>
    </div>
  );
}

// ─── Connection source cards ──────────────────────────────────────────────────

function SourceCard({
  title,
  icon: Icon,
  connected,
  children,
}: {
  title: string;
  icon: typeof Cable;
  connected: boolean;
  children?: React.ReactNode;
}) {
  return (
    <Card className="border-slate-700 bg-slate-900 text-slate-200">
      <CardHeader className="pb-2">
        <div className="flex items-center justify-between">
          <CardTitle className="flex items-center gap-2 text-slate-100">
            <Icon className="h-4 w-4 text-slate-400" />
            {title}
          </CardTitle>
          {connected ? (
            <CheckCircle2 className="h-4 w-4 text-emerald-500" />
          ) : (
            <XCircle className="h-4 w-4 text-slate-600" />
          )}
        </div>
      </CardHeader>
      <CardContent>
        <div
          className={`mb-3 inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-medium ${
            connected ? 'bg-emerald-500/10 text-emerald-400' : 'bg-slate-700/40 text-slate-500'
          }`}
        >
          <span
            className={`h-1.5 w-1.5 rounded-full ${connected ? 'bg-emerald-400' : 'bg-slate-600'}`}
          />
          {connected ? 'Connected' : 'Not connected'}
        </div>
        {children && <div className="space-y-2 text-xs text-slate-500">{children}</div>}
      </CardContent>
    </Card>
  );
}

function DetailRow({
  label,
  children,
  mono,
}: {
  label: string;
  children: React.ReactNode;
  mono?: boolean;
}) {
  return (
    <div className="flex flex-col gap-0.5 sm:flex-row sm:items-start sm:justify-between sm:gap-3">
      <span className="shrink-0">{label}</span>
      <span
        className={`min-w-0 break-words text-slate-300 sm:text-right ${mono ? 'font-mono' : ''}`}
      >
        {children}
      </span>
    </div>
  );
}

// ─── Main page ────────────────────────────────────────────────────────────────

export function DashboardPage() {
  const {
    sources,
    clients,
    features,
    router,
    loading,
    ethernetUp,
    repeaterUp,
    tetherUp,
    wan,
    wifiConn,
    usbTether,
    sysInfo,
    vpnActive,
    internetUp,
    allClients,
  } = useTopologyData();

  const usbDisplayIp =
    (usbTether?.ip_address || (wan?.type === 'usb' ? wan?.ip_address : '')) ?? '';

  return (
    <div className="space-y-6">
      <TimezoneAlert />

      <TopologyDiagram
        sources={sources}
        clients={clients}
        hostname={router.hostname}
        model={router.model}
        features={features}
        loading={loading}
      />

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <SourceCard title="Ethernet (WAN)" icon={Cable} connected={ethernetUp}>
          {ethernetUp && wan ? (
            <>
              <DetailRow label="Protocol">
                <span className="uppercase">{wan.type}</span>
              </DetailRow>
              {wan.ip_address && <DetailRow label="IP" mono>{wan.ip_address}</DetailRow>}
              {wan.gateway && <DetailRow label="Gateway" mono>{wan.gateway}</DetailRow>}
            </>
          ) : (
            <p className="text-slate-500">No Ethernet WAN connection detected.</p>
          )}
        </SourceCard>

        <SourceCard title="Repeater (WiFi)" icon={Wifi} connected={repeaterUp}>
          {repeaterUp && wifiConn ? (
            <>
              <DetailRow label="SSID" mono>
                {wifiConn.ssid}
              </DetailRow>
              <DetailRow label="Band">
                <span className="uppercase">{wifiConn.band}</span>
              </DetailRow>
              <DetailRow label="Signal">{wifiConn.signal_percent}%</DetailRow>
            </>
          ) : (
            <p className="text-slate-500">Repeater (STA) is disabled.</p>
          )}
        </SourceCard>

        <SourceCard title="USB Tethering" icon={Smartphone} connected={tetherUp}>
          {tetherUp ? (
            <>
              {usbTether?.detected && (
                <DetailRow label="Device">
                  <span className="capitalize">{usbTether.device_type || 'Unknown'}</span>
                </DetailRow>
              )}
              {usbDisplayIp ? <DetailRow label="IP" mono>{usbDisplayIp}</DetailRow> : null}
              {tetherUp && !usbTether?.detected && !usbDisplayIp ? (
                <p className="text-slate-500">USB uplink is active.</p>
              ) : null}
            </>
          ) : (
            <p className="text-slate-500">
              No tethering device found. Plug in your smartphone or USB modem to start.
            </p>
          )}
        </SourceCard>
      </div>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle>Quick status</CardTitle>
          <Monitor className="h-4 w-4 text-gray-500 dark:text-gray-400" />
        </CardHeader>
        <CardContent>
          {loading ? (
            <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-4">
              <Skeleton className="h-14 w-full" />
              <Skeleton className="h-14 w-full" />
              <Skeleton className="h-14 w-full" />
              <Skeleton className="h-14 w-full" />
            </div>
          ) : (
            <>
              <div className="grid gap-4 text-sm sm:grid-cols-2 lg:grid-cols-4">
                <div>
                  <span className="text-gray-500 dark:text-gray-400">Internet</span>
                  <p
                    className={`mt-0.5 font-medium ${
                      internetUp
                        ? 'text-emerald-600 dark:text-emerald-400'
                        : 'text-red-600 dark:text-red-400'
                    }`}
                  >
                    {internetUp ? 'Reachable' : 'Unreachable'}
                  </p>
                </div>
                <div>
                  <span className="text-gray-500 dark:text-gray-400">VPN</span>
                  <div className="mt-0.5 flex items-center gap-1.5">
                    {vpnActive ? (
                      <>
                        <Shield className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" />
                        <span className="font-medium text-emerald-600 dark:text-emerald-400">
                          On
                        </span>
                      </>
                    ) : (
                      <>
                        <Shield className="h-3.5 w-3.5 text-gray-500 dark:text-gray-400" />
                        <span className="font-medium text-gray-500 dark:text-gray-400">Off</span>
                      </>
                    )}
                  </div>
                </div>
                <div>
                  <span className="text-gray-500 dark:text-gray-400">Devices on your network</span>
                  <p className="mt-0.5 font-medium">{allClients.length}</p>
                </div>
                <div>
                  <span className="text-gray-500 dark:text-gray-400">Uptime</span>
                  <p className="mt-0.5 font-medium">
                    {sysInfo ? formatUptime(sysInfo.uptime_seconds) : '—'}
                  </p>
                </div>
              </div>
              <p className="mt-4 text-sm text-gray-500 dark:text-gray-400">
                <Link to="/system" className="underline-offset-4 hover:underline">
                  Device details and settings
                </Link>{' '}
                — hostname, firmware, CPU, and storage.
              </p>
            </>
          )}
        </CardContent>
      </Card>

      <QuickActions />

      <NetworkChart />
    </div>
  );
}

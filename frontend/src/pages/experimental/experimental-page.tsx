import { Cable, Wifi, Smartphone, Signal, Shield, Monitor, Router, CheckCircle2, XCircle } from 'lucide-react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { useNetworkStatus, useIPv6Status } from '@/hooks/use-network';
import { useVpnStatus } from '@/hooks/use-vpn';
import { useWifiConnection } from '@/hooks/use-wifi';
import { useSystemInfo } from '@/hooks/use-system';
import { useUSBTetherStatus } from '@/hooks/use-usb-tether';

// ─── Topology diagram ────────────────────────────────────────────────────────

interface SourceDef {
  label: string;
  icon: typeof Cable;
  connected: boolean;
  detail?: string;
}

interface ClientDef {
  label: string;
  icon: typeof Wifi;
  count: number;
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

  // Y centres for sources (evenly spaced)
  const sourceYs = sources.map((_, i) => ((i + 0.5) / sources.length) * DIAGRAM_H);
  // Y centres for clients
  const clientYs =
    clients.length === 1
      ? [DIAGRAM_H / 2]
      : clients.map((_, i) => ((i + 0.5) / clients.length) * DIAGRAM_H);

  const routerCY = DIAGRAM_H / 2;

  return (
    <div
      className="relative flex items-stretch rounded-xl bg-slate-900 dark:bg-slate-950 p-6 overflow-hidden"
      style={{ minHeight: DIAGRAM_H + 48 }}
    >
      {/* ── Left: upstream sources ── */}
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
                  className={`h-2 w-2 rounded-full flex-shrink-0 ${
                    src.connected ? 'bg-emerald-400' : 'bg-slate-600'
                  }`}
                />
                <Icon
                  className={`h-4 w-4 flex-shrink-0 ${
                    src.connected ? 'text-emerald-400' : 'text-slate-500'
                  }`}
                />
                <div className="min-w-0">
                  <div className="text-xs font-medium text-slate-200 leading-tight">{src.label}</div>
                  {src.detail && (
                    <div className="text-[10px] text-slate-500 leading-tight truncate">{src.detail}</div>
                  )}
                </div>
              </div>
            );
          })}
        </div>
      </div>

      {/* ── Left SVG: fan-in lines ── */}
      <div className="flex-1 relative" style={{ minWidth: 40 }}>
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

      {/* ── Center: router box ── */}
      <div
        className="flex-shrink-0 flex flex-col items-center justify-center gap-2"
        style={{ width: CENTER_W }}
      >
        {loading ? (
          <Skeleton className="h-20 w-20 rounded-2xl" />
        ) : (
          <>
            {/* Router icon */}
            <div className="relative flex h-20 w-20 items-center justify-center rounded-2xl border border-slate-600 bg-slate-800">
              <Router className="h-10 w-10 text-slate-300" />
              <div className="absolute -top-1 -right-1 h-3 w-3 rounded-full bg-emerald-400 ring-2 ring-slate-900" />
            </div>
            {/* Device name */}
            <div className="text-center">
              <div className="text-xs font-semibold text-slate-200 leading-tight">{hostname || 'OpenWRT'}</div>
              {model && (
                <div className="text-[10px] text-slate-500 leading-tight">{model}</div>
              )}
            </div>
            {/* Feature badges */}
            <div className="flex flex-wrap justify-center gap-1">
              {features.map((f) => (
                <span
                  key={f.label}
                  className={`rounded px-1.5 py-0.5 text-[10px] font-medium border ${
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
        )}
      </div>

      {/* ── Right SVG: fan-out lines ── */}
      <div className="flex-1 relative" style={{ minWidth: 40 }}>
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

      {/* ── Right: client counts ── */}
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
    <Card className="dark:bg-slate-900 dark:border-slate-700">
      <CardHeader className="pb-2">
        <div className="flex items-center justify-between">
          <CardTitle className="flex items-center gap-2 text-sm font-medium">
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
            connected
              ? 'bg-emerald-500/10 text-emerald-400'
              : 'bg-slate-700/40 text-slate-500'
          }`}
        >
          <span
            className={`h-1.5 w-1.5 rounded-full ${connected ? 'bg-emerald-400' : 'bg-slate-600'}`}
          />
          {connected ? 'Connected' : 'Not connected'}
        </div>
        {children && (
          <div className="space-y-1 text-xs text-slate-500 dark:text-slate-500">{children}</div>
        )}
      </CardContent>
    </Card>
  );
}

// ─── Main page ────────────────────────────────────────────────────────────────

export function ExperimentalPage() {
  const { data: network, isLoading: networkLoading } = useNetworkStatus();
  const { data: wifiConn, isLoading: wifiLoading } = useWifiConnection();
  const { data: vpnStatus } = useVpnStatus();
  const { data: sysInfo, isLoading: sysLoading } = useSystemInfo();
  const { data: ipv6Status } = useIPv6Status();
  const { data: usbTether } = useUSBTetherStatus();

  const loading = networkLoading || wifiLoading || sysLoading;

  // Connection states
  const wan = network?.wan;
  const ethernetUp = wan?.is_up ?? false;
  const repeaterUp = (wifiConn?.connected && wifiConn.mode === 'client') ?? false;
  const tetherUp = usbTether?.is_up ?? false;
  const vpnActive = vpnStatus?.some((v) => v.connected) ?? false;
  const ipv6Enabled = ipv6Status?.enabled ?? false;
  const internetUp = network?.internet_reachable ?? false;

  // Client breakdown by interface type
  const allClients = network?.clients ?? [];
  const wlanClients = allClients.filter(
    (c) =>
      c.interface_name.startsWith('wlan') ||
      c.interface_name.startsWith('ath') ||
      c.interface_name.includes('wifi'),
  ).length;
  const lanClients = allClients.length - wlanClients;

  const sources: SourceDef[] = [
    {
      label: 'Ethernet',
      icon: Cable,
      connected: ethernetUp,
      detail: ethernetUp ? wan?.ip_address : undefined,
    },
    {
      label: 'Repeater (WiFi)',
      icon: Wifi,
      connected: repeaterUp,
      detail: repeaterUp ? wifiConn?.ssid : 'Disabled',
    },
    {
      label: 'USB Tethering',
      icon: Smartphone,
      connected: tetherUp,
      detail: tetherUp ? usbTether?.device_type || 'Connected' : 'No device',
    },
    {
      label: 'Cellular',
      icon: Signal,
      connected: false,
      detail: 'No modem',
    },
  ];

  const clients: ClientDef[] = [
    { label: 'WLAN Clients', icon: Wifi, count: wlanClients },
    { label: 'LAN Clients', icon: Cable, count: lanClients },
  ];

  const features = [
    { label: 'IPv6', active: ipv6Enabled },
    { label: 'VPN', active: vpnActive },
    { label: 'Internet', active: internetUp },
  ];

  return (
    <div className="space-y-6">
      {/* Page header */}
      <div className="flex items-center gap-3">
        <Badge variant="outline" className="border-amber-500 text-amber-600 dark:text-amber-400">
          Experimental
        </Badge>
        <span className="text-sm text-muted-foreground">
          Network overview — inspired by GL-iNet admin panel
        </span>
      </div>

      {/* Topology diagram */}
      <TopologyDiagram
        sources={sources}
        clients={clients}
        hostname={sysInfo?.hostname ?? ''}
        model={sysInfo?.model ?? ''}
        features={features}
        loading={loading}
      />

      {/* Connection source detail cards */}
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {/* Ethernet / WAN */}
        <SourceCard title="Ethernet (WAN)" icon={Cable} connected={ethernetUp}>
          {ethernetUp && wan ? (
            <>
              <div className="flex justify-between">
                <span>Protocol</span>
                <span className="text-slate-300 dark:text-slate-300 uppercase">{wan.type}</span>
              </div>
              {wan.ip_address && (
                <div className="flex justify-between">
                  <span>IP</span>
                  <span className="text-slate-300 dark:text-slate-300 font-mono">{wan.ip_address}</span>
                </div>
              )}
              {wan.gateway && (
                <div className="flex justify-between">
                  <span>Gateway</span>
                  <span className="text-slate-300 dark:text-slate-300 font-mono">{wan.gateway}</span>
                </div>
              )}
            </>
          ) : (
            <p className="text-slate-500">No Ethernet WAN connection detected.</p>
          )}
        </SourceCard>

        {/* Repeater */}
        <SourceCard title="Repeater (WiFi)" icon={Wifi} connected={repeaterUp}>
          {repeaterUp && wifiConn ? (
            <>
              <div className="flex justify-between">
                <span>SSID</span>
                <span className="text-slate-300 dark:text-slate-300 font-mono truncate max-w-[80px]">
                  {wifiConn.ssid}
                </span>
              </div>
              <div className="flex justify-between">
                <span>Band</span>
                <span className="text-slate-300 dark:text-slate-300 uppercase">{wifiConn.band}</span>
              </div>
              <div className="flex justify-between">
                <span>Signal</span>
                <span className="text-slate-300 dark:text-slate-300">{wifiConn.signal_percent}%</span>
              </div>
            </>
          ) : (
            <p className="text-slate-500">Repeater (STA) is disabled.</p>
          )}
        </SourceCard>

        {/* USB Tethering */}
        <SourceCard title="USB Tethering" icon={Smartphone} connected={tetherUp}>
          {usbTether?.detected ? (
            <>
              <div className="flex justify-between">
                <span>Device</span>
                <span className="text-slate-300 dark:text-slate-300 capitalize">
                  {usbTether.device_type || 'Unknown'}
                </span>
              </div>
              {usbTether.ip_address && (
                <div className="flex justify-between">
                  <span>IP</span>
                  <span className="text-slate-300 dark:text-slate-300 font-mono">
                    {usbTether.ip_address}
                  </span>
                </div>
              )}
            </>
          ) : (
            <p className="text-slate-500">No tethering device found. Plug in your smartphone or USB modem to start.</p>
          )}
        </SourceCard>

        {/* Cellular */}
        <SourceCard title="Cellular" icon={Signal} connected={false}>
          <p className="text-slate-500">No modem device found. Plug in your USB modem to start.</p>
        </SourceCard>
      </div>

      {/* System info strip */}
      <Card className="dark:bg-slate-900 dark:border-slate-700">
        <CardHeader className="pb-2">
          <CardTitle className="flex items-center gap-2 text-sm font-medium">
            <Monitor className="h-4 w-4 text-slate-400" />
            Device Information
          </CardTitle>
        </CardHeader>
        <CardContent>
          {sysLoading ? (
            <div className="grid gap-2 sm:grid-cols-3">
              <Skeleton className="h-4 w-full" />
              <Skeleton className="h-4 w-full" />
              <Skeleton className="h-4 w-full" />
            </div>
          ) : (
            <div className="grid gap-3 text-xs sm:grid-cols-3">
              <div>
                <span className="text-muted-foreground">Hostname</span>
                <p className="font-medium mt-0.5">{sysInfo?.hostname ?? '—'}</p>
              </div>
              <div>
                <span className="text-muted-foreground">Model</span>
                <p className="font-medium mt-0.5">{sysInfo?.model ?? '—'}</p>
              </div>
              <div>
                <span className="text-muted-foreground">Firmware</span>
                <p className="font-medium mt-0.5">{sysInfo?.firmware_version ?? '—'}</p>
              </div>
              <div>
                <span className="text-muted-foreground">Kernel</span>
                <p className="font-medium mt-0.5">{sysInfo?.kernel_version ?? '—'}</p>
              </div>
              <div>
                <span className="text-muted-foreground">Internet</span>
                <p className={`font-medium mt-0.5 ${internetUp ? 'text-emerald-500' : 'text-red-500'}`}>
                  {internetUp ? 'Reachable' : 'Unreachable'}
                </p>
              </div>
              <div>
                <span className="text-muted-foreground">VPN</span>
                <div className="flex items-center gap-1.5 mt-0.5">
                  {vpnActive ? (
                    <>
                      <Shield className="h-3 w-3 text-emerald-500" />
                      <span className="font-medium text-emerald-500">Active</span>
                    </>
                  ) : (
                    <>
                      <Shield className="h-3 w-3 text-slate-500" />
                      <span className="font-medium text-slate-500">Inactive</span>
                    </>
                  )}
                </div>
              </div>
              <div>
                <span className="text-muted-foreground">IPv6</span>
                <p className={`font-medium mt-0.5 ${ipv6Enabled ? 'text-emerald-500' : 'text-slate-500'}`}>
                  {ipv6Enabled ? 'Enabled' : 'Disabled'}
                </p>
              </div>
              <div>
                <span className="text-muted-foreground">Total Clients</span>
                <p className="font-medium mt-0.5">{allClients.length}</p>
              </div>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

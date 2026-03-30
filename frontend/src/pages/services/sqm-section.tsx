import { useMemo, useState } from 'react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import type { ServiceInfo, SQMConfig, SQMQdisc, SQMScript } from '@shared/index';
import { useApplySQM, useSQMConfig, useSetSQMConfig } from '@/hooks/use-sqm';

type Props = {
  sqmService: ServiceInfo | undefined;
  onInstall: (id: string) => void;
  streamActionActive: boolean;
};

const DEFAULT_CFG: SQMConfig = {
  enabled: false,
  interface: '',
  download_kbit: 0,
  upload_kbit: 0,
  qdisc: 'cake',
  script: 'piece_of_cake.qos',
};

function toInt(s: string) {
  const v = Number.parseInt(s, 10);
  return Number.isFinite(v) ? v : 0;
}

export function SQMSection({ sqmService, onInstall, streamActionActive }: Props) {
  const installed = sqmService?.state !== 'not_installed';
  const { data, isLoading, isError } = useSQMConfig(installed);
  const setCfg = useSetSQMConfig();
  const apply = useApplySQM();

  const [draft, setDraft] = useState<SQMConfig | null>(null);
  const current = draft ?? data ?? DEFAULT_CFG;
  const hasUnsavedChanges = draft !== null;

  const disabled = useMemo(() => {
    return (
      streamActionActive ||
      setCfg.isPending ||
      apply.isPending ||
      sqmService?.state === 'not_installed' ||
      sqmService === undefined
    );
  }, [apply.isPending, setCfg.isPending, sqmService, streamActionActive]);

  const canEdit = installed && !isLoading && !isError;

  if (!sqmService || sqmService.state === 'not_installed') {
    return (
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">SQM (Traffic Shaping)</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <p className="text-sm">
            Reduce latency (bufferbloat) on slow or variable WAN links by shaping traffic.
          </p>
          <div className="flex gap-2">
            <Button onClick={() => onInstall('sqm')} disabled={streamActionActive}>
              Install SQM
            </Button>
          </div>
        </CardContent>
      </Card>
    );
  }

  const qdisc = current.qdisc as SQMQdisc;
  const script = current.script as SQMScript;

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">SQM (Traffic Shaping)</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        {data?.advanced_hint ? <p className="text-sm">{data.advanced_hint}</p> : null}
        {isLoading ? <p className="text-sm">Loading SQM configuration…</p> : null}
        {isError ? <p className="text-sm">Failed to load SQM configuration.</p> : null}
        {hasUnsavedChanges ? (
          <p className="text-sm">You have unsaved changes. Save before applying.</p>
        ) : null}

        <div className="flex items-center justify-between gap-3">
          <div className="space-y-1">
            <label htmlFor="sqm-enabled" className="text-sm font-medium leading-none">
              Enable SQM
            </label>
          </div>
          <Switch
            id="sqm-enabled"
            checked={current.enabled}
            disabled={disabled || !canEdit}
            onChange={(e) =>
              setDraft((p) => ({ ...(p ?? current), enabled: e.currentTarget.checked }))
            }
          />
        </div>

        <div className="grid gap-4 md:grid-cols-2">
          <div className="space-y-2">
            <label htmlFor="sqm-interface" className="text-sm font-medium leading-none">
              Interface
            </label>
            <Input
              id="sqm-interface"
              placeholder="pppoe-wan / eth0.2 / wan"
              value={current.interface}
              disabled={disabled || !canEdit}
              onChange={(e) => setDraft((p) => ({ ...(p ?? current), interface: e.target.value }))}
            />
          </div>

          <div className="space-y-2">
            <div className="text-sm font-medium leading-none">Queue discipline</div>
            <Select
              value={qdisc}
              disabled={disabled || !canEdit}
              onValueChange={(v) => {
                const next = v as SQMQdisc;
                setDraft((p) => ({
                  ...(p ?? current),
                  qdisc: next,
                  script: next === 'fq_codel' ? 'simple.qos' : 'piece_of_cake.qos',
                }));
              }}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="cake">cake</SelectItem>
                <SelectItem value="fq_codel">fq_codel</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-2">
            <div className="text-sm font-medium leading-none">Script preset</div>
            <Select
              value={script}
              disabled={disabled || !canEdit}
              onValueChange={(v) =>
                setDraft((p) => ({ ...(p ?? current), script: v as SQMScript }))
              }
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="piece_of_cake.qos">piece_of_cake.qos</SelectItem>
                <SelectItem value="layer_cake.qos">layer_cake.qos</SelectItem>
                <SelectItem value="simple.qos">simple.qos</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>

        <div className="grid gap-4 md:grid-cols-2">
          <div className="space-y-2">
            <label htmlFor="sqm-download" className="text-sm font-medium leading-none">
              Download (kbit/s)
            </label>
            <Input
              id="sqm-download"
              inputMode="numeric"
              value={String(current.download_kbit)}
              disabled={disabled || !canEdit}
              onChange={(e) =>
                setDraft((p) => ({ ...(p ?? current), download_kbit: toInt(e.target.value) }))
              }
            />
          </div>
          <div className="space-y-2">
            <label htmlFor="sqm-upload" className="text-sm font-medium leading-none">
              Upload (kbit/s)
            </label>
            <Input
              id="sqm-upload"
              inputMode="numeric"
              value={String(current.upload_kbit)}
              disabled={disabled || !canEdit}
              onChange={(e) =>
                setDraft((p) => ({ ...(p ?? current), upload_kbit: toInt(e.target.value) }))
              }
            />
          </div>
        </div>

        <div className="flex flex-wrap gap-2">
          <Button
            variant="secondary"
            disabled={disabled || !canEdit}
            onClick={() => {
              setCfg.mutate(current, { onSuccess: () => setDraft(null) });
            }}
          >
            Save
          </Button>
          <Button
            disabled={disabled || !canEdit || hasUnsavedChanges}
            onClick={() => apply.mutate()}
          >
            Apply
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}

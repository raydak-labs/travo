import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { EmptyState } from '@/components/ui/empty-state';
import { Skeleton } from '@/components/ui/skeleton';
import { Switch } from '@/components/ui/switch';
import { useAPConfigs } from '@/hooks/use-wifi';
import { APRadioSection } from './ap-radio-section';
import { APUnifiedConfigForm } from './ap-unified-config-form';

export function APConfigCard() {
  const { data: apConfigs, isLoading: apLoading } = useAPConfigs();
  const [sectionOverrides, setSectionOverrides] = useState<Record<string, boolean>>({});
  const [separatePerRadio, setSeparatePerRadio] = useState(false);
  const serverEnabledRef = useRef<Record<string, boolean>>({});

  useEffect(() => {
    if (!apConfigs?.length) return;
    setSectionOverrides((prev) => {
      const next = { ...prev };
      let changed = false;
      for (const ap of apConfigs) {
        const prevSrv = serverEnabledRef.current[ap.section];
        if (prevSrv !== undefined && prevSrv !== ap.enabled && ap.section in next) {
          delete next[ap.section];
          changed = true;
        }
      }
      return changed ? next : prev;
    });
    for (const ap of apConfigs) {
      serverEnabledRef.current[ap.section] = ap.enabled;
    }
  }, [apConfigs]);

  const handleEnabledChange = useCallback((section: string, enabled: boolean) => {
    setSectionOverrides((prev) => ({ ...prev, [section]: enabled }));
  }, []);

  const enabledBySection = useMemo(() => {
    if (!apConfigs) return {};
    const m: Record<string, boolean> = {};
    for (const a of apConfigs) {
      m[a.section] = sectionOverrides[a.section] ?? a.enabled;
    }
    return m;
  }, [apConfigs, sectionOverrides]);

  const activeEnabledCount = useMemo(() => {
    if (!apConfigs) return 0;
    return apConfigs.filter((a) => enabledBySection[a.section] ?? a.enabled).length;
  }, [apConfigs, enabledBySection]);

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm font-medium">Access Point Configuration</CardTitle>
      </CardHeader>
      <CardContent>
        {apLoading ? (
          <div className="space-y-2">
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-10 w-full" />
          </div>
        ) : !apConfigs || apConfigs.length === 0 ? (
          <EmptyState message="No access point radios detected" />
        ) : apConfigs.length === 1 ? (
          <APRadioSection
            ap={apConfigs[0]!}
            activeEnabledCount={activeEnabledCount}
            onEnabledChange={handleEnabledChange}
          />
        ) : (
          <div className="space-y-6">
            <div className="flex items-center justify-between rounded-lg border p-3">
              <div className="space-y-0.5">
                <span className="text-sm font-medium text-gray-900 dark:text-white">
                  Different settings per radio
                </span>
                <p className="text-xs text-gray-500 dark:text-gray-400">
                  Off: one network name and password for all bands. On: configure 2.4 GHz and 5 GHz
                  separately.
                </p>
              </div>
              <Switch
                id="ap-separate-per-radio"
                aria-label="Different settings per radio"
                checked={separatePerRadio}
                onChange={(e) => setSeparatePerRadio(e.target.checked)}
              />
            </div>
            {separatePerRadio ? (
              <div className="space-y-6">
                {apConfigs.map((ap) => (
                  <APRadioSection
                    key={ap.section}
                    ap={ap}
                    activeEnabledCount={activeEnabledCount}
                    onEnabledChange={handleEnabledChange}
                  />
                ))}
              </div>
            ) : (
              <APUnifiedConfigForm
                apConfigs={apConfigs}
                enabledBySection={enabledBySection}
                activeEnabledCount={activeEnabledCount}
                onEnabledChange={handleEnabledChange}
              />
            )}
          </div>
        )}
      </CardContent>
    </Card>
  );
}

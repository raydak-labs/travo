import { useCallback, useEffect, useMemo, useState } from 'react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { EmptyState } from '@/components/ui/empty-state';
import { Skeleton } from '@/components/ui/skeleton';
import { useAPConfigs } from '@/hooks/use-wifi';
import { APRadioSection } from './ap-radio-section';

export function APConfigCard() {
  const { data: apConfigs, isLoading: apLoading } = useAPConfigs();
  const [enabledBySection, setEnabledBySection] = useState<Record<string, boolean>>({});

  const handleEnabledChange = useCallback((section: string, enabled: boolean) => {
    setEnabledBySection((prev) => ({ ...prev, [section]: enabled }));
  }, []);

  useEffect(() => {
    if (apConfigs) {
      const m: Record<string, boolean> = {};
      for (const a of apConfigs) {
        m[a.section] = a.enabled;
      }
      setEnabledBySection(m);
    }
  }, [apConfigs]);

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
        ) : (
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
        )}
      </CardContent>
    </Card>
  );
}

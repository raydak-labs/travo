import { useState } from 'react';
import { ExternalLink, FileEdit } from 'lucide-react';
import { toast } from 'sonner';
import { Card, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { QueryCard } from '@/components/ui/query-card';
import { SectionHeading } from '@/components/ui/section-heading';
import { Skeleton } from '@/components/ui/skeleton';
import { useServices, useAdGuardConfig, useSetAdGuardConfig } from '@/hooks/use-services';
import { AdGuardConfigEditorDialog } from '@/pages/system/adguard-config-editor-dialog';

export function SystemQuickLinksCard() {
  const servicesQuery = useServices();
  const services = servicesQuery.data ?? [];
  const adguardConfigQuery = useAdGuardConfig();
  const setAdGuardConfig = useSetAdGuardConfig();
  const [configEditorOpen, setConfigEditorOpen] = useState(false);
  const [configContent, setConfigContent] = useState('');

  const adguardRunning = services.some((s) => s.id === 'adguardhome' && s.state === 'running');
  const adguardInstalled = services.some(
    (s) => s.id === 'adguardhome' && s.state !== 'not_installed',
  );

  const handleOpenConfigEditor = async () => {
    const result = await adguardConfigQuery.refetch();
    if (result.data) {
      setConfigContent(result.data.content);
      setConfigEditorOpen(true);
    } else if (result.error) {
      toast.error(
        result.error instanceof Error
          ? result.error.message
          : 'Failed to load AdGuard configuration',
      );
    }
  };

  const handleSaveConfig = () => {
    setAdGuardConfig.mutate(configContent, {
      onSuccess: () => setConfigEditorOpen(false),
    });
  };

  return (
    <>
      <div>
        <SectionHeading>Quick Links</SectionHeading>
        <Card>
          <CardContent className="pt-4">
            <div className="flex flex-wrap gap-2">
              <Button size="sm" variant="outline" asChild>
                <a
                  href={`http://${window.location.hostname}:8080/cgi-bin/luci`}
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  <ExternalLink className="h-3.5 w-3.5" />
                  LuCI Web Interface
                </a>
              </Button>
              {/* The AdGuard links exist only because the service list said so.
                  A failed list must not read as "AdGuard is not installed". */}
              <QueryCard
                isLoading={servicesQuery.isLoading}
                isError={servicesQuery.isError}
                error={servicesQuery.error}
                onRetry={() => void servicesQuery.refetch()}
                loading={<Skeleton className="h-8 w-40" />}
              >
                {adguardRunning ? (
                  <Button size="sm" variant="outline" asChild>
                    <a
                      href={`http://${window.location.hostname}:3000`}
                      target="_blank"
                      rel="noopener noreferrer"
                    >
                      <ExternalLink className="h-3.5 w-3.5" />
                      AdGuard Dashboard
                    </a>
                  </Button>
                ) : null}
                {adguardInstalled ? (
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={handleOpenConfigEditor}
                    disabled={adguardConfigQuery.isFetching}
                  >
                    <FileEdit className="h-3.5 w-3.5" />
                    {adguardConfigQuery.isFetching ? 'Loading…' : 'AdGuard Config'}
                  </Button>
                ) : null}
              </QueryCard>
            </div>
          </CardContent>
        </Card>
      </div>

      <AdGuardConfigEditorDialog
        open={configEditorOpen}
        onOpenChange={setConfigEditorOpen}
        configContent={configContent}
        onConfigContentChange={setConfigContent}
        onSave={handleSaveConfig}
        savePending={setAdGuardConfig.isPending}
      />
    </>
  );
}

import { useRef, useState } from 'react';
import { Download, Upload } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { ConfirmDialog } from '@/components/ui/confirm-dialog';
import { useBackup, useRestore } from '@/hooks/use-system';

export function SystemBackupRestoreCard() {
  const backup = useBackup();
  const restore = useRestore();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [showRestoreDialog, setShowRestoreDialog] = useState(false);
  const [pendingRestoreFile, setPendingRestoreFile] = useState<File | null>(null);

  return (
    <>
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">Backup & Restore</CardTitle>
          <Download className="h-4 w-4 text-gray-500" />
        </CardHeader>
        <CardContent>
          <div className="space-y-3">
            <Button
              variant="outline"
              size="sm"
              onClick={() => backup.mutate()}
              disabled={backup.isPending}
            >
              <Download className="mr-2 h-4 w-4" />
              {backup.isPending ? 'Creating backup…' : 'Download Backup'}
            </Button>
            <div>
              <input
                type="file"
                ref={fileInputRef}
                accept=".tar.gz,.gz"
                className="hidden"
                onChange={(e) => {
                  const file = e.target.files?.[0];
                  if (file) {
                    setPendingRestoreFile(file);
                    setShowRestoreDialog(true);
                    e.target.value = '';
                  }
                }}
              />
              <Button
                variant="outline"
                size="sm"
                onClick={() => fileInputRef.current?.click()}
                disabled={restore.isPending}
              >
                <Upload className="mr-2 h-4 w-4" />
                {restore.isPending ? 'Restoring…' : 'Restore from Backup'}
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>

      <ConfirmDialog
        open={showRestoreDialog}
        onOpenChange={(open) => {
          setShowRestoreDialog(open);
          if (!open) setPendingRestoreFile(null);
        }}
        title="Restore from Backup"
        description="Current configuration will be overwritten. A reboot will be needed to apply changes."
        warningText="This will replace all your current settings with the backup file."
        confirmLabel="Restore"
        isPending={restore.isPending}
        onConfirm={() => {
          if (pendingRestoreFile) {
            restore.mutate(pendingRestoreFile);
            setShowRestoreDialog(false);
            setPendingRestoreFile(null);
          }
        }}
      />
    </>
  );
}

import { useNavigate } from '@tanstack/react-router';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog';

type WireguardPostInstallDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
};

export function WireguardPostInstallDialog({
  open,
  onOpenChange,
}: WireguardPostInstallDialogProps) {
  const navigate = useNavigate();

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>WireGuard Installed!</DialogTitle>
        </DialogHeader>
        <p className="text-sm text-gray-700 dark:text-gray-300">
          WireGuard has been installed. Would you like to set up a VPN configuration now?
        </p>
        <DialogFooter className="flex-col gap-2 sm:flex-row">
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Later
          </Button>
          <Button
            onClick={() => {
              onOpenChange(false);
              void navigate({ to: '/vpn' });
            }}
          >
            Import .conf File
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

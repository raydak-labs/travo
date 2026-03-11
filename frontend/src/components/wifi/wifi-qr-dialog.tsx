import { QRCodeSVG } from 'qrcode.react';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import type { APConfig } from '@shared/index';

interface WifiQRDialogProps {
  readonly open: boolean;
  readonly onOpenChange: (open: boolean) => void;
  readonly ap: APConfig | null;
}

function escapeSpecial(value: string): string {
  return value.replace(/[\\;,":.]/g, '\\$&');
}

function getWifiQRString(ap: APConfig): string {
  const authType = ap.encryption === 'none' ? 'nopass' : 'WPA';
  const ssid = escapeSpecial(ap.ssid);
  const password = ap.encryption === 'none' ? '' : escapeSpecial(ap.key);
  return `WIFI:T:${authType};S:${ssid};P:${password};;`;
}

export function WifiQRDialog({ open, onOpenChange, ap }: WifiQRDialogProps) {
  if (!ap) return null;

  const bandLabel = ap.band === '2g' ? '2.4 GHz' : ap.band === '5g' ? '5 GHz' : ap.band;
  const encLabel = ap.encryption === 'none' ? 'Open' : ap.encryption.toUpperCase();

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Share WiFi — {ap.ssid}</DialogTitle>
        </DialogHeader>
        <div className="flex flex-col items-center space-y-4 py-4">
          <div className="rounded-lg bg-white p-4">
            <QRCodeSVG value={getWifiQRString(ap)} size={200} level="M" />
          </div>
          <p className="text-center text-sm text-gray-500">
            Scan this QR code to connect to <strong>{ap.ssid}</strong>
          </p>
          <div className="text-center text-xs text-gray-400">
            {bandLabel} · {encLabel}
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}

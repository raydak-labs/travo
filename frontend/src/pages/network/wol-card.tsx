import { useState } from 'react';
import { Zap } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { useSendWoL } from '@/hooks/use-network';

export function WoLCard() {
  const [mac, setMac] = useState('');
  const [iface, setIface] = useState('br-lan');
  const sendWoL = useSendWoL();

  function handleSend() {
    if (!mac.trim()) return;
    sendWoL.mutate({ mac: mac.trim(), interface: iface.trim() || 'br-lan' });
  }

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">Wake-on-LAN</CardTitle>
        <Zap className="h-4 w-4 text-gray-500" />
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="space-y-1">
          <label className="text-xs text-gray-500">MAC Address</label>
          <Input
            value={mac}
            onChange={(e) => setMac(e.target.value)}
            placeholder="AA:BB:CC:DD:EE:FF"
            className="font-mono"
          />
        </div>
        <div className="space-y-1">
          <label className="text-xs text-gray-500">Interface (optional)</label>
          <Input
            value={iface}
            onChange={(e) => setIface(e.target.value)}
            placeholder="br-lan"
          />
        </div>
        <Button
          size="sm"
          onClick={handleSend}
          disabled={sendWoL.isPending || !mac.trim()}
        >
          <Zap className="mr-1.5 h-3.5 w-3.5" />
          {sendWoL.isPending ? 'Sending…' : 'Send Magic Packet'}
        </Button>
      </CardContent>
    </Card>
  );
}

import { useState, useEffect } from 'react';
import { Shuffle, RotateCcw } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Skeleton } from '@/components/ui/skeleton';
import { useMACAddresses, useSetMAC, useRandomizeMAC } from '@/hooks/use-wifi';

const MAC_REGEX = /^[0-9a-fA-F]{2}(:[0-9a-fA-F]{2}){5}$/;

function generateRandomMAC(): string {
  const hex = () =>
    Math.floor(Math.random() * 256)
      .toString(16)
      .padStart(2, '0');
  const first = (Math.floor(Math.random() * 256) & 0xfe) | 0x02; // locally administered, unicast
  return [first.toString(16).padStart(2, '0'), hex(), hex(), hex(), hex(), hex()].join(':');
}

export function MACAddressCard() {
  const { data: macAddresses, isLoading: macLoading } = useMACAddresses();
  const setMAC = useSetMAC();
  const randomizeMAC = useRandomizeMAC();
  const [customMAC, setCustomMAC] = useState('');

  useEffect(() => {
    if (macAddresses && macAddresses.length > 0) {
      setCustomMAC(macAddresses[0].custom_mac || '');
    }
  }, [macAddresses]);

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm font-medium">MAC Address Cloning</CardTitle>
      </CardHeader>
      <CardContent>
        {macLoading ? (
          <div className="space-y-2">
            <Skeleton className="h-4 w-3/4" />
            <Skeleton className="h-10 w-full" />
          </div>
        ) : !macAddresses || macAddresses.length === 0 ? (
          <p className="text-sm text-gray-500">No STA interface detected</p>
        ) : (
          <div className="space-y-4">
            {macAddresses.map((mac) => (
              <div key={mac.interface} className="space-y-3">
                <div className="flex items-center gap-2">
                  <Badge variant="outline">STA Interface</Badge>
                  {mac.current_mac && (
                    <span className="text-sm font-mono text-gray-600 dark:text-gray-400">
                      Current: {mac.current_mac}
                    </span>
                  )}
                </div>
                {mac.custom_mac && (
                  <p className="text-xs text-amber-600 dark:text-amber-400">
                    Custom MAC active: {mac.custom_mac}
                  </p>
                )}
                <div className="space-y-2">
                  <label
                    htmlFor="mac-input"
                    className="text-xs font-medium text-gray-600 dark:text-gray-400"
                  >
                    Custom MAC Address
                  </label>
                  <Input
                    id="mac-input"
                    value={customMAC}
                    onChange={(e) => setCustomMAC(e.target.value)}
                    placeholder="AA:BB:CC:DD:EE:FF"
                    className="font-mono"
                  />
                </div>
                <div className="flex flex-wrap gap-2">
                  <Button size="sm" onClick={() => setCustomMAC(generateRandomMAC())}>
                    <Shuffle className="h-4 w-4 mr-1" />
                    Random
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    disabled={randomizeMAC.isPending}
                    onClick={() => randomizeMAC.mutate()}
                  >
                    <Shuffle className="h-4 w-4 mr-1" />
                    {randomizeMAC.isPending ? 'Randomizing...' : 'Randomize & Apply'}
                  </Button>
                  <Button
                    size="sm"
                    disabled={setMAC.isPending || (customMAC !== '' && !MAC_REGEX.test(customMAC))}
                    onClick={() => setMAC.mutate(customMAC)}
                  >
                    {setMAC.isPending ? 'Applying...' : 'Apply'}
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={setMAC.isPending}
                    onClick={() => {
                      setCustomMAC('');
                      setMAC.mutate('');
                    }}
                  >
                    <RotateCcw className="h-4 w-4 mr-1" />
                    Reset to Default
                  </Button>
                </div>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}

import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Zap } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { useSendWoL } from '@/hooks/use-network';
import { wolFormSchema, type WolFormValues } from '@/lib/schemas/network-forms';

export function WoLCard() {
  const sendWoL = useSendWoL();

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<WolFormValues>({
    resolver: zodResolver(wolFormSchema),
    defaultValues: { mac: '', interface: 'br-lan' },
    mode: 'onChange',
  });

  const onSubmit = (data: WolFormValues) => {
    sendWoL.mutate({
      mac: data.mac.trim(),
      interface: data.interface.trim() || 'br-lan',
    });
  };

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">Wake-on-LAN</CardTitle>
        <Zap className="h-4 w-4 text-gray-500" />
      </CardHeader>
      <CardContent>
        <form onSubmit={handleSubmit(onSubmit)} className="space-y-3" noValidate>
          <div className="space-y-1">
            <label className="text-xs text-gray-500">MAC Address</label>
            <Input
              className="font-mono"
              placeholder="AA:BB:CC:DD:EE:FF"
              aria-invalid={errors.mac ? 'true' : undefined}
              aria-describedby={errors.mac ? 'wol-mac-err' : undefined}
              {...register('mac')}
            />
            {errors.mac ? (
              <p id="wol-mac-err" className="text-xs text-red-500" role="alert">
                {errors.mac.message}
              </p>
            ) : null}
          </div>
          <div className="space-y-1">
            <label className="text-xs text-gray-500">Interface (optional)</label>
            <Input placeholder="br-lan" {...register('interface')} />
          </div>
          <Button type="submit" size="sm" disabled={sendWoL.isPending}>
            <Zap className="mr-1.5 h-3.5 w-3.5" />
            {sendWoL.isPending ? 'Sending…' : 'Send Magic Packet'}
          </Button>
        </form>
      </CardContent>
    </Card>
  );
}

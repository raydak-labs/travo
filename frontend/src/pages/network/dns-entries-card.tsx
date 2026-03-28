import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { MapPin, Trash2 } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Skeleton } from '@/components/ui/skeleton';
import { useDNSEntries, useAddDNSEntry, useDeleteDNSEntry } from '@/hooks/use-network';
import { dnsEntryFormSchema, type DnsEntryFormValues } from '@/lib/schemas/network-forms';

export function DnsEntriesCard() {
  const { data: dnsEntries, isLoading: dnsEntriesLoading } = useDNSEntries();
  const addDNSEntry = useAddDNSEntry();
  const deleteDNSEntry = useDeleteDNSEntry();

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<DnsEntryFormValues>({
    resolver: zodResolver(dnsEntryFormSchema),
    defaultValues: { name: '', ip: '' },
    mode: 'onChange',
  });

  const onAdd = (data: DnsEntryFormValues) => {
    addDNSEntry.mutate(
      { name: data.name.trim(), ip: data.ip.trim() },
      {
        onSuccess: () => reset({ name: '', ip: '' }),
      },
    );
  };

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">Local DNS Entries</CardTitle>
        <MapPin className="h-4 w-4 text-gray-500" />
      </CardHeader>
      <CardContent>
        {dnsEntriesLoading ? (
          <div className="space-y-2">
            <Skeleton className="h-8 w-full" />
            <Skeleton className="h-8 w-full" />
          </div>
        ) : (
          <div className="space-y-4">
            {dnsEntries && dnsEntries.length > 0 && (
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b text-left text-gray-500">
                      <th className="pb-2 font-medium">Hostname</th>
                      <th className="pb-2 font-medium">IP Address</th>
                      <th className="w-16 pb-2 font-medium"></th>
                    </tr>
                  </thead>
                  <tbody>
                    {dnsEntries.map((entry) => (
                      <tr key={entry.section} className="border-b last:border-0">
                        <td className="py-2 text-gray-900 dark:text-white">{entry.name}</td>
                        <td className="py-2 font-mono text-gray-900 dark:text-white">{entry.ip}</td>
                        <td className="py-2 text-right">
                          <Button
                            variant="ghost"
                            size="sm"
                            type="button"
                            onClick={() => entry.section && deleteDNSEntry.mutate(entry.section)}
                            disabled={deleteDNSEntry.isPending}
                          >
                            <Trash2 className="h-4 w-4 text-red-500" />
                          </Button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
            <form
              onSubmit={handleSubmit(onAdd)}
              className="grid grid-cols-1 items-end gap-2 sm:grid-cols-[1fr_1fr_auto]"
              noValidate
            >
              <div className="space-y-1">
                <label className="text-xs text-gray-500">Hostname</label>
                <Input
                  placeholder="myserver"
                  aria-invalid={errors.name ? 'true' : undefined}
                  aria-describedby={errors.name ? 'dns-entry-name-err' : undefined}
                  {...register('name')}
                />
                {errors.name ? (
                  <p id="dns-entry-name-err" className="text-xs text-red-500" role="alert">
                    {errors.name.message}
                  </p>
                ) : null}
              </div>
              <div className="space-y-1">
                <label className="text-xs text-gray-500">IP Address</label>
                <Input
                  placeholder="192.168.8.10"
                  className="font-mono"
                  aria-invalid={errors.ip ? 'true' : undefined}
                  aria-describedby={errors.ip ? 'dns-entry-ip-err' : undefined}
                  {...register('ip')}
                />
                {errors.ip ? (
                  <p id="dns-entry-ip-err" className="text-xs text-red-500" role="alert">
                    {errors.ip.message}
                  </p>
                ) : null}
              </div>
              <Button type="submit" size="sm" disabled={addDNSEntry.isPending}>
                {addDNSEntry.isPending ? 'Adding…' : 'Add'}
              </Button>
            </form>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

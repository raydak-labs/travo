import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { MapPin, Trash2 } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { QueryCard } from '@/components/ui/query-card';
import { Skeleton } from '@/components/ui/skeleton';
import { Label } from '@/components/ui/label';
import { FieldError } from '@/components/ui/field-error';
import { useDNSEntries, useAddDNSEntry, useDeleteDNSEntry } from '@/hooks/use-network';
import { dnsEntryFormSchema, type DnsEntryFormValues } from '@/lib/schemas/network-forms';

export function DnsEntriesCard() {
  const {
    data: dnsEntries,
    isLoading: dnsEntriesLoading,
    isError: entriesFailed,
    error: entriesError,
    refetch: refetchEntries,
  } = useDNSEntries();
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
        <CardTitle>Local DNS Entries</CardTitle>
        <MapPin className="h-4 w-4 text-gray-500 dark:text-gray-400" />
      </CardHeader>
      <CardContent>
        <QueryCard
          isLoading={dnsEntriesLoading}
          isError={entriesFailed}
          error={entriesError}
          onRetry={() => void refetchEntries()}
          loading={
            <div className="space-y-2">
              <Skeleton className="h-8 w-full" />
              <Skeleton className="h-8 w-full" />
            </div>
          }
        >
          <div className="space-y-4">
            {dnsEntries && dnsEntries.length > 0 && (
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b text-left text-gray-500 dark:text-gray-400">
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
                            aria-label={`Delete DNS entry ${entry.name}`}
                            onClick={() => entry.section && deleteDNSEntry.mutate(entry.section)}
                            disabled={deleteDNSEntry.isPending}
                          >
                            <Trash2 className="h-4 w-4 text-[var(--status-danger-text)]" />
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
                <Label htmlFor="dns-entry-name">Hostname</Label>
                <Input
                  id="dns-entry-name"
                  placeholder="myserver"
                  aria-invalid={errors.name ? 'true' : undefined}
                  aria-describedby={errors.name ? 'dns-entry-name-err' : undefined}
                  {...register('name')}
                />
                {errors.name ? (
                  <FieldError id="dns-entry-name-err">{errors.name.message}</FieldError>
                ) : null}
              </div>
              <div className="space-y-1">
                <Label htmlFor="dns-entry-ip">IP Address</Label>
                <Input
                  id="dns-entry-ip"
                  placeholder="192.168.8.10"
                  className="font-mono"
                  aria-invalid={errors.ip ? 'true' : undefined}
                  aria-describedby={errors.ip ? 'dns-entry-ip-err' : undefined}
                  {...register('ip')}
                />
                {errors.ip ? (
                  <FieldError id="dns-entry-ip-err">{errors.ip.message}</FieldError>
                ) : null}
              </div>
              <Button type="submit" disabled={addDNSEntry.isPending}>
                {addDNSEntry.isPending ? 'Adding…' : 'Add'}
              </Button>
            </form>
          </div>
        </QueryCard>
      </CardContent>
    </Card>
  );
}

import { useEffect, useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Pencil } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { useSetHostname } from '@/hooks/use-system';
import { hostnameFormSchema, type HostnameFormValues } from '@/lib/schemas/system-forms';

export function HostnameInlineForm({
  hostname,
  onUpdated,
}: {
  hostname: string;
  onUpdated: () => void;
}) {
  const [editing, setEditing] = useState(false);
  const mutation = useSetHostname();

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<HostnameFormValues>({
    resolver: zodResolver(hostnameFormSchema),
    defaultValues: { hostname },
  });

  useEffect(() => {
    if (!editing) {
      reset({ hostname });
    }
  }, [hostname, editing, reset]);

  const onSubmit = (data: HostnameFormValues) => {
    mutation.mutate(
      { hostname: data.hostname.trim() },
      {
        onSuccess: () => {
          setEditing(false);
          onUpdated();
        },
      },
    );
  };

  if (editing) {
    return (
      <form
        className="flex flex-wrap items-center gap-x-1 gap-y-1"
        onSubmit={handleSubmit(onSubmit)}
        noValidate
      >
        <Input
          className="h-6 w-32 text-xs"
          autoFocus
          aria-invalid={!!errors.hostname}
          aria-describedby={errors.hostname ? 'hostname-inline-error' : undefined}
          {...register('hostname')}
        />
        <Button
          type="submit"
          size="sm"
          variant="ghost"
          className="h-6 px-1 text-xs"
          disabled={mutation.isPending}
        >
          Save
        </Button>
        <Button
          type="button"
          size="sm"
          variant="ghost"
          className="h-6 px-1 text-xs"
          onClick={() => {
            reset({ hostname });
            setEditing(false);
          }}
        >
          Cancel
        </Button>
        {errors.hostname ? (
          <span id="hostname-inline-error" className="w-full text-xs text-red-500" role="alert">
            {errors.hostname.message}
          </span>
        ) : null}
      </form>
    );
  }

  return (
    <>
      {hostname}
      <button
        type="button"
        className="ml-1 text-gray-400 hover:text-gray-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 dark:hover:text-gray-300"
        onClick={() => {
          reset({ hostname });
          setEditing(true);
        }}
        aria-label="Edit hostname"
      >
        <Pencil className="h-3 w-3" />
      </button>
    </>
  );
}

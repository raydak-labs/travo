import { useEffect, useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Pencil, Check, X } from 'lucide-react';
import type { Client } from '@shared/index';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { useSetClientAlias } from '@/hooks/use-network';
import { clientAliasFormSchema, type ClientAliasFormValues } from '@/lib/schemas/clients-forms';

export interface ClientAliasCellProps {
  client: Client;
  /** Tailwind classes for the text input (width differs on Network vs Clients page). */
  inputClassName?: string;
  placeholder?: string;
  /** Classes for the resolved display name (e.g. `font-medium` on Clients page). */
  displayNameClassName?: string;
  /** e.g. `group-hover:opacity-100` vs `group-hover/alias:opacity-100` */
  editButtonClassName?: string;
}

export function ClientAliasCell({
  client,
  inputClassName = 'h-7 w-32 text-sm',
  placeholder = 'Alias',
  displayNameClassName = 'text-gray-900 dark:text-white',
  editButtonClassName = 'opacity-0 group-hover:opacity-100',
}: ClientAliasCellProps) {
  const [editing, setEditing] = useState(false);
  const setAlias = useSetClientAlias();

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<ClientAliasFormValues>({
    resolver: zodResolver(clientAliasFormSchema),
    defaultValues: { alias: client.alias ?? '' },
    mode: 'onChange',
  });

  useEffect(() => {
    reset({ alias: client.alias ?? '' });
  }, [client.mac_address, client.alias, reset]);

  const displayName = client.alias || client.hostname || '—';

  const onSubmit = (data: ClientAliasFormValues) => {
    setAlias.mutate(
      { mac: client.mac_address, alias: data.alias.trim() },
      { onSuccess: () => setEditing(false) },
    );
  };

  const onCancel = () => {
    reset({ alias: client.alias ?? '' });
    setEditing(false);
  };

  if (editing) {
    return (
      <form
        onSubmit={handleSubmit(onSubmit)}
        className="flex items-center gap-1"
        noValidate
        onKeyDown={(e) => {
          if (e.key === 'Escape') {
            e.preventDefault();
            onCancel();
          }
        }}
      >
        <div className="flex flex-col gap-0.5">
          <Input
            className={inputClassName}
            placeholder={placeholder}
            autoFocus
            aria-invalid={errors.alias ? 'true' : undefined}
            aria-describedby={errors.alias ? `alias-err-${client.mac_address}` : undefined}
            {...register('alias')}
          />
          {errors.alias ? (
            <span id={`alias-err-${client.mac_address}`} className="text-xs text-red-500" role="alert">
              {errors.alias.message}
            </span>
          ) : null}
        </div>
        <Button variant="ghost" size="icon" className="h-6 w-6 self-start" type="submit" disabled={setAlias.isPending}>
          <Check className="h-3 w-3" />
        </Button>
        <Button variant="ghost" size="icon" className="h-6 w-6 self-start" type="button" onClick={onCancel}>
          <X className="h-3 w-3" />
        </Button>
      </form>
    );
  }

  return (
    <div className="flex items-center gap-1">
      <div>
        <span className={displayNameClassName}>{displayName}</span>
        {client.alias && client.hostname && (
          <span className="ml-1 text-xs text-gray-400">({client.hostname})</span>
        )}
      </div>
      <Button
        variant="ghost"
        size="icon"
        className={`h-6 w-6 ${editButtonClassName}`}
        type="button"
        title="Edit alias"
        onClick={() => setEditing(true)}
      >
        <Pencil className="h-3 w-3" />
      </Button>
    </div>
  );
}

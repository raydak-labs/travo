import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import type { AddPortForwardRequest } from '@shared/index';
import { portForwardFormSchema, type PortForwardFormValues } from '@/lib/schemas/network-forms';
import { FirewallPortForwardAddFormGrid } from './firewall-port-forward-add-form-grid';

type FirewallPortForwardAddFormProps = {
  addRule: {
    mutate: (payload: AddPortForwardRequest, opts?: { onSuccess?: () => void }) => void;
    isPending: boolean;
  };
};

const emptyDefaults: PortForwardFormValues = {
  name: '',
  protocol: 'tcp',
  src_dport: '',
  dest_ip: '',
  dest_port: '',
};

export function FirewallPortForwardAddForm({ addRule }: FirewallPortForwardAddFormProps) {
  const {
    register,
    handleSubmit,
    control,
    reset,
    formState: { errors },
  } = useForm<PortForwardFormValues>({
    resolver: zodResolver(portForwardFormSchema),
    defaultValues: emptyDefaults,
    mode: 'onChange',
  });

  const onAdd = (data: PortForwardFormValues) => {
    addRule.mutate(
      {
        name: data.name.trim(),
        protocol: data.protocol,
        src_dport: data.src_dport.trim(),
        dest_ip: data.dest_ip.trim(),
        dest_port: data.dest_port.trim(),
        enabled: true,
      },
      {
        onSuccess: () => reset(emptyDefaults),
      },
    );
  };

  return (
    <form
      onSubmit={handleSubmit(onAdd)}
      className="space-y-2"
      noValidate
      aria-label="Add port forwarding rule"
    >
      <p className="text-xs text-gray-500">Add port forwarding rule</p>
      <FirewallPortForwardAddFormGrid
        register={register}
        control={control}
        errors={errors}
        isPending={addRule.isPending}
      />
    </form>
  );
}

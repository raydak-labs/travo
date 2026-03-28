import { useEffect } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import type { DataBudget } from '@shared/index';
import { dataBudgetFormSchema, type DataBudgetFormValues } from '@/lib/schemas/network-forms';

type DataUsageBudgetEditorProps = {
  ifaceName: string;
  current?: DataBudget;
  onSave: (b: DataBudget) => void;
};

export function DataUsageBudgetEditor({ ifaceName, current, onSave }: DataUsageBudgetEditorProps) {
  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<DataBudgetFormValues>({
    resolver: zodResolver(dataBudgetFormSchema),
    defaultValues: {
      limit_gb: current ? String(Math.round(current.monthly_limit_bytes / 1e9)) : '',
      warning_threshold_pct: current ? String(current.warning_threshold_pct) : '80',
    },
    mode: 'onChange',
  });

  useEffect(() => {
    reset({
      limit_gb: current ? String(Math.round(current.monthly_limit_bytes / 1e9)) : '',
      warning_threshold_pct: current ? String(current.warning_threshold_pct) : '80',
    });
  }, [current, ifaceName, reset]);

  const onSubmit = (data: DataBudgetFormValues) => {
    onSave({
      interface: ifaceName,
      monthly_limit_bytes: parseFloat(data.limit_gb) * 1e9,
      warning_threshold_pct: parseFloat(data.warning_threshold_pct),
      reset_day: 1,
    });
  };

  return (
    <form onSubmit={handleSubmit(onSubmit)} className="flex flex-wrap items-end gap-2" noValidate>
      <div className="flex flex-col gap-0.5">
        <Input
          className="h-7 w-24 text-sm"
          placeholder="Limit (GB)"
          aria-invalid={errors.limit_gb ? 'true' : undefined}
          aria-describedby={errors.limit_gb ? `budget-limit-${ifaceName}` : undefined}
          {...register('limit_gb')}
        />
        {errors.limit_gb ? (
          <span id={`budget-limit-${ifaceName}`} className="text-xs text-red-500" role="alert">
            {errors.limit_gb.message}
          </span>
        ) : null}
      </div>
      <span className="pb-2 text-xs text-gray-500">GB/month, warn at</span>
      <div className="flex flex-col gap-0.5">
        <Input
          className="h-7 w-16 text-sm"
          placeholder="80"
          aria-invalid={errors.warning_threshold_pct ? 'true' : undefined}
          aria-describedby={errors.warning_threshold_pct ? `budget-warn-${ifaceName}` : undefined}
          {...register('warning_threshold_pct')}
        />
        {errors.warning_threshold_pct ? (
          <span id={`budget-warn-${ifaceName}`} className="text-xs text-red-500" role="alert">
            {errors.warning_threshold_pct.message}
          </span>
        ) : null}
      </div>
      <span className="pb-2 text-xs text-gray-500">%</span>
      <Button type="submit" size="sm" className="h-7 text-xs">
        Save
      </Button>
    </form>
  );
}

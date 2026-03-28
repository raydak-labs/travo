import { useEffect, useState } from 'react';
import { useFieldArray, useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { ToggleLeft } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { useHardwareButtons, useSetButtonActions } from '@/hooks/use-system';
import {
  hardwareButtonsFormSchema,
  type HardwareButtonsFormValues,
} from '@/lib/schemas/system-forms';
import { HardwareButtonsSummaryView } from './hardware-buttons-summary-view';
import { HardwareButtonsEditForm } from './hardware-buttons-edit-form';

export function HardwareButtonsCard() {
  const { data: hardwareButtons = [] } = useHardwareButtons();
  const setButtonActions = useSetButtonActions();
  const [isEditing, setIsEditing] = useState(false);

  const { control, handleSubmit, reset } = useForm<HardwareButtonsFormValues>({
    resolver: zodResolver(hardwareButtonsFormSchema),
    defaultValues: { buttons: [] },
  });

  const { fields } = useFieldArray({
    control,
    name: 'buttons',
  });

  useEffect(() => {
    if (hardwareButtons.length > 0 && !isEditing) {
      reset({
        buttons: hardwareButtons.map((b) => ({ name: b.name, action: b.action })),
      });
    }
  }, [hardwareButtons, isEditing, reset]);

  const openEdit = () => {
    reset({
      buttons: hardwareButtons.map((b) => ({ name: b.name, action: b.action })),
    });
    setIsEditing(true);
  };

  const onCancel = () => {
    reset({
      buttons: hardwareButtons.map((b) => ({ name: b.name, action: b.action })),
    });
    setIsEditing(false);
  };

  const onSubmit = (data: HardwareButtonsFormValues) => {
    setButtonActions.mutate(
      { buttons: data.buttons.map((b) => ({ name: b.name, action: b.action })) },
      {
        onSuccess: () => setIsEditing(false),
      },
    );
  };

  if (hardwareButtons.length === 0) return null;

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">Hardware Buttons</CardTitle>
        <ToggleLeft className="h-4 w-4 text-gray-500" />
      </CardHeader>

      <CardContent className="space-y-4">
        <p className="text-xs text-gray-500">Configure what each physical button does when pressed.</p>

        {!isEditing ? (
          <HardwareButtonsSummaryView
            buttons={hardwareButtons}
            onEdit={openEdit}
            editDisabled={setButtonActions.isPending}
          />
        ) : (
          <HardwareButtonsEditForm
            fields={fields}
            control={control}
            handleSubmit={handleSubmit}
            onValidSubmit={onSubmit}
            onCancel={onCancel}
            savePending={setButtonActions.isPending}
          />
        )}
      </CardContent>
    </Card>
  );
}

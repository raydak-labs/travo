import type { Control, FieldErrors, UseFormRegister, UseFormSetValue } from 'react-hook-form';
import type { APConfig } from '@shared/index';
import type { APRadioFormValues } from '@/lib/schemas/wifi-forms';
import { ApRadioFormCredentialsAndActions } from '@/pages/wifi/ap-radio-form-credentials-and-actions';
import { ApRadioFormHeaderRow } from '@/pages/wifi/ap-radio-form-header-row';

export type ApRadioFormFieldsProps = {
  ap: APConfig;
  bandLabel: string;
  register: UseFormRegister<APRadioFormValues>;
  control: Control<APRadioFormValues>;
  errors: FieldErrors<APRadioFormValues>;
  encryption: string;
  setValue: UseFormSetValue<APRadioFormValues>;
  savePending: boolean;
  onOpenQr: () => void;
};

export function ApRadioFormFields({
  ap,
  bandLabel,
  register,
  control,
  errors,
  encryption,
  setValue,
  savePending,
  onOpenQr,
}: ApRadioFormFieldsProps) {
  return (
    <>
      <ApRadioFormHeaderRow ap={ap} bandLabel={bandLabel} register={register} />
      <ApRadioFormCredentialsAndActions
        ap={ap}
        register={register}
        control={control}
        errors={errors}
        encryption={encryption}
        setValue={setValue}
        savePending={savePending}
        onOpenQr={onOpenQr}
      />
    </>
  );
}

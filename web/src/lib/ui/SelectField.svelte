<script lang="ts">
import * as Select from '$lib/components/ui/select';

// shadcn Select for a plain list of options. bits-ui treats '' as "nothing selected", so an
// option whose value is '' (e.g. "All levels") travels under a sentinel inside the control.
type Option = { value: string; label: string };
const EMPTY = '__empty__';

let {
  value = $bindable(''),
  options,
  disabled = false,
  class: className = '',
  onchange,
}: {
  value?: string;
  options: Option[];
  disabled?: boolean;
  class?: string;
  onchange?: (value: string) => void;
} = $props();

const inner = (v: string) => (v === '' ? EMPTY : v);
const selected = $derived(options.find((o) => o.value === value));
</script>

<Select.Root
  type="single"
  value={inner(value)}
  onValueChange={(v) => {
    value = v === EMPTY ? '' : v;
    onchange?.(value);
  }}
  {disabled}
>
  <Select.Trigger class={className || 'w-full sm:w-48'}>
    <span data-slot="select-value">{selected?.label ?? ''}</span>
  </Select.Trigger>
  <Select.Content>
    {#each options as option (option.value)}
      <Select.Item value={inner(option.value)} label={option.label}>{option.label}</Select.Item>
    {/each}
  </Select.Content>
</Select.Root>

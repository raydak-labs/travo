<script lang="ts">
import Input from '$lib/components/ui/input/input.svelte';
import Label from '$lib/components/ui/label/label.svelte';

let {
  id,
  label,
  value = $bindable(''),
  autocomplete,
  error = null,
}: {
  id: string;
  label: string;
  value?: string;
  autocomplete: 'current-password' | 'new-password';
  error?: string | null;
} = $props();
</script>

<div class="flex flex-col gap-1">
  <Label for={id} class="text-sm font-medium">{label}</Label>
  <Input
    {id}
    type="password"
    name={id}
    {autocomplete}
    required
    bind:value
    aria-invalid={error ? 'true' : undefined}
    aria-describedby={error ? `${id}-error` : undefined}

  />
  {#if error}
    <p id="{id}-error" class="text-sm text-(--danger)">{error}</p>
  {/if}
</div>

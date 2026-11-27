<script lang="ts" module>
import CircleAlert from '@lucide/svelte/icons/circle-alert';
import CircleCheck from '@lucide/svelte/icons/circle-check';
import Info from '@lucide/svelte/icons/info';
import TriangleAlert from '@lucide/svelte/icons/triangle-alert';

export type NoticeTone = 'info' | 'warn' | 'danger' | 'ok';

const TONES = {
  info: {
    icon: Info,
    role: 'status',
    class: 'border-info/30 bg-[var(--info-bg)]',
    iconClass: 'text-info',
  },
  ok: {
    icon: CircleCheck,
    role: 'status',
    class: 'border-ok/30 bg-[var(--ok-bg)]',
    iconClass: 'text-ok',
  },
  warn: {
    icon: TriangleAlert,
    role: 'alert',
    class: 'border-warn/30 bg-[var(--warn-bg)]',
    iconClass: 'text-warn',
  },
  danger: {
    icon: CircleAlert,
    role: 'alert',
    class: 'border-danger/30 bg-[var(--danger-bg)]',
    iconClass: 'text-danger',
  },
} as const;
</script>

<script lang="ts">
import type { Snippet } from 'svelte';
import type { HTMLAttributes } from 'svelte/elements';
import * as Alert from '$lib/components/ui/alert';
import { cn } from '$lib/utils.js';

let {
  tone,
  title,
  children,
  actions,
  role,
  class: className,
  ...rest
}: Omit<HTMLAttributes<HTMLDivElement>, 'title' | 'children'> & {
  tone: NoticeTone;
  title?: string;
  children: Snippet;
  actions?: Snippet;
} = $props();

const style = $derived(TONES[tone]);
const Icon = $derived(style.icon);
</script>

<Alert.Root
  data-tone={tone}
  role={role ?? style.role}
  class={cn(
    'flex flex-col gap-3 p-4 has-data-[slot=alert-action]:pr-4 sm:flex-row sm:items-center',
    style.class,
    className,
  )}
  {...rest}
>
  <div class="flex min-w-0 flex-1 items-start gap-2.5">
    <Icon aria-hidden="true" class={cn('mt-0.5 size-4 shrink-0', style.iconClass)} />
    <div class="grid min-w-0 flex-1 gap-0.5">
      {#if title}<Alert.Title>{title}</Alert.Title>{/if}
      <Alert.Description class="text-foreground">{@render children()}</Alert.Description>
    </div>
  </div>
  {#if actions}
    <Alert.Action class="static flex shrink-0 flex-wrap items-center gap-2 pl-6.5 sm:pl-0">
      {@render actions()}
    </Alert.Action>
  {/if}
</Alert.Root>

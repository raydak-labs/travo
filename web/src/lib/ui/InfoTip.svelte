<script lang="ts">
import Info from '@lucide/svelte/icons/info';
import Button from '$lib/components/ui/button/button.svelte';
import * as Popover from '$lib/components/ui/popover';
import { m } from '../paraglide/messages.js';

let {
  topic,
  text,
  open = $bindable(false),
}: { topic: string; text: string; open?: boolean } = $props();
</script>

<Popover.Root bind:open>
  <Popover.Trigger>
    {#snippet child({ props })}
      <Button {...props} variant="ghost" size="icon-xs" type="button" class="text-muted-foreground"
        aria-label={m.info_more_about({ topic })}>
        <Info aria-hidden="true" />
      </Button>
    {/snippet}
  </Popover.Trigger>
  <Popover.Content class="w-72 max-w-[calc(100vw-2rem)] text-sm leading-relaxed">{text}</Popover.Content>
</Popover.Root>

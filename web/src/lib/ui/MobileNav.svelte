<script lang="ts">
import type { Snippet } from 'svelte';
import * as Sheet from '$lib/components/ui/sheet';
import { m } from '../paraglide/messages.js';

let {
  children,
  close,
  returnFocus,
}: { children: Snippet; close: () => void; returnFocus: HTMLElement | null } = $props();
let open = $state(true);
</script>

<Sheet.Root bind:open>
  <Sheet.Content id="sidebar" side="left" class="w-60 sm:max-w-none" aria-label={m.nav_label()}
    onOpenAutoFocus={(event) => { event.preventDefault(); document.querySelector<HTMLElement>('[data-slot="sheet-content"] [data-dialog-close]')?.focus(); }}
    onCloseAutoFocus={(event) => { event.preventDefault(); returnFocus?.focus(); close(); }}>
    <div class="px-4 py-3"><Sheet.Title>{m.app_name()}</Sheet.Title></div>
    {@render children()}
  </Sheet.Content>
</Sheet.Root>

<script lang="ts">
import EllipsisVertical from '@lucide/svelte/icons/ellipsis-vertical';
import LogOut from '@lucide/svelte/icons/log-out';
import MoonStar from '@lucide/svelte/icons/moon-star';
import Power from '@lucide/svelte/icons/power';
import Sun from '@lucide/svelte/icons/sun';
import Button from '$lib/components/ui/button/button.svelte';
import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
import logo from '../assets/logo.webp';
import { m } from '../paraglide/messages.js';
import { restart } from '../restart.svelte';
import { theme } from '../theme.svelte';
import ConfirmDialog from './ConfirmDialog.svelte';
import { createConfirm } from './confirm.svelte';

// Device and account actions in the header. Phones get the theme toggle here too; desktop keeps
// it as its own header button. Loaded on demand so the menu code stays out of the first load.
let {
  version,
  mobile = false,
  logout,
}: { version?: string; mobile?: boolean; logout: () => void } = $props();
const confirm = createConfirm();
</script>

<DropdownMenu.Root>
  <DropdownMenu.Trigger>
    {#snippet child({ props })}
      <Button {...props} variant="ghost" size="icon" aria-label={m.menu_open()}>
        <EllipsisVertical aria-hidden="true" size={20} />
      </Button>
    {/snippet}
  </DropdownMenu.Trigger>
  <DropdownMenu.Content align="end" class="w-56">
    <DropdownMenu.Label class="flex items-center gap-2">
      <img src={logo} alt="" class="size-5" />
      <span>{m.app_name()}</span>
      {#if version}<span class="ml-auto font-normal text-muted-foreground">{version}</span>{/if}
    </DropdownMenu.Label>
    <DropdownMenu.Separator />
    {#if mobile}
      <DropdownMenu.Item onSelect={() => theme.toggle()}>
        {#if theme.value === 'dark'}<Sun aria-hidden="true" />{:else}<MoonStar aria-hidden="true" />{/if}
        {m.theme_toggle()}
      </DropdownMenu.Item>
    {/if}
    <DropdownMenu.Item onSelect={logout}>
      <LogOut aria-hidden="true" />
      {m.logout()}
    </DropdownMenu.Item>
    <DropdownMenu.Separator />
    <DropdownMenu.Item variant="destructive" disabled={restart.state !== 'idle'} onSelect={() => void restart.ask(confirm)}>
      <Power aria-hidden="true" />
      {m.reboot_router()}
    </DropdownMenu.Item>
  </DropdownMenu.Content>
</DropdownMenu.Root>

<ConfirmDialog {confirm} />

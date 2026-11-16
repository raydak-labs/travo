<script lang="ts">
import Alert from '$lib/components/ui/alert/alert.svelte';
import Button from '$lib/components/ui/button/button.svelte';
import type { WiFiProfileList, WiFiProfileSummary } from '../api/gen';
import { m } from '../paraglide/messages.js';
import Status from '../ui/Status.svelte';
import { securityLabel } from '../wifi';
import { moveProfile } from '../wifi-profiles';

let {
  list,
  busy = false,
  joinBusy = false,
  reorder,
  forget,
  select,
}: {
  list: WiFiProfileList;
  busy?: boolean;
  joinBusy?: boolean;
  reorder: (ids: string[]) => void;
  forget: (profile: WiFiProfileSummary) => void;
  select: (profile: WiFiProfileSummary) => void;
} = $props();

let confirming = $state<string>();
</script>

{#if list.profiles.length === 0}
  <p class="text-muted-foreground">{m.wifi_saved_empty()}</p>
{:else}
  <ol class="divide-y divide-border" aria-label={m.wifi_saved_title()}>
    {#each list.profiles as profile, index (profile.id)}
      {@const up = moveProfile(list.profiles, index, -1)}
      {@const down = moveProfile(list.profiles, index, 1)}
      {@const connected = profile.id === list.connected_id}
      <li class="py-2">
        <div class="flex flex-wrap items-center gap-2">
          <span class="text-sm text-muted-foreground">{index + 1}.</span>
          <span class="break-all font-medium">{profile.ssid}</span>
          {#if connected}<Status kind="ok">{m.wifi_connected()}</Status>{/if}
          <span class="text-sm text-muted-foreground">{securityLabel(profile.security)}</span>
        </div>
        {#if profile.bssid}
          <p class="break-all text-sm text-muted-foreground">{m.wifi_saved_pinned({ bssid: profile.bssid })}</p>
        {/if}
        <div class="mt-2 flex flex-wrap gap-2">
          <Button variant="outline" size="sm" type="button" disabled={busy || !up}
            aria-label={m.wifi_saved_move_up({ ssid: profile.ssid })}
            onclick={() => up && reorder(up)}>{m.wifi_saved_up()}</Button>
          <Button variant="outline" size="sm" type="button" disabled={busy || !down}
            aria-label={m.wifi_saved_move_down({ ssid: profile.ssid })}
            onclick={() => down && reorder(down)}>{m.wifi_saved_down()}</Button>
          <Button variant="outline" size="sm" type="button" disabled={busy || joinBusy}
            aria-label={m.wifi_saved_select_ssid({ ssid: profile.ssid })}
            onclick={() => select(profile)}>{m.wifi_saved_select()}</Button>
          <Button variant="outline" size="sm" type="button" disabled={busy}
            aria-label={m.wifi_saved_forget_ssid({ ssid: profile.ssid })}
            onclick={() => { confirming = profile.id; }}>{m.wifi_saved_forget()}</Button>
        </div>
        {#if confirming === profile.id}
          <Alert class="mt-2 grid gap-2">
            <p class="font-medium">{m.wifi_saved_forget_confirm({ ssid: profile.ssid })}</p>
            {#if connected}<p class="text-sm">{m.wifi_saved_forget_connected()}</p>{/if}
            <div class="flex gap-2">
              <Button variant="destructive" size="sm" type="button" disabled={busy}
                onclick={() => { confirming = undefined; forget(profile); }}>{m.wifi_saved_forget()}</Button>
              <Button variant="outline" size="sm" type="button"
                onclick={() => { confirming = undefined; }}>{m.wifi_saved_forget_cancel()}</Button>
            </div>
          </Alert>
        {/if}
      </li>
    {/each}
  </ol>
{/if}

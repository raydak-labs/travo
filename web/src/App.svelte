<script lang="ts">
import LayoutDashboard from '@lucide/svelte/icons/layout-dashboard';
import LogOut from '@lucide/svelte/icons/log-out';
import Menu from '@lucide/svelte/icons/menu';
import MoonStar from '@lucide/svelte/icons/moon-star';
import Network from '@lucide/svelte/icons/network';
import Server from '@lucide/svelte/icons/server';
import Settings from '@lucide/svelte/icons/settings';
import Shield from '@lucide/svelte/icons/shield';
import Sun from '@lucide/svelte/icons/sun';
import Wifi from '@lucide/svelte/icons/wifi';
import Button from '$lib/components/ui/button/button.svelte';
import { ownDashboardEvents, restoreDashboardEvents } from './lib/app-dashboard-events';
import { dashboardEvents } from './lib/dashboard-events.svelte';
import { navigate } from './lib/navigate';
import { lazyPage, pages } from './lib/pages/lazy';
import PageHost from './lib/pages/PageHost.svelte';
import { m } from './lib/paraglide/messages.js';
import { guard, href, type Route } from './lib/router';
import { router } from './lib/router.svelte';
import { session } from './lib/session.svelte';
import { theme } from './lib/theme.svelte';
import LoadState from './lib/ui/LoadState.svelte';
import type { LoadStatus } from './lib/ui/loader.svelte';

let drawerOpen = $state(false);
let navChunk = $state<LoadStatus>({ state: 'loading' });
let menuButton: HTMLElement | null = $state(null);
const mobileNav = lazyPage(() => import('./lib/ui/MobileNav.svelte'));
// Below md, mount the modal navigation only while open.
const mobile = $state({ value: false });
$effect(() => {
  const mq = matchMedia('(max-width: 767.98px)');
  const sync = () => {
    mobile.value = mq.matches;
    if (!mq.matches) drawerOpen = false;
  };
  sync();
  mq.addEventListener('change', sync);
  return () => mq.removeEventListener('change', sync);
});

function openDrawer() {
  navChunk = { state: 'loading' };
  drawerOpen = true;
}

function closeDrawer() {
  if (!drawerOpen) return;
  drawerOpen = false;
  queueMicrotask(() => menuButton?.focus());
}

function onNavStatus(status: LoadStatus) {
  navChunk = status;
}

// Escape dismisses until the sheet chunk loads. A failed chunk has no sheet, so the shell draws Close.
function onDrawerKey(event: KeyboardEvent) {
  if (!drawerOpen || navChunk.state === 'ready' || event.key !== 'Escape') return;
  event.preventDefault();
  closeDrawer();
}

// Groups per spec. Sections without a route; the rest render the "not yet" card.
const nav: { route: Route | null; label: () => string; icon: typeof Wifi }[] = [
  { route: 'dashboard', label: () => m.nav_dashboard(), icon: LayoutDashboard },
  { route: 'wifi', label: () => m.nav_wifi(), icon: Wifi },
  { route: null, label: () => m.nav_network(), icon: Network },
  { route: 'vpn', label: () => m.nav_vpn(), icon: Shield },
  { route: null, label: () => m.nav_services(), icon: Server },
  { route: 'system', label: () => m.nav_system(), icon: Settings },
];

void session.bootstrap();

function logout() {
  dashboardEvents.clear();
  // Leave the page first so the guard does not remember it as the place to return to.
  navigate('dashboard');
  void session.logout();
}

const authenticated = $derived(session.state === 'authenticated');
// The shell, not a page, owns the stream. Navigation does not restart it; login,
// expiry and a replacement session after password change each key a new owner.
$effect(() => ownDashboardEvents(authenticated, session.generation, dashboardEvents));
// What is shown: the requested route after the guard. While the state is unknown nothing is shown
// but the loader (or the bootstrap error), so a reload never flashes the wrong page.
const shown = $derived(guard(session.state, router.route));
const bootstrapStatus = $derived(
  session.error === null
    ? ({ state: 'loading' } as const)
    : ({ state: 'error', message: session.error } as const),
);

// Apply the guard to the address bar too, and remember where an expired or anonymous user was going.
// Leaving login/setup after authenticating goes to returnTo here, in one step, so the dashboard
// never mounts in between.
$effect(() => {
  const wanted = router.route;
  if (shown === wanted) return;
  if (shown === 'login') session.returnTo = wanted === 'setup' ? null : wanted;
  if (session.state === 'authenticated' && (wanted === 'login' || wanted === 'setup')) {
    const next = session.returnTo ?? 'dashboard';
    session.returnTo = null;
    navigate(next, { replace: true });
    return;
  }
  navigate(shown, { replace: true });
});

// Close the drawer after navigating.
$effect(() => {
  void router.route;
  drawerOpen = false;
});
</script>

<svelte:window
  onkeydown={onDrawerKey}
  onpagehide={() => dashboardEvents.stop()}
  onpageshow={(e) =>
    restoreDashboardEvents(e.persisted, authenticated, session.generation, dashboardEvents)}
/>

{#snippet navigation()}
    <nav aria-label={m.nav_label()} class="flex-1 overflow-y-auto px-2 pb-4">
      <ul class="flex flex-col gap-1">
        {#each nav as item (item.label())}
          {@const Icon = item.icon}
          <li>
            {#if item.route}
              <Button variant="ghost"
                href={href(item.route)}
                aria-current={shown === item.route ? 'page' : undefined}
                class="w-full justify-start gap-3 text-base font-normal aria-[current=page]:bg-bg aria-[current=page]:font-semibold"
                ><Icon aria-hidden="true" size={18} />{item.label()}</Button
              >
            {:else}
              <span class="flex items-center gap-3 rounded-md px-3 py-2 text-muted"
                ><Icon aria-hidden="true" size={18} />{item.label()}</span
              >
            {/if}
          </li>
        {/each}
      </ul>
    </nav>
{/snippet}

<div class="flex h-full w-full overflow-hidden">
  {#if authenticated}
    {#if mobile.value}
      {#if drawerOpen}
        {#if navChunk.state === 'error'}
          <button
            type="button"
            class="fixed inset-0 z-40 bg-black/50"
            aria-label={m.nav_close()}
            onclick={closeDrawer}
          ></button>
        {/if}
        <div
          class={navChunk.state === 'error'
            ? 'fixed inset-y-0 left-0 z-50 flex w-60 flex-col gap-3 overflow-y-auto border-r border-border bg-surface p-3'
            : 'contents'}
        >
          {#if navChunk.state === 'error'}
            <Button variant="outline" type="button" onclick={closeDrawer}>{m.nav_close()}</Button>
          {/if}
          <PageHost
            load={mobileNav}
            onStatus={onNavStatus}
            props={{ children: navigation, close: closeDrawer, returnFocus: menuButton }}
          />
        </div>
      {/if}
    {:else}
      <aside id="sidebar" class="flex w-60 shrink-0 flex-col border-r border-border bg-surface">
        <div class="px-4 py-3 text-lg font-semibold">{m.app_name()}</div>
        {@render navigation()}
      </aside>
    {/if}
  {/if}

  <div class="flex min-w-0 flex-1 flex-col">
    <header class="flex items-center justify-between border-b border-border bg-surface px-4 py-2">
      <Button variant="ghost" size="icon"
        type="button"
        class="md:invisible {authenticated ? '' : 'invisible'}"
        aria-label={m.nav_open()}
        aria-controls="sidebar"
        aria-expanded={drawerOpen}
        bind:ref={menuButton}
        onclick={openDrawer}
      >
        <Menu aria-hidden="true" size={20} />
      </Button>
      <div class="flex items-center gap-1">
        <Button variant="ghost" size="icon"
          type="button"
          aria-label={m.theme_toggle()}
          onclick={() => theme.toggle()}
        >
          {#if theme.value === 'dark'}
            <Sun aria-hidden="true" size={20} />
          {:else}
            <MoonStar aria-hidden="true" size={20} />
          {/if}
        </Button>
        {#if authenticated}
          <Button variant="ghost" size="icon"
            type="button"
            aria-label={m.logout()}
            onclick={logout}
          >
            <LogOut aria-hidden="true" size={20} />
          </Button>
        {/if}
      </div>
    </header>

    <main class="flex-1 overflow-y-auto p-4">
      <div class="mx-auto max-w-3xl">
        {#key shown}
          {#if session.state === 'unknown'}
            <LoadState status={bootstrapStatus} retry={() => session.bootstrap()}>
              <span></span>
            </LoadState>
          {:else if shown === 'dashboard'}
            <PageHost load={pages.dashboard} props={{ events: dashboardEvents }} />
          {:else if shown === 'wifi'}
            <PageHost load={pages.wifi} />
          {:else if shown === 'vpn'}
            <PageHost load={pages.vpn} />
          {:else if shown === 'system'}
            <PageHost load={pages.system} />
          {:else if shown === 'login'}
            <PageHost load={pages.login} />
          {:else if shown === 'setup'}
            <PageHost load={pages.setup} />
          {:else}
            <PageHost load={pages.notYet} />
          {/if}
        {/key}
      </div>
    </main>
  </div>
</div>

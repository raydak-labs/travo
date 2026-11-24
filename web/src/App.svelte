<script lang="ts">
import ChevronRight from '@lucide/svelte/icons/chevron-right';
import Globe from '@lucide/svelte/icons/globe';
import LayoutDashboard from '@lucide/svelte/icons/layout-dashboard';
import Menu from '@lucide/svelte/icons/menu';
import MoonStar from '@lucide/svelte/icons/moon-star';
import Network from '@lucide/svelte/icons/network';
import Server from '@lucide/svelte/icons/server';
import Settings from '@lucide/svelte/icons/settings';
import Shield from '@lucide/svelte/icons/shield';
import Sun from '@lucide/svelte/icons/sun';
import Users from '@lucide/svelte/icons/users';
import Wifi from '@lucide/svelte/icons/wifi';
import { untrack } from 'svelte';
import Button from '$lib/components/ui/button/button.svelte';
import * as Collapsible from '$lib/components/ui/collapsible';
import { Toaster } from '$lib/components/ui/sonner';
import { getAdGuard } from './lib/api/gen';
import { ownDashboardEvents, restoreDashboardEvents } from './lib/app-dashboard-events';
import logo from './lib/assets/logo.webp';
import { dashboardEvents } from './lib/dashboard-events.svelte';
import { loadNavState, type NavState, saveNavState, withRoute } from './lib/nav-state';
import { navigate } from './lib/navigate';
import { lazyPage, pages } from './lib/pages/lazy';
import PageHost from './lib/pages/PageHost.svelte';
import { m } from './lib/paraglide/messages.js';
import { guard, href, NAV_GROUPS, type NavGroup, type Route, section } from './lib/router';
import { router } from './lib/router.svelte';
import { session } from './lib/session.svelte';
import { theme } from './lib/theme.svelte';
import AlertBanner from './lib/ui/AlertBanner.svelte';
import LoadState from './lib/ui/LoadState.svelte';
import type { LoadStatus } from './lib/ui/loader.svelte';

let drawerOpen = $state(false);
let navChunk = $state<LoadStatus>({ state: 'loading' });
let menuButton: HTMLElement | null = $state(null);
const mobileNav = lazyPage(() => import('./lib/ui/MobileNav.svelte'));
const headerMenu = lazyPage(() => import('./lib/ui/HeaderMenu.svelte'));
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

// Everyday pages first; expert settings sit one level deeper in the Network and System groups.
type NavItem = { route: Route; label: () => string; icon: typeof Wifi };
const everyday: NavItem[] = [
  { route: 'dashboard', label: () => m.nav_dashboard(), icon: LayoutDashboard },
  { route: 'internet', label: () => m.nav_internet(), icon: Globe },
  { route: 'wifi', label: () => m.nav_wifi(), icon: Wifi },
  { route: 'vpn', label: () => m.nav_vpn(), icon: Shield },
  { route: 'clients', label: () => m.nav_clients(), icon: Users },
  { route: 'services', label: () => m.nav_services(), icon: Server },
];
const childLabels: Partial<Record<Route, () => string>> = {
  'network-wan': () => m.nav_network_wan(),
  'network-lan': () => m.nav_network_lan(),
  'network-dhcp': () => m.nav_network_dhcp(),
  'network-port-forwards': () => m.nav_network_port_forwards(),
  'network-ipv6': () => m.nav_network_ipv6(),
  'network-diagnostics': () => m.nav_network_diagnostics(),
  system: () => m.nav_system(),
  'system-leds': () => m.nav_system_leds(),
  'system-logs': () => m.nav_system_logs(),
  'system-updates': () => m.nav_system_updates(),
  'system-tokens': () => m.nav_system_tokens(),
  'system-reset': () => m.nav_system_reset(),
};
const groups: { id: NavGroup; label: () => string; icon: typeof Wifi }[] = [
  { id: 'network', label: () => m.nav_group_network(), icon: Network },
  { id: 'system', label: () => m.nav_group_system(), icon: Settings },
];

// Open groups survive reloads (nav:v1); the group holding the current page always opens.
const storage = (() => {
  try {
    return localStorage;
  } catch {
    return undefined;
  }
})();
let navState = $state<NavState>(loadNavState(storage));
$effect(() => {
  const route = shown;
  untrack(() => {
    navState = withRoute(navState, route);
  });
});
function setGroup(group: NavGroup, open: boolean) {
  navState = { ...navState, [group]: open };
  saveNavState(storage, navState);
}

// Services leaves the nav only when the daemon cannot supervise AdGuard Home at all (503).
// An absent package stays visible: the page is the retry path for a failed install.
let servicesUnavailable = $state(false);
$effect(() => {
  if (!authenticated) return;
  const request = new AbortController();
  getAdGuard({ signal: request.signal })
    .then((r) => {
      servicesUnavailable = r.response?.status === 503;
    })
    .catch(() => {});
  return () => request.abort();
});
const shownNav = $derived(
  everyday.filter((item) => !(item.route === 'services' && servicesUnavailable)),
);

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

{#snippet navLink(route: Route, label: string, Icon?: typeof Wifi)}
  <Button variant="ghost"
    href={href(route)}
    aria-current={section(shown) === route ? 'page' : undefined}
    class="w-full justify-start gap-3 text-base font-normal aria-[current=page]:bg-bg aria-[current=page]:font-semibold"
    >{#if Icon}<Icon aria-hidden="true" size={18} />{/if}{label}</Button
  >
{/snippet}

{#snippet navigation()}
    <nav aria-label={m.nav_label()} class="flex-1 overflow-y-auto px-2 pb-4">
      <ul class="flex flex-col gap-1">
        {#each shownNav as item (item.route)}
          <li>{@render navLink(item.route, item.label(), item.icon)}</li>
        {/each}
        {#each groups as group (group.id)}
          {@const Icon = group.icon}
          {@const children = NAV_GROUPS[group.id]}
          <li>
            <Collapsible.Root open={navState[group.id]} onOpenChange={(open) => setGroup(group.id, open)}>
              <div class="flex items-center gap-1">
                <Button variant="ghost"
                  href={href(children[0] as Route)}
                  class="min-w-0 flex-1 justify-start gap-3 text-base font-normal"
                  ><Icon aria-hidden="true" size={18} />{group.label()}</Button
                >
                <Collapsible.Trigger>
                  {#snippet child({ props })}
                    <Button {...props} variant="ghost" size="icon" type="button"
                      aria-label={m.nav_group_toggle({ group: group.label() })}
                      class="shrink-0 aria-expanded:bg-transparent [&[data-state=open]>svg]:rotate-90"
                      ><ChevronRight aria-hidden="true" size={18} class="transition-transform" /></Button
                    >
                  {/snippet}
                </Collapsible.Trigger>
              </div>
              <Collapsible.Content>
                <ul class="mt-1 flex flex-col gap-1 pl-7">
                  {#each children as route (route)}
                    <li>{@render navLink(route, childLabels[route]?.() ?? route)}</li>
                  {/each}
                </ul>
              </Collapsible.Content>
            </Collapsible.Root>
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
        <div class="flex items-center gap-2 px-4 py-3 text-lg font-semibold">
          <img src={logo} alt="" class="size-7" />{m.app_name()}
        </div>
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
          class={authenticated && mobile.value ? 'hidden' : ''}
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
          <PageHost
            load={headerMenu}
            props={{ version: dashboardEvents.resources.data?.app.version, mobile: mobile.value, logout }}
          />
        {/if}
      </div>
    </header>

    <main class="flex-1 overflow-y-auto p-4 md:p-6">
      <div class="mx-auto max-w-3xl">
        {#if authenticated}
          <AlertBanner list={dashboardEvents.alerts.data} />
        {/if}
        {#key shown}
          {#if session.state === 'unknown'}
            <LoadState status={bootstrapStatus} retry={() => session.bootstrap()}>
              <span></span>
            </LoadState>
          {:else}
            <PageHost load={pages[shown]} props={shown === 'dashboard' ? { events: dashboardEvents } : {}} />
          {/if}
        {/key}
      </div>
    </main>
  </div>
</div>

<!-- The shell owns the one toast region; pages call notify / transactionToast (docs/web-ui.md). -->
<Toaster theme={theme.value} containerAriaLabel={m.notifications()} />

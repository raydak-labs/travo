# Real-time Topology Dashboard — Design Spec

**Date:** 2026-03-28
**Status:** Approved

---

## Overview

Two goals:

1. Push network status and client changes to the frontend in real time using the existing WebSocket infrastructure (event-driven via ubus, not polling).
2. Consolidate all topology-related state derivation into a single `useTopologyData()` hook that composes existing individual hooks without replacing them.

---

## Backend: Event-driven Network Status Broadcast

### New message type

The WebSocket hub broadcasts a new message type:

```json
{ "type": "network_status", "data": <NetworkStatus> }
```

`NetworkStatus` is the existing `models.NetworkStatus` struct already serialised by `GET /api/v1/network/status`.

---

### NetworkEventWatcher service

**File:** `backend/internal/services/network_event_watcher.go`

Runs as a long-lived goroutine. Subscribes to ubus events by spawning `ubus listen` as a subprocess and reading its JSON output line by line.

**Watched namespaces:**

| Namespace | Covers |
|---|---|
| `network.interface` | WAN up/down, WWAN (WiFi uplink) changes |
| `hostapd.*` | WiFi client associate / disassociate |
| `dhcp` | New DHCP leases (wired clients) |

On each matching event the watcher debounces (300 ms) to absorb rapid bursts (e.g. client roaming across bands), then calls `networkSvc.GetNetworkStatus()` and sends the result on `networkStatusCh chan models.NetworkStatus`.

An initial snapshot is emitted at startup so the first WebSocket client receives data immediately without waiting for an event.

#### Lifecycle — matches existing `Stop()` pattern

```go
type NetworkEventWatcher struct {
    networkSvc      *NetworkService
    networkStatusCh chan models.NetworkStatus
    stopCh          chan struct{}
}

func NewNetworkEventWatcher(networkSvc *NetworkService) *NetworkEventWatcher
func (w *NetworkEventWatcher) Start()   // called as go w.Start()
func (w *NetworkEventWatcher) Stop()    // closes stopCh — matches hub.Stop()
func (w *NetworkEventWatcher) Ch() <-chan models.NetworkStatus
```

The owner (`main.go`) calls `w.Stop()` alongside `hub.Stop()` on SIGINT/SIGTERM. No `context.Context` parameter needed — consistent with the existing `Stop()` channel pattern used by `Hub` and `AlertService`.

#### Subprocess restart / back-off

If `ubus listen` exits unexpectedly, the watcher restarts it with exponential back-off: 1 s → 2 s → 4 s → … capped at 30 s. Consistent with the frontend's 3 s reconnect delay.

#### Mock mode

`NetworkEventWatcher` accepts a `subprocessRunner` interface (analogous to how `ubus.Ubus` is mocked elsewhere). In mock mode (`cfg.MockMode == true`) `main.go` passes a `NoopEventWatcher` that emits nothing but satisfies the same interface, so `NewHub` always receives a valid (possibly empty) channel.

```go
type EventWatcher interface {
    Start()
    Stop()
    Ch() <-chan models.NetworkStatus
}
```

`main.go` picks the concrete type:

```go
var netWatcher services.EventWatcher
if cfg.MockMode {
    netWatcher = services.NewNoopEventWatcher()
} else {
    netWatcher = services.NewNetworkEventWatcher(networkSvc)
}
```

---

### Hub changes

#### Constructor

`NewHub` gains a third parameter to receive the network status channel. Existing call sites (`hub_test.go` lines 16, 30 and `main.go` line 174) must be updated.

```go
func NewHub(
    systemSvc *services.SystemService,
    alertSvc  *services.AlertService,
    networkStatusCh <-chan models.NetworkStatus,   // new
) *Hub
```

In tests, pass `nil` or a closed channel — the `select` case safely ignores a `nil` channel.

#### New `broadcastNetworkStatus` helper

Follows the same pattern as the existing `broadcastStats` and `broadcastAlert` helpers:

```go
func (h *Hub) broadcastNetworkStatus(ns models.NetworkStatus) {
    msg := map[string]interface{}{
        "type": "network_status",
        "data": ns,
    }
    data, err := json.Marshal(msg)
    if err != nil {
        return
    }
    h.Broadcast(data)
}
```

#### Extended `Start()` select loop

```go
case ns, ok := <-h.networkStatusCh:
    if ok {
        h.broadcastNetworkStatus(ns)
    }
```

---

### Wiring in `main.go`

```go
var netWatcher services.EventWatcher
if cfg.MockMode {
    netWatcher = services.NewNoopEventWatcher()
} else {
    netWatcher = services.NewNetworkEventWatcher(networkSvc)
}
go netWatcher.Start()

hub := ws.NewHub(systemSvc, alertSvc, netWatcher.Ch())
hub.Start()

// On shutdown:
hub.Stop()
netWatcher.Stop()
```

---

### Crash-safety

`NetworkEventWatcher` is read-only (`ubus listen`, then `GetNetworkStatus` reads). No UCI commits or system-state mutations occur. No guard file is required.

---

## Frontend: WebSocket Message Bus

### Problem

`useWebSocket` hard-codes its `onmessage` handler to process only `system_stats`. `useTopologyData` needs to observe `network_status` messages on the **same** connection. Opening a second WebSocket connection would waste resources. Passing the `wsRef` out of the hook creates tight coupling.

### Solution: `WebSocketContext` message bus

**New file:** `frontend/src/lib/ws-context.tsx`

A React context that owns exactly one WebSocket connection for the app. Consumers subscribe to specific message types via a callback; the context dispatches incoming messages to all matching subscribers.

```ts
interface WsContextValue {
  connected: boolean;
  subscribe(type: string, handler: (data: unknown) => void): () => void;
  // returns an unsubscribe function
}
```

The `WsProvider` component (wraps the app in `main.tsx` or `App.tsx`) manages the connection lifecycle, reconnect logic, and JWT token — moving this responsibility out of the individual hook.

#### Migration of `useWebSocket`

`useWebSocket` becomes a thin consumer of `WsContext`. It subscribes to `system_stats` via `subscribe('system_stats', handler)` and continues returning the same `{ dataPoints, interfaceDataPoints, connected }` API. **No change to its public interface** — existing callers are unaffected.

#### `useTopologyData` subscribes to `network_status`

```ts
const { subscribe } = useContext(WsContext);

useEffect(() => {
  const unsub = subscribe('network_status', (data) => {
    queryClient.setQueryData<NetworkStatus>(['network', 'status'], data as NetworkStatus);
  });
  return unsub;
}, [subscribe, queryClient]);
```

---

## Frontend: `useTopologyData` hook

**File:** `frontend/src/hooks/use-topology-data.ts`

### Responsibilities

1. Composes existing hooks — does **not** replace them:
   - `useNetworkStatus()`
   - `useWifiConnection()`
   - `useUSBTetherStatus()`
   - `useVpnStatus()`
   - `useIPv6Status()`
   - `useSystemInfo()`

2. Subscribes to `network_status` WS messages and writes them into the React Query cache for `['network', 'status']`. All components using `useNetworkStatus()` benefit automatically.

3. Sets `staleTime: Infinity` on the `useNetworkStatus()` call inside the hook so that a WS update is never immediately overwritten by a stale HTTP refetch. Other components that use `useNetworkStatus()` outside this hook are unaffected (they use their own query options).

4. Derives all topology display state in one place — including the `wan.type` logic.

### Return type

```ts
interface TopologyData {
  sources: SourceDef[]                                            // ethernet, repeater, usb, cellular
  clients: { label: string; icon: LucideIcon; count: number }[]  // WLAN + LAN — matches ClientDef in TopologyDiagram exactly
  features: { label: string; active: boolean }[]                 // IPv6, VPN, Internet — matches TopologyDiagram prop type exactly
  router: { hostname: string; model: string }
  loading: boolean
}

interface SourceDef {
  label: string;
  icon: LucideIcon;
  connected: boolean;
  detail?: string;
}
```

`SourceDef` is shared with `TopologyDiagram` (it already defines this shape). `clients` and `features` use inline object shapes matching the existing `TopologyDiagram` props exactly — no new named types imposed on the component.

### Connection-type derivation (moved here from the component)

```ts
const ethernetUp = wan?.is_up === true && wan.type !== 'wifi' && wan.type !== 'usb'
const repeaterUp =
  (wan?.is_up === true && wan.type === 'wifi') ||
  (wifiConn?.connected === true && wifiConn.mode === 'client')
const tetherUp =
  (wan?.is_up === true && wan.type === 'usb') ||
  (usbTether?.is_up === true)
```

`wan.type === 'wifi'` means the WWAN (WiFi uplink) interface is the active upstream path — not Ethernet.

---

## `experimental-page.tsx` after refactor

Calls only `useTopologyData()` and passes the result straight to `<TopologyDiagram>`. No connection-type logic remains in the component. The component is a thin renderer.

### Implementation notes

- `broadcastNetworkStatus` should guard on `h.ClientCount() == 0` (same as `broadcastStats`) to avoid serialising on every event when no clients are connected.
- `netWatcher` must be returned from `setupAppWithConfig` (or have its `Stop()` called inside it on shutdown) so that test usages of `setupApp()` do not leak the watcher goroutine.

---

## What is NOT changing

- Individual hooks (`useNetworkStatus`, `useWifiConnection`, etc.) are **untouched** — other pages continue to use them.
- The `TopologyDiagram` component is **untouched** — its prop types are unchanged.
- The hub's existing `system_stats` and `alert` broadcast paths are **untouched**.
- No new HTTP endpoints are added.

---

## Testing

### Backend

- **`NetworkEventWatcher` unit test**: inject a mock subprocess runner that emits synthetic ubus event lines; assert the correct `NetworkStatus` value appears on `Ch()` within the debounce window.
- **`NoopEventWatcher` unit test**: `Ch()` never sends, `Stop()` does not block.
- **Hub unit test** (`hub_test.go`): update `NewHub` call sites to pass `nil` for `networkStatusCh` (a nil channel is never selected — safe). Add one test that passes a buffered channel, sends a `NetworkStatus` value, starts the hub, and asserts `Broadcast` is called with `"type":"network_status"`.

### Frontend

- **`WsProvider` unit test**: mock `WebSocket`; assert `subscribe` callbacks are invoked with the correct `data` when a matching message type arrives; assert unknown types are not dispatched.
- **`useTopologyData` unit test**:
  - Mock `WsContext` to inject a `network_status` message; assert `queryClient.setQueryData` is called and the returned `sources` reflect the updated state.
  - Parameterise over `wan.type` values (`'wan'`, `'wifi'`, `'usb'`, `null`) and assert correct `ethernetUp`/`repeaterUp`/`tetherUp` derivation.
- **`useWebSocket` smoke test**: still passes after being refactored to consume `WsContext` — verify `dataPoints` update when `system_stats` messages arrive.

---

## Optional cleanup (post-implementation)

If the refactor reveals that some individual hooks are only ever used by the topology page, they can be merged into `useTopologyData`. Do not merge hooks used by other pages. Evaluate after the feature is stable.

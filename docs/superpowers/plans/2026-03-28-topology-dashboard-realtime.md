# Real-time Topology Dashboard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Push network/client changes to the experimental topology dashboard in real time via ubus-event-driven WebSocket broadcasts, and consolidate all topology state into a clean `useTopologyData()` hook.

**Architecture:** A new `NetworkEventWatcher` service on the backend subscribes to `ubus listen` and emits `network_status` WebSocket messages on relevant events. On the frontend, a new `WsContext` message bus owns the single WebSocket connection so multiple hooks can subscribe by message type without opening duplicate connections. `useTopologyData()` composes existing hooks and subscribes to `network_status` to keep the cache live.

**Tech Stack:** Go/Fiber backend, `gofiber/websocket/v2` (already in go.mod), React + TypeScript + TanStack Query frontend, Vitest + Testing Library for frontend tests.

---

## File Map

**Backend — new:**
- `backend/internal/services/network_event_watcher.go` — `EventWatcher` interface, `NetworkEventWatcher` (subprocess, debounce, backoff), `NoopEventWatcher`
- `backend/internal/services/network_event_watcher_test.go` — unit tests

**Backend — modified:**
- `backend/internal/ws/hub.go` — add `networkStatusCh` field, `broadcastNetworkStatus` helper, update `NewHub` + `Start()`
- `backend/internal/ws/hub_test.go` — update two `NewHub` call sites (pass `nil`), add `network_status` broadcast test
- `backend/cmd/server/main.go` — wire `netWatcher`, update `setupAppWithConfig` return (7th value), update `setupApp` and shutdown

**Frontend — new:**
- `frontend/src/lib/ws-context.tsx` — `WsContext` + `WsProvider` message bus (owns the WebSocket connection, wraps the app)
- `frontend/src/lib/__tests__/ws-context.test.tsx` — unit tests
- `frontend/src/hooks/use-topology-data.ts` — `useTopologyData()` hook
- `frontend/src/hooks/__tests__/use-topology-data.test.ts` — unit tests

**Frontend — modified:**
- `frontend/src/hooks/use-network.ts` — update `useNetworkStatus` to accept optional query options (for `staleTime: Infinity` override)
- `frontend/src/hooks/use-websocket.ts` — consume `WsContext.subscribe` instead of managing its own WebSocket; public API unchanged
- `frontend/src/hooks/__tests__/use-websocket-smoke.test.ts` — new smoke test confirming migrated hook still processes `system_stats`
- `frontend/src/App.tsx` — wrap `RouterProvider` with `<WsProvider>` (inside `QueryClientProvider`)
- `frontend/src/pages/experimental/experimental-page.tsx` — replace all derived state and hook calls with `useTopologyData()`

---

## Task 1: EventWatcher interface + NoopEventWatcher

**Files:**
- Create: `backend/internal/services/network_event_watcher.go`
- Create: `backend/internal/services/network_event_watcher_test.go`

- [ ] **Step 1: Write the failing test**

```go
// backend/internal/services/network_event_watcher_test.go
package services

import (
    "testing"
    "time"
)

func TestNoopEventWatcher(t *testing.T) {
    w := NewNoopEventWatcher()
    go w.Start()

    select {
    case <-w.Ch():
        t.Fatal("NoopEventWatcher should never send")
    case <-time.After(50 * time.Millisecond):
        // pass
    }

    w.Stop() // must not block
}
```

Run: `cd backend && go test ./internal/services/... -run TestNoopEventWatcher -v`
Expected: **FAIL** — `NewNoopEventWatcher` undefined

- [ ] **Step 2: Implement the interface and NoopEventWatcher**

```go
// backend/internal/services/network_event_watcher.go
package services

import "github.com/openwrt-travel-gui/backend/internal/models"

// EventWatcher is implemented by NetworkEventWatcher (real) and NoopEventWatcher (mock/test).
type EventWatcher interface {
    Start()
    Stop()
    Ch() <-chan models.NetworkStatus
}

// NoopEventWatcher satisfies EventWatcher but never emits anything.
// Used in mock mode and tests.
type NoopEventWatcher struct {
    ch     chan models.NetworkStatus
    stopCh chan struct{}
}

func NewNoopEventWatcher() *NoopEventWatcher {
    return &NoopEventWatcher{
        ch:     make(chan models.NetworkStatus),
        stopCh: make(chan struct{}),
    }
}

func (w *NoopEventWatcher) Start() {
    <-w.stopCh // block until Stop is called
}

func (w *NoopEventWatcher) Stop() {
    close(w.stopCh)
}

func (w *NoopEventWatcher) Ch() <-chan models.NetworkStatus {
    return w.ch
}
```

- [ ] **Step 3: Run test — expect PASS**

Run: `cd backend && go test ./internal/services/... -run TestNoopEventWatcher -v`
Expected: `PASS`

- [ ] **Step 4: Commit**

```bash
git add backend/internal/services/network_event_watcher.go \
        backend/internal/services/network_event_watcher_test.go
git commit -m "feat(ws): add EventWatcher interface and NoopEventWatcher"
```

---

## Task 2: NetworkEventWatcher (real implementation)

**Files:**
- Modify: `backend/internal/services/network_event_watcher.go`
- Modify: `backend/internal/services/network_event_watcher_test.go`

The watcher spawns `ubus listen` as a subprocess, reads JSON lines, debounces 300 ms, then calls `networkSvc.GetNetworkStatus()` and sends the result on its channel. The `subprocessRunner` interface allows test injection of fake event streams.

- [ ] **Step 1: Write the failing test**

```go
// Add to network_event_watcher_test.go

func TestNetworkEventWatcher_EmitsOnEvent(t *testing.T) {
    ub := ubus.NewMockUbus()
    u := uci.NewMockUCI()
    networkSvc := NewNetworkService(u, ub)

    // fakeRunner feeds one watched event line then blocks forever
    lines := make(chan string, 1)
    lines <- `{ "network.interface": { "action": "ifup", "interface": "wwan" } }`

    w := newNetworkEventWatcherWithRunner(networkSvc, &chanRunner{lines: lines})
    go w.Start()
    defer w.Stop()

    select {
    case ns := <-w.Ch():
        _ = ns // we just need any result; mock ubus returns a valid empty status
    case <-time.After(2 * time.Second):
        t.Fatal("expected network_status event within 2s (including 300ms debounce)")
    }
}

// chanRunner is a fake subprocessRunner whose Lines() method reads from a channel.
type chanRunner struct {
    lines chan string
}

func (r *chanRunner) Lines(stopCh <-chan struct{}) <-chan string {
    out := make(chan string)
    go func() {
        defer close(out)
        for {
            select {
            case line, ok := <-r.lines:
                if !ok {
                    return
                }
                out <- line
            case <-stopCh:
                return
            }
        }
    }()
    return out
}
```

Run: `cd backend && go test ./internal/services/... -run TestNetworkEventWatcher_EmitsOnEvent -v`
Expected: **FAIL** — `newNetworkEventWatcherWithRunner` undefined

- [ ] **Step 2: Implement NetworkEventWatcher with subprocessRunner**

Append to `network_event_watcher.go`. This is the complete, correct implementation — write it as shown (no intermediate broken version):

```go
import (
    "bufio"
    "log"
    "os/exec"
    "strings"
    "time"
)

// subprocessRunner abstracts launching `ubus listen`. Replaced in tests by chanRunner.
type subprocessRunner interface {
    // Lines returns a channel of raw output lines.
    // It closes the channel when the subprocess exits or stopCh is closed.
    Lines(stopCh <-chan struct{}) <-chan string
}

// realRunner launches `ubus listen` and streams stdout lines.
type realRunner struct{}

func (r *realRunner) Lines(stopCh <-chan struct{}) <-chan string {
    out := make(chan string)
    go func() {
        defer close(out)
        cmd := exec.Command("ubus", "listen", "network.interface", "hostapd", "dhcp")
        stdout, err := cmd.StdoutPipe()
        if err != nil {
            return
        }
        if err := cmd.Start(); err != nil {
            return
        }
        doneCh := make(chan struct{})
        go func() {
            defer close(doneCh)
            scanner := bufio.NewScanner(stdout)
            for scanner.Scan() {
                select {
                case out <- scanner.Text():
                case <-stopCh:
                    return
                }
            }
        }()
        select {
        case <-doneCh:
        case <-stopCh:
        }
        _ = cmd.Process.Kill()
        _ = cmd.Wait()
    }()
    return out
}

// watchedPrefixes are the ubus namespaces we care about.
var watchedPrefixes = []string{
    `"network.interface"`,
    `"hostapd.`,
    `"dhcp"`,
}

func isWatched(line string) bool {
    for _, prefix := range watchedPrefixes {
        if strings.Contains(line, prefix) {
            return true
        }
    }
    return false
}

// NetworkEventWatcher watches ubus events and emits NetworkStatus snapshots on change.
type NetworkEventWatcher struct {
    networkSvc *NetworkService
    runner     subprocessRunner
    ch         chan models.NetworkStatus
    stopCh     chan struct{}
}

func NewNetworkEventWatcher(networkSvc *NetworkService) *NetworkEventWatcher {
    return newNetworkEventWatcherWithRunner(networkSvc, &realRunner{})
}

func newNetworkEventWatcherWithRunner(networkSvc *NetworkService, runner subprocessRunner) *NetworkEventWatcher {
    return &NetworkEventWatcher{
        networkSvc: networkSvc,
        runner:     runner,
        ch:         make(chan models.NetworkStatus, 1),
        stopCh:     make(chan struct{}),
    }
}

func (w *NetworkEventWatcher) Ch() <-chan models.NetworkStatus { return w.ch }
func (w *NetworkEventWatcher) Stop()                           { close(w.stopCh) }

func (w *NetworkEventWatcher) Start() {
    // Emit an initial snapshot so the first WebSocket client gets data immediately.
    w.emitSnapshot()

    backoff := time.Second
    const maxBackoff = 30 * time.Second

    for {
        gotLine := false
        lines := w.runner.Lines(w.stopCh)
        var timer *time.Timer

    loop:
        for {
            select {
            case <-w.stopCh:
                if timer != nil {
                    timer.Stop()
                }
                return
            case line, ok := <-lines:
                if !ok {
                    break loop
                }
                gotLine = true
                if !isWatched(line) {
                    continue
                }
                // Debounce: reset a 300 ms timer on every watched event.
                if timer != nil {
                    timer.Stop()
                }
                timer = time.AfterFunc(300*time.Millisecond, func() {
                    w.emitSnapshot()
                })
            }
        }

        log.Printf("NetworkEventWatcher: ubus listen exited, restarting in %s", backoff)
        select {
        case <-time.After(backoff):
        case <-w.stopCh:
            return
        }
        // Reset backoff only if the subprocess produced at least one line (healthy session).
        if gotLine {
            backoff = time.Second
        } else {
            backoff *= 2
            if backoff > maxBackoff {
                backoff = maxBackoff
            }
        }
    }
}

func (w *NetworkEventWatcher) emitSnapshot() {
    ns, err := w.networkSvc.GetNetworkStatus()
    if err != nil {
        log.Printf("NetworkEventWatcher: GetNetworkStatus error: %v", err)
        return
    }
    // Non-blocking send. If the hub hasn't consumed the previous value, overwrite it.
    select {
    case w.ch <- ns:
    default:
        select {
        case <-w.ch:
        default:
        }
        select {
        case w.ch <- ns:
        default:
        }
    }
}

// Compile-time interface checks.
var _ EventWatcher = (*NetworkEventWatcher)(nil)
var _ EventWatcher = (*NoopEventWatcher)(nil)
```

- [ ] **Step 3: Run tests — expect PASS**

Run: `cd backend && go test ./internal/services/... -run 'TestNoopEventWatcher|TestNetworkEventWatcher' -v`
Expected: all `PASS`

- [ ] **Step 4: Commit**

```bash
git add backend/internal/services/network_event_watcher.go \
        backend/internal/services/network_event_watcher_test.go
git commit -m "feat(ws): implement NetworkEventWatcher with ubus listen and debounce"
```

---

## Task 3: Hub — add network_status broadcast

**Files:**
- Modify: `backend/internal/ws/hub.go`
- Modify: `backend/internal/ws/hub_test.go`

- [ ] **Step 1: Update hub tests first**

In `hub_test.go`, update the two existing `NewHub` calls to pass `nil` for the new channel, and add one new test:

```go
// TestNewHub: change  NewHub(svc, alertSvc)  →  NewHub(svc, alertSvc, nil)
// TestHubStartStop: change  NewHub(svc, alertSvc)  →  NewHub(svc, alertSvc, nil)

func TestHub_BroadcastsNetworkStatus(t *testing.T) {
    ub := ubus.NewMockUbus()
    svc := services.NewSystemService(ub, uci.NewMockUCI(), &services.MockStorageProvider{})
    alertSvc := services.NewAlertService(svc)

    nsCh := make(chan models.NetworkStatus, 1)
    hub := NewHub(svc, alertSvc, nsCh)
    hub.BroadcastInterval = 10 * time.Millisecond

    hub.Start()
    defer hub.Stop()

    // No WebSocket clients — no panic expected even when channel receives
    nsCh <- models.NetworkStatus{}
    time.Sleep(50 * time.Millisecond)
}
```

Run: `cd backend && go test ./internal/ws/... -v`
Expected: **FAIL** — `too many arguments in call to NewHub`

- [ ] **Step 2: Update Hub struct and NewHub**

In `backend/internal/ws/hub.go`:

```go
// Add field to Hub struct:
networkStatusCh <-chan models.NetworkStatus

// Replace NewHub:
func NewHub(
    systemSvc *services.SystemService,
    alertSvc  *services.AlertService,
    networkStatusCh <-chan models.NetworkStatus,
) *Hub {
    return &Hub{
        clients:           make(map[*websocket.Conn]bool),
        systemSvc:         systemSvc,
        alertSvc:          alertSvc,
        networkStatusCh:   networkStatusCh,
        stopCh:            make(chan struct{}),
        BroadcastInterval: 2 * time.Second,
    }
}
```

- [ ] **Step 3: Add broadcastNetworkStatus and update Start()**

Add helper (after `broadcastAlert`):

```go
func (h *Hub) broadcastNetworkStatus(ns models.NetworkStatus) {
    if h.ClientCount() == 0 {
        return
    }
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

In `Start()`, add alongside the `alertCh` nil-guard:

```go
var networkStatusCh <-chan models.NetworkStatus
if h.networkStatusCh != nil {
    networkStatusCh = h.networkStatusCh
}
```

Add to the `select` loop (after the `alertCh` case):

```go
case ns, ok := <-networkStatusCh:
    if ok {
        h.broadcastNetworkStatus(ns)
    }
```

- [ ] **Step 4: Run hub tests — expect PASS**

Run: `cd backend && go test ./internal/ws/... -v`
Expected: all `PASS`

- [ ] **Step 5: Commit**

```bash
git add backend/internal/ws/hub.go backend/internal/ws/hub_test.go
git commit -m "feat(ws): hub broadcasts network_status from EventWatcher channel"
```

---

## Task 4: Wire netWatcher into main.go

**Files:**
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: Update setupAppWithConfig signature**

Add `services.EventWatcher` as the 7th return value:

```go
func setupAppWithConfig(cfg config.Config) (
    *fiber.App,
    *ws.Hub,
    *services.AlertService,
    *services.UptimeTracker,
    *services.BandSwitchingService,
    *auth.TokenBlocklist,
    services.EventWatcher,
)
```

- [ ] **Step 2: Create and start netWatcher inside setupAppWithConfig**

After `networkSvc` is created (around line 80), add:

```go
var netWatcher services.EventWatcher
if cfg.MockMode {
    netWatcher = services.NewNoopEventWatcher()
} else {
    netWatcher = services.NewNetworkEventWatcher(networkSvc)
}
go netWatcher.Start()
```

Update the `ws.NewHub` call:

```go
hub := ws.NewHub(systemSvc, alertSvc, netWatcher.Ch())
```

Update the return statement to append `netWatcher`.

- [ ] **Step 3: Update setupApp (used in tests only)**

`setupApp` calls `setupAppWithConfig` and discards returns. Stop the watcher immediately to prevent goroutine leaks in tests:

```go
func setupApp() *fiber.App {
    cfg := config.DefaultConfig()
    app, _, _, _, _, _, netWatcher := setupAppWithConfig(cfg)
    netWatcher.Stop()
    return app
}
```

- [ ] **Step 4: Update main() to unpack and stop netWatcher**

```go
app, hub, alertSvc, uptimeTracker, bandSwitchSvc, blocklist, netWatcher := setupAppWithConfig(cfg)

// In the shutdown goroutine, add:
netWatcher.Stop()
```

- [ ] **Step 5: Build and run all backend tests**

Run: `cd backend && go build ./... && go test ./...`
Expected: all pass, no compile errors

- [ ] **Step 6: Commit**

```bash
git add backend/cmd/server/main.go
git commit -m "feat(ws): wire NetworkEventWatcher into server startup and shutdown"
```

---

## Task 5: Frontend — WsContext message bus

**Files:**
- Create: `frontend/src/lib/ws-context.tsx`
- Create: `frontend/src/lib/__tests__/ws-context.test.tsx`

This context owns exactly one WebSocket connection. Consumers call `subscribe(type, handler)` which returns an unsubscribe function.

- [ ] **Step 1: Write the failing tests**

```tsx
// frontend/src/lib/__tests__/ws-context.test.tsx
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import React from 'react';
import { WsProvider, useWsSubscribe } from '../ws-context';

class MockWebSocket {
  static OPEN = 1;
  readyState = MockWebSocket.OPEN;
  onopen: ((e: Event) => void) | null = null;
  onmessage: ((e: MessageEvent) => void) | null = null;
  onclose: ((e: CloseEvent) => void) | null = null;
  onerror: ((e: Event) => void) | null = null;
  close = vi.fn();
  send = vi.fn();
}

let mockWsInstance: MockWebSocket;

beforeEach(() => {
  vi.stubGlobal('WebSocket', vi.fn().mockImplementation(() => {
    mockWsInstance = new MockWebSocket();
    return mockWsInstance;
  }));
});

function wrapper({ children }: { children: React.ReactNode }) {
  return <WsProvider>{children}</WsProvider>;
}

describe('WsProvider', () => {
  it('dispatches message to subscriber matching type', () => {
    const handler = vi.fn();
    const { result } = renderHook(() => useWsSubscribe(), { wrapper });

    act(() => { result.current.subscribe('network_status', handler); });

    act(() => {
      mockWsInstance.onmessage?.({
        data: JSON.stringify({ type: 'network_status', data: { wan: null } }),
      } as MessageEvent);
    });

    expect(handler).toHaveBeenCalledWith({ wan: null });
  });

  it('does not dispatch to unrelated subscribers', () => {
    const handler = vi.fn();
    const { result } = renderHook(() => useWsSubscribe(), { wrapper });

    act(() => { result.current.subscribe('system_stats', handler); });

    act(() => {
      mockWsInstance.onmessage?.({
        data: JSON.stringify({ type: 'network_status', data: {} }),
      } as MessageEvent);
    });

    expect(handler).not.toHaveBeenCalled();
  });

  it('unsubscribes correctly', () => {
    const handler = vi.fn();
    const { result } = renderHook(() => useWsSubscribe(), { wrapper });

    let unsub!: () => void;
    act(() => { unsub = result.current.subscribe('network_status', handler); });
    act(() => { unsub(); });

    act(() => {
      mockWsInstance.onmessage?.({
        data: JSON.stringify({ type: 'network_status', data: {} }),
      } as MessageEvent);
    });

    expect(handler).not.toHaveBeenCalled();
  });

  it('sets connected=true after onopen fires', () => {
    const { result } = renderHook(() => useWsSubscribe(), { wrapper });
    expect(result.current.connected).toBe(false);

    act(() => { mockWsInstance.onopen?.(new Event('open')); });

    expect(result.current.connected).toBe(true);
  });
});
```

Run: `cd frontend && pnpm test --run src/lib/__tests__/ws-context.test.tsx`
Expected: **FAIL** — `WsProvider` not found

- [ ] **Step 2: Implement WsContext**

```tsx
// frontend/src/lib/ws-context.tsx
import React, { createContext, useCallback, useContext, useEffect, useRef, useState } from 'react';
import { getToken } from './api-client';

type MessageHandler = (data: unknown) => void;

interface WsContextValue {
  connected: boolean;
  subscribe: (type: string, handler: MessageHandler) => () => void;
}

const WsContext = createContext<WsContextValue>({
  connected: false,
  subscribe: () => () => {},
});

const RECONNECT_DELAY = 3000;

export function WsProvider({ children }: { children: React.ReactNode }) {
  const [connected, setConnected] = useState(false);
  const subscribersRef = useRef<Map<string, Set<MessageHandler>>>(new Map());
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const mountedRef = useRef(true);

  const connect = useCallback(() => {
    if (!mountedRef.current) return;
    const token = getToken();
    if (!token) return;

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const url = `${protocol}//${window.location.host}/api/v1/ws?token=${encodeURIComponent(token)}`;

    try {
      const ws = new WebSocket(url);
      wsRef.current = ws;

      ws.onopen = () => {
        if (mountedRef.current) setConnected(true);
      };

      ws.onmessage = (event) => {
        if (!mountedRef.current) return;
        try {
          const msg = JSON.parse(event.data as string) as { type: string; data: unknown };
          subscribersRef.current.get(msg.type)?.forEach((h) => h(msg.data));
        } catch {
          // ignore malformed messages
        }
      };

      ws.onclose = () => {
        if (mountedRef.current) {
          setConnected(false);
          reconnectTimer.current = setTimeout(connect, RECONNECT_DELAY);
        }
      };

      ws.onerror = () => { ws.close(); };
    } catch {
      if (mountedRef.current) {
        reconnectTimer.current = setTimeout(connect, RECONNECT_DELAY);
      }
    }
  }, []);

  useEffect(() => {
    mountedRef.current = true;
    connect();
    return () => {
      mountedRef.current = false;
      if (reconnectTimer.current) clearTimeout(reconnectTimer.current);
      wsRef.current?.close();
    };
  }, [connect]);

  const subscribe = useCallback((type: string, handler: MessageHandler): (() => void) => {
    if (!subscribersRef.current.has(type)) {
      subscribersRef.current.set(type, new Set());
    }
    subscribersRef.current.get(type)!.add(handler);
    return () => { subscribersRef.current.get(type)?.delete(handler); };
  }, []);

  return (
    <WsContext.Provider value={{ connected, subscribe }}>
      {children}
    </WsContext.Provider>
  );
}

/** Used by useWebSocket and useTopologyData to subscribe to typed messages. */
export function useWsSubscribe() {
  return useContext(WsContext);
}
```

- [ ] **Step 3: Run tests — expect PASS**

Run: `cd frontend && pnpm test --run src/lib/__tests__/ws-context.test.tsx`
Expected: all `PASS`

- [ ] **Step 4: Commit**

```bash
git add frontend/src/lib/ws-context.tsx \
        frontend/src/lib/__tests__/ws-context.test.tsx
git commit -m "feat(ws): add WsContext message bus for shared WebSocket connection"
```

---

## Task 6: Migrate useWebSocket to consume WsContext

**Files:**
- Modify: `frontend/src/hooks/use-websocket.ts`
- Create: `frontend/src/hooks/__tests__/use-websocket-smoke.test.ts`

The hook's public API (`{ dataPoints, interfaceDataPoints, connected }`) must not change. We replace the WebSocket management code with `subscribe` calls from `WsContext`.

Note: `WsProvider` is in scope for all callers because it wraps the app root (done in Task 8). The default context value is a no-op `subscribe` that returns an unsubscribe function, so callers outside `WsProvider` are safe.

- [ ] **Step 1: Write the smoke test first**

```ts
// frontend/src/hooks/__tests__/use-websocket-smoke.test.ts
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import React from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { WsProvider, useWsSubscribe } from '@/lib/ws-context';
import { useWebSocket } from '../use-websocket';

let mockWsInstance: {
  onopen: ((e: Event) => void) | null;
  onmessage: ((e: MessageEvent) => void) | null;
  onclose: ((e: CloseEvent) => void) | null;
  onerror: ((e: Event) => void) | null;
  close: ReturnType<typeof vi.fn>;
};

beforeEach(() => {
  vi.stubGlobal('WebSocket', vi.fn().mockImplementation(() => {
    mockWsInstance = {
      onopen: null, onmessage: null, onclose: null, onerror: null, close: vi.fn(),
    };
    return mockWsInstance;
  }));
});

function wrapper({ children }: { children: React.ReactNode }) {
  const qc = new QueryClient();
  return (
    <QueryClientProvider client={qc}>
      <WsProvider>{children}</WsProvider>
    </QueryClientProvider>
  );
}

describe('useWebSocket (after WsContext migration)', () => {
  it('accumulates dataPoints when system_stats messages arrive', () => {
    const { result } = renderHook(() => useWebSocket(), { wrapper });

    const statsMsg = {
      type: 'system_stats',
      data: {
        cpu: { usage_percent: 42, cores: 4, load_average: [0, 0, 0] },
        memory: { total_bytes: 1000, used_bytes: 500, free_bytes: 500, cached_bytes: 0, usage_percent: 50 },
        storage: { total_bytes: 1000, used_bytes: 200, free_bytes: 800, usage_percent: 20 },
        network: [{ interface: 'eth0', rx_bytes: 100, tx_bytes: 200 }],
      },
    };

    act(() => {
      mockWsInstance.onmessage?.({
        data: JSON.stringify(statsMsg),
      } as MessageEvent);
    });

    expect(result.current.dataPoints).toHaveLength(1);
    expect(result.current.dataPoints[0].cpu).toBe(42);
  });

  it('clears dataPoints when WS disconnects', () => {
    const { result } = renderHook(() => useWebSocket(), { wrapper });

    const statsMsg = {
      type: 'system_stats',
      data: {
        cpu: { usage_percent: 10, cores: 2, load_average: [0, 0, 0] },
        memory: { total_bytes: 1000, used_bytes: 100, free_bytes: 900, cached_bytes: 0, usage_percent: 10 },
        storage: { total_bytes: 1000, used_bytes: 100, free_bytes: 900, usage_percent: 10 },
        network: [],
      },
    };

    act(() => {
      mockWsInstance.onmessage?.({ data: JSON.stringify(statsMsg) } as MessageEvent);
    });
    expect(result.current.dataPoints).toHaveLength(1);

    // Simulate disconnect
    act(() => { mockWsInstance.onclose?.({} as CloseEvent); });

    expect(result.current.dataPoints).toHaveLength(0);
  });
});
```

Run: `cd frontend && pnpm test --run src/hooks/__tests__/use-websocket-smoke.test.ts`
Expected: **FAIL** (hook not yet migrated; or dataPoints clearing test fails)

- [ ] **Step 2: Rewrite useWebSocket**

Replace the entire file:

```ts
// frontend/src/hooks/use-websocket.ts
import { useEffect, useState } from 'react';
import type { SystemStats } from '@shared/index';
import { useWsSubscribe } from '@/lib/ws-context';

export interface StatsDataPoint {
  timestamp: number;
  cpu: number;
  memoryUsed: number;
  memoryTotal: number;
  rxBytes: number;
  txBytes: number;
}

export interface InterfaceDataPoint {
  timestamp: number;
  rxBytes: number;
  txBytes: number;
}

const MAX_POINTS = 15;

export function useWebSocket() {
  const { connected, subscribe } = useWsSubscribe();
  const [dataPoints, setDataPoints] = useState<StatsDataPoint[]>([]);
  const [interfaceDataPoints, setInterfaceDataPoints] = useState<
    Record<string, InterfaceDataPoint[]>
  >({});

  // Clear stale chart data when the connection drops.
  useEffect(() => {
    if (!connected) {
      setDataPoints([]);
      setInterfaceDataPoints({});
    }
  }, [connected]);

  useEffect(() => {
    return subscribe('system_stats', (raw) => {
      const msg = raw as SystemStats;
      if (!msg) return;

      const net = msg.network?.[0];
      const point: StatsDataPoint = {
        timestamp: Date.now(),
        cpu: msg.cpu.usage_percent,
        memoryUsed: msg.memory.used_bytes,
        memoryTotal: msg.memory.total_bytes,
        rxBytes: net?.rx_bytes ?? 0,
        txBytes: net?.tx_bytes ?? 0,
      };
      setDataPoints((prev) => {
        const next = [...prev, point];
        return next.length > MAX_POINTS ? next.slice(next.length - MAX_POINTS) : next;
      });

      if (msg.network) {
        const ts = Date.now();
        setInterfaceDataPoints((prev) => {
          const updated = { ...prev };
          for (const iface of msg.network) {
            const key = iface.interface;
            const ifPoint: InterfaceDataPoint = { timestamp: ts, rxBytes: iface.rx_bytes, txBytes: iface.tx_bytes };
            const existing = updated[key] ?? [];
            const next = [...existing, ifPoint];
            updated[key] = next.length > MAX_POINTS ? next.slice(next.length - MAX_POINTS) : next;
          }
          return updated;
        });
      }
    });
  }, [subscribe]);

  return { dataPoints, interfaceDataPoints, connected };
}
```

- [ ] **Step 3: Run smoke tests — expect PASS**

Run: `cd frontend && pnpm test --run src/hooks/__tests__/use-websocket-smoke.test.ts`
Expected: all `PASS`

- [ ] **Step 4: Run full frontend tests**

Run: `cd frontend && pnpm test --run`
Expected: all `PASS`

- [ ] **Step 5: Commit**

```bash
git add frontend/src/hooks/use-websocket.ts \
        frontend/src/hooks/__tests__/use-websocket-smoke.test.ts
git commit -m "refactor(ws): useWebSocket consumes WsContext; clears data on disconnect"
```

---

## Task 7: Update useNetworkStatus to accept query options

**Files:**
- Modify: `frontend/src/hooks/use-network.ts`

`useTopologyData` needs to pass `staleTime: Infinity` so WS-fresh data isn't overwritten by a 30-second HTTP refetch. To do this cleanly, `useNetworkStatus` needs to accept optional query overrides.

- [ ] **Step 1: Update useNetworkStatus**

In `frontend/src/hooks/use-network.ts`, change:

```ts
// Before:
export function useNetworkStatus() {
  return useQuery({
    queryKey: ['network', 'status'],
    queryFn: () => apiClient.get<NetworkStatus>(API_ROUTES.network.status),
  });
}

// After:
import type { UseQueryOptions } from '@tanstack/react-query';

export function useNetworkStatus(options?: Partial<UseQueryOptions<NetworkStatus>>) {
  return useQuery({
    queryKey: ['network', 'status'],
    queryFn: () => apiClient.get<NetworkStatus>(API_ROUTES.network.status),
    ...options,
  });
}
```

All existing callers pass no arguments so this is backwards-compatible.

- [ ] **Step 2: Build to verify no type errors**

Run: `cd frontend && pnpm build`
Expected: clean

- [ ] **Step 3: Commit**

```bash
git add frontend/src/hooks/use-network.ts
git commit -m "refactor(network): useNetworkStatus accepts optional query options"
```

---

## Task 8: useTopologyData hook

**Files:**
- Create: `frontend/src/hooks/use-topology-data.ts`
- Create: `frontend/src/hooks/__tests__/use-topology-data.test.ts`

- [ ] **Step 1: Write the failing tests**

```ts
// frontend/src/hooks/__tests__/use-topology-data.test.ts
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook } from '@testing-library/react';
import React from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { WsProvider } from '@/lib/ws-context';
import { useTopologyData } from '../use-topology-data';
import type { NetworkStatus } from '@shared/index';

beforeEach(() => {
  vi.stubGlobal('WebSocket', vi.fn().mockImplementation(() => ({
    readyState: 1, close: vi.fn(),
    onopen: null, onmessage: null, onclose: null, onerror: null,
  })));
});

vi.mock('@/hooks/use-network', () => ({
  useNetworkStatus: vi.fn(() => ({ data: undefined, isLoading: false })),
  useIPv6Status: vi.fn(() => ({ data: undefined })),
}));
vi.mock('@/hooks/use-wifi', () => ({
  useWifiConnection: vi.fn(() => ({ data: undefined, isLoading: false })),
}));
vi.mock('@/hooks/use-vpn', () => ({
  useVpnStatus: vi.fn(() => ({ data: undefined })),
}));
vi.mock('@/hooks/use-system', () => ({
  useSystemInfo: vi.fn(() => ({ data: undefined, isLoading: false })),
}));
vi.mock('@/hooks/use-usb-tether', () => ({
  useUSBTetherStatus: vi.fn(() => ({ data: undefined })),
}));

import { useNetworkStatus } from '@/hooks/use-network';

function makeWrapper() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return function Wrapper({ children }: { children: React.ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>
        <WsProvider>{children}</WsProvider>
      </QueryClientProvider>
    );
  };
}

function makeWan(type: string, up: boolean): NetworkStatus['wan'] {
  return { name: 'wan', type: type as never, ip_address: '1.2.3.4', netmask: '', gateway: '', dns_servers: [], mac_address: '', is_up: up, rx_bytes: 0, tx_bytes: 0 };
}

describe('useTopologyData — connection type derivation', () => {
  const cases = [
    { wanType: 'wan',  wanUp: true,  ethernet: true,  repeater: false, tether: false },
    { wanType: 'wifi', wanUp: true,  ethernet: false, repeater: true,  tether: false },
    { wanType: 'usb',  wanUp: true,  ethernet: false, repeater: false, tether: true  },
    { wanType: 'wan',  wanUp: false, ethernet: false, repeater: false, tether: false },
  ];

  cases.forEach(({ wanType, wanUp, ethernet, repeater, tether }) => {
    it(`wan.type=${wanType} wan.is_up=${wanUp} → eth=${ethernet} rep=${repeater} tether=${tether}`, () => {
      const ns: Partial<NetworkStatus> = {
        wan: makeWan(wanType, wanUp),
        clients: [], internet_reachable: false,
        lan: makeWan('lan', true) as never,
        interfaces: [],
      };
      vi.mocked(useNetworkStatus).mockReturnValue({ data: ns as NetworkStatus, isLoading: false } as never);

      const { result } = renderHook(() => useTopologyData(), { wrapper: makeWrapper() });

      expect(result.current.ethernetUp).toBe(ethernet);
      expect(result.current.repeaterUp).toBe(repeater);
      expect(result.current.tetherUp).toBe(tether);
    });
  });
});
```

Run: `cd frontend && pnpm test --run src/hooks/__tests__/use-topology-data.test.ts`
Expected: **FAIL** — `useTopologyData` not found

- [ ] **Step 2: Implement useTopologyData**

```ts
// frontend/src/hooks/use-topology-data.ts
import { useEffect } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { Cable, Wifi, Smartphone, Signal } from 'lucide-react';
import type { LucideIcon } from 'lucide-react';
import type { NetworkStatus } from '@shared/index';
import { useNetworkStatus, useIPv6Status } from './use-network';
import { useWifiConnection } from './use-wifi';
import { useVpnStatus } from './use-vpn';
import { useSystemInfo } from './use-system';
import { useUSBTetherStatus } from './use-usb-tether';
import { useWsSubscribe } from '@/lib/ws-context';

export interface SourceDef {
  label: string;
  icon: LucideIcon;
  connected: boolean;
  detail?: string;
}

export interface TopologyData {
  // Derived display state
  sources: SourceDef[];
  clients: { label: string; icon: LucideIcon; count: number }[];
  features: { label: string; active: boolean }[];
  router: { hostname: string; model: string };
  loading: boolean;
  // Named booleans so components don't need to index into sources[]
  ethernetUp: boolean;
  repeaterUp: boolean;
  tetherUp: boolean;
  // Raw data needed by ExperimentalPage detail cards
  wan: NetworkStatus['wan'];
  wifiConn: ReturnType<typeof useWifiConnection>['data'];
  usbTether: ReturnType<typeof useUSBTetherStatus>['data'];
  sysInfo: ReturnType<typeof useSystemInfo>['data'];
  vpnActive: boolean;
  ipv6Enabled: boolean;
  internetUp: boolean;
  allClients: NonNullable<NetworkStatus['clients']>;
}

export function useTopologyData(): TopologyData {
  const queryClient = useQueryClient();
  const { subscribe } = useWsSubscribe();

  // staleTime: Infinity prevents HTTP refetches from overwriting WS-fresh data.
  const { data: network, isLoading: networkLoading } = useNetworkStatus({ staleTime: Infinity });
  const { data: wifiConn, isLoading: wifiLoading } = useWifiConnection();
  const { data: vpnStatus } = useVpnStatus();
  const { data: sysInfo, isLoading: sysLoading } = useSystemInfo();
  const { data: ipv6Status } = useIPv6Status();
  const { data: usbTether } = useUSBTetherStatus();

  // On network_status WS message, update the React Query cache.
  // All components using useNetworkStatus() benefit automatically.
  useEffect(() => {
    return subscribe('network_status', (raw) => {
      queryClient.setQueryData<NetworkStatus>(['network', 'status'], raw as NetworkStatus);
    });
  }, [subscribe, queryClient]);

  const wan = network?.wan ?? null;
  const ethernetUp = wan?.is_up === true && wan.type !== 'wifi' && wan.type !== 'usb';
  const repeaterUp =
    (wan?.is_up === true && wan.type === 'wifi') ||
    (wifiConn?.connected === true && wifiConn.mode === 'client');
  const tetherUp =
    (wan?.is_up === true && wan.type === 'usb') ||
    (usbTether?.is_up === true);
  const vpnActive = vpnStatus?.some((v) => v.connected) ?? false;
  const ipv6Enabled = ipv6Status?.enabled ?? false;
  const internetUp = network?.internet_reachable ?? false;

  const allClients = network?.clients ?? [];
  const wlanClients = allClients.filter(
    (c) =>
      c.interface_name.startsWith('wlan') ||
      c.interface_name.startsWith('ath') ||
      c.interface_name.includes('wifi'),
  ).length;
  const lanClients = allClients.length - wlanClients;

  return {
    sources: [
      { label: 'Ethernet',       icon: Cable,       connected: ethernetUp, detail: ethernetUp ? (wan?.ip_address ?? undefined) : undefined },
      { label: 'Repeater (WiFi)', icon: Wifi,        connected: repeaterUp, detail: repeaterUp ? (wifiConn?.ssid ?? wan?.ip_address ?? undefined) : 'Disabled' },
      { label: 'USB Tethering',  icon: Smartphone,  connected: tetherUp,   detail: tetherUp ? (usbTether?.device_type || wan?.ip_address || 'Connected') : 'No device' },
      { label: 'Cellular',       icon: Signal,      connected: false,      detail: 'No modem' },
    ],
    clients: [
      { label: 'WLAN Clients', icon: Wifi,  count: wlanClients },
      { label: 'LAN Clients',  icon: Cable, count: lanClients },
    ],
    features: [
      { label: 'IPv6',     active: ipv6Enabled },
      { label: 'VPN',      active: vpnActive },
      { label: 'Internet', active: internetUp },
    ],
    router: { hostname: sysInfo?.hostname ?? '', model: sysInfo?.model ?? '' },
    loading: networkLoading || wifiLoading || sysLoading,
    ethernetUp,
    repeaterUp,
    tetherUp,
    wan,
    wifiConn,
    usbTether,
    sysInfo,
    vpnActive,
    ipv6Enabled,
    internetUp,
    allClients,
  };
}
```

- [ ] **Step 3: Run tests — expect PASS**

Run: `cd frontend && pnpm test --run src/hooks/__tests__/use-topology-data.test.ts`
Expected: all `PASS`

- [ ] **Step 4: Commit**

```bash
git add frontend/src/hooks/use-topology-data.ts \
        frontend/src/hooks/__tests__/use-topology-data.test.ts
git commit -m "feat(topology): add useTopologyData hook with WS-driven cache updates"
```

---

## Task 9: Mount WsProvider in App.tsx

**Files:**
- Modify: `frontend/src/App.tsx`

`WsProvider` must be inside `QueryClientProvider` (because `useTopologyData` calls `useQueryClient` inside a WS message callback). `RouterProvider` goes inside `WsProvider`.

- [ ] **Step 1: Add WsProvider**

```tsx
import { WsProvider } from '@/lib/ws-context';

function App() {
  return (
    <ThemeProvider>
      <QueryClientProvider client={queryClient}>
        <WsProvider>
          <RouterProvider router={router} />
          <ThemedToaster />
        </WsProvider>
      </QueryClientProvider>
    </ThemeProvider>
  );
}
```

- [ ] **Step 2: Build**

Run: `cd frontend && pnpm build`
Expected: clean

- [ ] **Step 3: Commit**

```bash
git add frontend/src/App.tsx
git commit -m "feat(ws): mount WsProvider in App root"
```

---

## Task 10: Refactor experimental-page.tsx

**Files:**
- Modify: `frontend/src/pages/experimental/experimental-page.tsx`

`TopologyDiagram` and `SourceCard` sub-components are unchanged. Only `ExperimentalPage` changes.

- [ ] **Step 1: Update imports**

Remove the six individual hook imports:
```tsx
// Remove:
import { useNetworkStatus, useIPv6Status } from '@/hooks/use-network';
import { useVpnStatus } from '@/hooks/use-vpn';
import { useWifiConnection } from '@/hooks/use-wifi';
import { useSystemInfo } from '@/hooks/use-system';
import { useUSBTetherStatus } from '@/hooks/use-usb-tether';

// Add:
import { useTopologyData } from '@/hooks/use-topology-data';
```

Remove the local `SourceDef` and `ClientDef` interface definitions (now exported from `use-topology-data.ts`). Import `SourceDef` from there if `TopologyDiagram` needs the type.

- [ ] **Step 2: Replace ExperimentalPage body**

Replace everything inside `ExperimentalPage` from the hook calls down to the first `return`:

```tsx
export function ExperimentalPage() {
  const {
    sources, clients, features, router, loading,
    ethernetUp, repeaterUp, tetherUp,
    wan, wifiConn, usbTether, sysInfo,
    vpnActive, ipv6Enabled, internetUp, allClients,
  } = useTopologyData();

  return (
    // JSX below is unchanged from the current file
  );
}
```

The JSX for `<TopologyDiagram>`, the four `<SourceCard>` blocks, and the system info `<Card>` are unchanged — they already reference the same variable names, which are now destructured from the hook.

- [ ] **Step 3: Build and lint**

Run: `cd frontend && pnpm build && pnpm lint`
Expected: clean

- [ ] **Step 4: Commit**

```bash
git add frontend/src/pages/experimental/experimental-page.tsx
git commit -m "refactor(topology): ExperimentalPage uses useTopologyData hook"
```

---

## Task 11: Full verification

- [ ] **Step 1: Run all tests**

Run: `make test`
Expected: all backend + frontend tests pass

- [ ] **Step 2: Build everything**

Run: `make build`
Expected: clean

- [ ] **Step 3: Lint**

Run: `make lint`
Expected: no errors

- [ ] **Step 4: Deploy and smoke-test on device**

Ensure you are logged in at `http://192.168.1.1` first (the WebSocket uses the JWT from the login session). Then:

```bash
./deploy-local.sh
```

Open the Experimental page. Connect a phone to the router's WiFi. The WLAN Clients counter should increment within ~1 second without refreshing the page.

- [ ] **Step 5: Final commit if any fixups needed**

```bash
git add -p
git commit -m "fix(topology): post-review fixups"
```

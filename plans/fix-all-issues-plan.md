# Implementation Plan: Fix All Review Issues (Detailed)

## Phase 1 — Backend Core: Real UCI/Ubus, Error Handling, Graceful Shutdown

### Objective
Implement real UCI and ubus backends that shell out to `/sbin/uci` and `ubus` CLI tools on OpenWrt, propagate UCI write errors instead of swallowing them, and add graceful shutdown with signal handling throughout the server lifecycle.

### Issues Addressed
- **#1** No real UCI implementation
- **#2** No real ubus implementation
- **#15** UCI write errors swallowed
- **#19** No graceful shutdown

### Files to Create
- `backend/internal/uci/real.go` — `RealUCI` struct implementing `UCI` interface via `exec.CommandContext` to `/sbin/uci`
- `backend/internal/uci/real_test.go` — integration-style tests (skipped unless `OPENWRT=1` env var set)
- `backend/internal/ubus/real.go` — `RealUbus` struct implementing `Ubus` interface via `exec.CommandContext` to `ubus call`
- `backend/internal/ubus/real_test.go` — integration-style tests (skipped unless `OPENWRT=1`)

### Files to Modify
- `backend/internal/services/network_service.go`
  - `SetWanConfig()`: change all `_ = n.uci.Set(...)` lines to `if err := n.uci.Set(...); err != nil { return fmt.Errorf("set %s: %w", option, err) }`
- `backend/internal/services/wifi_service.go`
  - `Connect()`: same error-propagation pattern for all `_ = w.uci.Set(...)` calls
  - `Disconnect()`: same
  - `SetMode()`: same
- `backend/internal/services/vpn_service.go`
  - `SetWireguardConfig()`: same error-propagation pattern for all `_ = v.uci.Set(...)` calls
  - `ToggleWireguard()`: same
- `backend/cmd/server/main.go`
  - Replace `app.Listen(addr)` with `go app.Listen(addr)` + `signal.NotifyContext` pattern
  - On context cancel: call `hub.Stop()`, `app.ShutdownWithContext(ctx)` with a 10s timeout
  - In the `else` branch of `cfg.MockMode`, instantiate `uci.NewRealUCI()` and `ubus.NewRealUbus()` instead of mock

### Tests to Write First (TDD)
- `backend/internal/uci/real_test.go`
  - `TestRealUCI_SkipsWithoutEnv` — verifies test is skipped when `OPENWRT=1` not set
  - `TestRealUCI_GetSetCommit` — (gated) calls Get/Set/Commit against real uci
- `backend/internal/ubus/real_test.go`
  - `TestRealUbus_SkipsWithoutEnv` — verifies test is skipped
  - `TestRealUbus_CallSystemBoard` — (gated) calls system board
- `backend/internal/services/network_service_test.go`
  - `TestSetWanConfig_PropagatesUCIError` — mock UCI's `Set` to return error, assert `SetWanConfig` returns that error
- `backend/internal/services/vpn_service_test.go`
  - `TestSetWireguardConfig_PropagatesUCIError`
  - `TestToggleWireguard_PropagatesUCIError`
- `backend/internal/services/wifi_service_test.go`
  - `TestConnect_PropagatesUCIError`
  - `TestDisconnect_PropagatesUCIError`
- `backend/cmd/server/main_test.go`
  - `TestGracefulShutdown` — start app in goroutine, send SIGTERM, assert exits cleanly within 5s

### Implementation Steps
1. **Write failing error-propagation tests.** In `network_service_test.go`, `vpn_service_test.go`, `wifi_service_test.go`: create a mock UCI that wraps `MockUCI` but overrides `Set()` to return `errors.New("mock write error")`. Call `SetWanConfig`, `SetWireguardConfig`, `Connect`, etc. Assert the returned error is non-nil and contains "mock write error".
2. **Fix all swallowed errors.** In `network_service.go` `SetWanConfig()`:
   ```go
   func (n *NetworkService) SetWanConfig(config models.WanConfig) error {
       if config.Type != "" {
           if err := n.uci.Set("network", "wan", "proto", config.Type); err != nil {
               return fmt.Errorf("set proto: %w", err)
           }
       }
       // ... same pattern for IPAddress, Netmask, Gateway
       return n.uci.Commit("network")
   }
   ```
   Same in `wifi_service.go` `Connect()`, `Disconnect()`, `SetMode()` and `vpn_service.go` `SetWireguardConfig()`, `ToggleWireguard()`.
3. **Run tests** — error-propagation tests should now pass.
4. **Create `backend/internal/uci/real.go`:**
   ```go
   package uci

   import (
       "bytes"
       "context"
       "fmt"
       "os/exec"
       "strings"
       "time"
   )

   type RealUCI struct{}

   func NewRealUCI() *RealUCI { return &RealUCI{} }

   var _ UCI = (*RealUCI)(nil) // compile-time check

   func (r *RealUCI) run(args ...string) (string, error) {
       ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
       defer cancel()
       cmd := exec.CommandContext(ctx, "/sbin/uci", args...)
       var stdout, stderr bytes.Buffer
       cmd.Stdout = &stdout
       cmd.Stderr = &stderr
       if err := cmd.Run(); err != nil {
           return "", fmt.Errorf("uci %s: %s: %w", strings.Join(args, " "), stderr.String(), err)
       }
       return strings.TrimSpace(stdout.String()), nil
   }

   func (r *RealUCI) Get(config, section, option string) (string, error) {
       return r.run("get", fmt.Sprintf("%s.%s.%s", config, section, option))
   }

   func (r *RealUCI) Set(config, section, option, value string) error {
       _, err := r.run("set", fmt.Sprintf("%s.%s.%s=%s", config, section, option, value))
       return err
   }

   func (r *RealUCI) GetAll(config, section string) (map[string]string, error) {
       out, err := r.run("show", fmt.Sprintf("%s.%s", config, section))
       if err != nil {
           return nil, err
       }
       result := make(map[string]string)
       prefix := fmt.Sprintf("%s.%s.", config, section)
       for _, line := range strings.Split(out, "\n") {
           line = strings.TrimSpace(line)
           if !strings.HasPrefix(line, prefix) {
               continue
           }
           kv := strings.SplitN(strings.TrimPrefix(line, prefix), "=", 2)
           if len(kv) == 2 {
               val := strings.Trim(kv[1], "'")
               result[kv[0]] = val
           }
       }
       return result, nil
   }

   func (r *RealUCI) Commit(config string) error {
       _, err := r.run("commit", config)
       return err
   }

   func (r *RealUCI) AddSection(config, section, stype string) error {
       _, err := r.run("set", fmt.Sprintf("%s.%s=%s", config, section, stype))
       return err
   }

   func (r *RealUCI) DeleteSection(config, section string) error {
       _, err := r.run("delete", fmt.Sprintf("%s.%s", config, section))
       return err
   }
   ```
5. **Create `backend/internal/ubus/real.go`:**
   ```go
   package ubus

   import (
       "bytes"
       "context"
       "encoding/json"
       "fmt"
       "os/exec"
       "time"
   )

   type RealUbus struct{}

   func NewRealUbus() *RealUbus { return &RealUbus{} }

   var _ Ubus = (*RealUbus)(nil) // compile-time check

   func (r *RealUbus) Call(path, method string, args map[string]interface{}) (map[string]interface{}, error) {
       ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
       defer cancel()

       cmdArgs := []string{"call", path, method}
       if args != nil {
           jsonBytes, err := json.Marshal(args)
           if err != nil {
               return nil, fmt.Errorf("ubus marshal args: %w", err)
           }
           cmdArgs = append(cmdArgs, string(jsonBytes))
       }

       cmd := exec.CommandContext(ctx, "ubus", cmdArgs...)
       var stdout, stderr bytes.Buffer
       cmd.Stdout = &stdout
       cmd.Stderr = &stderr
       if err := cmd.Run(); err != nil {
           return nil, fmt.Errorf("ubus call %s %s: %s: %w", path, method, stderr.String(), err)
       }

       var result map[string]interface{}
       if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
           return nil, fmt.Errorf("ubus parse response: %w", err)
       }
       return result, nil
   }
   ```
6. **Create integration tests** in `real_test.go` files guarded by `if os.Getenv("OPENWRT") != "1" { t.Skip("skipping: set OPENWRT=1 to run on real device") }`.
7. **Update `main.go` for graceful shutdown:**
   ```go
   func main() {
       cfg := config.LoadConfigFromEnv()
       app := setupAppWithConfig(cfg)
       addr := fmt.Sprintf(":%d", cfg.Port)
       log.Printf("Starting openwrt-travel-gui backend on %s (mock=%v)", addr, cfg.MockMode)

       ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
       defer stop()

       go func() {
           if err := app.Listen(addr); err != nil {
               log.Printf("Server error: %v", err)
           }
       }()

       <-ctx.Done()
       log.Println("Shutting down...")
       shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
       defer cancel()
       if err := app.ShutdownWithContext(shutdownCtx); err != nil {
           log.Printf("Shutdown error: %v", err)
       }
   }
   ```
   Also update the non-mock branch to use `uci.NewRealUCI()` and `ubus.NewRealUbus()`.
8. **Write `TestGracefulShutdown`** in `main_test.go`: start `setupApp()` in a goroutine binding to a random port, wait for listen, send OS signal or cancel context, assert clean exit.
9. **Run `cd backend && go test ./... -count=1`** — all must pass.

### Acceptance Criteria
- All `uci.Set` errors propagated — zero `_ =` assignments remain for `Set` calls in service files
- `RealUCI` implements the `UCI` interface (compile-time check via `var _ UCI = (*RealUCI)(nil)`)
- `RealUbus` implements the `Ubus` interface (same compile-time check)
- Server shuts down cleanly on SIGTERM within 10 seconds
- `go test ./...` passes with 0 failures

### Complexity
**L**

---

## Phase 2 — Backend Security: JWT Blocklist, Login Rate-Limiting, WebSocket Auth, CORS

### Objective
Harden the backend authentication layer by adding a JWT blocklist for real logout, rate-limiting on the login endpoint, authenticating WebSocket upgrade requests, and configuring proper CORS middleware.

### Issues Addressed
- **#4** WebSocket no auth
- **#13** JWT blocklist for logout
- **#14** Login rate-limiting
- **#27** CORS middleware

### Files to Create
- `backend/internal/auth/blocklist.go` — `TokenBlocklist` struct with `Add(tokenStr string, expiry time.Time)` and `IsBlocked(tokenStr string) bool`; uses `sync.Map` + periodic cleanup goroutine
- `backend/internal/auth/blocklist_test.go`
- `backend/internal/auth/ratelimit.go` — `LoginRateLimiter` struct using a sliding-window per-IP counter (in-memory map with mutex); `Allow(ip string) bool`
- `backend/internal/auth/ratelimit_test.go`

### Files to Modify
- `backend/internal/auth/auth.go`
  - Add `Blocklist *TokenBlocklist` field to `AuthService`
  - `NewAuthService(...)`: initialize blocklist, start cleanup goroutine
  - `ValidateToken(tokenStr)`: add `if a.Blocklist.IsBlocked(tokenStr) { return errTokenRevoked }` check before JWT parsing
  - New method `Logout(tokenStr string) error`: parse token to get expiry claim, call `Blocklist.Add(tokenStr, expiry)`
  - New helper method `ExtractToken(authHeader string) (string, error)`: splits "Bearer <token>" — DRY for middleware + ws
- `backend/internal/api/auth_handlers.go`
  - `LogoutHandler`: change signature to accept `*auth.AuthService`; extract token from `Authorization` header; call `authSvc.Logout(token)`; return `{"status": "ok"}`
- `backend/internal/api/router.go`
  - Pass `deps.Auth` to `LogoutHandler(deps.Auth)`
  - Add rate limiter middleware to login route
- `backend/internal/ws/handler.go`
  - `UpgradeMiddleware`: change signature to accept `*auth.AuthService`; extract `token` query param or `Authorization` header; validate via `authSvc.ValidateToken(token)`; reject with 401 if invalid
- `backend/cmd/server/main.go`
  - Add CORS middleware: import `"github.com/gofiber/fiber/v2/middleware/cors"`, configure:
    ```go
    app.Use(cors.New(cors.Config{
        AllowOrigins: cfg.CORSOrigins,
        AllowHeaders: "Authorization,Content-Type",
        AllowMethods: "GET,POST,PUT,DELETE",
    }))
    ```
    before auth middleware
  - Pass `authSvc` to WS `UpgradeMiddleware(authSvc)`
  - Create `LoginRateLimiter` in main, pass to login handler setup
- `backend/internal/config/config.go`
  - Add `CORSOrigins string` field, load from `CORS_ORIGINS` env, default `"http://localhost:5173"`
  - Add `RateLimitPerMinute int` field, load from `RATE_LIMIT` env, default `10`

### Tests to Write First (TDD)
- `backend/internal/auth/blocklist_test.go`
  - `TestBlocklist_AddAndCheck` — add token, assert `IsBlocked` returns true
  - `TestBlocklist_ExpiredCleanup` — add token with past expiry, trigger cleanup, assert not blocked
  - `TestBlocklist_UnblockedToken` — assert unknown token returns false
- `backend/internal/auth/ratelimit_test.go`
  - `TestRateLimiter_AllowsUnderLimit` — 10 calls same IP, all return true
  - `TestRateLimiter_BlocksOverLimit` — 11th call returns false
  - `TestRateLimiter_ResetsAfterWindow` — wait for window expiry, call again, returns true
- `backend/internal/auth/auth_test.go`
  - `TestLogout_InvalidatesToken` — login, get token, logout with it, assert ValidateToken returns error
- `backend/internal/api/auth_handlers_test.go`
  - `TestLogoutHandler_RevokesToken` — POST /logout with valid token succeeds, then GET /session with same token → 401
- `backend/internal/ws/handler_test.go` (new)
  - `TestWSUpgrade_RequiresAuth` — attempt WS upgrade without token → 401
  - `TestWSUpgrade_AcceptsValidToken` — upgrade with valid `?token=` query param → success

### Implementation Steps
1. **Write `blocklist_test.go`.** Implement `blocklist.go`:
   ```go
   type TokenBlocklist struct {
       tokens sync.Map // map[string]time.Time (token → expiry)
   }

   func NewTokenBlocklist() *TokenBlocklist {
       bl := &TokenBlocklist{}
       go bl.cleanupLoop(5 * time.Minute)
       return bl
   }

   func (bl *TokenBlocklist) Add(token string, expiry time.Time) {
       bl.tokens.Store(token, expiry)
   }

   func (bl *TokenBlocklist) IsBlocked(token string) bool {
       _, ok := bl.tokens.Load(token)
       return ok
   }

   func (bl *TokenBlocklist) cleanupLoop(interval time.Duration) {
       ticker := time.NewTicker(interval)
       defer ticker.Stop()
       for range ticker.C {
           now := time.Now()
           bl.tokens.Range(func(key, value interface{}) bool {
               if expiry, ok := value.(time.Time); ok && now.After(expiry) {
                   bl.tokens.Delete(key)
               }
               return true
           })
       }
   }
   ```
2. **Write `ratelimit_test.go`.** Implement `ratelimit.go`:
   ```go
   type LoginRateLimiter struct {
       mu       sync.Mutex
       attempts map[string][]time.Time
       max      int
       window   time.Duration
   }

   func NewLoginRateLimiter(maxPerMinute int) *LoginRateLimiter {
       return &LoginRateLimiter{
           attempts: make(map[string][]time.Time),
           max:      maxPerMinute,
           window:   time.Minute,
       }
   }

   func (rl *LoginRateLimiter) Allow(ip string) bool {
       rl.mu.Lock()
       defer rl.mu.Unlock()
       now := time.Now()
       cutoff := now.Add(-rl.window)
       // Prune old entries
       var recent []time.Time
       for _, t := range rl.attempts[ip] {
           if t.After(cutoff) {
               recent = append(recent, t)
           }
       }
       if len(recent) >= rl.max {
           rl.attempts[ip] = recent
           return false
       }
       rl.attempts[ip] = append(recent, now)
       return true
   }
   ```
3. **Modify `auth.go`:** Add `Blocklist` field. In `ValidateToken`, check blocklist first. Add `Logout(tokenStr)` method that parses expiry from claims and adds to blocklist. Add `ExtractToken(header)` helper.
4. **Update `LogoutHandler`:** Accept `*auth.AuthService`, extract bearer token, call `authSvc.Logout(token)`.
5. **Modify `UpgradeMiddleware`:** Accept `*auth.AuthService`. Check `c.Query("token")` first, then `c.Get("Authorization")`. Validate. Return `fiber.ErrUnauthorized` on failure.
6. **Update `main.go`:** Add CORS middleware before auth middleware. Create rate limiter. Pass to login handler. Pass authSvc to WS middleware+handler.
7. **Add login rate-limit:** Wrap `LoginHandler` or add check at start: `if !rateLimiter.Allow(c.IP()) { return c.Status(429).JSON(...) }`.
8. **Update `config.go`** with `CORSOrigins` and `RateLimitPerMinute`.
9. **Run all tests.** `go test ./... -count=1`.

### Acceptance Criteria
- POST `/api/v1/auth/logout` with valid token → that token is rejected on subsequent requests (ValidateToken returns error)
- WebSocket upgrade without a valid token returns HTTP 401
- WebSocket upgrade with `?token=<valid-jwt>` succeeds
- 11 login attempts in 1 minute from same IP → HTTP 429
- CORS `Access-Control-Allow-Origin` header present on responses
- `go test ./...` passes

### Complexity
**L**

---

## Phase 3 — Backend Data Accuracy: Real System Stats, Clients, Captive Portal

### Objective
Replace all hardcoded/stubbed data with real implementations: read CPU usage from `/proc/stat`, storage from `syscall.Statfs`, connected clients from DHCP leases + ARP table, and implement actual captive portal detection via HTTP probe.

### Issues Addressed
- **#3** Captive portal stubbed
- **#16** Hardcoded clients in NetworkService
- **#17** Hardcoded storage stats
- **#18** CPU usage = load average (incorrect)

### Files to Create
- `backend/internal/services/cpu_linux.go` — build-tagged `//go:build linux`; reads `/proc/stat` twice 200ms apart, computes real CPU percentage
- `backend/internal/services/cpu_other.go` — build-tagged `//go:build !linux`; fallback returning load-average-based estimate
- `backend/internal/services/cpu_test.go`
- `backend/internal/services/storage.go` — `readStorageStats() (models.StorageStats, error)` using `syscall.Statfs` on `/`
- `backend/internal/services/storage_test.go`
- `backend/internal/services/clients.go` — `ReadConnectedClients(leasePath string) ([]models.Client, error)` parsing DHCP leases + ARP table
- `backend/internal/services/clients_test.go`

### Files to Modify
- `backend/internal/services/system_service.go`
  - `GetSystemStats()`: replace hardcoded `stats.Storage` block with `readStorageStats()`
  - `GetSystemStats()`: replace `stats.CPU.UsagePercent = (l1/65536)*100` with `readCPUUsage()`; keep load averages
  - Add `stats.UptimeSeconds` from `info["uptime"]`
- `backend/internal/models/system.go`
  - Add `UptimeSeconds int64 \`json:"uptime_seconds"\`` to `SystemStats`
- `backend/internal/services/network_service.go`
  - `GetNetworkStatus()`: replace hardcoded `status.Clients` with `ReadConnectedClients("/tmp/dhcp.leases")`
  - `GetClients()`: call `ReadConnectedClients` directly
- `backend/internal/services/captive_service.go`
  - Add `mockMode bool` and `checkURL string` fields to struct
  - `NewCaptiveService(mockMode bool)`: set checkURL to `http://connectivitycheck.gstatic.com/generate_204`
  - `CheckCaptivePortal()`: if mockMode, return static result; else HTTP GET to checkURL with 5s timeout:
    - Status 204 → `Detected: false, CanReachInternet: true`
    - Status 302 → `Detected: true, PortalURL: Location header, CanReachInternet: true`
    - Error → `Detected: false, CanReachInternet: false`
- `backend/cmd/server/main.go`
  - Pass `cfg.MockMode` to `NewCaptiveService(cfg.MockMode)`

### Tests to Write First (TDD)
- `backend/internal/services/cpu_test.go`
  - `TestParseProcStat` — pass sample content lines, assert user/system/idle values
  - `TestCPUUsagePercent` — given two snapshots, compute expected delta percent
- `backend/internal/services/storage_test.go`
  - `TestReadStorageStats` — on any OS, returns non-zero total (real syscall)
- `backend/internal/services/clients_test.go`
  - `TestParseDHCPLeases` — sample input: `1700000000 aa:bb:cc:11:22:33 192.168.8.100 laptop *\n1700000000 aa:bb:cc:44:55:66 192.168.8.101 phone *`
  - `TestParseARPTable` — sample `/proc/net/arp` content
  - `TestMergeClientsWithARP` — merged result has correct MAC addresses
- `backend/internal/services/captive_service_test.go`
  - `TestCaptivePortal_MockMode` — returns `Detected: false, CanReachInternet: true`
  - `TestCaptivePortal_DetectsCaptive` — httptest server returns 302 + Location header
  - `TestCaptivePortal_DetectsInternet` — httptest server returns 204
  - `TestCaptivePortal_NoInternet` — unreachable URL → CanReachInternet: false
- `backend/internal/services/system_service_test.go`
  - `TestGetSystemStats_IncludesUptime` — assert UptimeSeconds > 0

### Implementation Steps
1. **CPU parsing.** Write `cpu_test.go` with sample `/proc/stat` content:
   ```
   cpu  4705 356 584 3699 23 0 20 0 0 0
   cpu0 2352 178 292 1849 11 0 10 0 0 0
   cpu1 2353 178 292 1850 12 0 10 0 0 0
   ```
   Implement `parseProcStat(content string) (user, nice, system, idle, iowait int64)` and `readCPUUsage() (percent float64, cores int, err error)` that reads `/proc/stat` twice 200ms apart. `cpu_other.go` returns `(loadAvg1 / cores * 100, cores, nil)` as fallback.
2. **Storage.** Implement `readStorageStats()` using `syscall.Statfs(path, &fs)` → compute total/free/used from `fs.Blocks * fs.Bsize`, etc.
3. **Clients parsing.** Format of `/tmp/dhcp.leases`: `<timestamp> <mac> <ip> <hostname> <clientid>`. Parse each line. Also parse `/proc/net/arp` (skip header line), columns: `IP address, HW type, Flags, HW address, Mask, Device`. Merge: for each lease entry, look up MAC in ARP; if present, confirm. Return `[]models.Client`.
4. **Update `network_service.go`:** In `GetNetworkStatus()`, replace hardcoded clients:
   ```go
   clients, err := ReadConnectedClients("/tmp/dhcp.leases")
   if err != nil {
       clients = []models.Client{} // degrade gracefully
   }
   status.Clients = clients
   ```
5. **Captive portal.** Add `mockMode` and `httpClient *http.Client` to `CaptiveService`. In `CheckCaptivePortal()`:
   ```go
   resp, err := c.httpClient.Get(c.checkURL)
   if err != nil {
       return models.CaptivePortalStatus{CanReachInternet: false}, nil
   }
   defer resp.Body.Close()
   if resp.StatusCode == 204 {
       return models.CaptivePortalStatus{CanReachInternet: true}, nil
   }
   if resp.StatusCode == 302 || resp.StatusCode == 301 {
       loc := resp.Header.Get("Location")
       return models.CaptivePortalStatus{Detected: true, PortalURL: &loc, CanReachInternet: true}, nil
   }
   ```
   In tests, use `httptest.NewServer` to simulate responses.
6. **Update `system_service.go`:** Replace storage with `readStorageStats()`, CPU with `readCPUUsage()`, add uptime to stats.
7. **Add `UptimeSeconds` to `models.SystemStats`.**
8. **Update `main.go`:** Pass `cfg.MockMode` to captive service constructor.
9. **Run all tests.**

### Acceptance Criteria
- CPU `usage_percent` comes from `/proc/stat` delta on Linux, not load average
- `SystemStats` includes real `uptime_seconds`
- Storage stats reflect real filesystem (`syscall.Statfs`)
- Connected clients parsed from DHCP leases, not hardcoded
- Captive portal makes real HTTP probe in non-mock mode
- All tests pass

### Complexity
**L**

---

## Phase 4 — Shared Types & Frontend API Plumbing: Routes Sync, API_ROUTES Usage, 401 Redirect

### Objective
Synchronize the shared `API_ROUTES` object with actual backend routes, make all frontend hooks use `API_ROUTES` constants instead of hardcoded strings, add 401-to-login redirect in the API client, and fix the `useToggleWireguard` hook to pass the enable state.

### Issues Addressed
- **#6** Shared routes out of sync
- **#8** No 401 redirect in api-client
- **#9** API_ROUTES not used in frontend hooks
- **#20** useToggleWireguard doesn't pass enable state
- **#21** WifiScan has enabled:false (no auto-scan)

### Files to Create
_None_

### Files to Modify
- **`shared/src/api/routes.ts`**
  - Fix `vpn.wireguard.config` → `/api/v1/vpn/wireguard` (backend has `/api/v1/vpn/wireguard`, not `/api/v1/vpn/wireguard/config`)
  - Fix `vpn.tailscale.status` → `/api/v1/vpn/tailscale` (backend has `/api/v1/vpn/tailscale`, not `/api/v1/vpn/tailscale/status`)
  - Add missing `wifi.connection: '/api/v1/wifi/connection'`
  - Add helper function: `export function serviceRoute(id: string, action: 'install' | 'remove' | 'start' | 'stop'): string { return \`/api/v1/services/${id}/${action}\`; }`
- **`frontend/src/lib/api-client.ts`**
  - In `request()`, after `if (!response.ok)`, add:
    ```typescript
    if (response.status === 401 && !path.endsWith('/auth/login')) {
        clearToken();
        window.location.href = '/login';
        throw new Error('Session expired');
    }
    ```
- **`frontend/src/hooks/use-wifi.ts`**
  - Import `{ API_ROUTES }` from `@shared/index`
  - Replace all 6 hardcoded path strings with `API_ROUTES.wifi.*`
  - `useWifiScan()`: remove `enabled: false`, add `staleTime: 10_000` (data stays fresh for 10s)
- **`frontend/src/hooks/use-vpn.ts`**
  - Import and use `API_ROUTES` for all 6 paths
  - `useToggleWireguard()`: change `mutationFn` from `() => apiClient.post(...)` to `(enable: boolean) => apiClient.post(API_ROUTES.vpn.wireguard.toggle, { enable })`
- **`frontend/src/hooks/use-network.ts`**
  - Import and use `API_ROUTES` for all 4 paths
- **`frontend/src/hooks/use-system.ts`**
  - Import and use `API_ROUTES` for both paths
- **`frontend/src/hooks/use-services.ts`**
  - Import `{ API_ROUTES, serviceRoute }` from `@shared/index`
  - Use `API_ROUTES.services.list` for list
  - Use `serviceRoute(id, 'install')` etc. for mutations
- **`frontend/src/hooks/use-captive-portal.ts`**
  - Import and use `API_ROUTES.captive.status`
- **`frontend/src/stores/auth-store.ts`**
  - Import `{ API_ROUTES }` from `@shared/index`, use `API_ROUTES.auth.login`
- **`frontend/src/pages/dashboard/system-stats-card.tsx`**
  - Replace inline `useQuery` with `useSystemStats()` hook import from `@/hooks/use-system`

### Tests to Write First (TDD)
- `shared/src/__tests__/routes.test.ts`
  - `test('API_ROUTES.wifi.connection is defined')` — assert equals `/api/v1/wifi/connection`
  - `test('API_ROUTES.vpn.wireguard.config matches backend route')` — assert equals `/api/v1/vpn/wireguard`
  - `test('API_ROUTES.vpn.tailscale.status matches backend route')` — assert equals `/api/v1/vpn/tailscale`
  - `test('serviceRoute generates correct path')` — assert `serviceRoute('adguardhome', 'install')` equals `/api/v1/services/adguardhome/install`
- `frontend/src/lib/__tests__/api-client.test.ts`
  - `test('redirects to /login on 401')` — mock `fetch` to return 401, call `apiClient.get('/api/v1/test')`, assert `window.location.href` changed to `/login` and token was cleared
  - `test('does not redirect on 401 for login endpoint')` — mock fetch 401 for `/api/v1/auth/login`, assert no redirect

### Implementation Steps
1. **Write shared routes tests.** Update `routes.ts`:
   ```typescript
   export const API_ROUTES = {
     auth: {
       login: '/api/v1/auth/login',
       logout: '/api/v1/auth/logout',
       session: '/api/v1/auth/session',
     },
     system: {
       info: '/api/v1/system/info',
       stats: '/api/v1/system/stats',
     },
     network: {
       status: '/api/v1/network/status',
       wan: '/api/v1/network/wan',
       clients: '/api/v1/network/clients',
     },
     wifi: {
       scan: '/api/v1/wifi/scan',
       connect: '/api/v1/wifi/connect',
       disconnect: '/api/v1/wifi/disconnect',
       connection: '/api/v1/wifi/connection',
       mode: '/api/v1/wifi/mode',
       saved: '/api/v1/wifi/saved',
     },
     vpn: {
       status: '/api/v1/vpn/status',
       wireguard: {
         config: '/api/v1/vpn/wireguard',
         toggle: '/api/v1/vpn/wireguard/toggle',
       },
       tailscale: {
         status: '/api/v1/vpn/tailscale',
         toggle: '/api/v1/vpn/tailscale/toggle',
       },
     },
     services: {
       list: '/api/v1/services',
     },
     captive: {
       status: '/api/v1/captive/status',
     },
   } as const;

   export function serviceRoute(id: string, action: 'install' | 'remove' | 'start' | 'stop'): string {
     return `/api/v1/services/${id}/${action}`;
   }
   ```
2. **Write api-client 401 test.** Update `api-client.ts`: add 401 redirect logic inside `request()` function, before the error throw. Guard against redirect loop on the login endpoint.
3. **Update all hooks** to import and use `API_ROUTES` and `serviceRoute`. For each hook file:
   - Add `import { API_ROUTES } from '@shared/index';` (or `import { API_ROUTES, serviceRoute }` for services)
   - Replace every string literal path with the corresponding `API_ROUTES.x.y` constant
4. **Fix `useToggleWireguard`:** Change signature to accept `enable: boolean`:
   ```typescript
   export function useToggleWireguard() {
     const queryClient = useQueryClient();
     return useMutation({
       mutationFn: (enable: boolean) =>
         apiClient.post<{ success: boolean }>(API_ROUTES.vpn.wireguard.toggle, { enable }),
       onSuccess: () => {
         void queryClient.invalidateQueries({ queryKey: ['vpn'] });
       },
     });
   }
   ```
5. **Fix `useWifiScan`:** Remove `enabled: false`, add `staleTime: 10_000`.
6. **Update `auth-store.ts`** to use `API_ROUTES.auth.login`.
7. **Update `system-stats-card.tsx`** to use `useSystemStats()` instead of inline `useQuery`.
8. **Run `cd shared && pnpm test`** and **`cd frontend && pnpm test`.**

### Acceptance Criteria
- `API_ROUTES` matches all actual backend routes (no stale/wrong paths)
- Zero hardcoded `/api/v1/...` strings remain in any hook file
- 401 responses from API automatically redirect to `/login` and clear token
- `useToggleWireguard` accepts a `boolean` parameter and sends it in the request body
- `useWifiScan` auto-fetches when component mounts
- All shared and frontend tests pass

### Complexity
**M**

---

## Phase 5 — Backend Input Validation & Frontend Quick Actions

### Objective
Add robust request body validation to all mutation handlers (reject malformed payloads with 400 + field-specific error messages), implement the three Quick Actions on the dashboard (Restart WiFi, Toggle VPN, Reboot), and add corresponding backend endpoints.

### Issues Addressed
- **#5** Quick Actions TODOs
- **#7** Missing input validation in handlers
- **#29** Form validation feedback

### Files to Create
- `backend/internal/api/validation.go` — validation helpers: `validateRequired(value, field)`, `validateIPAddress(value)`, `validateInSet(value, allowed)`, `validateMTU(mtu)`, `validateSSID(ssid)`, `validateBase64(value)`
- `backend/internal/api/validation_test.go`

### Files to Modify
- **`backend/internal/api/wifi_handlers.go`**
  - `WifiConnectHandler`: after body parse, validate `config.SSID` non-empty, validate `config.Password` non-empty when `config.Encryption != "none" && config.Encryption != ""`
  - `WifiSetModeHandler`: validate `body.Mode` in `["ap", "sta", "repeater"]`
- **`backend/internal/api/vpn_handlers.go`**
  - `SetWireguardHandler`: validate `config.PrivateKey` non-empty, `config.Address` matches CIDR pattern
- **`backend/internal/api/network_handlers.go`**
  - `SetWanConfigHandler`: validate `config.Type` in `["dhcp", "static", "pppoe"]`; if `static`: require `IPAddress`, `Netmask`, `Gateway` non-empty and valid IP format; validate MTU in range 68–9000
- **`backend/internal/api/auth_handlers.go`**
  - `LoginHandler`: validate `req.Password` non-empty before calling login
- **`backend/internal/api/system_handlers.go`**
  - Add `RebootHandler(svc *services.SystemService)` — calls `svc.Reboot()`
  - Add `RestartWifiHandler(svc *services.SystemService)` — calls `svc.RestartWifi()`
- **`backend/internal/api/router.go`**
  - Add routes: `v1.Post("/system/reboot", RebootHandler(deps.System))` and `v1.Post("/system/restart-wifi", RestartWifiHandler(deps.System))`
- **`backend/internal/services/system_service.go`**
  - Add `mockMode bool` field; update constructor `NewSystemService(ub ubus.Ubus, mockMode bool)`
  - Add `Reboot() error` — mock: log + no-op; real: `exec.Command("reboot").Run()`
  - Add `RestartWifi() error` — mock: no-op; real: `exec.Command("wifi", "down").Run()` then `exec.Command("wifi").Run()`
- **`backend/cmd/server/main.go`**
  - Update `NewSystemService(ub, cfg.MockMode)`
- **`shared/src/api/routes.ts`**
  - Add `system.reboot: '/api/v1/system/reboot'` and `system.restartWifi: '/api/v1/system/restart-wifi'`
- **`frontend/src/pages/dashboard/quick-actions.tsx`**
  - Import hooks: `useToggleWireguard`, `useVpnStatus` from `use-vpn`; `API_ROUTES` from shared; `apiClient`; `useMutation`
  - Implement `handleRestartWifi`: `useMutation` calling `apiClient.post(API_ROUTES.system.restartWifi)`
  - Implement `handleToggleVpn`: use `useVpnStatus()` to get current WG state, call `useToggleWireguard().mutate(!isEnabled)`
  - Implement `handleReboot`: `useMutation` calling `apiClient.post(API_ROUTES.system.reboot)` — add confirmation dialog
  - Add loading/disabled states to all three buttons

### Tests to Write First (TDD)
- `backend/internal/api/validation_test.go`
  - `TestValidateRequired_Empty` → error
  - `TestValidateRequired_NonEmpty` → nil
  - `TestValidateIPAddress_Valid` → nil ("192.168.1.1")
  - `TestValidateIPAddress_Invalid` → error ("not-an-ip")
  - `TestValidateMTU_TooLow` → error (10)
  - `TestValidateMTU_TooHigh` → error (10000)
  - `TestValidateMTU_Valid` → nil (1500)
  - `TestValidateSSID_Empty` → error
  - `TestValidateSSID_TooLong` → error (33 chars)
  - `TestValidateInSet_NotInSet` → error
  - `TestValidateInSet_InSet` → nil
- `backend/internal/api/wifi_handlers_test.go`
  - `TestWifiConnect_EmptySSID_Returns400`
  - `TestWifiConnect_ValidPayload_Returns200`
  - `TestWifiSetMode_InvalidMode_Returns400`
- `backend/internal/api/network_handlers_test.go`
  - `TestSetWanConfig_InvalidIP_Returns400`
  - `TestSetWanConfig_InvalidMTU_Returns400`
  - `TestSetWanConfig_StaticWithoutGateway_Returns400`
- `backend/internal/api/auth_handlers_test.go`
  - `TestLogin_EmptyPassword_Returns400`
- `backend/internal/api/system_handlers_test.go`
  - `TestRebootHandler_Returns200`
  - `TestRestartWifiHandler_Returns200`

### Implementation Steps
1. **Write `validation_test.go`.** Implement `validation.go`:
   ```go
   package api

   import (
       "fmt"
       "net"
   )

   func validateRequired(value, field string) error {
       if value == "" {
           return fmt.Errorf("%s: required", field)
       }
       return nil
   }

   func validateIPAddress(value string) error {
       if net.ParseIP(value) == nil {
           return fmt.Errorf("invalid IP address: %s", value)
       }
       return nil
   }

   func validateMTU(mtu int) error {
       if mtu < 68 || mtu > 9000 {
           return fmt.Errorf("mtu: must be between 68 and 9000, got %d", mtu)
       }
       return nil
   }

   func validateSSID(ssid string) error {
       if ssid == "" {
           return fmt.Errorf("ssid: required")
       }
       if len(ssid) > 32 {
           return fmt.Errorf("ssid: must be 32 characters or less")
       }
       return nil
   }

   func validateInSet(value string, allowed []string, field string) error {
       for _, a := range allowed {
           if value == a {
               return nil
           }
       }
       return fmt.Errorf("%s: must be one of %v", field, allowed)
   }
   ```
2. **Write handler tests that expect 400s.** Add validation to each handler after body parsing.
3. **Add `Reboot()` and `RestartWifi()`** to `SystemService`. Add handlers. Register routes.
4. **Update `shared/src/api/routes.ts`** with new system routes.
5. **Implement Quick Actions** in `quick-actions.tsx`:
   ```tsx
   import { useMutation } from '@tanstack/react-query';
   import { useVpnStatus, useToggleWireguard } from '@/hooks/use-vpn';
   import { apiClient } from '@/lib/api-client';
   import { API_ROUTES } from '@shared/index';

   export function QuickActions() {
     const { data: vpnStatuses } = useVpnStatus();
     const toggleWg = useToggleWireguard();

     const restartWifi = useMutation({
       mutationFn: () => apiClient.post(API_ROUTES.system.restartWifi),
     });

     const reboot = useMutation({
       mutationFn: () => apiClient.post(API_ROUTES.system.reboot),
     });

     const wgStatus = vpnStatuses?.find(s => s.type === 'wireguard');

     const handleToggleVpn = () => {
       toggleWg.mutate(!wgStatus?.enabled);
     };

     const handleReboot = () => {
       if (window.confirm('Are you sure you want to reboot?')) {
         reboot.mutate();
       }
     };

     // ... render with loading states
   }
   ```
6. **Run all tests.** `make test`.

### Acceptance Criteria
- POST `/wifi/connect` with empty SSID → 400 `{"error": "ssid: required"}`
- PUT `/network/wan` with `ip_address: "not-an-ip"` → 400 with specific error
- POST `/auth/login` with empty password → 400
- Quick Actions are functional with loading indicators
- Reboot shows confirmation before executing
- All tests pass

### Complexity
**M**

---

## Phase 6 — Frontend Polish: Deduplicate Utils, WireGuard Config Import, Uptime Fix

### Objective
Consolidate duplicated `formatBytes`/`formatUptime` functions, fix the hardcoded uptime in the dashboard, add a WireGuard `.conf` file parser for easy VPN setup, and add a simple bandwidth chart.

### Issues Addressed
- **#10** Hardcoded uptime in SystemStatsCard
- **#11** Duplicated formatBytes/formatUptime
- **#25** Bandwidth/usage graphs
- **#26** WireGuard .conf parser

### Files to Create
- `frontend/src/lib/wireguard-parser.ts` — `parseWireGuardConfig(content: string): WireguardConfig`
- `frontend/src/lib/__tests__/wireguard-parser.test.ts`
- `frontend/src/components/ui/bandwidth-chart.tsx` — SVG sparkline chart rendering rx/tx rates
- `frontend/src/hooks/use-bandwidth-history.ts` — hook accumulating bandwidth data from WebSocket

### Files to Modify
- **`frontend/src/pages/dashboard/system-stats-card.tsx`**
  - Delete local `formatUptime()` and `formatBytes()` functions (lines 10–29)
  - Add `import { formatUptime, formatBytes } from '@/lib/utils';`
  - Replace `formatUptime(86432)` with `formatUptime(stats.uptime_seconds)` (requires Phase 3's model change)
  - Use `useSystemStats()` hook instead of inline `useQuery`
- **`frontend/src/pages/vpn/vpn-page.tsx`**
  - Add "Import .conf" button below WireGuard config form
  - On click: open `<input type="file" accept=".conf">`, read file, call `parseWireGuardConfig(content)`, call `useSetWireguardConfig().mutate(parsed)`
- **`frontend/src/pages/dashboard/dashboard-page.tsx`**
  - Import and render `<BandwidthChart />` below `<QuickActions />`
- **`shared/src/api/system.ts`**
  - Add `uptime_seconds: number` to `SystemStats` interface

### Tests to Write First (TDD)
- `frontend/src/lib/__tests__/wireguard-parser.test.ts`
  - `test('parses minimal WireGuard config')`:
    ```
    [Interface]
    PrivateKey = abc123
    Address = 10.0.0.2/24

    [Peer]
    PublicKey = xyz789
    Endpoint = vpn.example.com:51820
    AllowedIPs = 0.0.0.0/0
    ```
    → `{ private_key: 'abc123', address: '10.0.0.2/24', dns: [], peers: [{ public_key: 'xyz789', endpoint: 'vpn.example.com:51820', allowed_ips: ['0.0.0.0/0'] }] }`
  - `test('parses config with DNS')` — `DNS = 1.1.1.1, 8.8.8.8` → `dns: ['1.1.1.1', '8.8.8.8']`
  - `test('parses config with PresharedKey')` → peer has `preshared_key`
  - `test('handles multiple peers')` — two `[Peer]` sections
  - `test('throws on missing Interface section')`
- `frontend/src/lib/__tests__/utils.test.ts`
  - `test('formatUptime 0 seconds')` → `"0 minutes"`
  - `test('formatUptime 90061')` → `"1 day, 1 hour, 1 minute"`
  - `test('formatBytes 0')` → `"0 B"`
  - `test('formatBytes 1073741824')` → `"1.0 GB"`

### Implementation Steps
1. **Write WireGuard parser tests.** Implement `wireguard-parser.ts`:
   ```typescript
   import type { WireguardConfig, WireguardPeer } from '@shared/index';

   export function parseWireGuardConfig(content: string): WireguardConfig {
     const lines = content.split('\n').map(l => l.trim());
     let currentSection: 'none' | 'interface' | 'peer' = 'none';
     const iface: Record<string, string> = {};
     const peers: Record<string, string>[] = [];
     let currentPeer: Record<string, string> = {};

     for (const line of lines) {
       if (line === '' || line.startsWith('#')) continue;
       if (line === '[Interface]') { currentSection = 'interface'; continue; }
       if (line === '[Peer]') {
         if (currentSection === 'peer') peers.push(currentPeer);
         currentSection = 'peer';
         currentPeer = {};
         continue;
       }
       const [key, ...rest] = line.split('=');
       const value = rest.join('=').trim();
       if (currentSection === 'interface') iface[key.trim()] = value;
       if (currentSection === 'peer') currentPeer[key.trim()] = value;
     }
     if (currentSection === 'peer') peers.push(currentPeer);

     if (!iface['PrivateKey']) throw new Error('Invalid config: missing PrivateKey');

     return {
       private_key: iface['PrivateKey'],
       address: iface['Address'] || '',
       dns: iface['DNS'] ? iface['DNS'].split(',').map(s => s.trim()) : [],
       peers: peers.map(p => ({
         public_key: p['PublicKey'] || '',
         endpoint: p['Endpoint'] || '',
         allowed_ips: p['AllowedIPs'] ? p['AllowedIPs'].split(',').map(s => s.trim()) : [],
         ...(p['PresharedKey'] ? { preshared_key: p['PresharedKey'] } : {}),
       } as WireguardPeer)),
     };
   }
   ```
2. **Remove duplicated functions** from `system-stats-card.tsx`. Import from `@/lib/utils`.
3. **Fix hardcoded uptime:** Replace `formatUptime(86432)` with `formatUptime(stats.uptime_seconds)`.
4. **Add `uptime_seconds`** to shared `SystemStats` type in `shared/src/api/system.ts`.
5. **Implement `use-bandwidth-history.ts`:** Connect to WebSocket, accumulate `{timestamp, rx, tx}` tuples in a 60-element ring buffer updated every 2s.
6. **Create `bandwidth-chart.tsx`:** Pure SVG sparkline. Accept `data: {rx: number, tx: number}[]`. Render two polylines (rx in blue, tx in green). Show latest rate in KB/s as text.
7. **Add .conf import UI** to VPN page: hidden file input, button trigger, parse + submit.
8. **Run `cd frontend && pnpm test`.**

### Acceptance Criteria
- Zero duplicated `formatBytes` or `formatUptime` functions — only `@/lib/utils` exports them
- Dashboard uptime shows real value from API, not `86432`
- WireGuard `.conf` files parsed correctly (5 test cases)
- Bandwidth chart renders in dashboard
- All frontend tests pass

### Complexity
**M**

---

## Phase 7 — UI Infrastructure: Error Boundaries, Toast System, Shadcn/UI, PWA

### Objective
Add React error boundaries to prevent white-screen crashes, implement a toast notification system for API feedback, set up shadcn/ui-compatible components, and add PWA manifest + favicon.

### Issues Addressed
- **#22** No error boundaries
- **#23** No toast/notification system
- **#24** Shadcn/UI replacement
- **#28** Favicon and PWA manifest

### Files to Create
- `frontend/src/components/error-boundary.tsx` — class component wrapping `componentDidCatch`; renders fallback card with error info + "Try Again" button
- `frontend/src/stores/toast-store.ts` — Zustand store: `{ toasts: Toast[], addToast(opts), removeToast(id) }`
- `frontend/src/lib/toast.ts` — convenience `toast.success(msg)`, `toast.error(msg)`, `toast.info(msg)` calling store
- `frontend/src/components/ui/toaster.tsx` — renders toast list from store, fixed bottom-right
- `frontend/src/components/ui/toast.tsx` — single toast component: icon, title, description, close button, auto-dismiss
- `frontend/src/components/ui/dialog.tsx` — modal dialog for confirmations (shadcn/ui pattern using portal)
- `frontend/src/components/ui/label.tsx` — form label component
- `frontend/src/components/ui/select.tsx` — styled select dropdown
- `frontend/public/favicon.svg` — simple SVG router/globe icon
- `frontend/public/manifest.json` — PWA manifest

### Files to Modify
- **`frontend/index.html`**
  - Add `<link rel="icon" href="/favicon.svg" />`
  - Add `<link rel="manifest" href="/manifest.json" />`
  - Add `<meta name="theme-color" content="#2563eb" />`
- **`frontend/src/App.tsx`**
  - Wrap `RouterProvider` in `<ErrorBoundary>`
  - Add `<Toaster />` at root level
- **`frontend/src/hooks/use-wifi.ts`**
  - Add `onError: (err) => toast.error(err.message)` and `onSuccess: () => toast.success('...')` to mutation options
- **`frontend/src/hooks/use-vpn.ts`**
  - Same toast pattern for toggle/config mutations
- **`frontend/src/hooks/use-network.ts`**
  - Same toast pattern for WAN config mutation
- **`frontend/src/hooks/use-services.ts`**
  - Same toast pattern for install/remove/start/stop mutations
- **`frontend/src/pages/dashboard/quick-actions.tsx`**
  - Use `toast.success`/`toast.error` instead of alerts
  - Replace `window.confirm` with `Dialog` component

### Tests to Write First (TDD)
- `frontend/src/components/__tests__/error-boundary.test.tsx`
  - `test('renders children when no error')` — mount child, visible
  - `test('renders fallback on child error')` — child throws, fallback visible
  - `test('retry remounts children')` — click retry, child re-renders
- `frontend/src/stores/__tests__/toast-store.test.ts`
  - `test('addToast adds to list')`
  - `test('removeToast removes by id')`
  - `test('toast auto-dismisses after timeout')` (use fake timers)
- `frontend/src/lib/__tests__/toast.test.ts`
  - `test('toast.success adds success variant')`
  - `test('toast.error adds error variant')`

### Implementation Steps
1. **Error boundary tests → implement.** Class component with `state: { hasError, error }`. `static getDerivedStateFromError`. `componentDidCatch` logs error. Renders fallback Card with `error.message` and "Try Again" button that sets `hasError: false`.
2. **Toast store tests → implement.** Zustand store:
   ```typescript
   interface Toast { id: string; title: string; description?: string; variant: 'success' | 'error' | 'info'; }
   interface ToastState {
     toasts: Toast[];
     addToast: (t: Omit<Toast, 'id'>) => void;
     removeToast: (id: string) => void;
   }
   ```
   `addToast` generates `crypto.randomUUID()` id, pushes toast, sets `setTimeout` for `removeToast` after 5000ms.
3. **Implement `toast.ts`:**
   ```typescript
   import { useToastStore } from '@/stores/toast-store';
   export const toast = {
     success: (title: string, description?: string) =>
       useToastStore.getState().addToast({ title, description, variant: 'success' }),
     error: (title: string, description?: string) =>
       useToastStore.getState().addToast({ title, description, variant: 'error' }),
     info: (title: string, description?: string) =>
       useToastStore.getState().addToast({ title, description, variant: 'info' }),
   };
   ```
4. **Create `toast.tsx`** (individual) and **`toaster.tsx`** (container). Style with Tailwind: `fixed bottom-4 right-4 z-50 flex flex-col gap-2`. Each toast: 300px card with colored left border, icon, close X button.
5. **Wrap app** in `<ErrorBoundary>` and add `<Toaster />` in `App.tsx`.
6. **Add toasts** to all mutation hooks' `onSuccess`/`onError` callbacks.
7. **Create `dialog.tsx`:** Portal-based modal with backdrop, content card, cancel/confirm buttons. Accept `open`, `onOpenChange`, `title`, `description`, `confirmLabel`, `onConfirm` props.
8. **Replace `window.confirm`** in quick-actions with `<Dialog>`.
9. **Create `favicon.svg`** (minimal router icon). Create **`manifest.json`**:
   ```json
   {
     "name": "OpenWRT Travel GUI",
     "short_name": "OpenWRT",
     "start_url": "/",
     "display": "standalone",
     "theme_color": "#2563eb",
     "background_color": "#f9fafb",
     "icons": [{ "src": "/favicon.svg", "sizes": "any", "type": "image/svg+xml" }]
   }
   ```
10. **Update `index.html`** with favicon, manifest, theme-color meta.
11. **Create `label.tsx` and `select.tsx`** as shadcn/ui-compatible components.
12. **Run all frontend tests.**

### Acceptance Criteria
- Component errors show "Something went wrong" UI with retry, not white screen
- All API mutations trigger toast notifications (success green, error red)
- Toasts auto-dismiss after 5 seconds; can be manually closed
- PWA manifest served correctly; favicon visible in browser tab
- `window.confirm` replaced with Dialog component
- All tests pass

### Complexity
**L**

---

## Phase 8 — Mobile UX, Animations, Login Polish, Final Cleanup

### Objective
Make the sidebar fully mobile-responsive with a hamburger-triggered slide-in overlay, add subtle animations and transitions throughout the app, polish the login page design, and verify no issues remain.

### Issues Addressed
- **#12** Mobile-responsive sidebar
- **#30** Login page polish
- **#31** No animations/transitions

### Files to Create
- `frontend/src/components/layout/mobile-sidebar.tsx` — full-screen overlay sidebar for mobile
- `frontend/src/hooks/use-media-query.ts` — `useMediaQuery(query): boolean` hook via `window.matchMedia`

### Files to Modify
- **`frontend/src/components/layout/app-shell.tsx`**
  - Use `useMediaQuery('(max-width: 768px)')` to detect mobile
  - On mobile: hide `<Sidebar>`, show `<MobileSidebar>` overlay controlled by `mobileOpen` state
  - Pass `onMenuClick` to `<Header>`
- **`frontend/src/components/layout/header.tsx`**
  - Accept optional `onMenuClick?: () => void` prop
  - Render `<Menu>` hamburger icon button when `onMenuClick` is provided
- **`frontend/src/components/layout/sidebar.tsx`**
  - Add `transition-all duration-200` for smooth collapse
- **`frontend/src/pages/login/login-page.tsx`**
  - Add gradient background: `bg-gradient-to-br from-blue-50 to-indigo-100 dark:from-gray-900 dark:to-gray-800`
  - Add `animate-fade-in` class to the Card
  - Add red border to password input on error: `clsx(error && 'border-red-500 ring-1 ring-red-500')`
  - Add version footer: small text with `API_VERSION`
- **`frontend/src/index.css`**
  - Add CSS keyframe animations:
    ```css
    @keyframes fade-in {
      from { opacity: 0; transform: translateY(8px); }
      to { opacity: 1; transform: translateY(0); }
    }
    @keyframes slide-in-left {
      from { transform: translateX(-100%); }
      to { transform: translateX(0); }
    }
    @keyframes slide-out-left {
      from { transform: translateX(0); }
      to { transform: translateX(-100%); }
    }
    .animate-fade-in { animation: fade-in 0.3s ease-out both; }
    .animate-slide-in-left { animation: slide-in-left 0.2s ease-out both; }
    ```
- **`frontend/src/components/ui/button.tsx`**
  - Add `transition-all duration-150 active:scale-[0.97]` to base class
- **`frontend/src/components/ui/card.tsx`**
  - Add `animate-fade-in` to card container
- **`frontend/src/pages/dashboard/dashboard-page.tsx`**
  - Add staggered animation delays to grid cards: `style={{ animationDelay: \`${i * 75}ms\` }}`

### Tests to Write First (TDD)
- `frontend/src/hooks/__tests__/use-media-query.test.ts`
  - `test('returns true when query matches')` — mock `matchMedia` returning `{ matches: true }`
  - `test('returns false when query does not match')`
  - `test('updates on change event')` — trigger listener, assert value updates
- `frontend/src/components/layout/__tests__/mobile-sidebar.test.tsx`
  - `test('renders all nav items')`
  - `test('calls onClose when backdrop clicked')`
  - `test('calls onClose when nav link clicked')`
- `frontend/src/pages/login/__tests__/login-page.test.tsx`
  - `test('shows error styling on failed login')` — submit wrong password, assert input has `border-red-500` class
  - `test('renders version text')`

### Implementation Steps
1. **Write `use-media-query` tests.** Implement:
   ```typescript
   import { useState, useEffect } from 'react';

   export function useMediaQuery(query: string): boolean {
     const [matches, setMatches] = useState(() =>
       typeof window !== 'undefined' ? window.matchMedia(query).matches : false
     );

     useEffect(() => {
       const mql = window.matchMedia(query);
       const handler = (e: MediaQueryListEvent) => setMatches(e.matches);
       mql.addEventListener('change', handler);
       setMatches(mql.matches);
       return () => mql.removeEventListener('change', handler);
     }, [query]);

     return matches;
   }
   ```
2. **Create `mobile-sidebar.tsx`:**
   ```tsx
   interface MobileSidebarProps {
     open: boolean;
     onClose: () => void;
   }

   export function MobileSidebar({ open, onClose }: MobileSidebarProps) {
     if (!open) return null;
     return (
       <div className="fixed inset-0 z-50 flex">
         {/* Backdrop */}
         <div className="fixed inset-0 bg-black/50" onClick={onClose} />
         {/* Sidebar panel */}
         <div className="relative w-64 bg-white dark:bg-gray-950 animate-slide-in-left">
           {/* Same nav items as Sidebar but always expanded */}
           <nav className="flex flex-col p-4 space-y-1">
             {navItems.map(({ to, label, icon: Icon }) => (
               <Link key={to} to={to} onClick={onClose} className="...">
                 <Icon className="h-5 w-5" />
                 <span>{label}</span>
               </Link>
             ))}
           </nav>
         </div>
       </div>
     );
   }
   ```
3. **Modify `app-shell.tsx`:**
   ```tsx
   const isMobile = useMediaQuery('(max-width: 768px)');
   const [mobileOpen, setMobileOpen] = useState(false);
   // ...
   {!isMobile && <Sidebar collapsed={collapsed} onToggle={() => setCollapsed(c => !c)} />}
   {isMobile && <MobileSidebar open={mobileOpen} onClose={() => setMobileOpen(false)} />}
   <Header title={title} onMenuClick={isMobile ? () => setMobileOpen(true) : undefined} />
   ```
4. **Modify `header.tsx`:** Accept `onMenuClick`; render hamburger:
   ```tsx
   {onMenuClick && (
     <Button variant="ghost" size="sm" onClick={onMenuClick} aria-label="Open menu">
       <Menu className="h-5 w-5" />
     </Button>
   )}
   ```
5. **Add CSS animations** to `index.css`.
6. **Add `animate-fade-in`** to card component. Add `transition-all active:scale-[0.97]` to button.
7. **Polish login page:** Gradient bg, animated card, error input styling, version footer.
8. **Add staggered animations** to dashboard grid (wrap cards with style prop for `animationDelay`).
9. **Run all tests.** `make test` from project root.

### Acceptance Criteria
- On viewport ≤768px: sidebar hidden, hamburger appears, tap opens slide-in overlay
- Clicking nav item in mobile sidebar navigates and closes overlay
- Cards fade in on page load with slight stagger
- Buttons have subtle press feedback
- Login page has gradient background, animated card entry, red input border on error
- Toasts slide in from right (Phase 7); mobile sidebar slides from left
- `make test` passes with 0 failures across all packages (Go + shared + frontend)

### Complexity
**M**

---
title: "Plan: Tailscale Integration"
description: "Planning / design notes: Plan: Tailscale Integration"
updated: 2026-04-13
tags: [plan, tailscale, traceability]
---

# Plan: Tailscale Integration

**Status:** Not implemented
**Priority:** Low (plan for future)
**Related requirements:** [3.2 Tailscale](../requirements/tasks_done.md#vpn-and-services)

---

## Goal

Full Tailscale integration: authenticate the router as a Tailscale node, show connected peers, configure as exit node, and manage Tailscale features from the travel router GUI.

---

## Background

- Tailscale on OpenWRT installs as a single binary (`tailscale` + `tailscaled`)
- Authentication requires a browser-based login flow (Tailscale control plane)
- Once authenticated, the device gets a Tailscale IP and can reach other nodes on the tailnet
- Exit node mode routes all traffic through a specific Tailscale peer

---

## Phases

### Phase 1 — Authentication Flow

**Challenge:** Tailscale auth requires opening a URL in a browser to log in.

**Implementation:**
1. Backend runs `tailscale up --auth-key=<key>` if user provides a pre-auth key
2. OR: Backend runs `tailscale up` which outputs an auth URL
3. Backend captures the auth URL and sends it to the frontend
4. Frontend shows the URL as a clickable link / QR code: "Open this link to authenticate your router with Tailscale"
5. Backend polls `tailscale status --json` until authenticated

**API:**
- `POST /api/v1/vpn/tailscale/auth` — start auth flow, returns `{ "auth_url": "https://login.tailscale.com/..." }`
- `GET /api/v1/vpn/tailscale/auth/status` — poll auth state: `{ "authenticated": true/false }`

### Phase 2 — Status & Connected Peers

**API:** `GET /api/v1/vpn/tailscale/status`

Parse output of `tailscale status --json`:
```json
{
  "authenticated": true,
  "tailscale_ip": "100.64.0.1",
  "hostname": "openwrt-travel",
  "peers": [
    {
      "hostname": "my-laptop",
      "tailscale_ip": "100.64.0.2",
      "os": "macOS",
      "online": true,
      "last_seen": "2026-03-15T10:00:00Z"
    }
  ],
  "exit_node": null
}
```

**Frontend:** Peer list with online/offline indicators, Tailscale IP display.

### Phase 3 — Exit Node Selection

**When:** User wants to route all traffic through a Tailscale peer (e.g., home server)

**API:**
- `GET /api/v1/vpn/tailscale/exit-nodes` — list peers that advertise as exit nodes
- `POST /api/v1/vpn/tailscale/exit-node` — set exit node: `{ "node_ip": "100.64.0.5" }` or `{ "node_ip": "" }` to clear

**Backend:**
```sh
tailscale set --exit-node=100.64.0.5
tailscale set --exit-node=  # clear
```

### Phase 4 — Tailscale Subnet Routing

**When:** User wants LAN clients to access Tailscale network

**Implementation:**
```sh
tailscale up --advertise-routes=192.168.1.0/24
```

Plus enable IP forwarding and configure firewall rules.

### Phase 5 — Frontend UI

- Tailscale tab within VPN page (alongside WireGuard)
- Auth section: show auth button or "Connected as <hostname>"
- Peer list with status
- Exit node dropdown
- Enable/disable toggle
- Subnet routing toggle with LAN CIDR display

---

## Testing Strategy

- **Unit tests:** Mock `tailscale status --json` output parsing
- **Auth flow:** Test the polling mechanism with mock responses
- **Real device:** Requires Tailscale account for end-to-end testing

---

## Notes

- Tailscale binary is ~20MB — significant for flash-constrained devices; warn about storage impact
- Auth tokens expire; may need re-authentication periodically
- Tailscale + WireGuard can coexist if on different interfaces, but avoid routing conflicts
- Consider showing Tailscale as a separate VPN mode (can't use both WireGuard and Tailscale as exit at the same time without confusion)

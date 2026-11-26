# 0002 One Go daemon behind ports

Status: accepted

One static Go binary serves REST and the SPA, supervised by procd. All OpenWrt access goes through a `Device` port (real CLI adapter over `uci`/`ubus`/netifd with a timeout tier per call; mock adapter) and a `PackageManager` port (apk, opkg). Rejected: rpcd-ucode without a daemon (no home for background state machines), a LuCI plugin (couples to LuCI).

Go over Rust: matches the agent-presets default for device binaries, easy cross-compile to aarch64, stdlib HTTP suffices; the ~12.8 MB budget holds. CLI adapter first; a native ubus socket adapter is possible later behind the same port if profiling on the AXT1800 demands it.

Consequence: a contract test fails if any feature writes UCI outside the transaction module.

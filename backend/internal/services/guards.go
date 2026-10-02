package services

// Crash guards — the on-disk markers that make an interrupted live-state change
// discoverable instead of silently leaving a half-applied device (AGENTS.md,
// ADR 0003).
//
// Every guard path in this package is built from crashGuardDir. That is a
// single source of truth on purpose: guards were historically spread across
// /etc/trafo and /etc/travo, and a guard written to one directory was invisible
// to a check that only looked at the other, so a stale guard could disable a
// feature with no log line at all.
//
// State files (aliases, WiFi priorities, repeater options, the bbolt store)
// deliberately stay in /etc/travo — they are configuration, not abort markers,
// and moving them would lose user data on upgrade. The distinction the ADR
// draws is "does startup consult this as an abort-if-present signal".
const crashGuardDir = "/etc/trafo"

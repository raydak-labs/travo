---
title: "ADR 0012: UI consistency — card plane, status language, summary bands, traffic history"
description: One card plane, one status language, one group heading, one load/error contract, summary bands, and server-owned traffic history.
status: Accepted
date: 2026-10-05
updated: 2026-10-05
tags: [adr, frontend, ui, theming, consistency, status, wifi, dashboard]
---

# ADR 0012: UI consistency — card plane, status language, summary bands, traffic history

A frontend presentation decision. It changes no device state and no safety
invariant; it constrains how the UI is composed so pages stop drifting apart.

## Status

Accepted.

## Context

The dashboard read well because it leaned on the shared primitives
(`Card`, `PageSection`, `QueryCard`). The rest of the app did not. Six pages
hand-rolled the same group-label `<h2>`; roughly one hundred status colours
appeared as raw `bg-green-100 text-green-800`-style strings with no dark-mode
discipline; two screens (dashboard source card, setup wizard) had their own
surface treatment; and most cards invented their own load/error handling, so a
failed request could render a confident "nothing here".

Two functional gaps came from the same root cause — each page invented its own
answer to "what is the state of this thing": the WiFi page had no overview (and
no internet reachability, which lives in the network domain), and the dashboard
throughput chart started empty on every page load because its buffer lived in
component state.

## Decision

1. **One card plane.** Every surface is `cardSurfaceVariants()`
   (`rounded-lg border border-gray-200 bg-white shadow-sm dark:border-white/10
   dark:bg-gray-950`). The dashboard topology source card and the setup wizard
   gradient were removed. There is no exempt screen.

2. **One status language.** Status colour comes from the semantic tokens in
   `frontend/src/index.css` (`--status-{ok,warn,danger,info,neutral,stale}-{text,surface,border}`)
   and reaches the screen through `StatusPill`. Raw `bg-*/text-*` status
   strings are a defect, not a style choice. `stale` is deliberately weaker than
   `warn`: it means "we had a value and it may no longer be true".

3. **One group heading.** `SectionHeading` is the only in-page group label.
   `CardTitle` stays the card title. No page writes its own uppercase `<h2>`.

4. **One load/error contract.** Any component rendering query data goes through
   `QueryCard` (skeleton / inline error + retry / content). A failed request must
   never produce an "absent" claim.

5. **Summary band.** A page whose domain has more than one thing to check starts
   with an always-visible `SummaryBand` of flat `SummaryTile`s: the facts that
   answer "is this working" without scrolling. It is not collapsible, and each
   tile degrades on its own — a failed tile shows its own error and retry, a
   tile holding pre-disconnect values keeps them dimmed and marked stale, a tile
   that never loaded shows a real empty state. Today only WiFi has one; the
   pattern is the template for the other pages.

6. **Traffic history is server-owned.** The chart's live tail is WebSocket push,
   but the ~10 minutes before it are read from `GET
   /api/v1/network/traffic-history`. The backend samples every broadcast tick
   into a bounded in-memory ring regardless of connected clients, so a freshly
   loaded dashboard has something to draw. History resets on restart; the UI
   shows the short window it has rather than pretending to ten minutes.

## Why

Hard to reverse (every page inherits these), surprising without context (the
tokens look like a style choice until you know the drift they end), and the
result of a real trade-off (ad-hoc colours were faster per page; they were also
the reason the same status reads differently on three pages).

The status tokens exist because `Badge` variants cannot express the tinted
panel + matching border + light/dark pair that status surfaces need. The
dashboard source card was normalized rather than exempted because "every screen
except two looks different" is exactly the rule that produced the drift.

Traffic history is server-owned because the alternative (client-side caching)
only works after a user has already visited, and only for that browser — which
is precisely the case that looked broken.

## Consequences and invariants

- **A new status colour is a token addition, not a className.** Adding a raw
  status pair in a page reintroduces the defect.
- **New pages start with a summary band** if they have more than one status
  question; the WiFi band (`wifi-summary-band.tsx`) is the reference.
- **Radio facts are labelled "configured".** `RadioInfo` is UCI-derived, not
  read from the air. A tile must not imply live channel, width or bitrate. A
  future `/api/v1/wifi/radios/status` may fill that gap without changing the
  tile contract.
- **Traffic history memory is bounded by count, not by time.** A stalled tick
  loop must not grow the ring.
- **`stale` never replaces a value with a blank.** Throwing away a fact that
  was true seconds ago is worse than showing it as possibly-outdated.

## Deliberate ceilings

- History is ~10 minutes. Longer ranges need server-side downsampling and are
  out of scope until someone asks for them.
- Band tiles mix domains deliberately: WiFi reads `NetworkStatus` for
  reachability because "is my internet up" is the question a user asks on the
  WiFi page. The alternative (leaving it to Network) answers the question in the
  wrong place.

## Provenance: work folded in from executed plans

This ADR supersedes the contracts in the UI consistency and nested-cards plans of
2026-07-20 and the traveler IA implementation checklist of 2026-07-24, which were
deleted rather than archived. What survives from them:

- **Card chrome.** The shared inset pattern (`CardInset`,
  `rounded-md border border-gray-200 p-3 dark:border-white/10`, no second
  competing shadow) and equal-height sibling cards, from the nested-cards plan.
  Decision 1 above subsumes it: `CardInset` nests, it does not introduce a plane.
- **Feedback and label defaults.** Loading is `Skeleton`, empty is `EmptyState`,
  load errors are `InlineError`, and `Label` defaults to
  `text-xs font-medium text-gray-500 dark:text-gray-400`. Those primitives were
  built by that work and are unchanged.
- **Page rhythm.** `space-y-6` at the page root and `CardHeader`/`CardTitle` as
  the card title contract.
- **What was deliberately reversed, and why.** The old plan allowed an ad-hoc status
  class and allowed the dashboard source card to keep custom chrome. Both are now
  defects: raw `bg-*/text-*` status strings are banned (decision 2) and there is
  no exempt screen (decision 1). Each was allowed for a reason that no longer
  holds — the ad-hoc class because per-page status colours were faster to write,
  the source-card exemption because that card predated the shared card plane — so
  if you want one of them back, you owe a new ADR, not a local exception.

Health fixtures on the repeater tests must disable conflicting mock radios, or
`repeater_same_radio_ap_sta` (ADR 0002) fires in unrelated tests.

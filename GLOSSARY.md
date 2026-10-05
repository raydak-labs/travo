---
title: Glossary
description: Shared vocabulary for the Travo domain. No implementation details here.
updated: 2026-10-05
tags: [glossary, domain]
---

# Glossary

Terms used across this repository that mean something more specific than the
word usually implies. Normative decisions live in
[`docs/architecture/overview.md`](docs/architecture/overview.md) and the
[ADRs](docs/adr/README.md); this file only fixes the words.

## Card plane

The single surface treatment every panel in the UI shares. One card plane means
a user does not learn a second visual grammar per page. See
[ADR 0012](docs/adr/0012-ui-consistency-and-status-language.md).

## Summary band

The always-visible row of facts at the top of a page that answers "is this
working" without scrolling. Flat tiles, never collapsible: its purpose is to be
seen before the user reads anything else.

## Summary tile

One cell of a summary band. A tile is not a page section — it cannot be
collapsed, and it does not own actions.

## Status tone

A named state meaning — ok, warn, danger, info, inactive — that carries its own
colour in light and dark mode. Tones are the vocabulary of status; colour alone
is never the message.

## Stale

A value last seen before the connection carrying it dropped. Stale is weaker
than a warning: the fact was true at some point and may not be true now. A stale
value is still shown, marked as such.

## Traffic history

The bounded window of recent per-interface traffic samples the router holds in
memory. It exists so a freshly loaded page has something to show instead of an
empty axis. It is not persistent: a router restart ends the window.

## Configured radio vs live radio

A **configured radio** is what the router is set to do, read from its saved
configuration. A **live radio** is what the radio is doing right now on the air.
They disagree during lockout, disabled, and mid-apply states, so the UI must say
which one it is showing rather than blending them into "the radio".

## Internet reachable

That the WAN reports carrier connectivity. It is not a reachability probe: an
uplink that has a lease but is dead or captive-portaled still reads as reachable.
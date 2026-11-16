# Dashboard WiFi action acceptance (#62)

Tested the production SPA against a local `--mock` daemon on 2026-11-15.
No router or operator credentials were used.

The mobile flow used three taps: Connect to WiFi → Scan → Join Neighbor2.
Cancel closed the dialog, and browser Back returned to Dashboard.

Additional checks used headless Chrome through `web/scripts/cdp.mjs` at
390 × 844 and 1280 × 800 CSS pixels, in both light and dark schemes:

- Tab reached Connect to WiFi in the normal focus order.
- The focused link matched `:focus-visible` and had a visible solid 2 px
  outline, with its full bounds inside the viewport.
- Enter opened the WiFi page. Browser Back returned to Dashboard.
- No horizontal document overflow occurred.
- Visual inspection confirmed the link was readable in both themes, and the
  keyboard focus outline was visible at both widths.

The component regression test covers the link while connectivity is
unavailable. `mise run check` passed after adding this evidence. Images from
the run are not stored in the repo.

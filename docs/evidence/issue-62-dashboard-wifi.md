# Dashboard WiFi action acceptance (#62)

Tested the production SPA against a local `--mock` daemon on 2026-11-15.
No router or operator credentials were used.

The mobile flow used three taps: Connect to WiFi → Scan → Join Neighbor2.
Cancel closed the dialog, and browser Back returned to Dashboard.
The [join dialog](issue-62-dashboard-wifi-join-390.png) records that flow.

Additional checks used headless Chrome through `web/scripts/cdp.mjs` at
390 × 844 and 1280 × 800 CSS pixels, in both light and dark schemes:

- Tab reached Connect to WiFi in the normal focus order.
- The focused link matched `:focus-visible` and had a visible solid 2 px
  outline, with its full bounds inside the viewport.
- Enter opened the WiFi page. Browser Back returned to Dashboard.
- No horizontal document overflow occurred.
- Visual inspection confirmed the link was readable in both themes.

The screenshots include the keyboard focus outline:

- [390 px, light](issue-62-dashboard-390-light.png)
- [390 px, dark](issue-62-dashboard-390-dark.png)
- [1280 px, light](issue-62-dashboard-1280-light.png)
- [1280 px, dark](issue-62-dashboard-1280-dark.png)

The component regression test covers the link while connectivity is
unavailable. `mise run check` passed after adding this evidence.

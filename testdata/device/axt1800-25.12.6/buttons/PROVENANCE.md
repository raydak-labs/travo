# buttons/ (2026-11-19, OpenWrt 25.12.6, GL-AXT1800, #195)

Captured over SSH, read-only. Verbatim; nothing needed redaction (devicetree key nodes carry
no secrets).

- devicetree-keys.tsv: one line per key node, run in `/sys/firmware/devicetree/base/keys`:
  `printf "%s\t%s\t%s\t%s\n" "$n" "$(tr -d "\000" < $n/label)" "$(hexdump -ve "1/1 \"%02x\"" $n/linux,code)" "$(hexdump -ve "1/1 \"%02x\"" $n/linux,input-type 2>/dev/null)"`.
  Cells are big-endian u32. `reset`: code `0x198` (KEY_RESTART, hotplug name `reset`), no
  input type (EV_KEY). `switch`: code `0x100` (BTN_0, hotplug name `BTN_0`), input type 5
  (EV_SW, a slide switch). The `label` files end in a NUL byte (`hexdump -C reset/label`:
  `72 65 73 65 74 00`).
- Not captured as files, read at the same time: `/etc/rc.button` holds the stock
  `failsafe power reboot reset rfkill wps` (no `BTN_0` script); `/etc/hotplug.d/button` does
  not exist on the stock image. `/etc/hotplug.json` runs `/etc/rc.button/%BUTTON%` for a
  button event and then, in a separate rule, `/sbin/hotplug-call %SUBSYSTEM%` when
  `/etc/hotplug.d/%SUBSYSTEM%` is a directory; `hotplug-call` sets
  `PATH=/usr/sbin:/usr/bin:/sbin:/bin` and sources each script in a subshell.

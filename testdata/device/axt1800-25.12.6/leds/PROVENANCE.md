# leds/ (2026-11-19, OpenWrt 25.12.6 r33311-981c978eff, GL-AXT1800, #194)

Captured over SSH. Verbatim; nothing needed redaction (LED names, triggers and the stock
`system` config carry no secrets).

- sysfs-default.tsv: before any write, one line per `/sys/class/leds/*` entry:
  `printf "%s\t%s\t%s\t%s\n" "$l" "$(cat $l/brightness)" "$(cat $l/max_brightness)" "$(cat $l/trigger)"`.
  The active trigger is the bracketed word. `blue:run` is the running LED (`none`, brightness 1,
  set by `/etc/diag.sh` `set_state done`); `mmc0::` follows the eMMC (`[mmc0]`).
- uci-show-system-default.txt: `uci show system` at the same time: no `led` section on the stock image.
- Write probe under the shared router lock, restored exactly (`uci export system` and the sysfs
  listing compared byte for byte afterwards: identical):
  1. `uci set system.travo_led_<name>=led` plus `name`, `sysfs`, `trigger=none`, `default=0`
     for `blue:run` and `white:system`, `uci commit system`, `/etc/init.d/led restart` (exit 0,
     prints `setting up led <name>`; no radio or network effect).
  2. sysfs-off.tsv: the same listing after step 1 (`blue:run` brightness 0).
  3. uci-show-system-off-grep-led.txt: `uci show system | grep led` after step 1 (the
     `ntp.enabled` line matches the grep).
  4. led-state-off.txt: `/var/run/led.state` after step 1: `load_led` records each LED's
     trigger and brightness before it applies a section.
  5. `uci delete` of both sections, `uci commit system`, `/etc/init.d/led restart`: `start()`
     replays `led.state` (blue:run back to brightness 1) and removes the file; the sysfs listing
     equalled sysfs-default.tsv again.
- `/etc/init.d/led` is not a procd service: `reload_config` does not restart it, so the daemon
  runs `/etc/init.d/led restart` after its commit.

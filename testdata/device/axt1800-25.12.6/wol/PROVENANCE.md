# wol/ (2026-11-19, OpenWrt 25.12.6, #247)

Captured on the GL-AXT1800 (Travo 0.1.0 after firstboot). Read-only; nothing needed redaction.

- tools.txt: `which etherwake ether-wake wakeonlan; echo "exit=$?"; apk list -I | grep -i wake; echo "exit=$?"`. Neither command printed anything: no Wake-on-LAN tool or package is on the image, so the daemon builds and sends the magic packet itself (internal/wol). The `exit=N` lines are added by the capture.

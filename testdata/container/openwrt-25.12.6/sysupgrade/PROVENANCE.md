Container capture (not hardware): `openwrt/rootfs:aarch64_generic-25.12.6`, 2026-11-19.
- validate-zero-image.json: `dd if=/dev/zero of=/tmp/zero.bin bs=1k count=4; /usr/libexec/validate_firmware_image /tmp/zero.bin` (stderr dropped). This is the script rpcd runs for `ubus call system validate_firmware_image`; its stdout is the ubus reply. A zero file is not an image: `valid:false`, still `forceable`. Verbatim.
The script on the AXT1800 (25.12.6, read with `cat`) is the same and adds the platform's own tests to `tests`; no image was validated on the shared router.

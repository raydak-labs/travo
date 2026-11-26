# dhcp/ (2026-11-19, OpenWrt 25.12.6, #239)

Captured on the GL-AXT1800 (Travo 0.1.0 after firstboot, AdGuard default DNS, guest network on). Read-only (GET-only probes); nothing needed redaction. Every byte is verbatim.

- uci-export-dhcp.txt: `uci export dhcp`. dhcp.lan has start/limit/leasetime set (100/150/12h); the dnsmasq section carries the DNS layer stack's options (`server`, `noresolv`, `rebind_protection`); there is no `domain` section yet.
- uci-show-network-lan-guest.txt: `uci show network.lan; uci show network.guest` (CIDR in `ipaddr`, no `netmask`).
- init-dnsmasq-dhcp-range.txt: `sed -n 570,575p /etc/init.d/dnsmasq; sed -n 600,616p /etc/init.d/dnsmasq` (`dhcp_add`): unset start/limit/leasetime default to 100/150/12h; `ipcalc` turns start/limit into the `--dhcp-range`; `dhcpv4 disabled` skips the range.
- init-dnsmasq-domain-add.txt: `sed -n 723,737p /etc/init.d/dnsmasq` (`dhcp_domain_add`): a `domain` section's `name` (space-separated names) and `ip` become one `<ip> <names>` line of dnsmasq's per-instance hosts file (`/tmp/hosts/dhcp.<cfg>`, loaded with `--addn-hosts`); sections without name or ip are skipped.

Not captured yet: a `domain` section on the device. The unit tests build user sections from the two fields the init script reads; the hardware playbook (docs/testing.md, "Local DNS names") captures `uci show dhcp.travo_dns_1` and the hosts-file line after the first live add.

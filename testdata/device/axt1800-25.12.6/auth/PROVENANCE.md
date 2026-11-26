Captured read-only from GL-AXT1800 (OpenWrt 25.12.6) on Mon Nov  9 16:41:56 UTC 2026 for slice 3 (auth). Old Travo 0.10.2 installed and running.
- uci-show-rpcd.txt: uci show rpcd
- ubus-session-list.txt: ubus -v list session
- session-login-wrong-password.txt: ubus call session login '{"username":"root","password":"greenfield-wrong-password"}'; echo "exit=$?"  (stderr included)
- session-login-unknown-user.txt: ubus call session login '{"username":"nosuchuser","password":"x"}'; echo "exit=$?"  (stderr included)
- uci-uhttpd-listen.txt: uci show uhttpd | grep -E "listen|redirect"
- firewall-zones.txt: uci show firewall | grep -E "=zone|\.name=|\.input=|\.network=|\.forward="
- listeners.txt: netstat -lntp | grep -E ":80 |:8080|:443|:8443|:3000"
Not captured on purpose: a successful login (needs the real root password; the reply shape is documented upstream and is covered in the booted container where the password is known). The rpcd login password value is the stock $p$root reference, not a secret, but check before committing.

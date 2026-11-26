# diagnostics/ (2026-11-19, OpenWrt 25.12.6, #236)

Captured on the GL-AXT1800 (Travo 0.1.0, uplink wwan on radio0 to the private upstream 10.0.1.0/24). Caller on Ethernet. Read-only: the probes send ICMP/UDP and one DNS query and change nothing on the router. Targets are private addresses, `localhost` and a reserved `.invalid` name, so nothing needed redaction.

Each file is `<command> > file 2>&1; echo "exit=$?" >> file`: stdout and stderr as printed, then one `exit=N` line added by the capture (not tool output).

- ping-ok.txt: `ping -c 4 -W 2 10.0.1.1` (the upstream gateway). exit 0.
- ping-no-reply.txt: `ping -c 2 -W 2 10.0.1.254` (no host). Statistics on stdout, exit 1.
- ping-bad-host.txt: `ping -c 4 -W 2 does-not-exist.invalid`. Only stderr (`ping: bad address ...`), exit 1.
- traceroute-ok.txt: `traceroute -n -w 2 -q 1 -m 20 10.0.1.1`. One hop, exit 0.
- lookup-ok.txt: `nslookup localhost 127.0.0.1`. A and AAAA answers, exit 0.
- lookup-nxdomain.txt: `nslookup does-not-exist.invalid 127.0.0.1`. NXDOMAIN per query type on stdout, exit 1.
- tools.txt: `ls -l /bin/ping /bin/traceroute /usr/bin/nslookup`: all BusyBox applets.

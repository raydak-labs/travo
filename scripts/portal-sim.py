#!/usr/bin/env python3
"""Hotel captive portal simulator for the hardware playbook (docs/testing.md).

Runs on the laptop that plays the hotel, next to dnsmasq answering every name with
this host. Stdlib only. Per client IP it is unauthenticated until the client accepts,
then every path answers 204 like generate_204. The portal never forwards traffic.

Modes (--mode):
  redirect   302 to /login (most hotels); /login is a terms form (checkbox + hidden token)
  intercept  200 with the terms form on any path (portals that answer in place)
  room       like redirect, but the form also asks for a room number (auto-accept must stop)
  meta       like redirect, but /login is a meta refresh splash to /terms (the form)
  empty200   200 with an empty body on every path (a proxy that rewrote 204: online)
  evil       /login redirects to --evil-target (default the router's LAN address): the
             auto-accept walker must refuse it
Restart the process to forget accepted clients.
"""
import argparse
import http.server
import urllib.parse

ACCEPTED: set[str] = set()

TERMS = (
    b'<html><body><h1>Hotel login</h1>'
    b'<form method="post" action="/accept">'
    b'<input type="hidden" name="token" value="sim-1">'
    b'%s'
    b'<label><input type="checkbox" name="terms" value="yes"> I accept the terms</label>'
    b'<button type="submit" name="go" value="connect">Connect</button>'
    b'</form></body></html>'
)
ROOM = b'<input type="text" name="room" required>'
SPLASH = b'<html><head><meta http-equiv="refresh" content="0; url=/terms"></head></html>'


class Portal(http.server.BaseHTTPRequestHandler):
    mode = 'redirect'
    evil_target = 'http://192.168.1.1/'

    def send(self, code: int, body: bytes = b'', location: str = '') -> None:
        self.send_response(code)
        if location:
            self.send_header('Location', location)
        self.send_header('Content-Length', str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def form(self) -> bytes:
        return TERMS % (ROOM if self.mode == 'room' else b'')

    def do_GET(self) -> None:
        path = urllib.parse.urlsplit(self.path).path
        if self.client_address[0] in ACCEPTED:
            return self.send(204)
        if self.mode == 'empty200':
            return self.send(200)
        if self.mode == 'intercept':
            return self.send(200, self.form())
        if path == '/login' and self.mode == 'evil':
            return self.send(302, location=self.evil_target)
        if path == '/login':
            return self.send(200, SPLASH if self.mode == 'meta' else self.form())
        if path == '/terms':
            return self.send(200, self.form())
        return self.send(302, location='http://portal.hotel.test/login')

    def do_POST(self) -> None:
        length = int(self.headers.get('Content-Length') or 0)
        fields = urllib.parse.parse_qs(self.rfile.read(length).decode())
        ok = fields.get('terms') == ['yes'] and fields.get('token') == ['sim-1']
        if self.mode == 'room':
            ok = ok and bool(fields.get('room', [''])[0])
        if not ok:
            return self.send(200, self.form())
        ACCEPTED.add(self.client_address[0])
        return self.send(302, location='http://portal.hotel.test/welcome')


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument('--bind', default='10.66.0.1')
    parser.add_argument('--port', type=int, default=80)
    parser.add_argument('--mode', default='redirect', choices=['redirect', 'intercept', 'room', 'meta', 'empty200', 'evil'])
    parser.add_argument('--evil-target', default='http://192.168.1.1/')
    args = parser.parse_args()
    Portal.mode = args.mode
    Portal.evil_target = args.evil_target
    http.server.ThreadingHTTPServer((args.bind, args.port), Portal).serve_forever()


if __name__ == '__main__':
    main()

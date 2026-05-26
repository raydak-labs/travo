package services

import (
	"strings"
	"testing"
)

func TestParseWireguardConfig_ValidConfig(t *testing.T) {
	conf := `[Interface]
PrivateKey = yAnz5TF+lXXJte14tji3zlMNq+hd2rYUIgJBgB3fBmk=
Address = 10.200.200.2/32
DNS = 10.200.200.1

[Peer]
PublicKey = xTIBA5rboUvnH4htodjb6e697QjLERt1NAB4mZqp8Dg=
AllowedIPs = 0.0.0.0/0
Endpoint = demo.wireguard.com:12912
PersistentKeepalive = 25
`

	parsed, err := ParseWireguardConfig(conf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if parsed.Interface.PrivateKey != "yAnz5TF+lXXJte14tji3zlMNq+hd2rYUIgJBgB3fBmk=" {
		t.Errorf("unexpected private key: %q", parsed.Interface.PrivateKey)
	}
	if parsed.Interface.Address != "10.200.200.2/32" {
		t.Errorf("unexpected address: %q", parsed.Interface.Address)
	}
	if parsed.Interface.DNS != "10.200.200.1" {
		t.Errorf("unexpected DNS: %q", parsed.Interface.DNS)
	}
	if len(parsed.Peers) != 1 {
		t.Fatalf("expected 1 peer, got %d", len(parsed.Peers))
	}

	peer := parsed.Peers[0]
	if peer.PublicKey != "xTIBA5rboUvnH4htodjb6e697QjLERt1NAB4mZqp8Dg=" {
		t.Errorf("unexpected public key: %q", peer.PublicKey)
	}
	if peer.AllowedIPs != "0.0.0.0/0" {
		t.Errorf("unexpected allowed IPs: %q", peer.AllowedIPs)
	}
	if peer.Endpoint != "demo.wireguard.com:12912" {
		t.Errorf("unexpected endpoint: %q", peer.Endpoint)
	}
	if peer.PersistentKeepalive != 25 {
		t.Errorf("unexpected persistent keepalive: %d", peer.PersistentKeepalive)
	}
}

func TestParseWireguardConfig_MultiplePeers(t *testing.T) {
	conf := `[Interface]
PrivateKey = yAnz5TF+lXXJte14tji3zlMNq+hd2rYUIgJBgB3fBmk=
Address = 10.0.0.1/24

[Peer]
PublicKey = peer1pubkey=
AllowedIPs = 10.0.0.2/32
Endpoint = peer1.example.com:51820

[Peer]
PublicKey = peer2pubkey=
AllowedIPs = 10.0.0.3/32
Endpoint = peer2.example.com:51820
PresharedKey = pskvalue=
`

	parsed, err := ParseWireguardConfig(conf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(parsed.Peers) != 2 {
		t.Fatalf("expected 2 peers, got %d", len(parsed.Peers))
	}
	if parsed.Peers[0].PublicKey != "peer1pubkey=" {
		t.Errorf("unexpected peer 0 public key: %q", parsed.Peers[0].PublicKey)
	}
	if parsed.Peers[1].PublicKey != "peer2pubkey=" {
		t.Errorf("unexpected peer 1 public key: %q", parsed.Peers[1].PublicKey)
	}
	if parsed.Peers[1].PresharedKey != "pskvalue=" {
		t.Errorf("unexpected peer 1 preshared key: %q", parsed.Peers[1].PresharedKey)
	}
}

func TestParseWireguardConfig_WithComments(t *testing.T) {
	conf := `# This is a WireGuard config
[Interface]
# My private key
PrivateKey = yAnz5TF+lXXJte14tji3zlMNq+hd2rYUIgJBgB3fBmk=
Address = 10.0.0.1/24

# First peer
[Peer]
PublicKey = xTIBA5rboUvnH4htodjb6e697QjLERt1NAB4mZqp8Dg=
AllowedIPs = 0.0.0.0/0
`

	parsed, err := ParseWireguardConfig(conf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Interface.PrivateKey != "yAnz5TF+lXXJte14tji3zlMNq+hd2rYUIgJBgB3fBmk=" {
		t.Errorf("unexpected private key: %q", parsed.Interface.PrivateKey)
	}
	if len(parsed.Peers) != 1 {
		t.Fatalf("expected 1 peer, got %d", len(parsed.Peers))
	}
}

func TestParseWireguardConfig_MissingPrivateKey(t *testing.T) {
	conf := `[Interface]
Address = 10.0.0.1/24

[Peer]
PublicKey = xTIBA5rboUvnH4htodjb6e697QjLERt1NAB4mZqp8Dg=
AllowedIPs = 0.0.0.0/0
`

	_, err := ParseWireguardConfig(conf)
	if err == nil {
		t.Fatal("expected error for missing private key")
	}
	if !strings.Contains(err.Error(), "PrivateKey") {
		t.Errorf("expected error about PrivateKey, got: %v", err)
	}
}

func TestParseWireguardConfig_EmptyInput(t *testing.T) {
	_, err := ParseWireguardConfig("")
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestParseWireguardConfig_ExtraWhitespace(t *testing.T) {
	conf := `  [Interface]
  PrivateKey  =  yAnz5TF+lXXJte14tji3zlMNq+hd2rYUIgJBgB3fBmk=
  Address  =  10.200.200.2/32
  DNS  =  10.200.200.1

  [Peer]
  PublicKey  =  xTIBA5rboUvnH4htodjb6e697QjLERt1NAB4mZqp8Dg=
  AllowedIPs  =  0.0.0.0/0
  Endpoint  =  demo.wireguard.com:12912
`

	parsed, err := ParseWireguardConfig(conf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Interface.PrivateKey != "yAnz5TF+lXXJte14tji3zlMNq+hd2rYUIgJBgB3fBmk=" {
		t.Errorf("unexpected private key: %q", parsed.Interface.PrivateKey)
	}
	if parsed.Interface.Address != "10.200.200.2/32" {
		t.Errorf("unexpected address: %q", parsed.Interface.Address)
	}
	if parsed.Peers[0].PublicKey != "xTIBA5rboUvnH4htodjb6e697QjLERt1NAB4mZqp8Dg=" {
		t.Errorf("unexpected public key: %q", parsed.Peers[0].PublicKey)
	}
}

func TestParseWireguardConfig_MTUAndListenPort(t *testing.T) {
	conf := `[Interface]
PrivateKey = yAnz5TF+lXXJte14tji3zlMNq+hd2rYUIgJBgB3fBmk=
Address = 10.0.0.1/24
MTU = 1420
ListenPort = 51820

[Peer]
PublicKey = xTIBA5rboUvnH4htodjb6e697QjLERt1NAB4mZqp8Dg=
AllowedIPs = 0.0.0.0/0
`

	parsed, err := ParseWireguardConfig(conf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Interface.MTU != 1420 {
		t.Errorf("expected MTU 1420, got %d", parsed.Interface.MTU)
	}
	if parsed.Interface.ListenPort != 51820 {
		t.Errorf("expected ListenPort 51820, got %d", parsed.Interface.ListenPort)
	}
}

func TestParseWireguardConfig_PeerMissingPublicKey(t *testing.T) {
	conf := `[Interface]
PrivateKey = yAnz5TF+lXXJte14tji3zlMNq+hd2rYUIgJBgB3fBmk=
Address = 10.0.0.1/24

[Peer]
AllowedIPs = 0.0.0.0/0
Endpoint = demo.wireguard.com:12912
`

	_, err := ParseWireguardConfig(conf)
	if err == nil {
		t.Fatal("expected error for peer missing public key")
	}
	if !strings.Contains(err.Error(), "PublicKey") {
		t.Errorf("expected error about PublicKey, got: %v", err)
	}
}

func TestParseWireguardConfig_AmneziaWG1x(t *testing.T) {
	conf := `[Interface]
PrivateKey = yAnz5TF+lXXJte14tji3zlMNq+hd2rYUIgJBgB3fBmk=
Address = 10.8.0.2/32
DNS = 1.1.1.1
Jc = 5
Jmin = 40
Jmax = 70
S1 = 55
S2 = 33

[Peer]
PublicKey = xTIBA5rboUvnH4htodjb6e697QjLERt1NAB4mZqp8Dg=
AllowedIPs = 0.0.0.0/0
Endpoint = vpn.example.com:51820
`
	parsed, err := ParseWireguardConfig(conf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !parsed.IsAmnezia() {
		t.Error("expected IsAmnezia() to be true for AWG 1.x config")
	}
	if parsed.Interface.Jc != 5 {
		t.Errorf("Jc = %d, want 5", parsed.Interface.Jc)
	}
	if parsed.Interface.Jmin != 40 {
		t.Errorf("Jmin = %d, want 40", parsed.Interface.Jmin)
	}
	if parsed.Interface.Jmax != 70 {
		t.Errorf("Jmax = %d, want 70", parsed.Interface.Jmax)
	}
	if parsed.Interface.S1 != 55 {
		t.Errorf("S1 = %d, want 55", parsed.Interface.S1)
	}
	if parsed.Interface.S2 != 33 {
		t.Errorf("S2 = %d, want 33", parsed.Interface.S2)
	}
}

func TestParseWireguardConfig_AmneziaWG2(t *testing.T) {
	conf := `[Interface]
PrivateKey = yAnz5TF+lXXJte14tji3zlMNq+hd2rYUIgJBgB3fBmk=
Address = 10.8.0.2/32
DNS = 1.1.1.1
Jc = 4
Jmin = 40
Jmax = 70
S1 = 15
S2 = 25
H1 = 1234567890
H2 = 987654321
H3 = 1122334455
H4 = 3566778899

[Peer]
PublicKey = xTIBA5rboUvnH4htodjb6e697QjLERt1NAB4mZqp8Dg=
AllowedIPs = 0.0.0.0/0
Endpoint = vpn.example.com:51820
`
	parsed, err := ParseWireguardConfig(conf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !parsed.IsAmnezia() {
		t.Error("expected IsAmnezia() to be true for AWG 2.0 config")
	}
	if parsed.Interface.H1 != 1234567890 {
		t.Errorf("H1 = %d, want 1234567890", parsed.Interface.H1)
	}
	if parsed.Interface.H2 != 987654321 {
		t.Errorf("H2 = %d, want 987654321", parsed.Interface.H2)
	}
	if parsed.Interface.H3 != 1122334455 {
		t.Errorf("H3 = %d, want 1122334455", parsed.Interface.H3)
	}
	if parsed.Interface.H4 != 3566778899 {
		t.Errorf("H4 = %d, want 3566778899", parsed.Interface.H4)
	}
}

func TestParseWireguardConfig_StandardWGNotAmnezia(t *testing.T) {
	conf := `[Interface]
PrivateKey = yAnz5TF+lXXJte14tji3zlMNq+hd2rYUIgJBgB3fBmk=
Address = 10.8.0.2/32
DNS = 1.1.1.1

[Peer]
PublicKey = xTIBA5rboUvnH4htodjb6e697QjLERt1NAB4mZqp8Dg=
AllowedIPs = 0.0.0.0/0
Endpoint = vpn.example.com:51820
`
	parsed, err := ParseWireguardConfig(conf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.IsAmnezia() {
		t.Error("expected IsAmnezia() to be false for standard WG config")
	}
}

func TestParseWireguardConfig_AmneziaWGCaseInsensitive(t *testing.T) {
	conf := `[Interface]
PrivateKey = yAnz5TF+lXXJte14tji3zlMNq+hd2rYUIgJBgB3fBmk=
Address = 10.8.0.2/32
jC = 3
JMIN = 20
jmax = 50

[Peer]
PublicKey = xTIBA5rboUvnH4htodjb6e697QjLERt1NAB4mZqp8Dg=
AllowedIPs = 0.0.0.0/0
`
	parsed, err := ParseWireguardConfig(conf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !parsed.IsAmnezia() {
		t.Error("expected IsAmnezia() to be true")
	}
	if parsed.Interface.Jc != 3 {
		t.Errorf("Jc = %d, want 3", parsed.Interface.Jc)
	}
	if parsed.Interface.Jmin != 20 {
		t.Errorf("Jmin = %d, want 20", parsed.Interface.Jmin)
	}
	if parsed.Interface.Jmax != 50 {
		t.Errorf("Jmax = %d, want 50", parsed.Interface.Jmax)
	}
}

func TestParseWireguardConfig_AmneziaInvalidIntegers(t *testing.T) {
	conf := `[Interface]
PrivateKey = yAnz5TF+lXXJte14tji3zlMNq+hd2rYUIgJBgB3fBmk=
Address = 10.8.0.2/32
Jc = notanumber
S1 = 42

[Peer]
PublicKey = xTIBA5rboUvnH4htodjb6e697QjLERt1NAB4mZqp8Dg=
AllowedIPs = 0.0.0.0/0
`
	parsed, err := ParseWireguardConfig(conf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Interface.Jc != 0 {
		t.Errorf("Jc = %d, want 0 (invalid input)", parsed.Interface.Jc)
	}
	if parsed.Interface.S1 != 42 {
		t.Errorf("S1 = %d, want 42", parsed.Interface.S1)
	}
}

func TestSplitWireGuardEndpoint(t *testing.T) {
	cases := []struct {
		in, host, port string
	}{
		{"vpn.example.com:51820", "vpn.example.com", "51820"},
		{"1.2.3.4:12345", "1.2.3.4", "12345"},
		{"[2001:db8::1]:51820", "2001:db8::1", "51820"},
		{"nohostyet", "nohostyet", "51820"},
	}
	for _, tc := range cases {
		h, p := SplitWireGuardEndpoint(tc.in)
		if h != tc.host || p != tc.port {
			t.Fatalf("SplitWireGuardEndpoint(%q) = (%q,%q), want (%q,%q)", tc.in, h, p, tc.host, tc.port)
		}
	}
}

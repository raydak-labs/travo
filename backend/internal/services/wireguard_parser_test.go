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

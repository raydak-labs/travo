package api

import (
	"encoding/base64"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/vpnprofiles"
)

func TestVPNProfileRoutes(t *testing.T) {
	const path = "/api/v1/vpn/profiles"
	e := newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) { d.VPNProfiles = vpnprofiles.New(filepath.Join(t.TempDir(), "profiles.json")) })
	private := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("a", 32)))
	public := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("b", 32)))
	makeBody := func(config string) string {
		body, err := json.Marshal(map[string]string{"name": "Hotel VPN", "config": config})
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	body := makeBody("[Interface]\nPrivateKey=" + private + "\nAddress=10.0.0.2/24\n[Peer]\nPublicKey=" + public + "\nAllowedIPs=0.0.0.0/0\nEndpoint=private.test:51820")
	if got := e.do(req{path: path}); got.Code != 401 {
		t.Fatal(got.Code)
	}
	cookie, csrf := e.login()
	if got := e.do(req{path: path, cookie: cookie}); got.Code != 200 || strings.TrimSpace(got.Body.String()) != `{"profiles":[],"selected_id":null}` {
		t.Fatal(got.Code, got.Body)
	}
	if got := e.do(req{method: "POST", path: path, cookie: cookie, body: body}); got.Code != 403 {
		t.Fatal(got.Code)
	}
	post := func(data string) req { return req{method: "POST", path: path, cookie: cookie, csrf: csrf, body: data} }
	got := e.do(post(body))
	if got.Code != 201 {
		t.Fatal(got.Code, got.Body)
	}
	id := bodyField(t, got, "id")
	for _, r := range []req{
		{path: path, cookie: cookie},
		{method: "POST", path: path + "/" + id + "/select", cookie: cookie, csrf: csrf},
		{method: "DELETE", path: path + "/" + id, cookie: cookie, csrf: csrf},
	} {
		result := e.do(r)
		output := result.Body.String()
		if result.Code != 200 || strings.Contains(output, private) || strings.Contains(output, "private.test") {
			t.Fatal("unexpected response", result.Code)
		}
	}
	for _, test := range []struct {
		r      req
		status int
		code   string
	}{
		{post(makeBody(private)), 400, "invalid_profile"},
		{post(`{"name":"valid","config":"valid","` + private + `":true}`), 400, "invalid_request"},
		{post(makeBody(strings.Repeat("x", 512<<10))), 413, ""},
		{req{method: "DELETE", path: path + "/bad", cookie: cookie, csrf: csrf}, 400, "invalid_request"},
		{req{method: "DELETE", path: path + "/" + id, cookie: cookie, csrf: csrf}, 404, "profile_not_found"},
	} {
		got := e.do(test.r)
		if got.Code != test.status || test.code != "" && bodyField(t, got, "code") != test.code || strings.Contains(got.Body.String(), private) {
			t.Fatal("unexpected error response", got.Code, got.Body)
		}
	}
}

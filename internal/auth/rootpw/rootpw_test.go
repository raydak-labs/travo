package rootpw

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestParseShadow(t *testing.T) {
	cases := []struct {
		name, in string
		set      bool
		err      bool
	}{
		{"empty field is open", "root::0:0:99999:7:::\ndaemon:*:0:0:99999:7:::\n", false, false},
		{"locked !", "root:!:0:0:99999:7:::\n", true, false},
		{"locked *", "root:*:0:0:99999:7:::\n", true, false},
		{"x only", "root:x:0:0:99999:7:::\n", true, false},
		{"sha256crypt hash (container capture shape)", "root:$5$salt$hash:0:0:99999:7:::\n", true, false},
		{"root not first", "daemon:*:0:0:::::\nroot:$1$a$b:0:0:::::\n", true, false},
		{"crlf", "root::0:0:::::\r\n", false, false},
		{"missing root line", "daemon:*:0:0:::::\n", false, true},
		{"empty file", "", false, true},
		{"root without field", "root\n", false, true},
		{"prefix is not root", "rootx::0:0:::::\n", false, true},
	}
	for _, c := range cases {
		set, err := ParseShadow(c.in)
		if (err != nil) != c.err || set != c.set {
			t.Errorf("%s: set=%v err=%v", c.name, set, err)
		}
		if err != nil && !errors.Is(err, ErrShadow) {
			t.Errorf("%s: error is not ErrShadow", c.name)
		}
	}
}

func TestLoginArgsKeepThePasswordInsideOneJSONArgument(t *testing.T) {
	pw := `we"ird'; $(rm -rf /) \ pass`
	args, err := LoginArgs(pw)
	if err != nil {
		t.Fatal(err)
	}
	if len(args) != 4 || strings.Join(args[:3], " ") != "call session login" {
		t.Fatalf("argv %q", args)
	}
	var got struct {
		Username, Password string
		Timeout            int
	}
	if err := json.Unmarshal([]byte(args[3]), &got); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	if got.Username != "root" || got.Password != pw || got.Timeout != 1 {
		t.Fatalf("%+v", got)
	}
}

func TestLoginArgsRefuseNUL(t *testing.T) {
	if _, err := LoginArgs("abc\x00def"); !errors.Is(err, ErrNUL) {
		t.Fatalf("got %v", err)
	}
}

func TestPasswdInputIsPasswordTwice(t *testing.T) {
	if got := PasswdInput("s3cret-pass"); got != "s3cret-pass\ns3cret-pass\n" {
		t.Fatalf("%q", got)
	}
}

package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/blackdark-org/travo-greenfield/internal/sshkeys"
)

// SSHKeys manages root's dropbear authorized_keys (internal/sshkeys, invariants §3).
type SSHKeys interface {
	List(context.Context) (sshkeys.KeyList, error)
	Add(context.Context, string) (sshkeys.PublicKey, error)
	Delete(context.Context, string) error
}

type sshKeyAddInput struct {
	Body struct {
		Key string `json:"key" minLength:"1" maxLength:"8192" doc:"One OpenSSH public key line: type, base64, optional comment; no options"`
	}
}

type sshKeyIDInput struct {
	ID string `path:"id" pattern:"^[0-9a-f]{64}$" doc:"Key id from the list (hex sha256 of the key)"`
}

func (s *server) registerSSHKeys(a huma.API) {
	const path = "/api/v1/system/ssh-keys"
	register(s, false, op("getSSHKeys", http.MethodGet, path, "Root SSH public keys", "system", 503), a, s.listSSHKeys)
	// Adding a key grants a root shell, like the password: a browser session only.
	add := op("addSSHKey", http.MethodPost, path, "Authorize a root SSH public key", "system", 400, 409, 503)
	add.DefaultStatus = http.StatusCreated
	add.MaxBodyBytes = 16 << 10
	registerSessionOnly(s, add, a, s.addSSHKey)
	registerSessionOnly(s, op("deleteSSHKey", http.MethodDelete, path+"/{id}", "Remove a root SSH public key", "system", 404, 503), a, s.deleteSSHKey)
}

func sshKeyError(err error) error {
	switch {
	case errors.Is(err, sshkeys.ErrInvalid):
		return fail(http.StatusBadRequest, "ssh_key_invalid", "not a single OpenSSH public key line (type, base64, comment; no options)")
	case errors.Is(err, sshkeys.ErrExists):
		return fail(http.StatusConflict, "ssh_key_exists", "this key is already authorized")
	case errors.Is(err, sshkeys.ErrLimit):
		return fail(http.StatusConflict, "ssh_key_limit", "key limit reached; remove one first")
	case errors.Is(err, sshkeys.ErrNotFound):
		return fail(http.StatusNotFound, "not_found", "no such SSH key")
	}
	slog.Warn("ssh keys", "err", err)
	return fail(http.StatusServiceUnavailable, "ssh_keys_unavailable", "authorized_keys is unavailable")
}

func (s *server) listSSHKeys(ctx context.Context, _ *struct{}) (*struct{ Body sshkeys.KeyList }, error) {
	if s.SSHKeys == nil {
		return nil, sshKeyError(errors.New("no store"))
	}
	l, err := s.SSHKeys.List(ctx)
	if err != nil {
		return nil, sshKeyError(err)
	}
	return &struct{ Body sshkeys.KeyList }{l}, nil
}

func (s *server) addSSHKey(ctx context.Context, in *sshKeyAddInput) (*struct{ Body sshkeys.PublicKey }, error) {
	if s.SSHKeys == nil {
		return nil, sshKeyError(errors.New("no store"))
	}
	k, err := s.SSHKeys.Add(ctx, in.Body.Key)
	if err != nil {
		return nil, sshKeyError(err)
	}
	return &struct{ Body sshkeys.PublicKey }{k}, nil
}

func (s *server) deleteSSHKey(ctx context.Context, in *sshKeyIDInput) (*struct{ Body okBody }, error) {
	if s.SSHKeys == nil {
		return nil, sshKeyError(errors.New("no store"))
	}
	if err := s.SSHKeys.Delete(ctx, in.ID); err != nil {
		return nil, sshKeyError(err)
	}
	out := &struct{ Body okBody }{}
	out.Body.Status = "ok"
	return out, nil
}

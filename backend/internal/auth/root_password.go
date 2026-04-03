package auth

import "sync"

// RootPassword holds the system root password in memory after a successful login.
// It is shared between AuthService (which writes it) and UCIApplyConfirm (which reads it)
// so that staged apply operations can authenticate with rpcd using the user's credentials.
type RootPassword struct {
	mu   sync.RWMutex
	pass string
}

// NewRootPassword creates an empty holder.
func NewRootPassword() *RootPassword {
	return &RootPassword{}
}

// Set stores the password.
func (r *RootPassword) Set(p string) {
	r.mu.Lock()
	r.pass = p
	r.mu.Unlock()
}

// Get returns the stored password.
func (r *RootPassword) Get() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.pass
}

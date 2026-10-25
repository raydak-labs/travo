package models

// LoginRequest is the payload for authentication.
type LoginRequest struct {
	Password string `json:"password"`
}

// LoginResponse is returned after successful authentication. ExpiresIn is
// relative (seconds) so clients never compare server and client wall clocks.
type LoginResponse struct {
	Token     string `json:"token"`
	ExpiresIn int64  `json:"expires_in"`
}

// SessionResponse is returned by GET /auth/session.
type SessionResponse struct {
	Valid     bool  `json:"valid"`
	ExpiresIn int64 `json:"expires_in"`
}

// ChangePasswordRequest is the payload for changing the admin password.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// Session represents an active authentication session.
type Session struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
	CreatedAt string `json:"created_at"`
}

package models

// LoginRequest is the payload for authentication.
type LoginRequest struct {
	Password string `json:"password"`
}

// LoginResponse is returned after successful authentication.
type LoginResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
}

// Session represents an active authentication session.
type Session struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
	CreatedAt string `json:"created_at"`
}

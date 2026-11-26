package pkgmgr

import "context"

// opSlot is the process-wide package-op lock: apk and opkg hold one database, and a free
// flash check is only true while no other install runs.
var opSlot = make(chan struct{}, 1)

// Exclusive waits for the package-op lock; every install, remove and list refresh holds it
// across its free-flash check and package calls. release must be called exactly once.
func Exclusive(ctx context.Context) (release func(), err error) {
	select {
	case opSlot <- struct{}{}:
		return func() { <-opSlot }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

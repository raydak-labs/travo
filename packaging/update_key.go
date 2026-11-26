// Package packaging exposes the release trust anchor shipped with the daemon.
package packaging

import _ "embed"

//go:embed minisign.pub
var UpdatePublicKey string

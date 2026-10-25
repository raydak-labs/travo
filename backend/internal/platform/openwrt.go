package platform

import (
	"sync"
)

const IPBin = "/sbin/ip"

const IfupBin = "/sbin/ifup"

const IfdownBin = "/sbin/ifdown"

// CrontabMu serialises read-modify-write of /etc/crontabs/root. Every writer of
// that file takes it: the WiFi schedule, the LED schedule and the auto-reconnect
// entry, which all reconcile at boot within a second of each other. It is keyed
// on the file, not on a UCI config.
var CrontabMu sync.Mutex

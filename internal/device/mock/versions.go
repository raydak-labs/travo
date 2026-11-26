package mock

import _ "embed"

// VersionsIndex is the OpenWrt release index mock mode answers instead of downloads.openwrt.org.
//
//go:embed testdata/versions.json
var VersionsIndex []byte

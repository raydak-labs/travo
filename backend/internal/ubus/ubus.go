package ubus

// Ubus abstracts the OpenWRT ubus message bus.
type Ubus interface {
	Call(path, method string, args map[string]interface{}) (map[string]interface{}, error)
}

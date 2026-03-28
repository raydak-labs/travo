package uci

// UCI abstracts the OpenWRT UCI configuration system.
type UCI interface {
	Get(config, section, option string) (string, error)
	Set(config, section, option, value string) error
	GetAll(config, section string) (map[string]string, error)
	GetSections(config string) (map[string]map[string]string, error)
	Commit(config string) error
	AddSection(config, section, stype string) error
	AddList(config, section, option, value string) error
	DeleteSection(config, section string) error
	DeleteOption(config, section, option string) error
}

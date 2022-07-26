package config

type GitConfig struct {
	Disabled bool
	Order    int

	Uri            string
	KnownHostsFile string `json:"knownHostsFile"`
	PrivateKey     string `json:"privateKey"`

	Basedir                string `json:"basedir"`
	DisableBaseDirCleaning bool   `json:"disableBaseDirCleaning"`

	DisableLabels     bool   `json:"disableLabels"`
	DefaultBranchName string `json:"defaultBranchName"`

	CloneOnStart bool `json:"clone-on-start"`
	ForcePull    bool `json:"force-pull"`

	ShowProgress bool `json:"show-progress"`

	TimeoutMillis     int64 `json:"timeout"`
	RefreshRateMillis int64 `json:"refreshRate"`
}

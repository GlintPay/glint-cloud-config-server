package config

type Configuration struct {
	ApplicationConfigFileYmlPath string `env:"APP_CONFIG_FILE_YML_PATH" envDefault:"application.yml"`
}

// ApplicationConfiguration Must use full names for `sigs.k8s.io/yaml`
type ApplicationConfiguration struct {
	Server     Server
	Prometheus Prometheus
	File       FileConfig
	Git        GitConfig
	Kubernetes K8sConfig
	Defaults   Defaults
	Tracing    Tracing
	Gotemplate GoTemplate
}

type Defaults struct {
	ResolvePropertySources    bool
	FlattenHierarchicalConfig bool
	FlattenedIndexedLists     bool
	LogResponses              bool
	PrettyPrintJson           bool
}

type Server struct {
	Port int
}

type Tracing struct {
	Enabled         bool
	Endpoint        string
	SamplerFraction float64
}

type Prometheus struct {
	Path string
}

type GoTemplate struct {
	LeftDelim  string
	RightDelim string
}

func (t GoTemplate) Validate() GoTemplate {
	if t.LeftDelim == "" {
		t.LeftDelim = "{{"
	}
	if t.RightDelim == "" {
		t.RightDelim = "}}"
	}
	return t
}

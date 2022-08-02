package api

type ConfigurationRequest struct {
	Applications []string
	Profiles     []string
	Labels       LabelsRequest

	RefreshBackend        bool
	FlattenHierarchies    bool
	FlattenedIndexedLists bool
	LogResponses          bool
	PrettyPrintJson       bool

	EnableTrace bool
}

type LabelsRequest struct {
	Branch string
}

type ResolvedConfigValues map[string]interface{}

type ResolutionMetadata struct {
	PrecedenceDisplayMessage string
}

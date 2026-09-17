package api

type Limits struct {
	JARBytes            int64 `json:"jarBytes"`
	TextArtifactBytes   int64 `json:"textArtifactBytes"`
	PreviewBytes        int64 `json:"previewBytes"`
	TopologyNodes       int   `json:"topologyNodes"`
	TopologyConnections int   `json:"topologyConnections"`
}

type GeneratorCapability struct {
	ID             string   `json:"id"`
	Outputs        []string `json:"outputs"`
	ChartFormats   []string `json:"chartFormats"`
	SupportedKinds []string `json:"supportedKinds"`
	SupportedModes []string `json:"supportedModes"`
}

type Capabilities struct {
	BlueprintVersions []string              `json:"blueprintVersions"`
	Generators        []GeneratorCapability `json:"generators"`
	Limits            Limits                `json:"limits"`
}

type Health struct {
	Status string `json:"status"`
}

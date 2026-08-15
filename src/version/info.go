package version

const (
	Protocol      = "BastilleDTL"
	Version       = "1.0.0"
	SchemaVersion = "bastille/v1"
)

type Info struct {
	Protocol      string `json:"protocol"`
	Version       string `json:"version"`
	SchemaVersion string `json:"schemaVersion"`
}

func Current() Info {
	return Info{Protocol: Protocol, Version: Version, SchemaVersion: SchemaVersion}
}

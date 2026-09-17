package chart

import "go-deployer/internal/config"

const (
	FormatLegacy = config.ChartFormatLegacy
	FormatV1     = config.ChartFormatV1
)

type Metadata struct {
	Format string
}

// ValidateFile loads a Crosswalk chart, applies the same defaults used by
// deployment, and returns its normalized format without contacting a server.
func ValidateFile(path string) (Metadata, error) {
	value, err := config.Load(path)
	if err != nil {
		return Metadata{}, err
	}
	return Metadata{Format: value.Format}, nil
}

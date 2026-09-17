package config

import (
	"strings"
	"testing"
)

func TestValidateChartFormatPreservesLegacyCompatibility(t *testing.T) {
	value := Config{}
	for attempt := 1; attempt <= 2; attempt++ {
		var errs []string
		validateChartFormat(&value, &errs)
		if len(errs) != 0 || value.Format != ChartFormatLegacy {
			t.Fatalf("attempt=%d format=%q errors=%v", attempt, value.Format, errs)
		}
	}
}

func TestValidateChartFormatAcceptsV1AndRejectsUnknown(t *testing.T) {
	accepted := Config{Format: " CROSSWALK/V1 "}
	var acceptedErrors []string
	validateChartFormat(&accepted, &acceptedErrors)
	if len(acceptedErrors) != 0 || accepted.Format != ChartFormatV1 {
		t.Fatalf("format=%q errors=%v", accepted.Format, acceptedErrors)
	}

	rejected := Config{Format: "crosswalk/v2"}
	var rejectedErrors []string
	validateChartFormat(&rejected, &rejectedErrors)
	if len(rejectedErrors) != 1 || !strings.Contains(rejectedErrors[0], ChartFormatV1) {
		t.Fatalf("errors=%v", rejectedErrors)
	}
}

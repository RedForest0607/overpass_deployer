package domain

import (
	"encoding/json"
	"os"
	"testing"
)

func TestISTOCKMALLDEVTopologySample(t *testing.T) {
	body, err := os.ReadFile("../../../examples/istockmall-dev-topology.json")
	if err != nil {
		t.Fatal(err)
	}
	var sample struct {
		Environment Environment     `json:"environment"`
		Blueprint   json.RawMessage `json:"blueprint"`
		Layout      json.RawMessage `json:"layout"`
	}
	if err := json.Unmarshal(body, &sample); err != nil {
		t.Fatal(err)
	}
	if sample.Environment != EnvironmentDev {
		t.Fatalf("environment=%q", sample.Environment)
	}
	if err := ValidateTopologyStructure(sample.Blueprint, sample.Layout, 500, 1000); err != nil {
		t.Fatalf("sample must satisfy the persisted topology contract: %v", err)
	}
	validation := ValidateDesign(sample.Blueprint, "istockmall-sample")
	if !validation.Valid {
		t.Fatalf("sample must pass design validation: %#v", validation.Issues)
	}
}

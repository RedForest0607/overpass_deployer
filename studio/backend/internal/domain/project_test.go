package domain

import (
	"encoding/json"
	"testing"
)

func TestValidateTopologyStructureAllowsIncompleteDesign(t *testing.T) {
	blueprint := json.RawMessage(`{"apiVersion":"studio.deployer/v1","nodes":[{"id":"app","kind":"java","name":"","spec":{"artifactRef":null,"javaVersion":"","maxHeap":"","ports":[]}}],"placements":[],"connections":[]}`)
	layout := json.RawMessage(`{"nodes":[{"nodeId":"app","position":{"x":0,"y":0}}],"viewport":{"x":0,"y":0,"zoom":1}}`)
	if err := ValidateTopologyStructure(blueprint, layout, 10, 10); err != nil {
		t.Fatal(err)
	}
}

func TestValidateTopologyStructureRejectsMissingKindSpecificFields(t *testing.T) {
	blueprint := json.RawMessage(`{"apiVersion":"studio.deployer/v1","nodes":[{"id":"app","kind":"java","name":"app","spec":{}}],"placements":[],"connections":[]}`)
	layout := json.RawMessage(`{"nodes":[],"viewport":{"x":0,"y":0,"zoom":1}}`)
	if err := ValidateTopologyStructure(blueprint, layout, 10, 10); err == nil {
		t.Fatal("expected missing Java spec fields to be rejected")
	}
}

func TestValidateTopologyStructureRejectsUnknownFieldsAndMissingArrays(t *testing.T) {
	for _, blueprint := range []string{
		`{"apiVersion":"studio.deployer/v1","nodes":[],"placements":[],"connections":[],"selected":true}`,
		`{"apiVersion":"studio.deployer/v1","nodes":[]}`,
	} {
		err := ValidateTopologyStructure(json.RawMessage(blueprint), json.RawMessage(`{"nodes":[],"viewport":{"x":0,"y":0,"zoom":1}}`), 10, 10)
		if err == nil {
			t.Fatalf("expected invalid blueprint: %s", blueprint)
		}
	}
}

package domain

import (
	"encoding/json"
	"testing"
)

func designFixture() map[string]any {
	return map[string]any{
		"apiVersion": "studio.deployer/v1",
		"nodes": []any{
			map[string]any{"id": "vm", "kind": "vm", "name": "host", "spec": map[string]any{"provider": "on-prem", "host": "10.0.0.1"}},
			map[string]any{"id": "app", "kind": "java", "name": "app", "spec": map[string]any{"artifactRef": nil, "javaVersion": "21", "maxHeap": "1g", "ports": []any{map[string]any{"name": "main", "servicePort": 8080, "protocol": "TCP"}}}},
			map[string]any{"id": "redis", "kind": "redis", "name": "redis", "spec": map[string]any{"location": "hosted", "ports": []any{map[string]any{"name": "main", "servicePort": 6379, "protocol": "TCP"}}}},
		},
		"placements":  []any{map[string]any{"workloadRef": "app", "targetRef": "vm", "mode": "process"}, map[string]any{"workloadRef": "redis", "targetRef": "vm", "mode": "process"}},
		"connections": []any{map[string]any{"id": "link", "kind": "connects_to", "sourceRef": "app", "targetRef": "redis", "targetPortName": "main"}},
	}
}

func TestDesignRules(t *testing.T) {
	cases := []struct {
		name, code string
		change     func(map[string]any)
	}{
		{"valid", "", nil},
		{"port collision", "HOST_PORT_CONFLICT", func(d map[string]any) {
			d["nodes"].([]any)[2].(map[string]any)["spec"].(map[string]any)["ports"].([]any)[0].(map[string]any)["servicePort"] = 8080
		}},
		{"bad reference", "INVALID_PLACEMENT", func(d map[string]any) {
			d["placements"].([]any)[0].(map[string]any)["targetRef"] = "missing"
		}},
		{"mode mismatch", "INVALID_PLACEMENT", func(d map[string]any) {
			d["placements"].([]any)[0].(map[string]any)["mode"] = "kubernetes"
		}},
		{"duplicate deployment", "PLACEMENT_COUNT", func(d map[string]any) {
			d["placements"] = append(d["placements"].([]any), d["placements"].([]any)[0])
		}},
		{"external deployed", "PLACEMENT_COUNT", func(d map[string]any) {
			spec := d["nodes"].([]any)[2].(map[string]any)["spec"].(map[string]any)
			spec["location"] = "external"
			spec["endpoint"] = "redis.internal"
		}},
		{"invalid port reference", "INVALID_PORT_REFERENCE", func(d map[string]any) {
			d["connections"].([]any)[0].(map[string]any)["targetPortName"] = "other"
		}},
		{"bad heap", "JVM_HEAP", func(d map[string]any) {
			d["nodes"].([]any)[1].(map[string]any)["spec"].(map[string]any)["maxHeap"] = "-1g"
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := designFixture()
			if tc.change != nil {
				tc.change(doc)
			}
			raw, err := json.Marshal(doc)
			if err != nil {
				t.Fatal(err)
			}
			if err = ValidateTopologyStructure(raw, json.RawMessage(`{"nodes":[],"viewport":{"x":0,"y":0,"zoom":1}}`), 500, 1000); err != nil {
				t.Fatal(err)
			}
			got := ValidateDesign(raw, "edit-1")
			if got.ClientRequestID != "edit-1" {
				t.Fatal("request ID lost")
			}
			if tc.code == "" {
				if !got.Valid {
					t.Fatalf("unexpected errors: %+v", got)
				}
				return
			}
			for _, issue := range got.Issues {
				if issue.Code == tc.code {
					if got.Valid {
						t.Fatal("error marked valid")
					}
					return
				}
			}
			t.Fatalf("missing %s in %+v", tc.code, got)
		})
	}
}

func TestLegacyRegistrySecretReferenceRemainsValid(t *testing.T) {
	doc := designFixture()
	doc["nodes"] = append(doc["nodes"].([]any), map[string]any{
		"id": "registry", "kind": "registry", "name": "registry",
		"spec": map[string]any{"endpoint": "harbor.example.com", "secretRef": "legacy-harbor-secret"},
	})
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if result := ValidateDesign(raw, "legacy-registry"); !result.Valid {
		t.Fatalf("legacy Registry reference should remain valid: %+v", result.Issues)
	}
}

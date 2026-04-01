package config

import (
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestStringListUnmarshalScalar(t *testing.T) {
	var actual struct {
		ExtraOpts StringList `yaml:"extra_opts"`
	}

	input := []byte(`extra_opts: -Dspring.profiles.active=prod --flag "value with spaces"`)
	if err := yaml.Unmarshal(input, &actual); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	expected := StringList{
		"-Dspring.profiles.active=prod",
		"--flag",
		"value with spaces",
	}
	if !reflect.DeepEqual(actual.ExtraOpts, expected) {
		t.Fatalf("unexpected extra opts: got %#v want %#v", actual.ExtraOpts, expected)
	}
}

func TestStringListUnmarshalSequence(t *testing.T) {
	var actual struct {
		ExtraOpts StringList `yaml:"extra_opts"`
	}

	input := []byte("extra_opts:\n  - -Dfoo=bar\n  - --debug\n")
	if err := yaml.Unmarshal(input, &actual); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	expected := StringList{"-Dfoo=bar", "--debug"}
	if !reflect.DeepEqual(actual.ExtraOpts, expected) {
		t.Fatalf("unexpected extra opts: got %#v want %#v", actual.ExtraOpts, expected)
	}
}

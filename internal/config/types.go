package config

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// StringList supports both a single YAML string and a YAML string sequence.
type StringList []string

func (s *StringList) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		var value string
		if err := node.Decode(&value); err != nil {
			return err
		}
		if value == "" {
			*s = nil
			return nil
		}
		values, err := splitShellWords(value)
		if err != nil {
			return err
		}
		*s = values
		return nil
	case yaml.SequenceNode:
		var values []string
		if err := node.Decode(&values); err != nil {
			return err
		}
		*s = values
		return nil
	default:
		return fmt.Errorf("expected string or list of strings, got YAML node kind %d", node.Kind)
	}
}

func splitShellWords(value string) ([]string, error) {
	var result []string
	var current strings.Builder
	var quote rune
	escaped := false

	flush := func() {
		if current.Len() == 0 {
			return
		}
		result = append(result, current.String())
		current.Reset()
	}

	for _, r := range value {
		switch {
		case escaped:
			current.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
		case r == ' ' || r == '\t' || r == '\n':
			flush()
		default:
			current.WriteRune(r)
		}
	}

	if escaped {
		return nil, fmt.Errorf("unterminated escape sequence")
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quoted string")
	}
	flush()
	return result, nil
}

package config

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// StringList는 YAML에서 단일 문자열과 문자열 배열을 모두 같은 옵션 목록으로 받아들인다.
type StringList []string

// UnmarshalYAML은 문자열 입력을 쉘 인자처럼 분리하고 배열 입력은 그대로 보존한다.
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

// splitShellWords는 따옴표와 이스케이프를 고려해 문자열 옵션을 공백 기준 인자 목록으로 나눈다.
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

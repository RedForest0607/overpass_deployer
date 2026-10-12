package config

import (
	"fmt"
	"path/filepath"
	"strings"
)

// validateTags는 태그를 소문자로 정규화하고 빈 값, 허용되지 않은 문자, 중복을 정리한다.
func validateTags(tags *[]string, prefix string, errs *[]string, checkUnresolvedEnv func(string, string)) {
	if tags == nil {
		return
	}

	seen := make(map[string]struct{}, len(*tags))
	normalized := make([]string, 0, len(*tags))

	for i, tag := range *tags {
		field := fmt.Sprintf("%s[%d]", prefix, i)
		checkUnresolvedEnv(tag, field)

		tag = normalizeTag(tag)
		if tag == "" {
			*errs = append(*errs, field+" must not be empty")
			continue
		}
		if !isValidTag(tag) {
			*errs = append(*errs, field+" must contain only letters, numbers, dots, hyphens, or underscores")
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		normalized = append(normalized, tag)
	}

	*tags = normalized
}

// normalizeTag는 태그 비교가 일관되도록 공백을 제거하고 소문자로 맞춘다.
func normalizeTag(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// isValidBastionAlias는 SSH alias로 안전하게 사용할 수 있는 문자만 포함하는지 확인한다.
func isValidBastionAlias(value string) bool {
	return bastionAliasPattern.MatchString(strings.TrimSpace(value))
}

// isValidTag는 태그 필터에 사용할 수 있는 안전한 문자 집합인지 확인한다.
func isValidTag(value string) bool {
	return bastionAliasPattern.MatchString(strings.TrimSpace(value))
}

func isSafeOwnerToken(value string) bool {
	return bastionAliasPattern.MatchString(strings.TrimSpace(value))
}

func hasProdLikeTags(tags []string) bool {
	for _, tag := range tags {
		switch normalizeTag(tag) {
		case "prod", "prd":
			return true
		}
	}
	return false
}

func rejectProdSampleFile(path string, fieldName string, prodLike bool, errs *[]string) {
	if !prodLike {
		return
	}
	base := strings.ToLower(filepath.Base(path))
	if strings.HasSuffix(base, ".env.sample") {
		*errs = append(*errs, fieldName+" cannot reference .env.sample for prod/prd servers")
		return
	}
	if strings.Contains(strings.ToLower(path), string(filepath.Separator)+"env"+string(filepath.Separator)) && strings.HasSuffix(base, ".sample") {
		*errs = append(*errs, fieldName+" cannot reference sample env files for prod/prd servers")
	}
}

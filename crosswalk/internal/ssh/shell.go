package ssh

import "strings"

// ShellQuote는 문자열 하나를 원격 쉘 명령의 단일 인자로 안전하게 전달하도록 이스케이프한다.
func ShellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

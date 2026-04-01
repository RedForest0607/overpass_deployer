package ssh

import "testing"

func TestShellQuote(t *testing.T) {
	tests := map[string]string{
		"":             "''",
		"/opt/app":     "'/opt/app'",
		"it's risky":   `'it'"'"'s risky'`,
		"two words":    "'two words'",
		"$HOME/config": "'$HOME/config'",
	}

	for input, expected := range tests {
		if actual := ShellQuote(input); actual != expected {
			t.Fatalf("unexpected quote for %q: got %q want %q", input, actual, expected)
		}
	}
}

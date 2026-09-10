package ssh

import (
	"strings"
	"sync"
	"testing"
)

func TestSynchronizedBufferSupportsConcurrentOutput(t *testing.T) {
	var buffer synchronizedBuffer
	var writers sync.WaitGroup
	for _, text := range []string{"stdout", "stderr"} {
		writers.Add(1)
		go func(text string) {
			defer writers.Done()
			if _, err := buffer.Write([]byte(text)); err != nil {
				t.Errorf("Write() returned error: %v", err)
			}
		}(text)
	}
	writers.Wait()

	output := buffer.String()
	for _, text := range []string{"stdout", "stderr"} {
		if !strings.Contains(output, text) {
			t.Fatalf("combined output does not contain %q: %q", text, output)
		}
	}
}

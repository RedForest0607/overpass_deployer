package main

import "testing"

func TestHealthError(t *testing.T) {
	if err := healthError("false"); err != nil {
		t.Fatalf("healthy fixture returned error: %v", err)
	}
	if err := healthError("true"); err == nil {
		t.Fatal("requested health failure was ignored")
	}
}

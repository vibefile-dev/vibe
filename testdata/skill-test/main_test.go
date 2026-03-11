package main

import "testing"

func TestAdd(t *testing.T) {
	if got := Add(2, 3); got != 5 {
		t.Errorf("Add(2, 3) = %d, want 5", got)
	}
}

func TestAddNegative(t *testing.T) {
	if got := Add(-1, 1); got != 0 {
		t.Errorf("Add(-1, 1) = %d, want 0", got)
	}
}

package executor

import (
	"testing"
)

func TestResultIsSuccess(t *testing.T) {
	r := &Result{ExitCode: 0}
	if !r.IsSuccess() {
		t.Error("exit 0 should be success")
	}
	if r.IsLegitimateFailure() || r.IsPreconditionFailure() || r.IsScriptError() {
		t.Error("exit 0 should not match other categories")
	}
}

func TestResultIsLegitimateFailure(t *testing.T) {
	r := &Result{ExitCode: 1}
	if !r.IsLegitimateFailure() {
		t.Error("exit 1 should be legitimate failure")
	}
	if r.IsSuccess() || r.IsPreconditionFailure() || r.IsScriptError() {
		t.Error("exit 1 should not match other categories")
	}
}

func TestResultIsPreconditionFailure(t *testing.T) {
	r := &Result{ExitCode: 2}
	if !r.IsPreconditionFailure() {
		t.Error("exit 2 should be precondition failure")
	}
	if r.IsSuccess() || r.IsLegitimateFailure() || r.IsScriptError() {
		t.Error("exit 2 should not match other categories")
	}
}

func TestResultIsScriptError(t *testing.T) {
	for _, code := range []int{3, 126, 127, 255} {
		r := &Result{ExitCode: code}
		if !r.IsScriptError() {
			t.Errorf("exit %d should be script error", code)
		}
		if r.IsSuccess() || r.IsLegitimateFailure() || r.IsPreconditionFailure() {
			t.Errorf("exit %d should not match other categories", code)
		}
	}
}

func TestCombinedOutputStdoutOnly(t *testing.T) {
	r := &Result{Stdout: "hello"}
	if r.CombinedOutput() != "hello" {
		t.Errorf("expected 'hello', got %q", r.CombinedOutput())
	}
}

func TestCombinedOutputStderrOnly(t *testing.T) {
	r := &Result{Stderr: "error"}
	if r.CombinedOutput() != "error" {
		t.Errorf("expected 'error', got %q", r.CombinedOutput())
	}
}

func TestCombinedOutputBoth(t *testing.T) {
	r := &Result{Stdout: "out", Stderr: "err"}
	expected := "out\nerr"
	if r.CombinedOutput() != expected {
		t.Errorf("expected %q, got %q", expected, r.CombinedOutput())
	}
}

func TestCombinedOutputEmpty(t *testing.T) {
	r := &Result{}
	if r.CombinedOutput() != "" {
		t.Errorf("expected empty, got %q", r.CombinedOutput())
	}
}

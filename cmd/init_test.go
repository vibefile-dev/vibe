package cmd

import (
	"strings"
	"testing"
)

func TestEmptyVibefileContent(t *testing.T) {
	content := emptyVibefileContent("my-project")

	if !strings.Contains(content, "model = claude-sonnet-4-6") {
		t.Error("expected default model declaration")
	}
	if !strings.Contains(content, "name  = my-project") {
		t.Error("expected project name variable")
	}
	if !strings.Contains(content, "# Add your targets below") {
		t.Error("expected instructional comment")
	}
	if !strings.Contains(content, "@require") {
		t.Error("expected @require example in comments")
	}
}

func TestEmptyVibefileContentSpecialChars(t *testing.T) {
	content := emptyVibefileContent("my-special_project.v2")
	if !strings.Contains(content, "name  = my-special_project.v2") {
		t.Errorf("expected project name to be preserved, got:\n%s", content)
	}
}

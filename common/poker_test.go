package common

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindRulePathFromPackageDirectory(t *testing.T) {
	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	path, err := findRulePath(workingDir)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "rule.json" {
		t.Fatalf("rule path = %q, want project rule.json", path)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("project rule file unavailable: %v", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(path), "go.mod")); err != nil {
		t.Fatalf("rule file was not resolved from project root: %v", err)
	}
}

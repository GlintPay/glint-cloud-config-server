package config

import "testing"

func TestApplyOverridesGitDefaultBranchName(t *testing.T) {
	app := ApplicationConfiguration{Git: GitConfig{DefaultBranchName: "master"}}

	Configuration{}.ApplyOverrides(&app)
	if app.Git.DefaultBranchName != "master" {
		t.Fatalf("unset env changed the branch to %q", app.Git.DefaultBranchName)
	}

	Configuration{GitDefaultBranchName: "env/alice-kyc"}.ApplyOverrides(&app)
	if app.Git.DefaultBranchName != "env/alice-kyc" {
		t.Fatalf("got %q, want env/alice-kyc", app.Git.DefaultBranchName)
	}
}

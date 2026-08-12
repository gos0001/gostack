package scaffold

import (
	"path/filepath"
	"testing"
)

func TestValidateName(t *testing.T) {
	valid := []string{"user", "get_user", "my-feature", "u", "user2", "a1_b2-c3"}
	for _, name := range valid {
		if err := ValidateName(name); err != nil {
			t.Errorf("ValidateName(%q) = %v, want nil", name, err)
		}
	}

	invalid := []string{
		"",           // empty
		"User",       // uppercase
		"2users",     // leading digit
		"_user",      // leading underscore
		"-user",      // leading dash
		"user.name",  // dot
		"user name",  // space
		"users/get",  // a path, not a name
		"user\\name", // backslash
	}
	for _, name := range invalid {
		if err := ValidateName(name); err == nil {
			t.Errorf("ValidateName(%q) = nil, want an error", name)
		}
	}
}

func TestValidateFeaturePath(t *testing.T) {
	valid := []string{"get_user", "users/get_profile", "admin/users/ban", "users/get_profile/"}
	for _, p := range valid {
		if err := ValidateFeaturePath(p); err != nil {
			t.Errorf("ValidateFeaturePath(%q) = %v, want nil", p, err)
		}
	}

	invalid := []string{
		"",              // empty
		"/",             // nothing but separators
		"users/Get",     // uppercase segment
		"users/2get",    // leading digit in a segment
		"users/get.pro", // dot in a segment
	}
	for _, p := range invalid {
		if err := ValidateFeaturePath(p); err == nil {
			t.Errorf("ValidateFeaturePath(%q) = nil, want an error", p)
		}
	}
}

func TestValidatePagePath(t *testing.T) {
	valid := []string{
		"home",
		"posts/[post_id]",
		"home/[user_id]/posts/[post_id]",
		"admin/dashboard",
		"my-page",
	}
	for _, p := range valid {
		if err := ValidatePagePath(p); err != nil {
			t.Errorf("ValidatePagePath(%q) = %v, want nil", p, err)
		}
	}

	invalid := []string{
		"",                // empty
		"posts/[Post_ID]", // uppercase inside the param
		"posts/[]",        // empty param
		"posts/[id",       // unclosed bracket
		"posts/Post",      // uppercase segment
		// A dash is legal in a plain segment but not inside a param, because
		// the param becomes a Go identifier via the handler.
		"posts/[post-id]",
	}
	for _, p := range invalid {
		if err := ValidatePagePath(p); err == nil {
			t.Errorf("ValidatePagePath(%q) = nil, want an error", p)
		}
	}
}

func TestDirMustNotExist(t *testing.T) {
	dir := t.TempDir()
	if err := DirMustNotExist(filepath.Join(dir, "nope")); err != nil {
		t.Errorf("DirMustNotExist on a missing path = %v, want nil", err)
	}
	if err := DirMustNotExist(dir); err == nil {
		t.Error("DirMustNotExist on an existing directory = nil, want an error")
	}
}

func TestOrchestratorNames(t *testing.T) {
	got := OrchestratorNames()
	want := []string{"bootstrap", "cron"}
	if len(got) != len(want) {
		t.Fatalf("OrchestratorNames() = %v, want %v", got, want)
	}
	// Sorted, so the --orchestrator help text and the error message do not
	// shuffle between runs of the same binary.
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("OrchestratorNames() = %v, want %v", got, want)
		}
	}
}

func TestValidateOrchestrator(t *testing.T) {
	for _, name := range OrchestratorNames() {
		if err := ValidateOrchestrator(name); err != nil {
			t.Errorf("ValidateOrchestrator(%q) = %v, want nil", name, err)
		}
	}
	for _, name := range []string{"", "workers", "Bootstrap", "cronjob"} {
		if err := ValidateOrchestrator(name); err == nil {
			t.Errorf("ValidateOrchestrator(%q) = nil, want an error", name)
		}
	}
}

// The two orchestrators are switched on differently — a bool for bootstrap, an
// interval for cron — and the CLI prints that line after generating. Getting it
// wrong sends the user looking for a variable that does not exist.
func TestOrchestratorEnableHint(t *testing.T) {
	cases := []struct{ orch, feature, want string }{
		{"bootstrap", "seed_super_admin", "SEED_SUPER_ADMIN_ENABLED=true"},
		{"cron", "outbox_drain", "OUTBOX_DRAIN_INTERVAL=30s"},
		{"nope", "whatever", ""},
	}
	for _, c := range cases {
		if got := OrchestratorEnableHint(c.orch, c.feature); got != c.want {
			t.Errorf("OrchestratorEnableHint(%q, %q) = %q, want %q",
				c.orch, c.feature, got, c.want)
		}
	}
}

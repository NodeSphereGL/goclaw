package security

import "testing"

func TestValidateLauncherArgsDeniesEnvCommandLaunch(t *testing.T) {
	if reason := validateLauncherArgs("env", []string{"bash", "-lc", "id"}); reason == "" {
		t.Fatal("expected env with command args to be denied")
	}
}

func TestValidateLauncherArgsAllowsPlainNonLauncherCommand(t *testing.T) {
	if reason := validateLauncherArgs("git", []string{"status"}); reason != "" {
		t.Fatalf("expected git args to be allowed, got %q", reason)
	}
}

func TestMatchAllowedBinary(t *testing.T) {
	cases := []struct {
		pattern, binary string
		want            bool
	}{
		{"**", "git", true},       // allow-all sentinel matches anything
		{"**", "rm", true},        // ...including destructive binaries (opt-in trust)
		{"*", "git", false},       // lone star still rejected
		{"git", "git", true},      // exact match
		{"git", "gitk", false},    // exact does not prefix-match
		{"python*", "python3", true},
		{"python*", "ruby", false},
		{"", "git", false}, // empty pattern never matches
	}
	for _, c := range cases {
		if got := MatchAllowedBinary(c.pattern, c.binary); got != c.want {
			t.Errorf("MatchAllowedBinary(%q, %q) = %v, want %v", c.pattern, c.binary, got, c.want)
		}
	}
}

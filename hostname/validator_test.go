package hostname

import (
	"testing"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		wantOK   bool
		wantMsg  string // substring to match in the message (empty = no message expected)
	}{
		// Valid 2-level hostnames
		{name: "simple 2-level", hostname: "whoami.local", wantOK: true},
		{name: "api 2-level", hostname: "api.local", wantOK: true},
		{name: "grafana 2-level", hostname: "grafana.local", wantOK: true},

		// Valid 3-level hostnames
		{name: "simple 3-level", hostname: "app.myserver.local", wantOK: true},
		{name: "auth 3-level", hostname: "auth.myserver.local", wantOK: true},
		{name: "dashboard 3-level", hostname: "dashboard.home.local", wantOK: true},

		// Invalid: not .local
		{name: "dot com", hostname: "example.com", wantOK: false, wantMsg: "does not end with .local"},
		{name: "dot net", hostname: "myapp.net", wantOK: false, wantMsg: "does not end with .local"},

		// Invalid: bare .local
		{name: "bare .local", hostname: ".local", wantOK: false, wantMsg: "bare .local"},

		// Invalid: too many levels (4+)
		{name: "4-level", hostname: "a.b.c.local", wantOK: false, wantMsg: "4 levels"},
		{name: "5-level", hostname: "a.b.c.d.local", wantOK: false, wantMsg: "5 levels"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, msg := Validate(tt.hostname)
			if ok != tt.wantOK {
				t.Errorf("Validate(%q) ok = %v, want %v (msg: %s)", tt.hostname, ok, tt.wantOK, msg)
			}
			if tt.wantOK && msg != "" {
				t.Errorf("Validate(%q) returned ok=true but non-empty message: %s", tt.hostname, msg)
			}
			if !tt.wantOK && tt.wantMsg != "" {
				if !contains(msg, tt.wantMsg) {
					t.Errorf("Validate(%q) message = %q, want substring %q", tt.hostname, msg, tt.wantMsg)
				}
			}
		})
	}
}

func TestExtract(t *testing.T) {
	input := []string{
		"whoami.local",        // valid 2-level
		"app.myserver.local",  // valid 3-level
		"example.com",         // invalid: not .local
		"a.b.c.local",         // invalid: 4 levels
		"grafana.local",       // valid 2-level
		"auth.myserver.local", // valid 3-level
	}

	valid, warnings := Extract(input)

	expectedValid := []string{"whoami.local", "app.myserver.local", "grafana.local", "auth.myserver.local"}
	if len(valid) != len(expectedValid) {
		t.Fatalf("Extract() returned %d valid, want %d", len(valid), len(expectedValid))
	}
	for i, v := range valid {
		if v != expectedValid[i] {
			t.Errorf("valid[%d] = %q, want %q", i, v, expectedValid[i])
		}
	}

	if len(warnings) != 2 {
		t.Fatalf("Extract() returned %d warnings, want 2", len(warnings))
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

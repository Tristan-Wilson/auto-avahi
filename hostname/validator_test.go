package hostname

import (
	"testing"
)

var (
	mdnsLocal = Options{Suffixes: []string{"local"}, EnforceDepthLimit: true}
	dnsCom    = Options{Suffixes: []string{"example.com"}, EnforceDepthLimit: false}
	multi     = Options{Suffixes: []string{"local", "example.com"}, EnforceDepthLimit: false}
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		opts     Options
		wantOK   bool
		wantMsg  string // substring to match in the message (empty = no message expected)
	}{
		// mDNS mode: 2-level .local hostnames
		{name: "mdns/2-level whoami", hostname: "whoami.local", opts: mdnsLocal, wantOK: true},
		{name: "mdns/2-level api", hostname: "api.local", opts: mdnsLocal, wantOK: true},

		// mDNS mode: 3-level .local hostnames
		{name: "mdns/3-level app", hostname: "app.myserver.local", opts: mdnsLocal, wantOK: true},
		{name: "mdns/3-level dashboard", hostname: "dashboard.home.local", opts: mdnsLocal, wantOK: true},

		// mDNS mode: rejects non-.local
		{name: "mdns/dot com", hostname: "example.com", opts: mdnsLocal, wantOK: false, wantMsg: "does not end with any configured suffix"},
		{name: "mdns/dot net", hostname: "myapp.net", opts: mdnsLocal, wantOK: false, wantMsg: "does not end with any configured suffix"},

		// mDNS mode: rejects bare suffix
		{name: "mdns/bare local", hostname: "local", opts: mdnsLocal, wantOK: false, wantMsg: "bare"},

		// mDNS mode: rejects too many levels (4+)
		{name: "mdns/4-level", hostname: "a.b.c.local", opts: mdnsLocal, wantOK: false, wantMsg: "labels before suffix"},
		{name: "mdns/5-level", hostname: "a.b.c.d.local", opts: mdnsLocal, wantOK: false, wantMsg: "labels before suffix"},

		// Non-mDNS mode: any depth allowed under the configured suffix
		{name: "dns/2-level", hostname: "auth.example.com", opts: dnsCom, wantOK: true},
		{name: "dns/3-level", hostname: "app.team.example.com", opts: dnsCom, wantOK: true},
		{name: "dns/4-level allowed", hostname: "a.b.c.example.com", opts: dnsCom, wantOK: true},
		{name: "dns/5-level allowed", hostname: "a.b.c.d.example.com", opts: dnsCom, wantOK: true},

		// Non-mDNS mode: still rejects wrong suffix
		{name: "dns/wrong suffix", hostname: "auth.other.org", opts: dnsCom, wantOK: false, wantMsg: "does not end with any configured suffix"},

		// Non-mDNS mode: still rejects bare suffix
		{name: "dns/bare suffix", hostname: "example.com", opts: dnsCom, wantOK: false, wantMsg: "bare"},

		// Multi-suffix: accepts hostnames matching either suffix
		{name: "multi/matches local", hostname: "whoami.local", opts: multi, wantOK: true},
		{name: "multi/matches com", hostname: "auth.example.com", opts: multi, wantOK: true},
		{name: "multi/matches neither", hostname: "auth.other.org", opts: multi, wantOK: false, wantMsg: "does not end with any configured suffix"},

		// Suffix-substring guard: "notexample.com" should not match suffix "example.com"
		// (must be preceded by a dot or be the bare suffix).
		{name: "dns/suffix substring guard", hostname: "auth.notexample.com", opts: dnsCom, wantOK: false, wantMsg: "does not end with any configured suffix"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, msg := Validate(tt.hostname, tt.opts)
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

func TestExtract_mDNS(t *testing.T) {
	input := []string{
		"whoami.local",        // valid 2-level
		"app.myserver.local",  // valid 3-level
		"example.com",         // invalid: not in suffixes
		"a.b.c.local",         // invalid: 4 levels in mDNS mode
		"grafana.local",       // valid 2-level
		"auth.myserver.local", // valid 3-level
	}

	valid, warnings := Extract(input, mdnsLocal)

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

func TestExtract_DNS(t *testing.T) {
	input := []string{
		"auth.example.com",         // valid
		"app.team.example.com",     // valid (deeper allowed without depth limit)
		"a.b.c.d.example.com",      // valid (any depth allowed)
		"whoami.local",             // invalid: not in suffixes
		"auth.notexample.com",      // invalid: suffix substring guard
	}

	valid, warnings := Extract(input, dnsCom)

	expectedValid := []string{"auth.example.com", "app.team.example.com", "a.b.c.d.example.com"}
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

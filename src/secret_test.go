package main

import "testing"

func TestMatchesAttrs(t *testing.T) {
	tests := []struct {
		name  string
		attrs map[string]string
		want  bool
	}{
		{"exact", map[string]string{"service": "gitlab", "host": "gitlab.com"}, true},
		{"with schema", map[string]string{"service": "gitlab", "host": "gitlab.com", "xdg:schema": "org.freedesktop.Secret.Generic"}, true},
		{"extra attribute", map[string]string{"service": "gitlab", "host": "gitlab.com", "user": "max"}, false},
		{"other host", map[string]string{"service": "gitlab", "host": "example.com"}, false},
		{"other service", map[string]string{"service": "github", "host": "gitlab.com"}, false},
		{"missing host", map[string]string{"service": "gitlab", "xdg:schema": "x"}, false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		if got := matchesAttrs(tt.attrs, "gitlab.com"); got != tt.want {
			t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
		}
	}
}

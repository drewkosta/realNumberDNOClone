package service

import "testing"

func TestNormalizePhone(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"5551234567", "5551234567"},
		{"(555) 123-4567", "5551234567"},
		{"+15551234567", "5551234567"},
		{"15551234567", "5551234567"},
		{"555-123-4567", "5551234567"},
		{"+1 (555) 123-4567", "5551234567"},
		{"  555 123 4567  ", "5551234567"},
	}

	for _, tt := range tests {
		got := normalizePhone(tt.input)
		if got != tt.want {
			t.Errorf("normalizePhone(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestPhoneRegex(t *testing.T) {
	valid := []string{"5551234567", "8001234567", "2125559999"}
	for _, p := range valid {
		if !phoneRegex.MatchString(p) {
			t.Errorf("expected %q to match phone regex", p)
		}
	}

	invalid := []string{"555", "123456789", "12345678901", "abcdefghij", ""}
	for _, p := range invalid {
		if phoneRegex.MatchString(p) {
			t.Errorf("expected %q to NOT match phone regex", p)
		}
	}
}

func TestDnoCacheKey(t *testing.T) {
	key := dnoCacheKey("5551234567", "voice")
	if key != "5551234567:voice" {
		t.Errorf("dnoCacheKey = %q, want 5551234567:voice", key)
	}
}

package models

import "testing"

func TestValidateChannel(t *testing.T) {
	for _, valid := range []string{"voice", "text", "both"} {
		if err := ValidateChannel(valid); err != nil {
			t.Errorf("ValidateChannel(%q) = %v, want nil", valid, err)
		}
	}
	for _, invalid := range []string{"", "fax", "sms", "VOICE"} {
		if err := ValidateChannel(invalid); err == nil {
			t.Errorf("ValidateChannel(%q) = nil, want error", invalid)
		}
	}
}

func TestValidateDataset(t *testing.T) {
	for _, valid := range []string{"auto", "subscriber", "itg", "tss_registry"} {
		if err := ValidateDataset(valid); err != nil {
			t.Errorf("ValidateDataset(%q) = %v, want nil", valid, err)
		}
	}
	if err := ValidateDataset("unknown"); err == nil {
		t.Error("ValidateDataset(unknown) = nil, want error")
	}
}

func TestValidateStatus(t *testing.T) {
	for _, valid := range []string{"active", "inactive", "pending"} {
		if err := ValidateStatus(valid); err != nil {
			t.Errorf("ValidateStatus(%q) = %v, want nil", valid, err)
		}
	}
	if err := ValidateStatus("deleted"); err == nil {
		t.Error("ValidateStatus(deleted) = nil, want error")
	}
}

func TestValidateNumberType(t *testing.T) {
	if err := ValidateNumberType("toll_free"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := ValidateNumberType("local"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := ValidateNumberType("mobile"); err == nil {
		t.Error("expected error for mobile")
	}
}

func TestValidateRole(t *testing.T) {
	for _, valid := range []string{"admin", "org_admin", "operator", "viewer"} {
		if err := ValidateRole(valid); err != nil {
			t.Errorf("ValidateRole(%q) = %v, want nil", valid, err)
		}
	}
	if err := ValidateRole("superadmin"); err == nil {
		t.Error("expected error for superadmin")
	}
}

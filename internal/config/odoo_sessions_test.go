package config

import (
	"testing"
)

func TestNormalizeOdooURL(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    string
		expectError bool
	}{
		{
			name:     "standard HTTPS url with trailing slash",
			input:    "https://mycompany.odoo.com/",
			expected: "https://mycompany.odoo.com",
		},
		{
			name:     "domain without scheme defaults to HTTPS",
			input:    "mycompany.odoo.com",
			expected: "https://mycompany.odoo.com",
		},
		{
			name:     "url with path and hash",
			input:    "https://MYCOMPANY.odoo.com/web#cids=1",
			expected: "https://mycompany.odoo.com",
		},
		{
			name:     "localhost with port over HTTP is allowed",
			input:    "localhost:8069",
			expected: "http://localhost:8069",
		},
		{
			name:     "127.0.0.1 over HTTP is allowed",
			input:    "http://127.0.0.1:8069/web",
			expected: "http://127.0.0.1:8069",
		},
		{
			name:     "private IP 192.168.1.50 over HTTP is allowed",
			input:    "http://192.168.1.50:8069/",
			expected: "http://192.168.1.50:8069",
		},
		{
			name:        "public domain over insecure HTTP is rejected",
			input:       "http://mycompany.odoo.com",
			expectError: true,
		},
		{
			name:        "empty URL is rejected",
			input:       "   ",
			expectError: true,
		},
		{
			name:        "invalid URL scheme is rejected",
			input:       "ftp://mycompany.odoo.com",
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := NormalizeOdooURL(tc.input)
			if tc.expectError {
				if err == nil {
					t.Fatalf("expected error for input %q, got none", tc.input)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error for input %q: %v", tc.input, err)
				}
				if result != tc.expected {
					t.Fatalf("expected %q, got %q", tc.expected, result)
				}
			}
		})
	}
}

func TestOdooInstancesManagement(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cm, err := NewManager()
	if err != nil {
		t.Fatalf("failed to create config manager: %v", err)
	}

	// 1. Initial list should be empty
	instances := cm.GetOdooInstances()
	if len(instances) != 0 {
		t.Fatalf("expected empty instances, got %d", len(instances))
	}

	// 2. Add first instance
	inst1, err := cm.AddOdooInstance("https://odoo1.example.com", "Odoo Production")
	if err != nil {
		t.Fatalf("failed to add inst1: %v", err)
	}
	if inst1.URL != "https://odoo1.example.com" || inst1.Name != "Odoo Production" {
		t.Fatalf("unexpected inst1 values: %+v", inst1)
	}

	// 3. Duplicate URL should be rejected
	_, err = cm.AddOdooInstance("https://odoo1.example.com/web#cids=1", "Duplicate")
	if err == nil {
		t.Fatalf("expected error adding duplicate URL, got nil")
	}

	// 4. Add second instance
	inst2, err := cm.AddOdooInstance("https://odoo2.example.com", "Odoo Staging")
	if err != nil {
		t.Fatalf("failed to add inst2: %v", err)
	}

	instances = cm.GetOdooInstances()
	if len(instances) != 2 {
		t.Fatalf("expected 2 instances, got %d", len(instances))
	}

	// 5. Save pairing for inst1
	err = cm.SaveOdooInstancePairing(inst1.ID, "tok-inst1", 42)
	if err != nil {
		t.Fatalf("failed to save pairing: %v", err)
	}

	active := cm.GetActiveOdooInstance()
	if active == nil || active.ID != inst1.ID || !active.IsActive || active.Token != "tok-inst1" || active.OboxID != 42 {
		t.Fatalf("expected active instance to be inst1 paired, got %+v", active)
	}

	// Verify legacy config is also in sync
	odooCfg := cm.GetOdooConfig()
	if odooCfg.DBURL != inst1.URL || odooCfg.Token != "tok-inst1" {
		t.Fatalf("expected legacy odoo config synced, got %+v", odooCfg)
	}

	// 6. Save pairing for inst2
	err = cm.SaveOdooInstancePairing(inst2.ID, "tok-inst2", 99)
	if err != nil {
		t.Fatalf("failed to save pairing for inst2: %v", err)
	}
	active = cm.GetActiveOdooInstance()
	if active == nil || active.ID != inst2.ID || active.Token != "tok-inst2" {
		t.Fatalf("expected active instance to be inst2, got %+v", active)
	}

	// 7. Switch active back to inst1
	err = cm.SetActiveOdooInstance(inst1.ID)
	if err != nil {
		t.Fatalf("failed to set active instance: %v", err)
	}
	active = cm.GetActiveOdooInstance()
	if active == nil || active.ID != inst1.ID {
		t.Fatalf("expected active instance to be inst1, got %+v", active)
	}
	if cm.GetOdooConfig().Token != "tok-inst1" {
		t.Fatalf("expected legacy token tok-inst1, got %s", cm.GetOdooConfig().Token)
	}

	// 8. Remove active instance
	err = cm.RemoveOdooInstance(inst1.ID)
	if err != nil {
		t.Fatalf("failed to remove inst1: %v", err)
	}
	if len(cm.GetOdooInstances()) != 1 {
		t.Fatalf("expected 1 instance left, got %d", len(cm.GetOdooInstances()))
	}
	if cm.HasOdooCredentials() {
		t.Fatalf("expected credentials cleared after removing active instance")
	}

	// 9. Persistence across re-initialization
	cm2, err := NewManager()
	if err != nil {
		t.Fatalf("failed to create cm2: %v", err)
	}
	if err := cm2.Load(); err != nil {
		t.Fatalf("failed to load cm2: %v", err)
	}
	instances2 := cm2.GetOdooInstances()
	if len(instances2) != 1 || instances2[0].ID != inst2.ID {
		t.Fatalf("expected persisted instance in cm2, got %+v", instances2)
	}
}

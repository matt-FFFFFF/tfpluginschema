package tfpluginschema

import (
	"testing"

	goversion "github.com/hashicorp/go-version"
)

// helper to build versions slice already sorted
func mustVersions(t *testing.T, vs ...string) goversion.Collection {
	t.Helper()
	out := make(goversion.Collection, 0, len(vs))
	for _, s := range vs {
		v, err := goversion.NewVersion(s)
		if err != nil {
			t.Fatalf("parse version %s: %v", s, err)
		}
		out = append(out, v)
	}
	// rely on caller to pass in sorted order; tests for unsorted case craft unsorted manually
	return out
}

func mustConstraints(t *testing.T, c string) goversion.Constraints {
	t.Helper()
	cons, err := goversion.NewConstraint(c)
	if err != nil {
		t.Fatalf("constraint %s: %v", c, err)
	}
	return cons
}

func TestGetLatestVersionMatch(t *testing.T) {
	tests := []struct {
		name        string
		versions    goversion.Collection
		constraints goversion.Constraints
		want        string
		wantErr     bool
	}{
		{
			name:        "empty versions",
			versions:    goversion.Collection{},
			constraints: mustConstraints(t, ">= 1.0.0"),
			wantErr:     true,
		},
		{
			name:        "nil constraints",
			versions:    mustVersions(t, "1.0.0", "1.1.0"),
			constraints: nil,
			want:        "1.1.0",
		},
		{
			name:        "empty constraints slice returns latest version",
			versions:    mustVersions(t, "1.0.0", "1.2.0", "2.0.0"),
			constraints: goversion.Constraints{},
			want:        "2.0.0",
		},
		{
			name:        "unsorted versions returns error",
			versions:    func() goversion.Collection { v := mustVersions(t, "1.0.0", "0.9.0"); return v }(),
			constraints: mustConstraints(t, ">= 0.1.0"),
			wantErr:     true,
		},
		{
			name:        "no matching version",
			versions:    mustVersions(t, "1.0.0", "1.1.0", "1.2.0"),
			constraints: mustConstraints(t, "< 1.0.0"),
			wantErr:     true,
		},
		{
			name:        "range picks highest match",
			versions:    mustVersions(t, "0.9.0", "1.0.0", "1.5.0", "1.9.9", "2.0.0"),
			constraints: mustConstraints(t, ">= 1.0.0, < 2.0.0"),
			want:        "1.9.9",
		},
		{
			name:        "constraint matches last (latest) version",
			versions:    mustVersions(t, "1.0.0", "1.1.0", "1.2.0"),
			constraints: mustConstraints(t, ">= 1.2.0"),
			want:        "1.2.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := GetLatestVersionMatch(tt.versions, tt.constraints)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got none (result=%v)", v)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if v == nil {
				t.Fatalf("expected version %s, got nil", tt.want)
			}
			if got := v.String(); got != tt.want {
				t.Fatalf("expected %s, got %s", tt.want, got)
			}
		})
	}
}

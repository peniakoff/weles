package sesiam_test

import (
	"strings"
	"testing"

	"github.com/peniakoff/weles/internal/sesiam"
)

func TestNormalizeIdentity(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", ""},
		{"  example.com  ", "example.com"},
		{"noreply@example.com", "noreply@example.com"},
		{"arn:aws:ses:eu-central-1:123456789012:identity/example.com", "example.com"},
		{"arn:aws-cn:ses:cn-north-1:123456789012:identity/noreply@example.com", "noreply@example.com"},
	}
	for _, tc := range tests {
		if got := sesiam.NormalizeIdentity(tc.in); got != tc.want {
			t.Fatalf("NormalizeIdentity(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestSplitIdentities(t *testing.T) {
	got := sesiam.SplitIdentities(" example.com , other.example.com,,arn:aws:ses:eu-central-1:1:identity/third.example.com ")
	want := []string{"example.com", "other.example.com", "third.example.com"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestCoversFromEmail(t *testing.T) {
	ids := []string{"example.com", "noreply@other.example.com"}
	tests := []struct {
		from string
		ok   bool
	}{
		{"ops@example.com", true},
		{"noreply@example.com", true},
		{"noreply@other.example.com", true},
		{"user@missing.example.com", false},
		{"", false},
		{"not-an-email", false},
	}
	for _, tc := range tests {
		if got := sesiam.CoversFromEmail(ids, tc.from); got != tc.ok {
			t.Fatalf("CoversFromEmail(%q)=%v want %v", tc.from, got, tc.ok)
		}
	}
}

func TestBuildARNs(t *testing.T) {
	arns, err := sesiam.BuildARNs("eu-central-1", "123456789012", []string{
		"example.com",
		"arn:aws:ses:eu-central-1:123456789012:identity/other.example.com",
		"example.com", // duplicate
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(arns) != 2 {
		t.Fatalf("len=%d want 2: %#v", len(arns), arns)
	}
	if arns[0] != "arn:aws:ses:eu-central-1:123456789012:identity/example.com" {
		t.Fatalf("arns[0]=%q", arns[0])
	}
	if arns[1] != "arn:aws:ses:eu-central-1:123456789012:identity/other.example.com" {
		t.Fatalf("arns[1]=%q", arns[1])
	}
}

func TestBuildARNs_Empty(t *testing.T) {
	if _, err := sesiam.BuildARNs("eu-central-1", "1", nil); err == nil {
		t.Fatal("expected error")
	}
}

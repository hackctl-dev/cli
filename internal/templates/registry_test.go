package templates

import (
	"reflect"
	"testing"
)

func TestSupportedIDs(t *testing.T) {
	expected := []string{"mern", "next-supabase", "nuxt-supabase", "pern", "sveltekit-supabase"}
	if got := SupportedIDs(); !reflect.DeepEqual(got, expected) {
		t.Fatalf("unexpected supported IDs: %v", got)
	}
}

func TestResolveKnownTemplate(t *testing.T) {
	resolved, err := Resolve("next-supabase")
	if err != nil {
		t.Fatalf("unexpected resolve error: %v", err)
	}

	if resolved.Subdir != "next-supabase" {
		t.Fatalf("unexpected template subdir: %s", resolved.Subdir)
	}

	if !IsOfficial("next-supabase") {
		t.Fatalf("expected template to be official")
	}
}

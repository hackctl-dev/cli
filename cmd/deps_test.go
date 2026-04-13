package cmd

import "testing"

func TestParseMajorVersion(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  int
		expectErr bool
	}{
		{name: "node", input: "v20.12.2\n", expected: 20},
		{name: "npm", input: "10.9.1", expected: 10},
		{name: "git", input: "git version 2.48.0.windows.1", expected: 2},
		{name: "empty", input: "", expectErr: true},
		{name: "no numbers", input: "version", expectErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			major, err := parseMajorVersion(tc.input)
			if tc.expectErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if major != tc.expected {
				t.Fatalf("expected major %d, got %d", tc.expected, major)
			}
		})
	}
}

func TestEnsureDependenciesMissingMessage(t *testing.T) {
	dep := requiredDependency{
		name: "hackctl_missing_dependency_for_test",
		url:  "https://example.com",
	}

	err := ensureDependencies(dep)
	if err == nil {
		t.Fatalf("expected missing dependency error")
	}

	expected := "Missing dependency: hackctl_missing_dependency_for_test (https://example.com)"
	if err.Error() != expected {
		t.Fatalf("unexpected error: %q", err.Error())
	}
}

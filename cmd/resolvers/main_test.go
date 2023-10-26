package main

import "testing"

func TestBuildHubURL(t *testing.T) {
	testCases := []struct {
		name       string
		configAPI  string
		defaultURL string
		expected   string
	}{
		{
			name:       "configAPI empty",
			configAPI:  "",
			defaultURL: "https://tekton.dev",
			expected:   "https://tekton.dev",
		},
		{
			name:       "configAPI not empty",
			configAPI:  "https://myhub.com",
			defaultURL: "https://foo.com",
			expected:   "https://myhub.com",
		},
		{
			name:       "defaultURL ends with slash",
			configAPI:  "",
			defaultURL: "https://bar.com/",
			expected:   "https://bar.com",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := buildHubURL(tc.configAPI, tc.defaultURL)
			if actual != tc.expected {
				t.Errorf("expected %s, but got %s", tc.expected, actual)
			}
		})
	}
}

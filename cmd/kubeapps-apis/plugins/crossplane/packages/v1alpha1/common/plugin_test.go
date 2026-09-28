// Copyright 2021-2023 the Kubeapps contributors.
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParsePluginConfig(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows")
	}

	t.Run("valid custom config", func(t *testing.T) {
		f, err := os.CreateTemp(".", "crossplane-plugin-config")
		if err != nil {
			t.Fatalf("CreateTemp: %v", err)
		}
		defer os.Remove(f.Name())
		if _, err := f.WriteString(`{
			"crossplane": {
				"packages": {
					"v1alpha1": {
						"deployPreference": "composite",
						"labelSelector": "catalog=true",
						"nameAllowlist": ["xwidgets.example.org"]
					}
				}
			}
		}`); err != nil {
			t.Fatalf("write config: %v", err)
		}
		if err := f.Close(); err != nil {
			t.Fatalf("close config: %v", err)
		}

		config, err := ParsePluginConfig(f.Name())
		if err != nil {
			t.Fatalf("ParsePluginConfig: %v", err)
		}
		want := &CrossplanePluginConfig{
			DeployPreference: DeployPreferenceComposite,
			LabelSelector:    "catalog=true",
			NameAllowlist:    []string{"xwidgets.example.org"},
		}
		if !cmp.Equal(want, config) {
			t.Fatalf("mismatch (-want +got):\n%s", cmp.Diff(want, config))
		}
	})

	t.Run("invalid deploy preference", func(t *testing.T) {
		f, err := os.CreateTemp(".", "crossplane-plugin-config-invalid")
		if err != nil {
			t.Fatalf("CreateTemp: %v", err)
		}
		defer os.Remove(f.Name())
		if _, err := f.WriteString(`{"crossplane":{"packages":{"v1alpha1":{"deployPreference":"invalid"}}}}`); err != nil {
			t.Fatalf("write config: %v", err)
		}
		if err := f.Close(); err != nil {
			t.Fatalf("close config: %v", err)
		}

		_, err = ParsePluginConfig(f.Name())
		if err == nil || !strings.Contains(err.Error(), "invalid deployPreference") {
			t.Fatalf("expected invalid deployPreference error, got %v", err)
		}
	})
}

func TestNewDefaultPluginConfig(t *testing.T) {
	config := NewDefaultPluginConfig()
	if config.DeployPreference != DeployPreferenceClaim {
		t.Fatalf("unexpected default deploy preference: %q", config.DeployPreference)
	}
}

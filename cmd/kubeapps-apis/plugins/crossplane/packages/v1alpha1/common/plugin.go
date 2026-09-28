// Copyright 2021-2023 the Kubeapps contributors.
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"encoding/json"
	"fmt"
	"os"

)

// DeployPreference controls whether claims or composites are deployed by default.
type DeployPreference string

const (
	DeployPreferenceClaim     DeployPreference = "claim"
	DeployPreferenceComposite DeployPreference = "composite"
)

// CrossplanePluginConfig holds plugin-specific configuration.
type CrossplanePluginConfig struct {
	// DeployPreference selects claim vs composite when an XRD defines claim names.
	DeployPreference DeployPreference
	// LabelSelector filters XRDs by metadata labels (optional).
	LabelSelector string
	// NameAllowlist limits catalog to these XRD names (optional).
	NameAllowlist []string
}

// NewDefaultPluginConfig returns the default Crossplane plugin configuration.
func NewDefaultPluginConfig() *CrossplanePluginConfig {
	return &CrossplanePluginConfig{
		DeployPreference: DeployPreferenceClaim,
	}
}

// ParsePluginConfig parses the kubeapps-apis plugin configuration JSON file.
func ParsePluginConfig(pluginConfigPath string) (*CrossplanePluginConfig, error) {
	type internalCrossplanePluginConfig struct {
		Crossplane struct {
			Packages struct {
				V1alpha1 struct {
					DeployPreference string   `json:"deployPreference"`
					LabelSelector    string   `json:"labelSelector"`
					NameAllowlist    []string `json:"nameAllowlist"`
				} `json:"v1alpha1"`
			} `json:"packages"`
		} `json:"crossplane"`
	}

	var parsed internalCrossplanePluginConfig
	// #nosec G304
	data, err := os.ReadFile(pluginConfigPath)
	if err != nil {
		return nil, fmt.Errorf("unable to open plugin config at %q: %w", pluginConfigPath, err)
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("unable to unmarshal plugin config: %w", err)
	}

	config := NewDefaultPluginConfig()
	if pref := parsed.Crossplane.Packages.V1alpha1.DeployPreference; pref != "" {
		switch DeployPreference(pref) {
		case DeployPreferenceClaim, DeployPreferenceComposite:
			config.DeployPreference = DeployPreference(pref)
		default:
			return nil, fmt.Errorf("invalid deployPreference: %q", pref)
		}
	}
	config.LabelSelector = parsed.Crossplane.Packages.V1alpha1.LabelSelector
	config.NameAllowlist = parsed.Crossplane.Packages.V1alpha1.NameAllowlist

	return config, nil
}

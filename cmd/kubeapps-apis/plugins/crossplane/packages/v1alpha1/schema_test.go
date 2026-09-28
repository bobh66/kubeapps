// Copyright 2021-2023 the Kubeapps contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"strings"
	"testing"

	apiextensionsv1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1"
	"github.com/SAP/kubeapps/cmd/kubeapps-apis/plugins/crossplane/packages/v1alpha1/common"
)

func TestExtractFormSchemaFromSampleXRD(t *testing.T) {
	xrd := loadSampleXRD(t)
	version, err := defaultServedVersion(xrd)
	if err != nil {
		t.Fatalf("defaultServedVersion: %v", err)
	}

	valuesSchema, defaultValues, err := extractFormSchema(xrd, version)
	if err != nil {
		t.Fatalf("extractFormSchema: %v", err)
	}

	var schema map[string]interface{}
	if err := json.Unmarshal([]byte(valuesSchema), &schema); err != nil {
		t.Fatalf("values schema is not valid JSON: %v", err)
	}

	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected properties in values schema, got %#v", schema)
	}
	for _, key := range []string{"replicas", "enabled", "mode", "config"} {
		if _, ok := properties[key]; !ok {
			t.Fatalf("expected property %q in values schema", key)
		}
	}
	if _, ok := properties["x-kubernetes-preserve-unknown-fields"]; ok {
		t.Fatalf("kubernetes extension fields should be stripped from values schema")
	}

	if !strings.Contains(defaultValues, "replicas: 2") {
		t.Fatalf("expected default-spec annotation values, got %q", defaultValues)
	}
	if !strings.Contains(defaultValues, "enabled: true") {
		t.Fatalf("expected default-spec annotation values, got %q", defaultValues)
	}
}

func TestExtractFormSchemaDefaultsFromOpenAPI(t *testing.T) {
	xrd := establishedXRD("defaults.example.org")
	version := versionWithSchema(`{
		"type": "object",
		"properties": {
			"spec": {
				"type": "object",
				"properties": {
					"replicas": {"type": "integer", "default": 3},
					"config": {
						"type": "object",
						"properties": {
							"size": {"type": "string", "default": "large"}
						}
					}
				}
			}
		}
	}`)

	_, defaultValues, err := extractFormSchema(xrd, version)
	if err != nil {
		t.Fatalf("extractFormSchema: %v", err)
	}
	if !strings.Contains(defaultValues, "replicas: 3") {
		t.Fatalf("expected schema default for replicas, got %q", defaultValues)
	}
	if !strings.Contains(defaultValues, "size: large") {
		t.Fatalf("expected nested schema default, got %q", defaultValues)
	}
}

func TestExtractFormSchemaEmptySpec(t *testing.T) {
	xrd := establishedXRD("empty.example.org")
	version := apiextensionsv1.CompositeResourceDefinitionVersion{
		Name:          "v1alpha1",
		Served:        true,
		Referenceable: true,
	}

	valuesSchema, defaultValues, err := extractFormSchema(xrd, version)
	if err != nil {
		t.Fatalf("extractFormSchema: %v", err)
	}

	var schema map[string]interface{}
	if err := json.Unmarshal([]byte(valuesSchema), &schema); err != nil {
		t.Fatalf("values schema is not valid JSON: %v", err)
	}
	if properties, ok := schema["properties"].(map[string]interface{}); !ok || len(properties) != 0 {
		t.Fatalf("expected empty properties for missing spec schema, got %#v", schema)
	}
	if strings.TrimSpace(defaultValues) != "{}" {
		t.Fatalf("expected empty default values, got %q", defaultValues)
	}
}

func TestDefaultValuesForXRDInvalidAnnotation(t *testing.T) {
	xrd := establishedXRD("invalid-default.example.org")
	xrd.Annotations = map[string]string{
		common.AnnotationDefaultSpec: "not: valid: yaml: [[[",
	}
	_, err := defaultValuesForXRD(xrd, map[string]interface{}{"type": "object"})
	if err == nil {
		t.Fatal("expected error for invalid default-spec annotation")
	}
}

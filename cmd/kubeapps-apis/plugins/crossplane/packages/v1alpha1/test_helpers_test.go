// Copyright 2021-2023 the Kubeapps contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	goruntime "runtime"
	"testing"

	apiextensionsv1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	corek8sv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

func loadSampleXRD(t *testing.T) *apiextensionsv1.CompositeResourceDefinition {
	t.Helper()
	_, filename, _, ok := goruntime.Caller(0)
	if !ok {
		t.Fatal("failed to determine test file path")
	}
	path := filepath.Join(filepath.Dir(filename), "testdata", "sample-xrd.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read sample XRD: %v", err)
	}
	xrd := &apiextensionsv1.CompositeResourceDefinition{}
	if err := yaml.Unmarshal(data, xrd); err != nil {
		t.Fatalf("failed to parse sample XRD: %v", err)
	}
	return xrd
}

func establishedXRD(name string) *apiextensionsv1.CompositeResourceDefinition {
	scope := apiextensionsv1.CompositeResourceScopeLegacyCluster
	return &apiextensionsv1.CompositeResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: apiextensionsv1.CompositeResourceDefinitionSpec{
			Group: "example.crossplane.io",
			Scope: &scope,
			Names: extv1.CustomResourceDefinitionNames{
				Kind:   "XWidget",
				Plural: "xwidgets",
			},
			ClaimNames: &extv1.CustomResourceDefinitionNames{
				Kind:   "XWidgetClaim",
				Plural: "xwidgetclaims",
			},
			Versions: []apiextensionsv1.CompositeResourceDefinitionVersion{
				{
					Name:          "v1alpha1",
					Served:        true,
					Referenceable: true,
				},
			},
		},
		Status: apiextensionsv1.CompositeResourceDefinitionStatus{
			ConditionedStatus: xpv2.ConditionedStatus{
				Conditions: []xpv2.Condition{
					{
						Type:   apiextensionsv1.TypeEstablished,
						Status: corek8sv1.ConditionTrue,
					},
				},
			},
		},
	}
}

func versionWithSchema(schemaJSON string) apiextensionsv1.CompositeResourceDefinitionVersion {
	return apiextensionsv1.CompositeResourceDefinitionVersion{
		Name:          "v1alpha1",
		Served:        true,
		Referenceable: true,
		Schema: &apiextensionsv1.CompositeResourceValidation{
			OpenAPIV3Schema: k8sruntime.RawExtension{Raw: []byte(schemaJSON)},
		},
	}
}

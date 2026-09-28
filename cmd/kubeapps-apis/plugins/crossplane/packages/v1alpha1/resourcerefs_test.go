// Copyright 2021-2023 the Kubeapps contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"

	cpresource "github.com/crossplane/cli/v2/cmd/crossplane/common/resource"
	corev1 "github.com/SAP/kubeapps/cmd/kubeapps-apis/gen/core/packages/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestFlattenResourceTreeToRefs(t *testing.T) {
	root := &cpresource.Resource{
		Unstructured: unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": "example.crossplane.io/v1alpha1",
			"kind":       "XWidgetClaim",
			"metadata": map[string]interface{}{
				"name":      "root",
				"namespace": "default",
			},
		}},
		Children: []*cpresource.Resource{
			{
				Unstructured: unstructured.Unstructured{Object: map[string]interface{}{
					"apiVersion": "example.crossplane.io/v1alpha1",
					"kind":       "XWidget",
					"metadata": map[string]interface{}{
						"name": "composite",
					},
				}},
				Children: []*cpresource.Resource{
					{
						Unstructured: unstructured.Unstructured{Object: map[string]interface{}{
							"apiVersion": "ec2.aws.upbound.io/v1beta1",
							"kind":       "Instance",
							"metadata": map[string]interface{}{
								"name":      "managed",
								"namespace": "crossplane-system",
							},
						}},
					},
				},
			},
		},
	}

	refs := flattenResourceTreeToRefs(root)
	if len(refs) != 3 {
		t.Fatalf("expected 3 resource refs, got %d", len(refs))
	}

	sortResourceRefs(refs)
	if refs[0].Kind != "XWidget" || refs[2].Kind != "XWidgetClaim" {
		t.Fatalf("unexpected sort order: %#v", refs)
	}

	dup := flattenResourceTreeToRefs(root)
	if len(dup) != 3 {
		t.Fatalf("expected deduplicated refs, got %d", len(dup))
	}
}

func TestResourceRefFromCrossplaneResourceSkipsInvalid(t *testing.T) {
	if resourceRefFromCrossplaneResource(nil) != nil {
		t.Fatal("nil node should not produce a ref")
	}
	node := &cpresource.Resource{
		Unstructured: unstructured.Unstructured{Object: map[string]interface{}{
			"metadata": map[string]interface{}{"name": "missing-kind"},
		}},
	}
	if resourceRefFromCrossplaneResource(node) != nil {
		t.Fatal("node without kind should not produce a ref")
	}
}

func TestSortResourceRefsStable(t *testing.T) {
	refs := []*corev1.ResourceRef{
		{ApiVersion: "v1", Kind: "Secret", Name: "b", Namespace: "ns"},
		{ApiVersion: "v1", Kind: "ConfigMap", Name: "a", Namespace: "ns"},
	}
	sortResourceRefs(refs)
	if refs[0].Kind != "ConfigMap" {
		t.Fatalf("unexpected sort order: %#v", refs)
	}
}

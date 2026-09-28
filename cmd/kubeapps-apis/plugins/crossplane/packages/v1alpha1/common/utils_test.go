// Copyright 2021-2023 the Kubeapps contributors.
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"testing"

	apiextensionsv1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDisplayNameAndCategories(t *testing.T) {
	xrd := &apiextensionsv1.CompositeResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "xwidgets.example.org",
			Annotations: map[string]string{
				AnnotationDisplayName:    "Example Widget",
				AnnotationLongDescription: "First line\nSecond line",
				AnnotationCategories:     "database, examples ,",
			},
		},
	}

	if DisplayName(xrd) != "Example Widget" {
		t.Fatalf("unexpected display name: %q", DisplayName(xrd))
	}
	if ShortDescription(xrd) != "First line" {
		t.Fatalf("unexpected short description: %q", ShortDescription(xrd))
	}
	if got := Categories(xrd); len(got) != 2 || got[0] != "database" || got[1] != "examples" {
		t.Fatalf("unexpected categories: %#v", got)
	}
}

func TestDisplayNameFallback(t *testing.T) {
	xrd := &apiextensionsv1.CompositeResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "fallback.example.org"},
	}
	if DisplayName(xrd) != "fallback.example.org" {
		t.Fatalf("expected metadata.name fallback, got %q", DisplayName(xrd))
	}
}

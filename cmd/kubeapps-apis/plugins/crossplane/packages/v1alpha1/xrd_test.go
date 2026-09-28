// Copyright 2021-2023 the Kubeapps contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"

	apiextensionsv1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	corev1 "github.com/SAP/kubeapps/cmd/kubeapps-apis/gen/core/packages/v1alpha1"
	"github.com/SAP/kubeapps/cmd/kubeapps-apis/plugins/crossplane/packages/v1alpha1/common"
)

func TestDeployTargetForXRDClaimPreference(t *testing.T) {
	xrd := loadSampleXRD(t)

	claimTarget, err := deployTargetForXRD(xrd, "v1alpha1", common.DeployPreferenceClaim)
	if err != nil {
		t.Fatalf("deployTargetForXRD claim: %v", err)
	}
	if claimTarget.GVK.Kind != "XWidgetClaim" || claimTarget.Scope != "Namespaced" {
		t.Fatalf("unexpected claim deploy target: %#v", claimTarget)
	}

	compositeTarget, err := deployTargetForXRD(xrd, "v1alpha1", common.DeployPreferenceComposite)
	if err != nil {
		t.Fatalf("deployTargetForXRD composite: %v", err)
	}
	if compositeTarget.GVK.Kind != "XWidget" || compositeTarget.Scope != "Cluster" {
		t.Fatalf("unexpected composite deploy target: %#v", compositeTarget)
	}
}

func TestIsXRDEstablishedAndFilters(t *testing.T) {
	xrd := loadSampleXRD(t)
	if !isXRDEstablished(xrd) {
		t.Fatal("expected sample XRD to be established")
	}

	unestablished := establishedXRD("pending.example.org")
	unestablished.Status.Conditions = nil
	if isXRDEstablished(unestablished) {
		t.Fatal("expected unestablished XRD to be filtered out")
	}

	filtered := filterXRDs([]apiextensionsv1.CompositeResourceDefinition{*xrd}, &corev1.FilterOptions{
		Query: "example widget",
	})
	if len(filtered) != 1 {
		t.Fatalf("expected query filter to match display name, got %d results", len(filtered))
	}

	filtered = filterXRDs([]apiextensionsv1.CompositeResourceDefinition{*xrd}, &corev1.FilterOptions{
		Categories: []string{"database"},
	})
	if len(filtered) != 1 {
		t.Fatalf("expected category filter to match, got %d results", len(filtered))
	}

	filtered = filterXRDs([]apiextensionsv1.CompositeResourceDefinition{*xrd}, &corev1.FilterOptions{
		PkgVersion: "v1alpha1",
	})
	if len(filtered) != 1 {
		t.Fatalf("expected version filter to match, got %d results", len(filtered))
	}
}

func TestMatchesNameAllowlist(t *testing.T) {
	if !matchesNameAllowlist("any", nil) {
		t.Fatal("empty allowlist should allow all names")
	}
	if matchesNameAllowlist("allowed", []string{"allowed"}) != true {
		t.Fatal("expected name to match allowlist")
	}
	if matchesNameAllowlist("denied", []string{"allowed"}) {
		t.Fatal("expected name to be rejected by allowlist")
	}
}

func TestAvailablePackageSummaryFromXRD(t *testing.T) {
	xrd := loadSampleXRD(t)
	summary, err := availablePackageSummaryFromXRD(xrd, "default")
	if err != nil {
		t.Fatalf("availablePackageSummaryFromXRD: %v", err)
	}
	if summary.DisplayName != "Example Widget" {
		t.Fatalf("unexpected display name: %q", summary.DisplayName)
	}
	if summary.AvailablePackageRef.Identifier != xrd.Name {
		t.Fatalf("unexpected identifier: %q", summary.AvailablePackageRef.Identifier)
	}
	if summary.LatestVersion.PkgVersion != "v1alpha1" {
		t.Fatalf("unexpected latest version: %q", summary.LatestVersion.PkgVersion)
	}
}

func TestCompositeScope(t *testing.T) {
	namespaced := apiextensionsv1.CompositeResourceScopeNamespaced
	cluster := apiextensionsv1.CompositeResourceScopeCluster
	xrd := &apiextensionsv1.CompositeResourceDefinition{
		Spec: apiextensionsv1.CompositeResourceDefinitionSpec{
			Scope: &namespaced,
			Names: extv1.CustomResourceDefinitionNames{Plural: "xwidgets"},
		},
	}
	if compositeScope(xrd) != "Namespaced" {
		t.Fatalf("expected Namespaced scope, got %q", compositeScope(xrd))
	}
	xrd.Spec.Scope = &cluster
	if compositeScope(xrd) != "Cluster" {
		t.Fatalf("expected Cluster scope, got %q", compositeScope(xrd))
	}
}

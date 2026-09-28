// Copyright 2021-2023 the Kubeapps contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"

	corev1 "github.com/SAP/kubeapps/cmd/kubeapps-apis/gen/core/packages/v1alpha1"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestInstalledPackageStatusReady(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"metadata": map[string]interface{}{
			"generation": int64(2),
		},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{
					"type":               string(xpv2.TypeReady),
					"status":             "True",
					"observedGeneration": int64(2),
					"message":            "Resource is ready",
				},
				map[string]interface{}{
					"type":   string(xpv2.TypeSynced),
					"status": "True",
				},
			},
		},
	}}

	status := installedPackageStatusFromObject(obj)
	if !status.Ready {
		t.Fatalf("expected ready status, got %#v", status)
	}
	if status.Reason != corev1.InstalledPackageStatus_STATUS_REASON_INSTALLED {
		t.Fatalf("unexpected reason: %v", status.Reason)
	}
}

func TestInstalledPackageStatusPendingReconcile(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"metadata": map[string]interface{}{
			"generation": int64(3),
		},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{
					"type":               string(xpv2.TypeReady),
					"status":             "True",
					"observedGeneration": int64(2),
				},
			},
		},
	}}

	ready, reason, userReason := crossplaneResourceReady(obj)
	if ready {
		t.Fatal("expected pending while generation is ahead of observed generation")
	}
	if reason != corev1.InstalledPackageStatus_STATUS_REASON_PENDING {
		t.Fatalf("unexpected reason: %v", reason)
	}
	if userReason == "" {
		t.Fatal("expected user reason for pending reconcile")
	}
}

func TestInstalledPackageStatusFailed(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"metadata": map[string]interface{}{
			"generation": int64(1),
		},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{
					"type":               string(xpv2.TypeReady),
					"status":             "False",
					"reason":             string(xpv2.ReasonReconcileError),
					"message":            "cannot compose resources",
					"observedGeneration": int64(1),
				},
			},
		},
	}}

	ready, reason, _ := crossplaneResourceReady(obj)
	if ready {
		t.Fatal("expected failed resource to not be ready")
	}
	if reason != corev1.InstalledPackageStatus_STATUS_REASON_FAILED {
		t.Fatalf("unexpected reason: %v", reason)
	}
}

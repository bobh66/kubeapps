// Copyright 2021-2023 the Kubeapps contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"strings"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	corev1 "github.com/SAP/kubeapps/cmd/kubeapps-apis/gen/core/packages/v1alpha1"
	corek8sv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func installedPackageStatusFromObject(obj *unstructured.Unstructured) *corev1.InstalledPackageStatus {
	ready, reason, userReason := crossplaneResourceReady(obj)
	return &corev1.InstalledPackageStatus{
		Ready:      ready,
		Reason:     reason,
		UserReason: userReason,
	}
}

func crossplaneResourceReady(obj *unstructured.Unstructured) (bool, corev1.InstalledPackageStatus_StatusReason, string) {
	generation, _, _ := unstructured.NestedInt64(obj.Object, "metadata", "generation")
	conditions, _, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")

	readyCond := findCondition(conditions, string(xpv2.TypeReady))
	syncedCond := findCondition(conditions, string(xpv2.TypeSynced))

	if readyCond != nil {
		if observedGeneration, ok := conditionObservedGeneration(readyCond); ok && observedGeneration < generation {
			return false, corev1.InstalledPackageStatus_STATUS_REASON_PENDING, "resource is being reconciled"
		}
		if status, _ := conditionStatus(readyCond); status == string(corek8sv1.ConditionTrue) {
			if syncedCond != nil {
				syncedStatus, _ := conditionStatus(syncedCond)
				if syncedStatus == string(corek8sv1.ConditionFalse) {
					return false, corev1.InstalledPackageStatus_STATUS_REASON_PENDING, conditionMessage(syncedCond)
				}
			}
			return true, corev1.InstalledPackageStatus_STATUS_REASON_INSTALLED, conditionMessage(readyCond)
		}
		if isTerminalFailure(readyCond) {
			return false, corev1.InstalledPackageStatus_STATUS_REASON_FAILED, conditionUserReason(readyCond)
		}
		return false, corev1.InstalledPackageStatus_STATUS_REASON_PENDING, conditionUserReason(readyCond)
	}

	if len(conditions) == 0 {
		return false, corev1.InstalledPackageStatus_STATUS_REASON_PENDING, "waiting for status conditions"
	}

	return false, corev1.InstalledPackageStatus_STATUS_REASON_UNSPECIFIED, "resource status is unknown"
}

func findCondition(conditions []interface{}, conditionType string) map[string]interface{} {
	for _, item := range conditions {
		condition, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if condType, ok := condition["type"].(string); ok && condType == conditionType {
			return condition
		}
	}
	return nil
}

func conditionStatus(condition map[string]interface{}) (string, bool) {
	status, ok := condition["status"].(string)
	return status, ok
}

func conditionMessage(condition map[string]interface{}) string {
	message, _ := condition["message"].(string)
	return strings.TrimSpace(message)
}

func conditionUserReason(condition map[string]interface{}) string {
	reason, _ := condition["reason"].(string)
	message := conditionMessage(condition)
	if reason != "" && message != "" {
		return fmt.Sprintf("%s: %s", reason, message)
	}
	if message != "" {
		return message
	}
	return reason
}

func conditionObservedGeneration(condition map[string]interface{}) (int64, bool) {
	value, found, err := unstructured.NestedInt64(condition, "observedGeneration")
	if err != nil || !found {
		return 0, false
	}
	return value, true
}

func isTerminalFailure(condition map[string]interface{}) bool {
	status, ok := conditionStatus(condition)
	if !ok || status != string(corek8sv1.ConditionFalse) {
		return false
	}
	reason, _ := condition["reason"].(string)
	switch reason {
	case string(xpv2.ReasonReconcileError), string(xpv2.ReasonUnavailable):
		return true
	default:
		return strings.Contains(strings.ToLower(reason), "error") || strings.Contains(strings.ToLower(reason), "failed")
	}
}

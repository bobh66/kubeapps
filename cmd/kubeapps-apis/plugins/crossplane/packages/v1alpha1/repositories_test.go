// Copyright 2021-2023 the Kubeapps contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"testing"

	"github.com/bufbuild/connect-go"
	corev1 "github.com/SAP/kubeapps/cmd/kubeapps-apis/gen/core/packages/v1alpha1"
)

func TestGetPackageRepositorySummariesEmpty(t *testing.T) {
	server := &Server{kubeappsCluster: "default"}
	resp, err := server.GetPackageRepositorySummaries(context.Background(), connect.NewRequest(&corev1.GetPackageRepositorySummariesRequest{
		Context: &corev1.Context{Cluster: "default"},
	}))
	if err != nil {
		t.Fatalf("GetPackageRepositorySummaries: %v", err)
	}
	if len(resp.Msg.PackageRepositorySummaries) != 0 {
		t.Fatalf("expected empty repository summaries, got %d", len(resp.Msg.PackageRepositorySummaries))
	}
}

func TestRepositoryMutationsUnimplemented(t *testing.T) {
	server := &Server{kubeappsCluster: "default"}
	_, err := server.AddPackageRepository(context.Background(), connect.NewRequest(&corev1.AddPackageRepositoryRequest{
		Context: &corev1.Context{Cluster: "default", Namespace: "default"},
	}))
	if connect.CodeOf(err) != connect.CodeUnimplemented {
		t.Fatalf("expected Unimplemented, got %v", err)
	}
}

func TestGetPackageRepositoryPermissions(t *testing.T) {
	server := &Server{kubeappsCluster: "default"}
	resp, err := server.GetPackageRepositoryPermissions(context.Background(), connect.NewRequest(&corev1.GetPackageRepositoryPermissionsRequest{
		Context: &corev1.Context{Cluster: "default"},
	}))
	if err != nil {
		t.Fatalf("GetPackageRepositoryPermissions: %v", err)
	}
	if len(resp.Msg.Permissions) != 1 {
		t.Fatalf("expected one plugin permissions entry, got %d", len(resp.Msg.Permissions))
	}
	if len(resp.Msg.Permissions[0].Global) != 0 || len(resp.Msg.Permissions[0].Namespace) != 0 {
		t.Fatalf("expected empty permission maps, got %#v", resp.Msg.Permissions[0])
	}
}

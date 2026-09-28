// Copyright 2021-2023 the Kubeapps contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"

	"github.com/bufbuild/connect-go"
	corev1 "github.com/SAP/kubeapps/cmd/kubeapps-apis/gen/core/packages/v1alpha1"
	log "k8s.io/klog/v2"
)

const repositoriesUnimplementedMsg = "Crossplane plugin does not use package repositories"

func repositoriesUnimplemented() error {
	return connect.NewError(connect.CodeUnimplemented, fmt.Errorf(repositoriesUnimplementedMsg))
}

func (s *Server) GetPackageRepositorySummaries(ctx context.Context, request *connect.Request[corev1.GetPackageRepositorySummariesRequest]) (*connect.Response[corev1.GetPackageRepositorySummariesResponse], error) {
	log.Infof("+crossplane GetPackageRepositorySummaries [%v]", request)
	if err := s.unsupportedCluster(request.Msg.GetContext().GetCluster()); err != nil {
		return nil, err
	}
	return connect.NewResponse(&corev1.GetPackageRepositorySummariesResponse{
		PackageRepositorySummaries: []*corev1.PackageRepositorySummary{},
	}), nil
}

func (s *Server) GetPackageRepositoryDetail(ctx context.Context, request *connect.Request[corev1.GetPackageRepositoryDetailRequest]) (*connect.Response[corev1.GetPackageRepositoryDetailResponse], error) {
	log.Infof("+crossplane GetPackageRepositoryDetail [%v]", request)
	if request.Msg.GetPackageRepoRef() != nil {
		if err := s.unsupportedCluster(request.Msg.GetPackageRepoRef().GetContext().GetCluster()); err != nil {
			return nil, err
		}
	}
	return nil, repositoriesUnimplemented()
}

func (s *Server) AddPackageRepository(ctx context.Context, request *connect.Request[corev1.AddPackageRepositoryRequest]) (*connect.Response[corev1.AddPackageRepositoryResponse], error) {
	log.Infof("+crossplane AddPackageRepository [%v]", request)
	if err := s.unsupportedCluster(request.Msg.GetContext().GetCluster()); err != nil {
		return nil, err
	}
	return nil, repositoriesUnimplemented()
}

func (s *Server) UpdatePackageRepository(ctx context.Context, request *connect.Request[corev1.UpdatePackageRepositoryRequest]) (*connect.Response[corev1.UpdatePackageRepositoryResponse], error) {
	log.Infof("+crossplane UpdatePackageRepository [%v]", request)
	if request.Msg.GetPackageRepoRef() != nil {
		if err := s.unsupportedCluster(request.Msg.GetPackageRepoRef().GetContext().GetCluster()); err != nil {
			return nil, err
		}
	}
	return nil, repositoriesUnimplemented()
}

func (s *Server) DeletePackageRepository(ctx context.Context, request *connect.Request[corev1.DeletePackageRepositoryRequest]) (*connect.Response[corev1.DeletePackageRepositoryResponse], error) {
	log.Infof("+crossplane DeletePackageRepository [%v]", request)
	if request.Msg.GetPackageRepoRef() != nil {
		if err := s.unsupportedCluster(request.Msg.GetPackageRepoRef().GetContext().GetCluster()); err != nil {
			return nil, err
		}
	}
	return nil, repositoriesUnimplemented()
}

func (s *Server) GetPackageRepositoryPermissions(ctx context.Context, request *connect.Request[corev1.GetPackageRepositoryPermissionsRequest]) (*connect.Response[corev1.GetPackageRepositoryPermissionsResponse], error) {
	log.Infof("+crossplane GetPackageRepositoryPermissions [%v]", request)
	if err := s.unsupportedCluster(request.Msg.GetContext().GetCluster()); err != nil {
		return nil, err
	}
	return connect.NewResponse(&corev1.GetPackageRepositoryPermissionsResponse{
		Permissions: []*corev1.PackageRepositoriesPermissions{
			{
				Plugin:    GetPluginDetail(),
				Global:    map[string]bool{},
				Namespace: map[string]bool{},
			},
		},
	}), nil
}

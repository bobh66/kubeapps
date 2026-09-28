// Copyright 2021-2023 the Kubeapps contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"

	"github.com/bufbuild/connect-go"
	apiextensionsv1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1"
	corev1 "github.com/SAP/kubeapps/cmd/kubeapps-apis/gen/core/packages/v1alpha1"
	crossplanev1 "github.com/SAP/kubeapps/cmd/kubeapps-apis/gen/plugins/crossplane/packages/v1alpha1"
	"github.com/SAP/kubeapps/cmd/kubeapps-apis/plugins/crossplane/packages/v1alpha1/common"
	"github.com/SAP/kubeapps/cmd/kubeapps-apis/plugins/pkg/paginate"
	"google.golang.org/protobuf/types/known/anypb"
	log "k8s.io/klog/v2"
)

func (s *Server) GetAvailablePackageSummaries(ctx context.Context, request *connect.Request[corev1.GetAvailablePackageSummariesRequest]) (*connect.Response[corev1.GetAvailablePackageSummariesResponse], error) {
	log.Infof("+crossplane GetAvailablePackageSummaries [%v]", request)
	if err := s.unsupportedCluster(request.Msg.GetContext().GetCluster()); err != nil {
		return nil, err
	}

	xrds, err := s.listXRDs(ctx, request.Header())
	if err != nil {
		return nil, err
	}
	xrds = filterXRDs(xrds, request.Msg.GetFilterOptions())
	categories := collectCategories(xrds)

	pageSize := request.Msg.GetPaginationOptions().GetPageSize()
	itemOffset, err := paginate.ItemOffsetFromPageToken(request.Msg.GetPaginationOptions().GetPageToken())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	end := len(xrds)
	if pageSize > 0 {
		if itemOffset > len(xrds) {
			itemOffset = len(xrds)
		}
		end = itemOffset + int(pageSize)
		if end > len(xrds) {
			end = len(xrds)
		}
	} else if itemOffset > 0 {
		if itemOffset > len(xrds) {
			itemOffset = len(xrds)
		}
	}

	summaries := make([]*corev1.AvailablePackageSummary, 0)
	for _, xrd := range xrds[itemOffset:end] {
		summary, err := availablePackageSummaryFromXRD(&xrd, s.kubeappsCluster)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("unable to build available package summary for XRD [%s]: %w", xrd.Name, err))
		}
		summaries = append(summaries, summary)
	}

	nextPageToken := ""
	if pageSize > 0 && end < len(xrds) {
		nextPageToken = fmt.Sprintf("%d", itemOffset+int(pageSize))
	}

	return connect.NewResponse(&corev1.GetAvailablePackageSummariesResponse{
		AvailablePackageSummaries: summaries,
		NextPageToken:             nextPageToken,
		Categories:                categories,
	}), nil
}

func (s *Server) GetAvailablePackageDetail(ctx context.Context, request *connect.Request[corev1.GetAvailablePackageDetailRequest]) (*connect.Response[corev1.GetAvailablePackageDetailResponse], error) {
	log.Infof("+crossplane GetAvailablePackageDetail [%v]", request)
	if request.Msg.GetAvailablePackageRef() == nil || request.Msg.GetAvailablePackageRef().GetIdentifier() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("available package identifier is required"))
	}
	if err := s.unsupportedCluster(request.Msg.GetAvailablePackageRef().GetContext().GetCluster()); err != nil {
		return nil, err
	}

	xrd, err := s.getXRD(ctx, request.Header(), request.Msg.GetAvailablePackageRef().GetIdentifier())
	if err != nil {
		return nil, err
	}

	version, err := versionByName(xrd, request.Msg.GetPkgVersion())
	if err != nil {
		return nil, err
	}

	detail, err := availablePackageDetailFromXRD(xrd, version, s.kubeappsCluster, s.pluginConfig.DeployPreference)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&corev1.GetAvailablePackageDetailResponse{
		AvailablePackageDetail: detail,
	}), nil
}

func (s *Server) GetAvailablePackageVersions(ctx context.Context, request *connect.Request[corev1.GetAvailablePackageVersionsRequest]) (*connect.Response[corev1.GetAvailablePackageVersionsResponse], error) {
	log.Infof("+crossplane GetAvailablePackageVersions [%v]", request)
	if request.Msg.GetAvailablePackageRef() == nil || request.Msg.GetAvailablePackageRef().GetIdentifier() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("available package identifier is required"))
	}
	if err := s.unsupportedCluster(request.Msg.GetAvailablePackageRef().GetContext().GetCluster()); err != nil {
		return nil, err
	}

	xrd, err := s.getXRD(ctx, request.Header(), request.Msg.GetAvailablePackageRef().GetIdentifier())
	if err != nil {
		return nil, err
	}

	versions := make([]*corev1.PackageAppVersion, 0, len(xrd.Spec.Versions))
	for _, version := range xrd.Spec.Versions {
		if !version.Served {
			continue
		}
		versions = append(versions, &corev1.PackageAppVersion{
			PkgVersion: version.Name,
			AppVersion: "",
		})
	}

	return connect.NewResponse(&corev1.GetAvailablePackageVersionsResponse{
		PackageAppVersions: versions,
	}), nil
}

func (s *Server) GetAvailablePackageMetadatas(ctx context.Context, request *connect.Request[corev1.GetAvailablePackageMetadatasRequest]) (*connect.Response[corev1.GetAvailablePackageMetadatasResponse], error) {
	return nil, unimplemented("GetAvailablePackageMetadatas")
}

func availablePackageDetailFromXRD(xrd *apiextensionsv1.CompositeResourceDefinition, version apiextensionsv1.CompositeResourceDefinitionVersion, kubeappsCluster string, deployPreference common.DeployPreference) (*corev1.AvailablePackageDetail, error) {
	target, err := deployTargetForXRD(xrd, version.Name, deployPreference)
	if err != nil {
		return nil, err
	}

	valuesSchema, defaultValues, err := extractFormSchema(xrd, version)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to extract form schema for XRD [%s]: %w", xrd.Name, err))
	}

	customDetail := &crossplanev1.CrossplanePackageCustomDetail{
		XrdName:       xrd.Name,
		DeployGroup:   target.GVK.Group,
		DeployVersion: target.GVK.Version,
		DeployKind:    target.GVK.Kind,
		Scope:         target.Scope,
	}
	if xrd.OffersClaim() {
		compositeGVK, err := compositeTargetForXRD(xrd, version.Name)
		if err != nil {
			return nil, err
		}
		customDetail.CompositeGroup = compositeGVK.Group
		customDetail.CompositeVersion = compositeGVK.Version
		customDetail.CompositeKind = compositeGVK.Kind
	}

	customDetailAny, err := anypb.New(customDetail)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to encode custom detail: %w", err))
	}

	return &corev1.AvailablePackageDetail{
		AvailablePackageRef: &corev1.AvailablePackageReference{
			Identifier: xrd.Name,
			Plugin:     GetPluginDetail(),
			Context: &corev1.Context{
				Cluster: kubeappsCluster,
			},
		},
		Name:             xrd.Name,
		DisplayName:      common.DisplayName(xrd),
		ShortDescription: common.ShortDescription(xrd),
		LongDescription:  xrd.Annotations[common.AnnotationLongDescription],
		Readme:           xrd.Annotations[common.AnnotationReadme],
		IconUrl:          xrd.Annotations[common.AnnotationIconURL],
		Categories:       common.Categories(xrd),
		Version: &corev1.PackageAppVersion{
			PkgVersion: version.Name,
			AppVersion: "",
		},
		ValuesSchema:   valuesSchema,
		DefaultValues: defaultValues,
		CustomDetail:   customDetailAny,
	}, nil
}

// Copyright 2021-2023 the Kubeapps contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"net/http"
	"sort"

	"github.com/bufbuild/connect-go"
	corev1 "github.com/SAP/kubeapps/cmd/kubeapps-apis/gen/core/packages/v1alpha1"
	crossplanev1 "github.com/SAP/kubeapps/cmd/kubeapps-apis/gen/plugins/crossplane/packages/v1alpha1"
	"github.com/SAP/kubeapps/cmd/kubeapps-apis/plugins/crossplane/packages/v1alpha1/common"
	"github.com/SAP/kubeapps/cmd/kubeapps-apis/plugins/pkg/connecterror"
	"github.com/SAP/kubeapps/cmd/kubeapps-apis/plugins/pkg/paginate"
	"google.golang.org/protobuf/types/known/anypb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	log "k8s.io/klog/v2"
	"sigs.k8s.io/yaml"
)

func (s *Server) GetInstalledPackageSummaries(ctx context.Context, request *connect.Request[corev1.GetInstalledPackageSummariesRequest]) (*connect.Response[corev1.GetInstalledPackageSummariesResponse], error) {
	log.Infof("+crossplane GetInstalledPackageSummaries [%v]", request)
	if err := s.unsupportedCluster(request.Msg.GetContext().GetCluster()); err != nil {
		return nil, err
	}

	itemOffset, err := paginate.ItemOffsetFromPageToken(request.Msg.GetPaginationOptions().GetPageToken())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	namespace := request.Msg.GetContext().GetNamespace()
	resources, err := s.listInstalledResources(ctx, request.Header(), namespace)
	if err != nil {
		return nil, err
	}

	pageSize := request.Msg.GetPaginationOptions().GetPageSize()
	end := len(resources)
	if pageSize > 0 {
		if itemOffset > len(resources) {
			itemOffset = len(resources)
		}
		end = itemOffset + int(pageSize)
		if end > len(resources) {
			end = len(resources)
		}
	} else if itemOffset > 0 {
		if itemOffset > len(resources) {
			itemOffset = len(resources)
		}
	}

	summaries := make([]*corev1.InstalledPackageSummary, 0, end-itemOffset)
	for _, resource := range resources[itemOffset:end] {
		summary, err := s.installedPackageSummaryFromResource(resource)
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, summary)
	}

	nextPageToken := ""
	if pageSize > 0 && end < len(resources) {
		nextPageToken = fmt.Sprintf("%d", itemOffset+int(pageSize))
	}

	return connect.NewResponse(&corev1.GetInstalledPackageSummariesResponse{
		InstalledPackageSummaries: summaries,
		NextPageToken:             nextPageToken,
	}), nil
}

func (s *Server) GetInstalledPackageDetail(ctx context.Context, request *connect.Request[corev1.GetInstalledPackageDetailRequest]) (*connect.Response[corev1.GetInstalledPackageDetailResponse], error) {
	log.Infof("+crossplane GetInstalledPackageDetail [%v]", request)
	if request.Msg.GetInstalledPackageRef() == nil || request.Msg.GetInstalledPackageRef().GetIdentifier() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("installed package identifier is required"))
	}
	if err := s.unsupportedCluster(request.Msg.GetInstalledPackageRef().GetContext().GetCluster()); err != nil {
		return nil, err
	}

	resource, err := s.getInstalledResource(ctx, request.Header(), request.Msg.GetInstalledPackageRef().GetIdentifier(), request.Msg.GetInstalledPackageRef().GetContext().GetNamespace())
	if err != nil {
		return nil, err
	}

	detail, err := s.installedPackageDetailFromResource(*resource)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&corev1.GetInstalledPackageDetailResponse{
		InstalledPackageDetail: detail,
	}), nil
}

func (s *Server) CreateInstalledPackage(ctx context.Context, request *connect.Request[corev1.CreateInstalledPackageRequest]) (*connect.Response[corev1.CreateInstalledPackageResponse], error) {
	log.Infof("+crossplane CreateInstalledPackage [%v]", request)
	if request.Msg.GetAvailablePackageRef() == nil || request.Msg.GetAvailablePackageRef().GetIdentifier() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("available package identifier is required"))
	}
	if request.Msg.GetName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("installed package name is required"))
	}
	if request.Msg.GetTargetContext() == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("target context is required"))
	}
	if err := s.unsupportedCluster(request.Msg.GetAvailablePackageRef().GetContext().GetCluster()); err != nil {
		return nil, err
	}
	if err := s.unsupportedCluster(request.Msg.GetTargetContext().GetCluster()); err != nil {
		return nil, err
	}

	xrd, err := s.getXRD(ctx, request.Header(), request.Msg.GetAvailablePackageRef().GetIdentifier())
	if err != nil {
		return nil, err
	}

	versionName := ""
	if request.Msg.GetPkgVersionReference() != nil {
		versionName = request.Msg.GetPkgVersionReference().GetVersion()
	}
	target, err := deployTargetForXRD(xrd, versionName, s.pluginConfig.DeployPreference)
	if err != nil {
		return nil, err
	}

	namespace := request.Msg.GetTargetContext().GetNamespace()
	if target.Scope == "Namespaced" && namespace == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("target namespace is required for namespaced Crossplane resources"))
	}
	if target.Scope == "Cluster" && namespace != "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("target namespace must be empty for cluster-scoped Crossplane composites"))
	}

	spec, err := parseSpecValues(request.Msg.GetValues())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid values: %w", err))
	}

	obj := buildUnstructuredResource(target, request.Msg.GetName(), namespace, spec)
	dynClient, err := s.clientGetter.Dynamic(request.Header(), s.kubeappsCluster)
	if err != nil {
		return nil, err
	}

	if err := createResource(ctx, dynClient, target, obj); err != nil {
		return nil, err
	}

	return connect.NewResponse(&corev1.CreateInstalledPackageResponse{
		InstalledPackageRef: installedPackageRefFromObject(obj, s.kubeappsCluster),
	}), nil
}

func (s *Server) UpdateInstalledPackage(ctx context.Context, request *connect.Request[corev1.UpdateInstalledPackageRequest]) (*connect.Response[corev1.UpdateInstalledPackageResponse], error) {
	log.Infof("+crossplane UpdateInstalledPackage [%v]", request)
	if request.Msg.GetInstalledPackageRef() == nil || request.Msg.GetInstalledPackageRef().GetIdentifier() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("installed package identifier is required"))
	}
	if err := s.unsupportedCluster(request.Msg.GetInstalledPackageRef().GetContext().GetCluster()); err != nil {
		return nil, err
	}

	resource, err := s.getInstalledResource(ctx, request.Header(), request.Msg.GetInstalledPackageRef().GetIdentifier(), request.Msg.GetInstalledPackageRef().GetContext().GetNamespace())
	if err != nil {
		return nil, err
	}

	if request.Msg.GetPkgVersionReference() != nil && request.Msg.GetPkgVersionReference().GetVersion() != "" {
		currentVersion := resource.Target.GVK.Version
		if request.Msg.GetPkgVersionReference().GetVersion() != currentVersion {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("changing API version from [%s] to [%s] is not supported", currentVersion, request.Msg.GetPkgVersionReference().GetVersion()))
		}
	}

	spec, err := parseSpecValues(request.Msg.GetValues())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid values: %w", err))
	}
	if err := unstructured.SetNestedMap(resource.Object.Object, spec, "spec"); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update spec: %w", err))
	}

	dynClient, err := s.clientGetter.Dynamic(request.Header(), s.kubeappsCluster)
	if err != nil {
		return nil, err
	}
	if err := updateResource(ctx, dynClient, resource.Target, resource.Object); err != nil {
		return nil, err
	}

	return connect.NewResponse(&corev1.UpdateInstalledPackageResponse{
		InstalledPackageRef: installedPackageRefFromObject(resource.Object, s.kubeappsCluster),
	}), nil
}

func (s *Server) DeleteInstalledPackage(ctx context.Context, request *connect.Request[corev1.DeleteInstalledPackageRequest]) (*connect.Response[corev1.DeleteInstalledPackageResponse], error) {
	log.Infof("+crossplane DeleteInstalledPackage [%v]", request)
	if request.Msg.GetInstalledPackageRef() == nil || request.Msg.GetInstalledPackageRef().GetIdentifier() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("installed package identifier is required"))
	}
	if err := s.unsupportedCluster(request.Msg.GetInstalledPackageRef().GetContext().GetCluster()); err != nil {
		return nil, err
	}

	resource, err := s.getInstalledResource(ctx, request.Header(), request.Msg.GetInstalledPackageRef().GetIdentifier(), request.Msg.GetInstalledPackageRef().GetContext().GetNamespace())
	if err != nil {
		return nil, err
	}

	dynClient, err := s.clientGetter.Dynamic(request.Header(), s.kubeappsCluster)
	if err != nil {
		return nil, err
	}
	if err := deleteResource(ctx, dynClient, resource.Target, resource.Object.GetName(), resource.Object.GetNamespace()); err != nil {
		return nil, err
	}

	return connect.NewResponse(&corev1.DeleteInstalledPackageResponse{}), nil
}

func (s *Server) GetInstalledPackageResourceRefs(ctx context.Context, request *connect.Request[corev1.GetInstalledPackageResourceRefsRequest]) (*connect.Response[corev1.GetInstalledPackageResourceRefsResponse], error) {
	log.Infof("+crossplane GetInstalledPackageResourceRefs [%v]", request)
	if request.Msg.GetInstalledPackageRef() == nil || request.Msg.GetInstalledPackageRef().GetIdentifier() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("installed package identifier is required"))
	}
	if err := s.unsupportedCluster(request.Msg.GetInstalledPackageRef().GetContext().GetCluster()); err != nil {
		return nil, err
	}

	installed, err := s.getInstalledResource(ctx, request.Header(), request.Msg.GetInstalledPackageRef().GetIdentifier(), request.Msg.GetInstalledPackageRef().GetContext().GetNamespace())
	if err != nil {
		return nil, err
	}

	refs, err := getInstalledPackageResourceRefs(ctx, request.Header(), s.kubeappsCluster, s.clientGetter, *installed)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&corev1.GetInstalledPackageResourceRefsResponse{
		Context: &corev1.Context{
			Cluster:   s.kubeappsCluster,
			Namespace: request.Msg.GetInstalledPackageRef().GetContext().GetNamespace(),
		},
		ResourceRefs: refs,
	}), nil
}

func (s *Server) listInstalledResources(ctx context.Context, headers http.Header, namespace string) ([]installedResource, error) {
	xrds, err := s.listXRDs(ctx, headers)
	if err != nil {
		return nil, err
	}

	targets, err := allDeployTargets(xrds, s.pluginConfig.DeployPreference)
	if err != nil {
		return nil, err
	}

	dynClient, err := s.clientGetter.Dynamic(headers, s.kubeappsCluster)
	if err != nil {
		return nil, err
	}

	resources := make([]installedResource, 0)
	for _, target := range targets {
		if target.Target.Scope == "Namespaced" {
			if namespace == "" {
				continue
			}
			items, err := listResources(ctx, dynClient, target.Target, namespace)
			if err != nil {
				return nil, err
			}
			for _, item := range items {
				resources = append(resources, installedResource{Object: item, XRD: target.XRD, Target: target.Target})
			}
			continue
		}
		if namespace != "" {
			continue
		}
		items, err := listResources(ctx, dynClient, target.Target, "")
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			resources = append(resources, installedResource{Object: item, XRD: target.XRD, Target: target.Target})
		}
	}

	sort.Slice(resources, func(i, j int) bool {
		left := resources[i].Object.GetName()
		right := resources[j].Object.GetName()
		if resources[i].Object.GetNamespace() != resources[j].Object.GetNamespace() {
			left = resources[i].Object.GetNamespace() + "/" + left
			right = resources[j].Object.GetNamespace() + "/" + right
		}
		return left < right
	})
	return resources, nil
}

func (s *Server) getInstalledResource(ctx context.Context, headers http.Header, name, namespace string) (*installedResource, error) {
	xrds, err := s.listXRDs(ctx, headers)
	if err != nil {
		return nil, err
	}
	targets, err := allDeployTargets(xrds, s.pluginConfig.DeployPreference)
	if err != nil {
		return nil, err
	}

	dynClient, err := s.clientGetter.Dynamic(headers, s.kubeappsCluster)
	if err != nil {
		return nil, err
	}

	for _, target := range targets {
		if target.Target.Scope == "Namespaced" && namespace == "" {
			continue
		}
		if target.Target.Scope == "Cluster" && namespace != "" {
			continue
		}
		obj, err := getResource(ctx, dynClient, target.Target, name, namespace)
		if err != nil {
			if connect.CodeOf(err) == connect.CodeNotFound {
				continue
			}
			return nil, err
		}
		return &installedResource{Object: obj, XRD: target.XRD, Target: target.Target}, nil
	}

	identifier := name
	if namespace != "" {
		identifier = namespace + "/" + name
	}
	return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("installed package [%s] not found", identifier))
}

func (s *Server) installedPackageSummaryFromResource(resource installedResource) (*corev1.InstalledPackageSummary, error) {
	detail, err := s.installedPackageDetailFromResource(resource)
	if err != nil {
		return nil, err
	}

	latestVersion, err := defaultServedVersion(&resource.XRD)
	if err != nil {
		return nil, err
	}

	return &corev1.InstalledPackageSummary{
		InstalledPackageRef: detail.InstalledPackageRef,
		Name:                detail.Name,
		PkgVersionReference: detail.PkgVersionReference,
		CurrentVersion:      detail.CurrentVersion,
		IconUrl:             resource.XRD.Annotations[common.AnnotationIconURL],
		PkgDisplayName:      common.DisplayName(&resource.XRD),
		ShortDescription:    common.ShortDescription(&resource.XRD),
		Status:              detail.Status,
		LatestVersion: &corev1.PackageAppVersion{
			PkgVersion: latestVersion.Name,
			AppVersion: "",
		},
	}, nil
}

func (s *Server) installedPackageDetailFromResource(resource installedResource) (*corev1.InstalledPackageDetail, error) {
	valuesApplied, err := specValuesYAML(resource.Object)
	if err != nil {
		return nil, err
	}

	generation, _, _ := unstructured.NestedInt64(resource.Object.Object, "metadata", "generation")
	customDetailAny, err := anypb.New(&crossplanev1.CrossplaneInstalledPackageCustomDetail{
		ResourceName:            resource.Object.GetName(),
		ResourceNamespace:       resource.Object.GetNamespace(),
		Generation:              generation,
		ReadyConditionMessage:   readyConditionMessage(resource.Object),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to encode custom detail: %w", err))
	}

	return &corev1.InstalledPackageDetail{
		InstalledPackageRef: installedPackageRefFromObject(resource.Object, s.kubeappsCluster),
		Name:                resource.Object.GetName(),
		PkgVersionReference: &corev1.VersionReference{
			Version: resource.Target.GVK.Version,
		},
		CurrentVersion: &corev1.PackageAppVersion{
			PkgVersion: resource.Target.GVK.Version,
			AppVersion: "",
		},
		ValuesApplied: valuesApplied,
		AvailablePackageRef: &corev1.AvailablePackageReference{
			Identifier: resource.XRD.Name,
			Plugin:     GetPluginDetail(),
			Context: &corev1.Context{
				Cluster: s.kubeappsCluster,
			},
		},
		Status:       installedPackageStatusFromObject(resource.Object),
		CustomDetail: customDetailAny,
	}, nil
}

func installedPackageRefFromObject(obj *unstructured.Unstructured, kubeappsCluster string) *corev1.InstalledPackageReference {
	return &corev1.InstalledPackageReference{
		Context: &corev1.Context{
			Namespace: obj.GetNamespace(),
			Cluster:   kubeappsCluster,
		},
		Identifier: obj.GetName(),
		Plugin:     GetPluginDetail(),
	}
}

func buildUnstructuredResource(target deployTarget, name, namespace string, spec map[string]interface{}) *unstructured.Unstructured {
	obj := map[string]interface{}{
		"apiVersion": target.GVK.GroupVersion().String(),
		"kind":       target.GVK.Kind,
		"metadata": map[string]interface{}{
			"name": name,
		},
		"spec": spec,
	}
	if target.Scope == "Namespaced" {
		obj["metadata"].(map[string]interface{})["namespace"] = namespace
	}
	return &unstructured.Unstructured{Object: obj}
}

func parseSpecValues(valuesString string) (map[string]interface{}, error) {
	if valuesString == "" {
		return map[string]interface{}{}, nil
	}
	spec := map[string]interface{}{}
	if err := yaml.Unmarshal([]byte(valuesString), &spec); err != nil {
		return nil, err
	}
	return spec, nil
}

func specValuesYAML(obj *unstructured.Unstructured) (string, error) {
	spec, found, err := unstructured.NestedMap(obj.Object, "spec")
	if err != nil {
		return "", err
	}
	if !found || spec == nil {
		spec = map[string]interface{}{}
	}
	yamlBytes, err := yaml.Marshal(spec)
	if err != nil {
		return "", err
	}
	return string(yamlBytes), nil
}

func readyConditionMessage(obj *unstructured.Unstructured) string {
	conditions, _, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	readyCond := findCondition(conditions, "Ready")
	if readyCond == nil {
		return ""
	}
	return conditionMessage(readyCond)
}

func resourceInterface(dynClient dynamic.Interface, target deployTarget, namespace string) dynamic.ResourceInterface {
	gvr := target.GVR()
	if target.Scope == "Cluster" {
		return dynClient.Resource(gvr)
	}
	return dynClient.Resource(gvr).Namespace(namespace)
}

func listResources(ctx context.Context, dynClient dynamic.Interface, target deployTarget, namespace string) ([]*unstructured.Unstructured, error) {
	list, err := resourceInterface(dynClient, target, namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, connecterror.FromK8sError("list", target.GVK.Kind, namespace, err)
	}
	items := make([]*unstructured.Unstructured, 0, len(list.Items))
	for i := range list.Items {
		items = append(items, list.Items[i].DeepCopy())
	}
	return items, nil
}

func getResource(ctx context.Context, dynClient dynamic.Interface, target deployTarget, name, namespace string) (*unstructured.Unstructured, error) {
	obj, err := resourceInterface(dynClient, target, namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, connecterror.FromK8sError("get", target.GVK.Kind, types.NamespacedName{Namespace: namespace, Name: name}.String(), err)
	}
	return obj, nil
}

func createResource(ctx context.Context, dynClient dynamic.Interface, target deployTarget, obj *unstructured.Unstructured) error {
	_, err := resourceInterface(dynClient, target, obj.GetNamespace()).Create(ctx, obj, metav1.CreateOptions{})
	if err != nil {
		return connecterror.FromK8sError("create", target.GVK.Kind, obj.GetName(), err)
	}
	return nil
}

func updateResource(ctx context.Context, dynClient dynamic.Interface, target deployTarget, obj *unstructured.Unstructured) error {
	_, err := resourceInterface(dynClient, target, obj.GetNamespace()).Update(ctx, obj, metav1.UpdateOptions{})
	if err != nil {
		return connecterror.FromK8sError("update", target.GVK.Kind, obj.GetName(), err)
	}
	return nil
}

func deleteResource(ctx context.Context, dynClient dynamic.Interface, target deployTarget, name, namespace string) error {
	err := resourceInterface(dynClient, target, namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return connecterror.FromK8sError("delete", target.GVK.Kind, types.NamespacedName{Namespace: namespace, Name: name}.String(), err)
	}
	return nil
}

// Copyright 2021-2023 the Kubeapps contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/bufbuild/connect-go"
	apiextensionsv1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1"
	corev1 "github.com/SAP/kubeapps/cmd/kubeapps-apis/gen/core/packages/v1alpha1"
	"github.com/SAP/kubeapps/cmd/kubeapps-apis/plugins/crossplane/packages/v1alpha1/common"
	"github.com/SAP/kubeapps/cmd/kubeapps-apis/plugins/pkg/connecterror"
	corek8sv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type deployTarget struct {
	GVK    schema.GroupVersionKind
	Scope  string
	Plural string
}

type xrdDeployTarget struct {
	XRD    apiextensionsv1.CompositeResourceDefinition
	Target deployTarget
}

type installedResource struct {
	Object *unstructured.Unstructured
	XRD    apiextensionsv1.CompositeResourceDefinition
	Target deployTarget
}

func (t deployTarget) GVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    t.GVK.Group,
		Version:  t.GVK.Version,
		Resource: t.Plural,
	}
}

func allDeployTargets(xrds []apiextensionsv1.CompositeResourceDefinition, pref common.DeployPreference) ([]xrdDeployTarget, error) {
	targets := make([]xrdDeployTarget, 0, len(xrds))
	for _, xrd := range xrds {
		version, err := defaultServedVersion(&xrd)
		if err != nil {
			continue
		}
		target, err := deployTargetForXRD(&xrd, version.Name, pref)
		if err != nil {
			return nil, err
		}
		targets = append(targets, xrdDeployTarget{XRD: xrd, Target: target})
	}
	return targets, nil
}

func (s *Server) listXRDs(ctx context.Context, headers http.Header) ([]apiextensionsv1.CompositeResourceDefinition, error) {
	client, err := s.clientGetter.ControllerRuntime(headers, s.kubeappsCluster)
	if err != nil {
		return nil, err
	}

	listOpts := []ctrlclient.ListOption{}
	if s.pluginConfig.LabelSelector != "" {
		selector, err := labels.Parse(s.pluginConfig.LabelSelector)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid labelSelector in plugin config: %w", err))
		}
		listOpts = append(listOpts, ctrlclient.MatchingLabelsSelector{Selector: selector})
	}

	var xrdList apiextensionsv1.CompositeResourceDefinitionList
	if err := client.List(ctx, &xrdList, listOpts...); err != nil {
		return nil, connecterror.FromK8sError("list", "CompositeResourceDefinition", "", err)
	}

	xrds := make([]apiextensionsv1.CompositeResourceDefinition, 0, len(xrdList.Items))
	for _, xrd := range xrdList.Items {
		if !isXRDEstablished(&xrd) {
			continue
		}
		if !matchesNameAllowlist(xrd.Name, s.pluginConfig.NameAllowlist) {
			continue
		}
		xrds = append(xrds, xrd)
	}

	sort.Slice(xrds, func(i, j int) bool {
		return xrds[i].Name < xrds[j].Name
	})
	return xrds, nil
}

func (s *Server) getXRD(ctx context.Context, headers http.Header, name string) (*apiextensionsv1.CompositeResourceDefinition, error) {
	client, err := s.clientGetter.ControllerRuntime(headers, s.kubeappsCluster)
	if err != nil {
		return nil, err
	}

	xrd := &apiextensionsv1.CompositeResourceDefinition{}
	if err := client.Get(ctx, ctrlclient.ObjectKey{Name: name}, xrd); err != nil {
		return nil, connecterror.FromK8sError("get", "CompositeResourceDefinition", name, err)
	}
	if !isXRDEstablished(xrd) {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("CompositeResourceDefinition [%s] is not established", name))
	}
	return xrd, nil
}

func isXRDEstablished(xrd *apiextensionsv1.CompositeResourceDefinition) bool {
	for _, condition := range xrd.Status.Conditions {
		if condition.Type == apiextensionsv1.TypeEstablished && condition.Status == corek8sv1.ConditionTrue {
			return true
		}
	}
	return false
}

func matchesNameAllowlist(name string, allowlist []string) bool {
	if len(allowlist) == 0 {
		return true
	}
	for _, allowed := range allowlist {
		if allowed == name {
			return true
		}
	}
	return false
}

func defaultServedVersion(xrd *apiextensionsv1.CompositeResourceDefinition) (apiextensionsv1.CompositeResourceDefinitionVersion, error) {
	var referenceable *apiextensionsv1.CompositeResourceDefinitionVersion
	var servedFallback *apiextensionsv1.CompositeResourceDefinitionVersion
	for i := range xrd.Spec.Versions {
		version := xrd.Spec.Versions[i]
		if !version.Served {
			continue
		}
		if version.Referenceable {
			referenceable = &xrd.Spec.Versions[i]
		}
		if servedFallback == nil {
			servedFallback = &xrd.Spec.Versions[i]
		}
	}
	if referenceable != nil {
		return *referenceable, nil
	}
	if servedFallback != nil {
		return *servedFallback, nil
	}
	return apiextensionsv1.CompositeResourceDefinitionVersion{}, fmt.Errorf("no served versions found for XRD [%s]", xrd.Name)
}

func versionByName(xrd *apiextensionsv1.CompositeResourceDefinition, versionName string) (apiextensionsv1.CompositeResourceDefinitionVersion, error) {
	if versionName == "" {
		return defaultServedVersion(xrd)
	}
	for _, version := range xrd.Spec.Versions {
		if version.Name == versionName {
			if !version.Served {
				return apiextensionsv1.CompositeResourceDefinitionVersion{}, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("version [%s] is not served for XRD [%s]", versionName, xrd.Name))
			}
			return version, nil
		}
	}
	return apiextensionsv1.CompositeResourceDefinitionVersion{}, connect.NewError(connect.CodeNotFound, fmt.Errorf("version [%s] not found for XRD [%s]", versionName, xrd.Name))
}

func deployTargetForXRD(xrd *apiextensionsv1.CompositeResourceDefinition, versionName string, pref common.DeployPreference) (deployTarget, error) {
	version, err := versionByName(xrd, versionName)
	if err != nil {
		return deployTarget{}, err
	}

	useClaim := xrd.OffersClaim() && pref != common.DeployPreferenceComposite
	if useClaim {
		return deployTarget{
			GVK: schema.GroupVersionKind{
				Group:   xrd.Spec.Group,
				Version: version.Name,
				Kind:    xrd.Spec.ClaimNames.Kind,
			},
			Scope:  "Namespaced",
			Plural: xrd.Spec.ClaimNames.Plural,
		}, nil
	}

	return deployTarget{
		GVK: schema.GroupVersionKind{
			Group:   xrd.Spec.Group,
			Version: version.Name,
			Kind:    xrd.Spec.Names.Kind,
		},
		Scope:  compositeScope(xrd),
		Plural: xrd.Spec.Names.Plural,
	}, nil
}

func compositeScope(xrd *apiextensionsv1.CompositeResourceDefinition) string {
	if xrd.Spec.Scope == nil {
		return "Cluster"
	}
	switch *xrd.Spec.Scope {
	case apiextensionsv1.CompositeResourceScopeNamespaced:
		return "Namespaced"
	default:
		return "Cluster"
	}
}

func compositeTargetForXRD(xrd *apiextensionsv1.CompositeResourceDefinition, versionName string) (schema.GroupVersionKind, error) {
	version, err := versionByName(xrd, versionName)
	if err != nil {
		return schema.GroupVersionKind{}, err
	}
	return schema.GroupVersionKind{
		Group:   xrd.Spec.Group,
		Version: version.Name,
		Kind:    xrd.Spec.Names.Kind,
	}, nil
}

func filterXRDs(xrds []apiextensionsv1.CompositeResourceDefinition, filters *corev1.FilterOptions) []apiextensionsv1.CompositeResourceDefinition {
	if filters == nil {
		return xrds
	}

	filtered := make([]apiextensionsv1.CompositeResourceDefinition, 0, len(xrds))
	for _, xrd := range xrds {
		if !matchesCategoryFilter(xrd, filters.Categories) {
			continue
		}
		if filters.PkgVersion != "" && !xrdHasServedVersion(xrd, filters.PkgVersion) {
			continue
		}
		if filters.Query != "" && !matchesQuery(xrd, filters.Query) {
			continue
		}
		filtered = append(filtered, xrd)
	}
	return filtered
}

func matchesCategoryFilter(xrd apiextensionsv1.CompositeResourceDefinition, categories []string) bool {
	if len(categories) == 0 {
		return true
	}
	xrdCategories := common.Categories(&xrd)
	for _, requested := range categories {
		for _, actual := range xrdCategories {
			if strings.EqualFold(requested, actual) {
				return true
			}
		}
	}
	return false
}

func xrdHasServedVersion(xrd apiextensionsv1.CompositeResourceDefinition, versionName string) bool {
	for _, version := range xrd.Spec.Versions {
		if version.Name == versionName && version.Served {
			return true
		}
	}
	return false
}

func matchesQuery(xrd apiextensionsv1.CompositeResourceDefinition, query string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return true
	}
	candidates := []string{
		xrd.Name,
		common.DisplayName(&xrd),
		common.ShortDescription(&xrd),
	}
	for _, candidate := range candidates {
		if strings.Contains(strings.ToLower(candidate), query) {
			return true
		}
	}
	return false
}

func collectCategories(xrds []apiextensionsv1.CompositeResourceDefinition) []string {
	seen := map[string]struct{}{}
	categories := make([]string, 0)
	for _, xrd := range xrds {
		for _, category := range common.Categories(&xrd) {
			if _, ok := seen[category]; ok {
				continue
			}
			seen[category] = struct{}{}
			categories = append(categories, category)
		}
	}
	sort.Strings(categories)
	return categories
}

func availablePackageSummaryFromXRD(xrd *apiextensionsv1.CompositeResourceDefinition, kubeappsCluster string) (*corev1.AvailablePackageSummary, error) {
	version, err := defaultServedVersion(xrd)
	if err != nil {
		return nil, err
	}

	return &corev1.AvailablePackageSummary{
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
		IconUrl:          xrd.Annotations[common.AnnotationIconURL],
		Categories:       common.Categories(xrd),
		LatestVersion: &corev1.PackageAppVersion{
			PkgVersion: version.Name,
			AppVersion: "",
		},
	}, nil
}

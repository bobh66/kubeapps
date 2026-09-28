// Copyright 2021-2023 the Kubeapps contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"net/http"
	"sort"

	"github.com/bufbuild/connect-go"
	cpresource "github.com/crossplane/cli/v2/cmd/crossplane/common/resource"
	"github.com/crossplane/cli/v2/cmd/crossplane/common/resource/xrm"
	corev1 "github.com/SAP/kubeapps/cmd/kubeapps-apis/gen/core/packages/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func getInstalledPackageResourceRefs(ctx context.Context, headers http.Header, kubeappsCluster string, clientGetter clientGetter, installed installedResource) ([]*corev1.ResourceRef, error) {
	ctrlClient, err := clientGetter.ControllerRuntime(headers, kubeappsCluster)
	if err != nil {
		return nil, err
	}

	treeClient, err := xrm.NewClient(ctrlClient, xrm.WithConnectionSecrets(false))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create Crossplane resource tree client: %w", err))
	}

	root := &cpresource.Resource{Unstructured: *installed.Object.DeepCopy()}
	if _, err := treeClient.GetResourceTree(ctx, root); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get Crossplane resource tree: %w", err))
	}
	if root.Error != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get installed Crossplane resource: %w", root.Error))
	}

	refs := flattenResourceTreeToRefs(root)
	sortResourceRefs(refs)
	return refs, nil
}

func flattenResourceTreeToRefs(root *cpresource.Resource) []*corev1.ResourceRef {
	seen := map[string]struct{}{}
	return collectResourceRefs(root, nil, seen)
}

func collectResourceRefs(node *cpresource.Resource, refs []*corev1.ResourceRef, seen map[string]struct{}) []*corev1.ResourceRef {
	if node == nil {
		return refs
	}

	if ref := resourceRefFromCrossplaneResource(node); ref != nil {
		key := ref.ApiVersion + "/" + ref.Kind + "/" + ref.Namespace + "/" + ref.Name
		if _, exists := seen[key]; !exists {
			seen[key] = struct{}{}
			refs = append(refs, ref)
		}
	}

	for _, child := range node.Children {
		refs = collectResourceRefs(child, refs, seen)
	}
	return refs
}

func resourceRefFromCrossplaneResource(node *cpresource.Resource) *corev1.ResourceRef {
	if node == nil || node.Unstructured.GetKind() == "" || node.Unstructured.GetName() == "" {
		return nil
	}
	return &corev1.ResourceRef{
		ApiVersion: node.Unstructured.GetAPIVersion(),
		Kind:       node.Unstructured.GetKind(),
		Name:       node.Unstructured.GetName(),
		Namespace:  node.Unstructured.GetNamespace(),
	}
}

func sortResourceRefs(refs []*corev1.ResourceRef) {
	sort.Slice(refs, func(i, j int) bool {
		left := refs[i].GetNamespace() + "/" + refs[i].GetKind() + "/" + refs[i].GetName()
		right := refs[j].GetNamespace() + "/" + refs[j].GetKind() + "/" + refs[j].GetName()
		if left == right {
			return refs[i].GetApiVersion() < refs[j].GetApiVersion()
		}
		return left < right
	})
}

type clientGetter interface {
	ControllerRuntime(headers http.Header, cluster string) (client.WithWatch, error)
}

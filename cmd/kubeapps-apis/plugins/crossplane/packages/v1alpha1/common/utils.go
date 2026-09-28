// Copyright 2021-2023 the Kubeapps contributors.
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"fmt"
	"strings"

	apiextensionsv1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	UserAgentPrefix = "kubeapps-apis/plugins"

	AnnotationDisplayName    = "kubeapps.crossplane.io/display-name"
	AnnotationIconURL        = "kubeapps.crossplane.io/icon-url"
	AnnotationDefaultSpec    = "kubeapps.crossplane.io/default-spec"
	AnnotationCategories     = "categories"
	AnnotationReadme         = "kubeapps.crossplane.io/readme"
	AnnotationLongDescription = "kubeapps.crossplane.io/description"
)

// CompositeResourceDefinitionGVR is the GVR for Crossplane XRDs.
var CompositeResourceDefinitionGVR = schema.GroupVersionResource{
	Group:    apiextensionsv1.Group,
	Version:  apiextensionsv1.Version,
	Resource: "compositeresourcedefinitions",
}

// UserAgentString returns the user agent for outbound HTTP calls.
func UserAgentString(pluginName, pluginVersion, version string) string {
	return fmt.Sprintf("%s/%s/%s/%s", UserAgentPrefix, pluginName, pluginVersion, version)
}

// DisplayName returns the user-facing name for an XRD.
func DisplayName(xrd *apiextensionsv1.CompositeResourceDefinition) string {
	if name := xrd.Annotations[AnnotationDisplayName]; name != "" {
		return name
	}
	return xrd.Name
}

// ShortDescription returns a short description for an XRD.
func ShortDescription(xrd *apiextensionsv1.CompositeResourceDefinition) string {
	if desc := xrd.Annotations[AnnotationLongDescription]; desc != "" {
		return firstLine(desc)
	}
	return ""
}

// Categories returns catalog categories for an XRD.
func Categories(xrd *apiextensionsv1.CompositeResourceDefinition) []string {
	if raw := xrd.Annotations[AnnotationCategories]; raw != "" {
		parts := strings.Split(raw, ",")
		categories := make([]string, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				categories = append(categories, part)
			}
		}
		return categories
	}
	return nil
}

func firstLine(s string) string {
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		return strings.TrimSpace(s[:idx])
	}
	return strings.TrimSpace(s)
}

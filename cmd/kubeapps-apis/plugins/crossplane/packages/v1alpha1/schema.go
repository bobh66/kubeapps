// Copyright 2021-2023 the Kubeapps contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"

	apiextensionsv1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1"
	"github.com/SAP/kubeapps/cmd/kubeapps-apis/plugins/crossplane/packages/v1alpha1/common"
	"sigs.k8s.io/yaml"
)

func extractFormSchema(xrd *apiextensionsv1.CompositeResourceDefinition, version apiextensionsv1.CompositeResourceDefinitionVersion) (string, string, error) {
	specSchema, err := specSchemaFromVersion(version)
	if err != nil {
		return "", "", err
	}

	normalized, err := normalizeJSONSchema(specSchema)
	if err != nil {
		return "", "", err
	}

	valuesSchemaBytes, err := json.Marshal(normalized)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal values schema: %w", err)
	}

	defaultValues, err := defaultValuesForXRD(xrd, normalized)
	if err != nil {
		return "", "", err
	}

	return string(valuesSchemaBytes), defaultValues, nil
}

func specSchemaFromVersion(version apiextensionsv1.CompositeResourceDefinitionVersion) (map[string]interface{}, error) {
	if version.Schema == nil || len(version.Schema.OpenAPIV3Schema.Raw) == 0 {
		return map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		}, nil
	}

	var root map[string]interface{}
	if err := json.Unmarshal(version.Schema.OpenAPIV3Schema.Raw, &root); err != nil {
		return nil, fmt.Errorf("failed to parse openAPIV3Schema: %w", err)
	}

	specSchema, ok := nestedPropertySchema(root, "spec")
	if !ok {
		return map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		}, nil
	}
	return specSchema, nil
}

func nestedPropertySchema(root map[string]interface{}, property string) (map[string]interface{}, bool) {
	properties, ok := root["properties"].(map[string]interface{})
	if !ok {
		return nil, false
	}
	value, ok := properties[property].(map[string]interface{})
	return value, ok
}

func normalizeJSONSchema(schema map[string]interface{}) (map[string]interface{}, error) {
	normalized := normalizeSchemaNode(schema)
	if normalized == nil {
		normalized = map[string]interface{}{}
	}

	if _, ok := normalized["type"]; !ok {
		normalized["type"] = "object"
	}
	if normalized["type"] == "object" {
		if _, ok := normalized["properties"]; !ok {
			normalized["properties"] = map[string]interface{}{}
		}
	}
	return normalized, nil
}

func normalizeSchemaNode(node interface{}) map[string]interface{} {
	switch value := node.(type) {
	case map[string]interface{}:
		normalized := make(map[string]interface{}, len(value))
		for key, child := range value {
			if stringsHasPrefix(key, "x-kubernetes-") {
				continue
			}
			switch key {
			case "properties":
				if props, ok := child.(map[string]interface{}); ok {
					normalizedProps := make(map[string]interface{}, len(props))
					for propName, propSchema := range props {
						if normalizedProp := normalizeSchemaNode(propSchema); normalizedProp != nil {
							normalizedProps[propName] = normalizedProp
						}
					}
					normalized[key] = normalizedProps
				}
			case "items":
				if normalizedItem := normalizeSchemaNode(child); normalizedItem != nil {
					normalized[key] = normalizedItem
				}
			case "additionalProperties":
				if normalizedAdditional := normalizeSchemaNode(child); normalizedAdditional != nil {
					normalized[key] = normalizedAdditional
				}
			default:
				if key == "allOf" || key == "anyOf" || key == "oneOf" {
					if list, ok := child.([]interface{}); ok {
						normalizedList := make([]interface{}, 0, len(list))
						for _, item := range list {
							if normalizedItem := normalizeSchemaNode(item); normalizedItem != nil {
								normalizedList = append(normalizedList, normalizedItem)
							}
						}
						normalized[key] = normalizedList
					}
					continue
				}
				normalized[key] = child
			}
		}
		return normalized
	default:
		return nil
	}
}

func stringsHasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func defaultValuesForXRD(xrd *apiextensionsv1.CompositeResourceDefinition, specSchema map[string]interface{}) (string, error) {
	if raw := xrd.Annotations[common.AnnotationDefaultSpec]; raw != "" {
		var parsed interface{}
		if err := yaml.Unmarshal([]byte(raw), &parsed); err != nil {
			if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
				return "", fmt.Errorf("failed to parse %s annotation: %w", common.AnnotationDefaultSpec, err)
			}
		}
		yamlBytes, err := yaml.Marshal(parsed)
		if err != nil {
			return "", fmt.Errorf("failed to marshal default spec annotation: %w", err)
		}
		return string(yamlBytes), nil
	}

	defaults := defaultsFromSchema(specSchema)
	yamlBytes, err := yaml.Marshal(defaults)
	if err != nil {
		return "", fmt.Errorf("failed to marshal default values: %w", err)
	}
	return string(yamlBytes), nil
}

func defaultsFromSchema(schema map[string]interface{}) map[string]interface{} {
	defaults := map[string]interface{}{}
	if schema == nil {
		return defaults
	}
	if defaultValue, ok := schema["default"]; ok {
		if defaultMap, ok := defaultValue.(map[string]interface{}); ok {
			return defaultMap
		}
	}

	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		return defaults
	}

	for name, propertySchema := range properties {
		propMap, ok := propertySchema.(map[string]interface{})
		if !ok {
			continue
		}
		if defaultValue, ok := propMap["default"]; ok {
			defaults[name] = defaultValue
			continue
		}
		if propMap["type"] == "object" {
			nested := defaultsFromSchema(propMap)
			if len(nested) > 0 {
				defaults[name] = nested
			}
		}
	}
	return defaults
}

// Copyright 2021-2023 the Kubeapps contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"

	"github.com/bufbuild/connect-go"
	apiextensionsv1 "github.com/crossplane/crossplane/apis/v2/apiextensions/v1"
	"github.com/SAP/kubeapps/cmd/kubeapps-apis/core"
	corev1connect "github.com/SAP/kubeapps/cmd/kubeapps-apis/gen/core/packages/v1alpha1/v1alpha1connect"
	"github.com/SAP/kubeapps/cmd/kubeapps-apis/gen/plugins/crossplane/packages/v1alpha1"
	"github.com/SAP/kubeapps/cmd/kubeapps-apis/plugins/crossplane/packages/v1alpha1/common"
	"github.com/SAP/kubeapps/cmd/kubeapps-apis/plugins/pkg/clientgetter"
	"k8s.io/apimachinery/pkg/runtime"
	log "k8s.io/klog/v2"
)

// Compile-time statement to ensure this service implementation satisfies the core packaging API.
var _ corev1connect.PackagesServiceHandler = (*Server)(nil)
var _ corev1connect.RepositoriesServiceHandler = (*Server)(nil)

// Server implements the crossplane packages v1alpha1 interface.
type Server struct {
	v1alpha1.UnimplementedCrossplanePackagesServiceServer
	v1alpha1.UnimplementedCrossplaneRepositoriesServiceServer

	kubeappsCluster string
	clientGetter    clientgetter.ClientProviderInterface
	pluginConfig    *common.CrossplanePluginConfig
}

// NewServer returns a Server configured with a function to obtain the k8s client config.
func NewServer(configGetter core.KubernetesConfigGetter, kubeappsCluster string, pluginConfigPath string, clientQPS float32, clientBurst int) (*Server, error) {
	log.Infof("+crossplane NewServer(kubeappsCluster: [%v], pluginConfigPath: [%s])",
		kubeappsCluster, pluginConfigPath)

	pluginConfig := common.NewDefaultPluginConfig()
	if pluginConfigPath != "" {
		parsed, err := common.ParsePluginConfig(pluginConfigPath)
		if err != nil {
			return nil, err
		}
		pluginConfig = parsed
		log.Infof("+crossplane using custom config: [%v]", *pluginConfig)
	} else {
		log.Info("+crossplane using default config since pluginConfigPath is empty")
	}

	scheme := runtime.NewScheme()
	if err := apiextensionsv1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("failed to register Crossplane API types: %w", err)
	}

	clientProvider, err := clientgetter.NewClientProvider(configGetter, clientgetter.Options{Scheme: scheme})
	if err != nil {
		return nil, err
	}

	return &Server{
		kubeappsCluster: kubeappsCluster,
		clientGetter:    clientProvider,
		pluginConfig:    pluginConfig,
	}, nil
}

func unimplemented(method string) error {
	return connect.NewError(connect.CodeUnimplemented, fmt.Errorf("%s is not implemented yet", method))
}

func (s *Server) unsupportedCluster(cluster string) error {
	if cluster != "" && cluster != s.kubeappsCluster {
		return connect.NewError(connect.CodeUnimplemented, fmt.Errorf("not supported yet: cluster: [%v]", cluster))
	}
	return nil
}

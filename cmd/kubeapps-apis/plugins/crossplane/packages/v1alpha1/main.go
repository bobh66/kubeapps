// Copyright 2021-2023 the Kubeapps contributors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	pluginsv1alpha1 "github.com/SAP/kubeapps/cmd/kubeapps-apis/core/plugins/v1alpha1"
	pluginsgrpcv1alpha1 "github.com/SAP/kubeapps/cmd/kubeapps-apis/gen/core/plugins/v1alpha1"
	"github.com/SAP/kubeapps/cmd/kubeapps-apis/gen/plugins/crossplane/packages/v1alpha1"
	packagesConnect "github.com/SAP/kubeapps/cmd/kubeapps-apis/gen/plugins/crossplane/packages/v1alpha1/v1alpha1connect"
	"google.golang.org/grpc"
	log "k8s.io/klog/v2"
)

// Set the pluginDetail once during a module init function so the single struct
// can be used throughout the plugin.
var (
	pluginDetail pluginsgrpcv1alpha1.Plugin
	// This version var is updated during the build (see the -ldflags option
	// in the cmd/kubeapps-apis/Dockerfile)
	version = "devel"
)

func init() {
	pluginDetail = pluginsgrpcv1alpha1.Plugin{
		Name:    "crossplane.packages",
		Version: "v1alpha1",
	}
}

// RegisterWithGRPCServer enables a plugin to register with a gRPC server
// returning the server implementation.
//
//nolint:deadcode
func RegisterWithGRPCServer(opts pluginsv1alpha1.GRPCPluginRegistrationOptions) (interface{}, error) {
	log.Info("+crossplane RegisterWithGRPCServer")

	svr, err := NewServer(opts.ConfigGetter, opts.ClustersConfig.KubeappsClusterName, opts.PluginConfigPath, opts.ClientQPS, opts.ClientBurst)
	if err != nil {
		return nil, err
	}
	opts.Mux.Handle(packagesConnect.NewCrossplanePackagesServiceHandler(svr))
	opts.Mux.Handle(packagesConnect.NewCrossplaneRepositoriesServiceHandler(svr))
	return svr, nil
}

// RegisterHTTPHandlerFromEndpoint enables a plugin to register an http
// handler to translate to the gRPC request.
//
//nolint:deadcode
func RegisterHTTPHandlerFromEndpoint(ctx context.Context, mux *runtime.ServeMux, endpoint string, opts []grpc.DialOption) error {
	log.Info("+crossplane RegisterHTTPHandlerFromEndpoint")
	if err := v1alpha1.RegisterCrossplanePackagesServiceHandlerFromEndpoint(ctx, mux, endpoint, opts); err != nil {
		return err
	}
	return v1alpha1.RegisterCrossplaneRepositoriesServiceHandlerFromEndpoint(ctx, mux, endpoint, opts)
}

// GetPluginDetail returns a core.plugins.Plugin describing itself.
func GetPluginDetail() *pluginsgrpcv1alpha1.Plugin {
	return &pluginDetail
}

# Crossplane XRD packaging plugin

The `crossplane.packages` plugin exposes Crossplane `CompositeResourceDefinition` (XRD) objects as Kubeapps packages. Users browse established XRDs in the catalog and deploy claims or composites using the same Helm-style deploy forms (Visual editor + YAML editor).

Kubeapps does **not** install Crossplane and does **not** use package repositories for this plugin.

## Enabling the plugin

Enable the plugin in the Kubeapps chart:

```bash
helm upgrade --install kubeapps oci://ghcr.io/sap/kubeapps/kubeapps \
  --namespace kubeapps \
  --set packaging.crossplane.enabled=true
```

Crossplane can run alongside Helm:

```bash
helm upgrade --install kubeapps oci://ghcr.io/sap/kubeapps/kubeapps \
  --namespace kubeapps \
  --set packaging.helm.enabled=true \
  --set packaging.crossplane.enabled=true
```

Optional plugin configuration is passed via `kubeappsapis.pluginConfig` in chart values (written to `plugins.conf`):

```json
{
  "crossplane": {
    "packages": {
      "v1alpha1": {
        "deployPreference": "claim",
        "labelSelector": "",
        "nameAllowlist": []
      }
    }
  }
}
```

| Setting | Description |
|---------|-------------|
| `deployPreference` | `claim` (default) or `composite` when an XRD defines `spec.claimNames` |
| `labelSelector` | Optional label selector limiting catalog XRDs |
| `nameAllowlist` | Optional list of XRD `metadata.name` values to expose |

## Cluster prerequisites

- Crossplane installed in the cluster
- XRDs in `Established` condition
- User RBAC to list/read XRDs and to create/update/delete the deployed API (claim or composite)

See [chart RBAC notes](https://github.com/SAP/kubeapps/blob/main/chart/kubeapps/README.md#crossplane-xrd-packaging) for example `ClusterRole` rules.

## XRD annotation conventions

| Annotation | Purpose |
|------------|---------|
| `kubeapps.crossplane.io/display-name` | Catalog display name |
| `kubeapps.crossplane.io/icon-url` | Catalog icon URL |
| `kubeapps.crossplane.io/description` | Short/long description (first line used in summaries) |
| `kubeapps.crossplane.io/readme` | Package detail readme |
| `kubeapps.crossplane.io/default-spec` | YAML or JSON pre-fill for deploy form |
| `categories` | Comma-separated catalog categories |

## Form mapping (Helm parity)

The plugin maps each served XRD version's `openAPIV3Schema.properties.spec` to:

- `values_schema` — JSON schema for the Visual editor
- `default_values` — YAML for the YAML editor

Precedence for defaults:

1. `kubeapps.crossplane.io/default-spec` annotation (if set)
2. OpenAPI `default` fields on `spec` properties (including nested objects)

If `spec` has no `properties`, the deploy form falls back to YAML-only (no Visual editor tab).

This mirrors Helm chart `values.schema.json` + `values.yaml`, but sources data from the XRD OpenAPI schema instead of a chart archive.

## Identifiers and lifecycle

| Context | `identifier` | `namespace` |
|---------|--------------|-------------|
| Available package (catalog) | XRD `metadata.name` | empty |
| Installed package | Claim/composite CR name | CR namespace (empty for cluster-scoped composites) |

Create/update operations send deploy form YAML as the CR `.spec` (not wrapped in a `spec:` key in the RPC — the plugin applies the parsed map to `.spec`).

## Resource references (installed app tab)

`GetInstalledPackageResourceRefs` uses the Crossplane CLI trace/xrm resource-tree logic (`github.com/crossplane/cli/v2/cmd/crossplane/common/resource/xrm`) to list composed resources for an installed claim or composite. Users need `get`/`list` on managed resources returned by the trace.

## Plugin layout

```
cmd/kubeapps-apis/plugins/crossplane/packages/v1alpha1/
  catalog.go          # XRD catalog RPCs
  schema.go           # OpenAPI spec → values_schema / default_values
  xrd.go              # XRD helpers, deploy target selection
  release.go          # Installed package CR lifecycle
  status.go           # Condition → InstalledPackageStatus
  resourcerefs.go     # Trace-based resource refs
  repositories.go     # Repository API stubs (no-op)
  testdata/sample-xrd.yaml
```

## Running unit tests

```bash
go test ./cmd/kubeapps-apis/plugins/crossplane/packages/v1alpha1/...
```

Sample XRD testdata lives in `testdata/sample-xrd.yaml` and exercises schema extraction, catalog metadata, and deploy-target selection.

## Building the plugin

The plugin is built as a Go plugin shared object in the `kubeapps-apis` image:

```bash
go build -buildmode=plugin \
  -o crossplane-packages-v1alpha1-plugin.so \
  ./cmd/kubeapps-apis/plugins/crossplane/packages/v1alpha1/*.go
```

See `cmd/kubeapps-apis/Dockerfile` for the production build target.

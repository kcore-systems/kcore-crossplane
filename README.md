# provider-kcore — Crossplane provider for the kcore controller API

Go module: `github.com/kcore/kcore-crossplane`

This repository implements a **Crossplane provider** that reconciles Kubernetes custom resources against the kcore **controller** gRPC API (`Controller` service), using definitions aligned with [`proto/kcore/controller/v1/controller.proto`](proto/kcore/controller/v1/controller.proto) (mirrored from the kcore project).

## Layout

| Path | Purpose |
|------|---------|
| `cmd/provider` | Provider controller manager entrypoint |
| `apis/kcore/v1alpha1` | CRD Go types (`kcore.crossplane.io/v1alpha1`) |
| `apis/apis.go` | Scheme registration |
| `internal/controller/` | Reconcilers: `config`, `sshkey`, `network`, `securitygroup`, `securitygroupattachment`, `vm`, `workload`, plus `kcore` gRPC dial helpers |
| `proto/kcore/controller/v1/` | Protobuf source; `buf.yaml` / `buf.gen.yaml` |
| `gen/proto/...` | Generated Go gRPC + protobuf (`buf generate`) |
| `package/crds/` | Generated CRD YAML (`controller-gen crd`) |
| `examples/cluster/` | Kind + Helm bootstrap for Crossplane and Argo CD |
| `examples/manifests/` | Namespace, ProviderConfig, RBAC, Deployment, sample managed resources |
| `examples/argocd/` | Example Argo CD `Application` (set `repoURL` to your Git remote) |
| `flake.nix` | `nix develop` shell: Go, buf, protobuf, kind, kubectl, helm, argocd, yq, jq |
| `Makefile` | `make generate`, `make build`, `make test` |
| `Dockerfile` | Multi-stage build → distroless image `provider-kcore` |

## API group and resources

**Group:** `kcore.crossplane.io/v1alpha1`

- **Provider configuration:** `ProviderConfig`, `ClusterProviderConfig`, and usage types (`ProviderConfigUsage`, `ClusterProviderConfigUsage`).
- **Managed resources:**
  - `SSHKey` — register SSH public keys
  - `Network` — create/list/delete networks
  - `SecurityGroup` — create/delete security groups
  - `SecurityGroupAttachment` — attach/detach a security group to a VM or network
  - `VirtualMachine` — create/update/delete VMs (`CreateVm`, `UpdateVm`, …)
  - `Workload` — VM or container workloads (`CreateWorkload`, …)

The **`ControllerAdmin`** gRPC service (replication, `ApplyNixConfig`, etc.) is **not** exposed as managed resources in this provider.

## Connection and credentials

`ProviderConfig` specifies:

- **`spec.endpoint`** — controller gRPC address `host:port`
- **`spec.insecure`** — if `true`, use plaintext gRPC (development only)
- **`spec.credentials`** — typically `source: Secret` with a Kubernetes `Secret` containing PEM material:
  - `ca.crt` (optional), `tls.crt`, `tls.key`

The CRD requires `secretRef.key` for schema compatibility; the controller reads **`ca.crt` / `tls.crt` / `tls.key`** from the Secret `Data` (the `key` field is not used to select a single blob).

For plaintext dev, use `insecure: true` and `credentials.source: None`.

## Build

```bash
nix develop   # optional: dev tools
make generate # buf + deepcopy + CRDs
make build    # outputs bin/provider
```

Or: `go build -o bin/provider ./cmd/provider`

## Cluster bootstrap (examples)

Non-interactive script (Kind cluster, Crossplane via Helm, Argo CD):

```bash
./examples/cluster/bootstrap.sh
```

Then install CRDs and wire the provider Deployment (see `examples/manifests/`).

## Docker image

```bash
docker build -t provider-kcore:dev .
kind load docker-image provider-kcore:dev --name "${CLUSTER_NAME:-kcore-crossplane}"
```

## Crossplane CLI

The Nix shell suggests installing the Crossplane CLI (crank) if `crossplane` is not on `PATH`:

```bash
go install github.com/crossplane/crossplane/cmd/crank@latest
```

## Sample manifests

Under `examples/manifests/`:

- `20-providerconfig-insecure.yaml` — dev ProviderConfig
- `30-providerconfig-tls.yaml` — mTLS ProviderConfig (replace Secret contents)
- `41-provider-rbac.yaml` + `42-deployment-provider.yaml` — run the provider in-cluster (tighten RBAC for production)
- `50–55-sample-*.yaml` — one example per managed resource kind (placeholders must match your cluster)

## License

Provider controller code follows the same Apache 2.0 style as upstream Crossplane templates where noted; protobuf-generated files carry their own codegen headers.

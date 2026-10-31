# FlexsHome pipeline demo deployment

This private repository owns the stateless demo Deployment, Service, namespaced
budgets, network policies and dev/UAT/prod Kustomize overlays. Application source
and build CI are in `FlukeSupakorn/flexshome-pipeline-demo`; the trusted
`flexshome-version-control` repository selects an immutable commit from here.

The base deliberately contains `registry.invalid/pipeline-demo:REPLACE_AFTER_VERIFIED_BUILD`.
Rendering and schema checks work, but the release policy and CI reject this image.
It must become `ghcr.io/flukesupakorn/flexshome-pipeline-demo@sha256:<64 lowercase hex>`
only after successful source CI provides the actual digest and source commit.
An invented digest is not release evidence. No deployment has been accepted yet.

## Reuse and application contract

The Go image serves `/healthz`, `/readyz`, and `/version` on port 8080. `APP_ENV` and
`LISTEN_ADDR=:8080` come from an overlay ConfigMap; its generated name changes the
Pod template when config changes. On SIGTERM the app stops being ready and drains
HTTP requests for at most 10 seconds; Kubernetes allows 30 seconds. Probes use HTTP
directly, so the minimal image needs no shell, curl, package manager, or lifecycle
script. The process logs to stdout/stderr and needs no writable filesystem.

Every overlay has one replica; this single-node pattern does not provide HA.
Requests are 25m/32Mi (dev), 50m/48Mi (UAT), 100m/64Mi (prod), with respective limits
250m/128Mi, 250m/128Mi, 500m/192Mi. Shared namespace budgets allow the old and new
Pod to overlap during a rolling update. Measure actual use before tuning them.
Startup, readiness and liveness checks address different failure conditions.

Only dev should be selected by the central GitOps catalog initially. UAT/prod are
validated templates and are inactive until explicitly selected and their
namespaces and external credentials are provisioned. Their Ingress hosts are
internal routing names, not configured DNS, public URLs, or TLS certificates.
The K3s Traefik Service remains internal; public exposure is a separate central
platform change.

## Namespace administrator contract

The central trusted repository/administrator supplies, outside this repository:

- `pipeline-demo-dev` (and later UAT/prod) Namespace with restricted Pod Security
  Admission labels, namespace access controls, and an Argo AppProject limited to
  these namespaced kinds. It must not grant this app Namespace, RBAC, Secret,
  cluster-scoped resources, privileged controllers, or GitOps root authority.
- `ghcr-read`, a `kubernetes.io/dockerconfigjson` Secret in each activated
  namespace, using a separate runtime `read:packages` credential. Job tokens are
  short-lived build credentials and are not runtime pull credentials.
- The read-only Argo repository credential, central manifest revision selection,
  platform controllers, namespace identity labels, monitoring and CI jobs.

This app supplies ResourceQuota/LimitRange only within its dedicated namespace.
No Namespace, Secret value, ServiceAccount, RBAC, custom resource or privileged
setup is included. Use a separate externally supplied Secret reference for a
future app's runtime secrets; never put a secret in ConfigMap literals. Update
the template and its policy deliberately if that workload needs secret envs.

Default-deny ingress/egress applies in each dedicated app namespace. HTTP 8080 is
allowed only from Traefik Pods in `kube-system`, Prometheus Pods in `monitoring`,
and smoke-test Pods in `arc-runners-manifest`. The namespace and Pod selectors
are in the **same peer**, so both must match. CI smoke Pods need the exact label
`app.kubernetes.io/name=pipeline-demo-smoke`; ordinary CI jobs gain no app access.
DNS TCP/UDP 53 is allowed only to CoreDNS in `kube-system`; other app egress is
blocked. Verify actual controller labels and both allowed/denied paths at runtime.
A future Tailscale proxy targeting the Service directly needs a reviewed,
explicit proxy namespace/Pod peer; it is not implicitly allowed here.

This demo has no PVC, volumes or fake persistent mount. A future stateful app
needs its own storage class, capacity, mount permissions, backup/restore evidence,
retention and migration/rollback design before adding a PVC. Removing a workload
and deleting its data must be separate operations.

## Validation and release

Use Go 1.27.3 and the exact tools in `scripts/install-tools.sh`:

```sh
go test ./...
go vet ./...
bash scripts/install-tools.sh
bash scripts/validate.sh
```

The installer verifies pinned official release SHA256 values. Validation renders
all three overlays, checks the vendored Kubernetes 1.36 strict schemas **offline**,
and enforces the app's digest, namespace, health, security, resource, config and
network contracts. An unknown resource kind or absent schema fails closed. CI
uses only the `flexshome-manifest` self-hosted ARC scale set and the pinned Go
container; trusted ARC hooks add restricted job-Pod confinement and inject the
Node runtime for pinned JavaScript actions. CI has no deployment credentials,
repository-writing permission, cluster API access or GitHub mutation steps.

Record the passed source run/commit and image digest in the release review. Change
the exact digest here, validate all overlays, and obtain a passed manifest commit.
Then central release verification selects that **full manifest commit**, and
pins/synchronizes the exact passed central commit. CI never edits this repository
or the central repository. Do not release mutable tags or restart/delete Pods as
a deployment mechanism.

Rollback selects a previously passed immutable manifest commit with its retained
image digest in the central catalog, validates that central commit, and syncs it.
Keep old images and source/release evidence. Runtime acceptance must check rollout,
`/version` matches the selected source commit/environment, both health endpoints,
restricted Pod state, ingress/network policy, and one Pod replacement. Manifests
and a green offline check alone do not prove runtime acceptance.

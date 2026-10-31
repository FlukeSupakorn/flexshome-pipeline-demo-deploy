# Template verification — 2026-10-31

Local preparation only. This is not a passed release, live CI run, private image
pull, or deployment acceptance record.

- `go test -count=1 ./...`: 32 test cases/subtests passed, zero failures.
- `go vet ./...`: passed.
- Actionlint1.7.12 initially rejected job-level `runner.temp` expressions in
  HOME/GOCACHE/GOMODCACHE. Runtime directories now use `$RUNNER_TEMP` in an early
  step and publish standard variables through `$GITHUB_ENV`. Actionlint passes
  with the explicit self-hosted runner label configuration. This is a static
  workflow check; the real ARC job still requires acceptance.
- New release-policy behavior was tested before implementation: 21 unsafe
  release mutations initially failed because the validator accepted them; after
  implementation they passed. Later selector, mixed secret/config source, and
  Ingress backend regression cases also went red before their checks were added.
- Kustomize5.8.1 rendered dev/UAT/prod. Each contained nine resources.
- Kubeconform0.8.0 with only the vendored Kubernetes1.36 standalone strict schemas:
  nine valid, zero invalid/errors/skipped for each environment.
- All seven checked-in schema checksums matched.
- Strict schema validation rejected an unknown Deployment field. A Namespace
  without a checked-in schema failed, confirming there is no missing-schema skip.
- The full `scripts/validate.sh` ran locally with the checksum-verified Windows
  tool builds under Git Bash. It checked all three overlays and returned1 because
  **each committed placeholder image was rejected**. Shell syntax checks passed
  for both scripts. The Linux installer and real ARC job have not run yet.
- Ignored local rendered fixtures with an explicitly synthetic digest passed the
  app policy for all three environments. Those fixtures were not committed and
  are not image evidence. The committed base retains the invalid placeholder.
- Policy tests reject malformed/duplicate YAML, missing resource budgets,
  unresolved/mutable images, cross-namespace/admin resources, unsafe container
  identity/confinement, missing probes/resources/pull secret, mismatched selectors,
  and broad or incorrectly separated network peers.

Remaining gates: install the repository-scoped runner App, obtain the separate
runtime GHCR credential, run source CI and record its real immutable digest,
replace the placeholder through review, pass manifest CI, and have the trusted
central release verifier select this repository's full passed commit. Then prove
actual image pull, admission, health/version, network allow/deny, rollout and
rollback on K3s. Activate dev first; UAT/prod stay unselected.

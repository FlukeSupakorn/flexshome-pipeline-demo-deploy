# Offline schema snapshot

These seven files are the Kubernetes `v1.36.0-standalone-strict` schemas from
[`yannh/kubernetes-json-schema` commit `47163ead3738a996c3e30be9691bb37b8071d9d3`](https://github.com/yannh/kubernetes-json-schema/tree/47163ead3738a996c3e30be9691bb37b8071d9d3/v1.36.0-standalone-strict).
The upstream license is preserved in `LICENSE`. `SHA256SUMS` records the exact
downloaded bytes and is checked before validation. These are generated upstream
schemas, not independent proof of admission or runtime behavior.

Kubeconform uses only this local directory, with strict checking and no missing
schema bypass. A new resource kind requires an explicit reviewed schema update.
The cluster is K3s 1.36.5; Kubernetes patch releases do not require a different API
schema snapshot. Actual admission, Pod security, image pull, networking, and health
still require the deployment acceptance tests.

Tool pins and original checksum sources:

- [Kustomize5.8.1 checksums](https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2Fv5.8.1/checksums.txt)
- [Kubeconform0.8.0 checksums](https://github.com/yannh/kubeconform/releases/download/v0.8.0/CHECKSUMS)
- [Kubeconform schema location documentation](https://github.com/yannh/kubeconform#overriding-schemas-location)

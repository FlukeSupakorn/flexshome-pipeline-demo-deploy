# FlexsHome pipeline demo deployment

This private repository owns the demo's Kubernetes resources, environment overlays and exact OCI image digest. Application source is in `FlukeSupakorn/flexshome-pipeline-demo`. The central `flexshome-version-control` repository selects a full commit from this repository before Argo deploys it.

No plaintext credentials, workstation paths, mutable image tags or manual Pod deletion form part of a release. Kubernetes namespace/access/bootstrap resources belong to the central platform repository. Deployment templates are added and validated in platform task05; this initial commit establishes the repository boundary only.

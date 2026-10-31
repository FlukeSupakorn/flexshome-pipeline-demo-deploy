package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// This fixture represents an independent release contract, rather than copying
// implementation defaults. Mutations exercise rejected release behavior.
func validObjects(t *testing.T) []object {
	t.Helper()
	var objects []object
	if err := json.Unmarshal([]byte(`[
{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"pipeline-demo","namespace":"pipeline-demo-dev"},"spec":{"replicas":1,"selector":{"matchLabels":{"app.kubernetes.io/name":"pipeline-demo"}},"template":{"metadata":{"labels":{"app.kubernetes.io/name":"pipeline-demo"}},"spec":{"automountServiceAccountToken":false,"terminationGracePeriodSeconds":30,"imagePullSecrets":[{"name":"ghcr-read"}],"securityContext":{"runAsNonRoot":true,"runAsUser":65532,"runAsGroup":65532,"seccompProfile":{"type":"RuntimeDefault"},"appArmorProfile":{"type":"RuntimeDefault"}},"containers":[{"name":"server","image":"ghcr.io/flukesupakorn/flexshome-pipeline-demo@sha256:abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd","ports":[{"name":"http","containerPort":8080}],"envFrom":[{"configMapRef":{"name":"pipeline-demo-config"}}],"resources":{"requests":{"cpu":"25m","memory":"32Mi"},"limits":{"cpu":"250m","memory":"128Mi"}},"securityContext":{"allowPrivilegeEscalation":false,"readOnlyRootFilesystem":true,"capabilities":{"drop":["ALL"]}},"startupProbe":{"httpGet":{"path":"/healthz","port":"http"},"periodSeconds":2,"failureThreshold":30},"readinessProbe":{"httpGet":{"path":"/readyz","port":"http"},"periodSeconds":3,"failureThreshold":2},"livenessProbe":{"httpGet":{"path":"/healthz","port":"http"},"periodSeconds":10,"failureThreshold":3}}]}}}},
{"apiVersion":"v1","kind":"Service","metadata":{"name":"pipeline-demo","namespace":"pipeline-demo-dev"},"spec":{"type":"ClusterIP","selector":{"app.kubernetes.io/name":"pipeline-demo"},"ports":[{"name":"http","port":80,"targetPort":"http"}]}},
{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"pipeline-demo-config","namespace":"pipeline-demo-dev"},"data":{"APP_ENV":"dev","LISTEN_ADDR":":8080"}},
{"apiVersion":"v1","kind":"ResourceQuota","metadata":{"name":"pipeline-demo","namespace":"pipeline-demo-dev"},"spec":{"hard":{"requests.cpu":"200m","requests.memory":"256Mi","limits.cpu":"1","limits.memory":"512Mi","pods":"3","services":"2","count/persistentvolumeclaims":"0"}}},
{"apiVersion":"v1","kind":"LimitRange","metadata":{"name":"pipeline-demo","namespace":"pipeline-demo-dev"},"spec":{"limits":[{"type":"Container","defaultRequest":{"cpu":"25m","memory":"32Mi"},"default":{"cpu":"250m","memory":"128Mi"}}]}},
{"apiVersion":"networking.k8s.io/v1","kind":"NetworkPolicy","metadata":{"name":"default-deny","namespace":"pipeline-demo-dev"},"spec":{"podSelector":{},"policyTypes":["Ingress","Egress"]}},
{"apiVersion":"networking.k8s.io/v1","kind":"NetworkPolicy","metadata":{"name":"allow-http","namespace":"pipeline-demo-dev"},"spec":{"podSelector":{"matchLabels":{"app.kubernetes.io/name":"pipeline-demo"}},"policyTypes":["Ingress"],"ingress":[{"ports":[{"protocol":"TCP","port":8080}],"from":[{"namespaceSelector":{"matchLabels":{"kubernetes.io/metadata.name":"kube-system"}},"podSelector":{"matchLabels":{"app.kubernetes.io/name":"traefik"}}},{"namespaceSelector":{"matchLabels":{"kubernetes.io/metadata.name":"monitoring"}},"podSelector":{"matchLabels":{"app.kubernetes.io/name":"prometheus"}}},{"namespaceSelector":{"matchLabels":{"kubernetes.io/metadata.name":"arc-runners-manifest"}},"podSelector":{"matchLabels":{"app.kubernetes.io/name":"pipeline-demo-smoke"}}}]}]}},
{"apiVersion":"networking.k8s.io/v1","kind":"NetworkPolicy","metadata":{"name":"allow-dns","namespace":"pipeline-demo-dev"},"spec":{"podSelector":{"matchLabels":{"app.kubernetes.io/name":"pipeline-demo"}},"policyTypes":["Egress"],"egress":[{"to":[{"namespaceSelector":{"matchLabels":{"kubernetes.io/metadata.name":"kube-system"}},"podSelector":{"matchLabels":{"k8s-app":"kube-dns"}}}],"ports":[{"protocol":"UDP","port":53},{"protocol":"TCP","port":53}]}]}}
]`), &objects); err != nil {
		t.Fatal(err)
	}
	return objects
}

func find(objects []object, kind, name string) object {
	for _, o := range objects {
		if o["kind"] == kind && get(o, "metadata", "name") == name {
			return o
		}
	}
	panic("test fixture missing " + kind + "/" + name)
}

func TestValidRelease(t *testing.T) {
	if err := validate(validObjects(t), "dev"); err != nil {
		t.Fatal(err)
	}
}

func TestRejectsUnsafeRelease(t *testing.T) {
	cases := []struct {
		name, want string
		change     func([]object)
	}{
		{"unresolved image", "digest", func(o []object) { container(o)["image"] = "ghcr.io/REPLACE_AFTER_VERIFIED_BUILD" }},
		{"synthetic zero digest", "digest", func(o []object) {
			container(o)["image"] = "ghcr.io/flukesupakorn/flexshome-pipeline-demo@sha256:" + strings.Repeat("0", 64)
		}},
		{"mutable tag", "digest", func(o []object) { container(o)["image"] = "ghcr.io/flukesupakorn/flexshome-pipeline-demo:latest" }},
		{"service account token", "automount", func(o []object) { pod(o)["automountServiceAccountToken"] = true }},
		{"root user", "security", func(o []object) { pod(o)["securityContext"].(map[string]any)["runAsUser"] = float64(0) }},
		{"unconfined apparmor", "security", func(o []object) {
			pod(o)["securityContext"].(map[string]any)["appArmorProfile"] = object{"type": "Unconfined"}
		}},
		{"writable root", "security", func(o []object) { container(o)["securityContext"].(map[string]any)["readOnlyRootFilesystem"] = false }},
		{"privileged", "security", func(o []object) { container(o)["securityContext"].(map[string]any)["privileged"] = true }},
		{"extra sidecar", "one container", func(o []object) {
			pod(o)["containers"] = append(pod(o)["containers"].([]any), object{"name": "escape"})
		}},
		{"host mount", "volume", func(o []object) { pod(o)["volumes"] = []any{object{"name": "host", "hostPath": object{"path": "/"}}} }},
		{"missing limit", "resources", func(o []object) { delete(container(o)["resources"].(map[string]any), "limits") }},
		{"wrong readiness path", "readinessProbe", func(o []object) {
			container(o)["readinessProbe"].(map[string]any)["httpGet"] = object{"path": "/", "port": "http"}
		}},
		{"missing private registry secret", "ghcr-read", func(o []object) { delete(pod(o), "imagePullSecrets") }},
		{"wrong config environment", "APP_ENV", func(o []object) {
			find(o, "ConfigMap", "pipeline-demo-config")["data"].(map[string]any)["APP_ENV"] = "prod"
		}},
		{"cross namespace resource", "namespace", func(o []object) { o[0]["metadata"].(map[string]any)["namespace"] = "kube-system" }},
		{"embedded secret", "kind", func(o []object) { o[2]["kind"] = "Secret" }},
		{"nodeport", "ClusterIP", func(o []object) { find(o, "Service", "pipeline-demo")["spec"].(map[string]any)["type"] = "NodePort" }},
		{"broad ingress peer", "NetworkPolicy", func(o []object) { httpRule(o)["from"] = []any{object{"namespaceSelector": object{}}} }},
		{"split namespace and pod peers", "NetworkPolicy", func(o []object) {
			httpRule(o)["from"] = []any{object{"namespaceSelector": object{"matchLabels": object{"kubernetes.io/metadata.name": "kube-system"}}}, object{"podSelector": object{"matchLabels": object{"app.kubernetes.io/name": "traefik"}}}}
		}},
		{"missing quota", "ResourceQuota", func(o []object) { o[3]["kind"] = "Ingress"; o[3]["apiVersion"] = "networking.k8s.io/v1" }},
		{"mismatched deployment selector", "selector", func(o []object) {
			o[0]["spec"].(map[string]any)["selector"] = object{"matchLabels": object{"app.kubernetes.io/name": "wrong"}}
		}},
		{"mismatched pod label", "selector", func(o []object) {
			get(o[0], "spec", "template", "metadata").(map[string]any)["labels"] = object{"app.kubernetes.io/name": "wrong"}
		}},
		{"additional selector expression", "selector", func(o []object) {
			get(o[0], "spec", "selector").(map[string]any)["matchExpressions"] = []any{object{"key": "missing", "operator": "Exists"}}
		}},
		{"mixed config and secret source", "environment", func(o []object) {
			get(container(o), "envFrom").([]any)[0].(map[string]any)["secretRef"] = object{"name": "unexpected"}
		}},
		{"container root override", "security", func(o []object) { container(o)["securityContext"].(map[string]any)["runAsUser"] = float64(0) }},
		{"request exceeds limit", "resources", func(o []object) { get(container(o), "resources", "requests").(map[string]any)["cpu"] = "2" }},
		{"split dns peers", "NetworkPolicy", func(o []object) {
			rule := get(find(o, "NetworkPolicy", "allow-dns"), "spec", "egress").([]any)[0].(map[string]any)
			rule["to"] = []any{object{"namespaceSelector": object{}}}
		}},
		{"duplicate identity", "duplicate", func(o []object) { o[2]["metadata"] = o[1]["metadata"]; o[2]["kind"] = o[1]["kind"] }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			o := validObjects(t)
			c.change(o)
			err := validate(o, "dev")
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("want rejection containing %q; got %v", c.want, err)
			}
		})
	}
}

func TestIngressRoutesOnlyInternalService(t *testing.T) {
	o := validObjects(t)
	ingress := object{"apiVersion": "networking.k8s.io/v1", "kind": "Ingress", "metadata": object{"name": "pipeline-demo", "namespace": "pipeline-demo-dev"}, "spec": object{"ingressClassName": "traefik", "rules": []any{object{"host": "pipeline-demo.dev.flexshome.internal", "http": object{"paths": []any{object{"path": "/", "pathType": "Prefix", "backend": object{"service": object{"name": "other-service", "port": object{"name": "http"}}}}}}}}}}
	o = append(o, ingress)
	if err := validate(o, "dev"); err == nil || !strings.Contains(err.Error(), "Ingress") {
		t.Fatalf("wrong Ingress service must reject: %v", err)
	}
	get(ingress, "spec", "rules").([]any)[0].(map[string]any)["http"] = object{"paths": []any{object{"path": "/", "pathType": "Prefix", "backend": object{"service": object{"name": "pipeline-demo", "port": object{"name": "http"}}}}}}
	if err := validate(o, "dev"); err != nil {
		t.Fatalf("internal app Ingress must pass: %v", err)
	}
}

func TestDecodeRejectsMalformedAndDuplicateYAML(t *testing.T) {
	for _, data := range []string{"kind: [", "kind: Service\nkind: Secret\n", "---\n"} {
		if _, err := decode([]byte(data)); err == nil {
			t.Fatalf("must reject malformed/empty YAML %q", data)
		}
	}
	objects, err := decode([]byte("kind: Service\n---\nkind: ConfigMap\n"))
	if err != nil || len(objects) != 2 {
		t.Fatalf("decode resource stream: %d %v", len(objects), err)
	}
}

func pod(o []object) object       { return get(o[0], "spec", "template", "spec").(map[string]any) }
func container(o []object) object { return pod(o)["containers"].([]any)[0].(map[string]any) }
func httpRule(o []object) object {
	return get(find(o, "NetworkPolicy", "allow-http"), "spec", "ingress").([]any)[0].(map[string]any)
}

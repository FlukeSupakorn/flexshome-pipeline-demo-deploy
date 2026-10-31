package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type object = map[string]any

func get(v any, keys ...string) any {
	for _, key := range keys {
		m, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = m[key]
	}
	return v
}

func same(a, b any) bool  { x, _ := json.Marshal(a); y, _ := json.Marshal(b); return bytes.Equal(x, y) }
func positive(v any) bool { n, err := strconv.Atoi(fmt.Sprint(v)); return err == nil && n > 0 }
func empty(v any) bool    { return v == nil || same(v, []any{}) || same(v, object{}) }

func decode(data []byte) ([]object, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var objects []object
	for {
		var o object
		err := decoder.Decode(&o)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("malformed YAML: %w", err)
		}
		if len(o) > 0 {
			objects = append(objects, o)
		}
	}
	if len(objects) == 0 {
		return nil, errors.New("manifest has no resources")
	}
	return objects, nil
}

var imagePattern = regexp.MustCompile(`^ghcr\.io/flukesupakorn/flexshome-pipeline-demo@sha256:[a-f0-9]{64}$`)
var quantityPattern = regexp.MustCompile(`^([1-9][0-9]*)(m|Mi|Gi)?$`)

func quantity(v any, cpu bool) int64 {
	m := quantityPattern.FindStringSubmatch(fmt.Sprint(v))
	if m == nil {
		return 0
	}
	n, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil || n > 1000000000 {
		return 0
	}
	if cpu {
		if m[2] == "m" {
			return n
		}
		if m[2] == "" {
			return n * 1000
		}
	} else {
		if m[2] == "Mi" {
			return n
		}
		if m[2] == "Gi" {
			return n * 1024
		}
	}
	return 0
}

func validate(objects []object, environment string) error {
	if environment != "dev" && environment != "uat" && environment != "prod" {
		return errors.New("environment must be dev, uat, or prod")
	}
	namespace := "pipeline-demo-" + environment
	allowed := map[string]string{"Deployment": "apps/v1", "Service": "v1", "ConfigMap": "v1", "ResourceQuota": "v1", "LimitRange": "v1", "NetworkPolicy": "networking.k8s.io/v1", "Ingress": "networking.k8s.io/v1"}
	seen := map[string]bool{}
	counts := map[string]int{}
	for _, o := range objects {
		kind, _ := o["kind"].(string)
		if allowed[kind] == "" || o["apiVersion"] != allowed[kind] {
			return fmt.Errorf("kind/apiVersion outside app allowlist: %v/%v", o["apiVersion"], o["kind"])
		}
		if get(o, "metadata", "namespace") != namespace {
			return fmt.Errorf("resource namespace must be %s", namespace)
		}
		name, _ := get(o, "metadata", "name").(string)
		if name == "" {
			return errors.New("resource needs metadata.name")
		}
		id := kind + "/" + name
		if seen[id] {
			return fmt.Errorf("duplicate resource %s", id)
		}
		seen[id] = true
		counts[kind]++
	}
	for _, kind := range []string{"Deployment", "Service", "ConfigMap", "ResourceQuota", "LimitRange"} {
		if counts[kind] != 1 {
			return fmt.Errorf("exactly one %s required", kind)
		}
	}
	if counts["NetworkPolicy"] != 3 || counts["Ingress"] > 1 {
		return errors.New("exactly three NetworkPolicy resources and at most one Ingress required")
	}
	for _, o := range objects {
		spec := get(o, "spec")
		switch o["kind"] {
		case "Deployment":
			selector := object{"app.kubernetes.io/name": "pipeline-demo"}
			if !same(get(spec, "selector"), object{"matchLabels": selector}) || get(o, "spec", "template", "metadata", "labels", "app.kubernetes.io/name") != "pipeline-demo" {
				return errors.New("Deployment selector must match pipeline-demo pod label")
			}
			p := get(o, "spec", "template", "spec")
			c, ok := get(p, "containers").([]any)
			if !ok || len(c) != 1 {
				return errors.New("Deployment requires one container")
			}
			container := c[0]
			if get(p, "automountServiceAccountToken") != false {
				return errors.New("automountServiceAccountToken must be false")
			}
			if !same(get(p, "imagePullSecrets"), []object{{"name": "ghcr-read"}}) {
				return errors.New("imagePullSecrets must reference ghcr-read")
			}
			if get(p, "hostNetwork") == true || get(p, "hostPID") == true || get(p, "hostIPC") == true || !empty(get(p, "initContainers")) || !empty(get(p, "ephemeralContainers")) {
				return errors.New("pod security forbids host namespaces or additional containers")
			}
			if !empty(get(p, "volumes")) || !empty(get(container, "volumeMounts")) {
				return errors.New("stateless demo forbids volumes")
			}
			s := get(p, "securityContext")
			if get(s, "runAsNonRoot") != true || fmt.Sprint(get(s, "runAsUser")) != "65532" || fmt.Sprint(get(s, "runAsGroup")) != "65532" || !same(get(s, "seccompProfile"), object{"type": "RuntimeDefault"}) || !same(get(s, "appArmorProfile"), object{"type": "RuntimeDefault"}) {
				return errors.New("pod security requires UID/GID65532 and RuntimeDefault seccomp/AppArmor")
			}
			s = get(container, "securityContext")
			if get(s, "allowPrivilegeEscalation") != false || get(s, "readOnlyRootFilesystem") != true || get(s, "privileged") == true || !same(get(s, "capabilities"), object{"drop": []string{"ALL"}}) || get(s, "runAsUser") != nil || get(s, "runAsGroup") != nil || get(s, "seccompProfile") != nil || get(s, "appArmorProfile") != nil {
				return errors.New("container security requires no escalation, read-only root, all capabilities dropped, and no identity/profile override")
			}
			image, _ := get(container, "image").(string)
			if !imagePattern.MatchString(image) || strings.HasSuffix(image, strings.Repeat("0", 64)) {
				return errors.New("image requires verified GHCR sha256 digest; unresolved placeholders forbidden")
			}
			if !same(get(container, "ports"), []object{{"name": "http", "containerPort": 8080}}) {
				return errors.New("container must expose named http port8080")
			}
			for _, resource := range []string{"cpu", "memory"} {
				r := quantity(get(container, "resources", "requests", resource), resource == "cpu")
				l := quantity(get(container, "resources", "limits", resource), resource == "cpu")
				if r == 0 || l == 0 || r > l {
					return errors.New("resources require positive CPU/memory requests <= limits")
				}
			}
			for probe, path := range map[string]string{"startupProbe": "/healthz", "readinessProbe": "/readyz", "livenessProbe": "/healthz"} {
				v := get(container, probe)
				if !same(get(v, "httpGet"), object{"path": path, "port": "http"}) || !positive(get(v, "periodSeconds")) || !positive(get(v, "failureThreshold")) {
					return fmt.Errorf("%s requires bounded HTTP%s on http", probe, path)
				}
			}
			grace, _ := strconv.Atoi(fmt.Sprint(get(p, "terminationGracePeriodSeconds")))
			if grace < 15 || grace > 60 || get(container, "lifecycle") != nil {
				return errors.New("shutdown requires 15-60s grace and direct SIGTERM without lifecycle shell hooks")
			}
			refs, ok := get(container, "envFrom").([]any)
			if !ok || len(refs) != 1 {
				return errors.New("envFrom requires one generated ConfigMap")
			}
			ref := get(refs[0], "configMapRef", "name")
			found := false
			for _, cm := range objects {
				if cm["kind"] == "ConfigMap" && get(cm, "metadata", "name") == ref {
					found = true
				}
			}
			if !found || !same(refs[0], object{"configMapRef": object{"name": ref}}) || get(container, "env") != nil {
				return errors.New("environment must use the generated ConfigMap without env overrides")
			}
		case "ConfigMap":
			if get(o, "data", "APP_ENV") != environment || get(o, "data", "LISTEN_ADDR") != ":8080" {
				return errors.New("ConfigMap requires matching APP_ENV and LISTEN_ADDR=:8080")
			}
		case "Service":
			if get(spec, "type") != "ClusterIP" || !empty(get(spec, "externalIPs")) || !same(get(spec, "selector"), object{"app.kubernetes.io/name": "pipeline-demo"}) || !same(get(spec, "ports"), []object{{"name": "http", "port": 80, "targetPort": "http"}}) {
				return errors.New("Service requires internal ClusterIP port80 -> http and app selector")
			}
		case "ResourceQuota":
			for _, key := range []string{"requests.cpu", "requests.memory", "limits.cpu", "limits.memory", "pods", "services"} {
				if get(spec, "hard", key) == nil {
					return errors.New("ResourceQuota lacks required app budget")
				}
			}
			if fmt.Sprint(get(spec, "hard", "count/persistentvolumeclaims")) != "0" {
				return errors.New("ResourceQuota must prohibit stateless demo PVCs")
			}
		case "LimitRange":
			limits, ok := get(spec, "limits").([]any)
			if !ok || len(limits) != 1 || get(limits[0], "type") != "Container" {
				return errors.New("LimitRange needs Container defaults")
			}
			for _, key := range []string{"cpu", "memory"} {
				if quantity(get(limits[0], "default", key), key == "cpu") == 0 || quantity(get(limits[0], "defaultRequest", key), key == "cpu") == 0 {
					return errors.New("LimitRange needs bounded CPU/memory defaults")
				}
			}
		case "NetworkPolicy":
			if err := networkPolicy(o); err != nil {
				return err
			}
		case "Ingress":
			expected := object{"ingressClassName": "traefik", "rules": []object{{"host": "pipeline-demo." + environment + ".flexshome.internal", "http": object{"paths": []object{{"path": "/", "pathType": "Prefix", "backend": object{"service": object{"name": "pipeline-demo", "port": object{"name": "http"}}}}}}}}}
			if !same(spec, expected) {
				return errors.New("Ingress requires environment-specific internal host, Traefik class, and pipeline-demo HTTP service")
			}
		}
	}
	for _, name := range []string{"default-deny", "allow-http", "allow-dns"} {
		if !seen["NetworkPolicy/"+name] {
			return fmt.Errorf("NetworkPolicy/%s required", name)
		}
	}
	return nil
}

func networkPolicy(o object) error {
	app := object{"matchLabels": object{"app.kubernetes.io/name": "pipeline-demo"}}
	peer := func(ns, key, value string) object {
		return object{"namespaceSelector": object{"matchLabels": object{"kubernetes.io/metadata.name": ns}}, "podSelector": object{"matchLabels": object{key: value}}}
	}
	var expected object
	switch get(o, "metadata", "name") {
	case "default-deny":
		expected = object{"podSelector": object{}, "policyTypes": []string{"Ingress", "Egress"}}
	case "allow-http":
		expected = object{"podSelector": app, "policyTypes": []string{"Ingress"}, "ingress": []object{{"ports": []object{{"protocol": "TCP", "port": 8080}}, "from": []object{peer("kube-system", "app.kubernetes.io/name", "traefik"), peer("monitoring", "app.kubernetes.io/name", "prometheus"), peer("arc-runners-manifest", "app.kubernetes.io/name", "pipeline-demo-smoke")}}}}
	case "allow-dns":
		expected = object{"podSelector": app, "policyTypes": []string{"Egress"}, "egress": []object{{"to": []object{peer("kube-system", "k8s-app", "kube-dns")}, "ports": []object{{"protocol": "UDP", "port": 53}, {"protocol": "TCP", "port": 53}}}}}
	default:
		return errors.New("unexpected NetworkPolicy")
	}
	if !same(get(o, "spec"), expected) {
		return fmt.Errorf("NetworkPolicy/%v must use exact namespace AND pod selectors and bounded ports", get(o, "metadata", "name"))
	}
	return nil
}

func main() {
	environment := flag.String("environment", "", "dev, uat, or prod")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: validate -environment dev rendered.yaml")
		os.Exit(2)
	}
	data, err := os.ReadFile(flag.Arg(0))
	if err == nil {
		var objects []object
		objects, err = decode(data)
		if err == nil {
			err = validate(objects, *environment)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("%s: app policy passed\n", *environment)
}

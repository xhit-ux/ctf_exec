package repository

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/ctf_exec/backend/internal/config"
	corev1 "k8s.io/api/core/v1"
)

func kubectlArgs(cfg *config.Config, args ...string) []string {
	result := make([]string, 0, len(args)+2)
	if cfg != nil && strings.TrimSpace(cfg.Container.KubeconfigPath) != "" {
		result = append(result, "--kubeconfig", strings.TrimSpace(cfg.Container.KubeconfigPath))
	}
	result = append(result, args...)
	return result
}

func runKubectl(cfg *config.Config, args ...string) (string, error) {
	return runCommand("kubectl", kubectlArgs(cfg, args...)...)
}

func ensureChallengeNamespace(cfg *config.Config, namespace string) error {
	ns := strings.TrimSpace(namespace)
	if ns == "" {
		return fmt.Errorf("namespace 不能为空")
	}
	if _, err := runKubectl(cfg, "get", "namespace", ns); err == nil {
		return nil
	}
	if output, err := runKubectl(cfg, "create", "namespace", ns); err != nil {
		return fmt.Errorf("%s", sanitizeCommandOutput(output, err))
	}
	return nil
}

func applyManifest(cfg *config.Config, manifest string) (string, error) {
	tmpFile, err := os.CreateTemp("", "ctf-k8s-*.yaml")
	if err != nil {
		return "", err
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.WriteString(manifest); err != nil {
		_ = tmpFile.Close()
		return "", err
	}
	if err := tmpFile.Close(); err != nil {
		return "", err
	}

	return runKubectl(cfg, "apply", "-f", tmpPath)
}

func deleteResourceIgnoreNotFound(cfg *config.Config, namespace, kind, name string) error {
	if strings.TrimSpace(namespace) == "" || strings.TrimSpace(kind) == "" || strings.TrimSpace(name) == "" {
		return nil
	}
	if output, err := runKubectl(cfg, "delete", kind, name, "-n", namespace, "--ignore-not-found=true"); err != nil {
		return fmt.Errorf("%s", sanitizeCommandOutput(output, err))
	}
	return nil
}

func getServiceNodePort(cfg *config.Config, namespace, serviceName string) (int, error) {
	output, err := runKubectl(cfg, "get", "service", serviceName, "-n", namespace, "-o", "jsonpath={.spec.ports[0].nodePort}")
	if err != nil {
		return 0, fmt.Errorf("%s", sanitizeCommandOutput(output, err))
	}
	port, parseErr := strconv.Atoi(strings.TrimSpace(output))
	if parseErr != nil || port <= 0 {
		return 0, fmt.Errorf("无效的 nodePort: %s", strings.TrimSpace(output))
	}
	return port, nil
}

func renderIngressHost(problemTpl, defaultTpl, instanceID, problemID, userID, podName string) string {
	tpl := strings.TrimSpace(defaultTpl)
	if tpl == "" {
		tpl = strings.TrimSpace(problemTpl)
	}
	if tpl == "" {
		return ""
	}
	replacer := strings.NewReplacer(
		"{id}", shortToken(instanceID),
		"{instance_id}", shortToken(instanceID),
		"{problem_id}", shortToken(problemID),
		"{user_id}", shortToken(userID),
		"{pod_name}", sanitizeK8sName(podName, 63),
	)
	return strings.TrimSpace(replacer.Replace(tpl))
}

func buildChallengeNamespace(prefix, userID string) string {
	cleanPrefix := sanitizeK8sName(prefix, 30)
	if cleanPrefix == "" {
		cleanPrefix = "ctf"
	}
	name := cleanPrefix + "-" + shortToken(userID)
	return sanitizeK8sName(name, 63)
}

func buildChallengePodName(problemID, userID string) string {
	name := fmt.Sprintf("chal-%s-%s", shortToken(problemID), shortToken(userID))
	return sanitizeK8sName(name, 63)
}

func shortToken(input string) string {
	clean := strings.ToLower(strings.TrimSpace(input))
	clean = strings.ReplaceAll(clean, "-", "")
	b := strings.Builder{}
	for _, ch := range clean {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') {
			b.WriteRune(ch)
		}
	}
	out := b.String()
	if out == "" {
		return "x0000000"
	}
	if len(out) > 8 {
		return out[:8]
	}
	for len(out) < 8 {
		out += "0"
	}
	return out
}

func sanitizeK8sName(input string, maxLen int) string {
	v := strings.ToLower(strings.TrimSpace(input))
	b := strings.Builder{}
	lastDash := false
	for _, ch := range v {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') {
			b.WriteRune(ch)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteRune('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "challenge"
	}
	if maxLen > 0 && len(out) > maxLen {
		out = strings.Trim(out[:maxLen], "-")
		if out == "" {
			out = "challenge"
		}
	}
	return out
}

func parseContainerEnv(raw json.RawMessage) [][2]string {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "" || strings.TrimSpace(string(raw)) == "null" {
		return nil
	}

	keyValues := make([][2]string, 0)
	obj := map[string]any{}
	if err := json.Unmarshal(raw, &obj); err == nil {
		keys := make([]string, 0, len(obj))
		for k := range obj {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			key := strings.TrimSpace(k)
			if key == "" {
				continue
			}
			keyValues = append(keyValues, [2]string{key, fmt.Sprintf("%v", obj[k])})
		}
		return keyValues
	}

	arr := make([]map[string]any, 0)
	if err := json.Unmarshal(raw, &arr); err == nil {
		for _, item := range arr {
			name := strings.TrimSpace(fmt.Sprintf("%v", item["name"]))
			if name == "" {
				continue
			}
			value := ""
			if v, ok := item["value"]; ok {
				value = fmt.Sprintf("%v", v)
			}
			keyValues = append(keyValues, [2]string{name, value})
		}
	}

	return keyValues
}

func convertEnv(raw json.RawMessage) []corev1.EnvVar {
	pairs := parseContainerEnv(raw)
	if len(pairs) == 0 {
		return nil
	}
	envVars := make([]corev1.EnvVar, 0, len(pairs))
	for _, p := range pairs {
		envVars = append(envVars, corev1.EnvVar{Name: p[0], Value: p[1]})
	}
	return envVars
}

func buildChallengePodManifest(namespace, podName string, spec *problemContainerRuntimeSpec, cfg *config.Config) (string, error) {
	if spec == nil {
		return "", fmt.Errorf("容器配置为空")
	}
	imageRef := strings.TrimSpace(spec.ImageName) + ":" + strings.TrimSpace(spec.ImageTag)

	envItems := parseContainerEnv(spec.Env)
	envSection := ""
	if len(envItems) > 0 {
		b := strings.Builder{}
		b.WriteString("      env:\n")
		for _, pair := range envItems {
			b.WriteString(fmt.Sprintf("        - name: %s\n", pair[0]))
			b.WriteString(fmt.Sprintf("          value: %q\n", pair[1]))
		}
		envSection = b.String()
	}

	resourceSection := ""
	if strings.TrimSpace(spec.CPULimit) != "" || strings.TrimSpace(spec.MemoryLimit) != "" {
		b := strings.Builder{}
		b.WriteString("      resources:\n")
		b.WriteString("        limits:\n")
		if strings.TrimSpace(spec.CPULimit) != "" {
			b.WriteString(fmt.Sprintf("          cpu: %q\n", strings.TrimSpace(spec.CPULimit)))
		}
		if strings.TrimSpace(spec.MemoryLimit) != "" {
			b.WriteString(fmt.Sprintf("          memory: %q\n", strings.TrimSpace(spec.MemoryLimit)))
		}
		resourceSection = b.String()
	}

	runtimeClassSection := ""
	if spec.UseKata && cfg != nil && strings.TrimSpace(cfg.Container.KataRuntimeClass) != "" {
		runtimeClassSection = fmt.Sprintf("  runtimeClassName: %s\n", strings.TrimSpace(cfg.Container.KataRuntimeClass))
	}

	manifest := fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  name: %s
  namespace: %s
  labels:
    app: %s
spec:
%s  restartPolicy: Never
  containers:
    - name: challenge
      image: %s
      imagePullPolicy: Always
      ports:
        - containerPort: %d
%s%s`, podName, namespace, podName, runtimeClassSection, imageRef, spec.ChallengePort, envSection, resourceSection)

	return manifest, nil
}

func buildChallengeServiceManifest(namespace, serviceName, podName string, spec *problemContainerRuntimeSpec) string {
	serviceType := "ClusterIP"
	servicePort := spec.ChallengePort
	if spec != nil && spec.ProblemType == "container_tcp" {
		serviceType = "NodePort"
	} else {
		servicePort = 80
	}

	return fmt.Sprintf(`apiVersion: v1
kind: Service
metadata:
  name: %s
  namespace: %s
spec:
  type: %s
  selector:
    app: %s
  ports:
    - protocol: TCP
      port: %d
      targetPort: %d
`, serviceName, namespace, serviceType, podName, servicePort, spec.ChallengePort)
}

func buildChallengeIngressManifest(namespace, ingressName, serviceName, host, ingressClass string, tlsEnabled bool, tlsSecret string) string {
	hostBlock := ""
	if strings.TrimSpace(host) != "" {
		hostBlock = fmt.Sprintf("    - host: %s\n", strings.TrimSpace(host))
	} else {
		hostBlock = "    -\n"
	}

	ingressClassSection := ""
	if strings.TrimSpace(ingressClass) != "" {
		ingressClassSection = fmt.Sprintf("  ingressClassName: %s\n", strings.TrimSpace(ingressClass))
	}

	tlsSection := ""
	if tlsEnabled && strings.TrimSpace(host) != "" {
		tlsSection = "  tls:\n"
		tlsSection += fmt.Sprintf("    - hosts:\n        - %s\n", strings.TrimSpace(host))
		if strings.TrimSpace(tlsSecret) != "" {
			tlsSection += fmt.Sprintf("      secretName: %s\n", strings.TrimSpace(tlsSecret))
		}
	}

	return fmt.Sprintf(`apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: %s
  namespace: %s
spec:
%s%s  rules:
%s      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: %s
                port:
                  number: 80
`, ingressName, namespace, ingressClassSection, tlsSection, hostBlock, serviceName)
}

func (r *ProblemRepository) resolveNodeHost() string {
	host := strings.TrimSpace(r.cfg.Container.InfraNodeName)
	if host != "" {
		return host
	}
	name, err := os.Hostname()
	if err == nil && strings.TrimSpace(name) != "" {
		return strings.TrimSpace(name)
	}
	return "127.0.0.1"
}

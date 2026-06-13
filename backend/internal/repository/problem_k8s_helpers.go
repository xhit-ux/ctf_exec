package repository

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/ctf_exec/backend/internal/config"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

func buildChallengePod(namespace, podName string, spec *problemContainerRuntimeSpec, cfg *config.Config, extraEnv []corev1.EnvVar) (*corev1.Pod, error) {
	if spec == nil {
		return nil, fmt.Errorf("容器配置为空")
	}
	imageRef := strings.TrimSpace(spec.ImageName) + ":" + strings.TrimSpace(spec.ImageTag)

	envVars := convertEnv(spec.Env)
	envVars = mergeEnvVars(envVars, extraEnv)

	container := corev1.Container{
		Name:            "challenge",
		Image:           imageRef,
		ImagePullPolicy: corev1.PullAlways,
		Ports: []corev1.ContainerPort{
			{ContainerPort: int32(spec.ChallengePort), Protocol: corev1.ProtocolTCP},
		},
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: ptr.To(false),
			Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
			ReadOnlyRootFilesystem:   ptr.To(true),
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "tmp", MountPath: "/tmp"},
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt(spec.ChallengePort)},
			},
			InitialDelaySeconds: 10,
			PeriodSeconds:       10,
			FailureThreshold:    3,
		},
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt(spec.ChallengePort)},
			},
			InitialDelaySeconds: 30,
			PeriodSeconds:       20,
			FailureThreshold:    3,
		},
	}
	if len(envVars) > 0 {
		container.Env = envVars
	}
	if strings.TrimSpace(spec.CPULimit) != "" || strings.TrimSpace(spec.MemoryLimit) != "" {
		container.Resources = corev1.ResourceRequirements{Limits: corev1.ResourceList{}}
		if strings.TrimSpace(spec.CPULimit) != "" {
			container.Resources.Limits[corev1.ResourceCPU] = resource.MustParse(spec.CPULimit)
		}
		if strings.TrimSpace(spec.MemoryLimit) != "" {
			container.Resources.Limits[corev1.ResourceMemory] = resource.MustParse(spec.MemoryLimit)
		}
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
			Labels:    map[string]string{"app": podName},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers:    []corev1.Container{container},
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot: ptr.To(true),
				SeccompProfile: &corev1.SeccompProfile{
					Type: corev1.SeccompProfileTypeRuntimeDefault,
				},
			},
			Volumes: []corev1.Volume{
				{Name: "tmp", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
			},
		},
	}
	if spec.UseKata && cfg != nil && strings.TrimSpace(cfg.Container.KataRuntimeClass) != "" {
		pod.Spec.RuntimeClassName = ptr.To(strings.TrimSpace(cfg.Container.KataRuntimeClass))
	}
	return pod, nil
}

func buildChallengeService(namespace, serviceName, podName string, spec *problemContainerRuntimeSpec) *corev1.Service {
	svcType := corev1.ServiceTypeClusterIP
	svcPort := int32(80)
	targetPort := intstr.FromInt(spec.ChallengePort)
	if spec != nil && spec.ProblemType == "container_tcp" {
		svcType = corev1.ServiceTypeNodePort
		svcPort = int32(spec.ChallengePort)
	}
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Type:     svcType,
			Selector: map[string]string{"app": podName},
			Ports: []corev1.ServicePort{
				{
					Port:       svcPort,
					TargetPort: targetPort,
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
}

func buildChallengeIngress(namespace, ingressName, serviceName, host, ingressClass string, tlsEnabled bool, tlsSecret string) *networkingv1.Ingress {
	pathType := networkingv1.PathTypePrefix
	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ingressName,
			Namespace: namespace,
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: host,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: serviceName,
											Port: networkingv1.ServiceBackendPort{Number: 80},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	if ingressClass != "" {
		ing.Spec.IngressClassName = ptr.To(ingressClass)
	}
	if tlsEnabled && host != "" {
		ing.Spec.TLS = []networkingv1.IngressTLS{
			{
				Hosts:      []string{host},
				SecretName: tlsSecret,
			},
		}
	}
	return ing
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

func mergeEnvVars(base []corev1.EnvVar, appended []corev1.EnvVar) []corev1.EnvVar {
	if len(base) == 0 && len(appended) == 0 {
		return nil
	}
	index := make(map[string]int, len(base)+len(appended))
	merged := make([]corev1.EnvVar, 0, len(base)+len(appended))
	for _, env := range base {
		key := strings.TrimSpace(env.Name)
		if key == "" {
			continue
		}
		index[key] = len(merged)
		merged = append(merged, env)
	}
	for _, env := range appended {
		key := strings.TrimSpace(env.Name)
		if key == "" {
			continue
		}
		if i, ok := index[key]; ok {
			merged[i] = env
			continue
		}
		index[key] = len(merged)
		merged = append(merged, env)
	}
	return merged
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

func mergeEnvPairs(base [][2]string, appended []corev1.EnvVar) [][2]string {
	if len(base) == 0 && len(appended) == 0 {
		return nil
	}
	merged := make([][2]string, 0, len(base)+len(appended))
	index := make(map[string]int, len(base)+len(appended))
	for _, pair := range base {
		key := strings.TrimSpace(pair[0])
		if key == "" {
			continue
		}
		index[key] = len(merged)
		merged = append(merged, [2]string{key, pair[1]})
	}
	for _, env := range appended {
		key := strings.TrimSpace(env.Name)
		if key == "" {
			continue
		}
		if i, ok := index[key]; ok {
			merged[i][1] = env.Value
			continue
		}
		index[key] = len(merged)
		merged = append(merged, [2]string{key, env.Value})
	}
	sort.SliceStable(merged, func(i, j int) bool {
		return merged[i][0] < merged[j][0]
	})
	return merged
}

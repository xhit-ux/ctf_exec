package k8s

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

// --- Challenge Pod ---

type ChallengePodSpec struct {
	Name             string
	Namespace        string
	ImageRef         string
	ContainerPort    int32
	CPULimit         string
	MemoryLimit      string
	EnvVars          []corev1.EnvVar
	KataRuntimeClass string
}

func BuildChallengePod(spec ChallengePodSpec) *corev1.Pod {
	labels := map[string]string{"app": spec.Name}
	container := corev1.Container{
		Name:            "challenge",
		Image:           spec.ImageRef,
		ImagePullPolicy: corev1.PullAlways,
		Ports: []corev1.ContainerPort{
			{ContainerPort: spec.ContainerPort, Protocol: corev1.ProtocolTCP},
		},
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: ptr.To(false),
			Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
			ReadOnlyRootFilesystem:   ptr.To(true),
		},
	}
	if len(spec.EnvVars) > 0 {
		container.Env = spec.EnvVars
	}
	if spec.CPULimit != "" || spec.MemoryLimit != "" {
		container.Resources = corev1.ResourceRequirements{
			Limits: corev1.ResourceList{},
		}
		if spec.CPULimit != "" {
			container.Resources.Limits[corev1.ResourceCPU] = resource.MustParse(spec.CPULimit)
		}
		if spec.MemoryLimit != "" {
			container.Resources.Limits[corev1.ResourceMemory] = resource.MustParse(spec.MemoryLimit)
		}
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      spec.Name,
			Namespace: spec.Namespace,
			Labels:    labels,
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
		},
	}
}

// --- Challenge Service ---

type ChallengeServiceSpec struct {
	Name        string
	Namespace   string
	PodLabel    string
	Port        int32
	ServiceType corev1.ServiceType
}

func BuildChallengeService(spec ChallengeServiceSpec) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      spec.Name,
			Namespace: spec.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Type:     spec.ServiceType,
			Selector: map[string]string{"app": spec.PodLabel},
			Ports: []corev1.ServicePort{
				{
					Port:       spec.Port,
					TargetPort: intstr.FromInt(int(spec.Port)),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
}

// --- Challenge Ingress ---

type ChallengeIngressSpec struct {
	Name         string
	Namespace    string
	ServiceName  string
	Host         string
	IngressClass string
	TLSEnabled   bool
	TLSSecret    string
}

func BuildChallengeIngress(spec ChallengeIngressSpec) *networkingv1.Ingress {
	pathType := networkingv1.PathTypePrefix
	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      spec.Name,
			Namespace: spec.Namespace,
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: spec.Host,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: spec.ServiceName,
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
	if spec.IngressClass != "" {
		ing.Spec.IngressClassName = ptr.To(spec.IngressClass)
	}
	if spec.TLSEnabled && spec.Host != "" {
		ing.Spec.TLS = []networkingv1.IngressTLS{
			{
				Hosts:      []string{spec.Host},
				SecretName: spec.TLSSecret,
			},
		}
	}
	return ing
}

// --- Registry PVC ---

type RegistryPVCConfig struct {
	Name         string
	Namespace    string
	StorageSize  string
	StorageClass string
}

func BuildRegistryPVC(cfg RegistryPVCConfig) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cfg.Name,
			Namespace: cfg.Namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(cfg.StorageSize),
				},
			},
			StorageClassName: ptr.To(cfg.StorageClass),
		},
	}
}

// --- Registry Secret (htpasswd) ---

type RegistrySecretConfig struct {
	Name      string
	Namespace string
	Htpasswd  []byte
}

func BuildRegistrySecret(cfg RegistrySecretConfig) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cfg.Name,
			Namespace: cfg.Namespace,
		},
		Data: map[string][]byte{
			"htpasswd": cfg.Htpasswd,
		},
		Type: corev1.SecretTypeOpaque,
	}
}

// --- Registry Deployment ---

type RegistryDeploymentConfig struct {
	Name       string
	Namespace  string
	NodeName   string
	Image      string
	SecretName string
	PVCName    string
}

func BuildRegistryDeployment(cfg RegistryDeploymentConfig) *appsv1.Deployment {
	labels := map[string]string{"app": "registry"}
	replicas := int32(1)

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cfg.Name,
			Namespace: cfg.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      "kubernetes.io/hostname",
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{cfg.NodeName},
											},
										},
									},
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "registry",
							Image: cfg.Image,
							Ports: []corev1.ContainerPort{
								{ContainerPort: 5000, Protocol: corev1.ProtocolTCP},
							},
							Env: []corev1.EnvVar{
								{Name: "REGISTRY_AUTH", Value: "htpasswd"},
								{Name: "REGISTRY_AUTH_HTPASSWD_REALM", Value: "Registry Realm"},
								{Name: "REGISTRY_AUTH_HTPASSWD_PATH", Value: "/auth/htpasswd"},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "data", MountPath: "/var/lib/registry"},
								{Name: "auth", MountPath: "/auth", ReadOnly: true},
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: ptr.To(false),
								Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
								ReadOnlyRootFilesystem:   ptr.To(true),
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: cfg.PVCName,
								},
							},
						},
						{
							Name: "auth",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: cfg.SecretName,
									Items: []corev1.KeyToPath{
										{Key: "htpasswd", Path: "htpasswd", Mode: ptr.To(int32(0444))},
									},
								},
							},
						},
					},
					// NOTE: registry:2 镜像必须以 root 运行，不设置 RunAsNonRoot。
				},
			},
		},
	}
}

// --- Registry Service ---

type RegistryServiceConfig struct {
	Name      string
	Namespace string
	Port      int32
}

func BuildRegistryService(cfg RegistryServiceConfig) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cfg.Name,
			Namespace: cfg.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeNodePort,
			Selector: map[string]string{"app": "registry"},
			Ports: []corev1.ServicePort{
				{
					Port:       cfg.Port,
					TargetPort: intstr.FromInt(int(cfg.Port)),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
}

var _ = fmt.Sprintf

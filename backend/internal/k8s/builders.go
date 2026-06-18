package k8s

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

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
								{Name: "REGISTRY_STORAGE_DELETE_ENABLED", Value: "true"},
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

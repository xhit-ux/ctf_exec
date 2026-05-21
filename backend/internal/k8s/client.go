package k8s

import (
	"context"
	"fmt"
	"log"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Client wraps the Kubernetes clientset for typed resource operations.
type Client struct {
	clientset  kubernetes.Interface
	restConfig *rest.Config
}

// NewClient creates a Client from a kubeconfig file path.
// If path is empty, falls back to in-cluster config.
func NewClient(kubeconfigPath string) (*Client, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfigPath != "" {
		loadingRules.ExplicitPath = kubeconfigPath
	}
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	restConfig, err := kubeConfig.ClientConfig()
	if err != nil {
		restConfig, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("无法加载 kubeconfig 且 in-cluster config 不可用: %w", err)
		}
		log.Println("[k8s] 使用 in-cluster 配置连接 Kubernetes")
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("创建 Kubernetes clientset 失败: %w", err)
	}

	return &Client{
		clientset:  clientset,
		restConfig: restConfig,
	}, nil
}

// --- Namespace ---

func (c *Client) EnsureNamespace(ctx context.Context, name string) error {
	_, err := c.clientset.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		return nil
	}
	if !errors.IsNotFound(err) {
		return fmt.Errorf("检查 namespace %s 失败: %w", name, err)
	}
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	_, err = c.clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			return nil
		}
		return fmt.Errorf("创建 namespace %s 失败: %w", name, err)
	}
	return nil
}

// --- Node ---

func (c *Client) ListNodeNames(ctx context.Context) ([]string, error) {
	nodes, err := c.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("列出节点失败: %w", err)
	}
	names := make([]string, 0, len(nodes.Items))
	for _, n := range nodes.Items {
		names = append(names, n.Name)
	}
	return names, nil
}

func (c *Client) LabelNode(ctx context.Context, nodeName string, labels map[string]string) error {
	node, err := c.clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("获取节点 %s 失败: %w", nodeName, err)
	}
	updated := node.DeepCopy()
	if updated.Labels == nil {
		updated.Labels = make(map[string]string)
	}
	for k, v := range labels {
		updated.Labels[k] = v
	}
	_, err = c.clientset.CoreV1().Nodes().Update(ctx, updated, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("更新节点 %s 标签失败: %w", nodeName, err)
	}
	return nil
}

// --- Pod ---

func (c *Client) CreatePod(ctx context.Context, namespace string, pod *corev1.Pod) (*corev1.Pod, error) {
	return c.clientset.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
}

func (c *Client) DeletePod(ctx context.Context, namespace, name string) error {
	grace := int64(0)
	err := c.clientset.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{GracePeriodSeconds: &grace})
	if errors.IsNotFound(err) {
		return nil
	}
	return err
}

// --- Service ---

func (c *Client) CreateService(ctx context.Context, namespace string, svc *corev1.Service) (*corev1.Service, error) {
	return c.clientset.CoreV1().Services(namespace).Create(ctx, svc, metav1.CreateOptions{})
}

func (c *Client) DeleteService(ctx context.Context, namespace, name string) error {
	err := c.clientset.CoreV1().Services(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if errors.IsNotFound(err) {
		return nil
	}
	return err
}

func (c *Client) GetService(ctx context.Context, namespace, name string) (*corev1.Service, error) {
	return c.clientset.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
}

func (c *Client) GetServiceNodePort(ctx context.Context, namespace, name string) (int, error) {
	svc, err := c.clientset.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return 0, fmt.Errorf("获取 service %s/%s 失败: %w", namespace, name, err)
	}
	if len(svc.Spec.Ports) == 0 {
		return 0, fmt.Errorf("service %s/%s 没有端口", namespace, name)
	}
	np := svc.Spec.Ports[0].NodePort
	if np == 0 {
		return 0, fmt.Errorf("service %s/%s 未分配 NodePort", namespace, name)
	}
	return int(np), nil
}

// --- Ingress ---

func (c *Client) CreateIngress(ctx context.Context, namespace string, ing *networkingv1.Ingress) (*networkingv1.Ingress, error) {
	return c.clientset.NetworkingV1().Ingresses(namespace).Create(ctx, ing, metav1.CreateOptions{})
}

func (c *Client) DeleteIngress(ctx context.Context, namespace, name string) error {
	err := c.clientset.NetworkingV1().Ingresses(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if errors.IsNotFound(err) {
		return nil
	}
	return err
}

// --- Deployment ---

func (c *Client) CreateDeployment(ctx context.Context, namespace string, dep *appsv1.Deployment) (*appsv1.Deployment, error) {
	return c.clientset.AppsV1().Deployments(namespace).Create(ctx, dep, metav1.CreateOptions{})
}

func (c *Client) GetDeployment(ctx context.Context, namespace, name string) (*appsv1.Deployment, error) {
	return c.clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
}

func (c *Client) WaitDeploymentReady(ctx context.Context, namespace, name string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		dep, err := c.clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("查询 deployment %s/%s 失败: %w", namespace, name, err)
		}
		if dep.Status.AvailableReplicas >= 1 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return fmt.Errorf("deployment %s/%s 在 %v 内未就绪", namespace, name, timeout)
}

// --- PVC ---

func (c *Client) CreatePVC(ctx context.Context, namespace string, pvc *corev1.PersistentVolumeClaim) (*corev1.PersistentVolumeClaim, error) {
	return c.clientset.CoreV1().PersistentVolumeClaims(namespace).Create(ctx, pvc, metav1.CreateOptions{})
}

func (c *Client) GetPVC(ctx context.Context, namespace, name string) (*corev1.PersistentVolumeClaim, error) {
	return c.clientset.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, name, metav1.GetOptions{})
}

// --- Secret ---

func (c *Client) CreateSecret(ctx context.Context, namespace string, secret *corev1.Secret) (*corev1.Secret, error) {
	return c.clientset.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
}

func (c *Client) GetSecret(ctx context.Context, namespace, name string) (*corev1.Secret, error) {
	return c.clientset.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
}

// --- Dynamic apply (escape hatch, not yet implemented) ---

func (c *Client) ApplyFromManifest(ctx context.Context, manifestYAML string) error {
	_, err := dynamic.NewForConfig(c.restConfig)
	if err != nil {
		return fmt.Errorf("创建 dynamic client 失败: %w", err)
	}
	_ = manifestYAML
	return fmt.Errorf("ApplyFromManifest 暂未实现，请使用 typed 方法")
}

package k8s

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"time"

	"golang.org/x/crypto/bcrypt"
	"k8s.io/apimachinery/pkg/api/errors"
)

const (
	registryNamespace    = "ctf-system"
	registryName         = "registry"
	registryPVCName      = "registry-data"
	registrySecretName   = "registry-htpasswd"
	registryStorageClass = "local-path"
	registryImage        = "registry:2"
	registryPort         = 5000
	registryReadyTimeout = 60 * time.Second
)

// RegistryInfo holds runtime configuration of the deployed registry.
type RegistryInfo struct {
	Host     string
	Username string
	Password string
	NodePort int
}

// NewRegistryConfig creates a RegistryInfo with auto-generated credentials.
func NewRegistryConfig() *RegistryInfo {
	return &RegistryInfo{
		Username: generateRandomString(12),
		Password: generateRandomString(24),
	}
}

// EnsureRegistry deploys the private Docker registry if not already running.
// Returns error if deployment fails; on success populates info with access details.
func EnsureRegistry(ctx context.Context, client *Client, info *RegistryInfo, nodeName string) error {
	_, err := client.GetDeployment(ctx, registryNamespace, registryName)
	if err == nil {
		np, npErr := client.GetServiceNodePort(ctx, registryNamespace, registryName)
		if npErr != nil {
			return fmt.Errorf("registry 已部署但无法获取 NodePort: %w", npErr)
		}
		info.NodePort = np
		info.Host = fmt.Sprintf("127.0.0.1:%d", np)
		secret, sErr := client.GetSecret(ctx, registryNamespace, registrySecretName)
		if sErr == nil && secret != nil {
			info.Username = string(secret.Data["username"])
			info.Password = string(secret.Data["password"])
		}
		return nil
	}
	if !errors.IsNotFound(err) {
		return fmt.Errorf("检查 registry deployment 失败: %w", err)
	}

	if nodeName == "" {
		nodes, lErr := client.ListNodeNames(ctx)
		if lErr != nil {
			return fmt.Errorf("无法发现集群节点: %w", lErr)
		}
		if len(nodes) == 0 {
			return fmt.Errorf("集群中没有可用节点")
		}
		nodeName = nodes[0]
	}

	if err := client.LabelNode(ctx, nodeName, map[string]string{"ctf-role": "registry"}); err != nil {
		return fmt.Errorf("节点 %s 打标签失败: %w", nodeName, err)
	}
	log.Printf("[infra] 节点 %s 已标记 ctf-role=registry", nodeName)

	if err := client.EnsureNamespace(ctx, registryNamespace); err != nil {
		return fmt.Errorf("创建 %s namespace 失败: %w", registryNamespace, err)
	}

	htpasswd, err := generateHtpasswd(info.Username, info.Password)
	if err != nil {
		return fmt.Errorf("生成 htpasswd 失败: %w", err)
	}

	secret := BuildRegistrySecret(RegistrySecretConfig{
		Name:      registrySecretName,
		Namespace: registryNamespace,
		Htpasswd:  htpasswd,
	})
	secret.Data["username"] = []byte(info.Username)
	secret.Data["password"] = []byte(info.Password)
	if _, err := client.CreateSecret(ctx, registryNamespace, secret); err != nil {
		if !errors.IsAlreadyExists(err) {
			return fmt.Errorf("创建 registry secret 失败: %w", err)
		}
	}

	pvc := BuildRegistryPVC(RegistryPVCConfig{
		Name:         registryPVCName,
		Namespace:    registryNamespace,
		StorageSize:  "30Gi",
		StorageClass: registryStorageClass,
	})
	if _, err := client.CreatePVC(ctx, registryNamespace, pvc); err != nil {
		if !errors.IsAlreadyExists(err) {
			return fmt.Errorf("创建 registry PVC 失败: %w", err)
		}
	}

	dep := BuildRegistryDeployment(RegistryDeploymentConfig{
		Name:       registryName,
		Namespace:  registryNamespace,
		NodeName:   nodeName,
		Image:      registryImage,
		SecretName: registrySecretName,
		PVCName:    registryPVCName,
	})
	if _, err := client.CreateDeployment(ctx, registryNamespace, dep); err != nil {
		if !errors.IsAlreadyExists(err) {
			return fmt.Errorf("创建 registry deployment 失败: %w", err)
		}
	}

	svc := BuildRegistryService(RegistryServiceConfig{
		Name:      registryName,
		Namespace: registryNamespace,
		Port:      registryPort,
	})
	if _, err := client.CreateService(ctx, registryNamespace, svc); err != nil {
		if !errors.IsAlreadyExists(err) {
			return fmt.Errorf("创建 registry service 失败: %w", err)
		}
	}

	np, err := client.GetServiceNodePort(ctx, registryNamespace, registryName)
	if err != nil {
		return fmt.Errorf("获取 registry NodePort 失败: %w", err)
	}
	info.NodePort = np
	info.Host = fmt.Sprintf("127.0.0.1:%d", np)

	if err := client.WaitDeploymentReady(ctx, registryNamespace, registryName, registryReadyTimeout); err != nil {
		return fmt.Errorf("registry deployment 未就绪: %w", err)
	}

	return nil
}

func generateHtpasswd(username, password string) ([]byte, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("bcrypt 哈希失败: %w", err)
	}
	return []byte(fmt.Sprintf("%s:%s\n", username, string(hash))), nil
}

func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			result[i] = charset[i%len(charset)]
			continue
		}
		result[i] = charset[n.Int64()]
	}
	return string(result)
}

package k8s

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
)

const dockerDaemonPath = "/etc/docker/daemon.json"

// ConfigureDockerInsecureRegistry adds the registry to Docker's insecure-registries list and reloads Docker.
func ConfigureDockerInsecureRegistry(nodePort int) error {
	target := fmt.Sprintf("127.0.0.1:%d", nodePort)
	cfg, err := loadDockerDaemonConfig()
	if err != nil {
		return fmt.Errorf("读取 %s 失败: %w", dockerDaemonPath, err)
	}

	registries, ok := cfg["insecure-registries"].([]interface{})
	if !ok {
		cfg["insecure-registries"] = []string{target}
	} else {
		for _, r := range registries {
			if s, ok := r.(string); ok && s == target {
				return nil
			}
		}
		newList := make([]string, 0, len(registries)+1)
		for _, r := range registries {
			if s, ok := r.(string); ok {
				newList = append(newList, s)
			}
		}
		cfg["insecure-registries"] = append(newList, target)
	}

	if err := writeDockerDaemonConfig(cfg); err != nil {
		return fmt.Errorf("写入 %s 失败: %w", dockerDaemonPath, err)
	}

	if err := reloadDocker(); err != nil {
		return fmt.Errorf("reload Docker 失败: %w", err)
	}

	return nil
}

func loadDockerDaemonConfig() (map[string]interface{}, error) {
	cfg := make(map[string]interface{})
	data, err := os.ReadFile(dockerDaemonPath)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析 JSON 失败: %w", err)
	}
	return cfg, nil
}

func writeDockerDaemonConfig(cfg map[string]interface{}) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 JSON 失败: %w", err)
	}
	data = append(data, '\n')
	tmpPath := dockerDaemonPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("写入临时文件失败: %w", err)
	}
	return os.Rename(tmpPath, dockerDaemonPath)
}

func reloadDocker() error {
	for _, cmd := range [][]string{
		{"systemctl", "reload", "docker"},
		{"service", "docker", "reload"},
	} {
		output, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
		if err == nil {
			log.Printf("[infra] Docker daemon 已 reload")
			return nil
		}
		log.Printf("[infra] Docker reload 尝试 %s 失败: %s", cmd, string(output))
	}
	return fmt.Errorf("Docker daemon reload 失败: 所有方法均未成功")
}

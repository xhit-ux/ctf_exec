package config

import "testing"

func TestLoadRegistryConfig(t *testing.T) {
	t.Setenv("REGISTRY_EXTERNAL", "false")
	t.Setenv("REGISTRY_HOST", "127.0.0.1:15000")
	t.Setenv("REGISTRY_USERNAME", "publisher")
	t.Setenv("REGISTRY_PASSWORD", "")

	cfg, err := loadRegistryConfig()
	if err != nil || cfg.External || cfg.Host != "" {
		t.Fatalf("集群内置仓库应由启动流程配置：%+v, %v", cfg, err)
	}

	t.Setenv("REGISTRY_EXTERNAL", "true")
	if _, err := loadRegistryConfig(); err == nil {
		t.Fatal("外部仓库凭证缺少密码时应返回错误")
	}

	t.Setenv("REGISTRY_USERNAME", "")
	cfg, err = loadRegistryConfig()
	if err != nil || !cfg.External || cfg.Host != "127.0.0.1:15000" {
		t.Fatalf("应读取外部仓库配置：%+v, %v", cfg, err)
	}
}

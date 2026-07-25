package conf

import (
	"encoding/json"
	"testing"
)

func TestDefaultConfigExcludesRemovedServices(t *testing.T) {
	body, err := json.Marshal(DefaultConfig(t.TempDir()))
	if err != nil {
		t.Fatalf("marshal default config: %v", err)
	}

	var config map[string]any
	if err := json.Unmarshal(body, &config); err != nil {
		t.Fatalf("unmarshal default config: %v", err)
	}
	for _, key := range []string{"s3", "ftp", "sftp", "mcp"} {
		if _, exists := config[key]; exists {
			t.Errorf("removed service %q remains in default config", key)
		}
	}

	scheme, ok := config["scheme"].(map[string]any)
	if !ok {
		t.Fatal("default config has no scheme object")
	}
	for _, key := range []string{
		"https_port",
		"force_https",
		"cert_file",
		"key_file",
		"enable_h3",
	} {
		if _, exists := scheme[key]; exists {
			t.Errorf("removed HTTPS option %q remains in default config", key)
		}
	}
}

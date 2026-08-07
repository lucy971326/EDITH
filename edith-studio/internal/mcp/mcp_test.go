package mcp

import (
	"os"
	"path/filepath"
	"testing"
)

// setUserConfigPath 让 os.UserHomeDir 返回临时目录，模拟用户级 mcp.json 的位置。
func setUserConfigPath(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	return home
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestLoadMergesProjectOverUser(t *testing.T) {
	home := setUserConfigPath(t)
	projectRoot := t.TempDir()

	writeFile(t, filepath.Join(home, ".edith", "mcp.json"), `{
	  "servers": {
	    "user-only": {"transport": "stdio", "command": "user-bin"},
	    "shared": {"transport": "stdio", "command": "user-bin", "env": {"A": "1"}}
	  }
	}`)
	writeFile(t, filepath.Join(projectRoot, ".edith", "mcp.json"), `{
	  "servers": {
	    "shared": {"transport": "stdio", "command": "project-bin"},
	    "project-only": {"transport": "sse", "serverUrl": "http://localhost:8080"}
	  }
	}`)

	config, err := Load(projectRoot)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(config.Servers) != 3 {
		t.Fatalf("server count = %d, want 3: %#v", len(config.Servers), config.Servers)
	}
	if got := config.Servers["shared"].Command; got != "project-bin" {
		t.Fatalf("shared command = %q, want project-bin (project overrides user)", got)
	}
	if _, ok := config.Servers["user-only"]; !ok {
		t.Fatal("user-only server was dropped")
	}
	if _, ok := config.Servers["project-only"]; !ok {
		t.Fatal("project-only server was dropped")
	}
}

func TestLoadIgnoresMissingConfigs(t *testing.T) {
	setUserConfigPath(t)
	config, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(config.Servers) != 0 {
		t.Fatalf("server count = %d, want 0", len(config.Servers))
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	home := setUserConfigPath(t)
	writeFile(t, filepath.Join(home, ".edith", "mcp.json"), `{
	  "servers": {"bad": {"transport": "stdio", "command": "x", "typo": true}}
	}`)
	if _, err := Load(t.TempDir()); err == nil {
		t.Fatal("unknown field should fail parsing")
	}
}

func TestConnectionConfigValidation(t *testing.T) {
	cases := []struct {
		name    string
		config  ServerConfig
		wantErr bool
	}{
		{"valid stdio", ServerConfig{Transport: "stdio", Command: "npx"}, false},
		{"valid streamable", ServerConfig{Transport: "streamable_http", ServerURL: "http://localhost:3000/mcp"}, false},
		{"valid sse", ServerConfig{Transport: "sse", ServerURL: "http://localhost:8080"}, false},
		{"stdio missing command", ServerConfig{Transport: "stdio"}, true},
		{"http missing url", ServerConfig{Transport: "sse"}, true},
		{"unknown transport", ServerConfig{Transport: "carrier_pigeon"}, true},
		{"bad timeout", ServerConfig{Transport: "stdio", Command: "npx", Timeout: "soon"}, true},
		{"good timeout", ServerConfig{Transport: "stdio", Command: "npx", Timeout: "10s"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := connectionConfig(tc.name, tc.config)
			if (err != nil) != tc.wantErr {
				t.Fatalf("connectionConfig err = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}

func TestAddServerFailureIsNotFatal(t *testing.T) {
	module := &Module{}
	// command 指向不存在的可执行文件，Init 必然失败；不应 panic，也不应阻塞其他 server。
	module.addServer("broken", ServerConfig{Transport: "stdio", Command: "edith-no-such-command", Timeout: "1s"})
	if len(module.ToolSets()) != 0 {
		t.Fatalf("broken server should not produce a ToolSet")
	}
	statuses := module.Status()
	if len(statuses) != 1 {
		t.Fatalf("status count = %d, want 1", len(statuses))
	}
	if statuses[0].Status != "error" {
		t.Fatalf("status = %q, want error", statuses[0].Status)
	}
	if statuses[0].Error == "" {
		t.Fatal("error status must carry a message")
	}
}

func TestInjectEnvSetsProcessEnvironment(t *testing.T) {
	injectEnv(map[string]string{"EDITH_MCP_TEST_KEY": "secret"})
	if got := os.Getenv("EDITH_MCP_TEST_KEY"); got != "secret" {
		t.Fatalf("env = %q, want secret", got)
	}
}

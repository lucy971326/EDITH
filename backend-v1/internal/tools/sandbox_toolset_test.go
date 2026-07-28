package tools

import "testing"

func TestSandboxToolSetRegistersEveryBuiltinTool(t *testing.T) {
	toolSet := &SandboxToolSet{}
	registered := map[string]bool{}
	for _, tool := range toolSet.Tools(t.Context()) {
		registered[tool.Declaration().Name] = true
	}

	for _, name := range []string{
		"sandbox_list_files",
		"sandbox_read_file",
		"sandbox_write_file",
		"sandbox_make_directory",
		"sandbox_move_path",
		"sandbox_delete_path",
		"sandbox_run_command",
		"sandbox_start_process",
		"sandbox_list_processes",
		"sandbox_kill_process",
	} {
		if !registered[name] {
			t.Errorf("SandboxToolSet did not register %q", name)
		}
	}
}

package sandbox

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/eric642/e2b-go-sdk"
	"github.com/google/uuid"
	"trpc.group/trpc-go/trpc-agent-go/tool"
	"trpc.group/trpc-go/trpc-agent-go/tool/function"
)

const (
	defaultCommandTimeout = 60_000
	maxCommandOutput      = 32 * 1024
)

type runCommandInput struct {
	Command   string            `json:"command" jsonschema:"description=Program to execute,required"`
	Args      []string          `json:"args,omitempty"`
	Envs      map[string]string `json:"envs,omitempty"`
	TimeoutMs int               `json:"timeoutMs,omitempty"`
}
type startProcessInput struct {
	Command string   `json:"command" jsonschema:"description=Long-running program,required"`
	Args    []string `json:"args,omitempty"`
}
type processInput struct {
	PID uint32 `json:"pid" jsonschema:"description=Sandbox process ID,required"`
}
type runCommandOutput struct {
	ExitCode        int    `json:"exitCode"`
	Stdout          string `json:"stdout"`
	Stderr          string `json:"stderr"`
	StdoutTruncated bool   `json:"stdoutTruncated"`
	StderrTruncated bool   `json:"stderrTruncated"`
}
type startProcessOutput struct {
	PID     uint32 `json:"pid"`
	LogPath string `json:"logPath"`
}
type processInfo struct {
	PID     uint32   `json:"pid"`
	Command string   `json:"command"`
	Args    []string `json:"args"`
	Cwd     string   `json:"cwd"`
	Tag     string   `json:"tag"`
}
type listProcessesOutput struct {
	Processes []processInfo `json:"processes"`
}
type killProcessOutput struct {
	PID     uint32 `json:"pid"`
	Message string `json:"message"`
}

func (s *toolSet) runCommandTool() tool.Tool {
	return function.NewFunctionTool(func(ctx context.Context, input runCommandInput) (runCommandOutput, error) {
		workspace, err := s.currentWorkspace(ctx)
		if err != nil {
			return runCommandOutput{}, err
		}
		command := strings.TrimSpace(input.Command)
		if command == "" {
			return runCommandOutput{}, fmt.Errorf("command is required")
		}
		timeout := input.TimeoutMs
		if timeout == 0 {
			timeout = defaultCommandTimeout
		}
		if timeout < 1 {
			return runCommandOutput{}, fmt.Errorf("command timeout must be positive")
		}
		handle, err := workspace.Commands.Run(ctx, command, e2b.RunOptions{Args: input.Args, Cwd: Workspace.WorkPath(), Envs: input.Envs, TimeoutMs: timeout})
		output := runCommandOutput{}
		if err != nil {
			var exit *e2b.CommandExitError
			if !errors.As(err, &exit) {
				return output, err
			}
			output.ExitCode = int(exit.Result.ExitCode)
			output.Stdout, output.StdoutTruncated = shorten(exit.Result.Stdout)
			output.Stderr, output.StderrTruncated = shorten(exit.Result.Stderr)
			return output, nil
		}
		if handle == nil {
			return output, fmt.Errorf("command completed without a result")
		}
		result, err := handle.Wait(ctx)
		if err != nil {
			return output, err
		}
		output.ExitCode = int(result.ExitCode)
		output.Stdout, output.StdoutTruncated = shorten(result.Stdout)
		output.Stderr, output.StderrTruncated = shorten(result.Stderr)
		return output, nil
	}, function.WithName("sandbox_run_command"), function.WithDescription(toolDescription("在当前会话 Sandbox 的 work/ 目录中前台执行程序，并返回输出。")))
}
func (s *toolSet) startProcessTool() tool.Tool {
	return function.NewFunctionTool(func(ctx context.Context, input startProcessInput) (startProcessOutput, error) {
		workspace, err := s.currentWorkspace(ctx)
		if err != nil {
			return startProcessOutput{}, err
		}
		command := strings.TrimSpace(input.Command)
		if command == "" {
			return startProcessOutput{}, fmt.Errorf("command is required")
		}
		directory := Workspace.WorkPath() + "/.edith/processes"
		if err := workspace.Files.MakeDir(ctx, directory, e2b.FsOptions{}); err != nil {
			return startProcessOutput{}, err
		}
		logPath := directory + "/" + uuid.NewString() + ".log"
		args := slices.Concat([]string{"-c", `log="$1"; shift; exec "$@" >"$log" 2>&1`, "--", logPath, command}, input.Args)
		handle, err := workspace.Commands.Start(ctx, "sh", e2b.RunOptions{Args: args, Cwd: Workspace.WorkPath(), Tag: "edith-agent"})
		if err != nil {
			return startProcessOutput{}, err
		}
		pid, err := waitForPID(ctx, handle)
		if err != nil {
			return startProcessOutput{}, err
		}
		return startProcessOutput{PID: pid, LogPath: relativePath(logPath)}, nil
	}, function.WithName("sandbox_start_process"), function.WithDescription(toolDescription("在当前会话 Sandbox 的 work/ 目录中启动长期进程。")))
}
func (s *toolSet) listProcessesTool() tool.Tool {
	return function.NewFunctionTool(func(ctx context.Context, _ struct{}) (listProcessesOutput, error) {
		workspace, err := s.currentWorkspace(ctx)
		if err != nil {
			return listProcessesOutput{}, err
		}
		processes, err := workspace.Commands.List(ctx)
		if err != nil {
			return listProcessesOutput{}, err
		}
		output := listProcessesOutput{Processes: []processInfo{}}
		for _, process := range processes {
			cwd := process.Cwd
			if strings.HasPrefix(cwd, Workspace.Root) {
				cwd = relativePath(cwd)
			}
			output.Processes = append(output.Processes, processInfo{PID: process.PID, Command: process.Cmd, Args: process.Args, Cwd: cwd, Tag: process.Tag})
		}
		return output, nil
	}, function.WithName("sandbox_list_processes"), function.WithDescription(toolDescription("列出当前会话 Sandbox 中正在运行的进程。")))
}
func (s *toolSet) killProcessTool() tool.Tool {
	return function.NewFunctionTool(func(ctx context.Context, input processInput) (killProcessOutput, error) {
		workspace, err := s.currentWorkspace(ctx)
		if err != nil {
			return killProcessOutput{}, err
		}
		if input.PID == 0 {
			return killProcessOutput{}, fmt.Errorf("process ID is required")
		}
		if _, err := workspace.Commands.Kill(ctx, input.PID); err != nil {
			return killProcessOutput{}, err
		}
		return killProcessOutput{PID: input.PID, Message: "process killed"}, nil
	}, function.WithName("sandbox_kill_process"), function.WithDescription(toolDescription("结束当前会话 Sandbox 中指定 PID 的进程。")))
}
func waitForPID(ctx context.Context, handle *e2b.CommandHandle) (uint32, error) {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()
	for {
		if pid := handle.PID(); pid != 0 {
			return pid, nil
		}
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-timeout.C:
			return 0, fmt.Errorf("sandbox process did not report a PID")
		case <-ticker.C:
		}
	}
}
func shorten(value string) (string, bool) {
	if len(value) <= maxCommandOutput {
		return value, false
	}
	return value[:maxCommandOutput], true
}

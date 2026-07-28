package tools

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

type sandboxRunCommandInput struct {
	Command   string            `json:"command" jsonschema:"description=Program to execute, for example go or npm,required"`
	Args      []string          `json:"args,omitempty" jsonschema:"description=Program arguments"`
	Envs      map[string]string `json:"envs,omitempty" jsonschema:"description=Environment variables for this command only"`
	TimeoutMs int               `json:"timeoutMs,omitempty" jsonschema:"description=Maximum execution time in milliseconds. Default is 60000."`
}

type sandboxStartProcessInput struct {
	Command string   `json:"command" jsonschema:"description=Long-running program to start, for example npm,required"`
	Args    []string `json:"args,omitempty" jsonschema:"description=Program arguments, for example [run, dev]"`
}

type sandboxProcessInput struct {
	PID uint32 `json:"pid" jsonschema:"description=Sandbox process ID,required"`
}

type sandboxRunCommandOutput struct {
	ExitCode        int    `json:"exitCode"`
	Stdout          string `json:"stdout"`
	Stderr          string `json:"stderr"`
	StdoutTruncated bool   `json:"stdoutTruncated"`
	StderrTruncated bool   `json:"stderrTruncated"`
}

type sandboxStartProcessOutput struct {
	PID     uint32 `json:"pid"`
	LogPath string `json:"logPath"`
}

type sandboxProcessInfo struct {
	PID     uint32   `json:"pid"`
	Command string   `json:"command"`
	Args    []string `json:"args"`
	Cwd     string   `json:"cwd"`
	Tag     string   `json:"tag"`
}

type sandboxListProcessesOutput struct {
	Processes []sandboxProcessInfo `json:"processes"`
}

type sandboxKillProcessOutput struct {
	PID     uint32 `json:"pid"`
	Message string `json:"message"`
}

func (s *SandboxToolSet) runCommandTool() tool.Tool {
	return function.NewFunctionTool(
		func(ctx context.Context, input sandboxRunCommandInput) (sandboxRunCommandOutput, error) {
			workspace, err := s.workspace(ctx)
			if err != nil {
				return sandboxRunCommandOutput{}, err
			}
			input.Command = strings.TrimSpace(input.Command)
			if input.Command == "" {
				return sandboxRunCommandOutput{}, fmt.Errorf("command is required")
			}
			timeout := input.TimeoutMs
			if timeout == 0 {
				timeout = defaultCommandTimeout
			}
			if timeout < 1 {
				return sandboxRunCommandOutput{}, fmt.Errorf("command timeout must be positive")
			}

			handle, err := workspace.Commands.Run(ctx, input.Command, e2b.RunOptions{
				Args:      input.Args,
				Cwd:       sandboxWorkspacePath,
				Envs:      input.Envs,
				TimeoutMs: timeout,
			})
			output := sandboxRunCommandOutput{}
			if err != nil {
				var exitError *e2b.CommandExitError
				if !errors.As(err, &exitError) {
					return output, err
				}
				output.ExitCode = int(exitError.Result.ExitCode)
				output.Stdout, output.StdoutTruncated = truncateCommandOutput(exitError.Result.Stdout)
				output.Stderr, output.StderrTruncated = truncateCommandOutput(exitError.Result.Stderr)
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
			output.Stdout, output.StdoutTruncated = truncateCommandOutput(result.Stdout)
			output.Stderr, output.StderrTruncated = truncateCommandOutput(result.Stderr)
			return output, nil
		},
		function.WithName("sandbox_run_command"),
		function.WithDescription("在当前会话 Sandbox 工作区中前台执行一个程序，并返回退出码、标准输出和错误输出。"),
	)
}

func (s *SandboxToolSet) startProcessTool() tool.Tool {
	return function.NewFunctionTool(
		func(ctx context.Context, input sandboxStartProcessInput) (sandboxStartProcessOutput, error) {
			workspace, err := s.workspace(ctx)
			if err != nil {
				return sandboxStartProcessOutput{}, err
			}
			input.Command = strings.TrimSpace(input.Command)
			if input.Command == "" {
				return sandboxStartProcessOutput{}, fmt.Errorf("command is required")
			}

			logDirectory := sandboxWorkspacePath + "/.edith/processes"
			if err := workspace.Files.MakeDir(ctx, logDirectory, e2b.FsOptions{}); err != nil {
				return sandboxStartProcessOutput{}, err
			}
			logPath := logDirectory + "/" + uuid.NewString() + ".log"
			args := slices.Concat(
				[]string{"-c", `log="$1"; shift; exec "$@" >"$log" 2>&1`, "--", logPath, input.Command},
				input.Args,
			)
			handle, err := workspace.Commands.Start(ctx, "sh", e2b.RunOptions{
				Args: args,
				Cwd:  sandboxWorkspacePath,
				Tag:  "edith-agent",
			})
			if err != nil {
				return sandboxStartProcessOutput{}, err
			}
			pid, err := waitForProcessID(ctx, handle)
			if err != nil {
				return sandboxStartProcessOutput{}, err
			}
			return sandboxStartProcessOutput{
				PID:     pid,
				LogPath: sandboxRelativePath(logPath),
			}, nil
		},
		function.WithName("sandbox_start_process"),
		function.WithDescription("在当前会话 Sandbox 中启动长期运行的进程。返回 PID 和日志文件路径；使用 sandbox_read_file 查看日志。"),
	)
}

func (s *SandboxToolSet) listProcessesTool() tool.Tool {
	return function.NewFunctionTool(
		func(ctx context.Context, _ struct{}) (sandboxListProcessesOutput, error) {
			workspace, err := s.workspace(ctx)
			if err != nil {
				return sandboxListProcessesOutput{}, err
			}
			processes, err := workspace.Commands.List(ctx)
			if err != nil {
				return sandboxListProcessesOutput{}, err
			}
			output := sandboxListProcessesOutput{Processes: []sandboxProcessInfo{}}
			for _, process := range processes {
				cwd := process.Cwd
				if strings.HasPrefix(cwd, sandboxWorkspacePath) {
					cwd = sandboxRelativePath(cwd)
				}
				output.Processes = append(output.Processes, sandboxProcessInfo{
					PID:     process.PID,
					Command: process.Cmd,
					Args:    process.Args,
					Cwd:     cwd,
					Tag:     process.Tag,
				})
			}
			return output, nil
		},
		function.WithName("sandbox_list_processes"),
		function.WithDescription("列出当前会话 Sandbox 中正在运行的进程。"),
	)
}

func (s *SandboxToolSet) killProcessTool() tool.Tool {
	return function.NewFunctionTool(
		func(ctx context.Context, input sandboxProcessInput) (sandboxKillProcessOutput, error) {
			workspace, err := s.workspace(ctx)
			if err != nil {
				return sandboxKillProcessOutput{}, err
			}
			if input.PID == 0 {
				return sandboxKillProcessOutput{}, fmt.Errorf("process ID is required")
			}
			if _, err := workspace.Commands.Kill(ctx, input.PID); err != nil {
				return sandboxKillProcessOutput{}, err
			}
			return sandboxKillProcessOutput{PID: input.PID, Message: "process killed"}, nil
		},
		function.WithName("sandbox_kill_process"),
		function.WithDescription("结束当前会话 Sandbox 中指定 PID 的进程。"),
	)
}

func waitForProcessID(ctx context.Context, handle *e2b.CommandHandle) (uint32, error) {
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

func truncateCommandOutput(output string) (string, bool) {
	if len(output) <= maxCommandOutput {
		return output, false
	}
	return output[:maxCommandOutput], true
}

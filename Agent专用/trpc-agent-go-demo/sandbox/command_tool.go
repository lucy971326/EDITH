package sandbox

import (
	"context"
	"fmt"

	"trpc.group/trpc-go/trpc-agent-go/tool"
	"trpc.group/trpc-go/trpc-agent-go/tool/function"
)

type runCommandRequest struct {
	Command string            `json:"command" jsonschema:"description=Shell command or program to run"`
	Args    []string          `json:"args,omitempty" jsonschema:"description=Command arguments"`
	Envs    map[string]string `json:"envs,omitempty" jsonschema:"description=Environment variables"`
}

type runCommandResponse struct {
	Command  string `json:"command"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
	Message  string `json:"message"`
}

func (s *sandboxToolSet) runCommandTool() tool.CallableTool {
	return function.NewFunctionTool(
		s.runCommand,
		function.WithName("run_command"),
		function.WithDescription(
			"Execute a program in the sandbox. "+
				"Use 'command' for the program name (e.g. 'whoami', 'cat', 'ls') "+
				"and 'args' for its arguments (e.g. ['/etc/os-release']). "+
				"For complex operations, 'args' can include '-c' and a shell expression "+
				"when 'command' is '/bin/sh' or '/bin/bash'.",
		),
	)
}

func (s *sandboxToolSet) runCommand(ctx context.Context, req runCommandRequest) (runCommandResponse, error) {
	rsp := runCommandResponse{Command: req.Command}
	backend, err := s.getBackend(ctx)
	if err != nil {
		rsp.Message = fmt.Sprintf("Error: %v", err)
		return rsp, err
	}
	result, err := backend.RunCommand(ctx, req.Command, req.Args, req.Envs)
	if err != nil {
		rsp.Message = fmt.Sprintf("Error: %v", err)
		return rsp, err
	}
	rsp.Stdout = result.Stdout
	rsp.Stderr = result.Stderr
	rsp.ExitCode = result.ExitCode
	if result.ExitCode == 0 {
		rsp.Message = "Command completed successfully."
	} else {
		rsp.Message = fmt.Sprintf("Command exited with code %d.", result.ExitCode)
	}
	return rsp, nil
}

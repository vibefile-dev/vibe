package executor

import (
	"bytes"
	"io"
	"os"
	"os/exec"
)

// Result holds the outcome of a script execution.
type Result struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

// IsSuccess returns true if the script exited with code 0.
func (r *Result) IsSuccess() bool { return r.ExitCode == 0 }

// IsLegitimateFailure returns true if the script ran correctly but the task
// found a real problem (exit code 1 — test failures, lint errors, etc.).
func (r *Result) IsLegitimateFailure() bool { return r.ExitCode == 1 }

// IsPreconditionFailure returns true if a preflight check failed (exit code 2
// — missing tool, wrong version, etc.).
func (r *Result) IsPreconditionFailure() bool { return r.ExitCode == 2 }

// IsScriptError returns true if the script itself is likely broken (exit code
// 3+). This includes command-not-found (127), permission denied (126), and any
// other unexpected failure.
func (r *Result) IsScriptError() bool { return r.ExitCode >= 3 }

// CombinedOutput returns stdout and stderr concatenated, for use in retry
// prompts.
func (r *Result) CombinedOutput() string {
	out := r.Stdout
	if r.Stderr != "" {
		if out != "" {
			out += "\n"
		}
		out += r.Stderr
	}
	return out
}

// Run executes a shell script in the given working directory. It streams
// stdout/stderr to the terminal in real time while also capturing them for
// the Result.
func Run(script, workDir string) *Result {
	var stdoutBuf, stderrBuf bytes.Buffer

	stdoutWriter := io.MultiWriter(os.Stdout, &stdoutBuf)
	stderrWriter := io.MultiWriter(os.Stderr, &stderrBuf)

	cmd := exec.Command("bash", "-c", script)
	cmd.Dir = workDir
	cmd.Stdout = stdoutWriter
	cmd.Stderr = stderrWriter
	cmd.Env = os.Environ()

	result := &Result{}

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = 127
		}
	}

	result.Stdout = stdoutBuf.String()
	result.Stderr = stderrBuf.String()
	return result
}

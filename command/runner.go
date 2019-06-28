// Package command contains primitives for running external commands.
package command

import (
	"bytes"
	"fmt"
	"os/exec"
)

// Runner provides an interface for running external commands.
type Runner interface {
	Execute(cmd string, args ...string) ([]byte, error)
}

// ShellRunner provides provides a simplified interface to exec.Command making it easier to process output and errors.
type ShellRunner struct{}

// Execute invokes a shell command with any number of arguments and returns standard output.
//
// If the command starts but does not complete successfully, an ExitError will be returned with output from standard
// error. Any other error will result in a panic.
func (ShellRunner) Execute(cmd string, args ...string) ([]byte, error) {
	c := exec.Command(cmd, args...)

	var stdout, stderr bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = &stderr
	err := c.Run()

	if err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			panic(fmt.Sprintf("unexpected error: %v", err))
		}
		err = newExitError(cmd, ee.ExitCode(), string(stderr.Bytes()))
	}

	return stdout.Bytes(), err
}
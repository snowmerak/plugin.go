package process

import (
	"fmt"
	"io"
	"os/exec"
)

// Process represents a forked external process with pipe-based communication.
// It provides access to the process's stdin and stdout via pipe readers/writers.
type Process struct {
	cmd          *exec.Cmd
	stdinWriter  *io.PipeWriter
	stdoutReader *io.PipeReader
}

func Fork(path string) (*Process, error) {
	if path == "" {
		return nil, fmt.Errorf("path cannot be empty")
	}

	cmd := exec.Command(path)
	stdinReader, stdinWriter := io.Pipe()
	stdoutReader, stdoutWriter := io.Pipe()
	cmd.Stdin = stdinReader
	cmd.Stdout = stdoutWriter
	cmd.Stderr = stdoutWriter

	if err := cmd.Start(); err != nil {
		// Clean up pipes if process failed to start
		stdinWriter.Close()
		stdinReader.Close()
		stdoutWriter.Close()
		stdoutReader.Close()
		return nil, fmt.Errorf("failed to start process: %w", err)
	}

	return &Process{
		cmd:          cmd,
		stdinWriter:  stdinWriter,
		stdoutReader: stdoutReader,
	}, nil
}

func (p *Process) Stdin() *io.PipeWriter {
	return p.stdinWriter
}

func (p *Process) Stdout() *io.PipeReader {
	return p.stdoutReader
}

// Pid returns the process ID
func (p *Process) Pid() int {
	if p.cmd.Process == nil {
		return -1
	}
	return p.cmd.Process.Pid
}

// IsAlive checks if the process is still running
func (p *Process) IsAlive() bool {
	if p.cmd.Process == nil {
		return false
	}
	// On Unix systems, sending signal 0 checks if process exists
	return p.cmd.Process.Signal(nil) == nil
}

func (p *Process) Wait() error {
	if err := p.cmd.Wait(); err != nil {
		return fmt.Errorf("process exited with error: %w", err)
	}
	return nil
}

func (p *Process) Close() error {
	var errs []error

	// Close pipes first
	if err := p.stdinWriter.Close(); err != nil {
		errs = append(errs, fmt.Errorf("failed to close stdin writer: %w", err))
	}
	if err := p.stdoutReader.Close(); err != nil {
		errs = append(errs, fmt.Errorf("failed to close stdout reader: %w", err))
	}

	// Terminate the process
	if p.cmd.Process != nil {
		if err := p.cmd.Process.Kill(); err != nil {
			errs = append(errs, fmt.Errorf("failed to kill process: %w", err))
		}
	}

	// Return combined errors if any
	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}
	return nil
}

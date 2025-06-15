package fork

import (
	"fmt"
	"io"
	"os/exec"
)

type Process struct {
	cmd          *exec.Cmd
	stdinWriter  *io.PipeWriter
	stdoutReader *io.PipeReader
}

func Fork(path string) (*Process, error) {
	cmd := exec.Command(path)
	stdinReader, stdinWriter := io.Pipe()
	stdoutReader, stdoutWriter := io.Pipe()
	cmd.Stdin = stdinReader
	cmd.Stdout = stdoutWriter
	cmd.Stderr = stdoutWriter
	if err := cmd.Start(); err != nil {
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

func (p *Process) Wait() error {
	if err := p.cmd.Wait(); err != nil {
		return fmt.Errorf("process exited with error: %w", err)
	}
	return nil
}

func (p *Process) Close() error {
	if err := p.stdinWriter.Close(); err != nil {
		return fmt.Errorf("failed to close stdin writer: %w", err)
	}
	if err := p.stdoutReader.Close(); err != nil {
		return fmt.Errorf("failed to close stdout reader: %w", err)
	}
	if err := p.cmd.Process.Kill(); err != nil {
		return fmt.Errorf("failed to kill process: %w", err)
	}
	return nil
}

package plugin

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"time"
)

// UnixSocketProvider provides Unix domain socket communication
type UnixSocketProvider struct {
	socketPath string
	listener   net.Listener
	conn       net.Conn
	isServer   bool
}

// NewUnixSocketProvider creates a new Unix domain socket communication provider
func NewUnixSocketProvider(socketPath string, isServer bool) (*UnixSocketProvider, error) {
	return &UnixSocketProvider{
		socketPath: socketPath,
		isServer:   isServer,
	}, nil
}

// CreateChannel implements CommunicationProvider for Unix domain sockets
func (u *UnixSocketProvider) CreateChannel(ctx context.Context, path string) (io.Reader, io.Writer, error) {
	if u.isServer {
		// Server side: create listener and wait for connection
		// Clean up any existing socket file
		os.Remove(u.socketPath)

		listener, err := net.Listen("unix", u.socketPath)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create Unix socket listener: %w", err)
		}
		u.listener = listener

		// Accept connection with timeout
		connChan := make(chan net.Conn, 1)
		errChan := make(chan error, 1)

		go func() {
			conn, err := listener.Accept()
			if err != nil {
				errChan <- err
				return
			}
			connChan <- conn
		}()

		select {
		case conn := <-connChan:
			u.conn = conn
			return conn, conn, nil
		case err := <-errChan:
			return nil, nil, fmt.Errorf("failed to accept connection: %w", err)
		case <-time.After(5 * time.Second):
			listener.Close()
			return nil, nil, fmt.Errorf("timeout waiting for connection")
		case <-ctx.Done():
			listener.Close()
			return nil, nil, ctx.Err()
		}
	} else {
		// Client side: connect to existing socket
		// Wait for socket file to exist
		for i := 0; i < 50; i++ { // Wait up to 5 seconds
			if _, err := os.Stat(u.socketPath); err == nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}

		conn, err := net.Dial("unix", u.socketPath)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to connect to Unix socket: %w", err)
		}
		u.conn = conn
		return conn, conn, nil
	}
}

// Close implements CommunicationProvider for Unix domain sockets
func (u *UnixSocketProvider) Close() error {
	var errs []error

	if u.conn != nil {
		if err := u.conn.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if u.listener != nil {
		if err := u.listener.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if u.isServer {
		// Clean up socket file
		os.Remove(u.socketPath)
	}

	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}
	return nil
}

// UnixSocketConfig holds configuration for Unix socket communication
type UnixSocketConfig struct {
	SocketPath string // Path to the Unix domain socket
	IsServer   bool   // True for server (host), false for client (plugin)
}

// WithUnixSocket creates loader options for Unix socket communication
func WithUnixSocket(config *UnixSocketConfig) *LoaderOptions {
	provider, _ := NewUnixSocketProvider(config.SocketPath, config.IsServer)
	return &LoaderOptions{
		CommunicationType: CommunicationTypeCustom,
		Provider:          provider,
	}
}

// WithUnixSocketModule creates module options for Unix socket communication
func WithUnixSocketModule(config *UnixSocketConfig) *ModuleOptions {
	provider, _ := NewUnixSocketProvider(config.SocketPath, config.IsServer)
	return &ModuleOptions{
		CommunicationType: CommunicationTypeCustom,
		Provider:          provider,
	}
}

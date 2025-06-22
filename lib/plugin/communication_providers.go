package plugin

import (
	"context"
	"fmt"
	"io"

	"github.com/snowmerak/plugin.go/lib/process"
)

// CommunicationType defines the type of communication channel
type CommunicationType int

const (
	// CommunicationTypeStdio uses stdin/stdout for communication (default)
	CommunicationTypeStdio CommunicationType = iota
	// CommunicationTypeHSQ uses HSQ shared memory for communication
	CommunicationTypeHSQ
	// CommunicationTypeCustom uses custom io.Reader/Writer
	CommunicationTypeCustom
)

// CommunicationProvider is an interface for creating communication channels
type CommunicationProvider interface {
	// CreateChannel creates a communication channel and returns reader/writer
	CreateChannel(ctx context.Context, path string) (io.Reader, io.Writer, error)
	// Close cleans up any resources
	Close() error
}

// StdioProvider provides stdin/stdout communication through process forking
type StdioProvider struct{}

// CreateChannel implements CommunicationProvider for stdio
func (s *StdioProvider) CreateChannel(ctx context.Context, path string) (io.Reader, io.Writer, error) {
	p, err := process.Fork(path)
	if err != nil {
		return nil, nil, err
	}
	// Store process reference for later cleanup (this would need to be handled differently)
	return p.Stdout(), p.Stdin(), nil
}

// Close implements CommunicationProvider for stdio
func (s *StdioProvider) Close() error {
	// Process cleanup would be handled separately
	return nil
}

// CustomProvider allows using custom io.Reader/Writer
type CustomProvider struct {
	Reader io.Reader
	Writer io.Writer
}

// CreateChannel implements CommunicationProvider for custom IO
func (c *CustomProvider) CreateChannel(ctx context.Context, path string) (io.Reader, io.Writer, error) {
	return c.Reader, c.Writer, nil
}

// Close implements CommunicationProvider for custom IO
func (c *CustomProvider) Close() error {
	// Custom cleanup if needed
	return nil
}

// HSQProvider provides HSQ shared memory communication
type HSQProvider struct {
	config       *HSQConfig
	realProvider *RealHSQProvider
}

// NewHSQProvider creates a new HSQ communication provider
func NewHSQProvider(config *HSQConfig) (*HSQProvider, error) {
	if config == nil {
		return nil, fmt.Errorf("HSQ config cannot be nil")
	}

	// Use the real HSQ provider implementation
	realProvider, err := NewRealHSQProvider(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create real HSQ provider: %w", err)
	}

	return &HSQProvider{
		config:       config,
		realProvider: realProvider,
	}, nil
}

// CreateChannel implements CommunicationProvider for HSQ
func (h *HSQProvider) CreateChannel(ctx context.Context, path string) (io.Reader, io.Writer, error) {
	// Delegate to the real HSQ provider
	return h.realProvider.CreateChannel(ctx, path)
}

// Close implements CommunicationProvider for HSQ
func (h *HSQProvider) Close() error {
	if h.realProvider != nil {
		return h.realProvider.Close()
	}
	return nil
}

// LoaderOptions defines options for creating a Loader
type LoaderOptions struct {
	// CommunicationType specifies the communication method
	CommunicationType CommunicationType

	// Provider specifies custom communication provider
	Provider CommunicationProvider

	// HSQConfig for HSQ-based communication
	HSQConfig *HSQConfig
}

// HSQConfig holds configuration for HSQ-based communication
type HSQConfig struct {
	SharedMemoryName string // Name for shared memory segment
	RingSize         int    // Number of slots in the ring buffer
	MaxMessageSize   int    // Maximum message size
	IsHost           bool   // True for host, false for plugin
}

// DefaultLoaderOptions returns default options using stdio communication
func DefaultLoaderOptions() *LoaderOptions {
	return &LoaderOptions{
		CommunicationType: CommunicationTypeStdio,
		Provider:          &StdioProvider{},
	}
}

// WithHSQ creates loader options for HSQ communication
func WithHSQ(config *HSQConfig) *LoaderOptions {
	return &LoaderOptions{
		CommunicationType: CommunicationTypeHSQ,
		HSQConfig:         config,
	}
}

// WithCustomProvider creates loader options with custom communication provider
func WithCustomProvider(provider CommunicationProvider) *LoaderOptions {
	return &LoaderOptions{
		CommunicationType: CommunicationTypeCustom,
		Provider:          provider,
	}
}

// ModuleOptions defines options for creating a Module
type ModuleOptions struct {
	// CommunicationType specifies the communication method
	CommunicationType CommunicationType

	// Provider specifies custom communication provider
	Provider CommunicationProvider

	// HSQConfig for HSQ-based communication
	HSQConfig *HSQConfig
}

// DefaultModuleOptions returns default options using stdio communication
func DefaultModuleOptions() *ModuleOptions {
	return &ModuleOptions{
		CommunicationType: CommunicationTypeStdio,
		Provider:          &StdioProvider{},
	}
}

// WithHSQModule creates module options for HSQ communication
func WithHSQModule(config *HSQConfig) *ModuleOptions {
	return &ModuleOptions{
		CommunicationType: CommunicationTypeHSQ,
		HSQConfig:         config,
	}
}

// WithCustomProviderModule creates module options with custom communication provider
func WithCustomProviderModule(provider CommunicationProvider) *ModuleOptions {
	return &ModuleOptions{
		CommunicationType: CommunicationTypeCustom,
		Provider:          provider,
	}
}

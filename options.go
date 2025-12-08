package logdot

import "time"

// Option is a functional option for configuring LogDot
type Option func(*Config)

// DefaultConfig returns sensible defaults
func DefaultConfig() Config {
	return Config{
		Timeout:        5 * time.Second,
		RetryAttempts:  3,
		RetryBaseDelay: 1 * time.Second,
		RetryMaxDelay:  30 * time.Second,
		Debug:          false,
	}
}

// WithHostname sets the hostname for logging
func WithHostname(hostname string) Option {
	return func(c *Config) {
		c.Hostname = hostname
	}
}

// WithEntityName sets the entity name for metrics
func WithEntityName(name string) Option {
	return func(c *Config) {
		c.EntityName = name
	}
}

// WithEntityDescription sets the entity description
func WithEntityDescription(desc string) Option {
	return func(c *Config) {
		c.EntityDescription = desc
	}
}

// WithTimeout sets the HTTP request timeout
func WithTimeout(d time.Duration) Option {
	return func(c *Config) {
		c.Timeout = d
	}
}

// WithRetry configures retry behavior
func WithRetry(attempts int, baseDelay, maxDelay time.Duration) Option {
	return func(c *Config) {
		c.RetryAttempts = attempts
		c.RetryBaseDelay = baseDelay
		c.RetryMaxDelay = maxDelay
	}
}

// WithDebug enables debug output
func WithDebug(enabled bool) Option {
	return func(c *Config) {
		c.Debug = enabled
	}
}

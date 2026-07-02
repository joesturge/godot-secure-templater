package internal

import (
	"fmt"
	"io"
	"log"
	"os"
)

// SimpleLogger is a basic logger implementation.
type SimpleLogger struct {
	verbose bool
}

// NewSimpleLogger creates a new logger.
func NewSimpleLogger(verbose bool) Logger {
	return &SimpleLogger{verbose: verbose}
}

func (l *SimpleLogger) Info(msg string, args ...interface{}) {
	log.Printf("[INFO] "+msg, args...)
}

func (l *SimpleLogger) Warn(msg string, args ...interface{}) {
	log.Printf("[WARN] "+msg, args...)
}

func (l *SimpleLogger) Error(msg string, args ...interface{}) {
	log.Printf("[ERROR] "+msg, args...)
}

func (l *SimpleLogger) Debug(msg string, args ...interface{}) {
	if l.verbose {
		log.Printf("[DEBUG] "+msg, args...)
	}
}

func (l *SimpleLogger) Printf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format, args...)
}

// NoOpHTTPClient is a stub for testing.
type NoOpHTTPClient struct{}

func (c *NoOpHTTPClient) Do(method, url string) (io.ReadCloser, int64, error) {
	return nil, 0, fmt.Errorf("no-op client: not implemented")
}

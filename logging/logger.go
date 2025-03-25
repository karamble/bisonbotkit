package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/decred/slog"
	"github.com/jrick/logrotate/rotator"
	"github.com/vctt94/bisonbotkit/utils"
)

// errMsgRE is a regexp that matches error log msgs.
var errMsgRE = regexp.MustCompile(`^\d{4}-\d\d-\d\d \d\d:\d\d:\d\d\.\d{3} \[ERR] `)

// LogBuffer is a simple buffer to store recent log lines
type LogBuffer struct {
	mu    sync.Mutex
	lines []string
	max   int
}

// NewLogBuffer creates a new buffer with the specified max size
func NewLogBuffer(maxLines int) *LogBuffer {
	return &LogBuffer{
		lines: make([]string, 0, maxLines),
		max:   maxLines,
	}
}

// Write adds a log line to the buffer
func (b *LogBuffer) Write(p []byte) (n int, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	line := string(p)
	if len(b.lines) >= b.max {
		// Remove oldest line by shifting all elements left
		b.lines = append(b.lines[1:], line)
	} else {
		b.lines = append(b.lines, line)
	}

	return len(p), nil
}

// LastLogLines returns the n most recent log lines
func (b *LogBuffer) LastLogLines(n int) []string {
	b.mu.Lock()
	defer b.mu.Unlock()

	if n > len(b.lines) {
		n = len(b.lines)
	}

	result := make([]string, n)
	copy(result, b.lines[len(b.lines)-n:])
	return result
}

// LogBackend is a custom logging backend with buffer and rotation capabilities
type LogBackend struct {
	logRotator      *rotator.Rotator
	bknd            *slog.Backend
	defaultLogLevel slog.Level
	logLevels       map[string]slog.Level

	loggersMtx sync.Mutex
	loggers    map[string]slog.Logger

	logCb     func(string)
	errorMsg  func(string)
	logBuffer *LogBuffer
	useStdout bool
}

// LogConfig contains configuration options for the logging system
type LogConfig struct {
	LogFile        string // Path to log file (empty for no log file)
	DebugLevel     string // Debug level string in format "subsys=level,subsys2=level2"
	MaxLogFiles    int    // Maximum number of log files to keep
	MaxBufferLines int    // Maximum number of log lines to buffer in memory
	LogCallback    func(string)
	ErrorCallback  func(string)
	UseStdout      *bool // Whether to output logs to stdout (defaults to true)
}

// NewLogBackend creates a new logging backend
func NewLogBackend(config LogConfig) (*LogBackend, error) {
	logFile := utils.CleanAndExpandPath(config.LogFile)
	var logRotator *rotator.Rotator
	if logFile != "" {
		logDir, _ := filepath.Split(logFile)
		err := os.MkdirAll(logDir, 0700)
		if err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}
		logRotator, err = rotator.New(logFile, 1024, false, config.MaxLogFiles)
		if err != nil {
			return nil, fmt.Errorf("failed to create file rotator: %w", err)
		}
	}

	// Set default value for UseStdout if not specified
	useStdout := true
	if config.UseStdout != nil {
		useStdout = *config.UseStdout
	}

	b := &LogBackend{
		logRotator:      logRotator,
		defaultLogLevel: slog.LevelInfo,
		logLevels:       make(map[string]slog.Level),
		logBuffer:       NewLogBuffer(config.MaxBufferLines),
		logCb:           config.LogCallback,
		errorMsg:        config.ErrorCallback,
		loggers:         make(map[string]slog.Logger),
		useStdout:       useStdout,
	}
	b.bknd = slog.NewBackend(b)

	// Parse the debugLevel string into log levels for each subsystem.
	if config.DebugLevel != "" {
		for _, v := range strings.Split(config.DebugLevel, ",") {
			fields := strings.Split(v, "=")
			if len(fields) == 1 {
				if fields[0] != "" {
					b.defaultLogLevel, _ = slog.LevelFromString(fields[0])
				}
			} else if len(fields) == 2 {
				subsys := fields[0]
				level, _ := slog.LevelFromString(fields[1])
				b.logLevels[subsys] = level
			} else {
				return nil, fmt.Errorf("unable to parse %q as subsys=level "+
					"debuglevel string", v)
			}
		}
	}

	return b, nil
}

// Write implements io.Writer interface for the backend
func (b *LogBackend) Write(p []byte) (int, error) {
	if b.logRotator != nil {
		b.logRotator.Write(p)
	}

	// Write to stdout if enabled
	if b.useStdout {
		os.Stdout.Write(p)
	}

	// Add to in-memory log buffer
	if n, err := b.logBuffer.Write(p); err != nil {
		return n, err
	}

	if b.logCb != nil {
		line := string(p)
		b.logCb(line)
	}

	if b.errorMsg != nil && errMsgRE.Match(p) {
		line := string(p[24:]) // Skip timestamp and [ERR] prefix
		b.errorMsg(line)
	}

	return len(p), nil
}

// Logger returns a logger for the given subsystem
func (b *LogBackend) Logger(subsys string) slog.Logger {
	b.loggersMtx.Lock()
	defer b.loggersMtx.Unlock()

	if l, ok := b.loggers[subsys]; ok {
		return l
	}

	l := b.bknd.Logger(subsys)
	b.loggers[subsys] = l

	if level, ok := b.logLevels[subsys]; ok {
		l.SetLevel(level)
	} else {
		l.SetLevel(b.defaultLogLevel)
	}

	return l
}

// SetLogLevel changes the logging level for a specific subsystem or the default
func (b *LogBackend) SetLogLevel(s string) error {
	if s == "" {
		return nil
	}

	fields := strings.Split(s, "=")
	if len(fields) == 1 {
		var ok bool
		b.defaultLogLevel, ok = slog.LevelFromString(fields[0])
		if !ok {
			return fmt.Errorf("unknown log level %q", fields[0])
		}

		b.loggersMtx.Lock()
		for subsys, l := range b.loggers {
			if _, hasSpecific := b.logLevels[subsys]; !hasSpecific {
				l.SetLevel(b.defaultLogLevel)
			}
		}
		b.loggersMtx.Unlock()
	} else if len(fields) == 2 {
		subsys := fields[0]
		level, ok := slog.LevelFromString(fields[1])
		if !ok {
			return fmt.Errorf("unknown log level %q", fields[1])
		}

		b.logLevels[subsys] = level

		b.loggersMtx.Lock()
		if l, ok := b.loggers[subsys]; ok {
			l.SetLevel(level)
		}
		b.loggersMtx.Unlock()
	} else {
		return fmt.Errorf("unable to parse %q as subsys=level "+
			"debuglevel string", s)
	}

	return nil
}

// LastLogLines returns the n most recent log lines
func (b *LogBackend) LastLogLines(n int) []string {
	return b.logBuffer.LastLogLines(n)
}

// Close shuts down the logger, closing any file handles
func (b *LogBackend) Close() error {
	if b.logRotator != nil {
		return b.logRotator.Close()
	}
	return nil
}

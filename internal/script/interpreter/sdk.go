package interpreter

import (
	"fmt"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/term"
	"os"
)

// EventManager manages middleware chains for events.
// Each event has an ordered list of middleware handlers that form a chain.
// Middleware handlers have the signature: tool(ctx, next) where:
//   - ctx: event-specific context object
//   - next: function to call the next middleware in chain
//
// Middleware can:
//   - Pass through: return next(ctx)
//   - Stop chain and override: return { result: ... } (don't call next)
//   - Transform context: modify ctx, then return next(ctx)
type EventManager struct {
	mu       sync.RWMutex
	handlers map[string][]*middlewareEntry // event -> ordered list of handlers
	nextID   int
}

// middlewareEntry holds a middleware handler and its metadata
type middlewareEntry struct {
	id      string
	handler *ToolValue
}

// NewEventManager creates a new event manager
func NewEventManager() *EventManager {
	return &EventManager{
		handlers: make(map[string][]*middlewareEntry),
		nextID:   0,
	}
}

// Use registers a middleware handler for an event and returns a unique handler ID.
// Middleware runs in registration order (first registered = first to run).
func (em *EventManager) Use(eventName string, handler *ToolValue) string {
	em.mu.Lock()
	defer em.mu.Unlock()

	// Generate unique handler ID
	em.nextID++
	handlerID := fmt.Sprintf("handler_%d", em.nextID)

	entry := &middlewareEntry{
		id:      handlerID,
		handler: handler,
	}

	// Append to the ordered list
	em.handlers[eventName] = append(em.handlers[eventName], entry)

	return handlerID
}

// Remove removes a middleware handler by tool reference.
// Returns true if removed, false if not found.
func (em *EventManager) Remove(eventName string, handler *ToolValue) bool {
	em.mu.Lock()
	defer em.mu.Unlock()

	entries := em.handlers[eventName]
	if entries == nil {
		return false
	}

	for i, entry := range entries {
		if entry.handler == handler {
			em.handlers[eventName] = append(entries[:i], entries[i+1:]...)
			// Clean up empty event lists
			if len(em.handlers[eventName]) == 0 {
				delete(em.handlers, eventName)
			}
			return true
		}
	}
	return false
}

// RemoveByID removes a middleware handler by its ID.
// Returns true if removed, false if not found.
func (em *EventManager) RemoveByID(eventName string, handlerID string) bool {
	em.mu.Lock()
	defer em.mu.Unlock()

	entries := em.handlers[eventName]
	if entries == nil {
		return false
	}

	for i, entry := range entries {
		if entry.id == handlerID {
			em.handlers[eventName] = append(entries[:i], entries[i+1:]...)
			// Clean up empty event lists
			if len(em.handlers[eventName]) == 0 {
				delete(em.handlers, eventName)
			}
			return true
		}
	}
	return false
}

// GetHandlers returns all handlers for a given event in registration order
func (em *EventManager) GetHandlers(eventName string) []*ToolValue {
	em.mu.RLock()
	defer em.mu.RUnlock()

	entries := em.handlers[eventName]
	if entries == nil {
		return nil
	}

	// Return handlers in order
	result := make([]*ToolValue, len(entries))
	for i, entry := range entries {
		result[i] = entry.handler
	}
	return result
}

// HasHandlers returns true if there are any handlers for the given event
func (em *EventManager) HasHandlers(eventName string) bool {
	em.mu.RLock()
	defer em.mu.RUnlock()
	return len(em.handlers[eventName]) > 0
}

// RemoveAll removes all handlers for an event.
// Returns the number of handlers removed.
func (em *EventManager) RemoveAll(eventName string) int {
	em.mu.Lock()
	defer em.mu.Unlock()

	entries := em.handlers[eventName]
	count := len(entries)
	delete(em.handlers, eventName)
	return count
}

// SDKConfig manages runtime configuration for the SDK
type SDKConfig struct {
	mu                        sync.RWMutex
	logger                    *zap.Logger
	atomicLevel               zap.AtomicLevel
	logFile                   string // read-only, set at initialization
	lastAgentRequest          Value
	completionMaxVisibleItems int
	// Models holds the model tier definitions (available in both REPL and script mode)
	models *Models
	// REPL context (nil in script mode)
	replContext *REPLContext
	// History provider for gsh.history access (nil in script mode)
	historyProvider HistoryProvider
}

const DefaultCompletionMaxVisibleItems = 10

// REPLContext holds REPL-specific state that's available in the SDK
type REPLContext struct {
	LastCommand             *REPLLastCommand
	PromptValue             Value        // Prompt string set by event handlers (read/write via gsh.prompt)
	ContinuationPromptValue Value        // Continuation prompt set by event handlers (read/write via gsh.continuationPrompt)
	Interpreter             *Interpreter // Reference to interpreter for event execution
}

// Models holds the model tier definitions (available in both REPL and script mode)
type Models struct {
	Lite      *ModelValue
	Workhorse *ModelValue
	Premium   *ModelValue
}

// REPLLastCommand holds information about the last executed command
type REPLLastCommand struct {
	Command    string
	ExitCode   int
	DurationMs int64
}

// HistoryEntry represents a single command history entry
type HistoryEntry struct {
	Command   string
	Timestamp int64
	ExitCode  int
}

// HistoryProvider provides access to command history for gsh scripts
type HistoryProvider interface {
	// FindPrefix returns history entries matching the given prefix, ordered by most recent first.
	// The limit parameter controls the maximum number of entries to search.
	FindPrefix(prefix string, limit int) ([]HistoryEntry, error)
	// GetRecent returns the most recent history entries in chronological order
	// (oldest first, most recent last). The limit parameter controls the maximum
	// number of entries to return.
	GetRecent(limit int) ([]HistoryEntry, error)
}

// NewSDKConfig creates a new SDK configuration
// The logger should have been created with an AtomicLevel for dynamic level changes to work
func NewSDKConfig(logger *zap.Logger, atomicLevel zap.AtomicLevel) *SDKConfig {
	// Extract log file from logger if available
	logFile := ""
	// Note: zap doesn't expose output paths directly, so logFile stays empty
	// It would need to be passed separately if needed in the future

	return &SDKConfig{
		logger:                    logger,
		atomicLevel:               atomicLevel,
		logFile:                   logFile,
		lastAgentRequest:          &NullValue{},
		completionMaxVisibleItems: DefaultCompletionMaxVisibleItems,
		models:                    &Models{}, // Initialize empty models (available in both REPL and script mode)
	}
}

// GetTermWidth returns the terminal width
func (sc *SDKConfig) GetTermWidth() int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return 80 // Default fallback
	}
	return width
}

// GetTermHeight returns the terminal height
func (sc *SDKConfig) GetTermHeight() int {
	_, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return 24 // Default fallback
	}
	return height
}

// IsTTY returns whether stdout is a TTY
func (sc *SDKConfig) IsTTY() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// GetLogLevel returns the current log level
func (sc *SDKConfig) GetLogLevel() string {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.atomicLevel.Level().String()
}

// SetLogLevel sets the log level dynamically
func (sc *SDKConfig) SetLogLevel(level string) error {
	// Parse the level string to zapcore.Level
	var zapLevel zapcore.Level
	switch level {
	case "debug":
		zapLevel = zapcore.DebugLevel
	case "info":
		zapLevel = zapcore.InfoLevel
	case "warn":
		zapLevel = zapcore.WarnLevel
	case "error":
		zapLevel = zapcore.ErrorLevel
	default:
		return fmt.Errorf("invalid log level '%s', must be one of: debug, info, warn, error", level)
	}

	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.atomicLevel.SetLevel(zapLevel)
	return nil
}

// GetLogFile returns the current log file path (read-only)
func (sc *SDKConfig) GetLogFile() string {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.logFile
}

// GetCompletionMaxVisibleItems returns the configured completion menu size.
func (sc *SDKConfig) GetCompletionMaxVisibleItems() int {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	if sc.completionMaxVisibleItems <= 0 {
		return DefaultCompletionMaxVisibleItems
	}
	return sc.completionMaxVisibleItems
}

// SetCompletionMaxVisibleItems sets the completion menu size.
func (sc *SDKConfig) SetCompletionMaxVisibleItems(value int) error {
	if value < 1 {
		return fmt.Errorf("completion max visible items must be at least 1")
	}

	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.completionMaxVisibleItems = value
	return nil
}

// GetLastAgentRequest returns the last agent request data
func (sc *SDKConfig) GetLastAgentRequest() Value {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.lastAgentRequest
}

// SetLastAgentRequest sets the last agent request data
func (sc *SDKConfig) SetLastAgentRequest(value Value) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.lastAgentRequest = value
}

// SetREPLContext sets the REPL context (called from REPL initialization)
func (sc *SDKConfig) SetREPLContext(ctx *REPLContext) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.replContext = ctx
}

// GetREPLContext returns the REPL context (nil in script mode)
func (sc *SDKConfig) GetREPLContext() *REPLContext {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.replContext
}

// UpdateLastCommand updates the last command's info including the command string, exit code and duration
func (sc *SDKConfig) UpdateLastCommand(command string, exitCode int, durationMs int64) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	if sc.replContext != nil && sc.replContext.LastCommand != nil {
		sc.replContext.LastCommand.Command = command
		sc.replContext.LastCommand.ExitCode = exitCode
		sc.replContext.LastCommand.DurationMs = durationMs
	}
}

// GetModels returns the models configuration (available in both REPL and script mode)
func (sc *SDKConfig) GetModels() *Models {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.models
}

// SetHistoryProvider sets the history provider for gsh.history access
func (sc *SDKConfig) SetHistoryProvider(provider HistoryProvider) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.historyProvider = provider
}

// GetHistoryProvider returns the history provider (nil in script mode)
func (sc *SDKConfig) GetHistoryProvider() HistoryProvider {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.historyProvider
}

// Package input provides a unified line input component for the gsh REPL.
// It merges functionality from pkg/gline and pkg/shellinput into a single
// cohesive Bubble Tea component that handles text input, cursor management,
// key bindings, tab completion, and LLM prediction integration.
package input

import (
	tea "github.com/charmbracelet/bubbletea"
	"go.uber.org/zap"
)

// ResultType indicates the type of result from the input component.
type ResultType int

const (
	// ResultNone indicates no result yet (still editing).
	ResultNone ResultType = iota
	// ResultSubmit indicates the user submitted the input (Enter).
	ResultSubmit
	// ResultInterrupt indicates the user interrupted (Ctrl+C).
	ResultInterrupt
	// ResultEOF indicates end of input (Ctrl+D on empty line).
	ResultEOF
)

// Result contains the outcome of an input session.
type Result struct {
	// Type indicates what action caused the input to complete.
	Type ResultType
	// Value is the input text (empty for interrupt/EOF).
	Value string
}

// HistorySearchFunc is a function type for searching history.
// It takes a query string and returns matching commands.
type HistorySearchFunc func(query string) []string

// Model is the Bubble Tea model for the unified input component.
// It coordinates the buffer, keymap, completion, prediction, and rendering.
type Model struct {
	// Core state
	buffer  *Buffer
	keymap  *KeyMap
	focused bool

	// Prompt
	prompt             string
	continuationPrompt string

	// History navigation
	historyValues       []string
	historyIndex        int // 0 = current input, 1+ = history entries
	savedCurrentInput   string
	hasNavigatedHistory bool

	// History search (Ctrl+R)
	historySearch     *HistorySearchState
	historySearchFunc HistorySearchFunc

	// Completion
	completion         *CompletionState
	completionProvider CompletionProvider

	// Prediction
	prediction        *PredictionState
	currentPrediction string

	// Rendering
	renderer          *Renderer
	width             int
	minHeight         int
	hasReceivedResize bool // tracks if we've received the initial WindowSizeMsg

	// Info panel content (help text, etc.)
	infoContent InfoPanelContent

	// Result state
	result Result

	// Logger
	logger *zap.Logger
}

// Config holds configuration for creating a new Model.
type Config struct {
	// Prompt is the prompt string to display.
	Prompt string

	// AliasExistsFunc returns true if the given name is currently defined as a shell alias
	// or shell function. If set, the input syntax highlighter will treat aliases and
	// functions (e.g., from .gshenv or .gsh_profile) as valid commands.
	AliasExistsFunc func(name string) bool

	// GetEnvFunc returns the value of an environment variable from the shell.
	// If set, the syntax highlighter will use the shell's PATH (which may have been
	// modified in .gshenv or .gsh_profile) for command lookups, and the shell's
	// environment for variable highlighting.
	GetEnvFunc func(name string) string

	// GetWorkingDirFunc returns the current working directory from the shell.
	// If set, the syntax highlighter will resolve relative paths against this
	// directory instead of the process working directory.
	GetWorkingDirFunc func() string

	// HistoryValues is the list of previous commands for history navigation.
	// Index 0 is the most recent.
	HistoryValues []string

	// HistorySearchFunc is a function for searching history (used by Ctrl+R).
	// If nil, history search will use the HistoryValues list.
	HistorySearchFunc HistorySearchFunc

	// CompletionProvider provides tab completion suggestions.
	CompletionProvider CompletionProvider

	// CompletionMaxVisible is the maximum number of completion suggestions shown at once.
	// If zero or negative, a default is used.
	CompletionMaxVisible int

	// PredictionState manages command predictions.
	PredictionState *PredictionState

	// KeyMap provides key bindings. If nil, DefaultKeyMap is used.
	KeyMap *KeyMap

	// RenderConfig provides styling. If nil, DefaultRenderConfig is used.
	RenderConfig *RenderConfig

	// ContinuationPrompt is the prompt shown on continuation lines for multi-line input.
	// If empty, defaults to "> ".
	ContinuationPrompt string

	// MinHeight is the minimum number of lines to render.
	MinHeight int

	// Width is the initial terminal width.
	Width int

	// Logger for debug output. If nil, a no-op logger is used.
	Logger *zap.Logger
}

// New creates a new input Model with the given configuration.
func New(cfg Config) Model {
	logger := cfg.Logger
	if logger == nil {
		logger = zap.NewNop()
	}

	keymap := cfg.KeyMap
	if keymap == nil {
		keymap = DefaultKeyMap()
	}

	renderConfig := cfg.RenderConfig
	if renderConfig == nil {
		defaultConfig := DefaultRenderConfig()
		renderConfig = &defaultConfig
	}

	width := cfg.Width
	if width <= 0 {
		width = 80
	}

	continuationPrompt := cfg.ContinuationPrompt
	if continuationPrompt == "" {
		continuationPrompt = "> "
	}

	renderer := NewRenderer(*renderConfig, NewHighlighter(cfg.AliasExistsFunc, cfg.GetEnvFunc, cfg.GetWorkingDirFunc))
	renderer.SetWidth(width)
	renderer.SetContinuationPrompt(continuationPrompt)
	renderer.SetCompletionMaxVisible(cfg.CompletionMaxVisible)

	return Model{
		buffer:             NewBuffer(),
		keymap:             keymap,
		focused:            true,
		prompt:             cfg.Prompt,
		continuationPrompt: continuationPrompt,
		historyValues:      cfg.HistoryValues,
		historyIndex:       0,
		historySearch:      NewHistorySearchState(),
		historySearchFunc:  cfg.HistorySearchFunc,
		completion:         NewCompletionState(),
		completionProvider: cfg.CompletionProvider,
		prediction:         cfg.PredictionState,
		renderer:           renderer,
		width:              width,
		minHeight:          cfg.MinHeight,
		result:             Result{Type: ResultNone},
		logger:             logger,
	}
}

// Init implements tea.Model. It triggers an initial prediction request.
func (m Model) Init() tea.Cmd {
	if m.prediction != nil {
		// Trigger initial prediction for empty input (null-state prediction)
		return m.requestPrediction("")
	}
	return nil
}

// Update implements tea.Model. It handles all input events.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !m.focused {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Track whether this is a real resize or just the initial size message
		isActualResize := m.hasReceivedResize && msg.Width != m.width
		m.hasReceivedResize = true
		m.width = msg.Width
		m.renderer.SetWidth(msg.Width)
		// Only clear the screen on actual resize to prevent duplicate prompt rendering.
		// Don't clear on the initial WindowSizeMsg to preserve the welcome screen.
		if isActualResize {
			return m, tea.ClearScreen
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case predictionResultMsg:
		return m.handlePredictionResult(msg)

	case pasteMsg:
		return m.handlePaste(string(msg))
	}

	return m, nil
}

// View implements tea.Model. It renders the input component.
func (m Model) View() string {
	if m.result.Type != ResultNone {
		// Input is complete, render final state
		return m.renderFinalView()
	}

	// Use history search prompt when in search mode
	prompt := m.prompt
	showBufferCursor := m.focused
	if m.historySearch.IsActive() {
		// Show cursor in search prompt, not in the buffer
		prompt = m.renderer.RenderHistorySearchPrompt(m.historySearch, m.focused)
		showBufferCursor = false
	}

	return m.renderer.RenderFullView(
		prompt,
		m.buffer,
		m.currentPrediction,
		showBufferCursor,
		m.completion,
		m.infoContent,
		m.minHeight,
	)
}

// Result returns the current result. Check Type != ResultNone to see if complete.
func (m Model) Result() Result {
	return m.result
}

// Value returns the current input text.
func (m Model) Value() string {
	return m.buffer.Text()
}

// SetValue sets the input text and moves cursor to end.
func (m *Model) SetValue(text string) {
	m.buffer.SetText(text)
	m.historyIndex = 0
	m.hasNavigatedHistory = false
}

// Focus sets the focus state on the model.
func (m *Model) Focus() {
	m.focused = true
}

// Blur removes focus from the model.
func (m *Model) Blur() {
	m.focused = false
}

// Focused returns whether the model is focused.
func (m Model) Focused() bool {
	return m.focused
}

// SetPrompt updates the prompt string.
func (m *Model) SetPrompt(prompt string) {
	m.prompt = prompt
}

// Prompt returns the current prompt string.
func (m Model) Prompt() string {
	return m.prompt
}

// ContinuationPrompt returns the continuation prompt for multi-line input.
func (m Model) ContinuationPrompt() string {
	return m.continuationPrompt
}

// SetHistoryValues updates the history values for navigation.
func (m *Model) SetHistoryValues(values []string) {
	m.historyValues = values
	m.historyIndex = 0
	m.hasNavigatedHistory = false
}

// Reset clears the input state for a new input session.
func (m *Model) Reset() {
	m.buffer.Clear()
	m.completion.Reset()
	if m.prediction != nil {
		m.prediction.Reset()
	}
	m.currentPrediction = ""
	m.historyIndex = 0
	m.savedCurrentInput = ""
	m.hasNavigatedHistory = false
	m.historySearch.Reset()
	m.result = Result{Type: ResultNone}
	m.infoContent = nil
}

// SetHistorySearchFunc sets the function used for history search.
func (m *Model) SetHistorySearchFunc(fn HistorySearchFunc) {
	m.historySearchFunc = fn
}

// HistorySearch returns the history search state (for testing/rendering).
func (m Model) HistorySearch() *HistorySearchState {
	return m.historySearch
}

// Buffer returns the underlying buffer (for testing).
func (m Model) Buffer() *Buffer {
	return m.buffer
}

// Completion returns the completion state (for testing).
func (m Model) Completion() *CompletionState {
	return m.completion
}

// CurrentPrediction returns the current prediction text.
func (m Model) CurrentPrediction() string {
	return m.currentPrediction
}

// handleKeyMsg processes keyboard input.
func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Look up the action for this key
	action := m.keymap.Lookup(msg)

	// When history search is active, handle keys specially
	if m.historySearch.IsActive() {
		return m.handleHistorySearchKey(msg, action)
	}

	// When completion is active, handle navigation keys specially
	if m.completion.IsActive() {
		switch action {
		case ActionComplete, ActionCursorDown:
			// Tab or Down arrow: cycle forward through completions
			return m.handleComplete()
		case ActionCompleteBackward, ActionCursorUp:
			// Shift+Tab or Up arrow: cycle backward through completions
			return m.handleCompleteBackward()
		case ActionCancel:
			// Escape: cancel completion
			return m.handleCompletionAction(action)
		case ActionSubmit:
			// Enter: accept current completion and submit
			m.completion.Reset()
			return m.handleSubmit()
		}
		// For other keys, reset completion and continue with normal handling
		m.completion.Reset()
	}

	// Handle special actions first
	switch action {
	case ActionSubmit:
		return m.handleSubmit()

	case ActionInsertNewline:
		return m.handleInsertNewline()

	case ActionInterrupt:
		return m.handleInterrupt()

	case ActionDeleteCharacterForward:
		// Ctrl+D on empty input triggers EOF
		if m.buffer.Len() == 0 {
			return m.handleEOF()
		}
		return m.handleDeleteCharacterForward()

	case ActionEOF:
		return m.handleEOF()

	case ActionClearScreen:
		return m, tea.ClearScreen

	case ActionPaste:
		return m, Paste

	case ActionComplete:
		return m.handleComplete()

	case ActionCompleteBackward:
		return m.handleCompleteBackward()

	case ActionCancel:
		return m.handleCancel()

	case ActionAcceptPrediction:
		return m.handleAcceptPrediction()

	case ActionHistorySearchBackward:
		return m.handleHistorySearchStart()

	// Navigation actions
	case ActionCharacterForward:
		return m.handleCharacterForward()

	case ActionCharacterBackward:
		m.buffer.SetPos(m.buffer.Pos() - 1)
		return m, nil

	case ActionWordForward:
		m.buffer.WordForward()
		return m, nil

	case ActionWordBackward:
		m.buffer.WordBackward()
		return m, nil

	case ActionLineStart:
		m.buffer.CursorStart()
		return m, nil

	case ActionLineEnd:
		m.buffer.CursorEnd()
		return m, nil

	// Deletion actions
	case ActionDeleteCharacterBackward:
		return m.handleDeleteCharacterBackward()

	case ActionDeleteWordBackward:
		return m.handleDeleteWordBackward()

	case ActionDeleteWordForward:
		return m.handleDeleteWordForward()

	case ActionDeleteBeforeCursor:
		return m.handleDeleteBeforeCursor()

	case ActionDeleteAfterCursor:
		return m.handleDeleteAfterCursor()

	// Vertical navigation (history when completion not active)
	case ActionCursorUp:
		return m.handleHistoryPrevious()

	case ActionCursorDown:
		return m.handleHistoryNext()

	default:
		// Insert regular characters
		if len(msg.Runes) > 0 {
			return m.handleInsertRunes(msg.Runes)
		}
	}

	return m, nil
}

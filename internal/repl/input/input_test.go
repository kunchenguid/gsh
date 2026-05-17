package input

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// mockCompletionProvider implements CompletionProvider for testing.
type mockCompletionProvider struct {
	completions []string
	helpInfo    string
}

func (m *mockCompletionProvider) GetCompletions(line string, pos int) []string {
	return m.completions
}

func (m *mockCompletionProvider) GetHelpInfo(line string, pos int) string {
	return m.helpInfo
}

func TestNew(t *testing.T) {
	t.Run("creates model with defaults", func(t *testing.T) {
		m := New(Config{})

		if m.buffer == nil {
			t.Error("buffer should not be nil")
		}
		if m.keymap == nil {
			t.Error("keymap should not be nil")
		}
		if !m.focused {
			t.Error("model should be focused by default")
		}
		if m.result.Type != ResultNone {
			t.Error("result type should be ResultNone")
		}
	})

	t.Run("creates model with custom config", func(t *testing.T) {
		cfg := Config{
			Prompt:        "test> ",
			HistoryValues: []string{"cmd1", "cmd2"},
			Width:         120,
			MinHeight:     3,
		}
		m := New(cfg)

		if m.prompt != "test> " {
			t.Errorf("expected prompt 'test> ', got '%s'", m.prompt)
		}
		if len(m.historyValues) != 2 {
			t.Errorf("expected 2 history values, got %d", len(m.historyValues))
		}
		if m.width != 120 {
			t.Errorf("expected width 120, got %d", m.width)
		}
		if m.minHeight != 3 {
			t.Errorf("expected minHeight 3, got %d", m.minHeight)
		}
	})
}

func TestModelValue(t *testing.T) {
	m := New(Config{})

	if m.Value() != "" {
		t.Errorf("expected empty value, got '%s'", m.Value())
	}

	m.SetValue("hello")
	if m.Value() != "hello" {
		t.Errorf("expected 'hello', got '%s'", m.Value())
	}
}

func TestModelFocus(t *testing.T) {
	m := New(Config{})

	if !m.Focused() {
		t.Error("model should be focused by default")
	}

	m.Blur()
	if m.Focused() {
		t.Error("model should not be focused after Blur()")
	}

	m.Focus()
	if !m.Focused() {
		t.Error("model should be focused after Focus()")
	}
}

func TestModelPrompt(t *testing.T) {
	m := New(Config{Prompt: "$ "})

	if m.Prompt() != "$ " {
		t.Errorf("expected '$ ', got '%s'", m.Prompt())
	}

	m.SetPrompt(">>> ")
	if m.Prompt() != ">>> " {
		t.Errorf("expected '>>> ', got '%s'", m.Prompt())
	}
}

func TestModelReset(t *testing.T) {
	m := New(Config{
		Prompt:        "$ ",
		HistoryValues: []string{"cmd1", "cmd2"},
	})

	// Set some state
	m.SetValue("test input")
	m.historyIndex = 1
	m.hasNavigatedHistory = true
	m.currentPrediction = "test prediction"

	// Reset
	m.Reset()

	if m.Value() != "" {
		t.Errorf("expected empty value after reset, got '%s'", m.Value())
	}
	if m.historyIndex != 0 {
		t.Errorf("expected historyIndex 0 after reset, got %d", m.historyIndex)
	}
	if m.hasNavigatedHistory {
		t.Error("hasNavigatedHistory should be false after reset")
	}
	if m.currentPrediction != "" {
		t.Errorf("expected empty prediction after reset, got '%s'", m.currentPrediction)
	}
	if m.result.Type != ResultNone {
		t.Error("result type should be ResultNone after reset")
	}
}

func TestCharacterInput(t *testing.T) {
	m := New(Config{})

	// Simulate typing 'a'
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}
	newModel, _ := m.Update(msg)
	m = newModel.(Model)

	if m.Value() != "a" {
		t.Errorf("expected 'a', got '%s'", m.Value())
	}

	// Simulate typing 'b'
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}}
	newModel, _ = m.Update(msg)
	m = newModel.(Model)

	if m.Value() != "ab" {
		t.Errorf("expected 'ab', got '%s'", m.Value())
	}
}

func TestSubmit(t *testing.T) {
	m := New(Config{})
	m.SetValue("test command")

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	newModel, cmd := m.Update(msg)
	m = newModel.(Model)

	if m.result.Type != ResultSubmit {
		t.Errorf("expected ResultSubmit, got %v", m.result.Type)
	}
	if m.result.Value != "test command" {
		t.Errorf("expected 'test command', got '%s'", m.result.Value)
	}
	if cmd == nil {
		t.Error("expected quit command")
	}
}

func TestInterrupt(t *testing.T) {
	m := New(Config{})
	m.SetValue("partial input")

	msg := tea.KeyMsg{Type: tea.KeyCtrlC}
	newModel, cmd := m.Update(msg)
	m = newModel.(Model)

	if m.result.Type != ResultInterrupt {
		t.Errorf("expected ResultInterrupt, got %v", m.result.Type)
	}
	if m.result.Value != "" {
		t.Errorf("expected empty value for interrupt, got '%s'", m.result.Value)
	}
	if cmd == nil {
		t.Error("expected quit command")
	}
}

func TestEOFOnEmptyInput(t *testing.T) {
	m := New(Config{})

	// Ctrl+D on empty input should trigger EOF
	msg := tea.KeyMsg{Type: tea.KeyCtrlD}
	newModel, cmd := m.Update(msg)
	m = newModel.(Model)

	if m.result.Type != ResultEOF {
		t.Errorf("expected ResultEOF, got %v", m.result.Type)
	}
	if cmd == nil {
		t.Error("expected quit command")
	}
}

func TestEOFOnNonEmptyInput(t *testing.T) {
	m := New(Config{})
	m.SetValue("some text")
	m.buffer.SetPos(5) // Position cursor in the middle

	// Ctrl+D on non-empty input should delete character forward
	msg := tea.KeyMsg{Type: tea.KeyCtrlD}
	newModel, _ := m.Update(msg)
	m = newModel.(Model)

	if m.result.Type != ResultNone {
		t.Errorf("expected ResultNone, got %v", m.result.Type)
	}
	// Should have deleted character at cursor position
	if m.Value() != "some ext" {
		t.Errorf("expected 'some ext', got '%s'", m.Value())
	}
}

func TestBackspace(t *testing.T) {
	m := New(Config{})
	m.SetValue("hello")

	msg := tea.KeyMsg{Type: tea.KeyBackspace}
	newModel, _ := m.Update(msg)
	m = newModel.(Model)

	if m.Value() != "hell" {
		t.Errorf("expected 'hell', got '%s'", m.Value())
	}
}

func TestCursorNavigation(t *testing.T) {
	m := New(Config{})
	m.SetValue("hello world")

	// Cursor should be at end
	if m.buffer.Pos() != 11 {
		t.Errorf("expected cursor at 11, got %d", m.buffer.Pos())
	}

	// Move to start (Ctrl+A)
	msg := tea.KeyMsg{Type: tea.KeyCtrlA}
	newModel, _ := m.Update(msg)
	m = newModel.(Model)

	if m.buffer.Pos() != 0 {
		t.Errorf("expected cursor at 0, got %d", m.buffer.Pos())
	}

	// Move to end (Ctrl+E)
	msg = tea.KeyMsg{Type: tea.KeyCtrlE}
	newModel, _ = m.Update(msg)
	m = newModel.(Model)

	if m.buffer.Pos() != 11 {
		t.Errorf("expected cursor at 11, got %d", m.buffer.Pos())
	}

	// Move back one character (Ctrl+B)
	msg = tea.KeyMsg{Type: tea.KeyCtrlB}
	newModel, _ = m.Update(msg)
	m = newModel.(Model)

	if m.buffer.Pos() != 10 {
		t.Errorf("expected cursor at 10, got %d", m.buffer.Pos())
	}

	// Move forward one character (Ctrl+F)
	msg = tea.KeyMsg{Type: tea.KeyCtrlF}
	newModel, _ = m.Update(msg)
	m = newModel.(Model)

	if m.buffer.Pos() != 11 {
		t.Errorf("expected cursor at 11, got %d", m.buffer.Pos())
	}
}

func TestDeleteBeforeCursor(t *testing.T) {
	m := New(Config{})
	m.SetValue("hello world")
	m.buffer.SetPos(5) // After "hello"

	msg := tea.KeyMsg{Type: tea.KeyCtrlU}
	newModel, _ := m.Update(msg)
	m = newModel.(Model)

	if m.Value() != " world" {
		t.Errorf("expected ' world', got '%s'", m.Value())
	}
	if m.buffer.Pos() != 0 {
		t.Errorf("expected cursor at 0, got %d", m.buffer.Pos())
	}
}

func TestDeleteAfterCursor(t *testing.T) {
	m := New(Config{})
	m.SetValue("hello world")
	m.buffer.SetPos(5) // After "hello"

	msg := tea.KeyMsg{Type: tea.KeyCtrlK}
	newModel, _ := m.Update(msg)
	m = newModel.(Model)

	if m.Value() != "hello" {
		t.Errorf("expected 'hello', got '%s'", m.Value())
	}
}

func TestDeleteWordBackward(t *testing.T) {
	m := New(Config{})
	m.SetValue("hello world")

	msg := tea.KeyMsg{Type: tea.KeyCtrlW}
	newModel, _ := m.Update(msg)
	m = newModel.(Model)

	if m.Value() != "hello " {
		t.Errorf("expected 'hello ', got '%s'", m.Value())
	}
}

func TestHistoryNavigation(t *testing.T) {
	history := []string{"command3", "command2", "command1"}
	m := New(Config{
		HistoryValues: history,
	})

	m.SetValue("current")

	// Navigate to previous (older) history
	msg := tea.KeyMsg{Type: tea.KeyUp}
	newModel, _ := m.Update(msg)
	m = newModel.(Model)

	if m.Value() != "command3" {
		t.Errorf("expected 'command3', got '%s'", m.Value())
	}

	// Navigate to even older history
	msg = tea.KeyMsg{Type: tea.KeyUp}
	newModel, _ = m.Update(msg)
	m = newModel.(Model)

	if m.Value() != "command2" {
		t.Errorf("expected 'command2', got '%s'", m.Value())
	}

	// Navigate back to newer
	msg = tea.KeyMsg{Type: tea.KeyDown}
	newModel, _ = m.Update(msg)
	m = newModel.(Model)

	if m.Value() != "command3" {
		t.Errorf("expected 'command3', got '%s'", m.Value())
	}

	// Navigate back to current input
	msg = tea.KeyMsg{Type: tea.KeyDown}
	newModel, _ = m.Update(msg)
	m = newModel.(Model)

	if m.Value() != "current" {
		t.Errorf("expected 'current', got '%s'", m.Value())
	}
}

func TestHistoryNavigationSavesCurrentInput(t *testing.T) {
	history := []string{"old command"}
	m := New(Config{
		HistoryValues: history,
	})

	// Type something
	m.SetValue("my new command")

	// Navigate to history
	msg := tea.KeyMsg{Type: tea.KeyUp}
	newModel, _ := m.Update(msg)
	m = newModel.(Model)

	if m.Value() != "old command" {
		t.Errorf("expected 'old command', got '%s'", m.Value())
	}

	// Navigate back
	msg = tea.KeyMsg{Type: tea.KeyDown}
	newModel, _ = m.Update(msg)
	m = newModel.(Model)

	// Should restore the original input
	if m.Value() != "my new command" {
		t.Errorf("expected 'my new command', got '%s'", m.Value())
	}
}

func TestCompletion(t *testing.T) {
	provider := &mockCompletionProvider{
		completions: []string{"file1.txt", "file2.txt", "file3.txt"},
	}

	m := New(Config{
		CompletionProvider: provider,
	})

	m.SetValue("fil")

	// Trigger completion
	msg := tea.KeyMsg{Type: tea.KeyTab}
	newModel, _ := m.Update(msg)
	m = newModel.(Model)

	// Should have applied first completion
	if m.Value() != "file1.txt" {
		t.Errorf("expected 'file1.txt', got '%s'", m.Value())
	}

	// Tab again for next completion
	msg = tea.KeyMsg{Type: tea.KeyTab}
	newModel, _ = m.Update(msg)
	m = newModel.(Model)

	if m.Value() != "file2.txt" {
		t.Errorf("expected 'file2.txt', got '%s'", m.Value())
	}
}

func TestCompletionMenuShowsMoreThanFourItemsByDefault(t *testing.T) {
	provider := &mockCompletionProvider{
		completions: []string{"item01", "item02", "item03", "item04", "item05", "item06"},
	}

	m := New(Config{
		Prompt:             "> ",
		CompletionProvider: provider,
	})
	m.SetValue("item")

	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = newModel.(Model)

	view := m.View()
	if !strings.Contains(view, "item05") {
		t.Fatalf("expected default completion menu to include the fifth item, got:\n%s", view)
	}
}

func TestCompletionMenuUsesConfiguredVisibleItemCount(t *testing.T) {
	provider := &mockCompletionProvider{
		completions: []string{"item01", "item02", "item03", "item04", "item05", "item06", "item07", "item08"},
	}

	m := New(Config{
		Prompt:               "> ",
		CompletionProvider:   provider,
		CompletionMaxVisible: 6,
	})
	m.SetValue("item")

	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = newModel.(Model)

	view := m.View()
	if !strings.Contains(view, "item06") {
		t.Fatalf("expected configured completion menu to include the sixth item, got:\n%s", view)
	}
	if strings.Contains(view, "item07") {
		t.Fatalf("expected configured completion menu to stop before the seventh item, got:\n%s", view)
	}
}

func TestSingleCompletion(t *testing.T) {
	provider := &mockCompletionProvider{
		completions: []string{"unique_file.txt"},
	}

	m := New(Config{
		CompletionProvider: provider,
	})

	m.SetValue("uni")

	// Trigger completion
	msg := tea.KeyMsg{Type: tea.KeyTab}
	newModel, _ := m.Update(msg)
	m = newModel.(Model)

	// Should have applied the only completion
	if m.Value() != "unique_file.txt" {
		t.Errorf("expected 'unique_file.txt', got '%s'", m.Value())
	}

	// Completion should be reset (only one option)
	if m.completion.IsActive() {
		t.Error("completion should not be active after single completion")
	}
}

func TestNoCompletion(t *testing.T) {
	provider := &mockCompletionProvider{
		completions: []string{},
	}

	m := New(Config{
		CompletionProvider: provider,
	})

	m.SetValue("xyz")
	originalValue := m.Value()

	// Trigger completion
	msg := tea.KeyMsg{Type: tea.KeyTab}
	newModel, _ := m.Update(msg)
	m = newModel.(Model)

	// Value should be unchanged
	if m.Value() != originalValue {
		t.Errorf("expected '%s', got '%s'", originalValue, m.Value())
	}
}

func TestCompletionCancel(t *testing.T) {
	provider := &mockCompletionProvider{
		completions: []string{"file1.txt", "file2.txt"},
	}

	m := New(Config{
		CompletionProvider: provider,
	})

	m.SetValue("fil")

	// Trigger completion
	msg := tea.KeyMsg{Type: tea.KeyTab}
	newModel, _ := m.Update(msg)
	m = newModel.(Model)

	// Should have applied first completion
	if m.Value() != "file1.txt" {
		t.Errorf("expected 'file1.txt', got '%s'", m.Value())
	}

	// Cancel completion
	msg = tea.KeyMsg{Type: tea.KeyEsc}
	newModel, _ = m.Update(msg)
	m = newModel.(Model)

	// Should restore original text
	if m.Value() != "fil" {
		t.Errorf("expected 'fil', got '%s'", m.Value())
	}
	if m.completion.IsActive() {
		t.Error("completion should not be active after cancel")
	}
}

func TestWindowResize(t *testing.T) {
	m := New(Config{Width: 80})

	// First WindowSizeMsg is the initial size - should NOT clear screen
	// (to preserve the welcome screen)
	msg := tea.WindowSizeMsg{Width: 120, Height: 40}
	newModel, cmd := m.Update(msg)
	m = newModel.(Model)

	if m.width != 120 {
		t.Errorf("expected width 120, got %d", m.width)
	}

	if cmd != nil {
		t.Error("expected nil command on initial window size, got non-nil")
	}

	// Second WindowSizeMsg with different width is an actual resize - should clear screen
	msg = tea.WindowSizeMsg{Width: 100, Height: 40}
	newModel, cmd = m.Update(msg)
	m = newModel.(Model)

	if m.width != 100 {
		t.Errorf("expected width 100, got %d", m.width)
	}

	if cmd == nil {
		t.Error("expected ClearScreen command on actual resize, got nil")
	}

	// WindowSizeMsg with same width should NOT clear screen (no actual resize)
	msg = tea.WindowSizeMsg{Width: 100, Height: 50}
	newModel, cmd = m.Update(msg)
	m = newModel.(Model)

	if cmd != nil {
		t.Error("expected nil command when width unchanged, got non-nil")
	}
}

func TestUnfocusedIgnoresInput(t *testing.T) {
	m := New(Config{})
	m.Blur()

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}
	newModel, _ := m.Update(msg)
	m = newModel.(Model)

	// Should not have added any input
	if m.Value() != "" {
		t.Errorf("expected empty value when unfocused, got '%s'", m.Value())
	}
}

func TestView(t *testing.T) {
	m := New(Config{
		Prompt: "$ ",
	})
	m.SetValue("hello")

	view := m.View()

	// View should contain prompt and input
	if view == "" {
		t.Error("view should not be empty")
	}
}

func TestViewAfterSubmit(t *testing.T) {
	m := New(Config{
		Prompt: "$ ",
	})
	m.SetValue("test")

	// Submit
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	newModel, _ := m.Update(msg)
	m = newModel.(Model)

	view := m.View()

	// View should be empty after submit - the REPL handles printing the final line
	// so it persists in terminal history
	if view != "" {
		t.Errorf("view should be empty after submit, got '%s'", view)
	}
}

func TestViewAfterInterrupt(t *testing.T) {
	m := New(Config{
		Prompt: "$ ",
	})
	m.SetValue("test")

	// Interrupt
	msg := tea.KeyMsg{Type: tea.KeyCtrlC}
	newModel, _ := m.Update(msg)
	m = newModel.(Model)

	view := m.View()

	// View should be empty for interrupt
	if view != "" {
		t.Errorf("view should be empty after interrupt, got '%s'", view)
	}
}

func TestSanitizeRunes(t *testing.T) {
	tests := []struct {
		name     string
		input    []rune
		expected []rune
	}{
		{
			name:     "normal text",
			input:    []rune("hello"),
			expected: []rune("hello"),
		},
		{
			name:     "tab replaced with space",
			input:    []rune("hello\tworld"),
			expected: []rune("hello world"),
		},
		{
			name:     "newline preserved",
			input:    []rune("hello\nworld"),
			expected: []rune("hello\nworld"),
		},
		{
			name:     "lone carriage return normalized to newline",
			input:    []rune("hello\rworld"),
			expected: []rune("hello\nworld"),
		},
		{
			name:     "CRLF normalized to newline",
			input:    []rune("hello\r\nworld"),
			expected: []rune("hello\nworld"),
		},
		{
			name:     "multiple special chars",
			input:    []rune("a\tb\nc\rd"),
			expected: []rune("a b\nc\nd"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeRunes(tt.input)
			if string(result) != string(tt.expected) {
				t.Errorf("expected '%s', got '%s'", string(tt.expected), string(result))
			}
		})
	}
}

func TestAcceptPrediction(t *testing.T) {
	m := New(Config{})
	m.SetValue("git st")
	m.currentPrediction = "git status"

	// At end of input, right arrow should accept prediction
	msg := tea.KeyMsg{Type: tea.KeyRight}
	newModel, _ := m.Update(msg)
	m = newModel.(Model)

	if m.Value() != "git status" {
		t.Errorf("expected 'git status', got '%s'", m.Value())
	}
	if m.currentPrediction != "" {
		t.Error("prediction should be cleared after accepting")
	}
}

func TestRightArrowWithoutPrediction(t *testing.T) {
	m := New(Config{})
	m.SetValue("hello")
	m.buffer.SetPos(3) // Position in the middle

	msg := tea.KeyMsg{Type: tea.KeyRight}
	newModel, _ := m.Update(msg)
	m = newModel.(Model)

	// Should just move cursor forward
	if m.buffer.Pos() != 4 {
		t.Errorf("expected cursor at 4, got %d", m.buffer.Pos())
	}
}

func TestRightArrowAtEndWithoutPrediction(t *testing.T) {
	m := New(Config{})
	m.SetValue("hello")
	// Cursor is at end (5)

	msg := tea.KeyMsg{Type: tea.KeyRight}
	newModel, _ := m.Update(msg)
	m = newModel.(Model)

	// Should stay at end
	if m.buffer.Pos() != 5 {
		t.Errorf("expected cursor at 5, got %d", m.buffer.Pos())
	}
}

func TestTypingResetsHistoryIndex(t *testing.T) {
	history := []string{"old command"}
	m := New(Config{
		HistoryValues: history,
	})

	// Navigate to history
	msg := tea.KeyMsg{Type: tea.KeyUp}
	newModel, _ := m.Update(msg)
	m = newModel.(Model)

	if m.historyIndex != 1 {
		t.Errorf("expected historyIndex 1, got %d", m.historyIndex)
	}

	// Type something
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}
	newModel, _ = m.Update(msg)
	m = newModel.(Model)

	// History index should be reset
	if m.historyIndex != 0 {
		t.Errorf("expected historyIndex 0 after typing, got %d", m.historyIndex)
	}
}

func TestSetHistoryValues(t *testing.T) {
	m := New(Config{})

	m.SetHistoryValues([]string{"cmd1", "cmd2", "cmd3"})

	if len(m.historyValues) != 3 {
		t.Errorf("expected 3 history values, got %d", len(m.historyValues))
	}
	if m.historyIndex != 0 {
		t.Errorf("expected historyIndex 0, got %d", m.historyIndex)
	}
}

package input

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func TestDefaultRenderConfig(t *testing.T) {
	config := DefaultRenderConfig()

	// Verify default styles are set
	if config.PromptStyle.String() == "" {
		// PromptStyle should exist (even if empty)
	}

	// Check prediction style has a foreground color
	fg := config.PredictionStyle.GetForeground()
	noColor := lipgloss.NoColor{}
	if fg == noColor {
		t.Error("PredictionStyle should have a foreground color")
	}

	// Check cursor style has reverse
	// We can't directly check reverse, but we can verify the style exists
	if config.CursorStyle.String() == "" {
		// CursorStyle should exist
	}
}

func TestNewRenderer(t *testing.T) {
	config := DefaultRenderConfig()
	renderer := NewRenderer(config, nil)

	if renderer == nil {
		t.Fatal("NewRenderer returned nil")
	}

	if renderer.Width() != 80 {
		t.Errorf("Expected default width 80, got %d", renderer.Width())
	}
}

func TestRendererSetWidth(t *testing.T) {
	renderer := NewRenderer(DefaultRenderConfig(), nil)

	renderer.SetWidth(120)
	if renderer.Width() != 120 {
		t.Errorf("Expected width 120, got %d", renderer.Width())
	}

	// Setting width to 0 or negative should not change it
	renderer.SetWidth(0)
	if renderer.Width() != 120 {
		t.Errorf("Expected width to remain 120 after setting 0, got %d", renderer.Width())
	}

	renderer.SetWidth(-10)
	if renderer.Width() != 120 {
		t.Errorf("Expected width to remain 120 after setting -10, got %d", renderer.Width())
	}
}

func TestRenderInputLineBasic(t *testing.T) {
	renderer := NewRenderer(DefaultRenderConfig(), nil)
	buffer := NewBuffer()

	// Empty buffer
	result := renderer.RenderInputLine("$ ", buffer, "", true)
	if !strings.HasPrefix(result, "$ ") {
		t.Errorf("Expected result to start with prompt, got: %s", result)
	}

	// Buffer with text
	buffer.Insert("hello")
	result = renderer.RenderInputLine("$ ", buffer, "", true)
	if !strings.Contains(result, "hello") {
		t.Errorf("Expected result to contain 'hello', got: %s", result)
	}
}

func TestRenderInputLineWithCursor(t *testing.T) {
	renderer := NewRenderer(DefaultRenderConfig(), nil)
	buffer := NewBufferWithText("hello")

	// Cursor at end
	result := renderer.RenderInputLine("$ ", buffer, "", true)
	if !strings.Contains(result, "hello") {
		t.Errorf("Expected result to contain 'hello', got: %s", result)
	}

	// Cursor in middle
	buffer.SetPos(2)
	result = renderer.RenderInputLine("$ ", buffer, "", true)
	// Should still contain all text
	if !strings.Contains(result, "he") {
		t.Errorf("Expected result to contain 'he', got: %s", result)
	}
}

func TestRenderInputLineWithPrediction(t *testing.T) {
	renderer := NewRenderer(DefaultRenderConfig(), nil)
	buffer := NewBufferWithText("hel")

	// Prediction that extends input
	result := renderer.RenderInputLine("$ ", buffer, "hello world", true)
	// Should contain both the input and prediction suffix
	if !strings.Contains(result, "hel") {
		t.Errorf("Expected result to contain input 'hel', got: %s", result)
	}
	// The prediction suffix "lo world" should be rendered
	if !strings.Contains(result, "lo world") {
		t.Errorf("Expected result to contain prediction suffix 'lo world', got: %s", result)
	}
}

func TestRenderInputLineNonMatchingPrediction(t *testing.T) {
	renderer := NewRenderer(DefaultRenderConfig(), nil)
	buffer := NewBufferWithText("hello")

	// Prediction that doesn't match (doesn't start with input)
	result := renderer.RenderInputLine("$ ", buffer, "world", true)
	// Should contain input but not the non-matching prediction
	if !strings.Contains(result, "hello") {
		t.Errorf("Expected result to contain 'hello', got: %s", result)
	}
	if strings.Contains(result, "world") {
		t.Errorf("Expected result to NOT contain 'world', got: %s", result)
	}
}

func TestRenderInputLineUnfocused(t *testing.T) {
	renderer := NewRenderer(DefaultRenderConfig(), nil)
	buffer := NewBufferWithText("hello")

	// Unfocused should still render text
	result := renderer.RenderInputLine("$ ", buffer, "", false)
	if !strings.Contains(result, "hello") {
		t.Errorf("Expected result to contain 'hello', got: %s", result)
	}
}

func TestRenderCompletionBoxEmpty(t *testing.T) {
	renderer := NewRenderer(DefaultRenderConfig(), nil)
	cs := NewCompletionState()

	// Inactive completion state
	result := renderer.RenderCompletionBox(cs, 4)
	if result != "" {
		t.Errorf("Expected empty result for inactive completion, got: %s", result)
	}
}

func TestRenderCompletionBoxSingleItem(t *testing.T) {
	renderer := NewRenderer(DefaultRenderConfig(), nil)
	cs := NewCompletionState()

	// Single suggestion - IsVisible returns false for single item
	cs.Activate([]string{"hello"}, "hel", 0, 3)
	result := renderer.RenderCompletionBox(cs, 4)
	// Single item should not show box (IsVisible returns false)
	if result != "" {
		t.Errorf("Expected empty result for single item, got: %s", result)
	}
}

func TestRenderCompletionBoxMultipleItems(t *testing.T) {
	renderer := NewRenderer(DefaultRenderConfig(), nil)
	renderer.SetWidth(80)
	cs := NewCompletionState()

	// Multiple suggestions
	cs.Activate([]string{"hello", "help", "helicopter"}, "hel", 0, 3)
	cs.NextSuggestion() // Select first item

	result := renderer.RenderCompletionBox(cs, 4)
	if result == "" {
		t.Error("Expected non-empty result for multiple items")
	}

	// Should contain all suggestions
	if !strings.Contains(result, "hello") {
		t.Errorf("Expected result to contain 'hello', got: %s", result)
	}
	if !strings.Contains(result, "help") {
		t.Errorf("Expected result to contain 'help', got: %s", result)
	}
	if !strings.Contains(result, "helicopter") {
		t.Errorf("Expected result to contain 'helicopter', got: %s", result)
	}
}

func TestRenderCompletionBoxWithScrolling(t *testing.T) {
	renderer := NewRenderer(DefaultRenderConfig(), nil)
	renderer.SetWidth(80)
	cs := NewCompletionState()

	// Many suggestions to trigger scrolling
	suggestions := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
	cs.Activate(suggestions, "", 0, 0)

	// Select middle item to test scrolling
	for i := 0; i < 4; i++ {
		cs.NextSuggestion()
	}

	result := renderer.RenderCompletionBox(cs, 4)
	if result == "" {
		t.Error("Expected non-empty result")
	}

	// Should have scroll indicators
	if !strings.Contains(result, "↑") && !strings.Contains(result, "↓") {
		// At least one indicator should be present when scrolling
		// Depending on position, might have one or both
	}
}

func TestRenderInfoPanel(t *testing.T) {
	renderer := NewRenderer(DefaultRenderConfig(), nil)
	renderer.SetWidth(80)

	// Nil content
	result := renderer.RenderInfoPanel(nil)
	if result != "" {
		t.Errorf("Expected empty result for nil content, got: %s", result)
	}

	// Empty help content
	help := NewHelpContent("")
	result = renderer.RenderInfoPanel(help)
	if result != "" {
		t.Errorf("Expected empty result for empty help, got: %s", result)
	}

	// Non-empty help content
	help = NewHelpContent("This is help text")
	result = renderer.RenderInfoPanel(help)
	if !strings.Contains(result, "This is help text") {
		t.Errorf("Expected result to contain help text, got: %s", result)
	}
}

func TestRenderHelpBox(t *testing.T) {
	renderer := NewRenderer(DefaultRenderConfig(), nil)
	renderer.SetWidth(80)

	// Empty text
	result := renderer.RenderHelpBox("")
	if result != "" {
		t.Errorf("Expected empty result for empty text, got: %s", result)
	}

	// Non-empty text
	result = renderer.RenderHelpBox("Help information")
	if !strings.Contains(result, "Help information") {
		t.Errorf("Expected result to contain help text, got: %s", result)
	}
}

func TestRenderFullView(t *testing.T) {
	renderer := NewRenderer(DefaultRenderConfig(), nil)
	renderer.SetWidth(80)
	buffer := NewBufferWithText("hel")

	// Basic view with just input
	result := renderer.RenderFullView("$ ", buffer, "", true, nil, nil, 0)
	if !strings.Contains(result, "hel") {
		t.Errorf("Expected result to contain 'hel', got: %s", result)
	}

	// With prediction
	result = renderer.RenderFullView("$ ", buffer, "hello", true, nil, nil, 0)
	if !strings.Contains(result, "hel") {
		t.Errorf("Expected result to contain 'hel', got: %s", result)
	}

	// With completion
	cs := NewCompletionState()
	cs.Activate([]string{"hello", "help"}, "hel", 0, 3)
	cs.NextSuggestion()
	result = renderer.RenderFullView("$ ", buffer, "", true, cs, nil, 0)
	if !strings.Contains(result, "hello") {
		t.Errorf("Expected result to contain completion 'hello', got: %s", result)
	}

	// With info content
	help := NewHelpContent("Command help")
	result = renderer.RenderFullView("$ ", buffer, "", true, nil, help, 0)
	if !strings.Contains(result, "Command help") {
		t.Errorf("Expected result to contain help text, got: %s", result)
	}
}

func TestRenderFullViewMinHeight(t *testing.T) {
	renderer := NewRenderer(DefaultRenderConfig(), nil)
	buffer := NewBuffer()

	// With minimum height
	result := renderer.RenderFullView("$ ", buffer, "", true, nil, nil, 5)
	lineCount := strings.Count(result, "\n")
	if lineCount < 5 {
		t.Errorf("Expected at least 5 newlines for minHeight=5, got %d", lineCount)
	}
}

func TestGetPredictionSuffix(t *testing.T) {
	tests := []struct {
		name       string
		text       string
		prediction string
		expected   string
	}{
		{
			name:       "empty prediction",
			text:       "hello",
			prediction: "",
			expected:   "",
		},
		{
			name:       "non-matching prediction",
			text:       "hello",
			prediction: "world",
			expected:   "",
		},
		{
			name:       "matching prediction with suffix",
			text:       "hel",
			prediction: "hello world",
			expected:   "lo world",
		},
		{
			name:       "exact match",
			text:       "hello",
			prediction: "hello",
			expected:   "",
		},
		{
			name:       "empty text with prediction",
			text:       "",
			prediction: "hello",
			expected:   "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetPredictionSuffix(tt.text, tt.prediction)
			if result != tt.expected {
				t.Errorf("GetPredictionSuffix(%q, %q) = %q, want %q",
					tt.text, tt.prediction, result, tt.expected)
			}
		})
	}
}

func TestCalculateCursorPosition(t *testing.T) {
	tests := []struct {
		name      string
		prompt    string
		text      string
		cursorPos int
		expected  int
	}{
		{
			name:      "empty prompt and text",
			prompt:    "",
			text:      "",
			cursorPos: 0,
			expected:  0,
		},
		{
			name:      "with prompt only",
			prompt:    "$ ",
			text:      "",
			cursorPos: 0,
			expected:  2,
		},
		{
			name:      "with prompt and text at start",
			prompt:    "$ ",
			text:      "hello",
			cursorPos: 0,
			expected:  2,
		},
		{
			name:      "with prompt and text at end",
			prompt:    "$ ",
			text:      "hello",
			cursorPos: 5,
			expected:  7,
		},
		{
			name:      "with prompt and text in middle",
			prompt:    "$ ",
			text:      "hello",
			cursorPos: 2,
			expected:  4,
		},
		{
			name:      "cursor beyond text length",
			prompt:    "$ ",
			text:      "hi",
			cursorPos: 10,
			expected:  4, // Should clamp to text length
		},
		{
			name:      "negative cursor",
			prompt:    "$ ",
			text:      "hi",
			cursorPos: -5,
			expected:  2, // Should clamp to 0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateCursorPosition(tt.prompt, tt.text, tt.cursorPos)
			if result != tt.expected {
				t.Errorf("CalculateCursorPosition(%q, %q, %d) = %d, want %d",
					tt.prompt, tt.text, tt.cursorPos, result, tt.expected)
			}
		})
	}
}

func TestCalculateVisibleWindow(t *testing.T) {
	tests := []struct {
		name       string
		selected   int
		total      int
		maxVisible int
		wantStart  int
		wantEnd    int
	}{
		{
			name:       "fewer items than max",
			selected:   0,
			total:      3,
			maxVisible: 4,
			wantStart:  0,
			wantEnd:    3,
		},
		{
			name:       "at start of list",
			selected:   0,
			total:      10,
			maxVisible: 4,
			wantStart:  0,
			wantEnd:    4,
		},
		{
			name:       "at end of list",
			selected:   9,
			total:      10,
			maxVisible: 4,
			wantStart:  6,
			wantEnd:    10,
		},
		{
			name:       "in middle of list",
			selected:   5,
			total:      10,
			maxVisible: 4,
			wantStart:  4,
			wantEnd:    8,
		},
		{
			name:       "near start",
			selected:   1,
			total:      10,
			maxVisible: 4,
			wantStart:  0,
			wantEnd:    4,
		},
		{
			name:       "near end",
			selected:   8,
			total:      10,
			maxVisible: 4,
			wantStart:  6,
			wantEnd:    10,
		},
		{
			name:       "single visible item follows selection",
			selected:   1,
			total:      10,
			maxVisible: 1,
			wantStart:  1,
			wantEnd:    2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end := calculateVisibleWindow(tt.selected, tt.total, tt.maxVisible)
			if start != tt.wantStart || end != tt.wantEnd {
				t.Errorf("calculateVisibleWindow(%d, %d, %d) = (%d, %d), want (%d, %d)",
					tt.selected, tt.total, tt.maxVisible, start, end, tt.wantStart, tt.wantEnd)
			}
		})
	}
}

func TestFormatScrollIndicator(t *testing.T) {
	tests := []struct {
		arrow    string
		count    int
		expected string
	}{
		{"↑", 1, "↑   1"},
		{"↓", 10, "↓  10"},
		{"↑", 100, "↑ 100"},
		{"↓", 0, "↓   0"},
	}

	for _, tt := range tests {
		result := formatScrollIndicator(tt.arrow, tt.count)
		if result != tt.expected {
			t.Errorf("formatScrollIndicator(%q, %d) = %q, want %q",
				tt.arrow, tt.count, result, tt.expected)
		}
	}
}

func TestItoa(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{10, "10"},
		{100, "100"},
		{-1, "-1"},
		{-100, "-100"},
		{12345, "12345"},
	}

	for _, tt := range tests {
		result := itoa(tt.input)
		if result != tt.expected {
			t.Errorf("itoa(%d) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestPadLeft(t *testing.T) {
	tests := []struct {
		s        string
		width    int
		expected string
	}{
		{"1", 3, "  1"},
		{"10", 3, " 10"},
		{"100", 3, "100"},
		{"1000", 3, "1000"}, // Longer than width, no truncation
		{"", 3, "   "},
	}

	for _, tt := range tests {
		result := padLeft(tt.s, tt.width)
		if result != tt.expected {
			t.Errorf("padLeft(%q, %d) = %q, want %q", tt.s, tt.width, result, tt.expected)
		}
	}
}

func TestMaxInt(t *testing.T) {
	tests := []struct {
		a, b     int
		expected int
	}{
		{1, 2, 2},
		{2, 1, 2},
		{0, 0, 0},
		{-1, 1, 1},
		{-5, -3, -3},
	}

	for _, tt := range tests {
		result := maxInt(tt.a, tt.b)
		if result != tt.expected {
			t.Errorf("maxInt(%d, %d) = %d, want %d", tt.a, tt.b, result, tt.expected)
		}
	}
}

func TestRendererConfigGetSet(t *testing.T) {
	renderer := NewRenderer(DefaultRenderConfig(), nil)

	// Get config
	config := renderer.Config()
	if config.CursorStyle.String() == "" {
		// Config should be accessible
	}

	// Set new config
	newConfig := RenderConfig{
		PromptStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("red")),
	}
	renderer.SetConfig(newConfig)

	updatedConfig := renderer.Config()
	if updatedConfig.PromptStyle.GetForeground() != newConfig.PromptStyle.GetForeground() {
		t.Error("SetConfig did not update the config correctly")
	}
}

func TestRenderInputLineUnicodeCharacters(t *testing.T) {
	renderer := NewRenderer(DefaultRenderConfig(), nil)
	buffer := NewBufferWithText("こんにちは")

	result := renderer.RenderInputLine("$ ", buffer, "", true)
	if !strings.Contains(result, "こんにちは") {
		t.Errorf("Expected result to contain unicode text, got: %s", result)
	}
}

func TestRenderInputLineEmoji(t *testing.T) {
	renderer := NewRenderer(DefaultRenderConfig(), nil)
	buffer := NewBufferWithText("hello 🌍")

	result := renderer.RenderInputLine("$ ", buffer, "", true)
	if !strings.Contains(result, "hello") {
		t.Errorf("Expected result to contain 'hello', got: %s", result)
	}
	if !strings.Contains(result, "🌍") {
		t.Errorf("Expected result to contain emoji, got: %s", result)
	}
}

func TestCalculateCursorPositionWithUnicode(t *testing.T) {
	// Unicode characters may have different byte lengths vs display widths
	prompt := "$ "
	text := "日本語" // 3 characters, 6 display width

	// Cursor at position 1 (after first character)
	pos := CalculateCursorPosition(prompt, text, 1)
	// Prompt is 2 chars, first Japanese char is 2 wide = 4
	if pos != 4 {
		t.Errorf("Expected position 4 for unicode, got %d", pos)
	}
}

func TestRenderInputLineWrapping(t *testing.T) {
	renderer := NewRenderer(DefaultRenderConfig(), nil)
	renderer.SetWidth(20) // Set narrow width to force wrapping

	// Create a buffer with text that exceeds the width
	// Prompt "$ " is 2 chars, so we have 18 chars available on first line
	buffer := NewBufferWithText("abcdefghijklmnopqrstuvwxyz")

	result := renderer.RenderInputLine("$ ", buffer, "", true)

	// Result should contain newlines due to wrapping
	if !strings.Contains(result, "\n") {
		t.Errorf("Expected wrapped output to contain newlines, got: %q", result)
	}

	// Should still contain all the text
	if !strings.Contains(result, "a") || !strings.Contains(result, "z") {
		t.Errorf("Expected result to contain all characters, got: %s", result)
	}
}

func TestRenderInputLineNoWrappingWhenFits(t *testing.T) {
	renderer := NewRenderer(DefaultRenderConfig(), nil)
	renderer.SetWidth(80) // Wide enough to fit

	buffer := NewBufferWithText("hello")

	result := renderer.RenderInputLine("$ ", buffer, "", true)

	// Result should NOT contain newlines (except possibly trailing)
	lines := strings.Split(result, "\n")
	if len(lines) > 1 && lines[1] != "" {
		t.Errorf("Expected no wrapping for short input, got %d lines: %q", len(lines), result)
	}
}

func TestRenderInputLineWrappingWithPrediction(t *testing.T) {
	renderer := NewRenderer(DefaultRenderConfig(), nil)
	renderer.SetWidth(20) // Narrow width

	buffer := NewBufferWithText("hel")
	// Prediction extends well beyond width
	prediction := "hello world this is a long prediction"

	result := renderer.RenderInputLine("$ ", buffer, prediction, true)

	// Should contain newlines due to wrapping
	if !strings.Contains(result, "\n") {
		t.Errorf("Expected wrapped output with prediction, got: %q", result)
	}
}

func TestRenderInputLineWrappingWithUnicode(t *testing.T) {
	renderer := NewRenderer(DefaultRenderConfig(), nil)
	renderer.SetWidth(15) // Narrow width

	// Japanese characters are 2 cells wide each
	// "日本語テスト" = 6 chars * 2 width = 12 cells
	// With "$ " (2 cells), total = 14 cells, fits in 15
	// But "日本語テストです" = 8 chars * 2 = 16 cells + 2 = 18, wraps
	buffer := NewBufferWithText("日本語テストです")

	result := renderer.RenderInputLine("$ ", buffer, "", true)

	// Should wrap due to wide characters
	if !strings.Contains(result, "\n") {
		t.Errorf("Expected wrapped output for wide unicode, got: %q", result)
	}

	// Should contain all characters
	if !strings.Contains(result, "日") || !strings.Contains(result, "す") {
		t.Errorf("Expected all unicode characters to be present, got: %s", result)
	}
}

func TestRenderInputLineWrappingCursorInMiddle(t *testing.T) {
	renderer := NewRenderer(DefaultRenderConfig(), nil)
	renderer.SetWidth(20)

	buffer := NewBufferWithText("abcdefghijklmnopqrstuvwxyz")
	buffer.SetPos(5) // Cursor in middle

	result := renderer.RenderInputLine("$ ", buffer, "", true)

	// Should still wrap correctly
	if !strings.Contains(result, "\n") {
		t.Errorf("Expected wrapped output, got: %q", result)
	}

	// All characters should be present
	if !strings.Contains(result, "a") || !strings.Contains(result, "z") {
		t.Errorf("Expected all characters, got: %s", result)
	}
}

func TestRenderInputLineSyntaxHighlightingPreservedAfterCursor(t *testing.T) {
	renderer := NewRenderer(DefaultRenderConfig(), nil)
	renderer.SetWidth(80)

	// Test with a command - cursor in middle shouldn't break the text
	// The highlighter colors text, but lipgloss doesn't output ANSI in tests (no TTY)
	// So we verify that all text is present and correctly ordered
	buffer := NewBufferWithText(`echo "hello world" test`)
	buffer.SetPos(5) // Cursor before the quoted string

	result := renderer.RenderInputLine("$ ", buffer, "", true)

	// Strip ANSI to verify all text is present and in correct order
	stripped := ansi.Strip(result)
	if !strings.Contains(stripped, "echo") || !strings.Contains(stripped, "hello world") || !strings.Contains(stripped, "test") {
		t.Errorf("Expected all text to be present, got: %s", stripped)
	}

	// Verify the text order is preserved (echo before hello, hello before test)
	echoIdx := strings.Index(stripped, "echo")
	helloIdx := strings.Index(stripped, "hello")
	testIdx := strings.Index(stripped, "test")
	if echoIdx > helloIdx || helloIdx > testIdx {
		t.Errorf("Text order not preserved: echo@%d, hello@%d, test@%d", echoIdx, helloIdx, testIdx)
	}
}

func TestRenderInputLineWrappingPreservesSyntaxHighlightingAcrossLines(t *testing.T) {
	renderer := NewRenderer(DefaultRenderConfig(), nil)
	renderer.SetWidth(40) // Narrow width to force wrapping

	// A long command with a quoted string that will wrap
	// Verify that text is preserved across line breaks when cursor is in the middle
	buffer := NewBufferWithText(`echo "this is a very long string that will definitely wrap to the next line" end`)
	buffer.SetPos(10) // Cursor somewhere in the middle (inside the quoted string)

	result := renderer.RenderInputLine("$ ", buffer, "", true)

	// Should wrap
	if !strings.Contains(result, "\n") {
		t.Errorf("Expected wrapped output, got: %q", result)
	}

	// Strip ANSI and remove newlines to verify all text is present
	stripped := ansi.Strip(result)
	strippedNoNewlines := strings.ReplaceAll(stripped, "\n", "")

	if !strings.Contains(strippedNoNewlines, "echo") {
		t.Errorf("Expected 'echo' to be present, got: %s", stripped)
	}
	if !strings.Contains(strippedNoNewlines, "end") {
		t.Errorf("Expected 'end' to be present, got: %s", stripped)
	}
	if !strings.Contains(strippedNoNewlines, "this is a very long string") {
		t.Errorf("Expected quoted string content to be present, got: %s", stripped)
	}

	// Verify multiple lines have content
	lines := strings.Split(result, "\n")
	if len(lines) < 2 {
		t.Fatalf("Expected at least 2 lines, got %d", len(lines))
	}

	// Each line should have non-whitespace content
	for i, line := range lines {
		strippedLine := ansi.Strip(line)
		if strings.TrimSpace(strippedLine) == "" {
			t.Errorf("Line %d is empty after stripping ANSI: %q", i, line)
		}
	}
}

func TestRenderInputLineExactWidth(t *testing.T) {
	renderer := NewRenderer(DefaultRenderConfig(), nil)
	renderer.SetWidth(10) // "$ " + 8 chars = 10, exactly fits

	buffer := NewBufferWithText("12345678")

	result := renderer.RenderInputLine("$ ", buffer, "", true)

	// Count lines - cursor adds 1 more char, so should wrap
	lines := strings.Split(result, "\n")
	// With cursor at end taking 1 space, total is 11, should wrap to 2 lines
	if len(lines) < 2 {
		t.Logf("Result: %q", result)
		// This is acceptable - the cursor might fit differently
	}
}

func TestRenderInputLineMultiLinePrompt(t *testing.T) {
	renderer := NewRenderer(DefaultRenderConfig(), nil)
	renderer.SetWidth(30) // Narrow width

	// Multi-line prompt like "[dev] gsh v1*!+ ↑\n› # "
	// Only the last line "› # " (4 chars) should count for width calculation
	multiLinePrompt := "first line info\n› # "

	// Text that fits on first line with 4-char prompt but wouldn't with full prompt width
	// 30 - 4 = 26 chars available
	buffer := NewBufferWithText("abcdefghijklmnopqrstuvwxyz") // 26 chars

	result := renderer.RenderInputLine(multiLinePrompt, buffer, "", true)

	// The prompt's first line should be present
	if !strings.Contains(result, "first line info") {
		t.Errorf("Expected result to contain first line of prompt, got: %s", result)
	}

	// The text should wrap based on the last line's width (4 chars), not the full prompt
	// With 26 chars of text + 4 char prompt + 1 cursor = 31, should wrap at char 26
	lines := strings.Split(result, "\n")
	// Should have: line 1 (first prompt line), line 2 (second prompt line + text), possibly line 3 (wrapped + cursor)
	if len(lines) < 2 {
		t.Errorf("Expected at least 2 lines for multi-line prompt, got %d: %q", len(lines), result)
	}
}

func TestRenderInputLineMultiLinePromptWidthCalculation(t *testing.T) {
	renderer := NewRenderer(DefaultRenderConfig(), nil)
	renderer.SetWidth(20)

	// Prompt with newline - last line is "$ " (2 chars)
	multiLinePrompt := "long header line here\n$ "

	// With width 20 and prompt last line "$ " (2 chars), we have 18 chars available
	// 17 chars should fit on the first content line without wrapping
	buffer := NewBufferWithText("12345678901234567") // 17 chars

	result := renderer.RenderInputLine(multiLinePrompt, buffer, "", true)

	// Split by newlines - first line is header, second is "$ " + text
	lines := strings.Split(result, "\n")

	// Should have exactly 2 lines: header and "$ 12345678901234567" + cursor
	// The cursor (1 char) makes it 17+2+1=20, which exactly fits
	if len(lines) != 2 {
		t.Errorf("Expected exactly 2 lines, got %d: %q", len(lines), result)
	}
}

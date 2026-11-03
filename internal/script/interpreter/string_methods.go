package interpreter

import (
	"fmt"
	"strings"
)

// String method implementations

// stringMethodImpl is a function type for string method implementations
type stringMethodImpl func(str *StringValue, args []Value) (Value, error)

// StringMethodValue wraps a string method that needs to be bound to an instance at call time
type StringMethodValue struct {
	Name string
	Impl stringMethodImpl
	Str  *StringValue // The string instance this method is bound to
}

func (s *StringMethodValue) Type() ValueType         { return ValueTypeTool }
func (s *StringMethodValue) String() string          { return fmt.Sprintf("<string method: %s>", s.Name) }
func (s *StringMethodValue) IsTruthy() bool          { return true }
func (s *StringMethodValue) Equals(other Value) bool { return false }

// stringToUpperImpl implements the toUpperCase method
func stringToUpperImpl(str *StringValue, args []Value) (Value, error) {
	return &StringValue{Value: strings.ToUpper(str.Value)}, nil
}

// stringToLowerImpl implements the toLowerCase method
func stringToLowerImpl(str *StringValue, args []Value) (Value, error) {
	return &StringValue{Value: strings.ToLower(str.Value)}, nil
}

// stringSplitImpl implements the split method
func stringSplitImpl(str *StringValue, args []Value) (Value, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("split() requires a separator argument")
	}
	if args[0].Type() != ValueTypeString {
		return nil, fmt.Errorf("split() separator must be a string")
	}

	separator := args[0].(*StringValue).Value
	parts := strings.Split(str.Value, separator)

	// Convert to array of string values
	elements := make([]Value, len(parts))
	for i, part := range parts {
		elements[i] = &StringValue{Value: part}
	}

	return &ArrayValue{Elements: elements}, nil
}

// stringTrimImpl implements the trim method
func stringTrimImpl(str *StringValue, args []Value) (Value, error) {
	return &StringValue{Value: strings.TrimSpace(str.Value)}, nil
}

// stringTrimStartImpl implements the trimStart method
func stringTrimStartImpl(str *StringValue, args []Value) (Value, error) {
	return &StringValue{Value: strings.TrimLeft(str.Value, " \t\n\r\v\f")}, nil
}

// stringTrimEndImpl implements the trimEnd method
func stringTrimEndImpl(str *StringValue, args []Value) (Value, error) {
	return &StringValue{Value: strings.TrimRight(str.Value, " \t\n\r\v\f")}, nil
}

// stringIndexOfImpl implements the indexOf method
func stringIndexOfImpl(str *StringValue, args []Value) (Value, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("indexOf() requires a search string argument")
	}
	if args[0].Type() != ValueTypeString {
		return nil, fmt.Errorf("indexOf() search string must be a string")
	}

	searchStr := args[0].(*StringValue).Value
	startIndex := 0

	// Optional start index parameter
	if len(args) > 1 {
		if args[1].Type() != ValueTypeNumber {
			return nil, fmt.Errorf("indexOf() start index must be a number")
		}
		startIndex = int(args[1].(*NumberValue).Value)
		if startIndex < 0 {
			startIndex = 0
		}
	}

	// Search from startIndex
	if startIndex >= len(str.Value) {
		return &NumberValue{Value: -1}, nil
	}

	index := strings.Index(str.Value[startIndex:], searchStr)
	if index == -1 {
		return &NumberValue{Value: -1}, nil
	}

	return &NumberValue{Value: float64(startIndex + index)}, nil
}

// stringLastIndexOfImpl implements the lastIndexOf method
func stringLastIndexOfImpl(str *StringValue, args []Value) (Value, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("lastIndexOf() requires a search string argument")
	}
	if args[0].Type() != ValueTypeString {
		return nil, fmt.Errorf("lastIndexOf() search string must be a string")
	}

	searchStr := args[0].(*StringValue).Value
	index := strings.LastIndex(str.Value, searchStr)
	return &NumberValue{Value: float64(index)}, nil
}

// stringSubstringImpl implements the substring method
func stringSubstringImpl(str *StringValue, args []Value) (Value, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("substring() requires at least one argument")
	}
	if args[0].Type() != ValueTypeNumber {
		return nil, fmt.Errorf("substring() start index must be a number")
	}

	runes := []rune(str.Value)
	length := len(runes)
	start := int(args[0].(*NumberValue).Value)

	// Clamp start to valid range
	if start < 0 {
		start = 0
	}
	if start > length {
		start = length
	}

	end := length
	if len(args) > 1 {
		if args[1].Type() != ValueTypeNumber {
			return nil, fmt.Errorf("substring() end index must be a number")
		}
		end = int(args[1].(*NumberValue).Value)
		// Clamp end to valid range
		if end < 0 {
			end = 0
		}
		if end > length {
			end = length
		}
	}

	// Ensure start <= end (swap if needed, as per JS substring behavior)
	if start > end {
		start, end = end, start
	}

	return &StringValue{Value: string(runes[start:end])}, nil
}

// stringSliceImpl implements the slice method
func stringSliceImpl(str *StringValue, args []Value) (Value, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("slice() requires at least one argument")
	}
	if args[0].Type() != ValueTypeNumber {
		return nil, fmt.Errorf("slice() start index must be a number")
	}

	runes := []rune(str.Value)
	length := len(runes)
	start := int(args[0].(*NumberValue).Value)

	// Handle negative start index
	if start < 0 {
		start = length + start
		if start < 0 {
			start = 0
		}
	}
	if start > length {
		start = length
	}

	end := length
	if len(args) > 1 {
		if args[1].Type() != ValueTypeNumber {
			return nil, fmt.Errorf("slice() end index must be a number")
		}
		end = int(args[1].(*NumberValue).Value)
		// Handle negative end index
		if end < 0 {
			end = length + end
			if end < 0 {
				end = 0
			}
		}
		if end > length {
			end = length
		}
	}

	// If start >= end, return empty string
	if start >= end {
		return &StringValue{Value: ""}, nil
	}

	return &StringValue{Value: string(runes[start:end])}, nil
}

// stringStartsWithImpl implements the startsWith method
func stringStartsWithImpl(str *StringValue, args []Value) (Value, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("startsWith() requires a search string argument")
	}
	if args[0].Type() != ValueTypeString {
		return nil, fmt.Errorf("startsWith() search string must be a string")
	}

	searchStr := args[0].(*StringValue).Value
	return &BoolValue{Value: strings.HasPrefix(str.Value, searchStr)}, nil
}

// stringEndsWithImpl implements the endsWith method
func stringEndsWithImpl(str *StringValue, args []Value) (Value, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("endsWith() requires a search string argument")
	}
	if args[0].Type() != ValueTypeString {
		return nil, fmt.Errorf("endsWith() search string must be a string")
	}

	searchStr := args[0].(*StringValue).Value
	return &BoolValue{Value: strings.HasSuffix(str.Value, searchStr)}, nil
}

// stringIncludesImpl implements the includes method
func stringIncludesImpl(str *StringValue, args []Value) (Value, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("includes() requires a search string argument")
	}
	if args[0].Type() != ValueTypeString {
		return nil, fmt.Errorf("includes() search string must be a string")
	}

	searchStr := args[0].(*StringValue).Value
	return &BoolValue{Value: strings.Contains(str.Value, searchStr)}, nil
}

// stringReplaceImpl implements the replace method (replaces first occurrence)
func stringReplaceImpl(str *StringValue, args []Value) (Value, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("replace() requires two arguments: search and replacement")
	}
	if args[0].Type() != ValueTypeString {
		return nil, fmt.Errorf("replace() search string must be a string")
	}
	if args[1].Type() != ValueTypeString {
		return nil, fmt.Errorf("replace() replacement string must be a string")
	}

	searchStr := args[0].(*StringValue).Value
	replaceStr := args[1].(*StringValue).Value

	// Replace only the first occurrence
	result := strings.Replace(str.Value, searchStr, replaceStr, 1)
	return &StringValue{Value: result}, nil
}

// stringReplaceAllImpl implements the replaceAll method (replaces all occurrences)
func stringReplaceAllImpl(str *StringValue, args []Value) (Value, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("replaceAll() requires two arguments: search and replacement")
	}
	if args[0].Type() != ValueTypeString {
		return nil, fmt.Errorf("replaceAll() search string must be a string")
	}
	if args[1].Type() != ValueTypeString {
		return nil, fmt.Errorf("replaceAll() replacement string must be a string")
	}

	searchStr := args[0].(*StringValue).Value
	replaceStr := args[1].(*StringValue).Value

	// Replace all occurrences
	result := strings.ReplaceAll(str.Value, searchStr, replaceStr)
	return &StringValue{Value: result}, nil
}

// stringRepeatImpl implements the repeat method
func stringRepeatImpl(str *StringValue, args []Value) (Value, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("repeat() requires a count argument")
	}
	if args[0].Type() != ValueTypeNumber {
		return nil, fmt.Errorf("repeat() count must be a number")
	}

	count := int(args[0].(*NumberValue).Value)
	if count < 0 {
		return nil, fmt.Errorf("repeat() count must be non-negative")
	}

	return &StringValue{Value: strings.Repeat(str.Value, count)}, nil
}

// stringPadStartImpl implements the padStart method
func stringPadStartImpl(str *StringValue, args []Value) (Value, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("padStart() requires a target length argument")
	}
	if args[0].Type() != ValueTypeNumber {
		return nil, fmt.Errorf("padStart() target length must be a number")
	}

	targetLength := int(args[0].(*NumberValue).Value)
	padString := " " // Default pad string

	if len(args) > 1 {
		if args[1].Type() != ValueTypeString {
			return nil, fmt.Errorf("padStart() pad string must be a string")
		}
		padString = args[1].(*StringValue).Value
		if padString == "" {
			return str, nil
		}
	}

	runes := []rune(str.Value)
	currentLength := len(runes)

	if currentLength >= targetLength {
		return str, nil
	}

	padLength := targetLength - currentLength

	// Build the padding
	var padding strings.Builder
	for padding.Len() < padLength {
		for _, r := range padString {
			if padding.Len() >= padLength {
				break
			}
			padding.WriteRune(r)
		}
	}

	return &StringValue{Value: padding.String() + str.Value}, nil
}

// stringPadEndImpl implements the padEnd method
func stringPadEndImpl(str *StringValue, args []Value) (Value, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("padEnd() requires a target length argument")
	}
	if args[0].Type() != ValueTypeNumber {
		return nil, fmt.Errorf("padEnd() target length must be a number")
	}

	targetLength := int(args[0].(*NumberValue).Value)
	padString := " " // Default pad string

	if len(args) > 1 {
		if args[1].Type() != ValueTypeString {
			return nil, fmt.Errorf("padEnd() pad string must be a string")
		}
		padString = args[1].(*StringValue).Value
		if padString == "" {
			return str, nil
		}
	}

	runes := []rune(str.Value)
	currentLength := len(runes)

	if currentLength >= targetLength {
		return str, nil
	}

	padLength := targetLength - currentLength

	// Build the padding
	var padding strings.Builder
	for padding.Len() < padLength {
		for _, r := range padString {
			if padding.Len() >= padLength {
				break
			}
			padding.WriteRune(r)
		}
	}

	return &StringValue{Value: str.Value + padding.String()}, nil
}

// stringCharAtImpl implements the charAt method
func stringCharAtImpl(str *StringValue, args []Value) (Value, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("charAt() requires an index argument")
	}
	if args[0].Type() != ValueTypeNumber {
		return nil, fmt.Errorf("charAt() index must be a number")
	}

	runes := []rune(str.Value)
	index := int(args[0].(*NumberValue).Value)

	// Return empty string if index is out of bounds
	if index < 0 || index >= len(runes) {
		return &StringValue{Value: ""}, nil
	}

	return &StringValue{Value: string(runes[index])}, nil
}

package interpreter

import (
	"fmt"
	"math"
	"math/rand"
)

// registerGshSDK registers the gsh SDK object with all its properties
func (i *Interpreter) registerGshSDK() {
	// Create gsh.terminal object (read-only properties)
	terminalObj := &ObjectValue{
		Properties: map[string]*PropertyDescriptor{
			"width":  {Value: &DynamicValue{Get: func() Value { return &NumberValue{Value: float64(i.sdkConfig.GetTermWidth())} }}, ReadOnly: true},
			"height": {Value: &DynamicValue{Get: func() Value { return &NumberValue{Value: float64(i.sdkConfig.GetTermHeight())} }}, ReadOnly: true},
			"isTTY":  {Value: &BoolValue{Value: i.sdkConfig.IsTTY()}, ReadOnly: true},
		},
	}

	// Create gsh.logging object (read/write)
	loggingObj := &LoggingObjectValue{interp: i}

	// Create gsh.completion object (read-only object, writable settings)
	completionObj := &ObjectValue{
		Properties: map[string]*PropertyDescriptor{
			"maxVisibleItems": {
				Getter: func() Value {
					return &NumberValue{Value: float64(i.sdkConfig.GetCompletionMaxVisibleItems())}
				},
				Setter: func(value Value) error {
					num, ok := value.(*NumberValue)
					if !ok {
						return fmt.Errorf("gsh.completion.maxVisibleItems must be a number, got %s", value.Type())
					}
					if math.Trunc(num.Value) != num.Value {
						return fmt.Errorf("gsh.completion.maxVisibleItems must be an integer")
					}
					if num.Value > float64(int(^uint(0)>>1)) {
						return fmt.Errorf("gsh.completion.maxVisibleItems is too large")
					}
					return i.sdkConfig.SetCompletionMaxVisibleItems(int(num.Value))
				},
			},
		},
	}

	// Create gsh.lastAgentRequest object (read-only, but properties updated by system)
	lastAgentRequestObj := &DynamicValue{
		Get: func() Value { return i.sdkConfig.GetLastAgentRequest() },
	}

	// Create gsh.models object (available in both REPL and script mode)
	// We cache the ModelsObjectValue to ensure consistent SDKModelRef instances
	modelsObj := NewModelsObjectValue(i.sdkConfig.GetModels())

	// Create gsh.lastCommand object (dynamic, reads from REPL context)
	lastCommandObj := &DynamicValue{
		Get: func() Value {
			replCtx := i.sdkConfig.GetREPLContext()
			if replCtx == nil {
				return &NullValue{}
			}
			return &LastCommandObjectValue{lastCommand: replCtx.LastCommand}
		},
	}

	// Create gsh.prompt (dynamic, reads from REPL context)
	promptObj := &DynamicValue{
		Get: func() Value {
			replCtx := i.sdkConfig.GetREPLContext()
			if replCtx == nil || replCtx.PromptValue == nil {
				return &StringValue{Value: ""}
			}
			return replCtx.PromptValue
		},
	}

	// Create gsh.continuationPrompt (dynamic, reads from REPL context)
	continuationPromptObj := &DynamicValue{
		Get: func() Value {
			replCtx := i.sdkConfig.GetREPLContext()
			if replCtx == nil || replCtx.ContinuationPromptValue == nil {
				return &StringValue{Value: ""}
			}
			return replCtx.ContinuationPromptValue
		},
	}

	// Create gsh.tools object with native tool implementations
	toolsObj := i.createNativeToolsObject()

	// Create gsh.ui object for UI control (spinner, styles, cursor)
	uiObj := i.createUIObject()

	// Create gsh.history object for command history access
	historyObj := i.createHistoryObject()

	// Create gsh.currentDirectory (dynamic, reads from interpreter's working directory)
	currentDirectoryObj := &DynamicValue{
		Get: func() Value {
			return &StringValue{Value: i.GetWorkingDir()}
		},
	}

	// Create Math object with common methods and constants
	mathObj := &ObjectValue{
		Properties: map[string]*PropertyDescriptor{
			// Methods
			"random": {Value: &BuiltinValue{
				Name: "Math.random",
				Fn:   builtinMathRandom,
			}, ReadOnly: true},
			"floor": {Value: &BuiltinValue{
				Name: "Math.floor",
				Fn:   builtinMathFloor,
			}, ReadOnly: true},
			"ceil": {Value: &BuiltinValue{
				Name: "Math.ceil",
				Fn:   builtinMathCeil,
			}, ReadOnly: true},
			"round": {Value: &BuiltinValue{
				Name: "Math.round",
				Fn:   builtinMathRound,
			}, ReadOnly: true},
			"abs": {Value: &BuiltinValue{
				Name: "Math.abs",
				Fn:   builtinMathAbs,
			}, ReadOnly: true},
			"min": {Value: &BuiltinValue{
				Name: "Math.min",
				Fn:   builtinMathMin,
			}, ReadOnly: true},
			"max": {Value: &BuiltinValue{
				Name: "Math.max",
				Fn:   builtinMathMax,
			}, ReadOnly: true},
			"pow": {Value: &BuiltinValue{
				Name: "Math.pow",
				Fn:   builtinMathPow,
			}, ReadOnly: true},
			"sqrt": {Value: &BuiltinValue{
				Name: "Math.sqrt",
				Fn:   builtinMathSqrt,
			}, ReadOnly: true},
			"sin": {Value: &BuiltinValue{
				Name: "Math.sin",
				Fn:   builtinMathSin,
			}, ReadOnly: true},
			"cos": {Value: &BuiltinValue{
				Name: "Math.cos",
				Fn:   builtinMathCos,
			}, ReadOnly: true},
			"tan": {Value: &BuiltinValue{
				Name: "Math.tan",
				Fn:   builtinMathTan,
			}, ReadOnly: true},
			"log": {Value: &BuiltinValue{
				Name: "Math.log",
				Fn:   builtinMathLog,
			}, ReadOnly: true},
			"log10": {Value: &BuiltinValue{
				Name: "Math.log10",
				Fn:   builtinMathLog10,
			}, ReadOnly: true},
			"log2": {Value: &BuiltinValue{
				Name: "Math.log2",
				Fn:   builtinMathLog2,
			}, ReadOnly: true},
			"exp": {Value: &BuiltinValue{
				Name: "Math.exp",
				Fn:   builtinMathExp,
			}, ReadOnly: true},
			// Constants
			"PI": {Value: &NumberValue{Value: math.Pi}, ReadOnly: true},
			"E":  {Value: &NumberValue{Value: math.E}, ReadOnly: true},
		},
	}

	// Create gsh object with SDK object value for custom property handling
	gshObj := &GshObjectValue{
		interp: i,
		baseProps: map[string]*PropertyDescriptor{
			"version":            {Value: &StringValue{Value: i.version}, ReadOnly: true},
			"terminal":           {Value: terminalObj, ReadOnly: true},
			"logging":            {Value: loggingObj},
			"completion":         {Value: completionObj, ReadOnly: true},
			"lastAgentRequest":   {Value: lastAgentRequestObj, ReadOnly: true},
			"tools":              {Value: toolsObj, ReadOnly: true},
			"ui":                 {Value: uiObj, ReadOnly: true},
			"models":             {Value: modelsObj, ReadOnly: true},
			"lastCommand":        {Value: lastCommandObj, ReadOnly: true},
			"history":            {Value: historyObj, ReadOnly: true},
			"currentDirectory":   {Value: currentDirectoryObj, ReadOnly: true},
			"prompt":             {Value: promptObj},
			"continuationPrompt": {Value: continuationPromptObj},
			"use": {Value: &BuiltinValue{
				Name: "gsh.use",
				Fn:   i.builtinGshUse,
			}, ReadOnly: true},
			"remove": {Value: &BuiltinValue{
				Name: "gsh.remove",
				Fn:   i.builtinGshRemove,
			}, ReadOnly: true},
			"removeAll": {Value: &BuiltinValue{
				Name: "gsh.removeAll",
				Fn:   i.builtinGshRemoveAll,
			}, ReadOnly: true},
		},
	}

	// Register Math as a global object (not under gsh)
	i.globalEnv.Set("Math", mathObj)

	// Register DateTime as a global object (not under gsh)
	dateTimeObj := createDateTimeObject()
	i.globalEnv.Set("DateTime", dateTimeObj)

	// Register Regexp as a global object (not under gsh)
	regexpObj := createRegexpObject()
	i.globalEnv.Set("Regexp", regexpObj)

	i.globalEnv.Set("gsh", gshObj)
}

// builtinGshUse implements gsh.use(event, handler)
// Registers a middleware handler for an event. Returns a unique handler ID.
// Middleware handlers have the signature: tool(ctx, next)
func (i *Interpreter) builtinGshUse(args []Value) (Value, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("gsh.use() takes 2 arguments (event: string, handler: tool), got %d", len(args))
	}

	eventName, ok := args[0].(*StringValue)
	if !ok {
		return nil, fmt.Errorf("gsh.use() first argument must be a string (event name), got %s", args[0].Type())
	}

	handler, ok := args[1].(*ToolValue)
	if !ok {
		return nil, fmt.Errorf("gsh.use() second argument must be a tool, got %s", args[1].Type())
	}

	// Validate tool has correct signature (ctx, next)
	if len(handler.Parameters) != 2 {
		return nil, fmt.Errorf("middleware handler must take 2 parameters (ctx, next), got %d", len(handler.Parameters))
	}

	// Register the handler and return the handler ID
	handlerID := i.eventManager.Use(eventName.Value, handler)
	return &StringValue{Value: handlerID}, nil
}

// builtinGshRemove implements gsh.remove(event, handler)
// Removes a previously registered middleware handler by tool reference.
func (i *Interpreter) builtinGshRemove(args []Value) (Value, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("gsh.remove() takes 2 arguments (event: string, handler: tool), got %d", len(args))
	}

	eventName, ok := args[0].(*StringValue)
	if !ok {
		return nil, fmt.Errorf("gsh.remove() first argument must be a string (event name), got %s", args[0].Type())
	}

	handler, ok := args[1].(*ToolValue)
	if !ok {
		return nil, fmt.Errorf("gsh.remove() second argument must be a tool, got %s", args[1].Type())
	}

	removed := i.eventManager.Remove(eventName.Value, handler)
	return &BoolValue{Value: removed}, nil
}

// builtinGshRemoveAll implements gsh.removeAll(event)
// Removes all registered handlers for an event. Returns the number of handlers removed.
func (i *Interpreter) builtinGshRemoveAll(args []Value) (Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("gsh.removeAll() takes 1 argument (event: string), got %d", len(args))
	}

	eventName, ok := args[0].(*StringValue)
	if !ok {
		return nil, fmt.Errorf("gsh.removeAll() argument must be a string (event name), got %s", args[0].Type())
	}

	count := i.eventManager.RemoveAll(eventName.Value)
	return &NumberValue{Value: float64(count)}, nil
}

// builtinMathRandom implements Math.random()
// Returns a random number between 0 (inclusive) and 1 (exclusive)
func builtinMathRandom(args []Value) (Value, error) {
	if len(args) != 0 {
		return nil, fmt.Errorf("Math.random() takes no arguments, got %d", len(args))
	}
	return &NumberValue{Value: rand.Float64()}, nil
}

// builtinMathFloor implements Math.floor()
// Returns the largest integer less than or equal to a given number
func builtinMathFloor(args []Value) (Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("Math.floor() takes exactly 1 argument, got %d", len(args))
	}

	numVal, ok := args[0].(*NumberValue)
	if !ok {
		return nil, fmt.Errorf("Math.floor() argument must be a number, got %s", args[0].Type())
	}

	return &NumberValue{Value: math.Floor(numVal.Value)}, nil
}

// builtinMathCeil implements Math.ceil()
// Returns the smallest integer greater than or equal to a given number
func builtinMathCeil(args []Value) (Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("Math.ceil() takes exactly 1 argument, got %d", len(args))
	}

	numVal, ok := args[0].(*NumberValue)
	if !ok {
		return nil, fmt.Errorf("Math.ceil() argument must be a number, got %s", args[0].Type())
	}

	return &NumberValue{Value: math.Ceil(numVal.Value)}, nil
}

// builtinMathRound implements Math.round()
// Returns the value of a number rounded to the nearest integer
func builtinMathRound(args []Value) (Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("Math.round() takes exactly 1 argument, got %d", len(args))
	}

	numVal, ok := args[0].(*NumberValue)
	if !ok {
		return nil, fmt.Errorf("Math.round() argument must be a number, got %s", args[0].Type())
	}

	return &NumberValue{Value: math.Round(numVal.Value)}, nil
}

// builtinMathAbs implements Math.abs()
// Returns the absolute value of a number
func builtinMathAbs(args []Value) (Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("Math.abs() takes exactly 1 argument, got %d", len(args))
	}

	numVal, ok := args[0].(*NumberValue)
	if !ok {
		return nil, fmt.Errorf("Math.abs() argument must be a number, got %s", args[0].Type())
	}

	return &NumberValue{Value: math.Abs(numVal.Value)}, nil
}

// builtinMathMin implements Math.min()
// Returns the smallest of zero or more numbers
func builtinMathMin(args []Value) (Value, error) {
	if len(args) == 0 {
		return &NumberValue{Value: math.Inf(1)}, nil
	}

	min := math.Inf(1)
	for _, arg := range args {
		numVal, ok := arg.(*NumberValue)
		if !ok {
			return nil, fmt.Errorf("Math.min() arguments must be numbers, got %s", arg.Type())
		}
		if numVal.Value < min {
			min = numVal.Value
		}
	}

	return &NumberValue{Value: min}, nil
}

// builtinMathMax implements Math.max()
// Returns the largest of zero or more numbers
func builtinMathMax(args []Value) (Value, error) {
	if len(args) == 0 {
		return &NumberValue{Value: math.Inf(-1)}, nil
	}

	max := math.Inf(-1)
	for _, arg := range args {
		numVal, ok := arg.(*NumberValue)
		if !ok {
			return nil, fmt.Errorf("Math.max() arguments must be numbers, got %s", arg.Type())
		}
		if numVal.Value > max {
			max = numVal.Value
		}
	}

	return &NumberValue{Value: max}, nil
}

// builtinMathPow implements Math.pow()
// Returns the base to the exponent power
func builtinMathPow(args []Value) (Value, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("Math.pow() takes exactly 2 arguments, got %d", len(args))
	}

	base, ok := args[0].(*NumberValue)
	if !ok {
		return nil, fmt.Errorf("Math.pow() first argument must be a number, got %s", args[0].Type())
	}

	exponent, ok := args[1].(*NumberValue)
	if !ok {
		return nil, fmt.Errorf("Math.pow() second argument must be a number, got %s", args[1].Type())
	}

	return &NumberValue{Value: math.Pow(base.Value, exponent.Value)}, nil
}

// builtinMathSqrt implements Math.sqrt()
// Returns the square root of a number
func builtinMathSqrt(args []Value) (Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("Math.sqrt() takes exactly 1 argument, got %d", len(args))
	}

	numVal, ok := args[0].(*NumberValue)
	if !ok {
		return nil, fmt.Errorf("Math.sqrt() argument must be a number, got %s", args[0].Type())
	}

	return &NumberValue{Value: math.Sqrt(numVal.Value)}, nil
}

// builtinMathSin implements Math.sin()
// Returns the sine of a number (in radians)
func builtinMathSin(args []Value) (Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("Math.sin() takes exactly 1 argument, got %d", len(args))
	}

	numVal, ok := args[0].(*NumberValue)
	if !ok {
		return nil, fmt.Errorf("Math.sin() argument must be a number, got %s", args[0].Type())
	}

	return &NumberValue{Value: math.Sin(numVal.Value)}, nil
}

// builtinMathCos implements Math.cos()
// Returns the cosine of a number (in radians)
func builtinMathCos(args []Value) (Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("Math.cos() takes exactly 1 argument, got %d", len(args))
	}

	numVal, ok := args[0].(*NumberValue)
	if !ok {
		return nil, fmt.Errorf("Math.cos() argument must be a number, got %s", args[0].Type())
	}

	return &NumberValue{Value: math.Cos(numVal.Value)}, nil
}

// builtinMathTan implements Math.tan()
// Returns the tangent of a number (in radians)
func builtinMathTan(args []Value) (Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("Math.tan() takes exactly 1 argument, got %d", len(args))
	}

	numVal, ok := args[0].(*NumberValue)
	if !ok {
		return nil, fmt.Errorf("Math.tan() argument must be a number, got %s", args[0].Type())
	}

	return &NumberValue{Value: math.Tan(numVal.Value)}, nil
}

// builtinMathLog implements Math.log()
// Returns the natural logarithm (base e) of a number
func builtinMathLog(args []Value) (Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("Math.log() takes exactly 1 argument, got %d", len(args))
	}

	numVal, ok := args[0].(*NumberValue)
	if !ok {
		return nil, fmt.Errorf("Math.log() argument must be a number, got %s", args[0].Type())
	}

	return &NumberValue{Value: math.Log(numVal.Value)}, nil
}

// builtinMathLog10 implements Math.log10()
// Returns the base-10 logarithm of a number
func builtinMathLog10(args []Value) (Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("Math.log10() takes exactly 1 argument, got %d", len(args))
	}

	numVal, ok := args[0].(*NumberValue)
	if !ok {
		return nil, fmt.Errorf("Math.log10() argument must be a number, got %s", args[0].Type())
	}

	return &NumberValue{Value: math.Log10(numVal.Value)}, nil
}

// builtinMathLog2 implements Math.log2()
// Returns the base-2 logarithm of a number
func builtinMathLog2(args []Value) (Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("Math.log2() takes exactly 1 argument, got %d", len(args))
	}

	numVal, ok := args[0].(*NumberValue)
	if !ok {
		return nil, fmt.Errorf("Math.log2() argument must be a number, got %s", args[0].Type())
	}

	return &NumberValue{Value: math.Log2(numVal.Value)}, nil
}

// builtinMathExp implements Math.exp()
// Returns e raised to the power of a number
func builtinMathExp(args []Value) (Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("Math.exp() takes exactly 1 argument, got %d", len(args))
	}

	numVal, ok := args[0].(*NumberValue)
	if !ok {
		return nil, fmt.Errorf("Math.exp() argument must be a number, got %s", args[0].Type())
	}

	return &NumberValue{Value: math.Exp(numVal.Value)}, nil
}

// LoggingObjectValue represents the gsh.logging object with dynamic properties
type LoggingObjectValue struct {
	interp *Interpreter
}

func (l *LoggingObjectValue) Type() ValueType { return ValueTypeObject }
func (l *LoggingObjectValue) String() string  { return "<gsh.logging>" }
func (l *LoggingObjectValue) IsTruthy() bool  { return true }
func (l *LoggingObjectValue) Equals(other Value) bool {
	_, ok := other.(*LoggingObjectValue)
	return ok
}

func (l *LoggingObjectValue) GetProperty(name string) Value {
	switch name {
	case "level":
		return &StringValue{Value: l.interp.sdkConfig.GetLogLevel()}
	case "file":
		file := l.interp.sdkConfig.GetLogFile()
		if file == "" {
			return &NullValue{}
		}
		return &StringValue{Value: file}
	default:
		return &NullValue{}
	}
}

func (l *LoggingObjectValue) SetProperty(name string, value Value) error {
	switch name {
	case "level":
		str, ok := value.(*StringValue)
		if !ok {
			return fmt.Errorf("gsh.logging.level must be a string, got %s", value.Type())
		}
		return l.interp.sdkConfig.SetLogLevel(str.Value)
	case "file":
		return fmt.Errorf("gsh.logging.file is read-only")
	default:
		return fmt.Errorf("cannot set property '%s' on gsh.logging", name)
	}
}

// DynamicValue represents a value with custom get/set behavior.
// It implements DynamicValueGetter so it can be unwrapped via UnwrapValue().
type DynamicValue struct {
	Get func() Value
	Set func(Value) error
}

func (d *DynamicValue) Type() ValueType { return ValueTypeObject }

// GetDynamicValue implements DynamicValueGetter interface.
// This allows UnwrapValue() to unwrap DynamicValue to its underlying value.
func (d *DynamicValue) GetDynamicValue() Value {
	if d.Get != nil {
		return d.Get()
	}
	return &NullValue{}
}
func (d *DynamicValue) String() string {
	if d.Get != nil {
		return d.Get().String()
	}
	return "<dynamic>"
}
func (d *DynamicValue) IsTruthy() bool {
	if d.Get != nil {
		return d.Get().IsTruthy()
	}
	return false
}
func (d *DynamicValue) Equals(other Value) bool {
	if d.Get != nil {
		return d.Get().Equals(other)
	}
	return false
}
func (d *DynamicValue) GetProperty(name string) Value {
	if d.Get != nil {
		innerVal := d.Get()
		if obj, ok := innerVal.(interface{ GetProperty(string) Value }); ok {
			return obj.GetProperty(name)
		}
	}
	return &NullValue{}
}
func (d *DynamicValue) SetProperty(name string, value Value) error {
	if d.Get != nil {
		innerVal := d.Get()
		if obj, ok := innerVal.(interface{ SetProperty(string, Value) error }); ok {
			return obj.SetProperty(name, value)
		}
	}
	return fmt.Errorf("cannot set property on dynamic value")
}

// GshObjectValue represents the gsh SDK object with custom property handling
type GshObjectValue struct {
	interp    *Interpreter
	baseProps map[string]*PropertyDescriptor
}

func (g *GshObjectValue) Type() ValueType { return ValueTypeObject }
func (g *GshObjectValue) String() string  { return "<gsh>" }
func (g *GshObjectValue) IsTruthy() bool  { return true }
func (g *GshObjectValue) Equals(other Value) bool {
	otherGsh, ok := other.(*GshObjectValue)
	return ok && g.interp == otherGsh.interp
}

func (g *GshObjectValue) GetProperty(name string) Value {
	if prop, ok := g.baseProps[name]; ok {
		// If the property is a DynamicValue, get its current value
		if dv, ok := prop.Value.(*DynamicValue); ok {
			return dv.GetDynamicValue()
		}
		return prop.Value
	}
	return &NullValue{}
}

func (g *GshObjectValue) SetProperty(name string, value Value) error {
	prop, ok := g.baseProps[name]
	if !ok {
		return fmt.Errorf("cannot set unknown property '%s' on gsh", name)
	}
	if prop.ReadOnly {
		return fmt.Errorf("cannot set read-only property '%s' on gsh", name)
	}

	// Handle special cases
	switch name {
	case "prompt":
		promptStr, ok := value.(*StringValue)
		if !ok {
			return fmt.Errorf("gsh.prompt must be a string, got %s", value.Type())
		}
		replCtx := g.interp.sdkConfig.GetREPLContext()
		if replCtx != nil {
			replCtx.PromptValue = promptStr
		}
		return nil
	case "continuationPrompt":
		cpStr, ok := value.(*StringValue)
		if !ok {
			return fmt.Errorf("gsh.continuationPrompt must be a string, got %s", value.Type())
		}
		replCtx := g.interp.sdkConfig.GetREPLContext()
		if replCtx != nil {
			replCtx.ContinuationPromptValue = cpStr
		}
		return nil
	default:
		// For other properties, delegate to the underlying value's SetProperty if it has one
		if dv, ok := prop.Value.(*DynamicValue); ok {
			actualVal := dv.GetDynamicValue()
			if setter, ok := actualVal.(interface{ SetProperty(string, Value) error }); ok {
				// This shouldn't happen for top-level gsh properties
				return setter.SetProperty(name, value)
			}
		}
		return fmt.Errorf("cannot set property '%s' on gsh", name)
	}
}

// ModelsObjectValue represents the gsh.models object.
// It holds pre-created SDKModelRef instances for each tier to avoid allocations
// on repeated access and to maintain pointer identity.
type ModelsObjectValue struct {
	liteRef      *SDKModelRef
	workhorseRef *SDKModelRef
	premiumRef   *SDKModelRef
}

// NewModelsObjectValue creates a new ModelsObjectValue with pre-allocated SDKModelRef instances.
func NewModelsObjectValue(models *Models) *ModelsObjectValue {
	return &ModelsObjectValue{
		liteRef:      &SDKModelRef{Tier: "lite", Models: models},
		workhorseRef: &SDKModelRef{Tier: "workhorse", Models: models},
		premiumRef:   &SDKModelRef{Tier: "premium", Models: models},
	}
}

func (m *ModelsObjectValue) Type() ValueType { return ValueTypeObject }
func (m *ModelsObjectValue) String() string  { return "<gsh.models>" }
func (m *ModelsObjectValue) IsTruthy() bool  { return true }
func (m *ModelsObjectValue) Equals(other Value) bool {
	_, ok := other.(*ModelsObjectValue)
	return ok
}

func (m *ModelsObjectValue) GetProperty(name string) Value {
	switch name {
	case "lite":
		// Check if the model is set via the SDKModelRef's Models reference
		if m.liteRef.Models == nil || m.liteRef.Models.Lite == nil {
			return &NullValue{}
		}
		// Return pre-allocated SDKModelRef for lazy resolution.
		// This allows dynamic model changes - if gsh.models.lite is reassigned later,
		// code holding this SDKModelRef will see the new model.
		return m.liteRef
	case "workhorse":
		if m.workhorseRef.Models == nil || m.workhorseRef.Models.Workhorse == nil {
			return &NullValue{}
		}
		return m.workhorseRef
	case "premium":
		if m.premiumRef.Models == nil || m.premiumRef.Models.Premium == nil {
			return &NullValue{}
		}
		return m.premiumRef
	default:
		return &NullValue{}
	}
}

func (m *ModelsObjectValue) SetProperty(name string, value Value) error {
	// Access the models through one of the refs (they all share the same Models pointer)
	models := m.liteRef.Models
	if models == nil {
		return fmt.Errorf("gsh.models is not initialized")
	}

	// Validate that the value is a ModelValue
	modelVal, ok := value.(*ModelValue)
	if !ok {
		return fmt.Errorf("gsh.models.%s must be a model, got %s", name, value.Type())
	}

	switch name {
	case "lite":
		models.Lite = modelVal
		return nil
	case "workhorse":
		models.Workhorse = modelVal
		return nil
	case "premium":
		models.Premium = modelVal
		return nil
	default:
		return fmt.Errorf("unknown property '%s' on gsh.models", name)
	}
}

// LastCommandObjectValue represents the gsh.lastCommand object
type LastCommandObjectValue struct {
	lastCommand *REPLLastCommand
}

func (c *LastCommandObjectValue) Type() ValueType { return ValueTypeObject }
func (c *LastCommandObjectValue) String() string  { return "<gsh.lastCommand>" }
func (c *LastCommandObjectValue) IsTruthy() bool  { return true }
func (c *LastCommandObjectValue) Equals(other Value) bool {
	_, ok := other.(*LastCommandObjectValue)
	return ok
}

func (c *LastCommandObjectValue) GetProperty(name string) Value {
	if c.lastCommand == nil {
		return &NullValue{}
	}
	switch name {
	case "command":
		return &StringValue{Value: c.lastCommand.Command}
	case "exitCode":
		return &NumberValue{Value: float64(c.lastCommand.ExitCode)}
	case "durationMs":
		return &NumberValue{Value: float64(c.lastCommand.DurationMs)}
	default:
		return &NullValue{}
	}
}

func (c *LastCommandObjectValue) SetProperty(name string, value Value) error {
	return fmt.Errorf("cannot set property '%s' on gsh.lastCommand", name)
}

// createNativeToolsObject creates the gsh.tools object with all native tool implementations.
// These tools use a single implementation shared between the SDK and the REPL agent.
// The tool definitions come from native_tools.go to avoid duplication.
func (i *Interpreter) createNativeToolsObject() *ObjectValue {
	return &ObjectValue{
		Properties: map[string]*PropertyDescriptor{
			"exec":      {Value: CreateExecNativeTool(), ReadOnly: true},
			"grep":      {Value: CreateGrepNativeTool(), ReadOnly: true},
			"view_file": {Value: CreateViewFileNativeTool(), ReadOnly: true},
			"edit_file": {Value: CreateEditFileNativeTool(), ReadOnly: true},
		},
	}
}

// createHistoryObject creates the gsh.history object for command history access.
func (i *Interpreter) createHistoryObject() *ObjectValue {
	return &ObjectValue{
		Properties: map[string]*PropertyDescriptor{
			"findPrefix": {Value: &BuiltinValue{
				Name: "gsh.history.findPrefix",
				Fn:   i.builtinHistoryFindPrefix,
			}, ReadOnly: true},
			"getRecent": {Value: &BuiltinValue{
				Name: "gsh.history.getRecent",
				Fn:   i.builtinHistoryGetRecent,
			}, ReadOnly: true},
		},
	}
}

// builtinHistoryFindPrefix implements gsh.history.findPrefix(prefix, limit)
// Returns an array of history entries that start with the given prefix, ordered by most recent first.
// Each entry is an object with { command, exitCode, timestamp }.
// Parameters:
//   - prefix (string): The prefix to search for
//   - limit (number, optional): Maximum number of history entries to return (default: 10)
func (i *Interpreter) builtinHistoryFindPrefix(args []Value) (Value, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, fmt.Errorf("gsh.history.findPrefix() takes 1-2 arguments (prefix: string, limit?: number), got %d", len(args))
	}

	// Get prefix argument
	prefix, ok := args[0].(*StringValue)
	if !ok {
		return nil, fmt.Errorf("gsh.history.findPrefix() first argument must be a string, got %s", args[0].Type())
	}

	// Get optional limit argument (default: 10)
	limit := 10
	if len(args) == 2 {
		limitVal, ok := args[1].(*NumberValue)
		if !ok {
			return nil, fmt.Errorf("gsh.history.findPrefix() second argument must be a number, got %s", args[1].Type())
		}
		limit = int(limitVal.Value)
		if limit <= 0 {
			limit = 10
		}
	}

	// Get history provider from SDK config
	provider := i.sdkConfig.GetHistoryProvider()
	if provider == nil {
		// No history provider available (e.g., in script mode)
		return &ArrayValue{Elements: []Value{}}, nil
	}

	// Search history for matching prefix
	entries, err := provider.FindPrefix(prefix.Value, limit)
	if err != nil {
		return &ArrayValue{Elements: []Value{}}, nil // Return empty array on error
	}

	// Convert to array of objects
	elements := make([]Value, len(entries))
	for i, entry := range entries {
		elements[i] = &ObjectValue{
			Properties: map[string]*PropertyDescriptor{
				"command":   {Value: &StringValue{Value: entry.Command}},
				"exitCode":  {Value: &NumberValue{Value: float64(entry.ExitCode)}},
				"timestamp": {Value: &NumberValue{Value: float64(entry.Timestamp)}},
			},
		}
	}

	return &ArrayValue{Elements: elements}, nil
}

// builtinHistoryGetRecent implements gsh.history.getRecent(limit)
// Returns an array of the most recent history entries in chronological order
// (oldest first, most recent last). This ordering is ideal for providing context
// to LLMs as it shows the natural flow of commands.
// Each entry is an object with { command, exitCode, timestamp }.
// Parameters:
//   - limit (number, optional): Maximum number of history entries to return (default: 10)
func (i *Interpreter) builtinHistoryGetRecent(args []Value) (Value, error) {
	if len(args) > 1 {
		return nil, fmt.Errorf("gsh.history.getRecent() takes 0-1 arguments (limit?: number), got %d", len(args))
	}

	// Get optional limit argument (default: 10)
	limit := 10
	if len(args) == 1 {
		limitVal, ok := args[0].(*NumberValue)
		if !ok {
			return nil, fmt.Errorf("gsh.history.getRecent() argument must be a number, got %s", args[0].Type())
		}
		limit = int(limitVal.Value)
		if limit <= 0 {
			limit = 10
		}
	}

	// Get history provider from SDK config
	provider := i.sdkConfig.GetHistoryProvider()
	if provider == nil {
		// No history provider available (e.g., in script mode)
		return &ArrayValue{Elements: []Value{}}, nil
	}

	// Get recent history entries
	entries, err := provider.GetRecent(limit)
	if err != nil {
		return &ArrayValue{Elements: []Value{}}, nil // Return empty array on error
	}

	// Convert to array of objects
	elements := make([]Value, len(entries))
	for i, entry := range entries {
		elements[i] = &ObjectValue{
			Properties: map[string]*PropertyDescriptor{
				"command":   {Value: &StringValue{Value: entry.Command}},
				"exitCode":  {Value: &NumberValue{Value: float64(entry.ExitCode)}},
				"timestamp": {Value: &NumberValue{Value: float64(entry.Timestamp)}},
			},
		}
	}

	return &ArrayValue{Elements: elements}, nil
}

package cli

import (
	"errors"
	"flag"
	"fmt"
	"reflect"
)

var errParse = errors.New("parse error")

// Choice Defines the definition of a choice.
type Choice interface {
	// FromString Returns a Choice value from string.
	FromString(s string) interface{}

	// ToString Returns the string representation of the given Choice value.
	ToString(i interface{}) string

	// Strings Returns all possible Choice values as string representation.
	Strings() []string
}

// NewStringerChoice Initializes a new instance of Choice that takes a list of fmt.Stringer instances used as choices.
func NewStringerChoice(ss ...fmt.Stringer) Choice {
	c := make(Choices, len(ss))
	for _, s := range ss {
		c[s.String()] = s
	}
	return NewChoice(c)
}

// NewStringChoice Initializes a new instance of Choice that takes a list of strings used as choices.
func NewStringChoice(ss ...string) Choice {
	c := make(Choices, len(ss))
	for _, s := range ss {
		c[s] = s
	}
	return NewChoice(c)
}

// Choices Maps a unique string value to any value.
type Choices map[string]interface{}

// NewChoice Initializes a new default implementation of Choice.
// The provided Choices need to have unique values.
func NewChoice(v Choices) Choice {
	out := new(defaultChoice)
	out.init(v)
	return out
}

type defaultChoice struct {
	vMap map[string]interface{}
	sMap map[interface{}]string
	ss   []string
}

func (c *defaultChoice) init(v Choices) {
	c.vMap = v

	c.sMap = make(map[interface{}]string, len(v))
	for k, v := range v {
		c.sMap[v] = k
	}

	c.ss = make([]string, len(v))
	i := 0
	for k := range v {
		c.ss[i] = k
		i++
	}
}

func (c *defaultChoice) FromString(s string) interface{} {
	if v, ok := c.vMap[s]; ok {
		return v
	}
	return nil
}

func (c *defaultChoice) ToString(v interface{}) string {
	if v, ok := c.sMap[v]; ok {
		return v
	}
	return ""
}

func (c *defaultChoice) Strings() []string {
	return c.ss
}

// ChoiceFlag A cli Flag that holds a Choice.
type ChoiceFlag struct {
	Name        string
	Aliases     []string
	Value       interface{}
	Choice      Choice
	EnvVars     []string
	FilePath    string
	Usage       string
	DefaultText string
	Required    bool
	Destination interface{}
	HasBeenSet  bool
}

// String Describes the Flag to the caller.
func (f *ChoiceFlag) String() string {
	return FlagStringer(f)
}

// Apply the value of the Flag to the cli.
func (f *ChoiceFlag) Apply(set *flag.FlagSet) error {
	if f.Choice == nil {
		return fmt.Errorf("choice must be provided for ChoiceFlag")
	}

	if v, ok := flagFromEnvOrFile(f.EnvVars, f.FilePath); ok {
		v := f.Choice.FromString(v)
		if v == nil {
			return errParse
		}
		f.Value = v
		f.HasBeenSet = true
	}

	for _, name := range f.Names() {
		if f.Destination != nil {
			v, err := newChoiceValueSwap(f.Choice, f.Value, f.Destination)
			if err != nil {
				return fmt.Errorf("failed to initialize new choice value swap: %w", err)
			}
			set.Var(v, name, f.Usage)
			continue
		}
		set.Var(newChoiceValue(f.Choice, f.Value), name, f.Usage)
	}

	return nil
}

// Names Returns all flag names of this cli.Flag.
func (f *ChoiceFlag) Names() []string {
	return append(f.Aliases, f.Name)
}

// IsSet Whether this cli.Flag has been set or not.
func (f *ChoiceFlag) IsSet() bool {
	return f.HasBeenSet
}

// IsRequired Whether this cli.Flag is required or not.
func (f *ChoiceFlag) IsRequired() bool {
	return f.Required
}

// TakesValue Whether this cli.Flag takes a value or not.
func (f *ChoiceFlag) TakesValue() bool {
	return true
}

// GetUsage Returns the usage description of this cli.Flag.
func (f *ChoiceFlag) GetUsage() string {
	return f.Usage
}

// GetValue Returns the current value of this cli.Flag.
func (f *ChoiceFlag) GetValue() string {
	return f.Choice.ToString(f.Value)
}

// Choice looks up the value of a local ChoiceFlag.
// Returns nil if not found.
func (c *Context) Choice(name string) interface{} {
	v := c.Value(name)
	if h, ok := v.(choiceValue); ok {
		return h.Value()
	}
	return nil
}

type choiceValue struct {
	value  reflect.Value
	choice Choice
}

func newChoiceValue(choice Choice, val interface{}) *choiceValue {
	return &choiceValue{choice: choice, value: reflect.ValueOf(val)}
}

func newChoiceValueSwap(choice Choice, val interface{}, p interface{}) (*choiceValue, error) {
	pV := reflect.ValueOf(p)

	if pV.Kind() != reflect.Ptr {
		return nil, fmt.Errorf("destination must be a ptr, not %s", pV.Type())
	}

	if pV.IsNil() {
		return nil, fmt.Errorf("destination must not be nil")
	}

	if val != nil {
		pV.Elem().Set(reflect.ValueOf(val))
	}

	return &choiceValue{
		value:  pV,
		choice: choice,
	}, nil
}

func (c *choiceValue) Set(s string) error {
	if !c.value.IsValid() || isNil(c.value) {
		return nil
	}

	if v := c.choice.FromString(s); v != nil {
		setValue(c.value, v)
		return nil
	}

	return errParse
}

func (c choiceValue) Get() interface{} { return c }

func (c *choiceValue) String() string {
	if !c.value.IsValid() || isNil(c.value) {
		return ""
	}
	return c.choice.ToString(interfaceOf(c.value))
}

func (c *choiceValue) Value() interface{} {
	if !c.value.IsValid() || isNil(c.value) {
		return nil
	}
	return interfaceOf(c.value)
}

func isNil(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Slice, reflect.Ptr:
		return v.IsNil()
	default:
		return false
	}
}

func interfaceOf(v reflect.Value) interface{} {
	switch v.Kind() {
	case reflect.Interface, reflect.Ptr:
		return v.Elem().Interface()
	default:
		return v.Interface()
	}
}

func setValue(v reflect.Value, val interface{}) {
	switch v.Kind() {
	case reflect.Interface, reflect.Ptr:
		v.Elem().Set(reflect.ValueOf(val))
	default:
		v.Set(reflect.ValueOf(val))
	}
}

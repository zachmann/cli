package cli

import (
	"flag"
	"fmt"
	"strconv"
)

// Uint64Flag is a flag with type uint64
type Uint64Flag struct {
	Name        string
	Aliases     []string
	Usage       string
	EnvVars     []string
	FilePath    string
	Required    bool
	Hidden      bool
	Value       uint64
	DefaultText string
	Destination *uint64
	HasBeenSet  bool
	Placeholder string
}

// IsSet returns whether or not the flag has been set through env or file
func (f *Uint64Flag) IsSet() bool {
	return f.HasBeenSet
}

// String returns a readable representation of this value
// (for usage defaults)
func (f *Uint64Flag) String() string {
	return FlagStringer(f)
}

// Names returns the names of the flag
func (f *Uint64Flag) Names() []string {
	return flagNames(f.Name, f.Aliases)
}

// IsRequired returns whether or not the flag is required
func (f *Uint64Flag) IsRequired() bool {
	return f.Required
}

// TakesValue returns true of the flag takes a value, otherwise false
func (f *Uint64Flag) TakesValue() bool {
	return true
}

// GetUsage returns the usage string for the flag
func (f *Uint64Flag) GetUsage() string {
	return f.Usage
}

// IsVisible returns true if the flag is not hidden, otherwise false
func (f *Uint64Flag) IsVisible() bool {
	return !f.Hidden
}

// Apply populates the flag given the flag set and environment
func (f *Uint64Flag) Apply(set *flag.FlagSet) error {
	if val, ok := flagFromEnvOrFile(f.EnvVars, f.FilePath); ok {
		if val != "" {
			valInt, err := strconv.ParseUint(val, 0, 64)
			if err != nil {
				return fmt.Errorf("could not parse %q as uint64 value for flag %s: %s", val, f.Name, err)
			}

			f.Value = valInt
			f.HasBeenSet = true
		}
	}

	for _, name := range f.Names() {
		if f.Destination != nil {
			set.Uint64Var(f.Destination, name, f.Value, f.Usage)
			continue
		}
		set.Uint64(name, f.Value, f.Usage)
	}

	return nil
}

// GetValue returns the flags value as string representation and an empty
// string if the flag takes no value at all.
func (f *Uint64Flag) GetValue() string {
	return fmt.Sprintf("%d", f.Value)
}

// Uint64 looks up the value of a local Uint64Flag, returns
// 0 if not found
func (c *Context) Uint64(name string) uint64 {
	for _, ctx := range c.Lineage() {
		if fs := ctx.lookupFlagSet(name); fs != nil {
			if f := flagSetLookupWithValueSet(fs, name); f != nil {
				return lookupUint64(f)
			}
		}
	}
	return 0
}

func lookupUint64(f *flag.Flag) uint64 {
	parsed, err := strconv.ParseUint(f.Value.String(), 0, 64)
	if err != nil {
		return 0
	}
	return parsed
}

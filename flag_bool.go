package cli

import (
	"flag"
	"fmt"
	"strconv"
)

// BoolFlag is a flag with type bool
type BoolFlag struct {
	Name             string
	Aliases          []string
	Usage            string
	EnvVars          []string
	FilePath         string
	Required         bool
	Hidden           bool
	Value            bool
	DefaultText      string
	Destination      *bool
	HasBeenSet       bool
	HideDefaultValue bool
	Placeholder      string
}

// IsSet returns whether or not the flag has been set through env or file
func (f *BoolFlag) IsSet() bool {
	return f.HasBeenSet
}

// String returns a readable representation of this value
// (for usage defaults)
func (f *BoolFlag) String() string {
	return FlagStringer(f)
}

// Names returns the names of the flag
func (f *BoolFlag) Names() []string {
	return flagNames(f.Name, f.Aliases)
}

// IsRequired returns whether or not the flag is required
func (f *BoolFlag) IsRequired() bool {
	return f.Required
}

// TakesValue returns true of the flag takes a value, otherwise false
func (f *BoolFlag) TakesValue() bool {
	return false
}

// GetUsage returns the usage string for the flag
func (f *BoolFlag) GetUsage() string {
	return f.Usage
}

// GetValue returns the flags value as string representation and an empty
// string if the flag takes no value at all.
func (f *BoolFlag) GetValue() string {
	return ""
}

// IsVisible returns true if the flag is not hidden, otherwise false
func (f *BoolFlag) IsVisible() bool {
	return !f.Hidden
}

// Apply populates the flag given the flag set and environment
func (f *BoolFlag) Apply(set *flag.FlagSet) error {
	if val, ok := flagFromEnvOrFile(f.EnvVars, f.FilePath); ok {
		if val != "" {
			valBool, err := strconv.ParseBool(val)

			if err != nil {
				return fmt.Errorf("could not parse %q as bool value for flag %s: %s", val, f.Name, err)
			}

			f.Value = valBool
			f.HasBeenSet = true
		}
	}

	for _, name := range f.Names() {
		if f.Destination != nil {
			set.BoolVar(f.Destination, name, f.Value, f.Usage)
			continue
		}
		set.Bool(name, f.Value, f.Usage)
	}

	return nil
}

// Bool looks up the value of a local BoolFlag, returns
// false if not found
func (c *Context) Bool(name string) bool {
	for _, ctx := range c.Lineage() {
		if fs := ctx.lookupFlagSet(name); fs != nil {
			if f := flagSetLookupWithValueSet(fs, name); f != nil {
				return lookupBool(f)
			}
		}
	}
	return false
}

func lookupBool(f *flag.Flag) bool {
	parsed, err := strconv.ParseBool(f.Value.String())
	if err != nil {
		return false
	}
	return parsed
}

package cli

import (
	"context"
	"flag"
	"strings"
)

// Context is a type that is passed through to
// each Handler action in a cli application. Context
// can be used to retrieve context-specific args and
// parsed command-line options.
type Context struct {
	context.Context
	App           *App
	Command       *Command
	shellComplete bool
	flagSet       *flag.FlagSet
	parentContext *Context
}

// NewContext creates a new context. For use in when invoking an App or Command action.
func NewContext(app *App, set *flag.FlagSet, parentCtx *Context) *Context {
	c := &Context{App: app, flagSet: set, parentContext: parentCtx}
	if parentCtx != nil {
		c.Context = parentCtx.Context
		c.shellComplete = parentCtx.shellComplete
		if parentCtx.flagSet == nil {
			parentCtx.flagSet = &flag.FlagSet{}
		}
	}

	c.Command = &Command{}

	if c.Context == nil {
		c.Context = context.Background()
	}

	return c
}

// NumFlags returns the number of flags set
func (c *Context) NumFlags() int {
	return c.flagSet.NFlag()
}

// Set sets a context flag to a value.
func (c *Context) Set(name, value string) error {
	return c.flagSet.Set(name, value)
}

// IsSet determines if the flag was actually set
func (c *Context) IsSet(name string) bool {
	for _, ctx := range c.Lineage() {
		if ctx.Command == nil {
			continue
		}

		for _, f := range ctx.Command.Flags {
			if !f.IsSet() {
				continue
			}
			for _, n := range f.Names() {
				if n == name {
					return true
				}
			}
		}
		if flagSetLookupWithValueSet(ctx.flagSet, name) != nil {
			return true
		}
	}

	if c.App != nil {
		for _, f := range c.App.Flags {
			if !f.IsSet() {
				continue
			}
			for _, n := range f.Names() {
				if n == name {
					return true
				}
			}
		}
		if flagSetLookupWithValueSet(c.flagSet, name) != nil {
			return true
		}
	}

	return false
}

// LocalFlagNames returns a slice of flag names used in this context.
func (c *Context) LocalFlagNames() []string {
	var names []string
	c.flagSet.Visit(makeFlagNameVisitor(&names))
	return names
}

// FlagNames returns a slice of flag names used by the this context and all of
// its parent contexts.
func (c *Context) FlagNames() []string {
	var names []string
	for _, ctx := range c.Lineage() {
		ctx.flagSet.Visit(makeFlagNameVisitor(&names))
	}
	return names
}

// Lineage returns *this* context and all of its ancestor contexts in order from
// child to parent
func (c *Context) Lineage() []*Context {
	var lineage []*Context

	for cur := c; cur != nil; cur = cur.parentContext {
		lineage = append(lineage, cur)
	}

	return lineage
}

// Value returns the value of the flag corresponding to `name`
func (c *Context) Value(name string) interface{} {
	for _, ctx := range c.Lineage() {
		if fs := ctx.lookupFlagSet(name); fs != nil {
			if f := flagSetLookupWithValueSet(fs, name); f != nil {
				return f.Value.(flag.Getter).Get()
			}
		}
	}
	return nil
}

// Args returns the command line arguments associated with the context.
func (c *Context) Args() Args {
	ret := args(c.flagSet.Args())
	return &ret
}

// NArg returns the number of the command line arguments.
func (c *Context) NArg() int {
	return c.Args().Len()
}

func (ctx *Context) lookupFlagSet(name string) *flag.FlagSet {
	for _, c := range ctx.Lineage() {
		if f := c.flagSet.Lookup(name); f != nil {
			return c.flagSet
		}
	}

	return nil
}

func flagSetLookupWithValueSet(fs *flag.FlagSet, name string) (f *flag.Flag) {
	fs.Visit(
		func(ff *flag.Flag) {
			if ff.Name == name {
				f = ff
			}
		})
	return
}

func (context *Context) checkRequiredFlags(flags []Flag) requiredFlagsErr {
	var missingFlags []string
	for _, f := range flags {
		if rf, ok := f.(RequiredFlag); ok && rf.IsRequired() {
			var flagPresent bool
			var flagName string

			for _, key := range f.Names() {
				if len(key) > 1 {
					flagName = key
				}

				if context.IsSet(strings.TrimSpace(key)) {
					flagPresent = true
				}
			}

			if !flagPresent && flagName != "" {
				missingFlags = append(missingFlags, flagName)
			}
		}
	}

	if len(missingFlags) != 0 {
		return &errRequiredFlags{missingFlags: missingFlags}
	}

	return nil
}

func makeFlagNameVisitor(names *[]string) func(*flag.Flag) {
	return func(f *flag.Flag) {
		nameParts := strings.Split(f.Name, ",")
		name := strings.TrimSpace(nameParts[0])

		for _, part := range nameParts {
			part = strings.TrimSpace(part)
			if len(part) > len(name) {
				name = part
			}
		}

		if name != "" {
			*names = append(*names, name)
		}
	}
}

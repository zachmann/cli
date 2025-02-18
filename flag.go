package cli

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const defaultPlaceholder = "VALUE"

var (
	slPfx = fmt.Sprintf("sl:::%d:::", time.Now().UTC().UnixNano())

	commaWhitespace = regexp.MustCompile("[, ]+.*")
)

// BashCompletionFlag enables bash-completion for all commands and subcommands
var BashCompletionFlag Flag = &BoolFlag{
	Name:   "generate-bash-completion",
	Hidden: true,
}

// VersionFlag prints the version for the application
var VersionFlag Flag = &BoolFlag{
	Name:    "version",
	Aliases: []string{"V"},
	Usage:   "Print the version and exit",
	HideDefaultValue: true,
}

// HelpFlag prints the help for all commands and subcommands.
// Set to nil to disable the flag.  The subcommand
// will still be added unless HideHelp or HideHelpCommand is set to true.
var HelpFlag Flag = &BoolFlag{
	Name:    "help",
	Aliases: []string{"h"},
	Usage:   "Show help",
	HideDefaultValue: true,
}

// FlagStringer converts a flag definition to a string. This is used by help
// to display a flag.
var FlagStringer FlagStringFunc = stringifyFlag

var FlagsStringer = func(flags []Flag, indent int) []string {
	strs:=make([][2]string, len(flags))
	maxTabPos := 0
	indentStr := strings.Repeat(" ", indent)
	for i, f := range flags {
		str := indentStr + FlagStringer(f)
		tabPos := strings.Index(str, "\t")
		if tabPos > maxTabPos {
			maxTabPos=tabPos
		}
		strs[i]=[2]string{str[:tabPos],str[tabPos+1:]}
	}
	final:=make([]string, len(flags))
	for i, s := range strs {
		offs := maxTabPos+2
		str := s[0]+strings.Repeat(" ", offs-len(s[0]))
			str+=wrap(s[1], offs+2, HelpWrapAt)
		final[i] = str
	}
	return final
}

// Serializer is used to circumvent the limitations of flag.FlagSet.Set
type Serializer interface {
	Serialize() string
}

// FlagNamePrefixer converts a full flag name and its placeholder into the help
// message flag prefix. This is used by the default FlagStringer.
var FlagNamePrefixer FlagNamePrefixFunc = prefixedNames

// FlagEnvHinter annotates flag help message with the environment variable
// details. This is used by the default FlagStringer.
var FlagEnvHinter FlagEnvHintFunc = withEnvHint

// FlagFileHinter annotates flag help message with the environment variable
// details. This is used by the default FlagStringer.
var FlagFileHinter FlagFileHintFunc = withFileHint

// FlagsByName is a slice of Flag.
type FlagsByName []Flag

func (f FlagsByName) Len() int {
	return len(f)
}

func (f FlagsByName) Less(i, j int) bool {
	if len(f[j].Names()) == 0 {
		return false
	} else if len(f[i].Names()) == 0 {
		return true
	}
	return lexicographicLess(f[i].Names()[0], f[j].Names()[0])
}

func (f FlagsByName) Swap(i, j int) {
	f[i], f[j] = f[j], f[i]
}

// Flag is a common interface related to parsing flags in cli.
// For more advanced flag parsing techniques, it is recommended that
// this interface be implemented.
type Flag interface {
	fmt.Stringer
	// Apply Flag settings to the given flag set
	Apply(*flag.FlagSet) error
	Names() []string
	IsSet() bool
}

// RequiredFlag is an interface that allows us to mark flags as required
// it allows flags required flags to be backwards compatible with the Flag interface
type RequiredFlag interface {
	Flag

	IsRequired() bool
}

// DocGenerationFlag is an interface that allows documentation generation for the flag
type DocGenerationFlag interface {
	Flag

	// TakesValue returns true if the flag takes a value, otherwise false
	TakesValue() bool

	// GetUsage returns the usage string for the flag
	GetUsage() string

	// GetValue returns the flags value as string representation and an empty
	// string if the flag takes no value at all.
	GetValue() string
}

// VisibleFlag is an interface that allows to check if a flag is visible
type VisibleFlag interface {
	Flag

	// IsVisible returns true if the flag is not hidden, otherwise false
	IsVisible() bool
}

func flagSet(name string, flags []Flag) (*flag.FlagSet, error) {
	set := flag.NewFlagSet(name, flag.ContinueOnError)

	for _, f := range flags {
		if err := f.Apply(set); err != nil {
			return nil, err
		}
	}
	set.SetOutput(ioutil.Discard)
	return set, nil
}

func copyFlag(name string, ff *flag.Flag, set *flag.FlagSet) {
	switch ff.Value.(type) {
	case Serializer:
		_ = set.Set(name, ff.Value.(Serializer).Serialize())
	default:
		_ = set.Set(name, ff.Value.String())
	}
}

func normalizeFlags(flags []Flag, set *flag.FlagSet) error {
	visited := make(map[string]bool)
	set.Visit(func(f *flag.Flag) {
		visited[f.Name] = true
	})
	for _, f := range flags {
		parts := f.Names()
		if len(parts) == 1 {
			continue
		}
		var ff *flag.Flag
		for _, name := range parts {
			name = strings.Trim(name, " ")
			if visited[name] {
				if ff != nil {
					return errors.New("Cannot use two forms of the same flag: " + name + " " + ff.Name)
				}
				ff = set.Lookup(name)
			}
		}
		if ff == nil {
			continue
		}
		for _, name := range parts {
			name = strings.Trim(name, " ")
			if !visited[name] {
				copyFlag(name, ff, set)
			}
		}
	}
	return nil
}

func visibleFlags(fl []Flag) []Flag {
	var visible []Flag
	for _, f := range fl {
		if vf, ok := f.(VisibleFlag); ok && vf.IsVisible() {
			visible = append(visible, f)
		}
	}
	return visible
}

func prefixFor(name string) (prefix string) {
	if len(name) == 1 {
		prefix = "-"
	} else {
		prefix = "--"
	}

	return
}

// Returns the placeholder, if any, and the unquoted usage string.
func unquoteUsage(usage string) (string, string) {
	for i := 0; i < len(usage); i++ {
		if usage[i] == '`' {
			for j := i + 1; j < len(usage); j++ {
				if usage[j] == '`' {
					name := usage[i+1 : j]
					usage = usage[:i] + name + usage[j+1:]
					return name, usage
				}
			}
			break
		}
	}
	return "", usage
}

func prefixedNames(names []string, placeholder string) string {
	var prefixed string
	for i, name := range names {
		if name == "" {
			continue
		}

		prefixed += prefixFor(name) + name
		if placeholder != "" {
			prefixed += " " + placeholder
		}
		if i < len(names)-1 {
			prefixed += ", "
		}
	}
	return prefixed
}

func withEnvHint(envVars []string, str string) string {
	envText := ""
	if envVars != nil && len(envVars) > 0 {
		prefix := "$"
		suffix := ""
		sep := ", $"
		if runtime.GOOS == "windows" {
			prefix = "%"
			suffix = "%"
			sep = "%, %"
		}

		envText = fmt.Sprintf(" [%s%s%s]", prefix, strings.Join(envVars, sep), suffix)
	}
	return str + envText
}

func flagNames(name string, aliases []string) []string {
	var ret []string

	for _, part := range append([]string{name}, aliases...) {
		// v1 -> v2 migration warning zone:
		// Strip off anything after the first found comma or space, which
		// *hopefully* makes it a tiny bit more obvious that unexpected behavior is
		// caused by using the v1 form of stringly typed "Name".
		ret = append(ret, commaWhitespace.ReplaceAllString(part, ""))
	}

	return ret
}

func flagStringSliceField(f Flag, name string) []string {
	fv := flagValue(f)
	field := fv.FieldByName(name)

	if field.IsValid() {
		return field.Interface().([]string)
	}

	return []string{}
}

func withFileHint(filePath, str string) string {
	fileText := ""
	if filePath != "" {
		fileText = fmt.Sprintf(" [%s]", filePath)
	}
	return str + fileText
}

func flagValue(f Flag) reflect.Value {
	fv := reflect.ValueOf(f)
	for fv.Kind() == reflect.Ptr {
		fv = reflect.Indirect(fv)
	}
	return fv
}

func formatDefault(format string) string {
	return " (default: " + format + ")"
}

func stringifyFlag(f Flag) string {
	fv := flagValue(f)

	switch f := f.(type) {
	case *IntSliceFlag:
		return withEnvHint(flagStringSliceField(f, "EnvVars"),
			stringifyIntSliceFlag(f))
	case *Int64SliceFlag:
		return withEnvHint(flagStringSliceField(f, "EnvVars"),
			stringifyInt64SliceFlag(f))
	case *Float64SliceFlag:
		return withEnvHint(flagStringSliceField(f, "EnvVars"),
			stringifyFloat64SliceFlag(f))
	case *StringSliceFlag:
		return withEnvHint(flagStringSliceField(f, "EnvVars"),
			stringifyStringSliceFlag(f))
	case *ChoiceFlag:
		return withEnvHint(flagStringSliceField(f, "EnvVars"),
			stringifyChoiceFlag(f))
	}

	placeholder, usage := unquoteUsage(fv.FieldByName("Usage").String())

	needsPlaceholder := false
	defaultValueString := ""
	val := fv.FieldByName("Value")
	hideDefaultValue := false

	if boolFlag, ok := f.(*BoolFlag); ok {
		hideDefaultValue = boolFlag.HideDefaultValue
	}

	if val.IsValid() {
		needsPlaceholder = val.Kind() != reflect.Bool
		defaultValueString = fmt.Sprintf(formatDefault("%v"), val.Interface())
		if hideDefaultValue {
			defaultValueString = ""
		}

		if val.Kind() == reflect.String && val.String() != "" {
			defaultValueString = fmt.Sprintf(formatDefault("%q"), val.String())
		}
	}

	helpText := fv.FieldByName("DefaultText")
	if helpText.IsValid() && helpText.String() != "" {
		needsPlaceholder = val.Kind() != reflect.Bool
		defaultValueString = fmt.Sprintf(formatDefault("%s"), helpText.String())
		if hideDefaultValue {
			defaultValueString = ""
		}
	}

	if defaultValueString == formatDefault("") {
		defaultValueString = ""
	}

	if needsPlaceholder && placeholder == "" {
		if pl := fv.FieldByName("Placeholder"); pl.IsValid() {
			placeholder = pl.String()
		}
	}

	if needsPlaceholder && placeholder == "" {
		placeholder = defaultPlaceholder
	}

	usageWithDefault := strings.TrimSpace(usage + defaultValueString)

	return withEnvHint(flagStringSliceField(f, "EnvVars"),
		fmt.Sprintf("%s\t%s", prefixedNames(f.Names(), placeholder), usageWithDefault))
}

func stringifyIntSliceFlag(f *IntSliceFlag) string {
	var defaultVals []string
	if f.Value != nil && len(f.Value.Value()) > 0 {
		for _, i := range f.Value.Value() {
			defaultVals = append(defaultVals, strconv.Itoa(i))
		}
	}

	return stringifySliceFlag(f.Usage, f.Names(), defaultVals, f.Placeholder)
}

func stringifyInt64SliceFlag(f *Int64SliceFlag) string {
	var defaultVals []string
	if f.Value != nil && len(f.Value.Value()) > 0 {
		for _, i := range f.Value.Value() {
			defaultVals = append(defaultVals, strconv.FormatInt(i, 10))
		}
	}

	return stringifySliceFlag(f.Usage, f.Names(), defaultVals, f.Placeholder)
}

func stringifyFloat64SliceFlag(f *Float64SliceFlag) string {
	var defaultVals []string

	if f.Value != nil && len(f.Value.Value()) > 0 {
		for _, i := range f.Value.Value() {
			defaultVals = append(defaultVals, strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", i), "0"), "."))
		}
	}

	return stringifySliceFlag(f.Usage, f.Names(), defaultVals, f.Placeholder)
}

func stringifyStringSliceFlag(f *StringSliceFlag) string {
	var defaultVals []string
	if f.Value != nil && len(f.Value.Value()) > 0 {
		for _, s := range f.Value.Value() {
			if len(s) > 0 {
				defaultVals = append(defaultVals, strconv.Quote(s))
			}
		}
	}

	return stringifySliceFlag(f.Usage, f.Names(), defaultVals, f.Placeholder)
}

func stringifySliceFlag(usage string, names, defaultVals []string, plchldr string) string {
	placeholder, usage := unquoteUsage(usage)

	if  placeholder == "" {
		placeholder = plchldr
	}
	if placeholder == "" {
		placeholder = defaultPlaceholder
	}

	defaultVal := ""
	if len(defaultVals) > 0 {
		defaultVal = fmt.Sprintf(formatDefault("%s"), strings.Join(defaultVals, ", "))
	}

	usageWithDefault := strings.TrimSpace(fmt.Sprintf("%s%s", usage, defaultVal))
	multiInputString := "(accepts multiple inputs)"
	if usageWithDefault != "" {
		multiInputString = "\t" + multiInputString
	}
	return fmt.Sprintf("%s\t%s%s", prefixedNames(names, placeholder), usageWithDefault, multiInputString)
}

func stringifyChoiceFlag(f *ChoiceFlag) string {
	placeholder, usage := unquoteUsage(f.Usage)

	defaultValueString := ""
	if v := f.Value; v != nil {
		defaultValueString = fmt.Sprintf(formatDefault("%q"), f.Choice.ToString(v))
	}

	if  placeholder == "" {
		placeholder = f.Placeholder
	}
	if placeholder == "" {
		placeholder = defaultPlaceholder
	}

	supportedValues := fmt.Sprintf(" (supported values: %s)", strings.Join(quoteStrings(f.Choice.Strings()), ", "))
	usageWithDefault := strings.TrimSpace(usage + defaultValueString)
	return fmt.Sprintf("%s\t%s", prefixedNames(f.Names(), placeholder), usageWithDefault+supportedValues)
}

func quoteStrings(ss []string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = "\"" + s + "\""
	}
	return out
}

func hasFlag(flags []Flag, fl Flag) bool {
	for _, existing := range flags {
		if fl == existing {
			return true
		}
	}

	return false
}

func flagFromEnvOrFile(envVars []string, filePath string) (val string, ok bool) {
	for _, envVar := range envVars {
		envVar = strings.TrimSpace(envVar)
		if val, ok := syscall.Getenv(envVar); ok {
			return val, true
		}
	}
	for _, fileVar := range strings.Split(filePath, ",") {
		if data, err := ioutil.ReadFile(fileVar); err == nil {
			return string(data), true
		}
	}
	return "", false
}

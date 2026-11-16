package main

import (
	"flag"
	"fmt"
	"strings"

	"jira-thing/internal/ops"
)

// flagTerminator is the argument that ends flag parsing, per the flag package.
const flagTerminator = "--"

// reorderFlagsBeforeKey moves flags (and their values) ahead of positional
// arguments so flag.Parse doesn't stop at the first non-flag argument when
// the ticket key is given before its flags (e.g. "attach PROJ-1 -f a.txt").
// It must be called after fs's flags are defined: fs is what tells a flag
// that takes a value from a boolean one, whose following argument is a
// positional. Arguments after an explicit "--" are always positional.
func reorderFlagsBeforeKey(fs *flag.FlagSet, args []string) []string {
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == flagTerminator:
			positional = append(positional, args[i+1:]...)
			i = len(args)
		case !isFlagArg(arg):
			positional = append(positional, arg)
		case flagTakesNextArg(fs, arg) && i+1 < len(args):
			flags = append(flags, arg, args[i+1])
			i++
		default:
			flags = append(flags, arg)
		}
	}
	return append(append(flags, flagTerminator), positional...)
}

// isFlagArg reports whether arg looks like a flag; a lone "-" is a positional.
func isFlagArg(arg string) bool {
	return len(arg) > 1 && strings.HasPrefix(arg, "-")
}

// flagTakesNextArg reports whether arg is a flag defined on fs whose value is
// the following argument: not a boolean, and not already given as -name=value.
// Undefined flags report false and are left for fs.Parse to reject.
func flagTakesNextArg(fs *flag.FlagSet, arg string) bool {
	name := strings.TrimLeft(arg, "-")
	if strings.Contains(name, "=") {
		return false
	}
	f := fs.Lookup(name)
	if f == nil {
		return false
	}
	boolFlag, ok := f.Value.(interface{ IsBoolFlag() bool })
	return !ok || !boolFlag.IsBoolFlag()
}

// customFlag is the repeatable -custom "Name=value" flag; each value is
// parsed as it is read, so a malformed one fails at flag parsing.
type customFlag []ops.CustomValue

func (c *customFlag) String() string { return fmt.Sprint(len(*c)) }

func (c *customFlag) Set(s string) error {
	v, err := ops.ParseCustom(s)
	if err != nil {
		return err
	}
	*c = append(*c, v)
	return nil
}

// customHelp is the -custom flag's usage text.
const customHelp = `Set any field: "Name=value" or "customfield_NNNNN=value" (repeatable; an empty value clears it). Lists are comma-separated`

// listEditFlag is a repeatable add/remove flag such as -label: "name" or
// "+name" adds, "-name" removes.
type listEditFlag []ops.ListEdit

func (l *listEditFlag) String() string { return fmt.Sprint(len(*l)) }

func (l *listEditFlag) Set(s string) error {
	e, err := ops.ParseListEdit(s)
	if err != nil {
		return err
	}
	*l = append(*l, e)
	return nil
}

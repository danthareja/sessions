package commands

import (
	"fmt"
	"strings"
)

type parsedArgs struct {
	strings map[string]string
	bools   map[string]bool
	pos     []string
}

func parseArgs(args []string, stringFlags, boolFlags map[string]bool) (parsedArgs, error) {
	out := parsedArgs{
		strings: map[string]string{},
		bools:   map[string]bool{},
	}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			out.pos = append(out.pos, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "--") || arg == "--" {
			out.pos = append(out.pos, arg)
			continue
		}

		nameValue := strings.TrimPrefix(arg, "--")
		name, value, hasValue := strings.Cut(nameValue, "=")
		if boolFlags[name] {
			if hasValue {
				switch value {
				case "1", "true", "yes", "on":
					out.bools[name] = true
				case "0", "false", "no", "off":
					out.bools[name] = false
				default:
					return out, fmt.Errorf("--%s expects a boolean value", name)
				}
			} else {
				out.bools[name] = true
			}
			continue
		}
		if stringFlags[name] {
			if !hasValue {
				i++
				if i >= len(args) {
					return out, fmt.Errorf("--%s requires a value", name)
				}
				value = args[i]
			}
			out.strings[name] = value
			continue
		}
		return out, fmt.Errorf("unknown flag --%s", name)
	}
	return out, nil
}

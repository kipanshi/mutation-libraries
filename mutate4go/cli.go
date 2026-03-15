package mutate4go

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
)

func ParseArgs(args []string) (CliArguments, error) {
	parsed := CliArguments{
		Mode:            ModeExplicitFiles,
		Lines:           map[int]struct{}{},
		TimeoutFactor:   10,
		MutationWarning: 50,
		MaxWorkers:      maxInt(1, runtime.NumCPU()/2),
	}

	if len(args) == 1 && args[0] == "--help" {
		parsed.Mode = ModeHelp
		return parsed, nil
	}

	var fileArgs []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "--") {
			fileArgs = append(fileArgs, arg)
			continue
		}
		switch arg {
		case "--scan":
			parsed.Scan = true
		case "--update-manifest":
			parsed.UpdateManifest = true
		case "--reuse-coverage":
			parsed.ReuseCoverage = true
		case "--since-last-run":
			parsed.SinceLastRun = true
		case "--mutate-all":
			parsed.MutateAll = true
		case "--verbose":
			parsed.Verbose = true
		case "--lines":
			if i+1 >= len(args) {
				return CliArguments{}, fmt.Errorf("--lines requires a value")
			}
			value := args[i+1]
			i++
			lines, err := parseLines(value)
			if err != nil {
				return CliArguments{}, err
			}
			parsed.Lines = lines
		case "--timeout-factor":
			value, next, err := parsePositiveInt(args, i, "--timeout-factor")
			if err != nil {
				return CliArguments{}, err
			}
			parsed.TimeoutFactor = value
			i = next
		case "--mutation-warning":
			value, next, err := parsePositiveInt(args, i, "--mutation-warning")
			if err != nil {
				return CliArguments{}, err
			}
			parsed.MutationWarning = value
			i = next
		case "--max-workers":
			value, next, err := parsePositiveInt(args, i, "--max-workers")
			if err != nil {
				return CliArguments{}, err
			}
			parsed.MaxWorkers = value
			i = next
		case "--test-command":
			if i+1 >= len(args) {
				return CliArguments{}, fmt.Errorf("--test-command requires a value")
			}
			value := strings.TrimSpace(args[i+1])
			i++
			if value == "" {
				return CliArguments{}, fmt.Errorf("--test-command must not be blank")
			}
			parsed.TestCommand = value
		default:
			return CliArguments{}, fmt.Errorf("Unknown option: %s", arg)
		}
	}

	parsed.FileArgs = fileArgs
	if len(fileArgs) == 0 {
		return CliArguments{}, fmt.Errorf("mutate4go requires exactly one Go file")
	}
	if len(fileArgs) != 1 {
		return CliArguments{}, fmt.Errorf("mutate4go accepts exactly one Go file")
	}
	if !strings.HasSuffix(fileArgs[0], ".go") {
		return CliArguments{}, fmt.Errorf("mutate4go target must be a .go file")
	}

	if len(parsed.Lines) > 0 && parsed.SinceLastRun {
		return CliArguments{}, fmt.Errorf("--lines may not be combined with --since-last-run")
	}
	if len(parsed.Lines) > 0 && parsed.MutateAll {
		return CliArguments{}, fmt.Errorf("--lines may not be combined with --mutate-all")
	}
	if parsed.SinceLastRun && parsed.MutateAll {
		return CliArguments{}, fmt.Errorf("--since-last-run may not be combined with --mutate-all")
	}
	if parsed.Scan && parsed.SinceLastRun {
		return CliArguments{}, fmt.Errorf("--scan may not be combined with --since-last-run")
	}
	if parsed.Scan && parsed.UpdateManifest {
		return CliArguments{}, fmt.Errorf("--scan may not be combined with --update-manifest")
	}
	if parsed.Scan && parsed.ReuseCoverage {
		return CliArguments{}, fmt.Errorf("--scan may not be combined with --reuse-coverage")
	}
	if parsed.Scan && parsed.MutateAll {
		return CliArguments{}, fmt.Errorf("--scan may not be combined with --mutate-all")
	}
	if parsed.UpdateManifest && parsed.SinceLastRun {
		return CliArguments{}, fmt.Errorf("--update-manifest may not be combined with --since-last-run")
	}
	if parsed.UpdateManifest && parsed.MutateAll {
		return CliArguments{}, fmt.Errorf("--update-manifest may not be combined with --mutate-all")
	}
	if parsed.UpdateManifest && parsed.ReuseCoverage {
		return CliArguments{}, fmt.Errorf("--update-manifest may not be combined with --reuse-coverage")
	}
	if parsed.UpdateManifest && len(parsed.Lines) > 0 {
		return CliArguments{}, fmt.Errorf("--update-manifest may not be combined with --lines")
	}

	return parsed, nil
}

func parsePositiveInt(args []string, index int, name string) (int, int, error) {
	if index+1 >= len(args) {
		return 0, index, fmt.Errorf("%s requires a value", name)
	}
	value, err := strconv.Atoi(args[index+1])
	if err != nil || value <= 0 {
		return 0, index, fmt.Errorf("%s must be a positive integer", name)
	}
	return value, index + 1, nil
}

func parseLines(value string) (map[int]struct{}, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || strings.Trim(trimmed, ",") == "" {
		return nil, fmt.Errorf("--lines requires at least one line number")
	}
	lines := map[int]struct{}{}
	for _, part := range strings.Split(trimmed, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		line, err := strconv.Atoi(part)
		if err != nil || line <= 0 {
			return nil, fmt.Errorf("--lines must be a positive integer")
		}
		lines[line] = struct{}{}
	}
	if len(lines) == 0 {
		return nil, fmt.Errorf("--lines requires at least one line number")
	}
	return lines, nil
}

func usageText() string {
	return `Usage: mutate4go <file.go> [options]

Examples:
  mutate4go demo/flag.go
  mutate4go demo/flag.go --scan
  mutate4go demo/flag.go --update-manifest
  mutate4go demo/flag.go --lines 12,18

Options:
  --scan
  --update-manifest
  --reuse-coverage
  --since-last-run
  --mutate-all
  --lines 12,18
  --timeout-factor N
  --mutation-warning N
  --max-workers N
  --test-command CMD
  --verbose
  --help
`
}

func maxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
}

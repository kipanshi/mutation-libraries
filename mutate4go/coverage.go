package mutate4go

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const baselineTimeoutMillis = 300000

type DefaultCoverageRunner struct {
	executor ProcessCommandExecutor
}

func ParseCoverageProfile(path string) (CoverageReport, error) {
	file, err := os.Open(path)
	if err != nil {
		return CoverageReport{}, err
	}
	defer file.Close()

	covered := map[string]map[int]struct{}{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "mode:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 3 {
			continue
		}
		count, err := strconv.Atoi(fields[2])
		if err != nil || count <= 0 {
			continue
		}
		pathAndRange := fields[0]
		parts := strings.SplitN(pathAndRange, ":", 2)
		if len(parts) != 2 {
			continue
		}
		lineRange := strings.SplitN(parts[1], ",", 2)
		if len(lineRange) != 2 {
			continue
		}
		startLine, err1 := parseCoverLine(lineRange[0])
		endLine, err2 := parseCoverLine(lineRange[1])
		if err1 != nil || err2 != nil {
			continue
		}
		pathKey := filepath.ToSlash(strings.TrimSpace(parts[0]))
		if covered[pathKey] == nil {
			covered[pathKey] = map[int]struct{}{}
		}
		for lineNo := startLine; lineNo <= endLine; lineNo++ {
			covered[pathKey][lineNo] = struct{}{}
		}
	}
	if err := scanner.Err(); err != nil {
		return CoverageReport{}, err
	}
	return CoverageReport{covered: covered}, nil
}

func parseCoverLine(value string) (int, error) {
	beforeDot := strings.SplitN(value, ".", 2)[0]
	return strconv.Atoi(beforeDot)
}

func (r CoverageReport) Covers(path string, line int) bool {
	if r.covered == nil {
		return false
	}
	lines := r.covered[filepath.ToSlash(path)]
	_, ok := lines[line]
	return ok
}

func (r CoverageReport) CoversAny(paths []string, line int) bool {
	for _, path := range paths {
		if r.Covers(path, line) {
			return true
		}
	}
	return false
}

func newCoverageReport(sites []CoverageSite) CoverageReport {
	covered := map[string]map[int]struct{}{}
	for _, site := range sites {
		pathKey := filepath.ToSlash(site.Path)
		if covered[pathKey] == nil {
			covered[pathKey] = map[int]struct{}{}
		}
		covered[pathKey][site.Line] = struct{}{}
	}
	return CoverageReport{covered: covered}
}

func (r DefaultCoverageRunner) GenerateCoverage(projectRoot string, reuse bool) (CoverageRun, error) {
	profilePath := coverageProfilePath(projectRoot)
	if reuse {
		if _, err := os.Stat(profilePath); err == nil {
			parsed, parseErr := ParseCoverageProfile(profilePath)
			if parseErr != nil {
				return CoverageRun{}, parseErr
			}
			return CoverageRun{Report: parsed, Reused: true, ReportAvailable: true}, nil
		}
		return CoverageRun{Report: newCoverageReport(nil), Reused: true, ReportAvailable: false}, nil
	}
	if err := os.MkdirAll(filepath.Dir(profilePath), 0o755); err != nil {
		return CoverageRun{}, err
	}
	_ = os.Remove(profilePath)
	command := []string{"go", "test", "-coverprofile=" + profilePath, "./..."}
	result, err := r.executor.Run(command, projectRoot, baselineTimeoutMillis)
	if err != nil {
		return CoverageRun{}, err
	}
	baseline := &TestRun{ExitCode: result.ExitCode, Output: result.Output, DurationMillis: result.DurationMillis, TimedOut: result.TimedOut}
	if result.ExitCode == 0 {
		if _, statErr := os.Stat(profilePath); statErr == nil {
			parsed, parseErr := ParseCoverageProfile(profilePath)
			if parseErr != nil {
				return CoverageRun{}, parseErr
			}
			return CoverageRun{Baseline: baseline, Report: parsed, ReportAvailable: true}, nil
		}
	}
	return CoverageRun{Baseline: baseline, Report: newCoverageReport(nil), ReportAvailable: false}, nil
}

func coverageProfilePath(projectRoot string) string {
	return filepath.Join(projectRoot, ".mutate", "coverage", "coverage.out")
}

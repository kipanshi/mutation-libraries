package mutate4go

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
)

type PrintStreamProgressReporter struct {
	out io.Writer
}

type NoOpProgressReporter struct{}

func (NoOpProgressReporter) BaselineStarting(string)              {}
func (NoOpProgressReporter) BaselineFinished(TestRun)             {}
func (NoOpProgressReporter) RunStarting(int, int)                 {}
func (NoOpProgressReporter) MutationStarting(int, MutationJob)    {}
func (NoOpProgressReporter) MutationFinished(int, MutationResult) {}

func (p PrintStreamProgressReporter) BaselineStarting(projectRoot string) {
	fmt.Fprintf(p.out, "Baseline starting for %s\n", projectRoot)
}

func (p PrintStreamProgressReporter) BaselineFinished(run TestRun) {
	fmt.Fprintf(p.out, "Baseline finished: exit=%d timedOut=%t duration=%d ms\n", run.ExitCode, run.TimedOut, run.DurationMillis)
}

func (p PrintStreamProgressReporter) RunStarting(totalJobs int, workers int) {
	fmt.Fprintf(p.out, "Running %d mutations with %d workers.\n", totalJobs, workers)
}

func (p PrintStreamProgressReporter) MutationStarting(workerIndex int, job MutationJob) {
	fmt.Fprintf(p.out, "Worker %d starting %d/%d: %s\n", workerIndex, job.Order, job.TotalJobs, job.Site.Description)
}

func (p PrintStreamProgressReporter) MutationFinished(workerIndex int, result MutationResult) {
	status := "SURVIVED"
	if result.Killed {
		status = "KILLED"
	}
	fmt.Fprintf(p.out, "Worker %d finished %d/%d: %s\n", workerIndex, result.Order, result.TotalJobs, status)
}

func renderScan(out io.Writer, workspaceRoot string, sourceFile string, analysis SourceAnalysis, changed map[string]struct{}) {
	rel := workspaceRelative(workspaceRoot, sourceFile)
	fmt.Fprintf(out, "Scan: %d mutation sites in %s\n", len(analysis.Sites), rel)
	for _, site := range analysis.Sites {
		prefix := "  "
		if _, ok := changed[site.ScopeID]; ok {
			prefix = "* "
		}
		fmt.Fprintf(out, "%s%s:%d %s\n", prefix, rel, site.Line, site.Description)
	}
	if len(changed) > 0 {
		fmt.Fprintln(out, "* indicates a scope that differs from the stored manifest.")
	}
}

func renderSummary(out io.Writer, workspaceRoot string, sourceFile string, baseline TestRun, differential DifferentialSelection, coverage CoverageSelection, results []MutationResult, warningThreshold int) int {
	rel := workspaceRelative(workspaceRoot, sourceFile)
	fmt.Fprintf(out, "Baseline tests passed in %d ms.\n", baseline.DurationMillis)
	fmt.Fprintf(out, "Total mutation sites: %d\n", lenSiteCount(differential, coverage))
	fmt.Fprintf(out, "Covered mutation sites: %d\n", len(coverage.Covered))
	fmt.Fprintf(out, "Uncovered mutation sites: %d\n", len(coverage.Uncovered))
	fmt.Fprintf(out, "Changed mutation sites: %d\n", differential.ChangedMutationSites)
	fmt.Fprintf(out, "Manifest exists: %t\n", differential.ManifestPresent)
	fmt.Fprintf(out, "Module hash changed: %t\n", differential.ModuleHashChanged)
	fmt.Fprintf(out, "Differential surface area: %d\n", differential.DifferentialSurfaceArea)
	fmt.Fprintf(out, "Manifest-violating surface area: %d\n", differential.ManifestViolatingSurfaceArea)
	if len(coverage.Covered) > warningThreshold {
		fmt.Fprintf(out, "WARNING: Found %d mutations. Consider splitting this module.\n", len(coverage.Covered))
	}
	for _, site := range coverage.Uncovered {
		fmt.Fprintf(out, "UNCOVERED %s:%d %s\n", rel, site.Line, site.Description)
	}
	if len(results) == 0 {
		if len(coverage.Uncovered) > 0 {
			fmt.Fprintf(out, "Coverage: %d uncovered sites skipped.\n", len(coverage.Uncovered))
		}
		fmt.Fprintln(out, "No mutations need testing.")
		return 0
	}
	sort.Slice(results, func(i int, j int) bool { return results[i].Order < results[j].Order })
	killed := 0
	survived := 0
	for _, result := range results {
		status := "SURVIVED"
		if result.Killed {
			status = "KILLED"
			killed++
		} else {
			survived++
		}
		suffix := ""
		if result.TimedOut {
			suffix = " timed out"
		}
		fmt.Fprintf(out, "%s %s:%d %s (%d ms)%s\n", status, rel, result.Site.Line, result.Site.Description, result.DurationMillis, suffix)
	}
	fmt.Fprintf(out, "Coverage: %d uncovered sites skipped.\n", len(coverage.Uncovered))
	fmt.Fprintf(out, "Summary: %d killed, %d survived, %d total.\n", killed, survived, len(results))
	if survived > 0 {
		return 3
	}
	return 0
}

func lenSiteCount(differential DifferentialSelection, coverage CoverageSelection) int {
	if differential.TotalMutationSites > 0 {
		return differential.TotalMutationSites
	}
	return len(coverage.Covered) + len(coverage.Uncovered)
}

func workspaceRelative(workspaceRoot string, sourceFile string) string {
	rel, err := filepath.Rel(workspaceRoot, sourceFile)
	if err != nil {
		return filepath.ToSlash(filepath.Base(sourceFile))
	}
	return filepath.ToSlash(rel)
}

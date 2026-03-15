package mutate4go

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

func NewApplication(workspaceRoot string, out io.Writer, err io.Writer, executor TestExecutor, coverage CoverageProvider, reporter ProgressReporter) *Application {
	if out == nil {
		out = io.Discard
	}
	if err == nil {
		err = io.Discard
	}
	if executor == nil {
		defaultExecutor := NewProcessTestCommandExecutor(nil)
		executor = defaultExecutor
	}
	if coverage == nil {
		coverage = DefaultCoverageRunner{executor: ProcessCommandExecutor{}}
	}
	if reporter == nil {
		reporter = NoOpProgressReporter{}
	}
	return &Application{
		workspaceRoot: workspaceRoot,
		out:           out,
		err:           err,
		executor:      executor,
		coverage:      coverage,
		reporter:      reporter,
		catalog:       MutationCatalog{},
		manifestStore: ManifestStore{},
	}
}

func (a *Application) Execute(args []string) (int, error) {
	parsed, err := ParseArgs(args)
	if err != nil {
		fmt.Fprint(a.out, usageText())
		fmt.Fprintln(a.err, err.Error())
		return 1, nil
	}
	if parsed.Mode == ModeHelp {
		fmt.Fprint(a.out, usageText())
		return 0, nil
	}
	sourceFile, err := resolveSourceFile(a.workspaceRoot, parsed.FileArgs[0])
	if err != nil {
		fmt.Fprint(a.out, usageText())
		fmt.Fprintln(a.err, err.Error())
		return 1, nil
	}
	moduleRoot := findModuleRoot(sourceFile, a.workspaceRoot)
	modPath := modulePath(moduleRoot)
	analysis, err := a.catalog.Analyze(sourceFile)
	if err != nil {
		return 1, err
	}

	if parsed.Scan {
		changed, changedErr := a.manifestStore.ChangedScopes(moduleRoot, sourceFile, analysis)
		if changedErr != nil {
			return 1, changedErr
		}
		renderScan(a.out, a.workspaceRoot, sourceFile, analysis, changed.AllScopeIDs())
		return 0, nil
	}
	if parsed.UpdateManifest {
		if err := a.manifestStore.Write(moduleRoot, sourceFile, analysis); err != nil {
			return 1, err
		}
		fmt.Fprintf(a.out, "Updated manifest for %s\n", workspaceRelative(a.workspaceRoot, sourceFile))
		return 0, nil
	}

	coverageRun, filterEnabled, err := a.runBaseline(parsed, moduleRoot)
	if err != nil {
		return 1, err
	}
	baseline := *coverageRun.Baseline
	if baseline.ExitCode != 0 {
		fmt.Fprintln(a.err, "Baseline tests failed.")
		return 2, nil
	}

	differential, err := selectDifferential(parsed, moduleRoot, sourceFile, analysis, a.manifestStore)
	if err != nil {
		return 1, err
	}
	selected := filterLines(differential.Selected, parsed.Lines)
	coverageSelection := filterCoverage(moduleRoot, modPath, sourceFile, selected, coverageRun.Report, filterEnabled)
	if len(coverageSelection.Covered) == 0 {
		exit := renderSummary(a.out, a.workspaceRoot, sourceFile, baseline, differential, coverageSelection, nil, parsed.MutationWarning)
		if exit == 0 {
			if err := a.manifestStore.Write(moduleRoot, sourceFile, analysis); err != nil {
				return 1, err
			}
		}
		return exit, nil
	}

	timeoutMillis := maxInt64(1000, baseline.DurationMillis*int64(parsed.TimeoutFactor))
	results, err := a.runMutations(moduleRoot, coverageSelection.Covered, timeoutMillis, parsed.MaxWorkers, parsed.TestCommand)
	if err != nil {
		return 1, err
	}
	exit := renderSummary(a.out, a.workspaceRoot, sourceFile, baseline, differential, coverageSelection, results, parsed.MutationWarning)
	if exit == 0 {
		if err := a.manifestStore.Write(moduleRoot, sourceFile, analysis); err != nil {
			return 1, err
		}
	}
	return exit, nil
}

func (a *Application) runBaseline(parsed CliArguments, moduleRoot string) (CoverageRun, bool, error) {
	a.reporter.BaselineStarting(moduleRoot)
	if parsed.TestCommand != "" {
		executor := TestExecutor(NewProcessTestCommandExecutor(nil).WithCommand(parsed.TestCommand))
		baseline, err := executor.RunTests(moduleRoot, baselineTimeoutMillis)
		if err != nil {
			return CoverageRun{}, false, err
		}
		a.reporter.BaselineFinished(baseline)
		if parsed.ReuseCoverage {
			if _, err := os.Stat(coverageProfilePath(moduleRoot)); err == nil {
				report, parseErr := ParseCoverageProfile(coverageProfilePath(moduleRoot))
				if parseErr != nil {
					return CoverageRun{}, false, parseErr
				}
				fmt.Fprintln(a.err, "Reusing existing coverage data; coverage may be stale.")
				return CoverageRun{Baseline: &baseline, Report: report, Reused: true, ReportAvailable: true}, true, nil
			}
			fmt.Fprintln(a.err, "Coverage reuse requested, but no coverage report exists. Continuing without coverage filtering.")
		}
		return CoverageRun{Baseline: &baseline, Report: newCoverageReport(nil)}, false, nil
	}
	if parsed.ReuseCoverage {
		baseline, err := a.executor.RunTests(moduleRoot, baselineTimeoutMillis)
		if err != nil {
			return CoverageRun{}, false, err
		}
		a.reporter.BaselineFinished(baseline)
		if _, err := os.Stat(coverageProfilePath(moduleRoot)); err == nil {
			report, parseErr := ParseCoverageProfile(coverageProfilePath(moduleRoot))
			if parseErr != nil {
				return CoverageRun{}, false, parseErr
			}
			fmt.Fprintln(a.err, "Reusing existing coverage data; coverage may be stale.")
			return CoverageRun{Baseline: &baseline, Report: report, Reused: true, ReportAvailable: true}, true, nil
		}
		fmt.Fprintln(a.err, "Coverage reuse requested, but no coverage report exists. Continuing without coverage filtering.")
		return CoverageRun{Baseline: &baseline, Report: newCoverageReport(nil), Reused: true, ReportAvailable: false}, true, nil
	}
	run, err := a.coverage.GenerateCoverage(moduleRoot, false)
	if err != nil {
		return CoverageRun{}, false, err
	}
	if run.Baseline == nil {
		run.Baseline = &TestRun{}
	}
	a.reporter.BaselineFinished(*run.Baseline)
	return run, run.ReportAvailable, nil
}

func (a *Application) runMutations(moduleRoot string, sites []MutationSite, timeoutMillis int64, maxWorkers int, testCommand string) ([]MutationResult, error) {
	workers := maxInt(1, maxWorkers)
	if workers > len(sites) {
		workers = len(sites)
	}
	a.reporter.RunStarting(len(sites), workers)
	jobs := make([]MutationJob, 0, len(sites))
	for index, site := range sites {
		jobs = append(jobs, MutationJob{
			SourceRelativePath: relativeSourcePath(moduleRoot, filepath.FromSlash(site.File)),
			Site:               site,
			TimeoutMillis:      timeoutMillis,
			Order:              index + 1,
			TotalJobs:          len(sites),
		})
	}
	if workers == 1 {
		return a.runWorker(moduleRoot, 1, jobs, testCommand)
	}
	workerRoots, cleanup, err := prepareWorkerRoots(moduleRoot, workers)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	queue := make(chan MutationJob, len(jobs))
	for _, job := range jobs {
		queue <- job
	}
	close(queue)
	resultsCh := make(chan []MutationResult, workers)
	errCh := make(chan error, workers)
	var wg sync.WaitGroup
	for index, root := range workerRoots {
		wg.Add(1)
		go func(workerIndex int, workerRoot string) {
			defer wg.Done()
			results := make([]MutationResult, 0)
			for job := range queue {
				batch, runErr := a.runWorker(workerRoot, workerIndex, []MutationJob{job}, testCommand)
				if runErr != nil {
					errCh <- runErr
					return
				}
				results = append(results, batch...)
			}
			resultsCh <- results
		}(index+1, root)
	}
	wg.Wait()
	close(resultsCh)
	close(errCh)
	for runErr := range errCh {
		if runErr != nil {
			return nil, runErr
		}
	}
	results := make([]MutationResult, 0, len(sites))
	for batch := range resultsCh {
		results = append(results, batch...)
	}
	sort.Slice(results, func(i int, j int) bool { return results[i].Order < results[j].Order })
	return results, nil
}

func (a *Application) runWorker(moduleRoot string, workerIndex int, jobs []MutationJob, testCommand string) ([]MutationResult, error) {
	executor := a.executor
	if testCommand != "" {
		executor = NewProcessTestCommandExecutor(nil).WithCommand(testCommand)
	}
	results := make([]MutationResult, 0, len(jobs))
	for _, job := range jobs {
		a.reporter.MutationStarting(workerIndex, job)
		workerFile := filepath.Join(moduleRoot, filepath.FromSlash(job.SourceRelativePath))
		original, err := os.ReadFile(workerFile)
		if err != nil {
			return nil, err
		}
		mutated := mutateSource(string(original), job.Site)
		if err := os.WriteFile(workerFile, []byte(mutated), 0o644); err != nil {
			return nil, err
		}
		run, runErr := executor.RunTests(moduleRoot, job.TimeoutMillis)
		restoreErr := os.WriteFile(workerFile, original, 0o644)
		if runErr != nil {
			return nil, runErr
		}
		if restoreErr != nil {
			return nil, restoreErr
		}
		result := MutationResult{Site: job.Site, Killed: run.ExitCode != 0, DurationMillis: run.DurationMillis, TimedOut: run.TimedOut, Order: job.Order, TotalJobs: job.TotalJobs}
		a.reporter.MutationFinished(workerIndex, result)
		results = append(results, result)
	}
	return results, nil
}

func mutateSource(source string, site MutationSite) string {
	if site.Start < 0 || site.End < site.Start || site.End > len(source) {
		return source
	}
	return source[:site.Start] + site.ReplacementText + source[site.End:]
}

func Run(args []string, workspaceRoot string, out io.Writer, err io.Writer) int {
	reporter := ProgressReporter(NoOpProgressReporter{})
	parsed, parseErr := ParseArgs(args)
	if parseErr == nil && parsed.Verbose {
		reporter = PrintStreamProgressReporter{out: out}
	}
	app := NewApplication(workspaceRoot, out, err, NewProcessTestCommandExecutor(nil), DefaultCoverageRunner{executor: ProcessCommandExecutor{}}, reporter)
	exit, execErr := app.Execute(args)
	if execErr != nil {
		fmt.Fprintln(err, execErr.Error())
		return 1
	}
	return exit
}

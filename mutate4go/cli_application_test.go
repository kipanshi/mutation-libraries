package mutate4go

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestApplicationPrintsHelpAndExitsZero(t *testing.T) {
	var out bytes.Buffer
	var err bytes.Buffer

	exit, runErr := NewApplication(t.TempDir(), &out, &err, &stubExecutor{}, &stubCoverageProvider{}, noOpProgressReporter{}).Execute([]string{"--help"})
	if runErr != nil {
		t.Fatalf("unexpected error: %v", runErr)
	}
	if exit != 0 || !bytes.Contains(out.Bytes(), []byte("Usage:")) {
		t.Fatalf("unexpected result: exit=%d out=%q err=%q", exit, out.String(), err.String())
	}
}

func TestApplicationReportsMutationProgress(t *testing.T) {
	root := t.TempDir()
	file := writeSampleProjectSource(t, root)
	coverage := &stubCoverageProvider{run: CoverageRun{Baseline: &TestRun{ExitCode: 0, DurationMillis: 10}, Report: newCoverageReport([]CoverageSite{{Path: rel(root, file), Line: 4}}), ReportAvailable: true}}
	executor := &stubExecutor{results: []TestRun{{ExitCode: 1, Output: "killed", DurationMillis: 5}}}
	var progress bytes.Buffer

	exit, runErr := NewApplication(root, &bytes.Buffer{}, &bytes.Buffer{}, executor, coverage, PrintStreamProgressReporter{out: &progress}).Execute([]string{rel(root, file), "--verbose", "--lines", "4"})
	if runErr != nil {
		t.Fatalf("unexpected error: %v", runErr)
	}
	if exit != 0 {
		t.Fatalf("unexpected exit: %d", exit)
	}
	text := progress.String()
	for _, fragment := range []string{"Baseline starting for", "Baseline finished: exit=0", "Running 1 mutations with 1 workers.", "Worker 1 starting 1/1:", "Worker 1 finished 1/1: KILLED"} {
		if !bytes.Contains(progress.Bytes(), []byte(fragment)) {
			t.Fatalf("expected progress fragment %q in %q", fragment, text)
		}
	}
}

func TestApplicationScansMutationSitesWithoutRunningCoverageOrMutants(t *testing.T) {
	root := t.TempDir()
	file := writeSampleProjectSource(t, root)
	coverage := &stubCoverageProvider{}
	executor := &stubExecutor{}
	var out bytes.Buffer

	exit, runErr := NewApplication(root, &out, &bytes.Buffer{}, executor, coverage, noOpProgressReporter{}).Execute([]string{rel(root, file), "--scan"})
	if runErr != nil {
		t.Fatalf("unexpected error: %v", runErr)
	}
	if exit != 0 {
		t.Fatalf("unexpected exit: %d", exit)
	}
	if !bytes.Contains(out.Bytes(), []byte("Scan: 2 mutation sites in demo/sample.go")) {
		t.Fatalf("unexpected output: %q", out.String())
	}
	if executor.invocations != 0 || coverage.invocations != 0 {
		t.Fatalf("unexpected invocations executor=%d coverage=%d", executor.invocations, coverage.invocations)
	}
}

func TestApplicationReusesCoverageWithoutRunningCoverageCommand(t *testing.T) {
	root := t.TempDir()
	file := writeSampleProjectSource(t, root)
	writeCoverageProfile(t, root, []CoverageSite{{Path: rel(root, file), Line: 4}, {Path: rel(root, file), Line: 8}})
	coverage := &stubCoverageProvider{}
	executor := &stubExecutor{results: []TestRun{{ExitCode: 0, Output: "baseline ok", DurationMillis: 10}, {ExitCode: 1, Output: "killed", DurationMillis: 5}, {ExitCode: 1, Output: "killed", DurationMillis: 6}}}
	var err bytes.Buffer

	exit, runErr := NewApplication(root, &bytes.Buffer{}, &err, executor, coverage, noOpProgressReporter{}).Execute([]string{rel(root, file), "--reuse-coverage"})
	if runErr != nil {
		t.Fatalf("unexpected error: %v", runErr)
	}
	if exit != 0 {
		t.Fatalf("unexpected exit: %d", exit)
	}
	if coverage.invocations != 0 || executor.invocations != 3 {
		t.Fatalf("unexpected invocations coverage=%d executor=%d", coverage.invocations, executor.invocations)
	}
	text := err.String()
	if !bytes.Contains(err.Bytes(), []byte("Reusing existing coverage data")) || !bytes.Contains(err.Bytes(), []byte("coverage may be stale")) {
		t.Fatalf("unexpected err output: %q", text)
	}
}

func TestApplicationWarnsWhenCoverageReuseRequestedButNoCoverageExists(t *testing.T) {
	root := t.TempDir()
	file := writeSampleProjectSource(t, root)
	coverage := &stubCoverageProvider{}
	executor := &stubExecutor{results: []TestRun{{ExitCode: 0, Output: "baseline ok", DurationMillis: 10}}}
	var err bytes.Buffer

	exit, runErr := NewApplication(root, &bytes.Buffer{}, &err, executor, coverage, noOpProgressReporter{}).Execute([]string{rel(root, file), "--reuse-coverage"})
	if runErr != nil {
		t.Fatalf("unexpected error: %v", runErr)
	}
	if exit != 0 {
		t.Fatalf("unexpected exit: %d", exit)
	}
	if coverage.invocations != 0 || executor.invocations != 1 {
		t.Fatalf("unexpected invocations coverage=%d executor=%d", coverage.invocations, executor.invocations)
	}
	text := err.String()
	if !bytes.Contains(err.Bytes(), []byte("Coverage reuse requested")) || !bytes.Contains(err.Bytes(), []byte("Continuing without coverage filtering.")) {
		t.Fatalf("unexpected err output: %q", text)
	}
}

func TestApplicationUpdatesManifestWithoutRunningCoverageOrMutants(t *testing.T) {
	root := t.TempDir()
	file := writeSampleProjectSource(t, root)
	coverage := &stubCoverageProvider{}
	executor := &stubExecutor{}
	var out bytes.Buffer

	exit, runErr := NewApplication(root, &out, &bytes.Buffer{}, executor, coverage, noOpProgressReporter{}).Execute([]string{rel(root, file), "--update-manifest"})
	if runErr != nil {
		t.Fatalf("unexpected error: %v", runErr)
	}
	if exit != 0 {
		t.Fatalf("unexpected exit: %d", exit)
	}
	if !bytes.Contains(out.Bytes(), []byte("Updated manifest for demo/sample.go")) {
		t.Fatalf("unexpected output: %q", out.String())
	}
	if executor.invocations != 0 || coverage.invocations != 0 {
		t.Fatalf("unexpected invocations executor=%d coverage=%d", executor.invocations, coverage.invocations)
	}
	if _, statErr := os.Stat(filepath.Join(root, ".mutate", "manifests", "demo", "sample.go.json")); statErr != nil {
		t.Fatalf("expected manifest file: %v", statErr)
	}
}

func TestApplicationStopsWhenBaselineTestsFail(t *testing.T) {
	root := t.TempDir()
	file := writeSampleProjectSource(t, root)
	coverage := &stubCoverageProvider{
		run: CoverageRun{Baseline: &TestRun{ExitCode: 1, Output: "failing baseline", DurationMillis: 10}, Report: newCoverageReport(nil)},
	}
	var err bytes.Buffer

	exit, runErr := NewApplication(root, &bytes.Buffer{}, &err, &stubExecutor{}, coverage, noOpProgressReporter{}).Execute([]string{rel(root, file)})
	if runErr != nil {
		t.Fatalf("unexpected error: %v", runErr)
	}
	if exit != 2 || !bytes.Contains(err.Bytes(), []byte("Baseline tests failed.")) {
		t.Fatalf("unexpected result: exit=%d err=%q", exit, err.String())
	}
}

func TestApplicationReturnsNonZeroWhenAnyMutationSurvives(t *testing.T) {
	root := t.TempDir()
	file := writeSampleProjectSource(t, root)
	coverage := &stubCoverageProvider{run: CoverageRun{Baseline: &TestRun{ExitCode: 0, DurationMillis: 10}, Report: newCoverageReport([]CoverageSite{{Path: rel(root, file), Line: 4}, {Path: rel(root, file), Line: 8}}), ReportAvailable: true}}
	executor := &stubExecutor{results: []TestRun{{ExitCode: 1, Output: "killed", DurationMillis: 5}, {ExitCode: 0, Output: "survived", DurationMillis: 6}}}
	var out bytes.Buffer

	exit, runErr := NewApplication(root, &out, &bytes.Buffer{}, executor, coverage, noOpProgressReporter{}).Execute([]string{rel(root, file)})
	if runErr != nil {
		t.Fatalf("unexpected error: %v", runErr)
	}
	if exit != 3 || !bytes.Contains(out.Bytes(), []byte("KILLED")) || !bytes.Contains(out.Bytes(), []byte("SURVIVED")) {
		t.Fatalf("unexpected result: exit=%d out=%q", exit, out.String())
	}
}

func TestApplicationReturnsZeroWhenAllDiscoveredSitesAreUncovered(t *testing.T) {
	root := t.TempDir()
	file := writeSampleProjectSource(t, root)
	coverage := &stubCoverageProvider{run: CoverageRun{Baseline: &TestRun{ExitCode: 0, DurationMillis: 10}, Report: newCoverageReport(nil), ReportAvailable: true}}
	executor := &stubExecutor{}
	var out bytes.Buffer

	exit, runErr := NewApplication(root, &out, &bytes.Buffer{}, executor, coverage, noOpProgressReporter{}).Execute([]string{rel(root, file)})
	if runErr != nil {
		t.Fatalf("unexpected error: %v", runErr)
	}
	if exit != 0 || !bytes.Contains(out.Bytes(), []byte("Coverage: 2 uncovered sites skipped.")) || executor.invocations != 0 {
		t.Fatalf("unexpected result: exit=%d out=%q invocations=%d", exit, out.String(), executor.invocations)
	}
}

func TestApplicationFiltersMutationsByRequestedLines(t *testing.T) {
	root := t.TempDir()
	file := writeSampleProjectSource(t, root)
	coverage := &stubCoverageProvider{run: CoverageRun{Baseline: &TestRun{ExitCode: 0, DurationMillis: 10}, Report: newCoverageReport([]CoverageSite{{Path: rel(root, file), Line: 4}}), ReportAvailable: true}}
	executor := &stubExecutor{results: []TestRun{{ExitCode: 1, Output: "killed", DurationMillis: 5}}}
	var out bytes.Buffer

	exit, runErr := NewApplication(root, &out, &bytes.Buffer{}, executor, coverage, noOpProgressReporter{}).Execute([]string{rel(root, file), "--lines", "4"})
	if runErr != nil {
		t.Fatalf("unexpected error: %v", runErr)
	}
	if exit != 0 || !bytes.Contains(out.Bytes(), []byte("Summary: 1 killed, 0 survived, 1 total.")) {
		t.Fatalf("unexpected result: exit=%d out=%q", exit, out.String())
	}
	if len(executor.timeouts) != 1 || executor.timeouts[0] != 1000 {
		t.Fatalf("unexpected timeout values: %#v", executor.timeouts)
	}
}

func TestApplicationCountsTimedOutMutantsAsKilled(t *testing.T) {
	root := t.TempDir()
	file := writeUnaryProjectSource(t, root)
	coverage := &stubCoverageProvider{run: CoverageRun{Baseline: &TestRun{ExitCode: 0, DurationMillis: 1000}, Report: newCoverageReport([]CoverageSite{{Path: rel(root, file), Line: 4}}), ReportAvailable: true}}
	executor := &stubExecutor{results: []TestRun{{ExitCode: 124, Output: "timed out mutant", DurationMillis: 1000, TimedOut: true}}}
	var out bytes.Buffer

	exit, runErr := NewApplication(root, &out, &bytes.Buffer{}, executor, coverage, noOpProgressReporter{}).Execute([]string{rel(root, file), "--lines", "4", "--timeout-factor", "1"})
	if runErr != nil {
		t.Fatalf("unexpected error: %v", runErr)
	}
	if exit != 0 || !bytes.Contains(out.Bytes(), []byte("timed out")) {
		t.Fatalf("unexpected result: exit=%d out=%q", exit, out.String())
	}
}

func TestApplicationReportsMatchingManifestAsNoMutationsNeeded(t *testing.T) {
	root := t.TempDir()
	file := writeSampleProjectSource(t, root)
	if err := writeManifestForCurrentAnalysis(root, file); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	coverage := &stubCoverageProvider{run: CoverageRun{Baseline: &TestRun{ExitCode: 0, DurationMillis: 10}, Report: newCoverageReport([]CoverageSite{{Path: rel(root, file), Line: 4}, {Path: rel(root, file), Line: 8}}), ReportAvailable: true}}
	var out bytes.Buffer

	exit, runErr := NewApplication(root, &out, &bytes.Buffer{}, &stubExecutor{}, coverage, noOpProgressReporter{}).Execute([]string{rel(root, file), "--since-last-run"})
	if runErr != nil {
		t.Fatalf("unexpected error: %v", runErr)
	}
	if exit != 0 || !bytes.Contains(out.Bytes(), []byte("No mutations need testing.")) {
		t.Fatalf("unexpected result: exit=%d out=%q", exit, out.String())
	}
}

func TestApplicationMutateAllIgnoresMatchingManifest(t *testing.T) {
	root := t.TempDir()
	file := writeSampleProjectSource(t, root)
	if err := writeManifestForCurrentAnalysis(root, file); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	coverage := &stubCoverageProvider{run: CoverageRun{Baseline: &TestRun{ExitCode: 0, DurationMillis: 10}, Report: newCoverageReport([]CoverageSite{{Path: rel(root, file), Line: 4}, {Path: rel(root, file), Line: 8}}), ReportAvailable: true}}
	executor := &stubExecutor{results: []TestRun{{ExitCode: 1, Output: "killed", DurationMillis: 5}, {ExitCode: 1, Output: "killed", DurationMillis: 5}}}
	var out bytes.Buffer

	exit, runErr := NewApplication(root, &out, &bytes.Buffer{}, executor, coverage, noOpProgressReporter{}).Execute([]string{rel(root, file), "--mutate-all"})
	if runErr != nil {
		t.Fatalf("unexpected error: %v", runErr)
	}
	if exit != 0 || bytes.Contains(out.Bytes(), []byte("No mutations need testing.")) || executor.invocations != 2 {
		t.Fatalf("unexpected result: exit=%d invocations=%d out=%q", exit, executor.invocations, out.String())
	}
}

func TestApplicationReportsSurfaceAreaForChangedAndUnregisteredScopes(t *testing.T) {
	root := t.TempDir()
	file := writeSampleProjectSource(t, root)
	if err := writeManifestForCurrentAnalysis(root, file); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	writeFile(t, file, `package demo

func truthy() bool {
	return false
}

func same(left int, right int) bool {
	return left == right
}

func brandNew() bool {
	return true
}
`)
	coverage := &stubCoverageProvider{run: CoverageRun{Baseline: &TestRun{ExitCode: 0, DurationMillis: 10}, Report: newCoverageReport([]CoverageSite{{Path: rel(root, file), Line: 4}, {Path: rel(root, file), Line: 12}}), ReportAvailable: true}}
	executor := &stubExecutor{results: []TestRun{{ExitCode: 1, Output: "killed", DurationMillis: 5}, {ExitCode: 1, Output: "killed", DurationMillis: 6}}}
	var out bytes.Buffer

	exit, runErr := NewApplication(root, &out, &bytes.Buffer{}, executor, coverage, noOpProgressReporter{}).Execute([]string{rel(root, file), "--since-last-run"})
	if runErr != nil {
		t.Fatalf("unexpected error: %v", runErr)
	}
	if exit != 0 {
		t.Fatalf("unexpected exit: %d output=%q", exit, out.String())
	}
	text := out.String()
	for _, fragment := range []string{"Total mutation sites: 3", "Covered mutation sites: 2", "Changed mutation sites: 2", "Manifest exists: true", "Module hash changed: true", "Differential surface area: 1", "Manifest-violating surface area: 1", "Summary: 2 killed, 0 survived, 2 total."} {
		if !bytes.Contains(out.Bytes(), []byte(fragment)) {
			t.Fatalf("expected output fragment %q in %q", fragment, text)
		}
	}
}

type stubCoverageProvider struct {
	invocations int
	run         CoverageRun
}

func (s *stubCoverageProvider) GenerateCoverage(projectRoot string, reuse bool) (CoverageRun, error) {
	s.invocations++
	return s.run, nil
}

type stubExecutor struct {
	invocations int
	results     []TestRun
	timeouts    []int64
}

func (s *stubExecutor) RunTests(projectRoot string, timeoutMillis int64) (TestRun, error) {
	s.invocations++
	s.timeouts = append(s.timeouts, timeoutMillis)
	if len(s.results) == 0 {
		return TestRun{ExitCode: 0, DurationMillis: 10}, nil
	}
	result := s.results[0]
	s.results = s.results[1:]
	return result, nil
}

type noOpProgressReporter struct{}

func (noOpProgressReporter) BaselineStarting(string)              {}
func (noOpProgressReporter) BaselineFinished(TestRun)             {}
func (noOpProgressReporter) RunStarting(int, int)                 {}
func (noOpProgressReporter) MutationStarting(int, MutationJob)    {}
func (noOpProgressReporter) MutationFinished(int, MutationResult) {}

func writeSampleProjectSource(t *testing.T, root string) string {
	t.Helper()
	file := filepath.Join(root, "demo", "sample.go")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, file, `package demo

func truthy() bool {
	return true
}

func same(left int, right int) bool {
	return left == right
}
`)
	return file
}

func writeUnaryProjectSource(t *testing.T, root string) string {
	t.Helper()
	file := filepath.Join(root, "demo", "guard.go")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, file, `package demo

func allows(blocked bool) bool {
	return !blocked
}
`)
	return file
}

func rel(root string, file string) string {
	relPath, err := filepath.Rel(root, file)
	if err != nil {
		panic(err)
	}
	return filepath.ToSlash(relPath)
}

func writeManifestForCurrentAnalysis(root string, file string) error {
	analysis, err := MutationCatalog{}.Analyze(file)
	if err != nil {
		return err
	}
	return ManifestStore{}.Write(root, file, analysis)
}

func writeCoverageProfile(t *testing.T, root string, sites []CoverageSite) {
	t.Helper()
	path := coverageProfilePath(root)
	var profile bytes.Buffer
	profile.WriteString("mode: set\n")
	for _, site := range sites {
		profile.WriteString(site.Path + ":" + intToString(site.Line) + ".1," + intToString(site.Line) + ".2 1 1\n")
	}
	writeFile(t, path, profile.String())
}

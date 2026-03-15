package mutate4go

import "io"

type CliMode string

const (
	ModeExplicitFiles CliMode = "explicit_files"
	ModeHelp          CliMode = "help"
)

type CliArguments struct {
	Mode            CliMode
	FileArgs        []string
	Lines           map[int]struct{}
	Scan            bool
	UpdateManifest  bool
	ReuseCoverage   bool
	SinceLastRun    bool
	MutateAll       bool
	TimeoutFactor   int
	MutationWarning int
	MaxWorkers      int
	TestCommand     string
	Verbose         bool
}

type MutationSite struct {
	File            string
	Line            int
	Start           int
	End             int
	OriginalText    string
	ReplacementText string
	Description     string
	ScopeID         string
	ScopeKind       string
	ScopeStartLine  int
	ScopeEndLine    int
}

type MutationScope struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	StartLine    int    `json:"start_line"`
	EndLine      int    `json:"end_line"`
	SemanticHash string `json:"semantic_hash"`
}

type SourceAnalysis struct {
	Source     string
	Sites      []MutationSite
	Scopes     []MutationScope
	ModuleHash string
}

type DifferentialManifest struct {
	Version    int             `json:"version"`
	ModuleHash string          `json:"module_hash"`
	Scopes     []MutationScope `json:"scopes"`
}

type DifferentialSelection struct {
	Selected                     []MutationSite
	SkipAll                      bool
	ManifestPresent              bool
	ModuleHashChanged            bool
	TotalMutationSites           int
	ChangedMutationSites         int
	DifferentialSurfaceArea      int
	ManifestViolatingSurfaceArea int
}

type CoverageSite struct {
	Path string
	Line int
}

type CoverageReport struct {
	covered map[string]map[int]struct{}
}

type CoverageRun struct {
	Baseline        *TestRun
	Report          CoverageReport
	Reused          bool
	ReportAvailable bool
}

type CoverageSelection struct {
	Covered   []MutationSite
	Uncovered []MutationSite
}

type TestRun struct {
	ExitCode       int
	Output         string
	DurationMillis int64
	TimedOut       bool
}

type MutationJob struct {
	SourceRelativePath string
	Site               MutationSite
	TimeoutMillis      int64
	Order              int
	TotalJobs          int
}

type MutationResult struct {
	Site           MutationSite
	Killed         bool
	DurationMillis int64
	TimedOut       bool
	Order          int
	TotalJobs      int
}

type MutantResultSummary struct {
	SourceFile string
	Baseline   TestRun
	ExtraText  string
	Uncovered  []MutationSite
	Results    []MutationResult
}

type CommandResult struct {
	ExitCode       int
	Output         string
	DurationMillis int64
	TimedOut       bool
}

type CoverageProvider interface {
	GenerateCoverage(projectRoot string, reuse bool) (CoverageRun, error)
}

type TestExecutor interface {
	RunTests(projectRoot string, timeoutMillis int64) (TestRun, error)
}

type ProgressReporter interface {
	BaselineStarting(projectRoot string)
	BaselineFinished(TestRun)
	RunStarting(totalJobs int, workers int)
	MutationStarting(workerIndex int, job MutationJob)
	MutationFinished(workerIndex int, result MutationResult)
}

type Application struct {
	workspaceRoot string
	out           io.Writer
	err           io.Writer
	executor      TestExecutor
	coverage      CoverageProvider
	reporter      ProgressReporter
	catalog       MutationCatalog
	manifestStore ManifestStore
}

type MutationCatalog struct{}

type ProcessCommandExecutor struct{}

type ProcessTestCommandExecutor struct {
	command      []string
	shellCommand string
	runner       ProcessCommandExecutor
}

type ManifestStore struct{}

type ChangedScopes struct {
	ManifestPresent         bool
	ModuleHashChanged       bool
	UnregisteredScopeIDs    map[string]struct{}
	ManifestViolationScopes map[string]struct{}
}

func (c ChangedScopes) AllScopeIDs() map[string]struct{} {
	all := map[string]struct{}{}
	for id := range c.UnregisteredScopeIDs {
		all[id] = struct{}{}
	}
	for id := range c.ManifestViolationScopes {
		all[id] = struct{}{}
	}
	return all
}

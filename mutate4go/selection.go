package mutate4go

func selectDifferential(args CliArguments, moduleRoot string, sourceFile string, analysis SourceAnalysis, store ManifestStore) (DifferentialSelection, error) {
	if args.MutateAll {
		return notDifferential(analysis), nil
	}
	if len(args.Lines) > 0 && !args.SinceLastRun {
		return notDifferential(analysis), nil
	}
	changed, err := store.ChangedScopes(moduleRoot, sourceFile, analysis)
	if err != nil {
		return DifferentialSelection{}, err
	}
	if !changed.ManifestPresent {
		return notDifferential(analysis), nil
	}
	all := changed.AllScopeIDs()
	changedCount := mutationCount(analysis.Sites, all)
	if len(all) == 0 {
		return DifferentialSelection{
			Selected:                     nil,
			SkipAll:                      true,
			ManifestPresent:              true,
			ModuleHashChanged:            changed.ModuleHashChanged,
			TotalMutationSites:           len(analysis.Sites),
			ChangedMutationSites:         changedCount,
			DifferentialSurfaceArea:      mutationCount(analysis.Sites, changed.UnregisteredScopeIDs),
			ManifestViolatingSurfaceArea: mutationCount(analysis.Sites, changed.ManifestViolationScopes),
		}, nil
	}
	selected := make([]MutationSite, 0)
	for _, site := range analysis.Sites {
		if _, ok := all[site.ScopeID]; ok {
			selected = append(selected, site)
		}
	}
	return DifferentialSelection{
		Selected:                     selected,
		ManifestPresent:              true,
		ModuleHashChanged:            changed.ModuleHashChanged,
		TotalMutationSites:           len(analysis.Sites),
		ChangedMutationSites:         changedCount,
		DifferentialSurfaceArea:      mutationCount(analysis.Sites, changed.UnregisteredScopeIDs),
		ManifestViolatingSurfaceArea: mutationCount(analysis.Sites, changed.ManifestViolationScopes),
	}, nil
}

func notDifferential(analysis SourceAnalysis) DifferentialSelection {
	return DifferentialSelection{Selected: analysis.Sites, TotalMutationSites: len(analysis.Sites)}
}

func mutationCount(sites []MutationSite, scopeIDs map[string]struct{}) int {
	count := 0
	for _, site := range sites {
		if _, ok := scopeIDs[site.ScopeID]; ok {
			count++
		}
	}
	return count
}

func filterLines(sites []MutationSite, lines map[int]struct{}) []MutationSite {
	if len(lines) == 0 {
		return sites
	}
	filtered := make([]MutationSite, 0)
	for _, site := range sites {
		if _, ok := lines[site.Line]; ok {
			filtered = append(filtered, site)
		}
	}
	return filtered
}

func filterCoverage(moduleRoot string, modPath string, sourceFile string, sites []MutationSite, report CoverageReport, filterEnabled bool) CoverageSelection {
	if !filterEnabled {
		return CoverageSelection{Covered: sites}
	}
	paths := coveragePathVariants(moduleRoot, modPath, sourceFile)
	covered := make([]MutationSite, 0)
	uncovered := make([]MutationSite, 0)
	for _, site := range sites {
		if report.CoversAny(paths, site.Line) {
			covered = append(covered, site)
		} else {
			uncovered = append(uncovered, site)
		}
	}
	return CoverageSelection{Covered: covered, Uncovered: uncovered}
}

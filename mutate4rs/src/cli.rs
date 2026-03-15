use std::collections::BTreeSet;

use crate::model::{CliArguments, CliMode};

pub fn parse_args(args: &[&str]) -> Result<CliArguments, String> {
    if args == ["--help"] {
        return Ok(CliArguments {
            mode: CliMode::Help,
            file_args: vec![],
            lines: BTreeSet::new(),
            scan: false,
            update_manifest: false,
            reuse_coverage: false,
            since_last_run: false,
            mutate_all: false,
            timeout_factor: 10,
            mutation_warning: 50,
            max_workers: std::cmp::max(
                1,
                std::thread::available_parallelism().map_or(1, usize::from) / 2,
            ),
            test_command: None,
            verbose: false,
        });
    }

    let mut file_args = Vec::new();
    let mut lines = BTreeSet::new();
    let mut scan = false;
    let mut update_manifest = false;
    let mut reuse_coverage = false;
    let mut since_last_run = false;
    let mut mutate_all = false;
    let mut timeout_factor = 10;
    let mut mutation_warning = 50;
    let mut max_workers = std::cmp::max(
        1,
        std::thread::available_parallelism().map_or(1, usize::from) / 2,
    );
    let mut test_command = None;
    let mut verbose = false;

    let mut index = 0;
    while index < args.len() {
        let arg = args[index];
        if !arg.starts_with("--") {
            file_args.push(arg.to_string());
            index += 1;
            continue;
        }

        match arg {
            "--scan" => scan = true,
            "--update-manifest" => update_manifest = true,
            "--reuse-coverage" => reuse_coverage = true,
            "--since-last-run" => since_last_run = true,
            "--mutate-all" => mutate_all = true,
            "--verbose" => verbose = true,
            "--lines" => {
                index += 1;
                let value = args.get(index).ok_or("--lines requires a value")?;
                lines = parse_lines(value)?;
            }
            "--timeout-factor" => {
                timeout_factor = parse_positive_int(args, &mut index, "--timeout-factor")?;
            }
            "--mutation-warning" => {
                mutation_warning = parse_positive_int(args, &mut index, "--mutation-warning")?;
            }
            "--max-workers" => {
                max_workers = parse_positive_int(args, &mut index, "--max-workers")?;
            }
            "--test-command" => {
                index += 1;
                let value = args
                    .get(index)
                    .ok_or("--test-command requires a value")?
                    .trim();
                if value.is_empty() {
                    return Err("--test-command must not be blank".to_string());
                }
                test_command = Some(value.to_string());
            }
            _ => return Err(format!("Unknown option: {arg}")),
        }
        index += 1;
    }

    if file_args.is_empty() {
        return Err("mutate4rs requires exactly one Rust file".to_string());
    }
    if file_args.len() != 1 {
        return Err("mutate4rs accepts exactly one Rust file".to_string());
    }
    if !file_args[0].ends_with(".rs") {
        return Err("mutate4rs target must be a .rs file".to_string());
    }
    if !lines.is_empty() && since_last_run {
        return Err("--lines may not be combined with --since-last-run".to_string());
    }

    Ok(CliArguments {
        mode: CliMode::ExplicitFiles,
        file_args,
        lines,
        scan,
        update_manifest,
        reuse_coverage,
        since_last_run,
        mutate_all,
        timeout_factor,
        mutation_warning,
        max_workers,
        test_command,
        verbose,
    })
}

fn parse_positive_int(args: &[&str], index: &mut usize, name: &str) -> Result<usize, String> {
    *index += 1;
    let value = args
        .get(*index)
        .ok_or_else(|| format!("{name} requires a value"))?;
    value
        .parse::<usize>()
        .map_err(|_| format!("{name} must be a positive integer"))
        .and_then(|parsed| {
            if parsed == 0 {
                Err(format!("{name} must be a positive integer"))
            } else {
                Ok(parsed)
            }
        })
}

fn parse_lines(value: &str) -> Result<BTreeSet<usize>, String> {
    let trimmed = value.trim();
    if trimmed.is_empty() || trimmed.trim_matches(',').is_empty() {
        return Err("--lines requires at least one line number".to_string());
    }
    let mut lines = BTreeSet::new();
    for part in trimmed.split(',') {
        let part = part.trim();
        if part.is_empty() {
            continue;
        }
        let line = part
            .parse::<usize>()
            .map_err(|_| "--lines must be a positive integer".to_string())?;
        if line == 0 {
            return Err("--lines must be a positive integer".to_string());
        }
        lines.insert(line);
    }
    if lines.is_empty() {
        return Err("--lines requires at least one line number".to_string());
    }
    Ok(lines)
}

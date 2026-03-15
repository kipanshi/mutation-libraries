use std::{fs, path::Path};

use crate::model::CoverageReport;

pub fn parse_lcov(path: &Path) -> Result<CoverageReport, String> {
    let content = fs::read_to_string(path).map_err(|err| err.to_string())?;
    let mut report = CoverageReport::default();
    let mut current_file: Option<String> = None;

    for line in content.lines() {
        if let Some(rest) = line.strip_prefix("SF:") {
            current_file = Some(rest.trim().to_string());
            continue;
        }
        if let Some(rest) = line.strip_prefix("DA:") {
            let Some(file) = current_file.clone() else {
                continue;
            };
            let mut parts = rest.split(',');
            let Ok(line_number) = parts.next().unwrap_or_default().parse::<usize>() else {
                continue;
            };
            let Ok(hits) = parts.next().unwrap_or_default().parse::<usize>() else {
                continue;
            };
            if hits > 0 {
                report.covered.entry(file).or_default().insert(line_number);
            }
        }
    }

    Ok(report)
}

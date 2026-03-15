use std::{
    fs,
    path::PathBuf,
    time::{SystemTime, UNIX_EPOCH},
};

use mutate4rs::MutationCatalog;

#[test]
fn discovers_boolean_equality_and_comparison_mutations() {
    let file = temp_rust_file(
        r#"fn truthy() -> bool {
    true
}

fn same(left: i32, right: i32) -> bool {
    left == right
}

fn larger(left: i32, right: i32) -> bool {
    left > right
}

fn smaller(left: i32, right: i32) -> bool {
    left <= right
}
"#,
    );

    let sites = MutationCatalog.discover(&[file.clone()]).unwrap();
    assert_eq!(
        vec![
            "replace true with false",
            "replace == with !=",
            "replace > with >=",
            "replace <= with <",
        ],
        sites
            .iter()
            .map(|site| site.description.clone())
            .collect::<Vec<_>>()
    );
}

#[test]
fn ignores_operators_inside_strings_and_comments() {
    let file = temp_rust_file(
        r#"fn sample(left: i32, right: i32) -> &'static str {
    let _text = "true == false > <";
    // left == right > 0
    if left == right {
        "same"
    } else {
        "different"
    }
}
"#,
    );

    let sites = MutationCatalog.discover(&[file]).unwrap();
    assert_eq!(
        vec!["replace == with !="],
        sites
            .iter()
            .map(|site| site.description.clone())
            .collect::<Vec<_>>()
    );
}

#[test]
fn discovers_arithmetic_logical_unary_and_constant_mutations() {
    let file = temp_rust_file(
        r#"fn add(left: i32, right: i32) -> i32 { left + right }
fn divide(left: i32, right: i32) -> i32 { left / right }
fn both(left: bool, right: bool) -> bool { left && right }
fn invert(value: bool) -> bool { !value }
fn negative(value: i32) -> i32 { -value }
fn zero() -> i32 { 0 }
fn one() -> i32 { 1 }
"#,
    );

    let sites = MutationCatalog.discover(&[file]).unwrap();
    assert_eq!(
        vec![
            "replace + with -",
            "replace / with *",
            "replace && with ||",
            "replace ! with removed !",
            "replace - with removed -",
            "replace 0 with 1",
            "replace 1 with 0",
        ],
        sites
            .iter()
            .map(|site| site.description.clone())
            .collect::<Vec<_>>()
    );
    assert_eq!("", sites[3].replacement_text);
    assert_eq!("", sites[4].replacement_text);
}

#[test]
fn analyze_returns_scopes_and_module_hash() {
    let file = temp_rust_file(
        r#"fn truthy() -> bool {
    true
}

fn same(left: i32, right: i32) -> bool {
    left == right
}
"#,
    );

    let analysis = MutationCatalog.analyze(&file).unwrap();
    assert!(!analysis.module_hash.is_empty());
    assert!(!analysis.scopes.is_empty());
    assert_eq!(2, analysis.sites.len());
}

fn temp_rust_file(content: &str) -> PathBuf {
    let unique = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap()
        .as_nanos();
    let dir = std::env::temp_dir().join(format!("mutate4rs-tests-{unique}"));
    fs::create_dir_all(&dir).unwrap();
    let file = dir.join("sample.rs");
    fs::write(&file, content).unwrap();
    file
}

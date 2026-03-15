use std::{
    collections::BTreeMap,
    fs,
    path::{Path, PathBuf},
};

use proc_macro2::LineColumn;
use syn::{
    spanned::Spanned, visit::Visit, BinOp, ExprBinary, ExprLit, ExprUnary, File, ItemFn, Lit, UnOp,
};

use crate::model::{MutationScope, MutationSite, SourceAnalysis};

pub struct MutationCatalog;

impl MutationCatalog {
    pub fn discover(&self, files: &[PathBuf]) -> Result<Vec<MutationSite>, String> {
        let mut sites = Vec::new();
        for file in files {
            sites.extend(self.analyze(file)?.sites);
        }
        sites.sort_by(|left, right| {
            left.file
                .cmp(&right.file)
                .then(left.start.cmp(&right.start))
        });
        Ok(sites)
    }

    pub fn analyze(&self, file: &Path) -> Result<SourceAnalysis, String> {
        let source = fs::read_to_string(file).map_err(|err| err.to_string())?;
        let parsed: File = syn::parse_file(&source).map_err(|err| err.to_string())?;
        let line_offsets = line_offsets(&source);
        let (mut scopes, mut sites) = {
            let mut visitor = MutationVisitor::new(file, &source, line_offsets);
            visitor.visit_file(&parsed);
            (
                visitor.scopes.into_values().collect::<Vec<_>>(),
                visitor.sites,
            )
        };
        scopes.sort_by(|left, right| left.id.cmp(&right.id));
        if scopes.is_empty() {
            scopes.push(MutationScope {
                id: format!(
                    "file:{}",
                    file.file_name()
                        .and_then(|name| name.to_str())
                        .unwrap_or("unknown.rs")
                ),
                kind: "file".to_string(),
                start_line: 1,
                end_line: source.lines().count().max(1),
                semantic_hash: hash_text(&source),
            });
        }
        sites.sort_by(|left, right| left.start.cmp(&right.start));
        let module_hash = hash_text(
            &scopes
                .iter()
                .map(|scope| format!("{}|{}", scope.id, scope.semantic_hash))
                .collect::<Vec<_>>()
                .join("\n"),
        );

        Ok(SourceAnalysis {
            source,
            sites,
            scopes,
            module_hash,
        })
    }
}

#[derive(Clone)]
struct ScopeRef {
    id: String,
    kind: String,
    start_line: usize,
    end_line: usize,
}

struct MutationVisitor<'a> {
    file: &'a Path,
    source: &'a str,
    line_offsets: Vec<usize>,
    sites: Vec<MutationSite>,
    scopes: BTreeMap<String, MutationScope>,
    scope_stack: Vec<ScopeRef>,
}

impl<'a> MutationVisitor<'a> {
    fn new(file: &'a Path, source: &'a str, line_offsets: Vec<usize>) -> Self {
        Self {
            file,
            source,
            line_offsets,
            sites: Vec::new(),
            scopes: BTreeMap::new(),
            scope_stack: Vec::new(),
        }
    }

    fn current_scope(&self, line: usize) -> ScopeRef {
        self.scope_stack
            .last()
            .cloned()
            .unwrap_or_else(|| ScopeRef {
                id: format!(
                    "file:{}",
                    self.file
                        .file_name()
                        .and_then(|name| name.to_str())
                        .unwrap_or("unknown.rs")
                ),
                kind: "file".to_string(),
                start_line: line,
                end_line: line,
            })
    }

    fn offset(&self, position: LineColumn) -> usize {
        self.line_offsets[position.line - 1] + position.column
    }

    fn add_site(
        &mut self,
        line: usize,
        start: usize,
        end: usize,
        original: &str,
        replacement: &str,
        description: String,
    ) {
        let scope = self.current_scope(line);
        self.sites.push(MutationSite {
            file: self.file.to_string_lossy().replace('\\', "/"),
            line,
            start,
            end,
            original_text: original.to_string(),
            replacement_text: replacement.to_string(),
            description,
            scope_id: scope.id,
            scope_kind: scope.kind,
            scope_start_line: scope.start_line,
            scope_end_line: scope.end_line,
        });
    }
}

impl<'ast, 'a> Visit<'ast> for MutationVisitor<'a> {
    fn visit_item_fn(&mut self, node: &'ast ItemFn) {
        let start = node.sig.ident.span().start().line;
        let end = node.block.brace_token.span.close().end().line;
        let id = format!("function:{}:{}", node.sig.ident, start);
        let span = node.span();
        let snippet = &self.source[self.offset(span.start())..self.offset(span.end())];
        self.scopes.insert(
            id.clone(),
            MutationScope {
                id: id.clone(),
                kind: "function".to_string(),
                start_line: start,
                end_line: end,
                semantic_hash: hash_text(snippet),
            },
        );
        self.scope_stack.push(ScopeRef {
            id,
            kind: "function".to_string(),
            start_line: start,
            end_line: end,
        });
        syn::visit::visit_item_fn(self, node);
        self.scope_stack.pop();
    }

    fn visit_expr_lit(&mut self, node: &'ast ExprLit) {
        match &node.lit {
            Lit::Bool(boolean) => {
                let span = boolean.span();
                let start = self.offset(span.start());
                let end = self.offset(span.end());
                let original = if boolean.value { "true" } else { "false" };
                let replacement = if boolean.value { "false" } else { "true" };
                self.add_site(
                    span.start().line,
                    start,
                    end,
                    original,
                    replacement,
                    format!("replace {original} with {replacement}"),
                );
            }
            Lit::Int(int) if int.base10_digits() == "0" || int.base10_digits() == "1" => {
                let span = int.span();
                let start = self.offset(span.start());
                let end = self.offset(span.end());
                let original = int.base10_digits();
                let replacement = if original == "0" { "1" } else { "0" };
                self.add_site(
                    span.start().line,
                    start,
                    end,
                    original,
                    replacement,
                    format!("replace {original} with {replacement}"),
                );
            }
            _ => {}
        }
        syn::visit::visit_expr_lit(self, node);
    }

    fn visit_expr_binary(&mut self, node: &'ast ExprBinary) {
        if let Some(replacement) = binary_replacement(&node.op) {
            let left_end = self.offset(node.left.span().end());
            let right_start = self.offset(node.right.span().start());
            let between = &self.source[left_end..right_start];
            let operator = between.trim();
            if !operator.is_empty() {
                if let Some(relative) = between.find(operator) {
                    let start = left_end + relative;
                    let end = start + operator.len();
                    self.add_site(
                        node.span().start().line,
                        start,
                        end,
                        operator,
                        replacement,
                        format!("replace {operator} with {replacement}"),
                    );
                }
            }
        }
        syn::visit::visit_expr_binary(self, node);
    }

    fn visit_expr_unary(&mut self, node: &'ast ExprUnary) {
        if let Some(operator) = unary_operator(&node.op) {
            let expr_start = self.offset(node.span().start());
            let operand_start = self.offset(node.expr.span().start());
            let between = &self.source[expr_start..operand_start];
            if let Some(relative) = between.find(operator) {
                let start = expr_start + relative;
                let end = start + operator.len();
                self.add_site(
                    node.span().start().line,
                    start,
                    end,
                    operator,
                    "",
                    format!("replace {operator} with removed {operator}"),
                );
            }
        }
        syn::visit::visit_expr_unary(self, node);
    }
}

fn line_offsets(source: &str) -> Vec<usize> {
    let mut offsets = vec![0];
    for (index, ch) in source.char_indices() {
        if ch == '\n' {
            offsets.push(index + 1);
        }
    }
    offsets
}

fn hash_text(text: &str) -> String {
    use std::hash::{Hash, Hasher};
    let mut hasher = std::collections::hash_map::DefaultHasher::new();
    text.hash(&mut hasher);
    format!("{:016x}", hasher.finish())
}

fn binary_replacement(op: &BinOp) -> Option<&'static str> {
    Some(match op {
        BinOp::Eq(_) => "!=",
        BinOp::Ne(_) => "==",
        BinOp::Gt(_) => ">=",
        BinOp::Ge(_) => ">",
        BinOp::Lt(_) => "<=",
        BinOp::Le(_) => "<",
        BinOp::Add(_) => "-",
        BinOp::Sub(_) => "+",
        BinOp::Mul(_) => "/",
        BinOp::Div(_) => "*",
        BinOp::And(_) => "||",
        BinOp::Or(_) => "&&",
        _ => return None,
    })
}

fn unary_operator(op: &UnOp) -> Option<&'static str> {
    Some(match op {
        UnOp::Not(_) => "!",
        UnOp::Neg(_) => "-",
        _ => return None,
    })
}

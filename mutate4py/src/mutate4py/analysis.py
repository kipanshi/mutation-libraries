from __future__ import annotations

import ast
import hashlib
from dataclasses import dataclass
from pathlib import Path

from mutate4py.model import MutationScope, MutationSite, SourceAnalysis


class MutationCatalog:
    def discover(self, files: list[str]) -> list[MutationSite]:
        sites: list[MutationSite] = []
        for file_path in files:
            sites.extend(self.analyze(file_path).sites)
        sites.sort(key=lambda site: (site.file, site.start))
        return sites

    def analyze(self, file_path: str) -> SourceAnalysis:
        source = Path(file_path).read_text(encoding="utf-8")
        tree = ast.parse(source, filename=file_path)
        offsets = _line_offsets(source)
        visitor = _MutationVisitor(file_path, source, offsets)
        visitor.visit(tree)
        scopes = sorted(visitor.scopes.values(), key=lambda scope: scope.id)
        if not scopes:
            scopes = [
                MutationScope(
                    id=f"file:{Path(file_path).name}",
                    kind="file",
                    start_line=1,
                    end_line=max(1, source.count("\n") + 1),
                    semantic_hash=_hash_text(source),
                )
            ]
        module_hash = _hash_text(
            "\n".join(f"{scope.id}|{scope.semantic_hash}" for scope in scopes)
        )
        return SourceAnalysis(
            source=source, sites=visitor.sites, scopes=scopes, module_hash=module_hash
        )


@dataclass(frozen=True)
class _ScopeRef:
    id: str
    kind: str
    start_line: int
    end_line: int


class _MutationVisitor(ast.NodeVisitor):
    def __init__(self, file_path: str, source: str, offsets: list[int]) -> None:
        self.file_path = file_path
        self.source = source
        self.offsets = offsets
        self.sites: list[MutationSite] = []
        self.scopes: dict[str, MutationScope] = {}
        self.scope_stack: list[_ScopeRef] = []

    def visit_FunctionDef(self, node: ast.FunctionDef) -> None:
        self._visit_function(node, "function")

    def visit_AsyncFunctionDef(self, node: ast.AsyncFunctionDef) -> None:
        self._visit_function(node, "function")

    def visit_Constant(self, node: ast.Constant) -> None:
        current = self._current_scope(node)
        if isinstance(node.value, bool):
            original = "True" if node.value else "False"
            replacement = "False" if node.value else "True"
            self._add_site(
                node,
                original,
                replacement,
                f"replace {original} with {replacement}",
                current,
            )
        elif isinstance(node.value, int) and node.value in (0, 1):
            original = str(node.value)
            replacement = "1" if node.value == 0 else "0"
            self._add_site(
                node,
                original,
                replacement,
                f"replace {original} with {replacement}",
                current,
            )
        self.generic_visit(node)

    def visit_Compare(self, node: ast.Compare) -> None:
        current = self._current_scope(node)
        if len(node.ops) == 1 and len(node.comparators) == 1:
            replacement = _compare_replacement(node.ops[0])
            if replacement is not None:
                left_end = self._node_offset(node.left, end=True)
                right_start = self._node_offset(node.comparators[0], end=False)
                operator = self.source[left_end:right_start].strip()
                start = self.source.find(operator, left_end, right_start)
                if start >= 0:
                    self.sites.append(
                        MutationSite(
                            file=self.file_path,
                            line=node.lineno,
                            start=start,
                            end=start + len(operator),
                            original_text=operator,
                            replacement_text=replacement,
                            description=f"replace {operator} with {replacement}",
                            scope_id=current.id,
                            scope_kind=current.kind,
                            scope_start_line=current.start_line,
                            scope_end_line=current.end_line,
                        )
                    )
        self.generic_visit(node)

    def visit_BinOp(self, node: ast.BinOp) -> None:
        current = self._current_scope(node)
        replacement = _binop_replacement(node.op)
        if replacement is not None:
            left_end = self._node_offset(node.left, end=True)
            right_start = self._node_offset(node.right, end=False)
            operator = self.source[left_end:right_start].strip()
            start = self.source.find(operator, left_end, right_start)
            if start >= 0:
                self.sites.append(
                    MutationSite(
                        file=self.file_path,
                        line=node.lineno,
                        start=start,
                        end=start + len(operator),
                        original_text=operator,
                        replacement_text=replacement,
                        description=f"replace {operator} with {replacement}",
                        scope_id=current.id,
                        scope_kind=current.kind,
                        scope_start_line=current.start_line,
                        scope_end_line=current.end_line,
                    )
                )
        self.generic_visit(node)

    def visit_BoolOp(self, node: ast.BoolOp) -> None:
        current = self._current_scope(node)
        replacement = _boolop_replacement(node.op)
        if replacement is not None and len(node.values) >= 2:
            left_end = self._node_offset(node.values[0], end=True)
            right_start = self._node_offset(node.values[1], end=False)
            operator = self.source[left_end:right_start].strip()
            start = self.source.find(operator, left_end, right_start)
            if start >= 0:
                self.sites.append(
                    MutationSite(
                        file=self.file_path,
                        line=node.lineno,
                        start=start,
                        end=start + len(operator),
                        original_text=operator,
                        replacement_text=replacement,
                        description=f"replace {operator} with {replacement}",
                        scope_id=current.id,
                        scope_kind=current.kind,
                        scope_start_line=current.start_line,
                        scope_end_line=current.end_line,
                    )
                )
        self.generic_visit(node)

    def visit_UnaryOp(self, node: ast.UnaryOp) -> None:
        current = self._current_scope(node)
        operator = None
        if isinstance(node.op, ast.Not):
            operator = "not"
        elif isinstance(node.op, ast.USub):
            operator = "-"
        if operator is not None:
            start = self._node_offset(node, end=False)
            operand_start = self._node_offset(node.operand, end=False)
            slice_text = self.source[start:operand_start]
            index = slice_text.find(operator)
            if index >= 0:
                operator_start = start + index
                self.sites.append(
                    MutationSite(
                        file=self.file_path,
                        line=node.lineno,
                        start=operator_start,
                        end=operator_start + len(operator),
                        original_text=operator,
                        replacement_text="",
                        description=f"replace {operator} with removed {operator}",
                        scope_id=current.id,
                        scope_kind=current.kind,
                        scope_start_line=current.start_line,
                        scope_end_line=current.end_line,
                    )
                )
        self.generic_visit(node)

    def _visit_function(self, node: ast.AST, kind: str) -> None:
        name = node.name  # type: ignore[attr-defined]
        start_line = node.lineno  # type: ignore[attr-defined]
        end_line = node.end_lineno  # type: ignore[attr-defined]
        scope_id = f"{kind}:{name}:{start_line}"
        start = self._node_offset(node, end=False)
        end = self._node_offset(node, end=True)
        self.scopes[scope_id] = MutationScope(
            id=scope_id,
            kind=kind,
            start_line=start_line,
            end_line=end_line,
            semantic_hash=_hash_text(self.source[start:end]),
        )
        self.scope_stack.append(_ScopeRef(scope_id, kind, start_line, end_line))
        self.generic_visit(node)
        self.scope_stack.pop()

    def _add_site(
        self,
        node: ast.AST,
        original: str,
        replacement: str,
        description: str,
        current: _ScopeRef,
    ) -> None:
        start = self._node_offset(node, end=False)
        end = self._node_offset(node, end=True)
        self.sites.append(
            MutationSite(
                file=self.file_path,
                line=getattr(node, "lineno", 1),
                start=start,
                end=end,
                original_text=original,
                replacement_text=replacement,
                description=description,
                scope_id=current.id,
                scope_kind=current.kind,
                scope_start_line=current.start_line,
                scope_end_line=current.end_line,
            )
        )

    def _current_scope(self, node: ast.AST) -> _ScopeRef:
        if self.scope_stack:
            return self.scope_stack[-1]
        line = getattr(node, "lineno", 1)
        end_line = getattr(node, "end_lineno", line)
        return _ScopeRef("file:" + Path(self.file_path).name, "file", line, end_line)

    def _node_offset(self, node: ast.AST, *, end: bool) -> int:
        line = (
            getattr(node, "end_lineno", getattr(node, "lineno", 1))
            if end
            else getattr(node, "lineno", 1)
        ) - 1
        column = (
            getattr(node, "end_col_offset", getattr(node, "col_offset", 0))
            if end
            else getattr(node, "col_offset", 0)
        )
        return self.offsets[line] + column


def _line_offsets(source: str) -> list[int]:
    offsets = [0]
    for index, char in enumerate(source):
        if char == "\n":
            offsets.append(index + 1)
    return offsets


def _hash_text(text: str) -> str:
    return hashlib.sha256(text.encode("utf-8")).hexdigest()


def _compare_replacement(node: ast.AST) -> str | None:
    if isinstance(node, ast.Eq):
        return "!="
    if isinstance(node, ast.NotEq):
        return "=="
    if isinstance(node, ast.Gt):
        return ">="
    if isinstance(node, ast.GtE):
        return ">"
    if isinstance(node, ast.Lt):
        return "<="
    if isinstance(node, ast.LtE):
        return "<"
    return None


def _binop_replacement(node: ast.AST) -> str | None:
    if isinstance(node, ast.Add):
        return "-"
    if isinstance(node, ast.Sub):
        return "+"
    if isinstance(node, ast.Mult):
        return "/"
    if isinstance(node, ast.Div):
        return "*"
    return None


def _boolop_replacement(node: ast.AST) -> str | None:
    if isinstance(node, ast.And):
        return "or"
    if isinstance(node, ast.Or):
        return "and"
    return None

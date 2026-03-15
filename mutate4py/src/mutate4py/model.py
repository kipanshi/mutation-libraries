from dataclasses import dataclass, field
from enum import Enum


class CliMode(str, Enum):
    EXPLICIT_FILES = "explicit_files"
    HELP = "help"


@dataclass(frozen=True)
class CliArguments:
    mode: CliMode
    file_args: list[str]
    lines: set[int] = field(default_factory=set)
    scan: bool = False
    update_manifest: bool = False
    reuse_coverage: bool = False
    since_last_run: bool = False
    mutate_all: bool = False
    timeout_factor: int = 10
    mutation_warning: int = 50
    max_workers: int = 1
    test_command: str | None = None
    verbose: bool = False


@dataclass(frozen=True)
class MutationSite:
    file: str
    line: int
    start: int
    end: int
    original_text: str
    replacement_text: str
    description: str
    scope_id: str
    scope_kind: str
    scope_start_line: int
    scope_end_line: int


@dataclass(frozen=True)
class MutationScope:
    id: str
    kind: str
    start_line: int
    end_line: int
    semantic_hash: str


@dataclass(frozen=True)
class SourceAnalysis:
    source: str
    sites: list[MutationSite]
    scopes: list[MutationScope]
    module_hash: str


@dataclass(frozen=True)
class DifferentialManifest:
    version: int
    module_hash: str
    scopes: list[MutationScope]


@dataclass(frozen=True)
class ChangedScopes:
    manifest_present: bool
    module_hash_changed: bool
    unregistered_scope_ids: set[str] = field(default_factory=set)
    manifest_violation_scope_ids: set[str] = field(default_factory=set)

    def all_scope_ids(self) -> set[str]:
        return self.unregistered_scope_ids | self.manifest_violation_scope_ids


@dataclass(frozen=True)
class CoverageReport:
    covered: dict[str, set[int]]

    def covers(self, path: str, line: int) -> bool:
        return line in self.covered.get(path, set())

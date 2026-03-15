from pathlib import Path
import sys
from unittest import TestCase


sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src"))

from mutate4py.cli import CliMode, parse_args


class CliArgumentsParserTest(TestCase):
    def test_rejects_missing_file_argument(self) -> None:
        with self.assertRaisesRegex(
            ValueError, "mutate4py requires exactly one Python file"
        ):
            parse_args([])

    def test_parses_single_explicit_file_argument(self) -> None:
        parsed = parse_args(["demo/sample.py"])

        self.assertEqual(CliMode.EXPLICIT_FILES, parsed.mode)
        self.assertEqual(["demo/sample.py"], parsed.file_args)
        self.assertEqual(set(), parsed.lines)
        self.assertFalse(parsed.scan)
        self.assertFalse(parsed.update_manifest)
        self.assertFalse(parsed.reuse_coverage)
        self.assertFalse(parsed.since_last_run)
        self.assertFalse(parsed.mutate_all)
        self.assertEqual(10, parsed.timeout_factor)
        self.assertEqual(50, parsed.mutation_warning)
        self.assertIsNone(parsed.test_command)
        self.assertFalse(parsed.verbose)

    def test_parses_line_filter_and_timeout_factor(self) -> None:
        parsed = parse_args(
            ["demo/sample.py", "--lines", "12,18", "--timeout-factor", "15"]
        )

        self.assertEqual({12, 18}, parsed.lines)
        self.assertEqual(15, parsed.timeout_factor)

    def test_parses_help_mode(self) -> None:
        parsed = parse_args(["--help"])
        self.assertEqual(CliMode.HELP, parsed.mode)

    def test_rejects_non_python_target(self) -> None:
        with self.assertRaisesRegex(ValueError, "mutate4py target must be a .py file"):
            parse_args(["bogus"])

    def test_rejects_unknown_option(self) -> None:
        with self.assertRaisesRegex(ValueError, "Unknown option: --bogus"):
            parse_args(["demo/sample.py", "--bogus"])

    def test_rejects_lines_combined_with_since_last_run(self) -> None:
        with self.assertRaisesRegex(
            ValueError, "--lines may not be combined with --since-last-run"
        ):
            parse_args(["demo/sample.py", "--lines", "5", "--since-last-run"])

    def test_rejects_blank_test_command(self) -> None:
        with self.assertRaisesRegex(ValueError, "--test-command must not be blank"):
            parse_args(["demo/sample.py", "--test-command", "   "])

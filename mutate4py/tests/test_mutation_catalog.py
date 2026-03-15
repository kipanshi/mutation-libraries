from pathlib import Path
from textwrap import dedent
import sys
from unittest import TestCase


sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src"))

from mutate4py.analysis import MutationCatalog


class MutationCatalogTest(TestCase):
    def test_discovers_boolean_equality_and_comparison_mutations(self) -> None:
        with TemporaryPythonFile(
            dedent(
                """
                def truthy():
                    return True

                def same(left, right):
                    return left == right

                def larger(left, right):
                    return left > right

                def smaller(left, right):
                    return left <= right
                """
            )
        ) as path:
            sites = MutationCatalog().discover([path])

        self.assertEqual(
            [
                "replace True with False",
                "replace == with !=",
                "replace > with >=",
                "replace <= with <",
            ],
            [site.description for site in sites],
        )

    def test_ignores_operators_inside_strings_and_comments(self) -> None:
        with TemporaryPythonFile(
            dedent(
                """
                def sample(left, right):
                    text = "True == False > <"
                    # left == right > 0
                    if left == right:
                        return text
                    return "different"
                """
            )
        ) as path:
            sites = MutationCatalog().discover([path])

        self.assertEqual(["replace == with !="], [site.description for site in sites])

    def test_discovers_arithmetic_logical_unary_and_constant_mutations(self) -> None:
        with TemporaryPythonFile(
            dedent(
                """
                def add(left, right):
                    return left + right

                def divide(left, right):
                    return left / right

                def both(left, right):
                    return left and right

                def invert(value):
                    return not value

                def negative(value):
                    return -value

                def zero():
                    return 0

                def one():
                    return 1
                """
            )
        ) as path:
            sites = MutationCatalog().discover([path])

        self.assertEqual(
            [
                "replace + with -",
                "replace / with *",
                "replace and with or",
                "replace not with removed not",
                "replace - with removed -",
                "replace 0 with 1",
                "replace 1 with 0",
            ],
            [site.description for site in sites],
        )
        self.assertEqual("", sites[3].replacement_text)
        self.assertEqual("", sites[4].replacement_text)

    def test_analyze_returns_scopes_and_module_hash(self) -> None:
        with TemporaryPythonFile(
            dedent(
                """
                def truthy():
                    return True

                def same(left, right):
                    return left == right
                """
            )
        ) as path:
            analysis = MutationCatalog().analyze(path)

        self.assertTrue(analysis.module_hash)
        self.assertTrue(analysis.scopes)
        self.assertEqual(2, len(analysis.sites))


class TemporaryPythonFile:
    def __init__(self, content: str) -> None:
        self._content = content
        self._tmpdir = None
        self.path = None

    def __enter__(self) -> str:
        import tempfile

        self._tmpdir = tempfile.TemporaryDirectory()
        self.path = Path(self._tmpdir.name) / "sample.py"
        self.path.write_text(self._content, encoding="utf-8")
        return str(self.path)

    def __exit__(self, exc_type, exc, tb) -> None:
        if self._tmpdir is not None:
            self._tmpdir.cleanup()

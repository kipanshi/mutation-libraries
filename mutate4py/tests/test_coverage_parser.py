from pathlib import Path
from textwrap import dedent
import sys
from unittest import TestCase


sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src"))

from mutate4py.coverage import parse_coverage_xml


class CoverageParserTest(TestCase):
    def test_parses_covered_lines(self) -> None:
        with TemporaryCoverageFile(
            dedent(
                """
                <coverage>
                  <packages>
                    <package name="demo">
                      <classes>
                        <class name="demo.sample" filename="demo/sample.py">
                          <lines>
                            <line number="5" hits="1"/>
                            <line number="9" hits="0"/>
                          </lines>
                        </class>
                      </classes>
                    </package>
                  </packages>
                </coverage>
                """
            )
        ) as path:
            report = parse_coverage_xml(path)

        self.assertTrue(report.covers("demo/sample.py", 5))
        self.assertFalse(report.covers("demo/sample.py", 9))

    def test_ignores_malformed_numbers(self) -> None:
        with TemporaryCoverageFile(
            dedent(
                """
                <coverage>
                  <packages>
                    <package name="demo">
                      <classes>
                        <class name="demo.sample" filename="demo/sample.py">
                          <lines>
                            <line number="oops" hits="nan"/>
                          </lines>
                        </class>
                      </classes>
                    </package>
                  </packages>
                </coverage>
                """
            )
        ) as path:
            report = parse_coverage_xml(path)

        self.assertFalse(report.covers("demo/sample.py", 0))


class TemporaryCoverageFile:
    def __init__(self, content: str) -> None:
        self._content = content
        self._tmpdir = None
        self.path = None

    def __enter__(self) -> str:
        import tempfile

        self._tmpdir = tempfile.TemporaryDirectory()
        self.path = Path(self._tmpdir.name) / "coverage.xml"
        self.path.write_text(self._content, encoding="utf-8")
        return str(self.path)

    def __exit__(self, exc_type, exc, tb) -> None:
        if self._tmpdir is not None:
            self._tmpdir.cleanup()

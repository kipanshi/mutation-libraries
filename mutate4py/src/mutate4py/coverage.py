from __future__ import annotations

import xml.etree.ElementTree as etree

from mutate4py.model import CoverageReport


def parse_coverage_xml(path: str) -> CoverageReport:
    document = etree.parse(path)
    covered: dict[str, set[int]] = {}
    for class_element in document.findall(".//class"):
        filename = class_element.get("filename")
        if not filename:
            continue
        for line_element in class_element.findall("./lines/line"):
            try:
                line_number = int(line_element.get("number", "0"))
                hits = int(line_element.get("hits", "0"))
            except ValueError:
                continue
            if hits <= 0:
                continue
            covered.setdefault(filename, set()).add(line_number)
    return CoverageReport(covered=covered)

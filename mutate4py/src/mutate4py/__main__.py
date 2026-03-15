from __future__ import annotations

import os
import sys

from mutate4py.app import Application


def main(argv: list[str] | None = None) -> int:
    arguments = sys.argv[1:] if argv is None else argv
    app = Application(os.getcwd(), sys.stdout, sys.stderr)
    return app.execute(arguments)


if __name__ == "__main__":
    raise SystemExit(main())

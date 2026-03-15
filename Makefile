SHELL := /bin/bash

.PHONY: help test test-go test-py test-rs self-check self-check-go self-check-py self-check-rs package package-go package-py package-rs release release-go release-py release-rs release-check clean

define package_if_changed
	@if git rev-parse --is-inside-work-tree >/dev/null 2>&1 && git rev-parse --verify HEAD >/dev/null 2>&1; then \
		if git diff --quiet HEAD -- $(1) && git diff --cached --quiet -- $(1) && [ -z "$$(git ls-files --others --exclude-standard -- $(1))" ]; then \
			printf "No changes in $(1); skipping release packaging\n"; \
		else \
			$(2); \
		fi; \
	elif git rev-parse --is-inside-work-tree >/dev/null 2>&1; then \
		printf "Git repo has no commits yet; packaging $(1)\n"; \
		$(2); \
	else \
		printf "No git repo detected; packaging $(1)\n"; \
		$(2); \
	fi
endef

help:
	@printf "Targets:\n"
	@printf "  make test          Run all package test suites\n"
	@printf "  make self-check    Run all three ports on themselves\n"
	@printf "  make package       Build local release artifacts\n"
	@printf "  make release       Package only changed libraries when in git\n"
	@printf "  make release-check Run tests, self-checks, and packaging checks\n"
	@printf "  make package-go    Build only the Go artifact\n"
	@printf "  make package-py    Build only the Python artifacts\n"
	@printf "  make package-rs    Build only the Rust package\n"
	@printf "  make clean         Remove generated artifacts\n"

test: test-go test-py test-rs

test-go:
	cd mutate4go && go test ./...

test-py:
	cd mutate4py && python3 -m unittest discover -s tests -p 'test_*.py'

test-rs:
	cd mutate4rs && cargo test

self-check: self-check-go self-check-py self-check-rs

self-check-go:
	mkdir -p dist/self-check
	cd mutate4go && go build -o ../dist/self-check/mutate4go-self ./cmd/mutate4go && ../dist/self-check/mutate4go-self cli.go --lines 33 > ../dist/self-check/mutate4go.txt

self-check-py:
	mkdir -p dist/self-check
	cd mutate4py && PYTHONPATH=src python3 -m mutate4py src/mutate4py/cli.py --lines 34 > ../dist/self-check/mutate4py.txt

self-check-rs:
	mkdir -p dist/self-check
	cd mutate4rs && cargo build --quiet && target/debug/mutate4rs src/cli.rs --lines 53 > ../dist/self-check/mutate4rs.txt

package: package-go package-py package-rs

package-go:
	mkdir -p dist/mutate4go
	cd mutate4go && go build -o ../dist/mutate4go/mutate4go ./cmd/mutate4go

package-py:
	mkdir -p dist/mutate4py
	cd mutate4py && python3 -m build --sdist --wheel --outdir ../dist/mutate4py

package-rs:
	mkdir -p dist/mutate4rs
	cd mutate4rs && cargo package --allow-dirty && cp target/package/*.crate ../dist/mutate4rs/

release: release-go release-py release-rs

release-go:
	$(call package_if_changed,mutate4go,mkdir -p dist/mutate4go && cd mutate4go && go build -o ../dist/mutate4go/mutate4go ./cmd/mutate4go)

release-py:
	$(call package_if_changed,mutate4py,mkdir -p dist/mutate4py && cd mutate4py && python3 -m build --sdist --wheel --outdir ../dist/mutate4py)

release-rs:
	$(call package_if_changed,mutate4rs,mkdir -p dist/mutate4rs && cd mutate4rs && cargo package --allow-dirty && cp target/package/*.crate ../dist/mutate4rs/)

release-check: test self-check package

clean:
	rm -rf dist
	rm -rf mutate4py/build
	rm -rf mutate4py/*.egg-info
	rm -rf mutate4rs/target
	find mutate4py -type d -name __pycache__ -prune -exec rm -rf {} +

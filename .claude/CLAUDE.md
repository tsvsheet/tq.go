# tq.go

The standalone `tq` binary (module `github.com/tsvsheet/tq.go`, class CLI: `go.mod` + `cmd/tq/`): a thin urfave/cli/v3 frontend over [go-tq](https://github.com/tsvsheet/go-tq), the same thin-client shape as [tsvsheet.go](https://github.com/tsvsheet/tsvsheet.go) over go-tsvsheet. The normative language definition lives in [tsvsheet/tq](https://github.com/tsvsheet/tq) (SPECIFICATION.md + grammars); the capability spec and API contract live in `tsvsheet/_projects/specs/tq/`.

## Layout

- [cmd/tq](../cmd/tq/) — the entry point: version ldflags, `os.Exit` indirection, nothing else.
- [internal/cli](../internal/cli/) — the command tier: the root command runs `tq '<query>' [file]` (file omitted or `-` reads stdin, TSV out on stdout); `completion <shell>` emits bash/zsh/fish scripts via urfave/cli's built-in (renamed) templates. Command logic lives in stream-injected functions (`Streams`, `positional`) so it is fully testable; the `cli.Command` wrappers only bind flags and streams.
- [internal/constants](../internal/constants/) — this repo's own `errs.Const` sentinels (argument/flag handling only).

## Rules

- **No tq semantics in this repo.** Every behavior beyond argument handling is a go-tq call: `Parse`, `ReadTable`, `Program.Run`, `WriteTable`. Flags map 1:1 onto `tq.Options` (`--no-header`→`IsHeaderless`, `--strict`→`IsStrict`, `--raw`→`IsRaw`, `--max-cells`→`MaxCells`, `--at`→`At`, defaulting to process start).
- Exit codes: 0 success; 1 program-vs-data error (`ErrUnknownColumn`, `ErrCellRef`, `ErrHeaderless`, `ErrStrict`, `ErrLimit`) with a one-line stderr diagnostic; 2 query syntax error (`ErrSyntax`, with position) — matched with `errors.Is` against go-tq's sentinels, never strings.
- Sentinels are `errs.Const`; never `fmt.Errorf`/`errors.New`. Value receivers only; named parameter types; gocognit ≤ 7; 100.0% aggregate coverage. `make check` must exit 0 (`make tools` first to populate `${GOBIN}`).
- Shared `Makefile`, `.golangci.yaml`, `.editorconfig`, `.gitignore`, `.github/` are distributed by `nicerobot/tools.repository` — never edit in-tree; repo-local gate scoping goes in `Makefile.local`.

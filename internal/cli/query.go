package cli

import (
	tq "github.com/tsvsheet/go-tq"

	"github.com/tsvsheet/tq.go/internal/constants"
)

// runQuery parses the query, reads the table (file or stdin), runs the
// program over it, and writes the resulting table as TSV to the output
// stream. Every tq semantic — parsing, planning, verb execution, compute-first
// — is a go-tq call; errors pass through as go-tq's sentinels so the exit-code
// mapping can errors.Is them.
func runQuery(streams Streams, args positional, opts tq.Options) error {
	if len(args) > 2 {
		return constants.ErrTooManyArguments.With(nil, "unexpected", args.text(2))
	}
	query := args.text(0)
	if query == "" {
		return constants.ErrMissingArgument.With(nil, "argument", "query")
	}
	program, err := tq.Parse(tq.Query(query))
	if err != nil {
		return err
	}
	reader, release, err := args.at(1).open(streams.In)
	if err != nil {
		return err
	}
	defer func() { _ = release() }()

	table, err := tq.ReadTable(reader, opts)
	if err != nil {
		return err
	}
	result, err := program.Run(table, opts)
	if err != nil {
		return err
	}
	return tq.WriteTable(streams.Out, result)
}

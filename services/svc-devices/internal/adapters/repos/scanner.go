//go:generate go tool github.com/maxbrunsfeld/counterfeiter/v6 -generate

package repos

import (
	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/jackc/pgx/v5"
)

//counterfeiter:generate -o ../../mocks/scanner.go . Scanner

type (
	// Scanner abstracts row scanning operations for testability
	// and architectural consistency with PoolOps.
	Scanner interface {
		ScanAll(dst any, rows pgx.Rows) error
		ScanOne(dst any, rows pgx.Rows) error
		IsNotFound(err error) bool
	}

	// PgxScanner implements Scanner using pgxscan.
	PgxScanner struct{}
)

// NewPgxScanner creates a new PgxScanner instance.
func NewPgxScanner() *PgxScanner {
	return &PgxScanner{}
}

// ScanAll scans all rows into the destination slice.
func (s *PgxScanner) ScanAll(dst any, rows pgx.Rows) error {
	return pgxscan.ScanAll(dst, rows)
}

// ScanOne scans a single row into the destination.
func (s *PgxScanner) ScanOne(dst any, rows pgx.Rows) error {
	return pgxscan.ScanOne(dst, rows)
}

// IsNotFound checks if the error indicates no rows were found.
func (s *PgxScanner) IsNotFound(err error) bool {
	return pgxscan.NotFound(err)
}

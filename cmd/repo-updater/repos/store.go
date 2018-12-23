package repos

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"strings"

	"github.com/keegancsmith/sqlf"
	"github.com/pkg/errors"
)

// A Store exposes methods to read and write persistent repositories.
type Store interface {
	ListRepos(ctx context.Context) ([]*Repo, error)
	UpsertRepos(ctx context.Context, repos ...*Repo) error
}

// DBStore implements the Store interface for reading and writing repos directly
// from the Postgres database.
type DBStore struct {
	db        *sql.DB
	listRepos *sql.Stmt
}

// NewDBStore instantiates and returns a new DBStore with prepared statements.
func NewDBStore(ctx context.Context, db *sql.DB) (*DBStore, error) {
	store := DBStore{db: db}
	return &store, store.prepare(ctx)
}

// ListRepos lists all configured repositories in Sourcegraph.
func (s DBStore) ListRepos(ctx context.Context) (repos []*Repo, err error) {
	var cursor, next int64 = -1, 0
	for cursor != next && err == nil {
		cursor = next
		if err = s.listReposPage(ctx, cursor, 500, &repos); len(repos) > 0 {
			next = int64(repos[len(repos)-1]._ID)
		}
	}
	return repos, err
}

func (s DBStore) listReposPage(ctx context.Context, cursor, limit int64, repos *[]*Repo) (err error) {
	rows, err := s.listRepos.QueryContext(ctx, cursor, limit)
	if err != nil {
		return err
	}

	return scanAll(rows, func(sc scanner) error {
		var r Repo
		if err = scanRepo(&r, sc); err != nil {
			return err
		}
		*repos = append(*repos, &r)
		return nil
	})
}

const listReposSQL = `
SELECT id, name, description, language, created_at, updated_at, deleted_at,
  external_id, external_service_type, external_service_id, enabled, archived, fork
FROM repo WHERE id > $1 ORDER BY id ASC LIMIT $2
`

// UpsertRepos updates or inserts the given repos in the Sourcegraph repository store.
// The _ID field of each given Repo is set on inserts.
func (s *DBStore) UpsertRepos(ctx context.Context, repos ...*Repo) error {
	q := upsertReposQuery(repos)
	rows, err := s.db.QueryContext(ctx, q.Query(sqlf.PostgresBindVar), q.Args()...)
	if err != nil {
		return err
	}

	i := -1
	return scanAll(rows, func(sc scanner) error {
		i++
		return scanRepo(repos[i], sc)
	})
}

func (s *DBStore) prepare(ctx context.Context) error {
	for _, st := range []struct {
		stmt  **sql.Stmt
		query string
	}{
		{&s.listRepos, listReposSQL},
	} {
		stmt, err := s.db.PrepareContext(ctx, st.query)
		if err != nil {
			return errors.Wrapf(err, "failed to prepare: %s", st.query)
		}
		*st.stmt = stmt
	}

	return nil
}

func upsertReposQuery(repos []*Repo) *sqlf.Query {
	values := make([]*sqlf.Query, 0, len(repos))
	for _, r := range repos {
		values = append(values, sqlf.Sprintf(
			upsertRepoValuesFmtstr,
			upsertRepoColumns(r)...,
		))
	}
	return sqlf.Sprintf(upsertReposQueryFmtstr, sqlf.Join(values, ",\n"))
}

var upsertReposQueryFmtstr = strings.TrimSpace(fmt.Sprintf(`
INSERT INTO repo
(%s)
VALUES
%%s
ON CONFLICT ON CONSTRAINT repo_external_service_unique
DO UPDATE SET
  name        = excluded.name,
  description = excluded.description,
  language    = excluded.language,
  updated_at  = GREATEST(repo.updated_at, excluded.updated_at),
  deleted_at  = GREATEST(repo.deleted_at, excluded.deleted_at),
  archived    = excluded.archived,
  fork        = excluded.fork
RETURNING id, name, description, language, created_at, updated_at,
  deleted_at, external_id, external_service_type, external_service_id,
  enabled, archived, fork
`, strings.Join(upsertRepoColumnNames, ", ")))

var upsertRepoColumnNames = []string{
	"name",
	"description",
	"language",
	"uri",
	"created_at",
	"updated_at",
	"deleted_at",
	"external_id",
	"external_service_type",
	"external_service_id",
	"enabled",
	"archived",
	"fork",
}

var upsertRepoValuesFmtstr = "(" + strings.TrimSuffix(strings.Repeat("%s, ", len(upsertRepoColumnNames)), ", ") + ")"

func upsertRepoColumns(r *Repo) []interface{} {
	return []interface{}{
		r.Name,
		r.Description,
		r.Language,
		"", // URI
		r.CreatedAt.UTC(),
		r.UpdatedAt.UTC(),
		r.DeletedAt.UTC(),
		r.ExternalRepo.ID,
		r.ExternalRepo.ServiceType,
		r.ExternalRepo.ServiceID,
		r.Enabled,
		r.Archived,
		r.Fork,
	}
}

// scanner captures the Scan method of sql.Rows and sql.Row
type scanner interface {
	Scan(dst ...interface{}) error
}

func scanAll(rows *sql.Rows, scan func(scanner) error) (err error) {
	defer closeErr(rows, &err)

	for rows.Next() {
		if err := scan(rows); err != nil {
			return err
		}
	}

	return rows.Err()
}

func closeErr(c io.Closer, err *error) {
	if e := c.Close(); err != nil && *err == nil {
		*err = e
	}
}

func scanRepo(r *Repo, s scanner) error {
	return s.Scan(
		&r._ID,
		&r.Name,
		&r.Description,
		&r.Language,
		&r.CreatedAt,
		&nullTime{&r.UpdatedAt},
		&nullTime{&r.DeletedAt},
		&r.ExternalRepo.ID,
		&r.ExternalRepo.ServiceType,
		&r.ExternalRepo.ServiceID,
		&r.Enabled,
		&r.Archived,
		&r.Fork,
	)
}

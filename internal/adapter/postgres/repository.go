package postgres

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go_hw/internal/domain"
)

//go:embed migrations/001_movies.sql
var migration string

type Repository struct{ pool *pgxpool.Pool }

func New(ctx context.Context, dsn string) (*Repository, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("database configuration: %w", err)
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database connection: %w", err)
	}
	return &Repository{pool: pool}, nil
}
func (r *Repository) Close() { r.pool.Close() }

func (r *Repository) Migrate(ctx context.Context) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(75413951)"); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "CREATE TABLE IF NOT EXISTS schema_migrations (version BIGINT PRIMARY KEY)"); err != nil {
		return err
	}
	var applied bool
	if err = tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version=1)").Scan(&applied); err != nil {
		return err
	}
	if !applied {
		if _, err = tx.Exec(ctx, migration); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, "INSERT INTO schema_migrations(version) VALUES(1)"); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
func scan(row pgx.Row) (domain.Movie, error) {
	var m domain.Movie
	err := row.Scan(&m.ID, &m.Title, &m.Year, &m.Director)
	if errors.Is(err, pgx.ErrNoRows) {
		return m, &domain.Error{Code: "not_found", Message: "movie not found"}
	}
	return m, err
}
func (r *Repository) GetMovie(ctx context.Context, id int64) (domain.Movie, error) {
	return scan(r.pool.QueryRow(ctx, "SELECT id,title,year,director FROM movies WHERE id=$1", id))
}
func (r *Repository) GetLastMovie(ctx context.Context) (domain.Movie, error) {
	return scan(r.pool.QueryRow(ctx, "SELECT id,title,year,director FROM movies ORDER BY id DESC LIMIT 1"))
}
func (r *Repository) ExistingIDs(ctx context.Context) (map[int64]bool, error) {
	rows, err := r.pool.Query(ctx, "SELECT id FROM movies")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make(map[int64]bool)
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		ids[id] = true
	}
	return ids, rows.Err()
}
func (r *Repository) Save(ctx context.Context, m domain.Movie) error {
	_, err := r.pool.Exec(ctx, "INSERT INTO movies(id,title,year,director) VALUES($1,$2,$3,$4) ON CONFLICT(id) DO NOTHING", m.ID, m.Title, m.Year, m.Director)
	return err
}

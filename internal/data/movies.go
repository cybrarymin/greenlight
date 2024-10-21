package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/uptrace/bun"
)

var (
	ErrInvalidRuntimeFormat = errors.New("invalid runtime format")
	ErrorRecordNotFound     = errors.New("record not found")
)

type Movie struct {
	bun.BaseModel `bun:"table:movies"`
	ID            int64     `json:"id" bun:",pk,autoincrement,notnull,type:bigserial"`                                              // ID is the identifier of the movie
	CreatedAt     time.Time `json:"-" bun:"created_at,notnull,nullzero,default:current_timestamp,type:timestamp(0) with time zone"` // timestamp when movies is added to the database
	Title         string    `json:"title" bun:",notnull"`                                                                           // Movie title
	Year          int32     `json:"year,omitempty" bun:",notnull"`                                                                  // Year movie created
	Runtime       Runtime   `json:"runtime,omitempty" bun:",notnull"`                                                               // Movie runtime in minutes
	Genres        []string  `json:"genres,omitempty" bun:"genres,array,notnull"`                                                    // Genres of the movie
	Version       int32     `json:"version" bun:",notnull,default:1"`                                                               // Version number will be increased each time the movies is updated
}

type MovieModel struct {
	db *bun.DB
}

func (m *MovieModel) Insert(ctx context.Context, movie *Movie) error {
	args := []interface{}{&movie.ID, &movie.CreatedAt, &movie.Version}
	err := m.db.NewInsert().Model(movie).Returning("id, created_at, version").Scan(ctx, args...)
	if err != nil {
		return err
	}
	return nil
}
func (m *MovieModel) Delete(ctx context.Context, id int64) error {
	if id < 1 {
		return ErrorRecordNotFound
	}
	result, err := m.db.NewDelete().Model((*Movie)(nil)).Where("id = ?", id).Exec(ctx)
	if ok, _ := result.RowsAffected(); ok == 0 {
		return ErrorRecordNotFound
	}
	if err != nil {
		return err
	}
	return nil
}
func (m *MovieModel) Update(ctx context.Context, id int64, movie *Movie) error {
	args := []interface{}{&movie.CreatedAt, &movie.Version}
	err := m.db.NewUpdate().Model(movie).Set("version = version + 1").Where("id = ?", id).Returning("created_at, version").Scan(ctx, args...)
	if err != nil {
		return err
	}
	return nil
}
func (m *MovieModel) Select(ctx context.Context, id int64) (*Movie, error) {
	nMovie := Movie{}
	if id < 1 {
		return nil, ErrorRecordNotFound
	}
	err := m.db.NewSelect().Model((*Movie)(nil)).Where("id = ?", id).Scan(ctx, &nMovie)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrorRecordNotFound
		default:
			return nil, err
		}
	}
	return &nMovie, nil
}

type Runtime int32

func (r Runtime) MarshalJSON() ([]byte, error) {
	runtime := fmt.Sprintf("%d mins", r)
	runtime = strconv.Quote(runtime)
	return []byte(runtime), nil
}

func (r *Runtime) UnmarshalJSON(jsonValue []byte) error {

	jsonValueUnquoted, err := strconv.Unquote(string(jsonValue))
	if err != nil {
		return ErrInvalidRuntimeFormat
	}
	parts := strings.Split(jsonValueUnquoted, " ")
	if len(parts) != 2 || parts[1] != "mins" {
		return ErrInvalidRuntimeFormat
	}
	i, err := strconv.ParseInt(parts[0], 10, 32)
	if err != nil {
		return ErrInvalidRuntimeFormat
	}
	*r = Runtime(i)
	return nil
}

func (m Movie) Validator(nValidator *Validator) {
	nValidator.Check(m.Title != "", "title", "must be provided")
	nValidator.Check(len(m.Title) <= 500, "title", "must be less than 500 bytes long")
	nValidator.Check(m.Year != 0, "year", "year should be specified")
	nValidator.Check(m.Year >= 1888, "year", "year must be after 1888")
	nValidator.Check(m.Year < int32(time.Now().Year()), "year", "year must be in future")
	nValidator.Check(m.Runtime != 0, "runtime", "runtime should be specified")
	nValidator.Check(m.Runtime > 0, "runtime", "runtime should be a positive integer")
	nValidator.Check(m.Genres != nil, "genres", "genres should be specified")
	nValidator.Check(len(m.Genres) >= 1, "genres", "genres must at least have one element")
	nValidator.Check(len(m.Genres) <= 5, "genres", "must not contain more than 5 genres")
	nValidator.Check(Unique(m.Genres), "genres", "duplicate value in genres")
}

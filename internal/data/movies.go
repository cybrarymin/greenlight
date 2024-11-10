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
	"github.com/uptrace/bun/dialect/pgdialect"
)

var (
	ErrInvalidRuntimeFormat = errors.New("invalid runtime format")
	ErrorRecordNotFound     = errors.New("record not found")
	ErrEditConflict         = errors.New("edit conflict")
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
	// define the timeouts context exactly before the process that needs that context to make sure only that specific process uses the countdown
	timeoutCtx, cancelFunc := context.WithTimeout(ctx, time.Second*5)
	defer cancelFunc()
	err := m.db.NewInsert().Model(movie).Returning("id, created_at, version").Scan(timeoutCtx, args...)
	if err != nil {
		return err
	}
	return nil
}

func (m *MovieModel) Delete(ctx context.Context, id int64) error {
	if id < 1 {
		return ErrorRecordNotFound
	}
	// define the timeouts context exactly before the process that needs that context to make sure only that specific process uses the countdown
	timeoutCtx, cancelFunc := context.WithTimeout(ctx, time.Second*5)
	defer cancelFunc()
	result, err := m.db.NewDelete().Model((*Movie)(nil)).Where("id = ?", id).Exec(timeoutCtx)
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
	movie.Version += 1
	timeoutCtx, cancelFunc := context.WithTimeout(ctx, time.Second*5)
	defer cancelFunc()
	err := m.db.NewUpdate().Model(movie).Where("id = ?", id).Where("version = ?", movie.Version).Returning("created_at, version").Scan(timeoutCtx, args...)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrEditConflict
		default:
			return err
		}
	}
	return nil
}

func (m *MovieModel) Select(ctx context.Context, id int64) (*Movie, error) {
	nMovie := Movie{}
	if id < 1 {
		return nil, ErrorRecordNotFound
	}
	timeoutCtx, cancelFunc := context.WithTimeout(ctx, time.Second*5)
	defer cancelFunc()
	err := m.db.NewSelect().Model((*Movie)(nil)).Where("id = ?", id).Scan(timeoutCtx, &nMovie)
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

func (m *MovieModel) List(ctx context.Context, title string, genres []string, filters *Filters) ([]Movie, int, error) {
	args := []struct {
		Count int
		Movie
	}{}
	nMovies := []Movie{}

	timeoutCtx, cancelFunc := context.WithTimeout(ctx, time.Second*5)
	defer cancelFunc()

	orderQuery := filters.SortColumn() + " " + filters.SortDirection()
	err := m.db.NewSelect().Model((*Movie)(nil)).ColumnExpr("COUNT(*) OVER(),*").Where("(title_tsvector @@ to_tsquery('simple',?)) OR (? = '')", title, title).Where("(genres @> ? OR ? = '{}')", pgdialect.Array(genres), pgdialect.Array(genres)).OrderExpr(orderQuery).Limit(filters.limit()).Offset(filters.offset()).Scan(timeoutCtx, &args)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, 0, ErrorRecordNotFound
		default:
			return nil, 0, err
		}
	}
	for _, v := range args {
		nMovies = append(nMovies, v.Movie)
	}
	return nMovies, args[0].Count, nil
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

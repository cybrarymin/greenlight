package api

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/cybrarymin/greenlight/internal/data"
	"github.com/rs/zerolog"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
)

const version = "1.0.0"

var (
	ListenPort           int
	Env                  string
	DBDSN                string
	DBMaxConnCount       int
	DBMaxIdleConnCount   int
	DBMaxIdleConnTimeout time.Duration
)

type config struct {
	port int
	env  string
	db   struct {
		dbDsn                string
		dbMaxConnCount       int
		DBMaxIdleConnCount   int
		DBMaxIdleConnTimeout time.Duration
	}
}

type application struct {
	config config
	log    *zerolog.Logger
	models *data.Models
}

func Api() {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	cfg := config{
		port: ListenPort,
		env:  Env,
		db: struct {
			dbDsn                string
			dbMaxConnCount       int
			DBMaxIdleConnCount   int
			DBMaxIdleConnTimeout time.Duration
		}{
			DBDSN,
			DBMaxConnCount,
			DBMaxIdleConnCount,
			DBMaxIdleConnTimeout,
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	db, err := openDB(ctx, &cfg)
	if err != nil {
		logger.Fatal().Err(err)
	}
	defer db.Close()

	app := &application{
		config: cfg,
		log:    &logger,
		models: data.NewModels(db),
	}

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.port),
		Handler:      app.routes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	app.log.Info().Msg("starting the http server .....")
	err = srv.ListenAndServe()
	if err != nil {
		app.log.Error().Err(err)
	}

}

func openDB(ctx context.Context, cfg *config) (*bun.DB, error) {
	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(cfg.db.dbDsn)))
	db := bun.NewDB(sqldb, pgdialect.New(), bun.WithDiscardUnknownColumns())
	db.SetMaxOpenConns(DBMaxConnCount)
	db.SetMaxIdleConns(DBMaxIdleConnCount)
	db.SetConnMaxIdleTime(DBMaxIdleConnTimeout)
	err := db.PingContext(ctx)
	if err != nil {
		return nil, err
	}
	return db, nil
}

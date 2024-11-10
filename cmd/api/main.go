package api

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/cybrarymin/greenlight/internal/data"
	"github.com/rs/zerolog"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"github.com/uptrace/bun/extra/bunzerolog"
)

const version = "1.0.0"

var (
	ListenPort           int
	Env                  string
	DBDSN                string
	DBMaxConnCount       int
	DBMaxIdleConnCount   int
	DBMaxIdleConnTimeout time.Duration
	LogLevel             int8
	DBLogs               bool
)

type config struct {
	port int
	env  string
	db   struct {
		dbDsn                string
		dbMaxConnCount       int
		DBMaxIdleConnCount   int
		DBMaxIdleConnTimeout time.Duration
		DBLogs               bool
	}
}

type application struct {
	config config
	log    *zerolog.Logger
	models *data.Models
}

func Api() {
	var logger zerolog.Logger
	if zerolog.Level(LogLevel).String() == zerolog.LevelTraceValue {
		logger = zerolog.New(os.Stdout).With().Stack().Timestamp().Logger().Level(zerolog.Level(LogLevel))
	} else {
		logger = zerolog.New(os.Stdout).With().Timestamp().Logger().Level(zerolog.Level(LogLevel))
	}

	cfg := config{
		port: ListenPort,
		env:  Env,
		db: struct {
			dbDsn                string
			dbMaxConnCount       int
			DBMaxIdleConnCount   int
			DBMaxIdleConnTimeout time.Duration
			DBLogs               bool
		}{
			DBDSN,
			DBMaxConnCount,
			DBMaxIdleConnCount,
			DBMaxIdleConnTimeout,
			DBLogs,
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	db, err := openDB(ctx, &cfg)
	if err != nil {
		logger.Fatal().Err(err)
	}
	defer db.Close()

	if cfg.db.DBLogs {
		db.AddQueryHook(bunzerolog.NewQueryHook(
			bunzerolog.WithLogger(&logger),
			bunzerolog.WithQueryLogLevel(zerolog.DebugLevel),      // Show database interaction logs by debug tag
			bunzerolog.WithSlowQueryLogLevel(zerolog.WarnLevel),   // Show database slow queries as warnings tag
			bunzerolog.WithErrorQueryLogLevel(zerolog.ErrorLevel), // Show database slow queries as error tag
			bunzerolog.WithSlowQueryThreshold(3*time.Second),
		))
	}

	app := &application{
		config: cfg,
		log:    &logger,
		models: data.NewModels(db),
	}

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.port),
		Handler:      app.routes(),
		IdleTimeout:  time.Minute,
		ErrorLog:     log.New(logger, "", 0),
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

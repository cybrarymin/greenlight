package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/cybrarymin/greenlight/internal/data"
	mailer "github.com/cybrarymin/greenlight/internal/mailter"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"github.com/uptrace/bun/extra/bunzerolog"
	"github.com/uptrace/opentelemetry-go-extra/otelsql"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
)

var Version = "local"
var BuildTime string

var (
	ListenPort           int
	Env                  string
	DBDSN                string
	DBMaxConnCount       int
	DBMaxIdleConnCount   int
	DBMaxIdleConnTimeout time.Duration
	LogLevel             int8
	DBLogs               bool
	GlobalRateLimit      int64
	PerClientRateLimit   int64
	EnableRateLimit      bool
	SMTPServer           string
	SMTPPort             int
	SMTPUserName         string
	SMTPPassword         string
	EmailSender          string
	VersionDisplay       bool
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
	rateLimit struct {
		globalRateLimit    int64
		perClientRateLimit int64
		enabled            bool
	}
	smtp struct {
		SMTPServer   string
		SMTPPort     int
		SMTPUserName string
		SMTPPassword string
		EmailSender  string
	}
}

type application struct {
	config config
	log    *zerolog.Logger
	models *data.Models
	mailer *mailer.Mailer
	wg     sync.WaitGroup
}

func Api() {
	var logger zerolog.Logger
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
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
		rateLimit: struct {
			globalRateLimit    int64
			perClientRateLimit int64
			enabled            bool
		}{
			globalRateLimit:    GlobalRateLimit,
			perClientRateLimit: PerClientRateLimit,
			enabled:            EnableRateLimit,
		},
		smtp: struct {
			SMTPServer   string
			SMTPPort     int
			SMTPUserName string
			SMTPPassword string
			EmailSender  string
		}{
			SMTPServer:   SMTPServer,
			SMTPPort:     SMTPPort,
			SMTPUserName: SMTPUserName,
			SMTPPassword: SMTPPassword,
			EmailSender:  EmailSender,
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
		mailer: mailer.New(cfg.smtp.SMTPServer, cfg.smtp.SMTPPort, cfg.smtp.SMTPUserName, cfg.smtp.SMTPPassword, "greenlight <no-reply@greenlight.net>"), // TODO: Flags should be provided for the input arguments
		wg:     sync.WaitGroup{},
	}

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.port),
		Handler:      app.routes(),
		IdleTimeout:  time.Minute,
		ErrorLog:     log.New(logger, "", 0),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	promInit(db)
	otelShutdown, err := setupOTelSDK(ctx)
	if err != nil {
		app.log.Error().Err(err)
	}

	shutdownErr := make(chan error)
	go app.gracefulShutdown(srv, shutdownErr, otelShutdown)

	app.log.Info().Msg("starting the http server .....")
	err = srv.ListenAndServe()
	if err != nil {
		app.log.Error().Err(err)
	}

	err = <-shutdownErr // This channel will block main appliction not to finish until shutdown method return it's errors.
	if err != nil {
		app.log.Error().Err(err)
	}
}

func openDB(ctx context.Context, cfg *config) (*bun.DB, error) {
	sqldb := otelsql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(cfg.db.dbDsn)),
		otelsql.WithAttributes(semconv.DBSystemPostgreSQL),
	)
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

func (app *application) gracefulShutdown(srv *http.Server, shutdownErr chan error, shutdown func(context.Context) error) {

	// Create a channel to redirect signal to it.
	quit := make(chan os.Signal, 1)

	// This will listen to signals specified and will relay to them to the channel specified.
	// This will impede program to exit by the signal
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	s := <-quit
	// Log that the signal has been catched.
	app.log.Info().Msgf("catched signal %s", s.String())

	// Shutdown method is waiting for all the requests to be processed and gracefully shuts down the http server without interrupting any active connection.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
	defer cancel()
	err := srv.Shutdown(ctx) // Shutdown here will block unitl it shutdown everything. we use channel to read in the main function
	if err != nil {
		shutdownErr <- err
	}

	// Exit the application with success status code
	app.log.Info().Msg("waiting for background tasks to finish")
	app.wg.Wait()
	shutdownErr <- nil

	app.log.Info().Msg("stopped server")
}

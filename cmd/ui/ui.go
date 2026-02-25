package ui

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/inngest/inngest/pkg/authn"
	"github.com/inngest/inngest/pkg/config"
	"github.com/inngest/inngest/pkg/coreapi"
	"github.com/inngest/inngest/pkg/cqrs/base_cqrs"
	sqlc_postgres "github.com/inngest/inngest/pkg/cqrs/base_cqrs/sqlc/postgres"
	"github.com/inngest/inngest/pkg/devserver"
	"github.com/inngest/inngest/pkg/headers"
	"github.com/inngest/inngest/pkg/history_drivers/memory_reader"
	"github.com/inngest/inngest/pkg/logger"
	"github.com/urfave/cli/v3"
)

func action(ctx context.Context, cmd *cli.Command) error {
	conf, err := config.Dev(ctx)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	portStr := cmd.String("port")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		fmt.Printf("Error: invalid port %q: %v\n", portStr, err)
		os.Exit(1)
	}
	conf.EventAPI.Port = port
	conf.CoreAPI.Port = port

	host := cmd.String("host")
	if host != "" {
		conf.EventAPI.Addr = host
		conf.CoreAPI.Addr = host
	}

	// PostgreSQL is required for UI-only mode
	postgresURI := cmd.String("postgres-uri")
	if postgresURI == "" {
		postgresURI = os.Getenv("INNGEST_POSTGRES_URI")
	}
	if postgresURI == "" {
		fmt.Println("Error: --postgres-uri is required for UI-only mode")
		os.Exit(1)
	}

	maxIdleConns := cmd.Int("postgres-max-idle-conns")
	maxOpenConns := cmd.Int("postgres-max-open-conns")
	connMaxIdleTime := cmd.Int("postgres-conn-max-idle-time")
	connMaxLifetime := cmd.Int("postgres-conn-max-lifetime")

	// Optional signing key for API auth
	var signingKey *string
	sk := cmd.String("signing-key")
	if sk == "" {
		sk = os.Getenv("INNGEST_SIGNING_KEY")
	}
	if sk != "" {
		signingKey = &sk
	}

	l := logger.StdlibLogger(ctx)
	l.Info("starting UI-only server",
		"port", port,
		"host", host,
	)

	// Connect to PostgreSQL
	db, err := base_cqrs.New(base_cqrs.BaseCQRSOptions{
		Persist:     true,
		PostgresURI: postgresURI,
	})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	poolOpts := sqlc_postgres.NewNormalizedOpts{
		MaxIdleConns:    maxIdleConns,
		MaxOpenConns:    maxOpenConns,
		ConnMaxIdle:     connMaxIdleTime,
		ConnMaxLifetime: connMaxLifetime,
	}
	dbcqrs := base_cqrs.NewCQRS(db, "postgres", poolOpts)

	// Use memory reader as the history reader. The v2 resolvers (which the
	// current UI uses) read run/trace data from the database via the CQRS
	// manager, so the memory reader being empty doesn't affect the main UI
	// functionality. Only legacy v1 history endpoints will return empty.
	historyReader := memory_reader.NewReader()

	// Build the Core API (GraphQL)
	core, err := coreapi.NewCoreApi(coreapi.Options{
		AuthMiddleware: authn.SigningKeyMiddleware(signingKey),
		Data:           dbcqrs,
		Config:         *conf,
		Logger:         l,
		HistoryReader:  historyReader,
		// Runner, Queue, EventHandler, Executor are intentionally nil.
		// Mutations that require them (invoke, cancel, rerun) will return
		// errors, but all read queries work via the CQRS database layer.
	})
	if err != nil {
		return fmt.Errorf("failed to create core API: %w", err)
	}

	// Build the main router
	router := chi.NewMux()
	router.Use(middleware.Recoverer)
	router.Use(middleware.RealIP)
	router.Use(cors.Handler(cors.Options{
		AllowOriginFunc:  func(r *http.Request, origin string) bool { return true },
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: false,
	}))
	router.Use(headers.StaticHeadersMiddleware(conf.GetServerKind()))

	// Health check
	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","mode":"ui-only"}`))
	})

	// Mount Core API at /v0 (GraphQL + run endpoints)
	router.Mount("/v0", core.Router)

	// Mount pprof for debugging
	router.Mount("/debug", middleware.Profiler())

	// Serve the embedded UI (SPA with fallback routing).
	// The UI assets are embedded in the devserver package.
	mountUI(router)

	// Start the server
	addr := fmt.Sprintf("%s:%d", conf.CoreAPI.Addr, conf.CoreAPI.Port)
	l.Info("UI-only server listening", "addr", addr)
	fmt.Printf("\n  Inngest UI-only dashboard: http://%s\n", resolveDisplayAddr(addr))
	fmt.Printf("  GraphQL playground:        http://%s/v0/\n\n", resolveDisplayAddr(addr))

	server := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	err = server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// resolveDisplayAddr replaces 0.0.0.0 with 127.0.0.1 for display purposes.
func resolveDisplayAddr(addr string) string {
	if len(addr) > 0 && addr[0] == ':' {
		return "127.0.0.1" + addr
	}
	if len(addr) > 7 && addr[:7] == "0.0.0.0" {
		return "127.0.0.1" + addr[7:]
	}
	return addr
}

// mountUI sets up the SPA routing for the embedded Inngest UI.
// It serves static assets and falls back to the shell HTML for client-side routing.
func mountUI(router chi.Router) {
	// Serve the dev info endpoint that the UI expects, returning minimal info
	// indicating this is a UI-only instance.
	router.Get("/dev", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"version": "ui-only",
			"startOpts": {},
			"functions": [],
			"handlers": [],
			"isSingleNodeService": false,
			"isUIOnly": true,
			"features": {
				"run-details-v4": true
			}
		}`))
	})

	// Serve static assets (images, JS/CSS bundles) from the embedded FS
	staticFS, _ := fs.Sub(devserver.StaticFS(), "static/client")
	fileServer := http.FileServer(http.FS(staticFS))
	router.Get("/images/*", fileServer.ServeHTTP)
	router.Get("/assets/*", fileServer.ServeHTTP)
	router.Get("/{file}.txt", fileServer.ServeHTTP)
	router.Get("/{file}.svg", fileServer.ServeHTTP)
	router.Get("/{file}.jpg", fileServer.ServeHTTP)
	router.Get("/{file}.png", fileServer.ServeHTTP)

	// SPA fallback: everything else loads the shell HTML for client-side routing
	router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		byt := devserver.Serve(r.URL.Path)
		if byt == nil {
			http.NotFound(w, r)
			return
		}
		// Set correct content type for HTML
		if !strings.Contains(path.Base(r.URL.Path), ".") {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
		}
		_, _ = w.Write(byt)
	})
}

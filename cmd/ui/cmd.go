package ui

import (
	"github.com/urfave/cli/v3"
)

func Command() *cli.Command {
	cmd := &cli.Command{
		Name:        "ui",
		Usage:       "Run a UI-only Inngest dashboard (no execution engine).",
		UsageText:   "inngest ui [options]",
		Description: "Starts a read-only UI dashboard that connects to an existing Inngest PostgreSQL database and Redis instance. No event ingestion, function execution, or SDK registration occurs.",
		Action:      action,

		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "host",
				Usage: "Server hostname to bind to",
			},
			&cli.StringFlag{
				Name:    "port",
				Aliases: []string{"p"},
				Value:   "8288",
				Usage:   "Server port",
			},

			// Database flags
			&cli.StringFlag{
				Category: "Database",
				Name:     "postgres-uri",
				Usage:    "PostgreSQL database URI (required)",
			},
			&cli.StringFlag{
				Category: "Database",
				Name:     "redis-uri",
				Usage:    "Redis server URI for reading run state",
			},
			&cli.IntFlag{
				Category: "Database",
				Name:     "postgres-max-idle-conns",
				Usage:    "Max idle PostgreSQL connections",
				Value:    10,
			},
			&cli.IntFlag{
				Category: "Database",
				Name:     "postgres-max-open-conns",
				Usage:    "Max open PostgreSQL connections",
				Value:    50,
			},
			&cli.IntFlag{
				Category: "Database",
				Name:     "postgres-conn-max-idle-time",
				Usage:    "Max idle time (minutes) for PostgreSQL connections",
				Value:    5,
			},
			&cli.IntFlag{
				Category: "Database",
				Name:     "postgres-conn-max-lifetime",
				Usage:    "Max lifetime (minutes) for PostgreSQL connections",
				Value:    30,
			},

			// Auth flags
			&cli.StringFlag{
				Category: "Auth",
				Name:     "signing-key",
				Usage:    "Signing key for API authentication (hex string). If not set, API endpoints are unauthenticated.",
			},
		},
	}

	return cmd
}

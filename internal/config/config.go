package config

import (
	"flag"
	"os"

	"github.com/dustin/go-humanize"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type Config struct {
	PublicURL            string
	ContentDirectory     string
	Debug                bool
	Port                 string
	MaxBytesTotal        int64
	MaxBytesPerFile      int64
	MaxBytesPerFileHuman string
	MinutesPerGigabyte   float64
}

func LoadConfig() *Config {
	cfg := &Config{}

	// Flag variables
	flag.StringVar(&cfg.ContentDirectory, "data", "data", "data directory")
	flag.StringVar(&cfg.PublicURL, "public", "", "public URL to use")
	flag.StringVar(&cfg.Port, "port", "8222", "port to use")
	flag.BoolVar(&cfg.Debug, "debug", false, "debug mode")
	flag.Int64Var(&cfg.MaxBytesPerFile, "max-file", 1000000000, "max bytes per file")
	flag.Int64Var(&cfg.MaxBytesTotal, "max-total", 10000000000, "max bytes total")
	flag.Float64Var(&cfg.MinutesPerGigabyte, "min-per-gig", 60, "minutes per gigabyte for auto-deletion")
	flag.Parse()

	// Initialize Zerolog with console writer and log level (if you want to keep this logic here)
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})
	if cfg.Debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	// Initialize config
	cfg.MaxBytesPerFileHuman = humanize.Bytes(uint64(cfg.MaxBytesPerFile))
	if cfg.PublicURL == "" {
		cfg.PublicURL = "http://localhost:" + cfg.Port
	}
	os.Mkdir(cfg.ContentDirectory, os.ModePerm)

	return cfg
}

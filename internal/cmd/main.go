package cmd

import (
	"github.com/rs/zerolog/log"

	"github.com/tuilakhanh/webshare/internal/config"
	"github.com/tuilakhanh/webshare/internal/handlers"
)

func Main() {
	cfg := config.LoadConfig()
	server := handlers.NewServer(cfg)

	log.Info().Msgf("Starting server on :%s", cfg.Port)
	if err := server.Start(); err != nil {
		log.Fatal().Err(err).Msg("Error starting server")
	}
}

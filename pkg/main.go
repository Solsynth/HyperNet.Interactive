package main

import (
	"fmt"
	pkg "git.solsynth.dev/hydrogen/interactive/pkg/internal"
	"git.solsynth.dev/hydrogen/interactive/pkg/internal/gap"
	"git.solsynth.dev/hypernet/nexus/pkg/nex/sec"
	"github.com/fatih/color"
	"os"
	"os/signal"
	"syscall"

	"git.solsynth.dev/hydrogen/interactive/pkg/internal/database"
	"git.solsynth.dev/hydrogen/interactive/pkg/internal/grpc"
	"git.solsynth.dev/hydrogen/interactive/pkg/internal/http"
	"git.solsynth.dev/hydrogen/interactive/pkg/internal/services"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

func init() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})
}

func main() {
	// Booting screen
	fmt.Println(color.YellowString(" ___       _                      _   _\n|_ _|_ __ | |_ ___ _ __ __ _  ___| |_(_)_   _____\n | || '_ \\| __/ _ \\ '__/ _` |/ __| __| \\ \\ / / _ \\\n | || | | | ||  __/ | | (_| | (__| |_| |\\ V /  __/\n|___|_| |_|\\__\\___|_|  \\__,_|\\___|\\__|_| \\_/ \\___|"))
	fmt.Printf("%s v%s\n", color.New(color.FgHiYellow).Add(color.Bold).Sprintf("Hypernet.Interactive"), pkg.AppVersion)
	fmt.Printf("The social networking service in Hypernet\n")
	color.HiBlack("=====================================================\n")

	// Configure settings
	viper.AddConfigPath(".")
	viper.AddConfigPath("..")
	viper.SetConfigName("settings")
	viper.SetConfigType("toml")

	// Load settings
	if err := viper.ReadInConfig(); err != nil {
		log.Panic().Err(err).Msg("An error occurred when loading settings.")
	}

	// Connect to nexus
	if err := gap.InitializeToNexus(); err != nil {
		log.Fatal().Err(err).Msg("An error occurred when connecting to nexus...")
	}

	// Load keypair
	if reader, err := sec.NewInternalTokenReader(viper.GetString("security.internal_public_key")); err != nil {
		log.Error().Err(err).Msg("An error occurred when reading internal public key for jwt. Authentication related features will be disabled.")
	} else {
		http.IReader = reader
		log.Info().Msg("Internal jwt public key loaded.")
	}

	// Connect to database
	if err := database.NewGorm(); err != nil {
		log.Fatal().Err(err).Msg("An error occurred when connect to database.")
	} else if err := database.RunMigration(database.C); err != nil {
		log.Fatal().Err(err).Msg("An error occurred when running database auto migration.")
	}

	// Configure timed tasks
	quartz := cron.New(cron.WithLogger(cron.VerbosePrintfLogger(&log.Logger)))
	quartz.AddFunc("@every 60m", services.DoAutoDatabaseCleanup)
	quartz.Start()

	// Server
	go http.NewServer().Listen()

	go grpc.NewGrpc().Listen()

	// Messages
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	quartz.Stop()
}

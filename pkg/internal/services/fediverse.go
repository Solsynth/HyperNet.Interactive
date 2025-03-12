package services

import (
	"fmt"

	"git.solsynth.dev/hypernet/interactive/pkg/internal/database"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/models"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/services/mastodon"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"
	"github.com/spf13/viper"
	"gorm.io/gorm/clause"
)

type FromFediversePost interface {
	ToFediversePost() models.FediversePost
}

type FediverseFriendConfig struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	Type      string `json:"type"`
	BatchSize int    `json:"batch_size" toml:"batch_size"`
}

var fediverseFriends []FediverseFriendConfig

func ReadFriendConfig() {
	if err := viper.UnmarshalKey("fediverse.friends", &fediverseFriends); err != nil {
		log.Error().Err(err).Msg("Failed to loading fediverse friend config...")
	}
	log.Info().Int("count", len(fediverseFriends)).Msg("Loaded fediverse friend config!")
}

func FetchFediversePost(cfg FediverseFriendConfig) ([]models.FediversePost, error) {
	switch cfg.Type {
	case "mastodon":
		data, err := mastodon.FetchTimeline(cfg.URL, 50)
		if err != nil {
			return nil, err
		}
		posts := lo.Map(data, func(item mastodon.MastodonPost, _ int) models.FediversePost {
			return item.ToFediversePost()
		})
		return posts, nil
	default:
		// TODO Other platform fetching is still under development
		// DO NOT USE THEM
		return nil, fmt.Errorf("unsupported fediverse service: %s", cfg.Type)
	}
}

func FetchFediverseTimedTask() {
	if len(fediverseFriends) == 0 {
		return
	}

	log.Debug().Msg("Starting fetching fediverse friends timeline...")

	var totalPosts []models.FediversePost
	for _, friend := range fediverseFriends {
		log.Info().Str("id", friend.ID).Str("url", friend.URL).Msg("Fetching fediverse friend timeline...")
		posts, err := FetchFediversePost(friend)
		if err != nil {
			log.Error().Err(err).Str("id", friend.ID).Str("url", friend.URL).Msg("Failed to fetch fediverse friend timelime...")
		}
		log.Info().Str("id", friend.ID).Str("url", friend.URL).Int("count", len(posts)).Msg("Fetched fediverse friend timeline...")
		totalPosts = append(totalPosts, posts...)
	}

	if err := database.C.Clauses(clause.OnConflict{DoNothing: true}).Create(&totalPosts).Error; err != nil {
		log.Error().Err(err).Msg("Failed to save timeline posts...")
	}
}

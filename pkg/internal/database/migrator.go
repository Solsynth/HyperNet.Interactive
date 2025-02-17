package database

import (
	"git.solsynth.dev/hypernet/interactive/pkg/internal/models"
	"gorm.io/gorm"
)

var AutoMaintainRange = []any{
	&models.Publisher{},
	&models.Category{},
	&models.Tag{},
	&models.Post{},
	&models.PostInsight{},
	&models.Subscription{},
	&models.Poll{},
	&models.PollAnswer{},
	&models.PostFlag{},
	&models.PostView{},
}

func RunMigration(source *gorm.DB) error {
	if err := source.AutoMigrate(
		append(
			AutoMaintainRange,
			&models.Reaction{},
		)...,
	); err != nil {
		return err
	}

	return nil
}

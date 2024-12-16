package services

import (
	"fmt"

	"git.solsynth.dev/hypernet/interactive/pkg/internal/database"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/gap"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/models"
	"git.solsynth.dev/hypernet/passport/pkg/authkit"
	"git.solsynth.dev/hypernet/pusher/pkg/pushkit"
	"github.com/rs/zerolog/log"
)

func GetAccountWithID(id uint) (models.Publisher, error) {
	var account models.Publisher
	if err := database.C.Where("id = ?", id).First(&account).Error; err != nil {
		return account, fmt.Errorf("unable to get account by id: %v", err)
	}
	return account, nil
}

func ModifyPosterVoteCount(user models.Publisher, isUpvote bool, delta int) error {
	if isUpvote {
		user.TotalUpvote += delta
		return database.C.Model(&user).Update("total_upvote", user.TotalUpvote).Error
	} else {
		user.TotalDownvote += delta
		return database.C.Model(&user).Update("total_downvote", user.TotalDownvote).Error
	}
}

func NotifyPosterAccount(pub models.Publisher, post models.Post, title, body, topic string, subtitle ...string) error {
	if pub.AccountID == nil {
		return nil
	}

	if len(subtitle) == 0 {
		subtitle = append(subtitle, "")
	}

	err := authkit.NotifyUser(gap.Nx, uint64(*pub.AccountID), pushkit.Notification{
		Topic:    topic,
		Title:    title,
		Subtitle: subtitle[0],
		Body:     body,
		Priority: 4,
		Metadata: map[string]any{
			"related_post": TruncatePostContent(post),
			"avatar":       pub.Avatar,
		},
	})
	if err != nil {
		log.Warn().Err(err).Msg("An error occurred when notify account...")
	} else {
		log.Debug().Uint("uid", pub.ID).Msg("Notified account.")
	}

	return err
}

package queries

import (
	"fmt"

	"git.solsynth.dev/hypernet/interactive/pkg/internal/database"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/gap"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/models"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/services"
	"git.solsynth.dev/hypernet/paperclip/pkg/filekit"
	fmodels "git.solsynth.dev/hypernet/paperclip/pkg/filekit/models"
	"git.solsynth.dev/hypernet/passport/pkg/authkit"
	amodels "git.solsynth.dev/hypernet/passport/pkg/authkit/models"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"
	"gorm.io/gorm"
)

// This api still is experimental and finally with replace the old one
// Some changes between ListPost and ListPostV2:
//   - Post reply to and repost to are not included
func ListPostV2(tx *gorm.DB, take int, offset int, order any, user *uint) ([]models.Post, error) {
	if take > 100 {
		take = 100
	}

	if take >= 0 {
		tx = tx.Limit(take)
	}
	if offset >= 0 {
		tx = tx.Offset(offset)
	}

	tx = tx.Preload("Tags").
		Preload("Categories").
		Preload("Publisher")

	// Fetch posts
	var posts []models.Post
	if err := tx.Order(order).Find(&posts).Error; err != nil {
		return nil, err
	}

	// If no posts found, return early
	if len(posts) == 0 {
		return posts, nil
	}

	// Collect post IDs
	idx := make([]uint, len(posts))
	itemMap := make(map[uint]*models.Post, len(posts))
	for i, item := range posts {
		idx[i] = item.ID
		itemMap[item.ID] = &item
	}

	// Batch load reactions
	if mapping, err := services.BatchListPostReactions(database.C.Where("post_id IN ?", idx), "post_id"); err != nil {
		return posts, err
	} else {
		for postID, reactions := range mapping {
			if post, exists := itemMap[postID]; exists {
				post.Metric.ReactionList = reactions
			}
		}
	}

	// Batch load reply counts efficiently
	var replies []struct {
		PostID uint
		Count  int64
	}
	if err := database.C.Model(&models.Post{}).
		Select("reply_id as post_id, COUNT(id) as count").
		Where("reply_id IN (?)", idx).
		Group("post_id").
		Find(&replies).Error; err != nil {
		return posts, err
	}
	for _, info := range replies {
		if post, exists := itemMap[info.PostID]; exists {
			post.Metric.ReplyCount = info.Count
		}
	}

	// Batch load some metadata
	var attachmentsRid []string
	var usersId []uint

	// Scan records that can be load egearly
	for _, info := range posts {
		if info.Publisher.AccountID != nil {
			usersId = append(usersId, *info.Publisher.AccountID)
		}
		if raw, ok := info.Body["attachments"].([]any); ok && len(raw) > 0 {
			attachmentsRid := make([]string, 0, len(raw))
			for _, v := range raw {
				if str, ok := v.(string); ok {
					attachmentsRid = append(attachmentsRid, str)
				}
			}
		}
	}
	log.Debug().Int("attachments", len(attachmentsRid)).Int("users", len(usersId)).Msg("Scanned metadata to load for listing post...")

	// Batch load attachments
	attachmentsRid = lo.Uniq(attachmentsRid)
	attachments, err := filekit.ListAttachment(gap.Nx, attachmentsRid)
	if err != nil {
		return posts, fmt.Errorf("failed to load attachments: %v", err)
	}

	// Batch load publisher users
	usersId = lo.Uniq(usersId)
	users, err := authkit.ListUser(gap.Nx, usersId)
	if err != nil {
		return posts, fmt.Errorf("failed to load users: %v", err)
	}

	// Putting information back to data
	log.Info().Int("attachments", len(attachments)).Int("users", len(users)).Msg("Batch loaded metadata for listing post...")
	for idx, item := range posts {
		var this []fmodels.Attachment
		var val []string
		if raw, ok := item.Body["attachments"].([]any); ok && len(raw) > 0 {
			val = lo.Map(raw, func(v any, _ int) string {
				return v.(string) // Safe if you're sure all elements are strings
			})
		} else if raw, ok := item.Body["attachments"].([]string); ok {
			val = raw
		}
		if len(val) > 0 {
			this = lo.Filter(attachments, func(item fmodels.Attachment, _ int) bool {
				return lo.Contains(val, item.Rid)
			})
		}
		item.Body["attachments"] = this
		item.Publisher.Account = lo.FindOrElse(users, amodels.Account{}, func(acc amodels.Account) bool {
			if item.Publisher.AccountID == nil {
				return false
			}
			return acc.ID == *item.Publisher.AccountID
		})
		posts[idx] = item
	}

	// Add post views for the user
	if user != nil {
		services.AddPostViews(posts, *user)
	}

	return posts, nil
}

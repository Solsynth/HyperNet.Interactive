package queries

import (
	"encoding/json"
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

const singularAttachmentFields = []string{"video", "thumbnail"}

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
	var bodies []models.PostStoryBody
	{
		raw, _ := json.Marshal(posts)
		json.Unmarshal(raw, &bodies)
	}
	for idx, info := range posts {
		if info.Publisher.AccountID != nil {
			usersId = append(usersId, *info.Publisher.AccountID)
		}
		attachmentsRid = append(attachmentsRid, bodies[idx].Attachments...)
		for _, field := range singularAttachmentFields {
			if raw, ok := info.Body[field]; ok {
				if str, ok := raw.(string); ok {
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
		if len(bodies[idx].Attachments) > 0 {
			this = lo.Filter(attachments, func(item fmodels.Attachment, _ int) bool {
				return lo.Contains(bodies[idx].Attachments, item.Rid)
			})
		}
		for _, field := range singularAttachmentFields {
			if raw, ok := item.Body[field]; ok {
				if str, ok := raw.(string); ok {
					item.Body[field] = lo.FindOrElse(this, fmodels.Attachment{}, func(item fmodels.Attachment) bool {
						return item.Rid == str
					})
				}
			}
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

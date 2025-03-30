package queries

import (
	"fmt"

	"github.com/goccy/go-json"

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

var singularAttachmentFields = []string{"video", "thumbnail"}

func CompletePostMeta(in ...models.Post) ([]models.Post, error) {
	// Collect post IDs
	idx := make([]uint, len(in))
	itemMap := make(map[uint]*models.Post, len(in))
	for i, item := range in {
		idx[i] = item.ID
		itemMap[item.ID] = &in[i]
	}

	// Batch load reactions
	if mapping, err := services.BatchListPostReactions(database.C.Where("post_id IN ?", idx), "post_id"); err != nil {
		return in, err
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
		return in, err
	}
	for _, info := range replies {
		if post, exists := itemMap[info.PostID]; exists {
			post.Metric.ReplyCount = info.Count
		}
	}

	// Batch load some metadata
	var err error
	var attachmentsRid []string
	var usersId []uint
	var realmsId []uint

	// Scan records that can be load eagerly
	var bodies []models.PostStoryBody
	{
		raw, _ := json.Marshal(lo.Map(in, func(item models.Post, _ int) map[string]any {
			return item.Body
		}))
		json.Unmarshal(raw, &bodies)
	}
	for idx, info := range in {
		if info.Publisher.AccountID != nil {
			usersId = append(usersId, *info.Publisher.AccountID)
		}
		if info.RealmID != nil {
			realmsId = append(realmsId, *info.RealmID)
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
	var attachments []fmodels.Attachment
	if len(attachmentsRid) > 0 {
		attachments, err = filekit.ListAttachment(gap.Nx, attachmentsRid)
		if err != nil {
			return in, fmt.Errorf("failed to load attachments: %v", err)
		}
	}

	// Batch load publisher users
	usersId = lo.Uniq(usersId)
	var users []amodels.Account
	if len(users) > 0 {
		users, err = authkit.ListUser(gap.Nx, usersId)
		if err != nil {
			return in, fmt.Errorf("failed to load users: %v", err)
		}
	}

	// Batch load posts realm
	realmsId = lo.Uniq(realmsId)
	var realms []amodels.Realm
	if len(realmsId) > 0 {
		realms, err = authkit.ListRealm(gap.Nx, realmsId)
		if err != nil {
			return in, fmt.Errorf("failed to load realms: %v", err)
		}
	}

	// Putting information back to data
	log.Info().Int("attachments", len(attachments)).Int("users", len(users)).Msg("Batch loaded metadata for listing post...")
	for idx, item := range in {
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
		if item.Publisher.AccountID != nil {
			item.Publisher.Account = lo.FindOrElse(users, amodels.Account{}, func(acc amodels.Account) bool {
				return acc.ID == *item.Publisher.AccountID
			})
		}
		if item.RealmID != nil {
			item.Realm = lo.ToPtr(lo.FindOrElse(realms, amodels.Realm{}, func(realm amodels.Realm) bool {
				return realm.ID == *item.RealmID
			}))
		}
		in[idx] = item
	}

	return in, nil
}

func GetPost(tx *gorm.DB, id uint, user *uint) (models.Post, error) {
	var post models.Post
	if err := tx.Preload("Tags").
		Preload("Categories").
		Preload("Publisher").
		Preload("Poll").
		First(&post, id).Error; err != nil {
		return post, err
	}

	out, err := CompletePostMeta(post)
	if err != nil {
		return post, err
	}

	if user != nil {
		services.AddPostView(post, *user)
	}

	return out[0], nil
}

func GetPostByAlias(tx *gorm.DB, alias, area string, user *uint) (models.Post, error) {
	var post models.Post
	if err := tx.Preload("Tags").
		Preload("Categories").
		Preload("Publisher").
		Preload("Poll").
		Where("alias = ?", alias).
		Where("alias_prefix = ?", area).
		First(&post).Error; err != nil {
		return post, err
	}

	out, err := CompletePostMeta(post)
	if err != nil {
		return post, err
	}

	if user != nil {
		services.AddPostView(post, *user)
	}

	return out[0], nil
}

func ListPost(tx *gorm.DB, take int, offset int, order any, user *uint) ([]models.Post, error) {
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
		Preload("Publisher").
		Preload("Poll")

	// Fetch posts
	var posts []models.Post
	if err := tx.Order(order).Find(&posts).Error; err != nil {
		return nil, err
	}

	// If no posts found, return early
	if len(posts) == 0 {
		return posts, nil
	}

	// Load data eagerly
	posts, err := CompletePostMeta(posts...)
	if err != nil {
		return nil, err
	}

	// Add post views for the user
	if user != nil {
		services.AddPostViews(posts, *user)
	}

	return posts, nil
}

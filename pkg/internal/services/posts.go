package services

import (
	"context"
	"errors"
	"fmt"
	localCache "git.solsynth.dev/hypernet/interactive/pkg/internal/cache"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/gap"
	"git.solsynth.dev/hypernet/nexus/pkg/proto"
	"git.solsynth.dev/hypernet/passport/pkg/authkit"
	authm "git.solsynth.dev/hypernet/passport/pkg/authkit/models"
	"github.com/eko/gocache/lib/v4/cache"
	"github.com/eko/gocache/lib/v4/marshaler"
	"github.com/eko/gocache/lib/v4/store"
	"regexp"
	"strconv"
	"strings"
	"time"

	"git.solsynth.dev/hypernet/interactive/pkg/internal/database"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/models"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func FilterPostWithUserContext(tx *gorm.DB, user *authm.Account) *gorm.DB {
	if user == nil {
		return tx.Where("visibility = ?", models.PostVisibilityAll)
	}

	const (
		FriendsVisibility  = models.PostVisibilityFriends
		SelectedVisibility = models.PostVisibilitySelected
		FilteredVisibility = models.PostVisibilityFiltered
		NoneVisibility     = models.PostVisibilityNone
	)

	type userContextState struct {
		Allowlist []uint
		Blocklist []uint
	}

	cacheManager := cache.New[any](localCache.S)
	marshal := marshaler.New(cacheManager)
	ctx := context.Background()

	var allowlist, blocklist []uint

	statusCacheKey := fmt.Sprintf("post-user-context-query#%d", user.ID)
	statusCache, err := marshal.Get(ctx, statusCacheKey, new(userContextState))
	if err == nil {
		state := statusCache.(*userContextState)
		allowlist = state.Allowlist
		blocklist = state.Blocklist
	} else {
		userFriends, _ := authkit.ListRelative(gap.Nx, user.ID, int32(authm.RelationshipFriend), true)
		userBlocked, _ := authkit.ListRelative(gap.Nx, user.ID, int32(authm.RelationshipBlocked), true)
		userFriendList := lo.Map(userFriends, func(item *proto.UserInfo, index int) uint {
			return uint(item.GetId())
		})
		userBlockList := lo.Map(userBlocked, func(item *proto.UserInfo, index int) uint {
			return uint(item.GetId())
		})

		// Query the publishers according to the user's relationship
		var publishers []models.Publisher
		database.C.Where(
			"id IN ? AND type = ?",
			append(userFriendList, userBlockList...),
			models.PublisherTypePersonal,
		).Find(&publishers)

		allowlist = lo.Map(lo.Filter(publishers, func(item models.Publisher, index int) bool {
			if item.AccountID == nil {
				return false
			}
			return lo.Contains(userFriendList, *item.AccountID)
		}), func(item models.Publisher, index int) uint {
			return uint(item.ID)
		})
		blocklist = lo.Map(lo.Filter(publishers, func(item models.Publisher, index int) bool {
			if item.AccountID == nil {
				return false
			}
			return lo.Contains(userBlockList, *item.AccountID)
		}), func(item models.Publisher, index int) uint {
			return uint(item.ID)
		})

		_ = marshal.Set(
			ctx,
			statusCacheKey,
			userContextState{
				Allowlist: allowlist,
				Blocklist: blocklist,
			},
			store.WithExpiration(5*time.Minute),
			store.WithTags([]string{"post-user-context-query", fmt.Sprintf("user#%d", user.ID)}),
		)
	}

	tx = tx.Where(
		"(visibility != ? OR (visibility = ? AND publisher_id IN ? AND publisher_id NOT IN ?) OR (visibility = ? AND ?) OR (visibility = ? AND NOT ?) OR publisher_id = ?)",
		NoneVisibility,
		FriendsVisibility,
		allowlist,
		blocklist,
		SelectedVisibility,
		datatypes.JSONQuery("visible_users").HasKey(strconv.Itoa(int(user.ID))),
		FilteredVisibility,
		datatypes.JSONQuery("invisible_users").HasKey(strconv.Itoa(int(user.ID))),
		user.ID,
	)

	return tx
}

func FilterPostWithCategory(tx *gorm.DB, alias string) *gorm.DB {
	return tx.Joins("JOIN post_categories ON posts.id = post_categories.post_id").
		Joins("JOIN categories ON categories.id = post_categories.category_id").
		Where("categories.alias IN ?", strings.Split(alias, ",")).
		Distinct("posts.id")
}

func FilterPostWithTag(tx *gorm.DB, alias string) *gorm.DB {
	aliases := strings.Split(alias, ",")
	return tx.Joins("JOIN post_tags ON posts.id = post_tags.post_id").
		Joins("JOIN tags ON tags.id = post_tags.tag_id").
		Where("tags.alias IN ?", aliases).
		Group("posts.id").
		Having("COUNT(DISTINCT tags.id) = ?", len(aliases))
}

func FilterPostWithType(tx *gorm.DB, t string) *gorm.DB {
	return tx.Where("type = ?", t)
}

func FilterPostReply(tx *gorm.DB, replyTo ...uint) *gorm.DB {
	if len(replyTo) > 0 && replyTo[0] > 0 {
		return tx.Where("reply_id = ?", replyTo[0])
	} else {
		return tx.Where("reply_id IS NULL")
	}
}

func FilterPostWithPublishedAt(tx *gorm.DB, date time.Time) *gorm.DB {
	return tx.
		Where("published_at <= ? OR published_at IS NULL", date).
		Where("published_until > ? OR published_until IS NULL", date)
}

func FilterPostWithAuthorDraft(tx *gorm.DB, uid uint) *gorm.DB {
	return tx.Where("publisher_id = ? AND is_draft = ?", uid, true)
}

func FilterPostDraft(tx *gorm.DB) *gorm.DB {
	return tx.Where("is_draft = ? OR is_draft IS NULL", false)
}

func FilterPostWithFuzzySearch(tx *gorm.DB, probe string) *gorm.DB {
	if len(probe) == 0 {
		return tx
	}

	probe = "%" + probe + "%"
	return tx.
		Where("? AND body->>'content' ILIKE ?", gorm.Expr("body ? 'content'"), probe).
		Or("? AND body->>'title' ILIKE ?", gorm.Expr("body ? 'title'"), probe).
		Or("? AND body->>'description' ILIKE ?", gorm.Expr("body ? 'description'"), probe)
}

func PreloadGeneral(tx *gorm.DB) *gorm.DB {
	return tx.
		Preload("Tags").
		Preload("Categories").
		Preload("Publisher").
		Preload("ReplyTo").
		Preload("ReplyTo.Publisher").
		Preload("ReplyTo.Tags").
		Preload("ReplyTo.Categories").
		Preload("RepostTo").
		Preload("RepostTo.Publisher").
		Preload("RepostTo.Tags").
		Preload("RepostTo.Categories")
}

func GetPost(tx *gorm.DB, id uint, ignoreLimitation ...bool) (models.Post, error) {
	if len(ignoreLimitation) == 0 || !ignoreLimitation[0] {
		tx = FilterPostWithPublishedAt(tx, time.Now())
	}

	var item models.Post
	if err := PreloadGeneral(tx).
		Where("id = ?", id).
		First(&item).Error; err != nil {
		return item, err
	}

	return item, nil
}

func GetPostByAlias(tx *gorm.DB, alias, area string, ignoreLimitation ...bool) (models.Post, error) {
	if len(ignoreLimitation) == 0 || !ignoreLimitation[0] {
		tx = FilterPostWithPublishedAt(tx, time.Now())
	}

	var item models.Post
	if err := PreloadGeneral(tx).
		Where("alias = ?", alias).
		Where("area_alias = ?", area).
		First(&item).Error; err != nil {
		return item, err
	}

	return item, nil
}

func CountPost(tx *gorm.DB) (int64, error) {
	var count int64
	if err := tx.Model(&models.Post{}).Count(&count).Error; err != nil {
		return count, err
	}

	return count, nil
}

func CountPostReply(id uint) int64 {
	var count int64
	if err := database.C.Model(&models.Post{}).
		Where("reply_id = ?", id).
		Count(&count).Error; err != nil {
		return 0
	}

	return count
}

func CountPostReactions(id uint) int64 {
	var count int64
	if err := database.C.Model(&models.Reaction{}).
		Where("post_id = ?", id).
		Count(&count).Error; err != nil {
		return 0
	}

	return count
}

func ListPost(tx *gorm.DB, take int, offset int, order any, noReact ...bool) ([]*models.Post, error) {
	if take > 100 {
		take = 100
	}

	var items []*models.Post
	if err := PreloadGeneral(tx).
		Limit(take).Offset(offset).
		Order(order).
		Find(&items).Error; err != nil {
		return items, err
	}

	idx := lo.Map(items, func(item *models.Post, index int) uint {
		return item.ID
	})

	// Load reactions
	if len(noReact) <= 0 || !noReact[0] {
		if mapping, err := BatchListPostReactions(database.C.Where("post_id IN ?", idx), "post_id"); err != nil {
			return items, err
		} else {
			itemMap := lo.SliceToMap(items, func(item *models.Post) (uint, *models.Post) {
				return item.ID, item
			})

			for k, v := range mapping {
				if post, ok := itemMap[k]; ok {
					post.Metric = models.PostMetric{
						ReactionList: v,
					}
				}
			}
		}
	}

	// Load replies
	if len(noReact) <= 0 || !noReact[0] {
		var replies []struct {
			PostID uint
			Count  int64
		}

		if err := database.C.Model(&models.Post{}).
			Select("reply_id as post_id, COUNT(id) as count").
			Where("reply_id IN (?)", idx).
			Group("post_id").
			Scan(&replies).Error; err != nil {
			return items, err
		}

		itemMap := lo.SliceToMap(items, func(item *models.Post) (uint, *models.Post) {
			return item.ID, item
		})

		list := map[uint]int64{}
		for _, info := range replies {
			list[info.PostID] = info.Count
		}

		for k, v := range list {
			if post, ok := itemMap[k]; ok {
				post.Metric = models.PostMetric{
					ReactionList: post.Metric.ReactionList,
					ReplyCount:   v,
				}
			}
		}
	}

	return items, nil
}

func ListPostMinimal(tx *gorm.DB, take int, offset int, order any) ([]*models.Post, error) {
	if take > 500 {
		take = 500
	}

	var items []*models.Post
	if err := tx.
		Limit(take).Offset(offset).
		Order(order).
		Find(&items).Error; err != nil {
		return items, err
	}

	return items, nil
}

func EnsurePostCategoriesAndTags(item models.Post) (models.Post, error) {
	var err error
	for idx, category := range item.Categories {
		item.Categories[idx], err = GetCategory(category.Alias)
		if err != nil {
			return item, err
		}
	}
	for idx, tag := range item.Tags {
		item.Tags[idx], err = GetTagOrCreate(tag.Alias, tag.Name)
		if err != nil {
			return item, err
		}
	}
	return item, nil
}

func NewPost(user models.Publisher, item models.Post) (models.Post, error) {
	if item.Alias != nil && len(*item.Alias) == 0 {
		item.Alias = nil
	}
	if item.PublishedAt != nil && item.PublishedAt.Unix() < time.Now().Unix() {
		return item, fmt.Errorf("post cannot be published before now")
	}

	if item.Alias != nil {
		re := regexp.MustCompile(`^[a-z0-9.-]+$`)
		if !re.MatchString(*item.Alias) {
			return item, fmt.Errorf("invalid post alias, learn more about alias rule on our wiki")
		}
	}

	if item.Realm != nil {
		item.AliasPrefix = &item.Realm.Alias
	} else {
		item.AliasPrefix = &user.Name
	}

	log.Debug().Any("body", item.Body).Msg("Posting a post...")
	start := time.Now()

	log.Debug().Any("tags", item.Tags).Any("categories", item.Categories).Msg("Preparing categories and tags...")
	item, err := EnsurePostCategoriesAndTags(item)
	if err != nil {
		return item, err
	}

	log.Debug().Msg("Saving post record into database...")
	if err := database.C.Save(&item).Error; err != nil {
		return item, err
	}

	// Notify the original poster its post has been replied
	if item.ReplyID != nil {
		var op models.Post
		if err := database.C.
			Where("id = ?", item.ReplyID).
			Preload("Publisher").
			First(&op).Error; err == nil {
			if op.Publisher.AccountID != nil && op.Publisher.ID != user.ID {
				log.Debug().Uint("user", *op.Publisher.AccountID).Msg("Notifying the original poster their post got replied...")
				err = NotifyPosterAccount(
					op.Publisher,
					op,
					"Post got replied",
					fmt.Sprintf("%s (%s) replied your post (#%d).", user.Nick, user.Name, op.ID),
					"interactive.reply",
					fmt.Sprintf("%s replied you", user.Nick),
				)
				if err != nil {
					log.Error().Err(err).Msg("An error occurred when notifying user...")
				}
			}
		}
	}

	// Notify the subscriptions
	if content, ok := item.Body["content"].(string); ok {
		var title *string
		title, _ = item.Body["title"].(*string)
		go func() {
			if err := NotifyUserSubscription(user, content, title); err != nil {
				log.Error().Err(err).Msg("An error occurred when notifying subscriptions user by user...")
			}
			for _, tag := range item.Tags {
				if err := NotifyTagSubscription(tag, user, content, title); err != nil {
					log.Error().Err(err).Msg("An error occurred when notifying subscriptions user by tag...")
				}
			}
			for _, category := range item.Categories {
				if err := NotifyCategorySubscription(category, user, content, title); err != nil {
					log.Error().Err(err).Msg("An error occurred when notifying subscriptions user by category...")
				}
			}
		}()
	}

	log.Debug().Dur("elapsed", time.Since(start)).Msg("The post is posted.")
	return item, nil
}

func EditPost(item models.Post) (models.Post, error) {
	if item.Alias != nil && len(*item.Alias) == 0 {
		item.Alias = nil
	}
	if item.PublishedAt != nil && item.PublishedAt.Unix() < item.CreatedAt.Unix() {
		return item, fmt.Errorf("post cannot be published before it is created")
	}

	if item.Alias != nil {
		re := regexp.MustCompile(`^[a-z0-9.-]+$`)
		if !re.MatchString(*item.Alias) {
			return item, fmt.Errorf("invalid post alias, learn more about alias rule on our wiki")
		}
	}

	if item.Realm != nil {
		item.AliasPrefix = &item.Realm.Alias
	} else {
		item.AliasPrefix = &item.Publisher.Name
	}

	item, err := EnsurePostCategoriesAndTags(item)
	if err != nil {
		return item, err
	}

	err = database.C.Save(&item).Error

	return item, err
}

func DeletePost(item models.Post) error {
	return database.C.Delete(&item).Error
}

func ReactPost(user authm.Account, reaction models.Reaction) (bool, models.Reaction, error) {
	var op models.Post
	if err := database.C.
		Where("id = ?", reaction.PostID).
		Preload("Publisher").
		First(&op).Error; err != nil {
		return true, reaction, err
	}

	if err := database.C.Where(reaction).First(&reaction).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if op.Publisher.AccountID != nil && *op.Publisher.AccountID != user.ID {
				err = NotifyPosterAccount(
					op.Publisher,
					op,
					"Post got reacted",
					fmt.Sprintf("%s (%s) reacted your post a %s.", user.Nick, user.Name, reaction.Symbol),
					"interactive.feedback",
					fmt.Sprintf("%s reacted you", user.Nick),
				)
				if err != nil {
					log.Error().Err(err).Msg("An error occurred when notifying user...")
				}
			}

			err = database.C.Save(&reaction).Error
			if err == nil && reaction.Attitude != models.AttitudeNeutral {
				_ = ModifyPosterVoteCount(op.Publisher, reaction.Attitude == models.AttitudePositive, 1)

				if reaction.Attitude == models.AttitudePositive {
					op.TotalUpvote++
				} else {
					op.TotalDownvote++
				}
				database.C.Save(&op)
			}

			return true, reaction, err
		} else {
			return true, reaction, err
		}
	} else {
		err = database.C.Delete(&reaction).Error
		if err == nil && reaction.Attitude != models.AttitudeNeutral {
			_ = ModifyPosterVoteCount(op.Publisher, reaction.Attitude == models.AttitudePositive, -1)

			if reaction.Attitude == models.AttitudePositive {
				op.TotalUpvote--
			} else {
				op.TotalDownvote--
			}
			database.C.Save(&op)
		}

		return false, reaction, err
	}
}

func PinPost(post models.Post) (bool, error) {
	if post.PinnedAt != nil {
		post.PinnedAt = nil
	} else {
		post.PinnedAt = lo.ToPtr(time.Now())
	}

	if err := database.C.Save(&post).Error; err != nil {
		return post.PinnedAt != nil, err
	}
	return post.PinnedAt != nil, nil
}

const TruncatePostContentThreshold = 160

func TruncatePostContent(post models.Post) models.Post {
	if post.Body["content"] != nil {
		if val, ok := post.Body["content"].(string); ok {
			length := TruncatePostContentThreshold
			post.Body["content_length"] = len([]rune(val))
			if len([]rune(val)) >= length {
				post.Body["content"] = string([]rune(val)[:length]) + "..."
				post.Body["content_truncated"] = true
			}
		}
	}

	return post
}

const TruncatePostContentShortThreshold = 80

func TruncatePostContentShort(content string) string {
	length := TruncatePostContentShortThreshold
	if len([]rune(content)) >= length {
		return string([]rune(content)[:length]) + "..."
	} else {
		return content
	}
}

package services

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"git.solsynth.dev/hypernet/nexus/pkg/nex"
	"git.solsynth.dev/hypernet/nexus/pkg/nex/cachekit"
	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v2"

	"git.solsynth.dev/hypernet/interactive/pkg/internal/gap"
	"git.solsynth.dev/hypernet/nexus/pkg/proto"
	"git.solsynth.dev/hypernet/paperclip/pkg/filekit"
	pproto "git.solsynth.dev/hypernet/paperclip/pkg/proto"
	"git.solsynth.dev/hypernet/passport/pkg/authkit"
	authm "git.solsynth.dev/hypernet/passport/pkg/authkit/models"
	aproto "git.solsynth.dev/hypernet/passport/pkg/proto"
	"gorm.io/datatypes"

	"git.solsynth.dev/hypernet/interactive/pkg/internal/database"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/models"
	"github.com/rs/zerolog/log"
	"github.com/samber/lo"
	"gorm.io/gorm"
)

func FilterPostWithUserContext(c *fiber.Ctx, tx *gorm.DB, user *authm.Account) *gorm.DB {
	if user == nil {
		return tx.Where("visibility = ? AND realm_id IS NULL", models.PostVisibilityAll)
	}

	const (
		AllVisibility      = models.PostVisibilityAll
		FriendsVisibility  = models.PostVisibilityFriends
		SelectedVisibility = models.PostVisibilitySelected
		FilteredVisibility = models.PostVisibilityFiltered
	)

	type userContextState struct {
		Self          []uint `json:"self"`
		Allowlist     []uint `json:"allow"`
		InvisibleList []uint `json:"invisible"`
		FollowList    []uint `json:"follow"`
		RealmList     []uint `json:"realm"`
	}

	var self, allowlist, invisibleList, followList, realmList []uint

	statusCacheKey := fmt.Sprintf("post-user-filter#%d", user.ID)
	state, err := cachekit.Get[userContextState](gap.Ca, statusCacheKey)
	if err == nil {
		allowlist = state.Allowlist
		invisibleList = state.InvisibleList
		followList = state.FollowList
		realmList = state.RealmList
		self = state.Self
	} else {
		// Get itself
		{
			var publishers []models.Publisher
			if err := database.C.Where("account_id = ?", user.ID).Find(&publishers).Error; err != nil {
				return tx
			}
			self = lo.Map(publishers, func(item models.Publisher, index int) uint {
				return item.ID
			})
			allowlist = append(allowlist, self...)
		}

		// Getting the relationships
		userFriends, _ := authkit.ListRelative(gap.Nx, user.ID, int32(authm.RelationshipFriend), true)
		userGotBlocked, _ := authkit.ListRelative(gap.Nx, user.ID, int32(authm.RelationshipBlocked), true)
		userBlocked, _ := authkit.ListRelative(gap.Nx, user.ID, int32(authm.RelationshipBlocked), false)

		// Getting the realm list
		{
			conn, err := gap.Nx.GetClientGrpcConn(nex.ServiceTypeAuth)
			if err == nil {
				ac := aproto.NewRealmServiceClient(conn)
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
				defer cancel()
				resp, err := ac.ListAvailableRealm(ctx, &aproto.LookupUserRealmRequest{
					UserId:        uint64(user.ID),
					IncludePublic: lo.ToPtr(true),
				})
				if err == nil {
					realmList = lo.Map(resp.GetData(), func(item *aproto.RealmInfo, index int) uint {
						return uint(item.GetId())
					})
				} else {
					log.Warn().Err(err).Uint("user", user.ID).Msg("An error occurred when getting realm list from grpc...")
				}
			} else {
				log.Warn().Err(err).Uint("user", user.ID).Msg("An error occurred when getting grpc connection to Auth...")
			}
		}

		userFriendList := lo.Map(userFriends, func(item *proto.UserInfo, index int) uint {
			return uint(item.GetId())
		})
		userGotBlockList := lo.Map(userGotBlocked, func(item *proto.UserInfo, index int) uint {
			return uint(item.GetId())
		})
		userBlocklist := lo.Map(userBlocked, func(item *proto.UserInfo, index int) uint {
			return uint(item.GetId())
		})

		// Query the publishers according to the user's relationship
		var publishers []models.Publisher
		database.C.Where(
			"account_id IN ? AND type = ?",
			lo.Uniq(append(append(userFriendList, userGotBlockList...), userBlocklist...)),
			models.PublisherTypePersonal,
		).Find(&publishers)

		// Getting the follow list
		{
			var subs []models.Subscription
			if err := database.C.Where("follower_id = ? AND account_id IS NOT NULL", user.ID).Find(&subs).Error; err != nil {
				log.Error().Err(err).Msg("An error occurred when getting subscriptions...")
			}
			followList = lo.Map(lo.Filter(subs, func(item models.Subscription, index int) bool {
				return item.AccountID != nil
			}), func(item models.Subscription, index int) uint {
				return *item.AccountID
			})
		}

		allowlist = lo.Map(lo.Filter(publishers, func(item models.Publisher, index int) bool {
			if item.AccountID == nil {
				return false
			}
			return lo.Contains(userFriendList, *item.AccountID)
		}), func(item models.Publisher, index int) uint {
			return item.ID
		})
		invisibleList = lo.Map(lo.Filter(publishers, func(item models.Publisher, index int) bool {
			if item.AccountID == nil {
				return false
			}
			return lo.Contains(userBlocklist, *item.AccountID)
		}), func(item models.Publisher, index int) uint {
			return item.ID
		})

		cachekit.Set(
			gap.Ca,
			statusCacheKey,
			userContextState{
				Allowlist:     allowlist,
				InvisibleList: invisibleList,
				RealmList:     realmList,
				FollowList:    followList,
				Self:          self,
			},
			5*time.Minute,
			fmt.Sprintf("user#%d", user.ID),
		)
	}

	if len(self) == 0 && len(allowlist) == 0 {
		tx = tx.Where(
			"(visibility = ? OR"+
				"(visibility = ? AND ?) OR "+
				"(visibility = ? AND NOT ?))",
			AllVisibility,
			SelectedVisibility,
			datatypes.JSONQuery("visible_users").HasKey(strconv.Itoa(int(user.ID))),
			FilteredVisibility,
			datatypes.JSONQuery("invisible_users").HasKey(strconv.Itoa(int(user.ID))),
		)
	} else if len(self) == 0 {
		tx = tx.Where(
			"(visibility = ? OR"+
				"(visibility = ? AND publisher_id IN ?) OR "+
				"(visibility = ? AND ?) OR "+
				"(visibility = ? AND NOT ?))",
			AllVisibility,
			FriendsVisibility,
			allowlist,
			SelectedVisibility,
			datatypes.JSONQuery("visible_users").HasKey(strconv.Itoa(int(user.ID))),
			FilteredVisibility,
			datatypes.JSONQuery("invisible_users").HasKey(strconv.Itoa(int(user.ID))),
		)
	} else {
		tx = tx.Where(
			"(publisher_id IN ? OR visibility = ? OR"+
				"(visibility = ? AND publisher_id IN ?) OR "+
				"(visibility = ? AND ?) OR "+
				"(visibility = ? AND NOT ?))",
			self,
			AllVisibility,
			FriendsVisibility,
			allowlist,
			SelectedVisibility,
			datatypes.JSONQuery("visible_users").HasKey(strconv.Itoa(int(user.ID))),
			FilteredVisibility,
			datatypes.JSONQuery("invisible_users").HasKey(strconv.Itoa(int(user.ID))),
		)
	}

	if len(invisibleList) > 0 {
		tx = tx.Where("publisher_id NOT IN ?", invisibleList)
	}
	if len(c.Query("realm")) == 0 {
		if len(realmList) > 0 {
			tx = tx.Where("realm_id IN ? OR realm_id IS NULL", realmList)
		} else {
			tx = tx.Where("realm_id IS NULL")
		}
	}

	switch c.Query("channel") {
	case "friends":
		tx = tx.Where("publisher_id IN ?", allowlist)
	case "following":
		tx = tx.Where("publisher_id IN ?", followList)
	}

	return tx
}

func FilterPostWithRealm(tx *gorm.DB, probe string) *gorm.DB {
	if numericId, err := strconv.Atoi(probe); err == nil {
		return tx.Where("realm_id = ?", uint(numericId))
	}

	realm, err := authkit.GetRealmByAlias(gap.Nx, probe)
	if err != nil {
		log.Warn().Msgf("Failed to find realm with alias %s: %s", probe, err)
		return tx
	}

	return tx.Where("realm_id = ?", realm.ID)
}

func FilterPostWithCategory(tx *gorm.DB, alias string) *gorm.DB {
	aliases := strings.Split(alias, ",")
	return tx.Joins("JOIN post_categories ON posts.id = post_categories.post_id").
		Joins("JOIN categories ON categories.id = post_categories.category_id").
		Where("categories.alias IN ?", aliases).
		Group("posts.id").
		Having("COUNT(DISTINCT categories.id) = ?", len(aliases))
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

func FilterPostWithPublishedAt(tx *gorm.DB, date time.Time, uid ...uint) *gorm.DB {
	var publishers []models.Publisher
	if len(uid) > 0 {
		if err := database.C.Where("account_id = ?", uid[0]).Find(&publishers).Error; err == nil {
		}
	}

	return tx.
		Where("(published_at < ? OR published_at IS NULL)", date).
		Where("(published_until >= ? OR published_until IS NULL)", date)
}

func FilterPostWithAuthorDraft(tx *gorm.DB, uid uint) *gorm.DB {
	var publishers []models.Publisher
	if err := database.C.Where("account_id = ?", uid).Find(&publishers).Error; err != nil {
		return FilterPostDraft(tx)
	}
	if len(publishers) == 0 {
		return FilterPostDraft(tx)
	}
	idSet := lo.Map(publishers, func(item models.Publisher, index int) uint {
		return item.ID
	})
	return tx.Where("publisher_id IN ? AND is_draft = ?", idSet, true)
}

func FilterPostDraft(tx *gorm.DB) *gorm.DB {
	return tx.Where("is_draft = ? OR is_draft IS NULL", false)
}

func FilterPostDraftWithAuthor(tx *gorm.DB, uid uint) *gorm.DB {
	var publishers []models.Publisher
	if err := database.C.Where("account_id = ?", uid).Find(&publishers).Error; err != nil {
		return FilterPostDraft(tx)
	}
	if len(publishers) == 0 {
		return FilterPostDraft(tx)
	}
	idSet := lo.Map(publishers, func(item models.Publisher, index int) uint {
		return item.ID
	})
	return tx.Where("(is_draft = ? OR is_draft IS NULL) OR publisher_id IN ?", false, idSet)
}

func FilterPostWithFuzzySearch(tx *gorm.DB, probe string) *gorm.DB {
	if len(probe) == 0 {
		return tx
	}

	probe = "%" + probe + "%"
	return tx.
		Where(
			"(? AND body->>'content' ILIKE ? OR ? AND body->>'title' ILIKE ? OR ? AND body->>'description' ILIKE ?)",
			gorm.Expr("body ? 'content'"),
			probe,
			gorm.Expr("body ? 'title'"),
			probe,
			gorm.Expr("body ? 'description'"),
			probe,
		)
}

func PreloadGeneral(tx *gorm.DB) *gorm.DB {
	return tx.
		Preload("Tags").
		Preload("Categories").
		Preload("Publisher").
		Preload("Poll")
}

func GetPost(tx *gorm.DB, id uint) (models.Post, error) {
	var item models.Post
	if err := PreloadGeneral(tx).
		Where("id = ?", id).
		First(&item).Error; err != nil {
		return item, err
	}

	return item, nil
}

func GetPostByAlias(tx *gorm.DB, alias, area string) (models.Post, error) {
	var item models.Post
	if err := PreloadGeneral(tx).
		Where("alias = ?", alias).
		Where("alias_prefix = ?", area).
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

func ListPost(tx *gorm.DB, take int, offset int, order any, user *uint, noReact ...bool) ([]models.Post, error) {
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
		itemMap[item.ID] = &posts[i]
	}

	// Batch load reactions
	if mapping, err := BatchListPostReactions(database.C.Where("post_id IN ?", idx), "post_id"); err != nil {
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

	// Add post views for the user
	if user != nil {
		AddPostViews(posts, *user)
	}

	return posts, nil
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

func NotifyReplying(item models.Post, user models.Publisher) error {
	content, ok := item.Body["content"].(string)
	if !ok {
		content = "Posted a post"
	} else {
		content = TruncatePostContentShort(content)
	}

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
				fmt.Sprintf("%s (%s) replied you: %s", user.Nick, user.Name, content),
				"interactive.reply",
				fmt.Sprintf("%s replied your post #%d", user.Nick, *item.ReplyID),
			)
			if err != nil {
				log.Error().Err(err).Msg("An error occurred when notifying user...")
			}
		}
	}
	return nil
}

func NotifySubscribers(item models.Post, user models.Publisher) error {
	content, ok := item.Body["content"].(string)
	if !ok {
		content = "Posted a post"
	}
	var title *string
	title, _ = item.Body["title"].(*string)
	item.Publisher = user
	if err := NotifyUserSubscription(user, item, content, title); err != nil {
		log.Error().Err(err).Msg("An error occurred when notifying subscriptions user by user...")
	}
	for _, tag := range item.Tags {
		if err := NotifyTagSubscription(tag, user, item, content, title); err != nil {
			log.Error().Err(err).Msg("An error occurred when notifying subscriptions user by tag...")
		}
	}
	for _, category := range item.Categories {
		if err := NotifyCategorySubscription(category, user, item, content, title); err != nil {
			log.Error().Err(err).Msg("An error occurred when notifying subscriptions user by category...")
		}
	}
	return nil
}

func NewPost(user models.Publisher, item models.Post) (models.Post, error) {
	if item.Alias != nil && len(*item.Alias) == 0 {
		item.Alias = nil
	}
	if item.PublishedAt != nil && item.PublishedAt.UTC().Unix() < time.Now().UTC().Unix() {
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

	item.Publisher = user
	err = UpdatePostAttachmentMeta(item)
	if err != nil {
		log.Error().Err(err).Msg("An error occurred when updating post attachment meta...")
	}

	// Notify the original poster its post has been replied
	if item.ReplyID != nil && !item.IsDraft {
		go NotifyReplying(item, user)
	}
	// Notify the subscriptions
	if item.ReplyID == nil && !item.IsDraft {
		go NotifySubscribers(item, user)
	}

	log.Debug().Dur("elapsed", time.Since(start)).Msg("The post is posted.")
	return item, nil
}

func EditPost(item models.Post, og models.Post) (models.Post, error) {
	if _, ok := item.Body["content_truncated"]; ok {
		return item, fmt.Errorf("prevented from editing post with truncated content")
	}

	if !item.IsDraft && item.PublishedAt == nil {
		item.PublishedAt = lo.ToPtr(time.Now())
	}
	if item.Alias != nil && len(*item.Alias) == 0 {
		item.Alias = nil
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

	_ = database.C.Model(&item).Association("Categories").Replace(item.Categories)
	_ = database.C.Model(&item).Association("Tags").Replace(item.Tags)

	pub := item.Publisher
	err = database.C.Save(&item).Error

	if err == nil {
		item.Publisher = pub
		err = UpdatePostAttachmentMeta(item)
		if err != nil {
			log.Error().Err(err).Msg("An error occurred when updating post attachment meta...")
		}

		if og.IsDraft && !item.IsDraft {
			// Notify the original poster its post has been replied
			if item.ReplyID != nil {
				go NotifyReplying(item, item.Publisher)
			}
			// Notify the subscriptions
			if item.ReplyID == nil {
				go NotifySubscribers(item, item.Publisher)
			}
		}
	}

	return item, err
}

func UpdatePostAttachmentMeta(item models.Post, old ...models.Post) error {
	log.Debug().Any("attachments", item.Body["attachments"]).Msg("Updating post attachments meta...")

	// Marking usage
	sameAsOld := false
	if len(old) > 0 {
		sameAsOld = reflect.DeepEqual(old[0].Body, item.Body)
	}

	var oldBody, newBody models.PostStoryBody
	if len(old) > 0 {
		raw, _ := json.Marshal(old[0].Body)
		json.Unmarshal(raw, &oldBody)
	}
	{
		raw, _ := json.Marshal(item.Body)
		json.Unmarshal(raw, &newBody)
	}
	var minusAttachments, plusAttachments []string
	if len(old) > 0 && !sameAsOld {
		minusAttachments = append(minusAttachments, oldBody.Attachments...)
	}
	if len(old) == 0 || !sameAsOld {
		plusAttachments = append(plusAttachments, newBody.Attachments...)
	}
	if dat, ok := item.Body["thumbnail"].(string); ok {
		plusAttachments = append(plusAttachments, dat)
	}
	if dat, ok := item.Body["video"].(string); ok {
		plusAttachments = append(plusAttachments, dat)
	}
	if len(minusAttachments) > 0 {
		filekit.CountAttachmentUsage(gap.Nx, &pproto.UpdateUsageRequest{
			Rid:   minusAttachments,
			Delta: -1,
		})
	}
	if len(plusAttachments) > 0 {
		filekit.CountAttachmentUsage(gap.Nx, &pproto.UpdateUsageRequest{
			Rid:   plusAttachments,
			Delta: 1,
		})
	}

	// Updating visibility
	if item.Publisher.AccountID == nil {
		log.Warn().Msg("Post publisher did not have account id, skip updating attachments meta...")
		return nil
	}

	if val, ok := item.Body["attachments"].([]any); ok && len(val) > 0 {
		conn, err := gap.Nx.GetClientGrpcConn("uc")
		if err != nil {
			log.Error().Err(err).Msg("An error occurred when getting grpc connection to Paperclip...")
			return nil
		}

		pc := pproto.NewAttachmentServiceClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		resp, err := pc.UpdateVisibility(ctx, &pproto.UpdateVisibilityRequest{
			Rid: lo.Map(val, func(item any, _ int) string {
				return item.(string)
			}),
			UserId:      lo.ToPtr(uint64(*item.Publisher.AccountID)),
			IsIndexable: item.Visibility == models.PostVisibilityAll,
		})
		if err != nil {
			log.Error().Any("attachments", val).Err(err).Msg("An error occurred when updating post attachment visibility...")
			return err
		}

		log.Debug().Any("attachments", val).Int32("count", resp.Count).Msg("Post attachment visibility updated.")
	} else {
		log.Debug().Any("attachments", val).Msg("Post attachment visibility update skipped...")
	}

	return nil
}

func DeletePost(item models.Post) error {
	copiedItem := item
	if err := database.C.Delete(&copiedItem).Error; err != nil {
		return err
	}

	// Cleaning up related attachments
	var body models.PostStoryBody
	{
		raw, _ := json.Marshal(item.Body)
		json.Unmarshal(raw, &body)
	}
	if len(body.Attachments) > 0 {
		if item.Publisher.AccountID == nil {
			return nil
		}

		err := filekit.CountAttachmentUsage(gap.Nx, &pproto.UpdateUsageRequest{
			Rid: lo.Uniq(body.Attachments),
		})
		if err != nil {
			log.Error().Err(err).Msg("An error occurred when deleting post attachment...")
		}
	}

	return nil
}

func DeletePostInBatch(items []models.Post) error {
	if err := database.C.Delete(&items).Error; err != nil {
		return err
	}

	var bodies []models.PostStoryBody
	{
		raw, _ := json.Marshal(items)
		json.Unmarshal(raw, &bodies)
	}

	var attachments []string
	for idx := range items {
		if len(bodies[idx].Attachments) > 0 {
			attachments = append(attachments, bodies[idx].Attachments...)
		}
	}

	err := filekit.CountAttachmentUsage(gap.Nx, &pproto.UpdateUsageRequest{
		Rid: lo.Uniq(attachments),
	})
	if err != nil {
		log.Error().Err(err).Msg("An error occurred when deleting post attachment...")
	}

	return nil
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
					database.C.Model(&op).Update("total_upvote", op.TotalUpvote)
				} else {
					op.TotalDownvote++
					database.C.Model(&op).Update("total_downvote", op.TotalDownvote)
				}
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
				database.C.Model(&op).Update("total_upvote", op.TotalUpvote)
			} else {
				op.TotalDownvote--
				database.C.Model(&op).Update("total_downvote", op.TotalDownvote)
			}
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

	if err := database.C.Model(&post).Update("pinned_at", post.PinnedAt).Error; err != nil {
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

	if post.RepostTo != nil {
		post.RepostTo = lo.ToPtr(TruncatePostContent(*post.RepostTo))
	}
	if post.ReplyTo != nil {
		post.ReplyTo = lo.ToPtr(TruncatePostContent(*post.ReplyTo))
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

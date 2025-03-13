package services

import (
	"fmt"
	"sort"
	"time"

	"git.solsynth.dev/hypernet/interactive/pkg/internal/database"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/models"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/samber/lo"
	"gorm.io/gorm"
)

type FeedEntry struct {
	Type      string    `json:"type"`
	Data      any       `json:"data"`
	CreatedAt time.Time `json:"created_at"`
}

func GetFeed(c *fiber.Ctx, limit int, user *uint, cursor *time.Time) ([]FeedEntry, error) {
	// We got two types of data for now
	// Plan to let each of them take 50% of the output

	var feed []FeedEntry

	interTx, err := UniversalPostFilter(c, database.C)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare load interactive posts: %v", err)
	}
	if cursor != nil {
		interTx = interTx.Where("published_at < ?", *cursor)
	}
	interPosts, err := ListPostForFeed(interTx, limit/2, user)
	if err != nil {
		return nil, fmt.Errorf("failed to load interactive posts: %v", err)
	}
	feed = append(feed, interPosts...)

	fediTx := database.C
	if cursor != nil {
		fediTx = fediTx.Where("created_at < ?", *cursor)
	}
	fediPosts, err := ListFediversePostForFeed(fediTx, limit/2)
	if err != nil {
		return feed, fmt.Errorf("failed to load fediverse posts: %v", err)
	}
	feed = append(feed, fediPosts...)

	sort.Slice(feed, func(i, j int) bool {
		return feed[i].CreatedAt.After(feed[j].CreatedAt)
	})

	return feed, nil
}

// We assume the database context already handled the filtering and pagination
// Only manage to pulling the content only

func ListPostForFeed(tx *gorm.DB, limit int, user *uint) ([]FeedEntry, error) {
	posts, err := ListPost(tx, limit, -1, "published_at DESC", user)
	if err != nil {
		return nil, err
	}
	entries := lo.Map(posts, func(post *models.Post, _ int) FeedEntry {
		return FeedEntry{
			Type:      "interactive.post",
			Data:      services.TruncatePostContent(post),
			CreatedAt: post.CreatedAt,
		}
	})
	return entries, nil
}

func ListFediversePostForFeed(tx *gorm.DB, limit int) ([]FeedEntry, error) {
	var posts []models.FediversePost
	if err := tx.
		Preload("User").Limit(limit).
		Find(&posts).Error; err != nil {
		return nil, err
	}
	entries := lo.Map(posts, func(post models.FediversePost, _ int) FeedEntry {
		return FeedEntry{
			Type:      "fediverse.post",
			Data:      post,
			CreatedAt: post.CreatedAt,
		}
	})
	return entries, nil
}

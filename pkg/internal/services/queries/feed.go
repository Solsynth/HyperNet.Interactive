package queries

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"git.solsynth.dev/hypernet/interactive/pkg/internal/database"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/gap"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/models"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/services"
	"git.solsynth.dev/hypernet/interactive/pkg/proto"
	"git.solsynth.dev/hypernet/nexus/pkg/nex"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
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

	// Planing the feed
	limitF := float64(limit)
	interCount := int(math.Ceil(limitF * 0.7))
	readerCount := int(math.Ceil(limitF * 0.3))

	// Internal posts
	interTx, err := services.UniversalPostFilter(c, database.C)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare load interactive posts: %v", err)
	}
	if cursor != nil {
		interTx = interTx.Where("published_at < ?", *cursor)
	}
	posts, err := ListPostForFeed(interTx, interCount, user, c.Get("X-API-Version", "1"))
	if err != nil {
		return nil, fmt.Errorf("failed to load interactive posts: %v", err)
	}
	feed = append(feed, posts...)

	sort.Slice(feed, func(i, j int) bool {
		return feed[i].CreatedAt.After(feed[j].CreatedAt)
	})

	// News today - from Reader
	if news, err := ListReaderPagesForFeed(readerCount, cursor); err != nil {
		log.Error().Err(err).Msg("Failed to load news in getting feed...")
	} else {
		feed = append(feed, news...)
	}

	return feed, nil
}

// We assume the database context already handled the filtering and pagination
// Only manage to pulling the content only

func ListPostForFeed(tx *gorm.DB, limit int, user *uint, api string) ([]FeedEntry, error) {
	var posts []models.Post
	var err error
	rankOrder := `(COALESCE(total_upvote, 0) - COALESCE(total_downvote, 0) + 
		LOG(1 + COALESCE(total_aggressive_views, 0))) /
		POWER(EXTRACT(EPOCH FROM NOW() - published_at) / 3600 + 2, 1.5) DESC`
	if api == "2" {
		posts, err = ListPost(tx, limit, -1, rankOrder, user)
	} else {
		posts, err = services.ListPost(tx, limit, -1, rankOrder, user)
	}
	if err != nil {
		return nil, err
	}
	entries := lo.Map(posts, func(post models.Post, _ int) FeedEntry {
		return FeedEntry{
			Type:      "interactive.post",
			Data:      services.TruncatePostContent(post),
			CreatedAt: post.CreatedAt,
		}
	})
	return entries, nil
}

func ListReaderPagesForFeed(limit int, cursor *time.Time) ([]FeedEntry, error) {
	conn, err := gap.Nx.GetClientGrpcConn("re")
	if err != nil {
		return nil, fmt.Errorf("failed to get grpc connection with reader: %v", err)
	}
	client := proto.NewFeedServiceClient(conn)
	request := &proto.GetFeedRequest{
		Limit: int64(limit),
	}
	if cursor != nil {
		request.Cursor = lo.ToPtr(uint64(cursor.UnixMilli()))
	}
	resp, err := client.GetFeed(context.Background(), request)
	if err != nil {
		return nil, fmt.Errorf("failed to get feed from reader: %v", err)
	}
	var createdAt time.Time
	return lo.Map(resp.Items, func(item *proto.FeedItem, _ int) FeedEntry {
		cta := time.UnixMilli(int64(item.CreatedAt))
		createdAt = lo.Ternary(createdAt.Before(cta), cta, createdAt)
		return FeedEntry{
			Type:      item.Type,
			Data:      nex.DecodeMap(item.Content),
			CreatedAt: cta,
		}
	}), nil
}

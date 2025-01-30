package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	iproto "git.solsynth.dev/hypernet/insight/pkg/proto"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/database"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/gap"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/models"
	"github.com/rs/zerolog/log"
)

func GeneratePostInsights(post models.Post, user uint) (string, error) {
	var insight models.PostInsight
	if err := database.C.Where("post_id = ?", post.ID).First(&insight).Error; err == nil {
		return insight.Response, nil
	}

	var compactBuilder []string
	if val, ok := post.Body["title"].(string); ok && len(val) > 0 {
		compactBuilder = append(compactBuilder, "Title: "+val)
	}
	if val, ok := post.Body["description"].(string); ok && len(val) > 0 {
		compactBuilder = append(compactBuilder, "Description: "+val)
	}
	if val, ok := post.Body["content"].(string); ok && len(val) > 0 {
		compactBuilder = append(compactBuilder, val)
	}

	compact := strings.Join(compactBuilder, "\n")

	conn, err := gap.Nx.GetClientGrpcConn("ai")
	if err != nil {
		return "", fmt.Errorf("failed to connect Insight: %v", err)
	}
	ic := iproto.NewInsightServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	resp, err := ic.GenerateInsight(ctx, &iproto.InsightRequest{
		Source: compact,
		UserId: uint64(user),
	})
	if err != nil {
		return "", err
	}

	insight = models.PostInsight{
		Response: resp.Response,
		Post:     post,
		PostID:   post.ID,
	}
	if err := database.C.Create(&insight).Error; err != nil {
		log.Error().Err(err).Msg("Failed to create post insight result in database...")
	}

	return resp.Response, nil
}

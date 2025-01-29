package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	iproto "git.solsynth.dev/hypernet/insight/pkg/proto"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/gap"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/models"
)

func GetPostInsightCacheKey(postId uint) string {
	return fmt.Sprintf("post-insight-%d", postId)
}

func GeneratePostInsights(post models.Post, user uint) (string, error) {
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
		return resp.Response, err
	}

	return resp.Response, nil
}

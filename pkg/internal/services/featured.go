package services

import (
	"git.solsynth.dev/hypernet/interactive/pkg/internal/database"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/models"
	"time"
)

// GetFeaturedPosts How to determine featured posts?
// Get the most upvoted posts in the last 7 days
// And then how to get the upvote count of each post in the last 7 days?
// We will get the reactions that attitude equals to 1 and created within the last 7 days
// By the way, the upvote count will subtract the downvote count
// Notice, this function is a raw query, it is not recommended to return the result directly
// Instead, you should get the id and query it again via the ListPost function
func GetFeaturedPosts(count int) ([]models.Post, error) {
	deadline := time.Now().Add(-7 * 24 * time.Hour)

	var posts []models.Post
	if err := database.C.Raw(`
		SELECT p.*, t.social_points
        FROM posts p
        JOIN (
            SELECT 
                post_id, 
                SUM(CASE WHEN attitude = 1 THEN 1 ELSE 0 END) -
                SUM(CASE WHEN attitude = 2 THEN 1 ELSE 0 END) AS social_points
            FROM reactions
            WHERE created_at >= ?
            GROUP BY post_id
            ORDER BY social_points DESC
            LIMIT ?
        ) t ON p.id = t.post_id
		WHERE p.visibility = ?
        ORDER BY t.social_points DESC, p.published_at DESC
	`, deadline, count, models.PostVisibilityAll).Scan(&posts).Error; err != nil {
		return posts, err
	}

	return posts, nil
}

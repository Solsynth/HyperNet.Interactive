package services

import (
	"fmt"

	"git.solsynth.dev/hypernet/interactive/pkg/internal/database"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/models"
	"github.com/samber/lo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var postViewQueue []models.PostView

func AddPostView(post models.Post, account uint) {
	postViewQueue = append(postViewQueue, models.PostView{
		AccountID: account,
		PostID:    post.ID,
	})
}

func AddPostViews(posts []models.Post, account uint) {
	for _, post := range posts {
		postViewQueue = append(postViewQueue, models.PostView{
			AccountID: account,
			PostID:    post.ID,
		})
	}
}

func FlushPostViews() {
	if len(postViewQueue) == 0 {
		return
	}
	workingQueue := make([]models.PostView, len(postViewQueue))
	copy(workingQueue, postViewQueue)
	clear(postViewQueue)
	updateRequiredPost := make(map[uint]int)
	for _, item := range workingQueue {
		updateRequiredPost[item.PostID]++
	}
	workingQueue = lo.UniqBy(workingQueue, func(item models.PostView) string {
		return fmt.Sprintf("%d:%d", item.PostID, item.AccountID)
	})
	_ = database.C.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(workingQueue, 1000).Error
	for k, v := range updateRequiredPost {
		var count int64
		if err := database.C.Model(&models.PostView{}).Where("post_id = ?", k).Count(&count).Error; err != nil {
			continue
		}
		database.C.Model(&models.Post{}).Where("id = ?", k).Updates(map[string]any{
			"total_views":            count,
			"total_aggressive_views": gorm.Expr("total_aggressive_views + ?", v),
		})
	}
}

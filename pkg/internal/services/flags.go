package services

import (
	"fmt"

	"git.solsynth.dev/hypernet/interactive/pkg/internal/database"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/models"
)

func NewFlag(post models.Post, account uint) (models.PostFlag, error) {
	var flag models.PostFlag
	if err := database.C.Where("post_id = ? AND account_id = ?", post.ID, account).Error; err == nil {
		return flag, fmt.Errorf("flag already exists")
	}
	flag = models.PostFlag{
		PostID:    post.ID,
		AccountID: account,
	}
	if err := database.C.Save(&flag).Error; err != nil {
		return flag, err
	}
	if err := FlagCalculateCollapseStatus(post); err != nil {
		return flag, err
	}
	return flag, nil
}

func FlagCalculateCollapseStatus(post models.Post) error {
	collapseLimit := 0.5

	var flagCount int64
	if err := database.C.Model(&models.PostFlag{}).Where("post_id = ?", post.ID).Count(&flagCount).Error; err != nil {
		return err
	}
	if float64(flagCount)/float64(post.TotalViews) >= collapseLimit {
		return database.C.Model(&post).Update("is_collapsed", true).Error
	}
	return nil
}

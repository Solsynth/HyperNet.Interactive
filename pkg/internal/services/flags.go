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
	return flag, nil
}

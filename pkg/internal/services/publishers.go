package services

import (
	"fmt"
	"git.solsynth.dev/hydrogen/interactive/pkg/internal/database"
	"git.solsynth.dev/hydrogen/interactive/pkg/internal/models"
)

func GetPublisher(id uint, userID uint) (models.Publisher, error) {
	var publisher models.Publisher
	if err := database.C.Where("id = ? AND account_id = ?", id, userID).First(&publisher).Error; err != nil {
		return publisher, fmt.Errorf("unable to get publisher: %v", err)
	}
	return publisher, nil
}

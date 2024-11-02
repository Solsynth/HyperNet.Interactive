package services

import (
	"git.solsynth.dev/hypernet/interactive/pkg/internal/database"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/models"
)

func ListTags(take int, offset int) ([]models.Tag, error) {
	var tags []models.Tag
	err := database.C.Offset(offset).Limit(take).Find(&tags).Error

	return tags, err
}

func SearchTags(take int, offset int, probe string) ([]models.Tag, error) {
	probe = "%" + probe + "%"

	var tags []models.Tag
	err := database.C.Where("alias LIKE ?", probe).Offset(offset).Limit(take).Find(&tags).Error

	return tags, err
}

func GetTag(alias string) (models.Tag, error) {
	var tag models.Tag
	if err := database.C.Where(models.Tag{Alias: alias}).First(&tag).Error; err != nil {
		return tag, err
	}
	return tag, nil
}

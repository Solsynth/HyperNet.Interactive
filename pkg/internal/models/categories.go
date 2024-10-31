package models

import "git.solsynth.dev/hypernet/nexus/pkg/nex/cruda"

type Tag struct {
	cruda.BaseModel

	Alias       string `json:"alias" gorm:"uniqueIndex" validate:"lowercase"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Posts       []Post `json:"posts" gorm:"many2many:post_tags"`
}

type Category struct {
	cruda.BaseModel

	Alias       string `json:"alias" gorm:"uniqueIndex" validate:"lowercase,alphanum"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Posts       []Post `json:"posts" gorm:"many2many:post_categories"`
}

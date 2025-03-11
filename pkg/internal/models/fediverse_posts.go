package models

import "git.solsynth.dev/hypernet/nexus/pkg/nex/cruda"

type FediversePost struct {
	cruda.BaseModel

	Identifier string   `json:"identifier" gorm:"uniqueIndex"`
	Origin     string   `json:"origin"`
	Content    string   `json:"content"`
	Language   string   `json:"language"`
	Images     []string `json:"images"`

	User   FediverseUser `json:"user"`
	UserID uint          `json:"user_id"`
}

type FediverseUser struct {
	cruda.BaseModel

	Identifier string `json:"identifier" gorm:"uniqueIndex"`
	Origin     string `json:"origin"`
	Name       string `json:"name"`
	Nick       string `json:"nick"`
}

package models

import (
	"git.solsynth.dev/hypernet/nexus/pkg/nex/cruda"
	"git.solsynth.dev/hypernet/passport/pkg/authkit/models"
)

const (
	PublisherTypePersonal = iota
	PublisherTypeOrganization
	PublisherTypeAnonymous
)

type Publisher struct {
	cruda.BaseModel

	Type int `json:"type"`

	Name        string `json:"name" gorm:"uniqueIndex"`
	Nick        string `json:"nick"`
	Description string `json:"description"`
	Avatar      string `json:"avatar"`
	Banner      string `json:"banner"`

	Posts []Post `json:"posts"`

	TotalUpvote   int `json:"total_upvote"`
	TotalDownvote int `json:"total_downvote"`

	RealmID   *uint `json:"realm_id"`
	AccountID *uint `json:"account_id"`

	Account models.Account `gorm:"-"`
	Realm   models.Realm   `gorm:"-"`
}

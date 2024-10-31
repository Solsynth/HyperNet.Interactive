package models

import "git.solsynth.dev/hypernet/nexus/pkg/nex/cruda"

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

	Posts         []Post         `json:"posts" gorm:"foreignKey:AuthorID"`
	Reactions     []Reaction     `json:"reactions"`
	Subscriptions []Subscription `json:"subscriptions" gorm:"foreginKey:FollowerID"`

	TotalUpvote   int `json:"total_upvote"`
	TotalDownvote int `json:"total_downvote"`

	RealmID   *uint `json:"realm_id"`
	AccountID *uint `json:"account_id"`
}

package models

import "git.solsynth.dev/hypernet/nexus/pkg/nex/cruda"

type Subscription struct {
	cruda.BaseModel

	FollowerID uint  `json:"follower_id"`
	AccountID  *uint `json:"account_id,omitempty"`
	TagID      *uint `json:"tag_id,omitempty"`
	CategoryID *uint `json:"category_id,omitempty"`
}

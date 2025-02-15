package models

import "git.solsynth.dev/hypernet/nexus/pkg/nex/cruda"

type Subscription struct {
	cruda.BaseModel

	FollowerID uint       `json:"follower_id"`
	AccountID  *uint      `json:"account_id,omitempty"`
	Account    *Publisher `json:"account,omitempty"`
	TagID      *uint      `json:"tag_id,omitempty"`
	Tag        Tag        `json:"tag,omitempty"`
	CategoryID *uint      `json:"category_id,omitempty"`
	Category   Category   `json:"category,omitempty"`
}

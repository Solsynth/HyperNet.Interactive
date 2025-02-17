package models

import "git.solsynth.dev/hypernet/nexus/pkg/nex/cruda"

type PostFlag struct {
	cruda.BaseModel

	PostID    uint `json:"post_id"`
	AccountID uint `json:"account_id"`
}

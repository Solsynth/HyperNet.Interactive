package models

import (
	"time"

	"git.solsynth.dev/hypernet/nexus/pkg/nex/cruda"
	"gorm.io/datatypes"
)

type Poll struct {
	cruda.BaseModel

	ExpiredAt *time.Time                      `json:"expired_at"`
	Options   datatypes.JSONSlice[PollOption] `json:"options"`
	AccountID uint                            `json:"account_id"`

	Metric PollMetric `json:"metric" gorm:"-"`
}

type PollMetric struct {
	TotalAnswer int64            `json:"total_answer"`
	ByOptions   map[string]int64 `json:"by_options"`
}

type PollOption struct {
	ID          string `json:"id"`
	Icon        string `json:"icon"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type PollAnswer struct {
	cruda.BaseModel

	Answer    string `json:"answer"`
	PollID    uint   `json:"poll_id"`
	AccountID uint   `json:"account_id"`
}

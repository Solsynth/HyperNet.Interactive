package models

import (
	"time"
)

type ReactionAttitude = uint8

const (
	AttitudeNeutral = ReactionAttitude(iota)
	AttitudePositive
	AttitudeNegative
)

type Reaction struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	Symbol   string           `json:"symbol"`
	Attitude ReactionAttitude `json:"attitude"`

	PostID    uint `json:"post_id"`
	AccountID uint `json:"account_id"`
}

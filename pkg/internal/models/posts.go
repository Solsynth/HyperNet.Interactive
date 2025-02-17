package models

import (
	"time"

	"git.solsynth.dev/hypernet/nexus/pkg/nex/cruda"
	authm "git.solsynth.dev/hypernet/passport/pkg/authkit/models"

	"gorm.io/datatypes"
)

const (
	PostTypeStory    = "story"
	PostTypeArticle  = "article"
	PostTypeQuestion = "question"
	PostTypeVideo    = "video"
)

type PostVisibilityLevel = int8

const (
	PostVisibilityAll = PostVisibilityLevel(iota)
	PostVisibilityFriends
	PostVisibilityFiltered
	PostVisibilitySelected
	PostVisibilityNone
)

type Post struct {
	cruda.BaseModel

	Type        string            `json:"type"`
	Body        datatypes.JSONMap `json:"body" gorm:"index:,type:gin"`
	Language    string            `json:"language"`
	Alias       *string           `json:"alias"`
	AliasPrefix *string           `json:"alias_prefix"`
	Tags        []Tag             `json:"tags" gorm:"many2many:post_tags"`
	Categories  []Category        `json:"categories" gorm:"many2many:post_categories"`
	Reactions   []Reaction        `json:"reactions"`
	Replies     []Post            `json:"replies" gorm:"foreignKey:ReplyID"`
	Flags       []PostFlag        `json:"flags" gorm:"foreignKey:PostID"`
	ReplyID     *uint             `json:"reply_id"`
	RepostID    *uint             `json:"repost_id"`
	ReplyTo     *Post             `json:"reply_to" gorm:"foreignKey:ReplyID"`
	RepostTo    *Post             `json:"repost_to" gorm:"foreignKey:RepostID"`

	VisibleUsers   datatypes.JSONSlice[uint] `json:"visible_users_list"`
	InvisibleUsers datatypes.JSONSlice[uint] `json:"invisible_users_list"`
	Visibility     PostVisibilityLevel       `json:"visibility"`

	EditedAt *time.Time `json:"edited_at"`
	PinnedAt *time.Time `json:"pinned_at"`
	LockedAt *time.Time `json:"locked_at"`

	IsDraft        bool       `json:"is_draft"`
	PublishedAt    *time.Time `json:"published_at"`
	PublishedUntil *time.Time `json:"published_until"`

	TotalUpvote          int   `json:"total_upvote"`
	TotalDownvote        int   `json:"total_downvote"`
	TotalViews           int64 `json:"total_views"`
	TotalAggressiveViews int64 `json:"total_aggressive_views"`

	PollID *uint `json:"poll_id"`
	Poll   *Poll `json:"poll"`

	RealmID *uint        `json:"realm_id"`
	Realm   *authm.Realm `json:"realm" gorm:"-"`

	PublisherID uint      `json:"publisher_id"`
	Publisher   Publisher `json:"publisher"`

	Metric PostMetric `json:"metric" gorm:"-"`
}

type PostStoryBody struct {
	Thumbnail   *string  `json:"thumbnail"`
	Title       *string  `json:"title"`
	Content     string   `json:"content"`
	Location    *string  `json:"location"`
	Attachments []string `json:"attachments"`
}

type PostArticleBody struct {
	Thumbnail   *string  `json:"thumbnail"`
	Title       string   `json:"title"`
	Description *string  `json:"description"`
	Content     string   `json:"content"`
	Attachments []string `json:"attachments"`
}

type PostQuestionBody struct {
	PostStoryBody
	Answer *uint   `json:"answer"`
	Reward float64 `json:"reward"`
}

type PostVideoBody struct {
	Thumbnail   *string           `json:"thumbnail"`
	Title       string            `json:"title"`
	Description *string           `json:"description"`
	Location    *string           `json:"location"`
	Video       string            `json:"video"`
	IsLive      bool              `json:"is_live"`
	IsLiveEnded bool              `json:"is_live_ended"`
	Subtitles   map[string]string `json:"subtitles"`
}

type PostInsight struct {
	cruda.BaseModel

	Response string `json:"response"`
	Post     Post   `json:"post"`
	PostID   uint   `json:"post_id"`
}

type PostView struct {
	AccountID uint      `json:"account_id" gorm:"primaryKey"`
	PostID    uint      `json:"post_id" gorm:"primaryKey"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

package models

import "time"

type Post struct {
	BaseModel

	Alias       string     `json:"alias" gorm:"uniqueIndex"`
	Title       string     `json:"title"`
	Content     string     `json:"content"`
	Tags        []Tag      `gorm:"many2many:post_tags"`
	Categories  []Category `gorm:"many2many:post_categories"`
	PublishedAt time.Time  `json:"published_at"`
	RealmID     *uint      `json:"realm_id"`
	AuthorID    uint       `json:"author_id"`
	Author      Account    `json:"author"`
}

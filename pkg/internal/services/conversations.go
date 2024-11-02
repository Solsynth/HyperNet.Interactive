package services

import (
	"fmt"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/database"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/models"
	"github.com/spf13/viper"
)

func GetConversation(start uint, offset, take int, order string, participants []uint) ([]models.Post, error) {
	var posts []models.Post

	tablePrefix := viper.GetString("database.prefix")
	table := tablePrefix + "posts"

	result := database.C.Raw(fmt.Sprintf(
		`
        WITH RECURSIVE conversation AS (
            SELECT *
            FROM %s
            WHERE id = ?

            UNION ALL

            SELECT p.*
            FROM %s p
            INNER JOIN conversation c ON p.reply_id = c.id AND p.author_id IN (?)
        )
        SELECT * FROM conversation ORDER BY %s DESC OFFSET %d LIMIT %d`,
		table, table, order, offset, take,
	), start, participants).Scan(&posts)

	// Check for errors
	if result.Error != nil {
		return nil, result.Error
	}

	return posts, nil
}

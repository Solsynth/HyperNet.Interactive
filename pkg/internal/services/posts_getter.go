package services

import (
	"time"

	"git.solsynth.dev/hypernet/interactive/pkg/internal/database"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/models"
	authm "git.solsynth.dev/hypernet/passport/pkg/authkit/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type UniversalPostFilterConfig struct {
	ShowDraft     bool
	ShowReply     bool
	ShowCollapsed bool
	TimeCursor    *time.Time
}

func UniversalPostFilter(c *fiber.Ctx, tx *gorm.DB, cfg ...UniversalPostFilterConfig) (*gorm.DB, error) {
	var config UniversalPostFilterConfig
	if len(cfg) > 0 {
		config = cfg[0]
	} else {
		config = UniversalPostFilterConfig{}
	}

	timeCursor := time.Now()
	if config.TimeCursor != nil {
		timeCursor = *config.TimeCursor
	}

	if user, authenticated := c.Locals("user").(authm.Account); authenticated {
		tx = FilterPostWithUserContext(c, tx, &user)
		if c.QueryBool("noDraft", true) && !config.ShowDraft {
			tx = FilterPostDraft(tx)
			tx = FilterPostWithPublishedAt(tx, timeCursor)
		} else {
			tx = FilterPostDraftWithAuthor(database.C, user.ID)
			tx = FilterPostWithPublishedAt(tx, timeCursor, user.ID)
		}
	} else {
		tx = FilterPostWithUserContext(c, tx, nil)
		tx = FilterPostDraft(tx)
		tx = FilterPostWithPublishedAt(tx, timeCursor)
	}

	if c.QueryBool("noReply", true) && !config.ShowReply {
		tx = FilterPostReply(tx)
	}
	if c.QueryBool("noCollapse", true) && !config.ShowCollapsed {
		tx = tx.Where("is_collapsed = ? OR is_collapsed IS NULL", false)
	}

	if len(c.Query("author")) > 0 {
		var author models.Publisher
		if err := database.C.Where("name = ?", c.Query("author")).First(&author).Error; err != nil {
			return tx, fiber.NewError(fiber.StatusNotFound, err.Error())
		}
		tx = tx.Where("publisher_id = ?", author.ID)
	}

	if len(c.Query("categories")) > 0 {
		tx = FilterPostWithCategory(tx, c.Query("categories"))
	}
	if len(c.Query("tags")) > 0 {
		tx = FilterPostWithTag(tx, c.Query("tags"))
	}

	if len(c.Query("type")) > 0 {
		tx = FilterPostWithType(tx, c.Query("type"))
	}

	if len(c.Query("realm")) > 0 {
		tx = FilterPostWithRealm(tx, c.Query("realm"))
	}

	return tx, nil
}

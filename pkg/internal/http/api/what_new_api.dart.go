package api

import (
	"git.solsynth.dev/hypernet/interactive/pkg/internal/database"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/services"
	"git.solsynth.dev/hypernet/nexus/pkg/nex/sec"
	authm "git.solsynth.dev/hypernet/passport/pkg/authkit/models"
	"github.com/gofiber/fiber/v2"
)

func getWhatsNew(c *fiber.Ctx) error {
	if err := sec.EnsureAuthenticated(c); err != nil {
		return err
	}
	user := c.Locals("user").(authm.Account)

	pivot := c.QueryInt("pivot", 0)
	if pivot < 0 {
		return fiber.NewError(fiber.StatusBadRequest, "pivot must be greater than zero")
	}

	tx := services.FilterPostDraft(database.C)
	tx = services.FilterPostWithUserContext(tx, &user)

	tx = tx.Where("id > ?", pivot)

	countTx := tx
	count, err := services.CountPost(countTx)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	order := "published_at DESC"
	if c.QueryBool("featured", false) {
		order = "published_at DESC, (COALESCE(total_upvote, 0) - COALESCE(total_downvote, 0)) DESC"
	}

	items, err := services.ListPost(tx, 10, 0, order)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	return c.JSON(fiber.Map{
		"count": count,
		"data":  items,
	})
}

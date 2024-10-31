package api

import (
	"git.solsynth.dev/hydrogen/dealer/pkg/hyper"
	"git.solsynth.dev/hydrogen/interactive/pkg/internal/database"
	"git.solsynth.dev/hydrogen/interactive/pkg/internal/models"
	"git.solsynth.dev/hydrogen/interactive/pkg/internal/services"
	"github.com/gofiber/fiber/v2"
)

func listUserPinnedPost(c *fiber.Ctx) error {
	account := c.Params("account")

	var user models.Publisher
	if err := database.C.
		Where(&hyper.BaseUser{Name: account}).
		First(&user).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	tx := services.FilterPostDraft(database.C)
	tx = tx.Where("author_id = ?", user.ID)
	tx = tx.Where("pinned_at IS NOT NULL")

	items, err := services.ListPost(tx, 100, 0, "published_at DESC")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	return c.JSON(items)
}

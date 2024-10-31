package api

import (
	"git.solsynth.dev/hydrogen/interactive/pkg/internal/models"
	"git.solsynth.dev/hydrogen/interactive/pkg/internal/services"
	"github.com/gofiber/fiber/v2"
)

func getTag(c *fiber.Ctx) error {
	alias := c.Params("tag")

	tag, err := services.GetTag(alias)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, err.Error())
	}

	return c.JSON(tag)
}

func listTags(c *fiber.Ctx) error {
	take := c.QueryInt("take", 0)
	offset := c.QueryInt("offset", 0)
	probe := c.Query("probe")

	if take > 100 {
		take = 100
	}

	var tags []models.Tag
	var err error
	if len(probe) > 0 {
		tags, err = services.SearchTags(take, offset, probe)
	} else {
		tags, err = services.ListTags(take, offset)
	}
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(tags)
}

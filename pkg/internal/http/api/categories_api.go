package api

import (
	"git.solsynth.dev/hydrogen/interactive/pkg/internal/http/exts"
	"git.solsynth.dev/hydrogen/interactive/pkg/internal/services"
	"git.solsynth.dev/hypernet/nexus/pkg/nex/sec"
	"github.com/gofiber/fiber/v2"
)

func getCategory(c *fiber.Ctx) error {
	alias := c.Params("category")

	category, err := services.GetCategory(alias)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, err.Error())
	}

	return c.JSON(category)
}

func listCategories(c *fiber.Ctx) error {
	categories, err := services.ListCategory()
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, err.Error())
	}

	return c.JSON(categories)
}

func newCategory(c *fiber.Ctx) error {
	if err := sec.EnsureGrantedPerm(c, "CreatePostCategories", true); err != nil {
		return err
	}

	var data struct {
		Alias       string `json:"alias" validate:"required"`
		Name        string `json:"name" validate:"required"`
		Description string `json:"description"`
	}

	if err := exts.BindAndValidate(c, &data); err != nil {
		return err
	}

	category, err := services.NewCategory(data.Alias, data.Name, data.Description)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	return c.JSON(category)
}

func editCategory(c *fiber.Ctx) error {
	if err := sec.EnsureGrantedPerm(c, "CreatePostCategories", true); err != nil {
		return err
	}

	id, _ := c.ParamsInt("categoryId", 0)
	category, err := services.GetCategoryWithID(uint(id))
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, err.Error())
	}

	var data struct {
		Alias       string `json:"alias" validate:"required"`
		Name        string `json:"name" validate:"required"`
		Description string `json:"description"`
	}

	if err := exts.BindAndValidate(c, &data); err != nil {
		return err
	}

	category, err = services.EditCategory(category, data.Alias, data.Name, data.Description)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	return c.JSON(category)
}

func deleteCategory(c *fiber.Ctx) error {
	if err := sec.EnsureGrantedPerm(c, "CreatePostCategories", true); err != nil {
		return err
	}

	id, _ := c.ParamsInt("categoryId", 0)
	category, err := services.GetCategoryWithID(uint(id))
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, err.Error())
	}

	if err := services.DeleteCategory(category); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	return c.JSON(category)
}

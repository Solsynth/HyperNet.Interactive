package api

import (
	"fmt"
	"git.solsynth.dev/hydrogen/interactive/pkg/internal/database"
	"git.solsynth.dev/hydrogen/interactive/pkg/internal/models"
	"git.solsynth.dev/hydrogen/interactive/pkg/internal/services"
	"github.com/gofiber/fiber/v2"
)

func listPostReplies(c *fiber.Ctx) error {
	take := c.QueryInt("take", 0)
	offset := c.QueryInt("offset", 0)

	tx := database.C
	var post models.Post
	if err := database.C.Where("id = ?", c.Params("postId")).First(&post).Error; err != nil {
		return fiber.NewError(fiber.StatusBadRequest, fmt.Sprintf("unable to find post: %v", err))
	} else {
		tx = services.FilterPostReply(tx, post.ID)
	}

	if len(c.Query("author")) > 0 {
		var author models.Publisher
		if err := database.C.Where("name = ?", c.Query("author")).First(&author).Error; err != nil {
			return fiber.NewError(fiber.StatusNotFound, err.Error())
		}
		tx = tx.Where("author_id = ?", author.ID)
	}

	if len(c.Query("category")) > 0 {
		tx = services.FilterPostWithCategory(tx, c.Query("category"))
	}
	if len(c.Query("tag")) > 0 {
		tx = services.FilterPostWithTag(tx, c.Query("tag"))
	}

	count, err := services.CountPost(tx)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	items, err := services.ListPost(tx, take, offset, "published_at DESC")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	return c.JSON(fiber.Map{
		"count": count,
		"data":  items,
	})
}

func listPostFeaturedReply(c *fiber.Ctx) error {
	take := c.QueryInt("take", 0)
	take = max(1, min(take, 3))

	tx := database.C
	var post models.Post
	if err := database.C.Where("id = ?", c.Params("postId")).First(&post).Error; err != nil {
		return fiber.NewError(fiber.StatusBadRequest, fmt.Sprintf("unable to find post: %v", err))
	} else {
		tx = services.FilterPostReply(tx, post.ID)
	}

	if len(c.Query("author")) > 0 {
		var author models.Publisher
		if err := database.C.Where("name = ?", c.Query("author")).First(&author).Error; err != nil {
			return fiber.NewError(fiber.StatusNotFound, err.Error())
		}
		tx = tx.Where("author_id = ?", author.ID)
	}

	if len(c.Query("category")) > 0 {
		tx = services.FilterPostWithCategory(tx, c.Query("category"))
	}
	if len(c.Query("tag")) > 0 {
		tx = services.FilterPostWithTag(tx, c.Query("tag"))
	}

	items, err := services.ListPost(tx, take, 0, "(COALESCE(total_upvote, 0) - COALESCE(total_downvote, 0)) DESC, published_at DESC")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	return c.JSON(items)
}

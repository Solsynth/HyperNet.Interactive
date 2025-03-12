package api

import (
	"time"

	"git.solsynth.dev/hypernet/interactive/pkg/internal/database"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/models"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/services"
	authm "git.solsynth.dev/hypernet/passport/pkg/authkit/models"
	"github.com/gofiber/fiber/v2"
	"github.com/samber/lo"
)

func listRecommendation(c *fiber.Ctx) error {
	const featuredMax = 5

	posts, err := services.GetFeaturedPosts(featuredMax)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	postIdx := lo.Map(posts, func(item models.Post, index int) uint {
		return item.ID
	})

	var userId *uint
	if user, authenticated := c.Locals("user").(authm.Account); authenticated {
		userId = &user.ID
	}

	tx := database.C.Where("id IN ?", postIdx)
	newPosts, err := services.ListPost(tx, featuredMax, 0, "id ASC", userId)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	newPostMap := lo.SliceToMap(newPosts, func(item *models.Post) (uint, models.Post) {
		return item.ID, *item
	})

	// Revert the position
	for idx, item := range posts {
		posts[idx] = newPostMap[item.ID]
	}

	return c.JSON(posts)
}

func listRecommendationShuffle(c *fiber.Ctx) error {
	take := c.QueryInt("take", 0)
	offset := c.QueryInt("offset", 0)

	tx := database.C

	var err error
	if tx, err = services.UniversalPostFilter(c, tx); err != nil {
		return err
	}

	var userId *uint
	if user, authenticated := c.Locals("user").(authm.Account); authenticated {
		userId = &user.ID
	}

	countTx := tx
	count, err := services.CountPost(countTx)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	items, err := services.ListPost(tx, take, offset, "RANDOM()", userId)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	if c.QueryBool("truncate", true) {
		for _, item := range items {
			if item != nil {
				item = lo.ToPtr(services.TruncatePostContent(*item))
			}
		}
	}

	return c.JSON(fiber.Map{
		"count": count,
		"data":  items,
	})
}

func getRecommendationFeed(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 20)
	cursor := c.QueryInt("cursor", 0)

	var cursorTime *time.Time
	if cursor > 0 {
		cursorTime = lo.ToPtr(time.Unix(int64(cursor), 0))
	}

	var userId *uint
	if user, authenticated := c.Locals("user").(authm.Account); authenticated {
		userId = &user.ID
	}

	entries, err := services.GetFeed(c, limit, userId, cursorTime)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(entries)
}

package api

import (
	"context"
	"strconv"
	"strings"

	localCache "git.solsynth.dev/hypernet/interactive/pkg/internal/cache"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/database"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/models"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/services"
	"git.solsynth.dev/hypernet/nexus/pkg/nex/sec"
	authm "git.solsynth.dev/hypernet/passport/pkg/authkit/models"
	"github.com/eko/gocache/lib/v4/cache"
	"github.com/eko/gocache/lib/v4/marshaler"
	"github.com/gofiber/fiber/v2"
)

func getPostInsight(c *fiber.Ctx) error {
	if err := sec.EnsureAuthenticated(c); err != nil {
		return err
	}
	user := c.Locals("user").(authm.Account)

	id := c.Params("postId")

	var item models.Post
	var err error

	tx := services.FilterPostDraft(database.C)

	if user, authenticated := c.Locals("user").(authm.Account); authenticated {
		tx = services.FilterPostWithUserContext(tx, &user)
	} else {
		tx = services.FilterPostWithUserContext(tx, nil)
	}

	if numericId, paramErr := strconv.Atoi(id); paramErr == nil {
		item, err = services.GetPost(tx, uint(numericId))
	} else {
		segments := strings.Split(id, ":")
		if len(segments) != 2 {
			return fiber.NewError(fiber.StatusBadRequest, "invalid post id, must be a number or a string with two segment divided by a colon")
		}
		area := segments[0]
		alias := segments[1]
		item, err = services.GetPostByAlias(tx, alias, area)
	}

	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, err.Error())
	}

	cacheManager := cache.New[any](localCache.S)
	marshal := marshaler.New(cacheManager)
	contx := context.Background()

	var response string
	if val, err := marshal.Get(contx, services.GetPostInsightCacheKey(item.ID), new(string)); err == nil {
		response = *(val.(*string))
	} else {
		response, err = services.GeneratePostInsights(item, user.ID)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, err.Error())
		}
		marshal.Set(contx, services.GetPostInsightCacheKey(item.ID), response)
	}

	return c.JSON(fiber.Map{
		"response": response,
	})
}

package api

import (
	"fmt"
	"strconv"
	"strings"

	"git.solsynth.dev/hypernet/nexus/pkg/nex/cruda"
	"git.solsynth.dev/hypernet/nexus/pkg/nex/sec"
	"git.solsynth.dev/hypernet/passport/pkg/authkit"
	authm "git.solsynth.dev/hypernet/passport/pkg/authkit/models"

	"git.solsynth.dev/hypernet/interactive/pkg/internal/database"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/gap"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/http/exts"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/models"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/services"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/services/queries"
	"github.com/gofiber/fiber/v2"
	"github.com/samber/lo"
)

func getPost(c *fiber.Ctx) error {
	id := c.Params("postId")

	var item models.Post
	var err error

	var userId *uint
	if user, authenticated := c.Locals("user").(authm.Account); authenticated {
		userId = &user.ID
	}

	tx := database.C
	if tx, err = services.UniversalPostFilter(c, tx, services.UniversalPostFilterConfig{
		ShowReply:     true,
		ShowDraft:     true,
		ShowCollapsed: true,
	}); err != nil {
		return err
	}

	if numericId, paramErr := strconv.Atoi(id); paramErr == nil {
		if c.Get("X-API-Version", "1") == "2" {
			item, err = queries.GetPost(tx, uint(numericId), userId)
		} else {
			item, err = services.GetPost(tx, uint(numericId))
		}
	} else {
		segments := strings.Split(id, ":")
		if len(segments) != 2 {
			return fiber.NewError(fiber.StatusBadRequest, "invalid post id, must be a number or a string with two segment divided by a colon")
		}
		area := segments[0]
		alias := segments[1]
		if c.Get("X-API-Version", "1") == "2" {
			item, err = queries.GetPostByAlias(tx, alias, area, userId)
		} else {
			item, err = services.GetPostByAlias(tx, alias, area)
		}
	}

	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, err.Error())
	}

	item.Metric = models.PostMetric{
		ReplyCount:    services.CountPostReply(item.ID),
		ReactionCount: services.CountPostReactions(item.ID),
	}
	item.Metric.ReactionList, err = services.ListPostReactions(database.C.Where("post_id = ?", item.ID))
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(item)
}

func searchPost(c *fiber.Ctx) error {
	take := c.QueryInt("take", 10)
	offset := c.QueryInt("offset", 0)

	tx := database.C

	probe := c.Query("probe")
	if len(probe) == 0 && len(c.Query("tags")) == 0 && len(c.Query("categories")) == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "search term (probe, tags or categories) is required")
	}

	tx = services.FilterPostWithFuzzySearch(tx, probe)

	var err error
	if tx, err = services.UniversalPostFilter(c, tx, services.UniversalPostFilterConfig{
		ShowReply: true,
	}); err != nil {
		return err
	}

	var userId *uint
	if user, authenticated := c.Locals("user").(authm.Account); authenticated {
		userId = &user.ID
	}

	var count int64
	countTx := tx
	count, err = services.CountPost(countTx)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	var items []models.Post

	if c.Get("X-API-Version", "1") == "2" {
		items, err = queries.ListPost(tx, take, offset, "published_at DESC", userId)
	} else {
		items, err = services.ListPost(tx, take, offset, "published_at DESC", userId)
	}
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	if c.QueryBool("truncate", true) {
		for _, item := range items {
			item = services.TruncatePostContent(item)
		}
	}

	return c.JSON(fiber.Map{
		"count": count,
		"data":  items,
	})
}

func listPost(c *fiber.Ctx) error {
	take := c.QueryInt("take", 10)
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

	var count int64
	countTx := tx
	count, err = services.CountPost(countTx)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	var items []models.Post

	if c.Get("X-API-Version", "1") == "2" {
		items, err = queries.ListPost(tx, take, offset, "published_at DESC", userId)
	} else {
		items, err = services.ListPost(tx, take, offset, "published_at DESC", userId)
	}
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	if c.QueryBool("truncate", true) {
		for _, item := range items {
			item = services.TruncatePostContent(item)
		}
	}

	return c.JSON(fiber.Map{
		"count": count,
		"data":  items,
	})
}

func listPostMinimal(c *fiber.Ctx) error {
	take := c.QueryInt("take", 10)
	offset := c.QueryInt("offset", 0)

	tx := database.C

	var err error
	if tx, err = services.UniversalPostFilter(c, tx); err != nil {
		return err
	}

	countTx := tx
	count, err := services.CountPost(countTx)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	items, err := services.ListPostMinimal(tx, take, offset, "published_at DESC")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	if c.QueryBool("truncate", false) {
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

func listDraftPost(c *fiber.Ctx) error {
	take := c.QueryInt("take", 10)
	offset := c.QueryInt("offset", 0)

	if err := sec.EnsureAuthenticated(c); err != nil {
		return err
	}
	user := c.Locals("user").(authm.Account)

	var err error
	tx := services.FilterPostWithAuthorDraft(database.C, user.ID)

	var userId *uint
	if user, authenticated := c.Locals("user").(authm.Account); authenticated {
		userId = &user.ID
	}

	var count int64
	countTx := tx
	count, err = services.CountPost(countTx)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	var items []models.Post

	if c.Get("X-API-Version", "1") == "2" {
		items, err = queries.ListPost(tx, take, offset, "published_at DESC", userId)
	} else {
		items, err = services.ListPost(tx, take, offset, "published_at DESC", userId)
	}
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	if c.QueryBool("truncate", true) {
		for _, item := range items {
			item = services.TruncatePostContent(item)
		}
	}

	return c.JSON(fiber.Map{
		"count": count,
		"data":  items,
	})
}

func deletePost(c *fiber.Ctx) error {
	if err := sec.EnsureAuthenticated(c); err != nil {
		return err
	}
	user := c.Locals("user").(authm.Account)
	id, _ := c.ParamsInt("postId", 0)

	publisherId := c.QueryInt("publisherId", 0)
	if publisherId <= 0 {
		return fiber.NewError(fiber.StatusBadRequest, "missing publisher id in request")
	}

	publisher, err := services.GetPublisher(uint(publisherId), user.ID)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	var item models.Post
	if err := database.C.Where(models.Post{
		BaseModel:   cruda.BaseModel{ID: uint(id)},
		PublisherID: publisher.ID,
	}).Preload("Publisher").First(&item).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, err.Error())
	}

	if err := services.DeletePost(item); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	} else {
		_ = authkit.AddEventExt(
			gap.Nx,
			"posts.delete",
			map[string]any{"post": item},
			c,
		)
	}

	return c.SendStatus(fiber.StatusOK)
}

func reactPost(c *fiber.Ctx) error {
	if err := sec.EnsureGrantedPerm(c, "CreateReactions", true); err != nil {
		return err
	}
	user := c.Locals("user").(authm.Account)

	var data struct {
		Symbol   string                  `json:"symbol"`
		Attitude models.ReactionAttitude `json:"attitude"`
	}

	if err := exts.BindAndValidate(c, &data); err != nil {
		return err
	}

	reaction := models.Reaction{
		Symbol:    data.Symbol,
		Attitude:  data.Attitude,
		AccountID: user.ID,
	}

	var res models.Post
	if err := database.C.Where("id = ?", c.Params("postId")).Select("id").First(&res).Error; err != nil {
		return fiber.NewError(fiber.StatusBadRequest, fmt.Sprintf("unable to find post to react: %v", err))
	} else {
		reaction.PostID = res.ID
	}

	if positive, reaction, err := services.ReactPost(user, reaction); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	} else {
		_ = authkit.AddEventExt(
			gap.Nx,
			"posts.react",
			map[string]any{"post_id": res.ID, "reaction": reaction},
			c,
		)

		return c.Status(lo.Ternary(positive, fiber.StatusCreated, fiber.StatusNoContent)).JSON(reaction)
	}
}

func pinPost(c *fiber.Ctx) error {
	if err := sec.EnsureAuthenticated(c); err != nil {
		return err
	}
	user := c.Locals("user").(authm.Account)

	var res models.Post
	if err := database.C.Where("id = ? AND publisher_id = ?", c.Params("postId"), user.ID).First(&res).Error; err != nil {
		return fiber.NewError(fiber.StatusBadRequest, fmt.Sprintf("unable to find post in your posts to pin: %v", err))
	}

	if status, err := services.PinPost(res); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	} else if status {
		_ = authkit.AddEventExt(
			gap.Nx,
			"posts.pin",
			map[string]any{"post": res},
			c,
		)
		return c.SendStatus(fiber.StatusOK)
	} else {
		_ = authkit.AddEventExt(
			gap.Nx,
			"posts.unpin",
			map[string]any{"post": res},
			c,
		)
		return c.SendStatus(fiber.StatusNoContent)
	}
}

func uncollapsePost(c *fiber.Ctx) error {
	id, _ := c.ParamsInt("postId", 0)

	if err := sec.EnsureGrantedPerm(c, "UncollapsePosts", true); err != nil {
		return err
	}

	if err := database.C.Model(&models.Post{}).Where("id = ?", id).Update("is_collapsed", false).Error; err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	return c.SendStatus(fiber.StatusOK)
}

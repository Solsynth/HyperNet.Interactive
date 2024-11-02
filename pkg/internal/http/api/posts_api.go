package api

import (
	"fmt"
	"git.solsynth.dev/hypernet/nexus/pkg/nex/cruda"
	"git.solsynth.dev/hypernet/nexus/pkg/nex/sec"
	"git.solsynth.dev/hypernet/passport/pkg/authkit"
	authm "git.solsynth.dev/hypernet/passport/pkg/authkit/models"
	"gorm.io/gorm"
	"strconv"
	"strings"

	"git.solsynth.dev/hypernet/interactive/pkg/internal/database"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/gap"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/http/exts"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/models"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/samber/lo"
)

func universalPostFilter(c *fiber.Ctx, tx *gorm.DB) (*gorm.DB, error) {
	tx = services.FilterPostDraft(tx)

	if user, authenticated := c.Locals("user").(authm.Account); authenticated {
		tx = services.FilterPostWithUserContext(tx, &user)
	} else {
		tx = services.FilterPostWithUserContext(tx, nil)
	}

	if c.QueryBool("noReply", true) {
		tx = services.FilterPostReply(tx)
	}

	if len(c.Query("author")) > 0 {
		var author models.Publisher
		if err := database.C.Where("name = ?", c.Query("author")).First(&author).Error; err != nil {
			return tx, fiber.NewError(fiber.StatusNotFound, err.Error())
		}
		tx = tx.Where("publisher_id = ?", author.ID)
	}

	if len(c.Query("category")) > 0 {
		tx = services.FilterPostWithCategory(tx, c.Query("category"))
	}
	if len(c.Query("tag")) > 0 {
		tx = services.FilterPostWithTag(tx, c.Query("tag"))
	}

	return tx, nil
}

func getPost(c *fiber.Ctx) error {
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
	take := c.QueryInt("take", 0)
	offset := c.QueryInt("offset", 0)

	tx := database.C

	probe := c.Query("probe")
	if len(probe) == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "probe is required")
	}

	tx = services.FilterPostWithFuzzySearch(tx, probe)

	var err error
	if tx, err = universalPostFilter(c, tx); err != nil {
		return err
	}

	countTx := tx
	count, err := services.CountPost(countTx)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	items, err := services.ListPost(tx, take, offset, "published_at DESC")
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

func listPost(c *fiber.Ctx) error {
	take := c.QueryInt("take", 0)
	offset := c.QueryInt("offset", 0)

	tx := database.C

	var err error
	if tx, err = universalPostFilter(c, tx); err != nil {
		return err
	}

	countTx := tx
	count, err := services.CountPost(countTx)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	items, err := services.ListPost(tx, take, offset, "published_at DESC")
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

func listPostMinimal(c *fiber.Ctx) error {
	take := c.QueryInt("take", 0)
	offset := c.QueryInt("offset", 0)

	tx := database.C

	var err error
	if tx, err = universalPostFilter(c, tx); err != nil {
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
	take := c.QueryInt("take", 0)
	offset := c.QueryInt("offset", 0)

	if err := sec.EnsureAuthenticated(c); err != nil {
		return err
	}
	user := c.Locals("user").(authm.Account)

	tx := services.FilterPostWithAuthorDraft(database.C, user.ID)

	count, err := services.CountPost(tx)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	items, err := services.ListPost(tx, take, offset, "created_at DESC", true)
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
	}).First(&item).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, err.Error())
	}

	if err := services.DeletePost(item); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	} else {
		_ = authkit.AddEventExt(
			gap.Nx,
			"posts.delete",
			strconv.Itoa(int(item.ID)),
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
			strconv.Itoa(int(res.ID)),
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
			strconv.Itoa(int(res.ID)),
			c,
		)
		return c.SendStatus(fiber.StatusOK)
	} else {
		_ = authkit.AddEventExt(
			gap.Nx,
			"posts.unpin",
			strconv.Itoa(int(res.ID)),
			c,
		)
		return c.SendStatus(fiber.StatusNoContent)
	}
}

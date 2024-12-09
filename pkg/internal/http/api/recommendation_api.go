package api

import (
	"git.solsynth.dev/hypernet/interactive/pkg/internal/database"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/gap"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/models"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/services"
	"git.solsynth.dev/hypernet/nexus/pkg/nex/sec"
	"git.solsynth.dev/hypernet/nexus/pkg/proto"
	"git.solsynth.dev/hypernet/passport/pkg/authkit"
	authm "git.solsynth.dev/hypernet/passport/pkg/authkit/models"
	"github.com/gofiber/fiber/v2"
	"github.com/samber/lo"
)

func listRecommendation(c *fiber.Ctx) error {
	const featuredMax = 3

	posts, err := services.GetFeaturedPosts(featuredMax)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	postIdx := lo.Map(posts, func(item models.Post, index int) uint {
		return item.ID
	})

	tx := database.C.Where("id IN ?", postIdx)
	newPosts, err := services.ListPost(tx, featuredMax, 0, " DESC")
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

func listRecommendationFriends(c *fiber.Ctx) error {
	if err := sec.EnsureAuthenticated(c); err != nil {
		return err
	}
	user := c.Locals("user").(authm.Account)

	take := c.QueryInt("take", 0)
	offset := c.QueryInt("offset", 0)

	tx := database.C

	var err error
	if tx, err = universalPostFilter(c, tx); err != nil {
		return err
	}

	friends, _ := authkit.ListRelative(gap.Nx, user.ID, int32(authm.RelationshipFriend), true)
	friendList := lo.Map(friends, func(item *proto.UserInfo, index int) uint {
		return uint(item.GetId())
	})

	tx = tx.Where("publisher_id IN ?", friendList)

	countTx := tx
	count, err := services.CountPost(countTx)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	order := "published_at DESC"
	if c.QueryBool("featured", false) {
		order = "published_at DESC, (COALESCE(total_upvote, 0) - COALESCE(total_downvote, 0)) DESC"
	}

	items, err := services.ListPost(tx, take, offset, order)
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

func listRecommendationShuffle(c *fiber.Ctx) error {
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

	items, err := services.ListPost(tx, take, offset, "RANDOM()")
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

package api

import (
	"fmt"

	"git.solsynth.dev/hypernet/interactive/pkg/internal/database"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/gap"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/http/exts"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/models"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/services"
	"git.solsynth.dev/hypernet/nexus/pkg/nex/sec"
	"git.solsynth.dev/hypernet/passport/pkg/authkit"
	authm "git.solsynth.dev/hypernet/passport/pkg/authkit/models"
	"github.com/gofiber/fiber/v2"
)

func listPinnedPost(c *fiber.Ctx) error {
	name := c.Params("name")

	var user models.Publisher
	if err := database.C.
		Where("name = ?", name).
		First(&user).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	tx := services.FilterPostDraft(database.C)
	tx = tx.Where("publisher_id = ?", user.ID)
	tx = tx.Where("pinned_at IS NOT NULL")

	var userId *uint
	if user, authenticated := c.Locals("user").(authm.Account); authenticated {
		userId = &user.ID
	}

	items, err := services.ListPost(tx, 100, 0, "published_at DESC", userId)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	return c.JSON(items)
}

func getPublisher(c *fiber.Ctx) error {
	name := c.Params("name")

	var publisher models.Publisher
	if err := database.C.Where("name = ?", name).First(&publisher).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, err.Error())
	}

	return c.JSON(publisher)
}

func listRelatedPublisher(c *fiber.Ctx) error {
	tx := database.C
	if len(c.Query("user")) > 0 {
		user, err := authkit.GetUserByName(gap.Nx, c.Query("user"))
		if err != nil {
			return fiber.NewError(fiber.StatusNotFound, fmt.Sprintf("unable to find user: %v", err))
		}
		tx = tx.Where("account_id = ? AND type = ?", user.ID, models.PublisherTypePersonal)
	} else if len(c.Query("realm")) > 0 {
		realm, err := authkit.GetRealmByAlias(gap.Nx, c.Query("realm"))
		if err != nil {
			return fiber.NewError(fiber.StatusNotFound, fmt.Sprintf("unable to find realm: %v", err))
		}
		tx = tx.Where("realm_id = ? AND type = ?", realm.ID, models.PublisherTypeOrganization)
	} else {
		return fiber.NewError(fiber.StatusBadRequest, "missing user or realm in query string")
	}

	var publishers []models.Publisher
	if err := tx.Find(&publishers).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, err.Error())
	}

	return c.JSON(publishers)
}

func listOwnedPublisher(c *fiber.Ctx) error {
	if err := sec.EnsureAuthenticated(c); err != nil {
		return err
	}
	user := c.Locals("user").(authm.Account)

	var publishers []models.Publisher
	if err := database.C.Where("account_id = ?", user.ID).Find(&publishers).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, err.Error())
	}

	return c.JSON(publishers)
}

func createPersonalPublisher(c *fiber.Ctx) error {
	if err := sec.EnsureGrantedPerm(c, "CreatePublishers", true); err != nil {
		return err
	}
	user := c.Locals("user").(authm.Account)

	var data struct {
		Name        string `json:"name" validate:"required,min=4,max=32,alphanum"`
		Nick        string `json:"nick" validate:"required,min=2,max=64"`
		Description string `json:"description"`
		Avatar      string `json:"avatar"`
		Banner      string `json:"banner"`
	}

	if err := exts.BindAndValidate(c, &data); err != nil {
		return err
	}

	if pub, err := services.CreatePersonalPublisher(
		user,
		data.Name,
		data.Nick,
		data.Description,
		data.Avatar,
		data.Banner,
	); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	} else {
		return c.JSON(pub)
	}
}

func createOrganizationPublisher(c *fiber.Ctx) error {
	if err := sec.EnsureGrantedPerm(c, "CreatePublishers", true); err != nil {
		return err
	}
	user := c.Locals("user").(authm.Account)

	var data struct {
		Realm       string `json:"realm" validate:"required"`
		Name        string `json:"name" validate:"required,min=4,max=32,alphanum"`
		Nick        string `json:"nick" validate:"required,min=2,max=64"`
		Description string `json:"description"`
		Avatar      string `json:"avatar"`
		Banner      string `json:"banner"`
	}

	if err := exts.BindAndValidate(c, &data); err != nil {
		return err
	}

	realm, err := authkit.GetRealmByAlias(gap.Nx, data.Realm)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, fmt.Sprintf("unable to get realm: %v", err))
	}
	if !authkit.CheckRealmMemberPerm(gap.Nx, realm.ID, int(user.ID), 100) {
		return fiber.NewError(fiber.StatusForbidden, "you least need to be the admin of this realm to create a publisher")
	}

	if pub, err := services.CreateOrganizationPublisher(
		user,
		realm,
		data.Name,
		data.Nick,
		data.Description,
		data.Avatar,
		data.Banner,
	); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	} else {
		return c.JSON(pub)
	}
}

func editPublisher(c *fiber.Ctx) error {
	if err := sec.EnsureAuthenticated(c); err != nil {
		return err
	}
	user := c.Locals("user").(authm.Account)

	name := c.Params("name")
	publisher, err := services.GetPublisherByName(name, user.ID)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, err.Error())
	}

	var data struct {
		Name        string `json:"name"`
		Nick        string `json:"nick"`
		Description string `json:"description"`
		Avatar      string `json:"avatar"`
		Banner      string `json:"banner"`
		AccountID   *uint  `json:"account_id"`
	}

	if err := exts.BindAndValidate(c, &data); err != nil {
		return err
	}

	og := publisher
	publisher.Name = data.Name
	publisher.Nick = data.Nick
	publisher.Description = data.Description
	publisher.Avatar = data.Avatar
	publisher.Banner = data.Banner
	if data.AccountID != nil {
		publisher.AccountID = data.AccountID
	}

	if publisher, err = services.EditPublisher(user, publisher, og); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	return c.JSON(publisher)
}

func deletePublisher(c *fiber.Ctx) error {
	if err := sec.EnsureAuthenticated(c); err != nil {
		return err
	}
	user := c.Locals("user").(authm.Account)

	name := c.Params("name")
	publisher, err := services.GetPublisherByName(name, user.ID)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, err.Error())
	}

	if err := services.DeletePublisher(publisher); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	return c.SendStatus(fiber.StatusOK)
}

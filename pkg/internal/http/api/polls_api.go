package api

import (
	"time"

	"git.solsynth.dev/hypernet/interactive/pkg/internal/database"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/http/exts"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/models"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/services"
	"git.solsynth.dev/hypernet/nexus/pkg/nex/sec"
	authm "git.solsynth.dev/hypernet/passport/pkg/authkit/models"
	"github.com/gofiber/fiber/v2"
)

func getMyPollAnswer(c *fiber.Ctx) error {
	if err := sec.EnsureAuthenticated(c); err != nil {
		return err
	}
	user := c.Locals("user").(authm.Account)

	pollId, _ := c.ParamsInt("pollId")

	var answer models.PollAnswer
	if err := database.C.Where("poll_id = ? AND account_id = ?", pollId, user.ID).First(&answer).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, err.Error())
	}

	return c.JSON(answer)
}

func getPoll(c *fiber.Ctx) error {
	pollId, _ := c.ParamsInt("pollId")

	var poll models.Poll
	if err := database.C.Where("id = ?", pollId).First(&poll).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, err.Error())
	}

	poll.Metric = services.GetPollMetric(poll)

	return c.JSON(poll)
}

func createPoll(c *fiber.Ctx) error {
	if err := sec.EnsureAuthenticated(c); err != nil {
		return err
	}
	user := c.Locals("user").(authm.Account)

	var data struct {
		Options   []models.PollOption `json:"options" validate:"required"`
		ExpiredAt *time.Time          `json:"expired_at"`
	}

	if err := exts.BindAndValidate(c, &data); err != nil {
		return err
	}

	poll := models.Poll{
		ExpiredAt: data.ExpiredAt,
		Options:   data.Options,
		AccountID: user.ID,
	}

	var err error
	if poll, err = services.NewPoll(poll); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	return c.JSON(poll)
}

func updatePoll(c *fiber.Ctx) error {
	pollId, _ := c.ParamsInt("pollId")

	if err := sec.EnsureAuthenticated(c); err != nil {
		return err
	}
	user := c.Locals("user").(authm.Account)

	var data struct {
		Options   []models.PollOption `json:"options" validate:"required"`
		ExpiredAt *time.Time          `json:"expired_at"`
	}

	if err := exts.BindAndValidate(c, &data); err != nil {
		return err
	}

	var poll models.Poll
	if err := database.C.Where("id = ? AND account_id = ?", pollId, user.ID).First(&poll).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, err.Error())
	}

	poll.Options = data.Options
	poll.ExpiredAt = data.ExpiredAt

	var err error
	if poll, err = services.UpdatePoll(poll); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	return c.JSON(poll)
}

func deletePoll(c *fiber.Ctx) error {
	pollId, _ := c.ParamsInt("pollId")

	if err := sec.EnsureAuthenticated(c); err != nil {
		return err
	}
	user := c.Locals("user").(authm.Account)

	var poll models.Poll
	if err := database.C.Where("id = ? AND account_id = ?", pollId, user.ID).First(&poll).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, err.Error())
	}

	if err := database.C.Delete(&poll).Error; err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	return c.JSON(poll)
}

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

func answerPoll(c *fiber.Ctx) error {
	pollId, _ := c.ParamsInt("pollId")

	if err := sec.EnsureAuthenticated(c); err != nil {
		return err
	}
	user := c.Locals("user").(authm.Account)

	var data struct {
		Answer string `json:"answer" validate:"required"`
	}

	if err := exts.BindAndValidate(c, &data); err != nil {
		return err
	}

	var poll models.Poll
	if err := database.C.Where("id = ?", pollId).First(&poll).Error; err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	if poll.ExpiredAt != nil && time.Now().Unix() >= poll.ExpiredAt.Unix() {
		return fiber.NewError(fiber.StatusBadRequest, "poll has been ended")
	}

	doesContains := false
	for _, option := range poll.Options {
		if option.ID == data.Answer {
			doesContains = true
			break
		}
	}
	if !doesContains {
		return fiber.NewError(fiber.StatusBadRequest, "poll does not have a option like that")
	}

	answer := models.PollAnswer{
		Answer:    data.Answer,
		PollID:    poll.ID,
		AccountID: user.ID,
	}

	if answer, err := services.AddPollAnswer(poll, answer); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	} else {
		return c.JSON(answer)
	}
}

package admin

import (
	"git.solsynth.dev/hypernet/interactive/pkg/internal/services"
	"git.solsynth.dev/hypernet/nexus/pkg/nex/sec"
	"github.com/gofiber/fiber/v2"
)

func adminTriggerFediverseFetch(c *fiber.Ctx) error {
	if err := sec.EnsureGrantedPerm(c, "AdminTriggerFediverseFetch", true); err != nil {
		return err
	}

	go services.FetchFediverseTimedTask()

	return c.SendStatus(fiber.StatusOK)
}

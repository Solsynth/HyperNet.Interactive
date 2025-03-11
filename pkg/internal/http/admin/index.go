package admin

import "github.com/gofiber/fiber/v2"

func MapControllers(app *fiber.App, baseURL string) {
	admin := app.Group(baseURL)
	{
		admin.Post("/fediverse", adminTriggerFediverseFetch)
	}
}

package api

import (
	"git.solsynth.dev/hypernet/interactive/pkg/internal/database"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/models"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/services"
	"github.com/gofiber/fiber/v2"
)

type WebFingerResponse struct {
	Subject string   `json:"subject"`
	Aliases []string `json:"aliases"`
	Links   []struct {
		Rel  string `json:"rel"`
		Type string `json:"type,omitempty"`
		Href string `json:"href"`
	} `json:"links"`
}

// Although this webfinger is desgined for users
// But in this case we will provide publisher for them
func getWebfinger(c *fiber.Ctx) error {
	resource := c.Query("resource")

	if len(resource) < 6 || resource[:5] != "acct:" {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid resource format"})
	}

	username := resource[5:]
	if username == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid username"})
	}

	var publisher models.Publisher
	if err := database.C.Where("name = ?", username).First(&publisher).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, err.Error())
	}

	response := WebFingerResponse{
		Subject: "acct:" + username,
		Aliases: []string{
			services.GetActivityID("/users/" + publisher.Name).String(),
		},
		Links: []struct {
			Rel  string `json:"rel"`
			Type string `json:"type,omitempty"`
			Href string `json:"href"`
		}{
			{
				Rel:  "self",
				Type: "application/activity+json",
				Href: services.GetActivityID("/users/" + publisher.Name).String(),
			},
			// TODO Add avatar here
		},
	}

	return c.JSON(response)
}

package api

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"git.solsynth.dev/hypernet/interactive/pkg/internal/database"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/models"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/services"
	"github.com/go-ap/activitypub"
	"github.com/gofiber/fiber/v2"
	"github.com/samber/lo"
)

func apUserInbox(c *fiber.Ctx) error {
	name := c.Params("name")

	var activity activitypub.Activity
	if err := c.BodyParser(&activity); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid activitypub event")
	}

	// TODO Handle all these
	switch activity.Type {
	case activitypub.LikeType:
		log.Printf("User %s received a Like on: %s", name, activity.Object.GetID())
	case activitypub.FollowType:
		log.Printf("User %s received a Follow request from: %s", name, activity.Actor.GetID())
	case activitypub.CreateType:
		log.Printf("New post received for %s: %s", name, activity.Object.GetID())
	default:
		log.Printf("Unhandled activity type received: %+v", activity)
	}

	return c.Status(http.StatusAccepted).SendString("Activity received")
}

func apUserOutbox(c *fiber.Ctx) error {
	name := c.Params("name")

	var publisher models.Publisher
	if err := database.C.Where("name = ?", name).First(&publisher).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, err.Error())
	}

	take := 50
	tx, err := UniversalPostFilter(c, database.C)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	var activities []activitypub.Item
	if posts, err := services.ListPost(tx, take, 0, "published_at DESC", nil); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	} else {
		for _, post := range posts {
			if post == nil {
				continue
			}
			var content string
			if val, ok := post.Body["content"].(string); ok {
				content = val
			} else {
				content = "Posted a post"
			}
			note := activitypub.Note{
				ID:           services.GetActivityID("/posts/" + strconv.Itoa(int(post.ID))),
				Type:         activitypub.NoteType,
				Attachment:   nil,
				AttributedTo: services.GetActivityIRI("/users/" + publisher.Name),
				Published: lo.TernaryF(post.PublishedAt == nil, func() time.Time {
					return post.CreatedAt
				}, func() time.Time {
					return *post.PublishedAt
				}),
				Updated: lo.TernaryF(post.EditedAt == nil, func() time.Time {
					return post.UpdatedAt
				}, func() time.Time {
					return *post.EditedAt
				}),
				To:      activitypub.ItemCollection{activitypub.PublicNS},
				Content: activitypub.DefaultNaturalLanguageValue(content),
			}
			activity := activitypub.Create{
				ID:     services.GetActivityID("/activities/posts/" + strconv.Itoa(int(post.ID))),
				Type:   activitypub.CreateType,
				Actor:  services.GetActivityIRI("/users/" + publisher.Name),
				Object: note,
			}
			activities = append(activities, activity)
		}
	}

	outbox := activitypub.OrderedCollection{
		ID:           services.GetActivityID("/users/" + publisher.Name + "/outbox"),
		Type:         activitypub.OrderedCollectionType,
		TotalItems:   uint(min(take, len(activities))),
		OrderedItems: activitypub.ItemCollection(activities),
	}

	return c.JSON(outbox)
}

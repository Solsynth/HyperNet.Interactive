package api

import (
	"github.com/gofiber/fiber/v2"
)

func MapControllers(app *fiber.App, baseURL string) {
	api := app.Group(baseURL).Name("API")
	{
		api.Get("/webfinger", getWebfinger)
		activitypub := api.Group("/activitypub").Name("ActivityPub API")
		{
			activitypub.Post("/users/:name/inbox", apUserInbox)
			activitypub.Get("/users/:name/outbox", apUserOutbox)
			activitypub.Get("/users/:name", apUserActor)
		}

		publishers := api.Group("/publishers").Name("Publisher API")
		{
			publishers.Get("/", listRelatedPublisher)
			publishers.Get("/me", listOwnedPublisher)
			publishers.Post("/personal", createPersonalPublisher)
			publishers.Post("/organization", createOrganizationPublisher)
			publishers.Get("/:name/pins", listPinnedPost)
			publishers.Get("/:name", getPublisher)
			publishers.Put("/:name", editPublisher)
			publishers.Delete("/:name", deletePublisher)
		}

		recommendations := api.Group("/recommendations").Name("Recommendations API")
		{
			recommendations.Get("/", listRecommendation)
			recommendations.Get("/shuffle", listRecommendationShuffle)
			recommendations.Get("/feed", getRecommendationFeed)
		}

		stories := api.Group("/stories").Name("Story API")
		{
			stories.Post("/", createStory)
			stories.Put("/:postId", editStory)
		}
		articles := api.Group("/articles").Name("Article API")
		{
			articles.Post("/", createArticle)
			articles.Put("/:postId", editArticle)
		}
		questions := api.Group("/questions").Name("Question API")
		{
			questions.Post("/", createQuestion)
			questions.Put("/:postId", editQuestion)
			questions.Put("/:postId/answer", selectQuestionAnswer)
		}
		videos := api.Group("/videos").Name("Video API")
		{
			videos.Post("/", createVideo)
			videos.Put("/:postId", editVideo)
		}

		posts := api.Group("/posts").Name("Posts API")
		{
			posts.Get("/", listPost)
			posts.Get("/search", searchPost)
			posts.Get("/minimal", listPostMinimal)
			posts.Get("/drafts", listDraftPost)
			posts.Get("/:postId", getPost)
			posts.Get("/:postId/insight", getPostInsight)
			posts.Post("/:postId/flag", createFlag)
			posts.Post("/:postId/react", reactPost)
			posts.Post("/:postId/pin", pinPost)
			posts.Post("/:postId/uncollapse", uncollapsePost)
			posts.Delete("/:postId", deletePost)

			posts.Get("/:postId/replies", listPostReplies)
			posts.Get("/:postId/replies/featured", listPostFeaturedReply)
		}

		polls := api.Group("/polls").Name("Polls API")
		{
			polls.Get("/:pollId", getPoll)
			polls.Post("/", createPoll)
			polls.Put("/:pollId", updatePoll)
			polls.Delete("/:pollId", deletePoll)
			polls.Post("/:pollId/answer", answerPoll)
			polls.Get("/:pollId/answer", getMyPollAnswer)
		}

		subscriptions := api.Group("/subscriptions").Name("Subscriptions API")
		{
			subscriptions.Get("/users/:userId", getSubscriptionOnUser)
			subscriptions.Get("/tags/:tagId", getSubscriptionOnTag)
			subscriptions.Get("/categories/:categoryId", getSubscriptionOnCategory)
			subscriptions.Post("/users/:userId", subscribeToUser)
			subscriptions.Post("/tags/:tagId", subscribeToTag)
			subscriptions.Post("/categories/:categoryId", subscribeToCategory)
			subscriptions.Delete("/users/:userId", unsubscribeFromUser)
			subscriptions.Delete("/tags/:tagId", unsubscribeFromTag)
			subscriptions.Delete("/categories/:categoryId", unsubscribeFromCategory)
		}

		api.Get("/categories", listCategories)
		api.Get("/categories/:category", getCategory)
		api.Post("/categories", newCategory)
		api.Put("/categories/:categoryId", editCategory)
		api.Delete("/categories/:categoryId", deleteCategory)

		api.Get("/tags", listTags)
		api.Get("/tags/:tag", getTag)

		api.Get("/whats-new", getWhatsNew)
	}
}

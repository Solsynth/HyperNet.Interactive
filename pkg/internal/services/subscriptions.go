package services

import (
	"errors"
	"fmt"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/database"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/gap"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/models"
	"git.solsynth.dev/hypernet/passport/pkg/authkit"
	authm "git.solsynth.dev/hypernet/passport/pkg/authkit/models"
	"git.solsynth.dev/hypernet/pusher/pkg/pushkit"
	"gorm.io/gorm"
)

func GetSubscriptionOnUser(user authm.Account, target models.Publisher) (*models.Subscription, error) {
	var subscription models.Subscription
	if err := database.C.Where("follower_id = ? AND account_id = ?", user.ID, target.ID).First(&subscription).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("unable to get subscription: %v", err)
	}
	return &subscription, nil
}

func GetSubscriptionOnTag(user authm.Account, target models.Tag) (*models.Subscription, error) {
	var subscription models.Subscription
	if err := database.C.Where("follower_id = ? AND tag_id = ?", user.ID, target.ID).First(&subscription).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("unable to get subscription: %v", err)
	}
	return &subscription, nil
}

func GetSubscriptionOnCategory(user authm.Account, target models.Category) (*models.Subscription, error) {
	var subscription models.Subscription
	if err := database.C.Where("follower_id = ? AND category_id = ?", user.ID, target.ID).First(&subscription).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("unable to get subscription: %v", err)
	}
	return &subscription, nil
}

func SubscribeToUser(user authm.Account, target models.Publisher) (models.Subscription, error) {
	var subscription models.Subscription
	if err := database.C.Where("follower_id = ? AND account_id = ?", user.ID, target.ID).First(&subscription).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return subscription, fmt.Errorf("subscription already exists")
		}
	}

	subscription = models.Subscription{
		FollowerID: user.ID,
		AccountID:  &target.ID,
	}

	err := database.C.Save(&subscription).Error
	return subscription, err
}

func SubscribeToTag(user authm.Account, target models.Tag) (models.Subscription, error) {
	var subscription models.Subscription
	if err := database.C.Where("follower_id = ? AND tag_id = ?", user.ID, target.ID).First(&subscription).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return subscription, fmt.Errorf("subscription already exists")
		}
	}

	subscription = models.Subscription{
		FollowerID: user.ID,
		TagID:      &target.ID,
	}

	err := database.C.Save(&subscription).Error
	return subscription, err
}

func SubscribeToCategory(user authm.Account, target models.Category) (models.Subscription, error) {
	var subscription models.Subscription
	if err := database.C.Where("follower_id = ? AND category_id = ?", user.ID, target.ID).First(&subscription).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return subscription, fmt.Errorf("subscription already exists")
		}
	}

	subscription = models.Subscription{
		FollowerID: user.ID,
		CategoryID: &target.ID,
	}

	err := database.C.Save(&subscription).Error
	return subscription, err
}

func UnsubscribeFromUser(user authm.Account, target models.Publisher) error {
	var subscription models.Subscription
	if err := database.C.Where("follower_id = ? AND account_id = ?", user.ID, target.ID).First(&subscription).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("subscription does not exist")
		}
		return fmt.Errorf("unable to check subscription is exists or not: %v", err)
	}

	err := database.C.Delete(&subscription).Error
	return err
}

func UnsubscribeFromTag(user authm.Account, target models.Tag) error {
	var subscription models.Subscription
	if err := database.C.Where("follower_id = ? AND tag_id = ?", user.ID, target.ID).First(&subscription).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("subscription does not exist")
		}
		return fmt.Errorf("unable to check subscription is exists or not: %v", err)
	}

	err := database.C.Delete(&subscription).Error
	return err
}

func UnsubscribeFromCategory(user authm.Account, target models.Category) error {
	var subscription models.Subscription
	if err := database.C.Where("follower_id = ? AND category_id = ?", user.ID, target.ID).First(&subscription).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("subscription does not exist")
		}
		return fmt.Errorf("unable to check subscription is exists or not: %v", err)
	}

	err := database.C.Delete(&subscription).Error
	return err
}

func NotifyUserSubscription(poster models.Publisher, content string, title *string) error {
	var subscriptions []models.Subscription
	if err := database.C.Where("account_id = ?", poster.ID).Preload("Follower").Find(&subscriptions).Error; err != nil {
		return fmt.Errorf("unable to get subscriptions: %v", err)
	}

	nTitle := fmt.Sprintf("New post from %s (%s)", poster.Nick, poster.Name)
	nSubtitle := "From your subscription"

	body := TruncatePostContentShort(content)
	if title != nil {
		body = fmt.Sprintf("%s\n%s", *title, body)
	}

	userIDs := make([]uint64, 0, len(subscriptions))
	for _, subscription := range subscriptions {
		userIDs = append(userIDs, uint64(subscription.Follower.ID))
	}

	err := authkit.NotifyUserBatch(gap.Nx, userIDs, pushkit.Notification{
		Topic:    "interactive.subscription",
		Title:    nTitle,
		Subtitle: nSubtitle,
		Body:     body,
		Priority: 3,
	})

	return err
}

func NotifyTagSubscription(poster models.Tag, og models.Publisher, content string, title *string) error {
	var subscriptions []models.Subscription
	if err := database.C.Where("tag_id = ?", poster.ID).Preload("Follower").Find(&subscriptions).Error; err != nil {
		return fmt.Errorf("unable to get subscriptions: %v", err)
	}

	nTitle := fmt.Sprintf("New post in %s by %s (%s)", poster.Name, og.Nick, og.Name)
	nSubtitle := "From your subscription"

	body := TruncatePostContentShort(content)
	if title != nil {
		body = fmt.Sprintf("%s\n%s", *title, body)
	}

	userIDs := make([]uint64, 0, len(subscriptions))
	for _, subscription := range subscriptions {
		userIDs = append(userIDs, uint64(subscription.Follower.ID))
	}

	err := authkit.NotifyUserBatch(gap.Nx, userIDs, pushkit.Notification{
		Topic:    "interactive.subscription",
		Title:    nTitle,
		Subtitle: nSubtitle,
		Body:     body,
		Priority: 3,
	})

	return err
}

func NotifyCategorySubscription(poster models.Category, og models.Publisher, content string, title *string) error {
	var subscriptions []models.Subscription
	if err := database.C.Where("category_id = ?", poster.ID).Preload("Follower").Find(&subscriptions).Error; err != nil {
		return fmt.Errorf("unable to get subscriptions: %v", err)
	}

	nTitle := fmt.Sprintf("New post in %s by %s (%s)", poster.Name, og.Nick, og.Name)
	nSubtitle := "From your subscription"

	body := TruncatePostContentShort(content)
	if title != nil {
		body = fmt.Sprintf("%s\n%s", *title, body)
	}

	userIDs := make([]uint64, 0, len(subscriptions))
	for _, subscription := range subscriptions {
		userIDs = append(userIDs, uint64(subscription.Follower.ID))
	}

	err := authkit.NotifyUserBatch(gap.Nx, userIDs, pushkit.Notification{
		Topic:    "interactive.subscription",
		Title:    nTitle,
		Subtitle: nSubtitle,
		Body:     body,
		Priority: 3,
	})

	return err
}

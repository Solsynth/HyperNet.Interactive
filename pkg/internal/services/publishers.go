package services

import (
	"fmt"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/database"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/models"
	authm "git.solsynth.dev/hypernet/passport/pkg/authkit/models"
)

func GetPublisher(id uint, userID uint) (models.Publisher, error) {
	var publisher models.Publisher
	if err := database.C.Where("id = ? AND account_id = ?", id, userID).First(&publisher).Error; err != nil {
		return publisher, fmt.Errorf("unable to get publisher: %v", err)
	}
	return publisher, nil
}

func GetPublisherByName(name string, userID uint) (models.Publisher, error) {
	var publisher models.Publisher
	if err := database.C.Where("name = ? AND account_id = ?", name, userID).First(&publisher).Error; err != nil {
		return publisher, fmt.Errorf("unable to get publisher: %v", err)
	}
	return publisher, nil
}

func CreatePersonalPublisher(user authm.Account, name, nick, desc, avatar, banner string) (models.Publisher, error) {
	publisher := models.Publisher{
		Type:        models.PublisherTypePersonal,
		Name:        name,
		Nick:        nick,
		Description: desc,
		Avatar:      avatar,
		Banner:      banner,
		AccountID:   &user.ID,
	}
	if user.Avatar != nil && len(publisher.Avatar) == 0 {
		publisher.Avatar = *user.Avatar
	}
	if user.Banner != nil && len(publisher.Banner) == 0 {
		publisher.Banner = *user.Banner
	}

	if err := database.C.Create(&publisher).Error; err != nil {
		return publisher, err
	}
	return publisher, nil
}

func CreateOrganizationPublisher(user authm.Account, realm authm.Realm, name, nick, desc, avatar, banner string) (models.Publisher, error) {
	publisher := models.Publisher{
		Type:        models.PublisherTypeOrganization,
		Name:        name,
		Nick:        nick,
		Description: desc,
		Avatar:      avatar,
		Banner:      banner,
		RealmID:     &realm.ID,
		AccountID:   &user.ID,
	}
	if realm.Avatar != nil && len(publisher.Avatar) == 0 {
		publisher.Avatar = *realm.Avatar
	}
	if realm.Banner != nil && len(publisher.Banner) == 0 {
		publisher.Banner = *realm.Banner
	}

	if err := database.C.Create(&publisher).Error; err != nil {
		return publisher, err
	}
	return publisher, nil
}

func EditPublisher(user authm.Account, publisher models.Publisher) (models.Publisher, error) {
	if publisher.Type == models.PublisherTypePersonal {
		if *publisher.AccountID != user.ID {
			return publisher, fmt.Errorf("you cannot transfer personal publisher")
		}
	}

	err := database.C.Save(&publisher).Error
	return publisher, err
}

func DeletePublisher(publisher models.Publisher) error {
	return database.C.Delete(&publisher).Error
}

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

func CreatePersonalPublisher(user authm.Account) (models.Publisher, error) {
	var publisher models.Publisher
	var count int64
	if err := database.C.
		Model(&models.Publisher{}).
		Where("account_id = ? AND type = ?", user.ID, models.PublisherTypePersonal).
		Count(&count).Error; err != nil {
		return publisher, fmt.Errorf("unable to count exsisting publisher: %v", err)
	}
	if count > 0 {
		return publisher, fmt.Errorf("personal publisher already exists")
	}

	publisher = models.Publisher{
		Type:        models.PublisherTypePersonal,
		Name:        user.Name,
		Nick:        user.Nick,
		Description: user.Description,
		AccountID:   &user.ID,
	}
	if user.Avatar != nil {
		publisher.Avatar = *user.Avatar
	}
	if user.Banner != nil {
		publisher.Banner = *user.Banner
	}

	if err := database.C.Create(&publisher).Error; err != nil {
		return publisher, err
	}
	return publisher, nil
}

func CreateOrganizationPublisher(user authm.Account, realm authm.Realm) (models.Publisher, error) {
	var publisher models.Publisher
	var count int64
	if err := database.C.
		Model(&models.Publisher{}).
		Where("realm_id = ? AND type = ?", realm.ID, models.PublisherTypeOrganization).
		Count(&count).Error; err != nil {
		return publisher, fmt.Errorf("unable to count exsisting publisher: %v", err)
	}
	if count > 0 {
		return publisher, fmt.Errorf("organization publisher already exists")
	}

	publisher = models.Publisher{
		Type:        models.PublisherTypeOrganization,
		Name:        realm.Alias,
		Nick:        realm.Name,
		Description: realm.Description,
		RealmID:     &realm.ID,
		AccountID:   &user.ID,
	}
	if realm.Avatar != nil {
		publisher.Avatar = *realm.Avatar
	}
	if realm.Banner != nil {
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

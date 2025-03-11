package services

import (
	"github.com/go-ap/activitypub"
	"github.com/spf13/viper"
)

func GetActivityID(uri string) activitypub.ID {
	baseUrl := viper.GetString("activitypub_base_url")
	return activitypub.ID(baseUrl + uri)
}

func GetActivityIRI(uri string) activitypub.IRI {
	baseUrl := viper.GetString("activitypub_base_url")
	return activitypub.IRI(baseUrl + uri)
}

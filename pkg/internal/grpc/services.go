package grpc

import (
	"context"

	"git.solsynth.dev/hypernet/interactive/pkg/internal/database"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/models"
	"git.solsynth.dev/hypernet/interactive/pkg/internal/services"
	"git.solsynth.dev/hypernet/nexus/pkg/nex"
	"git.solsynth.dev/hypernet/nexus/pkg/proto"
	jsoniter "github.com/json-iterator/go"
	"github.com/rs/zerolog/log"
)

func (v *App) BroadcastEvent(ctx context.Context, in *proto.EventInfo) (*proto.EventResponse, error) {
	switch in.GetEvent() {
	case "deletion":
		data := nex.DecodeMap(in.GetData())
		resType, ok := data["type"].(string)
		if !ok {
			break
		}
		switch resType {
		case "account":
			var data struct {
				ID int `json:"id"`
			}
			if err := jsoniter.Unmarshal(in.GetData(), &data); err != nil {
				break
			}
			tx := database.C.Begin()
			for _, model := range database.AutoMaintainRange {
				switch model.(type) {
				default:
					tx.Delete(model, "account_id = ?", data.ID)
				}
			}
			tx.Commit()
		case "realm":
			var data struct {
				ID int `json:"id"`
			}
			if err := jsoniter.Unmarshal(in.GetData(), &data); err != nil {
				break
			}
			var posts []models.Post
			if err := database.C.Where("realm_id = ?", data.ID).
				Select("Body").Select("ID").
				Find(&posts).Error; err != nil {
				break
			}
			if err := services.DeletePostInBatch(posts); err != nil {
				log.Error().Err(err).Msg("An error occurred when deleting post...")
			}
		}
	}

	return &proto.EventResponse{}, nil
}

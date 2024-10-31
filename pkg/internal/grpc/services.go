package grpc

import (
	"context"
	"git.solsynth.dev/hydrogen/dealer/pkg/proto"
	"git.solsynth.dev/hydrogen/interactive/pkg/internal/database"
	"git.solsynth.dev/hydrogen/interactive/pkg/internal/models"
	"strconv"
)

func (v *Server) BroadcastDeletion(ctx context.Context, request *proto.DeletionRequest) (*proto.DeletionResponse, error) {
	switch request.GetResourceType() {
	case "account":
		numericId, err := strconv.Atoi(request.GetResourceId())
		if err != nil {
			break
		}
		for _, model := range database.AutoMaintainRange {
			switch model.(type) {
			case *models.Post:
				database.C.Delete(model, "author_id = ?", numericId)
			default:
				database.C.Delete(model, "account_id = ?", numericId)
			}
		}
		database.C.Delete(&models.Publisher{}, "id = ?", numericId)
	}

	return &proto.DeletionResponse{}, nil
}

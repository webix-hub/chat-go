package service

import (
	"context"
	"time"

	"github.com/livekit/protocol/auth"
	livekit "github.com/livekit/protocol/livekit"
	lksdk "github.com/livekit/server-sdk-go"
)

type LivekitConfig struct {
	Enabled   bool
	Host      string
	ApiKey    string
	ApiSecret string
}

type livekitService struct {
	lksClient  *lksdk.RoomServiceClient
	APIKey     string
	APISercret string
}

func newLivekitService(cfg LivekitConfig) *livekitService {
	if !cfg.Enabled {
		return nil
	}

	return &livekitService{
		lksClient:  lksdk.NewRoomServiceClient(cfg.Host, cfg.ApiKey, cfg.ApiSecret),
		APIKey:     cfg.ApiKey,
		APISercret: cfg.ApiSecret,
	}
}

func (s *livekitService) CreateRoom(name string) (string, error) {
	room, err := s.lksClient.CreateRoom(context.Background(), &livekit.CreateRoomRequest{
		Name: name,
	})

	return room.GetName(), err
}

func (s *livekitService) DeleteRoom(name string) error {
	_, err := s.lksClient.DeleteRoom(context.Background(), &livekit.DeleteRoomRequest{
		Room: name,
	})
	return err
}

func (s *livekitService) DisconnectParticipant(roomName, userId string) error {
	res, _ := s.lksClient.ListParticipants(context.Background(), &livekit.ListParticipantsRequest{
		Room: roomName,
	})

	found := false
	for i := range res.Participants {
		if res.Participants[i].Identity == userId {
			if res.Participants[i].State == livekit.ParticipantInfo_DISCONNECTED {
				return nil
			}
			found = true
			break
		}
	}

	if !found {
		return nil
	}

	_, err := s.lksClient.RemoveParticipant(context.Background(), &livekit.RoomParticipantIdentity{
		Room:     roomName,
		Identity: userId,
	})

	return err
}

func (s *livekitService) CreateJoinToken(roomName, userId string) (string, error) {
	at := auth.NewAccessToken(s.APIKey, s.APISercret)
	grant := &auth.VideoGrant{
		RoomJoin: true,
		Room:     roomName,
	}
	at.AddGrant(grant).
		SetIdentity(userId).
		SetValidFor(time.Hour)

	return at.ToJWT()
}

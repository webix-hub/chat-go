package api

import (
	"context"
	"time"

	"github.com/livekit/protocol/auth"
	livekit "github.com/livekit/protocol/livekit"
	lksdk "github.com/livekit/server-sdk-go"
)

type LivekitConfig struct {
	Host      string
	ApiKey    string
	ApiSecret string
}

type LivekitService struct {
	lksClient  *lksdk.RoomServiceClient
	APIKey     string
	APISercret string
}

type RoomConfig struct {
	RoomName  string `json:"room"`
	JoinToken string `json:"token"`
}

func newLivekitService(cfg LivekitConfig) *LivekitService {
	return &LivekitService{
		lksClient:  lksdk.NewRoomServiceClient(cfg.Host, cfg.ApiKey, cfg.ApiSecret),
		APIKey:     cfg.ApiKey,
		APISercret: cfg.ApiSecret,
	}
}

func (s *LivekitService) CreateRoom(name string) (string, error) {
	room, err := s.lksClient.CreateRoom(context.Background(), &livekit.CreateRoomRequest{
		Name: name,
	})

	return room.GetName(), err
}

func (s *LivekitService) DeleteRoom(name string) error {
	_, err := s.lksClient.DeleteRoom(context.Background(), &livekit.DeleteRoomRequest{
		Room: name,
	})
	return err
}

func (s *LivekitService) GetJoinToken(roomName, userId string) (string, error) {
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

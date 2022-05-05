package main

import (
	"context"
	"log"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/livekit/protocol/auth"
	livekit "github.com/livekit/protocol/livekit"
	lksdk "github.com/livekit/server-sdk-go"
)

type LivekitServerConfig struct {
	Host      string
	ApiKey    string
	ApiSecret string
}

func main() {
	r := gin.Default()

	serverConfig := LivekitServerConfig{
		Host:      "http://127.0.0.1:7880",
		ApiKey:    "key_62737293bad5e",
		ApiSecret: "627372c4b4756",
	}

	r.GET("/api/get-join-token", func(c *gin.Context) {
		userName := c.Query("user-name")
		roomName := c.Query("room-name")
		if userName == "" || roomName == "" {
			c.JSON(200, gin.H{
				"error":   1,
				"message": "Your name or Room name cannot be empty",
			})
			return
		}

		at := auth.NewAccessToken(serverConfig.ApiKey, serverConfig.ApiSecret)
		grant := &auth.VideoGrant{
			RoomJoin: true,
			Room:     roomName,
		}
		at.AddGrant(grant).
			SetIdentity(userName).
			SetValidFor(time.Hour)

		token, err := at.ToJWT()
		if err != nil {
			log.Printf("get join token error: %v", err)
			c.JSON(200, gin.H{
				"error":   1,
				"message": "Get token error",
			})
			return
		}

		c.JSON(200, gin.H{
			"token": token,
		})
	})

	r.GET("/api/create-room", func(c *gin.Context) {
		roomName := c.Query("room-name")
		if roomName == "" {
			c.JSON(200, gin.H{
				"error":   1,
				"message": "Room name cannot be empty",
			})
			return
		}

		roomClient := lksdk.NewRoomServiceClient(serverConfig.Host, serverConfig.ApiKey, serverConfig.ApiSecret)
		room, err := roomClient.CreateRoom(context.Background(), &livekit.CreateRoomRequest{
			Name: roomName,
		})
		log.Printf("roon info: %v", room)
		if err != nil {
			log.Panicf("create room error: %v", err)
			c.JSON(200, gin.H{
				"error":   1,
				"message": "create room error",
			})
			return
		}

		c.JSON(200, gin.H{
			"sid": room.Sid,
		})
	})

	r.GET("/api/list-room", func(c *gin.Context) {
		roomClient := lksdk.NewRoomServiceClient(serverConfig.Host, serverConfig.ApiKey, serverConfig.ApiSecret)
		res, err := roomClient.ListRooms(context.Background(), &livekit.ListRoomsRequest{})
		if err != nil {
			log.Printf("get list room error: %v", err)
			c.JSON(200, gin.H{
				"error":   1,
				"message": "get list room error",
			})
			return
		}

		if len(res.Rooms) == 0 {
			c.JSON(200, gin.H{
				"message": "room not found",
			})
			return
		}

		c.JSON(200, res.Rooms)
	})

	r.GET("/api/delete-room", func(c *gin.Context) {
		roomId := c.Query("room-id")
		if roomId == "" {
			c.JSON(200, gin.H{
				"error":   1,
				"message": "Room id cannot be empty",
			})
			return
		}

		roomClient := lksdk.NewRoomServiceClient(serverConfig.Host, serverConfig.ApiKey, serverConfig.ApiSecret)
		_, err := roomClient.DeleteRoom(context.Background(), &livekit.DeleteRoomRequest{
			Room: roomId,
		})
		if err != nil {
			log.Printf("delete room error: %v", err)
			c.JSON(200, gin.H{
				"error":   1,
				"message": "delete room error",
			})
			return
		}

		c.JSON(200, gin.H{
			"ok": 1,
		})
	})

	r.GET("/api/list-participants", func(c *gin.Context) {
		roomName := c.Query("room-name")
		if roomName == "" {
			c.JSON(200, gin.H{
				"error":   1,
				"message": "Room name cannot be empty",
			})
			return
		}

		roomClient := lksdk.NewRoomServiceClient(serverConfig.Host, serverConfig.ApiKey, serverConfig.ApiSecret)
		res, err := roomClient.ListParticipants(context.Background(), &livekit.ListParticipantsRequest{
			Room: roomName,
		})
		if err != nil {
			log.Printf("get list participants error: %v", err)
			c.JSON(200, gin.H{
				"error":   1,
				"message": "get list participants error",
			})
			return
		}

		c.JSON(200, res.Participants)
	})

	r.GET("/api/remove-participant", func(c *gin.Context) {
		roomName := c.Query("room-name")
		userName := c.Query("user-name")
		if roomName == "" || userName == "" {
			c.JSON(200, gin.H{
				"error":   1,
				"message": "Room name or User name cannot be empty",
			})
			return
		}

		roomClient := lksdk.NewRoomServiceClient(serverConfig.Host, serverConfig.ApiKey, serverConfig.ApiSecret)
		_, err := roomClient.RemoveParticipant(context.Background(), &livekit.RoomParticipantIdentity{
			Room:     roomName,
			Identity: userName,
		})
		if err != nil {
			log.Printf("remove participant error: %v", err)
			c.JSON(200, gin.H{
				"error":   1,
				"message": "remove participant error",
			})
			return
		}

		c.JSON(200, gin.H{
			"ok": 1,
		})
	})

	r.GET("/api/mute-mic-participant", func(c *gin.Context) {
		roomName := c.Query("room-name")
		userName := c.Query("user-name")
		trackSid := c.Query("track-sid")
		if roomName == "" || userName == "" || trackSid == "" {
			c.JSON(200, gin.H{
				"error":   1,
				"message": "Invalid params",
			})
			return
		}

		roomClient := lksdk.NewRoomServiceClient(serverConfig.Host, serverConfig.ApiKey, serverConfig.ApiSecret)
		_, err := roomClient.MutePublishedTrack(context.Background(), &livekit.MuteRoomTrackRequest{
			Room:     roomName,
			Identity: userName,
			TrackSid: trackSid,
			Muted:    true,
		})
		if err != nil {
			log.Printf("mute participant error: %v", err)
			c.JSON(200, gin.H{
				"error":   1,
				"message": "mute participant error",
			})
			return
		}

		c.JSON(200, gin.H{
			"ok": 1,
		})
	})

	r.Run(":1010")
}

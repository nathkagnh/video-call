package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/livekit/protocol/auth"
	livekit "github.com/livekit/protocol/livekit"
	lksdk "github.com/livekit/server-sdk-go"
)

type LivekitServerConfig struct {
	Host      string
	ApiKey    string
	ApiSecret string
}

type RoomMetaData struct {
	RealName string `json:"real_name"`
}

type ParticipantMetaData struct {
	Avatar string `json:"avatar"`
}

type JoinTokenPostData struct {
	UserName string `json:"user_name"`
	Avatar   string `json:"avatar"`
	Room     string `json:"room"`
	Create   bool   `json:"create"`
	Passcode string `json:"passcode"`
}

func main() {
	serverConfig := LivekitServerConfig{
		Host:      "http://127.0.0.1:7880",
		ApiKey:    "key_62737293bad5e",
		ApiSecret: "627372c4b4756",
	}

	r := gin.Default()

	r.POST("/api/get-join-token", func(c *gin.Context) {
		var postData JoinTokenPostData
		if c.ShouldBind(&postData) != nil {
			c.JSON(200, gin.H{
				"error":   1,
				"message": "Invalid data",
			})
			return
		}
		if postData.UserName == "" || (postData.Room == "" && !postData.Create) {
			c.JSON(200, gin.H{
				"error":   1,
				"message": "Invalid data",
			})
			return
		}

		room := postData.Room
		if postData.Create {
			room = uuid.New().String()

			metaData := RoomMetaData{
				RealName: postData.Room,
			}
			metaDataJsonString, err := json.Marshal(metaData)
			if err != nil {
				log.Printf("encode meta data error: %v", err)
			}

			roomClient := lksdk.NewRoomServiceClient(serverConfig.Host, serverConfig.ApiKey, serverConfig.ApiSecret)
			_, err = roomClient.CreateRoom(context.Background(), &livekit.CreateRoomRequest{
				Name:     room,
				Metadata: string(metaDataJsonString),
			})
			if err != nil {
				log.Printf("create room error: %v", err)
				c.JSON(200, gin.H{
					"error":   1,
					"message": "Create room error",
				})
				return
			}

			// save passcode to redis
			if postData.Create && postData.Passcode != "" {
				var ctx = context.Background()
				rdb := redis.NewClient(&redis.Options{
					Addr:     "127.0.0.1:6379",
					Password: "",
					DB:       0,
				})
				key := "room_info:" + room
				passcode, err := bcrypt.GenerateFromPassword([]byte(postData.Passcode), 14)
				if err != nil {
					log.Printf("create room error: %v", err)
					c.JSON(200, gin.H{
						"error":   1,
						"message": "Create room error",
					})
				}
				err = rdb.Set(ctx, key, string(passcode), 0).Err()
				if err != nil {
					log.Printf("create room error: %v", err)
					c.JSON(200, gin.H{
						"error":   1,
						"message": "Create room error",
					})
				}
			}
		}

		// check passcode
		if !postData.Create {
			var ctx = context.Background()
			rdb := redis.NewClient(&redis.Options{
				Addr:     "127.0.0.1:6379",
				Password: "",
				DB:       0,
			})
			key := "room_info:" + room
			passcode, err := rdb.Get(ctx, key).Result()
			if err != nil && err != redis.Nil {
				log.Printf("create room error: %v", err)
				c.JSON(200, gin.H{
					"error":   1,
					"message": "Create room error",
				})
			}
			if passcode != "" {
				err = bcrypt.CompareHashAndPassword([]byte(passcode), []byte(postData.Passcode))
				if err != nil {
					c.JSON(200, gin.H{
						"error":   1,
						"message": "Passcode invalid",
					})
					return
				}
			}
		}

		at := auth.NewAccessToken(serverConfig.ApiKey, serverConfig.ApiSecret)
		grant := &auth.VideoGrant{
			RoomJoin: true,
			Room:     room,
		}
		at.AddGrant(grant).
			SetIdentity(uuid.New().String()).
			SetName(postData.UserName).
			SetValidFor(time.Hour)
		if postData.Avatar != "" {
			metaData := ParticipantMetaData{
				Avatar: postData.Avatar,
			}
			metaDataJsonString, err := json.Marshal(metaData)
			if err != nil {
				log.Printf("encode meta data error: %v", err)
			}

			at.SetMetadata(string(metaDataJsonString))
		}

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
			"room":  room,
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
			log.Printf("create room error: %v", err)
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

	r.GET("/api/update-room", func(c *gin.Context) {
		roomName := c.Query("room-name")
		if roomName == "" {
			c.JSON(200, gin.H{
				"error":   1,
				"message": "Room name cannot be empty",
			})
			return
		}

		roomClient := lksdk.NewRoomServiceClient(serverConfig.Host, serverConfig.ApiKey, serverConfig.ApiSecret)
		_, err := roomClient.UpdateRoomMetadata(context.Background(), &livekit.UpdateRoomMetadataRequest{
			Room:     roomName,
			Metadata: "dsads dsa dsadsa das dsad",
		})
		if err != nil {
			log.Printf("update room error: %v", err)
			c.JSON(200, gin.H{
				"error":   1,
				"message": "update room error",
			})
			return
		}

		c.JSON(200, gin.H{
			"ok": 1,
		})
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

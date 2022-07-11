package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
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
	Host     string `json:"host"`
}

type ParticipantMetaData struct {
	Avatar string `json:"avatar"`
}

type JoinTokenPostData struct {
	UserInfo string `json:"uii"`
	UserName string `json:"user_name"`
	Avatar   string `json:"avatar"`
	Room     string `json:"room"`
	Create   bool   `json:"create"`
	Passcode string `json:"passcode"`
}

type PMR struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Host         string `json:"host"`
	HostName     string `json:"host_name"`
	Passcode     string `json:"passcode"`
	CreationTime int32  `json:"creation_time"`
}

var ctx = context.Background()
var redisClient *redis.Client

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

		// find|create participant id
		var participantID, secretID string
		secretKey := "_cf-fa-7d"
		if postData.UserInfo != "" {
			userInfo := strings.Split(postData.UserInfo, ",")
			if len(userInfo) == 2 {
				xxx := userInfo[1] + secretKey
				err := bcrypt.CompareHashAndPassword([]byte(userInfo[0]), []byte(xxx))
				if err == nil {
					participantID = userInfo[1]
					secretID = userInfo[0]
				}
			}
		}
		if participantID == "" {
			participantID = pseudo_uuid()
			xxx := participantID + secretKey
			sID, err := bcrypt.GenerateFromPassword([]byte(xxx), 10)
			if err != nil {
				log.Printf("create room error: %v", err)
				c.JSON(200, gin.H{
					"error":   1,
					"message": "Get token error",
				})
				return
			}
			secretID = string(sID)
		}

		room := postData.Room
		if postData.Create {
			room = pseudo_uuid()

			metaData := RoomMetaData{
				RealName: postData.Room,
				Host:     participantID,
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
			if postData.Passcode != "" {
				rdb, err := getRedisClient()
				if err != nil {
					log.Printf("create room error: %v", err)
					c.JSON(200, gin.H{
						"error":   1,
						"message": "Create room error",
					})
					return
				}
				key := "room_info:" + room
				passcode, err := bcrypt.GenerateFromPassword([]byte(postData.Passcode), 10)
				if err != nil {
					log.Printf("create room error: %v", err)
					c.JSON(200, gin.H{
						"error":   1,
						"message": "Create room error",
					})
					return
				}
				err = rdb.Set(ctx, key, string(passcode), 0).Err()
				if err != nil {
					log.Printf("create room error: %v", err)
					c.JSON(200, gin.H{
						"error":   1,
						"message": "Create room error",
					})
					return
				}
			}
		} else {
			// check passcode
			rdb, err := getRedisClient()
			if err != nil {
				c.JSON(200, gin.H{
					"error":   1,
					"message": "Incorrect passcode",
				})
				return
			}
			key := "room_info:" + room
			passcode, err := rdb.Get(ctx, key).Result()
			if err == redis.Nil {
				pmr, err := getPMR(room)
				if err != nil {
					c.JSON(200, gin.H{
						"error":   1,
						"message": "Incorrect passcode",
					})
					return
				}
				if pmr.ID != "" {
					passcode = pmr.Passcode
				}
			} else if err != nil {
				c.JSON(200, gin.H{
					"error":   1,
					"message": "Incorrect passcode",
				})
				return
			}
			if passcode != "" {
				err = bcrypt.CompareHashAndPassword([]byte(passcode), []byte(postData.Passcode))
				if err != nil {
					c.JSON(200, gin.H{
						"error":   1,
						"message": "Incorrect passcode",
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
		if postData.Create {
			grant.RoomAdmin = true
		}

		at.AddGrant(grant).
			SetIdentity(participantID).
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
			"uii":   secretID + "," + participantID,
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

func getRedisClient() (*redis.Client, error) {
	if redisClient == nil {
		redisClient = redis.NewClient(&redis.Options{
			Addr:     "127.0.0.1:6379",
			Password: "",
			DB:       0,
		})
	}
	return redisClient, nil
}

func pseudo_uuid() (uuid string) {
	b := make([]byte, 3)
	_, err := rand.Read(b)
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}

	uuid = fmt.Sprintf("%X-%X-%X", b[0:1], b[1:2], b[2:3])
	uuid = strings.ToLower(uuid)

	return
}

func getPMR(ID string) (PMR, error) {
	var pmr PMR
	rdb, err := getRedisClient()
	if err != nil {
		log.Println(err)
		return pmr, err
	}
	key := "pmr:" + ID
	data, err := rdb.Get(ctx, key).Result()
	if err != nil && err != redis.Nil {
		log.Println(err)
		return pmr, err
	}
	if data != "" {
		err := json.Unmarshal([]byte(data), &pmr)
		if err != nil {
			return pmr, err
		}
	}

	return pmr, nil
}

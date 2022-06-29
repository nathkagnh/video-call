package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"backend-meeting.fptonline.net/internal/config"
	"backend-meeting.fptonline.net/internal/stream"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
	"github.com/livekit/protocol/livekit"
	lksdk "github.com/livekit/server-sdk-go"
	"golang.org/x/crypto/bcrypt"
)

type RoomRender struct {
	ID              string `json:"sid"`
	Name            string `json:"name"`
	RealName        string `json:"real_name"`
	CreationTime    string
	EnabledCodecs   string
	NumParticipants int32 `json:"num_participants"`
}

type ParticipantRender struct {
	ID         string `json:"sid"`
	Identity   string `json:"identity"`
	Name       string `json:"name"`
	JoinedAt   string
	Permission string

	ScreenShare         bool
	ScreenShare_TrackID string

	CameraEnable   bool
	Camera_TrackID string

	MicrophoneMuted    bool
	Microphone_TrackID string
}

type RoomMetaData struct {
	RealName string `json:"real_name"`
}

var wsupgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type PMR struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Host         string `json:"host"`
	HostName     string `json:"host_name"`
	Passcode     string `json:"passcode"`
	CreationTime int32  `json:"creation_time"`
}

func main() {
	serverConfig := config.LivekitServerConfig{
		Host:      "http://127.0.0.1:7880",
		ApiKey:    "key_627372ca26d03",
		ApiSecret: "627372b54ba4e",
	}

	r := gin.Default()

	r.Static("/manager/assets", "./assets")

	r.LoadHTMLGlob("templates/*")

	r.GET("/manager", func(c *gin.Context) {
		listRoom, err := getListRoom(serverConfig)
		if err != nil {
			log.Fatalf("get list room error: %v", err)
		}

		log.Printf("%v", listRoom)

		c.HTML(http.StatusOK, "manager.html", gin.H{
			"nav":      "rooms",
			"listRoom": listRoom,
		})
	})

	r.GET("/manager/room", func(c *gin.Context) {
		roomName := c.Query("name")
		if roomName == "" {
			c.Redirect(http.StatusFound, "/manager")
			return
		}

		listParticipant, err := getListParticipant(roomName, serverConfig)
		if err != nil {
			log.Fatalf("get participant error: %v", err)
			c.Redirect(http.StatusFound, "/manager")
			return
		}

		log.Printf("%v", listParticipant)

		if len(listParticipant) == 0 {
			c.Redirect(http.StatusFound, "/manager")
		}

		roomDetail, err := getRoomDetail(roomName, serverConfig)
		if err != nil {
			c.Redirect(http.StatusFound, "/manager")
			return
		}
		metaData := RoomMetaData{}
		err = json.Unmarshal([]byte(roomDetail.Metadata), &metaData)
		if err != nil {
			log.Printf("decode metadata error: %v", err)
		}
		realName := roomName
		if metaData.RealName != "" {
			realName = metaData.RealName
		}

		c.HTML(http.StatusOK, "room.html", gin.H{
			"nav":             "rooms",
			"room":            roomDetail.Name,
			"roomName":        realName,
			"listParticipant": listParticipant,
		})
	})

	r.GET("/manager/room/pmr", func(c *gin.Context) {
		data := gin.H{
			"nav": "pmr",
		}
		listPMR, err := getAllPMR()
		if err == nil {
			data["list"] = listPMR
		}

		c.HTML(http.StatusOK, "pmr.html", data)
	})

	r.GET("/manager/room/create-pmr", func(c *gin.Context) {
		c.HTML(http.StatusOK, "edit-pmr.html", gin.H{
			"nav": "pmr",
		})
	})

	r.POST("/manager/room/create-pmr", func(c *gin.Context) {
		host := c.PostForm("host")
		hostName := c.PostForm("host-name")
		name := c.PostForm("name")
		passcode := c.PostForm("passcode")

		data := gin.H{
			"nav": "pmr",
		}
		var errors []string
		if host == "" || hostName == "" {
			errors = append(errors, "Identify invalid")
		}
		if name == "" || passcode == "" {
			errors = append(errors, "Room info invalid")
		}

		passcodeHashed, err := bcrypt.GenerateFromPassword([]byte(passcode), 10)
		if err != nil {
			errors = append(errors, "Something wrong, Please try again.")
		}

		pmrInfo := PMR{
			Name:     name,
			Host:     host,
			HostName: hostName,
		}

		if len(errors) == 0 {
			ID := getPMI(name)
			pmrInfo.ID = ID
			pmrInfo.Passcode = string(passcodeHashed)
			err := createPMR(pmrInfo)
			if err == nil {
				data["success"] = true
				c.Redirect(http.StatusFound, "/manager/room/pmr")
				return
			} else {
				errors = append(errors, "Room already exists")
			}
		}

		if len(errors) > 0 {
			data["error"] = true
			data["errorMessages"] = strings.Join(errors, ", ")
		}
		data["data"] = pmrInfo
		c.HTML(http.StatusOK, "edit-pmr.html", data)
	})

	r.GET("/manager/room/edit-pmr", func(c *gin.Context) {
		ID := c.Query("pmi")
		if ID == "" {
			c.Redirect(http.StatusFound, "/manager/room/pmr")
			return
		}

		pmr, err := getPMR(ID)
		if err != nil || pmr.ID == "" {
			c.Redirect(http.StatusFound, "/manager/room/pmr")
			return
		}

		data := gin.H{
			"nav":  "pmr",
			"data": pmr,
			"ID":   ID,
		}
		act := c.Query("action")
		if act == "delete" {
			c.Redirect(http.StatusFound, "/manager/room/pmr")
			return
		}

		c.HTML(http.StatusOK, "edit-pmr.html", data)
	})

	r.POST("/manager/room/edit-pmr", func(c *gin.Context) {
		ID := c.PostForm("id")
		oldPasscode := c.PostForm("old-passcode")
		passcode := c.PostForm("passcode")

		pmr, err := getPMR(ID)
		if err != nil || pmr.ID == "" {
			c.Redirect(http.StatusFound, "/manager/room/pmr")
			return
		}

		data := gin.H{
			"nav":  "pmr",
			"data": pmr,
			"ID":   ID,
		}
		var errors []string

		err = bcrypt.CompareHashAndPassword([]byte(pmr.Passcode), []byte(oldPasscode))
		if err != nil {
			errors = append(errors, "Passcode invalid")
		}

		if len(errors) == 0 {
			passcodeHashed, err := bcrypt.GenerateFromPassword([]byte(passcode), 10)
			if err != nil {
				errors = append(errors, "Something wrong, Please try again.")
			} else {
				err := updatePMR(PMR{
					ID:       ID,
					Passcode: string(passcodeHashed),
				})
				if err != nil {
					errors = append(errors, "Update error")
				} else {
					data["success"] = true
				}
			}
		}

		if len(errors) > 0 {
			data["error"] = true
			data["errorMessages"] = strings.Join(errors, ", ")
		}
		c.HTML(http.StatusOK, "edit-pmr.html", data)
	})

	r.GET("/manager/room/kick-out-all", func(c *gin.Context) {
		roomName := c.Query("room")
		if roomName == "" {
			c.Redirect(http.StatusFound, "/manager")
			return
		}

		_ = removeRoom(roomName, serverConfig)
		time.Sleep(time.Second)

		c.Redirect(http.StatusFound, "/manager")
	})

	r.GET("/manager/room/kick-out", func(c *gin.Context) {
		roomName := c.Query("room")
		if roomName == "" {
			c.Redirect(http.StatusFound, "/manager")
			return
		}

		userName := c.Query("user-name")
		if userName == "" {
			c.Redirect(http.StatusFound, "/manager/room?name="+roomName)
			return
		}

		_ = removeParticipant(roomName, userName, serverConfig)
		time.Sleep(time.Second)

		c.Redirect(http.StatusFound, "/manager/room?name="+roomName)
	})

	r.GET("/manager/room/toggle-mic-participant", func(c *gin.Context) {
		roomName := c.Query("room")
		if roomName == "" {
			c.Redirect(http.StatusFound, "/manager")
			return
		}

		userName := c.Query("user-name")
		if userName == "" {
			c.Redirect(http.StatusFound, "/manager/room?name="+roomName)
			return
		}

		trackSid := c.Query("track-sid")
		if trackSid == "" {
			c.Redirect(http.StatusFound, "/manager/room?name="+roomName)
			return
		}

		muted := c.Query("muted") == "true"

		_ = toggleMicParticipant(roomName, userName, trackSid, muted, serverConfig)
		time.Sleep(time.Second)

		c.Redirect(http.StatusFound, "/manager/room?name="+roomName)
	})

	r.GET("/manager/room/toggle-cam-participant", func(c *gin.Context) {
		roomName := c.Query("room")
		if roomName == "" {
			c.Redirect(http.StatusFound, "/manager")
			return
		}

		sid := c.Query("sid")
		if sid == "" {
			c.Redirect(http.StatusFound, "/manager/room?name="+roomName)
			return
		}

		enable := c.Query("enable")
		data := []byte(`{"toggle_camera":` + enable + `}`)

		_ = sendData(roomName, data, livekit.DataPacket_RELIABLE, []string{sid}, serverConfig)
		time.Sleep(time.Second)

		c.Redirect(http.StatusFound, "/manager/room?name="+roomName)
	})

	r.GET("/manager/room/turn-off-screen-sharing", func(c *gin.Context) {
		roomName := c.Query("room")
		if roomName == "" {
			c.Redirect(http.StatusFound, "/manager")
			return
		}

		sid := c.Query("sid")
		if sid == "" {
			c.Redirect(http.StatusFound, "/manager/room?name="+roomName)
			return
		}

		data := []byte(`{"off_screenshare":true}`)

		_ = sendData(roomName, data, livekit.DataPacket_RELIABLE, []string{sid}, serverConfig)
		time.Sleep(time.Second)

		c.Redirect(http.StatusFound, "/manager/room?name="+roomName)
	})

	r.GET("/manager/streams", func(c *gin.Context) {
		listRoom, err := getListRoom(serverConfig)
		if err != nil {
			log.Fatalf("get list room error: %v", err)
		}

		c.HTML(http.StatusOK, "stream.html", gin.H{
			"nav":      "streams",
			"listRoom": listRoom,
		})
	})

	r.GET("/manager/streams/send", func(c *gin.Context) {
		roomName := c.Query("room-name")
		rtmpLink := c.Query("rtmp-link")
		if roomName == "" || rtmpLink == "" {
			c.JSON(http.StatusOK, gin.H{
				"error":   1,
				"message": "Room name and RTMP link cannot be empty",
			})
			return
		}
	})

	r.GET("/manager/stop-streaming", func(c *gin.Context) {
		roomName := c.Query("room-name")
		if roomName == "" {
			c.Redirect(http.StatusFound, "/manager/streams")
			return
		}

		_ = removeParticipant(roomName, "bot", serverConfig)
		time.Sleep(time.Second)

		c.Redirect(http.StatusFound, "/manager/streams")
	})

	r.GET("/ws", func(c *gin.Context) {
		ws, err := wsupgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("failed to set websocket upgrade: %v", err)
			return
		}

		for {
			t, msg, err := ws.ReadMessage()
			if err != nil {
				break
			}

			var responseMsg string
			if string(msg) == "ping" {
				responseMsg = "pong"
			} else {
				log.Printf("msg: %v", string(msg))

				var dataJson map[string]string
				err := json.Unmarshal(msg, &dataJson)
				if err != nil {
					log.Printf("recieved data error: %v", err)
				}

				ws.WriteMessage(t, []byte("sending..."))
				stream.PublishTrackToRoom(dataJson["roomName"], dataJson["rtmpLink"], serverConfig, ws, t)
			}

			ws.WriteMessage(t, []byte(responseMsg))
		}
	})

	r.Run(":2020")
}

func getListRoom(serverConfig config.LivekitServerConfig) ([]RoomRender, error) {
	roomClient := lksdk.NewRoomServiceClient(serverConfig.Host, serverConfig.ApiKey, serverConfig.ApiSecret)
	res, err := roomClient.ListRooms(context.Background(), &livekit.ListRoomsRequest{})
	if err != nil {
		log.Printf("get list room error: %v", err)
		return nil, err
	}

	rooms := []RoomRender{}
	for _, room := range res.Rooms {
		metaData := RoomMetaData{}
		err := json.Unmarshal([]byte(room.Metadata), &metaData)
		if err != nil {
			log.Printf("decode metadata error: %v", err)
		}
		realName := room.Name
		if metaData.RealName != "" {
			realName = metaData.RealName
		}

		roomRender := RoomRender{
			ID:              room.Sid,
			Name:            room.Name,
			RealName:        realName,
			NumParticipants: int32(room.NumParticipants),
		}
		roomRender.CreationTime = time.Unix(int64(room.CreationTime), 0).Format(time.RFC3339)

		if len(room.EnabledCodecs) > 0 {
			var listCodec []string
			for _, codec := range room.EnabledCodecs {
				listCodec = append(listCodec, codec.Mime)
			}
			roomRender.EnabledCodecs = strings.Join(listCodec, ", ")
		}

		rooms = append(rooms, roomRender)
	}

	return rooms, nil
}

func getRoomDetail(room string, serverConfig config.LivekitServerConfig) (*livekit.Room, error) {
	roomClient := lksdk.NewRoomServiceClient(serverConfig.Host, serverConfig.ApiKey, serverConfig.ApiSecret)
	res, err := roomClient.ListRooms(context.Background(), &livekit.ListRoomsRequest{
		Names: []string{room},
	})
	if err != nil || len(res.Rooms) == 0 {
		return nil, err
	}

	return res.Rooms[0], err
}

func getListParticipant(roomName string, serverConfig config.LivekitServerConfig) ([]ParticipantRender, error) {
	roomClient := lksdk.NewRoomServiceClient(serverConfig.Host, serverConfig.ApiKey, serverConfig.ApiSecret)
	res, err := roomClient.ListParticipants(context.Background(), &livekit.ListParticipantsRequest{
		Room: roomName,
	})
	if err != nil {
		log.Printf("get list participants error: %v", err)
		return nil, err
	}

	listParticipant := []ParticipantRender{}
	for _, participant := range res.Participants {
		participantRender := ParticipantRender{
			ID:       participant.Sid,
			Identity: participant.Identity,
			Name:     participant.Name,
		}
		participantRender.JoinedAt = time.Unix(int64(participant.JoinedAt), 0).Format(time.RFC3339)

		var listPermission []string
		if participant.Permission.CanPublish {
			listPermission = append(listPermission, "can_publish")
		}
		if participant.Permission.CanPublishData {
			listPermission = append(listPermission, "can_publish_data")
		}
		if participant.Permission.CanSubscribe {
			listPermission = append(listPermission, "can_subscribe")
		}
		if len(listPermission) > 0 {
			participantRender.Permission = strings.Join(listPermission, ", ")
		}

		if len(participant.Tracks) > 0 {
			for _, track := range participant.Tracks {
				_source := track.Source.String()
				if _source == "SCREEN_SHARE" {
					participantRender.ScreenShare = true
					participantRender.ScreenShare_TrackID = track.Sid
				} else {
					_type := track.Type.String()
					if _type == "VIDEO" {
						participantRender.CameraEnable = !track.Muted
						participantRender.Camera_TrackID = track.Sid
					} else if _type == "AUDIO" {
						participantRender.MicrophoneMuted = track.Muted
						participantRender.Microphone_TrackID = track.Sid
					}
				}
				log.Printf("DEBUG: %v-%v", track, participantRender)
			}
		}

		listParticipant = append(listParticipant, participantRender)
	}

	return listParticipant, nil
}

func removeParticipant(roomName string, userName string, serverConfig config.LivekitServerConfig) error {
	roomClient := lksdk.NewRoomServiceClient(serverConfig.Host, serverConfig.ApiKey, serverConfig.ApiSecret)
	_, err := roomClient.RemoveParticipant(context.Background(), &livekit.RoomParticipantIdentity{
		Room:     roomName,
		Identity: userName,
	})
	if err != nil {
		log.Printf("remove participant error: %v", err)
		return err
	}

	return nil
}

func removeRoom(roomName string, serverConfig config.LivekitServerConfig) error {
	roomClient := lksdk.NewRoomServiceClient(serverConfig.Host, serverConfig.ApiKey, serverConfig.ApiSecret)
	_, err := roomClient.DeleteRoom(context.Background(), &livekit.DeleteRoomRequest{
		Room: roomName,
	})
	if err != nil {
		log.Printf("delete room error: %v", err)
		return err
	}
	return nil
}

func toggleMicParticipant(roomName string, userName string, trackSid string, muted bool, serverConfig config.LivekitServerConfig) error {
	roomClient := lksdk.NewRoomServiceClient(serverConfig.Host, serverConfig.ApiKey, serverConfig.ApiSecret)
	_, err := roomClient.MutePublishedTrack(context.Background(), &livekit.MuteRoomTrackRequest{
		Room:     roomName,
		Identity: userName,
		TrackSid: trackSid,
		Muted:    muted,
	})
	if err != nil {
		log.Printf("mute participant error: %v", err)
	}
	return nil
}

// func updateSubscriptions(roomName string, userName string, trackSids []string, subscribe bool, serverConfig config.LivekitServerConfig) error {
// 	roomClient := lksdk.NewRoomServiceClient(serverConfig.Host, serverConfig.ApiKey, serverConfig.ApiSecret)
// 	_, err := roomClient.UpdateSubscriptions(context.Background(), &livekit.UpdateSubscriptionsRequest{
// 		Room:      roomName,
// 		Identity:  userName,
// 		TrackSids: trackSids,
// 		Subscribe: subscribe,
// 	})
// 	if err != nil {
// 		log.Printf("update subscriptions error: %v", err)
// 	}
// 	return nil
// }

func sendData(roomName string, data []byte, kind livekit.DataPacket_Kind, destinationSids []string, serverConfig config.LivekitServerConfig) error {
	roomClient := lksdk.NewRoomServiceClient(serverConfig.Host, serverConfig.ApiKey, serverConfig.ApiSecret)
	_, err := roomClient.SendData(context.Background(), &livekit.SendDataRequest{
		Room:            roomName,
		Data:            data,
		Kind:            kind,
		DestinationSids: destinationSids,
	})
	if err != nil {
		log.Printf("send data error: %v", err)
	}
	return nil
}

func getPMI(content string) string {
	content = regexp.MustCompile("(à|á|ạ|ả|ã|â|ầ|ấ|ậ|ẩ|ẫ|ă|ằ|ắ|ặ|ẳ|ẵ)").ReplaceAllString(content, "a")
	content = regexp.MustCompile("(À|Á|Ạ|Ả|Ã|Â|Ầ|Ấ|Ậ|Ẩ|Ẫ|Ă|Ằ|Ắ|Ặ|Ẳ|Ẵ)").ReplaceAllString(content, "A")
	content = regexp.MustCompile("(è|é|ẹ|ẻ|ẽ|ê|ề|ế|ệ|ể|ễ)").ReplaceAllString(content, "e")
	content = regexp.MustCompile("(È|É|Ẹ|Ẻ|Ẽ|Ê|Ề|Ế|Ệ|Ể|Ễ)").ReplaceAllString(content, "E")
	content = regexp.MustCompile("(ì|í|ị|ỉ|ĩ)").ReplaceAllString(content, "i")
	content = regexp.MustCompile("(Ì|Í|Ị|Ỉ|Ĩ)").ReplaceAllString(content, "I")
	content = regexp.MustCompile("(ò|ó|ọ|ỏ|õ|ô|ồ|ố|ộ|ổ|ỗ|ơ|ờ|ớ|ợ|ở|ỡ)").ReplaceAllString(content, "o")
	content = regexp.MustCompile("(Ò|Ó|Ọ|Ỏ|Õ|Ô|Ồ|Ố|Ộ|Ổ|Ỗ|Ơ|Ờ|Ớ|Ợ|Ở|Ỡ)").ReplaceAllString(content, "O")
	content = regexp.MustCompile("(ù|ú|ụ|ủ|ũ|ư|ừ|ứ|ự|ử|ữ)").ReplaceAllString(content, "u")
	content = regexp.MustCompile("(Ù|Ú|Ụ|Ủ|Ũ|Ư|Ừ|Ứ|Ự|Ử|Ữ)").ReplaceAllString(content, "U")
	content = regexp.MustCompile("(ỳ|ý|ỵ|ỷ|ỹ)").ReplaceAllString(content, "y")
	content = regexp.MustCompile("(Ỳ|Ý|Ỵ|Ỷ|Ỹ)").ReplaceAllString(content, "Y")
	content = regexp.MustCompile("(đ)").ReplaceAllString(content, "d")
	content = regexp.MustCompile("(Đ)").ReplaceAllString(content, "D")
	content = regexp.MustCompile(`(\s+)`).ReplaceAllString(content, "-")
	content = regexp.MustCompile("([^a-zA-Z0-9-])").ReplaceAllString(content, "")

	return content
}

func createPMR(pmr PMR) error {
	var ctx = context.Background()
	rdb := redis.NewClient(&redis.Options{
		Addr:     "127.0.0.1:6379",
		Password: "",
		DB:       0,
	})
	key := "pmr:" + pmr.ID
	data, err := rdb.Get(ctx, key).Result()
	if err != nil && err != redis.Nil {
		log.Println(err)
		return err
	}
	if data == "" {
		pmr.CreationTime = int32(time.Now().Unix())
		pmrJson, err := json.Marshal(pmr)
		if err != nil {
			return err
		}
		err = rdb.Set(ctx, key, pmrJson, 0).Err()
		if err != nil {
			log.Println(err)
			return err
		}
		keyInfo := "room_info:" + pmr.ID
		err = rdb.Set(ctx, keyInfo, pmr.Passcode, 0).Err()
		if err != nil {
			log.Println(err)
			return err
		}
	} else {
		return errors.New("room exist")
	}
	return nil
}

func updatePMR(pmr PMR) error {
	pmrUpdate, err := getPMR(pmr.ID)
	if err != nil || pmrUpdate.ID == "" {
		return err
	}
	pmrUpdate.Passcode = pmr.Passcode
	pmrJson, err := json.Marshal(pmrUpdate)
	if err != nil {
		return err
	}
	key := "pmr:" + pmr.ID
	var ctx = context.Background()
	rdb := redis.NewClient(&redis.Options{
		Addr:     "127.0.0.1:6379",
		Password: "",
		DB:       0,
	})
	err = rdb.Set(ctx, key, pmrJson, 0).Err()
	if err != nil {
		log.Println(err)
		return err
	}
	keyInfo := "room_info:" + pmr.ID
	err = rdb.Set(ctx, keyInfo, pmr.Passcode, 0).Err()
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}

func getAllPMR() ([]PMR, error) {
	var ctx = context.Background()
	rdb := redis.NewClient(&redis.Options{
		Addr:     "127.0.0.1:6379",
		Password: "",
		DB:       0,
	})
	key := "pmr:*"

	var keys []string
	var cursor uint64
	var n int
	for {
		var _keys []string
		var err error
		_keys, cursor, err = rdb.Scan(ctx, cursor, key, 10).Result()
		if err != nil {
			return nil, err
		}
		n += len(_keys)
		keys = append(keys, _keys...)
		if cursor == 0 {
			break
		}
	}

	var listPMR []PMR
	if len(keys) > 0 {
		for _, key := range keys {
			key = strings.Replace(key, "pmr:", "", 1)
			pmr, err := getPMR(key)
			if err == nil {
				listPMR = append(listPMR, pmr)
			}
		}
	}
	log.Printf("DEBUG: %v", listPMR)
	return listPMR, nil
}

func getPMR(ID string) (PMR, error) {
	var ctx = context.Background()
	rdb := redis.NewClient(&redis.Options{
		Addr:     "127.0.0.1:6379",
		Password: "",
		DB:       0,
	})

	var pmr PMR
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

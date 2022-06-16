package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/goccy/go-json"
	"github.com/livekit/protocol/livekit"
	lksdk "github.com/livekit/server-sdk-go"
	"github.com/skip2/go-qrcode"
)

type LivekitServerConfig struct {
	Host      string
	ApiKey    string
	ApiSecret string
}

type RoomMetaData struct {
	RealName string `json:"real_name"`
}

func main() {
	serverConfig := LivekitServerConfig{
		Host:      "http://127.0.0.1:7880",
		ApiKey:    "key_62737293bad5e",
		ApiSecret: "627372c4b4756",
	}

	r := gin.Default()

	r.Static("/assets", "./assets")
	r.StaticFile("/favicon.ico", "./favicon.ico")
	r.StaticFile("/manifest.json", "./manifest.json")
	r.StaticFile("/sw.js", "./sw.js")

	r.LoadHTMLGlob("templates/*")

	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{
			"verStatic": time.Now().Unix(),
		})
	})

	r.GET("/:room", func(c *gin.Context) {
		room := c.Param("room")
		data := gin.H{
			"creationTime":     0,
			"requiredPasscode": false,
			"verStatic":        time.Now().Unix(),
		}
		if room != "" {
			roomClient := lksdk.NewRoomServiceClient(serverConfig.Host, serverConfig.ApiKey, serverConfig.ApiSecret)
			res, err := roomClient.ListRooms(context.Background(), &livekit.ListRoomsRequest{
				Names: []string{room},
			})
			if err != nil {
				c.Redirect(http.StatusFound, "/")
				return
			} else {
				log.Printf("room detail: %v", res)
				if len(res.Rooms) != 1 {
					c.Redirect(http.StatusFound, "/")
					return
				}

				roomName := res.Rooms[0].Name
				if res.Rooms[0].Metadata != "" {
					metaData := RoomMetaData{}
					err := json.Unmarshal([]byte(res.Rooms[0].Metadata), &metaData)
					if err != nil {
						c.Redirect(http.StatusFound, "/")
						return
					}
					if metaData.RealName != "" {
						roomName = metaData.RealName
					}
				} else {
					c.Redirect(http.StatusFound, "/")
					return
				}

				var ctx = context.Background()
				rdb := redis.NewClient(&redis.Options{
					Addr:     "127.0.0.1:6379",
					Password: "",
					DB:       0,
				})
				key := "room_info:" + room
				passcode, err := rdb.Get(ctx, key).Result()
				if err != nil && err != redis.Nil {
					panic(err)
				}

				data = gin.H{
					"creationTime":     res.Rooms[0].CreationTime,
					"room":             room,
					"roomName":         roomName,
					"requiredPasscode": passcode != "",
					"verStatic":        time.Now().Unix(),
				}
			}
		}

		c.HTML(http.StatusOK, "index.html", data)
	})

	r.GET("/avatar/*path", func(c *gin.Context) {
		srcPath := c.Param("path")
		if srcPath == "" {
			c.String(404, "Not found")
			return
		}
		srcPath = "./upload/avatar/" + srcPath

		extension := filepath.Ext(srcPath)
		resizePath := srcPath[0:len(srcPath)-len(extension)] + "_200x200" + extension
		log.Printf("%s - %s", srcPath, resizePath)
		var fileAvatar string
		if _, err := os.Stat(srcPath); err == nil {
			err := resize(srcPath, 200, 200, resizePath)
			if err == nil {
				fileAvatar = resizePath
				log.Printf("resize %s", resizePath)
			} else {
				log.Printf("failed to save image: %v", err)
			}
		} else {
			log.Printf("not found: %v", err)
		}

		if fileAvatar != "" {
			file, err := os.Open(fileAvatar)
			if err != nil {
				log.Println(err)
			}
			defer file.Close()

			fileInfo, _ := file.Stat()
			bytes := make([]byte, fileInfo.Size())

			buffer := bufio.NewReader(file)
			buffer.Read(bytes)

			c.Set("image-byte", bytes)
			c.Data(http.StatusOK, http.DetectContentType(bytes), bytes)
		} else {
			c.String(404, "Not found")
		}
	})

	r.POST("/upload-avatar", func(c *gin.Context) {
		file, err := c.FormFile("file")
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"status":  0,
				"message": "Upload failed. Please try again",
			})
			return
		}

		limitSize := 10
		if file.Size > int64(limitSize)*1024*1024 {
			c.JSON(http.StatusOK, gin.H{
				"status":  0,
				"message": fmt.Sprintf("Dung lượng file quá lớn, cho phép tối đa %dMb", limitSize),
			})
			return
		}
		log.Println(file.Filename)
		log.Printf("%v\n", file.Size)

		contentType := file.Header["Content-Type"][0]
		if contentType != "image/jpeg" && contentType != "image/png" && contentType != "image/gif" {
			c.JSON(http.StatusOK, gin.H{
				"status":  0,
				"message": fmt.Sprintf("File không hợp lệ %s", contentType),
			})
			return
		}

		extension := strings.ToLower(filepath.Ext(file.Filename))
		if extension != ".jpg" && extension != ".jpeg" && extension != ".png" && extension != ".gif" {
			c.JSON(http.StatusOK, gin.H{
				"status":  0,
				"message": fmt.Sprintf("Không hỗ trợ định dạng này %s", extension),
			})
			return
		}

		year, month, day := time.Now().Date()
		strMonth := strconv.Itoa(int(month))
		if month < 10 {
			strMonth = "0" + strMonth
		}
		strDay := strconv.Itoa(day)
		if day < 10 {
			strDay = "0" + strDay
		}

		fileName := strconv.Itoa(rand.Intn(9999999999-1000000000)) + "_" + strconv.Itoa(int(time.Now().Unix())) + extension
		subPathImage := "/" + strconv.Itoa(year) + "/" + strMonth + "/" + strDay
		pathImage := "./upload/avatar" + subPathImage + "/" + fileName
		log.Println(pathImage)
		if _, err := os.Stat(pathImage); os.IsNotExist(err) {
			err := os.MkdirAll(filepath.Dir(pathImage), 0755)
			if err != nil {
				log.Printf("failed to create folder: %v", err)
				c.JSON(http.StatusOK, gin.H{
					"status":  0,
					"message": "Failed to create folder",
				})
				return
			}
		}
		err = c.SaveUploadedFile(file, pathImage)
		if err == nil {
			c.JSON(http.StatusOK, gin.H{
				"status":  1,
				"message": "/avatar" + subPathImage + "/" + fileName,
			})
			return
		}

		log.Printf("upload failed: %v", err)
		c.JSON(http.StatusOK, gin.H{
			"status":  0,
			"message": "Upload fail",
		})
	})

	r.GET("/qr-code/:id", func(c *gin.Context) {
		id := c.Param("id")
		if id == "" {
			c.String(404, "Not found")
			return
		}

		var png []byte
		png, err := qrcode.Encode("https://meeting.fptonline.net/"+id, qrcode.Medium, 256)
		if err != nil {
			log.Println(err)
			c.String(404, "Not found")
			return
		}
		c.Set("image-byte", png)
		c.Data(http.StatusOK, http.DetectContentType(png), png)
	})

	r.Run(":8080")
}

func resize(src string, w int, h int, savePath string) error {
	srcImage, err := imaging.Open(src)
	if err != nil {
		log.Printf("failed to open image: %v", err)
		return err
	}

	dstImageFill := imaging.Fill(srcImage, w, h, imaging.Center, imaging.Lanczos)
	if _, err := os.Stat(savePath); os.IsNotExist(err) {
		err := os.MkdirAll(filepath.Dir(savePath), 0755)
		if err != nil {
			log.Printf("failed to create folder: %v", err)
			return err
		}
	}

	err = imaging.Save(dstImageFill, savePath)
	if err == nil {
		log.Printf("from resize %s", savePath)
	} else {
		log.Printf("failed to save image: %v", err)
		return err
	}

	return nil
}

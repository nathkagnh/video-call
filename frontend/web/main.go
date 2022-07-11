package main

import (
	"bufio"
	"context"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"regexp"

	// "regexp"
	"strconv"
	"strings"
	"time"

	"github.com/catcombo/go-staticfiles"
	"github.com/disintegration/imaging"

	// "github.com/gin-contrib/sessions"
	// "github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/goccy/go-json"
	"github.com/livekit/protocol/livekit"
	lksdk "github.com/livekit/server-sdk-go"
	"github.com/skip2/go-qrcode"
	// "gopkg.in/ldap.v2"
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

type PMR struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Host         string `json:"host"`
	HostName     string `json:"host_name"`
	Passcode     string `json:"passcode"`
	CreationTime int32  `json:"creation_time"`
}

type Login struct {
	User     string `json:"user" form:"user"`
	Password string `json:"password" form:"password"`
}

// var Secret = []byte("secret")

// const Userkey = "user"

func main() {
	serverConfig := LivekitServerConfig{
		Host:      "http://127.0.0.1:7880",
		ApiKey:    "key_62737293bad5e",
		ApiSecret: "627372c4b4756",
	}

	r := gin.Default()

	r.Static("/assets", "./staticfiles")
	r.StaticFile("/favicon.ico", "./favicon.ico")
	r.StaticFile("/manifest.json", "./manifest.json")
	r.StaticFile("/sw.js", "./sw.js")

	storage, err := staticfiles.NewStorage("staticfiles")
	if err != nil {
		log.Fatal(err)
	}
	storage.AddInputDir("assets")
	storage.RegisterRule(postProcessCSS)
	err = storage.CollectStatic()
	if err != nil {
		log.Fatal(err)
	}
	r.SetFuncMap(template.FuncMap{
		"static": func(relPath string) string {
			return "assets/" + storage.Resolve(relPath)
		},
	})
	r.LoadHTMLGlob("templates/*")

	// r.Use(sessions.Sessions("session", cookie.NewStore(Secret)))

	// r.GET("/login", func(c *gin.Context) {
	// 	redirectUrl := c.DefaultQuery("redirect", "/")
	// 	session := sessions.Default(c)
	// 	user := session.Get(Userkey)
	// 	if user != nil {
	// 		c.Redirect(http.StatusMovedPermanently, redirectUrl)
	// 		return
	// 	}

	// 	c.HTML(http.StatusOK, "login.html", gin.H{})
	// })

	// r.POST("/login", func(c *gin.Context) {
	// 	redirectUrl := c.DefaultQuery("redirect", "/")
	// 	session := sessions.Default(c)
	// 	user := session.Get(Userkey)
	// 	if user != nil {
	// 		c.Redirect(http.StatusMovedPermanently, redirectUrl)
	// 		return
	// 	}

	// 	var loginInfo Login
	// 	if c.ShouldBind(&loginInfo) != nil && loginInfo.User != "" && loginInfo.Password != "" {
	// 		c.HTML(http.StatusOK, "login.html", gin.H{
	// 			"error":     "Incorrect username or password",
	// 		})
	// 		return
	// 	}

	// 	loginInfo.User = regexp.MustCompile(`(@.*)$`).ReplaceAllString(loginInfo.User, "")

	// 	log.Printf("DEBUG: %v", loginInfo)
	// 	if err := checkUserPass(loginInfo); err != nil {
	// 		c.HTML(http.StatusOK, "login.html", gin.H{
	// 			"user":      loginInfo.User,
	// 			"password":  loginInfo.Password,
	// 			"error":     "Incorrect username or password",
	// 		})
	// 		return
	// 	}

	// 	session.Set(Userkey, loginInfo.User)
	// 	if err := session.Save(); err != nil {
	// 		c.HTML(http.StatusOK, "login.html", gin.H{
	// 			"user":      loginInfo.User,
	// 			"password":  loginInfo.Password,
	// 			"error":     "Failed to save session",
	// 		})
	// 		return
	// 	}

	// 	c.Redirect(http.StatusMovedPermanently, redirectUrl)
	// })

	// r.GET("/logout", func(c *gin.Context) {
	// 	redirectUrl := c.DefaultQuery("redirect", "/")
	// 	session := sessions.Default(c)
	// 	user := session.Get(Userkey)
	// 	if user != nil {
	// 		session.Delete(Userkey)
	// 		_ = session.Save()
	// 	}
	// 	c.Redirect(http.StatusMovedPermanently, "/login?redirect="+redirectUrl)
	// })

	// memoryStore := persist.NewMemoryStore(1 * time.Minute)

	r.GET("/",
		// cache.CacheByRequestURI(memoryStore, 2*time.Minute),
		common(),
		func(c *gin.Context) {
			sounds := c.MustGet("sounds").(map[string]string)
			c.HTML(http.StatusOK, "index.html", gin.H{
				"sounds": sounds,
			})
		})

	r.GET("/:room", common(), func(c *gin.Context) {
		room := c.Param("room")
		sounds := c.MustGet("sounds").(map[string]string)
		data := gin.H{
			"sounds":           sounds,
			"creationTime":     0,
			"requiredPasscode": false,
		}
		if room != "" {
			data["ended"] = true
			roomClient := lksdk.NewRoomServiceClient(serverConfig.Host, serverConfig.ApiKey, serverConfig.ApiSecret)
			res, err := roomClient.ListRooms(context.Background(), &livekit.ListRoomsRequest{
				Names: []string{room},
			})
			if err == nil {
				log.Printf("room detail: %v", res)
				if len(res.Rooms) == 1 {
					roomName := res.Rooms[0].Name
					if res.Rooms[0].Metadata != "" {
						metaData := RoomMetaData{}
						err := json.Unmarshal([]byte(res.Rooms[0].Metadata), &metaData)
						if err == nil {
							if metaData.RealName != "" {
								roomName = metaData.RealName
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
								"sounds":           sounds,
								"creationTime":     res.Rooms[0].CreationTime,
								"room":             room,
								"roomName":         roomName,
								"requiredPasscode": passcode != "",
							}
						}
					}
				} else {
					pmr, err := getPMR(room)
					if err == nil && pmr.ID != "" {
						metaData := RoomMetaData{
							RealName: pmr.Name,
							Host:     pmr.Host,
						}
						metaDataJsonString, err := json.Marshal(metaData)
						if err == nil {
							roomClient := lksdk.NewRoomServiceClient(serverConfig.Host, serverConfig.ApiKey, serverConfig.ApiSecret)
							_, err = roomClient.CreateRoom(context.Background(), &livekit.CreateRoomRequest{
								Name:     pmr.ID,
								Metadata: string(metaDataJsonString),
							})
							if err == nil {
								data = gin.H{
									"sounds":           sounds,
									"room":             pmr.ID,
									"roomName":         pmr.Name,
									"requiredPasscode": true,
								}
							}
						}
					}
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
		if _, err := os.Stat(resizePath); err != nil {
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
		} else {
			fileAvatar = resizePath
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

			c.Writer.Header().Set("Cache-Control", "private, max-age=604800, immutable")
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

// func checkLogin() gin.HandlerFunc {
// 	return func(c *gin.Context) {
// 		session := sessions.Default(c)
// 		user := session.Get(Userkey)
// 		if user == nil {
// 			redirectUrl := c.FullPath()
// 			c.Redirect(http.StatusMovedPermanently, "/login?redirect="+redirectUrl)
// 			c.Abort()
// 			return
// 		}
// 	}
// }

// func checkUserPass(loginInfo Login) error {
// 	l, err := ldap.Dial("tcp", fmt.Sprintf("%s:%d", "180.148.136.1", 389))
// 	if err != nil {
// 		log.Println(err)
// 		return err
// 	}
// 	defer l.Close()
// 	l.Debug = true
// 	err = l.Bind(loginInfo.User+"@fo", loginInfo.Password)
// 	if err != nil {
// 		log.Println(err)
// 		return err
// 	}

// 	return nil
// }

func common() gin.HandlerFunc {
	return func(c *gin.Context) {
		files, err := ioutil.ReadDir("./assets/sounds")
		if err == nil {
			sounds := map[string]string{}
			for _, f := range files {
				fileName := regexp.MustCompile(`\..*?$`).ReplaceAllString(f.Name(), "")
				filePath := "sounds/" + fileName + ".wav"
				sounds[fileName] = filePath
			}

			c.Set("sounds", sounds)
		} else {
			log.Println(err)
		}
	}
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

func postProcessCSS(storage *staticfiles.Storage, file *staticfiles.StaticFile) error {
	if filepath.Ext(file.Path) != ".css" {
		return nil
	}

	buf, err := ioutil.ReadFile(file.Path)
	if err != nil {
		return err
	}

	content := string(buf)
	changed := false

	var (
		ignoreRegex = regexp.MustCompile(`^\w+:`)
		urlPatterns = []*regexp.Regexp{
			regexp.MustCompile(`url\(['"]?(?P<url>.*?)['"]?\)`),
			regexp.MustCompile(`@import\s*['"](?P<url>.*?)['"]`),
			regexp.MustCompile(`sourceMappingURL=(?P<url>[-\\.\w]+)`),
		}
	)

	for _, regex := range urlPatterns {
		content = regex.ReplaceAllStringFunc(content, func(s string) string {
			url := findSubmatchGroup(regex, s, "url")
			urlOrigin := url
			url = regexp.MustCompile(`\?.*?$`).ReplaceAllString(url, "")

			// Skip data URI schemes and absolute urls
			if ignoreRegex.MatchString(url) {
				return s
			}

			urlFileName := filepath.Base(urlOrigin)
			urlFilePath := filepath.ToSlash(filepath.Join(filepath.Dir(file.Path), url))
			for _, file := range storage.FilesMap {
				if file.Path == urlFilePath {
					hashedName := filepath.Base(file.StoragePath)
					s = strings.Replace(s, urlFileName, hashedName, 1)
					changed = true
					break
				}
			}

			return s
		})
	}

	if changed {
		err = ioutil.WriteFile(file.StoragePath, []byte(content), 0)
		if err != nil {
			return err
		}
	}

	return nil
}

func findSubmatchGroup(regex *regexp.Regexp, s, groupName string) string {
	matches := regex.FindStringSubmatch(s)

	if matches != nil {
		for i, name := range regex.SubexpNames() {
			if name == groupName {
				return matches[i]
			}
		}
	}

	return ""
}

package stream

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"backend-video-call.vnexpress.net/internal/config"
	"github.com/gorilla/websocket"
	lksdk "github.com/livekit/server-sdk-go"
)

var (
	filesOutput = []string{"output.h264", "output.ogg"}
	trackSIDs   = []string{}
)

func PublishTrackToRoom(roomName string, rtmpLink string, serverConfig config.LivekitServerConfig, ws *websocket.Conn, msgWSType int) {
	room, err := lksdk.ConnectToRoom(serverConfig.Host, lksdk.ConnectInfo{
		APIKey:              serverConfig.ApiKey,
		APISecret:           serverConfig.ApiSecret,
		RoomName:            roomName,
		ParticipantIdentity: "bot",
		ParticipantName:     "bot",
	})
	if err != nil {
		panic(err)
	}
	defer room.Disconnect()

	done := make(chan bool)

	room.Callback.OnDisconnected = func() {
		ws.WriteMessage(msgWSType, []byte("room disconnected"))
		ws.Close()
		done <- true
	}

	go ffmpegRun(rtmpLink, ws, msgWSType, done)
	defer cleanupFilesOutput()
	cleanupFilesOutput()
	publishTracks(room)

	<-done
	ws.WriteMessage(msgWSType, []byte("room disconnected"))
	ws.Close()
}

func cleanupFilesOutput() {
	if len(filesOutput) == 0 {
		return
	}
	for _, f := range filesOutput {
		if _, err := os.Stat(f); err == nil {
			os.Remove(f)
		}
	}
}

func ffmpegRun(rtmpLink string, ws *websocket.Conn, msgWSType int, done chan bool) {
	// cmd := exec.Command("ffmpeg", "-i", "rtmp://111.65.249.25:1930/live_10s_720/testmeeting", "-c:v", "libvpx", "-preset", "veryfast", "-b:v", "3000k", "-maxrate", "3000k", "-bufsize", "6000k", "output.ivf", "-c:a", "libopus", "-page_duration", "20000", "-vn", "output.ogg")

	cleanupFilesOutput()
	cmd := exec.Command("ffmpeg", "-i", rtmpLink, "-fs", "1M", "-c:v", "libx264", "-preset", "veryfast", "-b:v", "3000k", "-maxrate", "3000k", "-bufsize", "6000k", "-x264-params", "keyint=120", "-max_delay", "0", "-bf", "0", "output.h264", "-b:a", "64k", "-c:a", "libopus", "-page_duration", "20000", "-vn", "output.ogg")
	ws.WriteMessage(msgWSType, []byte(cmd.String()))
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		ws.WriteMessage(msgWSType, []byte(err.Error()))
		return
	}
	cmd.Stderr = cmd.Stdout
	err = cmd.Start()
	if err != nil {
		ws.WriteMessage(msgWSType, []byte(err.Error()))
		return
	}
	defer cmd.Process.Kill()

	errorTimes := 0
	for {
		tmp := make([]byte, 1024)
		_, err := stdout.Read(tmp)
		if err != nil {
			err := ws.WriteMessage(msgWSType, []byte(err.Error()))
			if err != nil {
				errorTimes++
			} else {
				errorTimes = 0
			}
		} else {
			if string(tmp) != "" {
				err := ws.WriteMessage(msgWSType, tmp)
				if err != nil {
					errorTimes++
				} else {
					errorTimes = 0
				}
			}
		}
		if errorTimes > 5 {
			cmd.Process.Kill()
			break
		}
		log.Println(string(tmp))
	}

	done <- true
	cleanupFilesOutput()
}

func publishTrack(file string, room *lksdk.Room) error {
	log.Printf("publish track: %v", file)
	var pub *lksdk.LocalTrackPublication
	opts := []lksdk.FileSampleProviderOption{
		lksdk.FileTrackWithOnWriteComplete(func() {
			fmt.Println("finished writing file", file)
			if len(trackSIDs) > 0 {
				log.Printf("unpublish track: %v", trackSIDs)
				for _, SID := range trackSIDs {
					err := room.LocalParticipant.UnpublishTrack(SID)
					if err != nil {
						log.Printf("unpublish track error: %v-%v", SID, err)
					}
				}
				trackSIDs = nil
			}

			publishTrack(file, room)
		}),
	}
	ext := filepath.Ext(file)
	if ext == ".h264" || ext == ".ivf" {
		opts = append(opts, lksdk.FileTrackWithFrameDuration(33*time.Millisecond))
	} else {
		opts = append(opts, lksdk.FileTrackWithFrameDuration(20*time.Millisecond))
	}
	track, err := lksdk.NewLocalFileTrack(file, opts...)
	if err != nil {
		log.Printf("create new local file track error: %v", err)
		return err
	}
	if pub, err = room.LocalParticipant.PublishTrack(track, &lksdk.TrackPublicationOptions{
		Name: file,
	}); err != nil {
		log.Printf("publish track error: %v", err)
		return err
	}
	trackSIDs = append(trackSIDs, pub.SID())
	return nil
}

func publishTracks(room *lksdk.Room) {
	// only 2 tracks: video & audio
	if len(trackSIDs) >= 2 {
		return
	}
	log.Println("waiting for tracks")
	for {
		counter := 0
		for _, f := range filesOutput {
			if fileInfo, err := os.Stat(f); err == nil && fileInfo.Size() > 1024 {
				counter++
			}
		}
		if counter == len(filesOutput) {
			break
		}
	}
	for _, f := range filesOutput {
		publishTrack(f, room)
	}
}

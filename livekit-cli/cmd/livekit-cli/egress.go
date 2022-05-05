package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"math"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ggwhite/go-masker"
	"github.com/pkg/browser"
	"github.com/urfave/cli/v2"
	"google.golang.org/protobuf/encoding/protojson"

	provider2 "github.com/livekit/livekit-cli/pkg/provider"
	"github.com/livekit/protocol/egress"
	"github.com/livekit/protocol/livekit"
	lksdk "github.com/livekit/server-sdk-go"
)

const egressCategory = "Egress"

var (
	EgressCommands = []*cli.Command{
		{
			Name:     "start-egress",
			Usage:    "Start egress",
			Before:   createEgressClient,
			Action:   startEgress,
			Category: egressCategory,
			Flags: []cli.Flag{
				urlFlag,
				apiKeyFlag,
				secretFlag,
				verboseFlag,
				&cli.StringFlag{
					Name:     "request",
					Usage:    "StartEgressRequest as json file (see https://github.com/livekit/livekit-recorder#request)",
					Required: true,
				},
			},
		},
		{
			Name:     "list-egress",
			Usage:    "List all active egress",
			Before:   createEgressClient,
			Action:   listEgress,
			Category: egressCategory,
			Flags: []cli.Flag{
				urlFlag,
				apiKeyFlag,
				secretFlag,
				&cli.StringFlag{
					Name:     "room",
					Usage:    "limits list to a certain room name",
					Required: false,
				},
			},
		},
		{
			Name:     "update-layout",
			Usage:    "Updates layout for a live room composite egress",
			Before:   createEgressClient,
			Action:   updateLayout,
			Category: egressCategory,
			Flags: []cli.Flag{
				urlFlag,
				apiKeyFlag,
				secretFlag,
				&cli.StringFlag{
					Name:     "id",
					Usage:    "Egress ID",
					Required: true,
				},
				&cli.StringFlag{
					Name:     "layout",
					Usage:    "new web layout",
					Required: true,
				},
			},
		},
		{
			Name:     "update-stream",
			Usage:    "Adds or removes rtmp output urls from a live stream",
			Before:   createEgressClient,
			Action:   updateStream,
			Category: egressCategory,
			Flags: []cli.Flag{
				urlFlag,
				apiKeyFlag,
				secretFlag,
				&cli.StringFlag{
					Name:     "id",
					Usage:    "Egress ID",
					Required: true,
				},
				&cli.StringSliceFlag{
					Name:     "add-urls",
					Usage:    "urls to add",
					Required: false,
				},
				&cli.StringSliceFlag{
					Name:     "remove-urls",
					Usage:    "urls to remove",
					Required: false,
				},
			},
		},
		{
			Name:     "stop-egress",
			Usage:    "Stop egress",
			Before:   createEgressClient,
			Action:   stopEgress,
			Category: egressCategory,
			Flags: []cli.Flag{
				urlFlag,
				apiKeyFlag,
				secretFlag,
				&cli.StringFlag{
					Name:     "id",
					Usage:    "Egress ID",
					Required: true,
				},
			},
		},
		{
			Name:     "test-egress-template",
			Usage:    "See what your egress template will look like in a recording",
			Category: egressCategory,
			Action:   testEgressTemplate,
			Flags: []cli.Flag{
				urlFlag,
				apiKeyFlag,
				secretFlag,
				&cli.StringFlag{
					Name:     "base-url (e.g. https://recorder.livekit.io/#)",
					Usage:    "base template url",
					Required: true,
				},
				&cli.StringFlag{
					Name:     "layout",
					Usage:    "layout name",
					Required: true,
				},
				&cli.IntFlag{
					Name:     "publishers",
					Usage:    "number of publishers",
					Required: true,
				},
				&cli.StringFlag{
					Name:     "room",
					Usage:    "name of the room",
					Required: false,
				},
			},
			SkipFlagParsing:        false,
			HideHelp:               false,
			HideHelpCommand:        false,
			Hidden:                 false,
			UseShortOptionHandling: false,
			HelpName:               "",
			CustomHelpTemplate:     "",
		},
	}

	egressClient *lksdk.EgressClient
)

func createEgressClient(c *cli.Context) error {
	url := c.String("url")
	apiKey := c.String("api-key")
	apiSecret := c.String("api-secret")

	if c.Bool("verbose") {
		fmt.Printf("creating client to %s, with api-key: %s, secret: %s\n",
			url,
			masker.ID(apiKey),
			masker.ID(apiSecret))
	}

	egressClient = lksdk.NewEgressClient(url, apiKey, apiSecret)
	return nil
}

func startEgress(c *cli.Context) error {
	reqFile := c.String("request")
	reqBytes, err := ioutil.ReadFile(reqFile)
	if err != nil {
		return err
	}
	req := &livekit.RoomCompositeEgressRequest{}
	err = protojson.Unmarshal(reqBytes, req)
	if err != nil {
		return err
	}

	if c.Bool("verbose") {
		PrintJSON(req)
	}

	res, err := egressClient.StartRoomCompositeEgress(context.Background(), req)
	if err != nil {
		return err
	}

	fmt.Printf("Egress started. Egress ID: %s\n", res.EgressId)
	return nil
}

func listEgress(c *cli.Context) error {
	res, err := egressClient.ListEgress(context.Background(), &livekit.ListEgressRequest{
		RoomName: c.String("room"),
	})

	for _, item := range res.Items {
		fmt.Printf("%v (%v)\n", item.EgressId, item.Status)
	}
	return err
}

func updateLayout(c *cli.Context) error {
	_, err := egressClient.UpdateLayout(context.Background(), &livekit.UpdateLayoutRequest{
		EgressId: c.String("id"),
		Layout:   c.String("layout"),
	})
	return err
}

func updateStream(c *cli.Context) error {
	_, err := egressClient.UpdateStream(context.Background(), &livekit.UpdateStreamRequest{
		EgressId:         c.String("id"),
		AddOutputUrls:    c.StringSlice("add-urls"),
		RemoveOutputUrls: c.StringSlice("remove-urls"),
	})
	return err
}

func stopEgress(c *cli.Context) error {
	_, err := egressClient.StopEgress(context.Background(), &livekit.StopEgressRequest{
		EgressId: c.String("id"),
	})
	return err
}

func testEgressTemplate(c *cli.Context) error {
	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	numPublishers := c.Int("publishers")
	rooms := make([]*lksdk.Room, 0, numPublishers)
	defer func() {
		for _, room := range rooms {
			room.Disconnect()
		}
	}()

	roomName := c.String("room")
	if roomName == "" {
		roomName = fmt.Sprintf("layout-demo-%v", time.Now().Unix())
	}

	serverURL := c.String("url")
	apiKey := c.String("api-key")
	apiSecret := c.String("api-secret")

	for i := 0; i < numPublishers; i++ {
		room, err := lksdk.ConnectToRoom(serverURL, lksdk.ConnectInfo{
			APIKey:              apiKey,
			APISecret:           apiSecret,
			RoomName:            roomName,
			ParticipantIdentity: fmt.Sprintf("demo-publisher-%d", i),
		})
		if err != nil {
			return err
		}

		rooms = append(rooms, room)

		var tracks []*lksdk.LocalSampleTrack
		for q := livekit.VideoQuality_LOW; q <= livekit.VideoQuality_HIGH; q++ {
			height := 180 * int(math.Pow(2, float64(q)))
			provider, err := provider2.ButterflyLooper(height)
			if err != nil {
				return err
			}
			track, err := lksdk.NewLocalSampleTrack(provider.Codec(),
				lksdk.WithSimulcast(fmt.Sprintf("demo-video-%d", i), provider.ToLayer(q)),
			)
			if err != nil {
				return err
			}
			if err = track.StartWrite(provider, nil); err != nil {
				return err
			}
			tracks = append(tracks, track)
		}

		_, err = room.LocalParticipant.PublishSimulcastTrack(tracks, &lksdk.TrackPublicationOptions{
			Name: fmt.Sprintf("demo-%d", i),
		})
		if err != nil {
			return err
		}
	}

	token, err := egress.BuildEgressToken("template_test", apiKey, apiSecret, roomName)
	if err != nil {
		return err
	}

	templateURL := fmt.Sprintf(
		"%s/%s?url=%s&token=%s",
		c.String("base-url"), c.String("layout"), url.QueryEscape(serverURL), token,
	)
	if err := browser.OpenURL(templateURL); err != nil {
		return err
	}

	<-done
	return nil
}

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/harshilsharma63/mattermost-plugin-chess/server/chess"

	"github.com/mattermost/mattermost-plugin-api/cluster"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"

	"github.com/pkg/errors"
)

const (
	commandSubscribe      = "chesspuzzlesubscribe"
	commandUnsubscribe    = "chesspuzzleunsubscribe"
	keySubscribedChannels = "subscriptions"
)

var API plugin.API
var Helpers plugin.Helpers
var BotUserID string

// Plugin implements the interface expected by the Mattermost server to communicate between the server and plugin processes.
type Plugin struct {
	plugin.MattermostPlugin

	// configurationLock synchronizes access to the configuration.
	configurationLock sync.RWMutex

	// configuration is the active plugin configuration. Consult getConfiguration and
	// setConfiguration for usage.
	configuration *configuration

	job *cluster.Job
}

// ServeHTTP demonstrates a plugin that handles HTTP requests by greeting the world.
func (p *Plugin) ServeHTTP(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Hello, world!")
}

func (p *Plugin) OnActivate() error {
	API = p.API
	Helpers = p.Helpers

	if err := p.registerCommand(); err != nil {
		return err
	}

	botUserID, err := p.ensureBot()
	if err != nil {
		return err
	}

	BotUserID = botUserID

	if err := p.Run(); err != nil {
		return err
	}

	return nil
}

func (p *Plugin) registerCommand() error {
	err := p.API.RegisterCommand(&model.Command{
		Trigger:      commandSubscribe,
		AutoComplete: true,
	})

	if err != nil {
		fmt.Println("Failed to register command " + commandSubscribe)
		return err
	}

	err = p.API.RegisterCommand(&model.Command{
		Trigger:      commandUnsubscribe,
		AutoComplete: true,
	})

	if err != nil {
		fmt.Println("Failed to register command " + commandUnsubscribe)
		return err
	}

	return nil
}

func (p *Plugin) ensureBot() (string, error) {
	bot := &model.Bot{
		Username:    "chesspuzzlebot",
		DisplayName: "Chess Puzzle Bot",
		Description: "For now I'm a friendly bot who posts daily chess puzzzles from chess.com. But one day I'll take over the world ????",
	}

	botID, err := p.Helpers.EnsureBot(bot)

	if err != nil {
		fmt.Println(err.Error())
		return "", err
	}

	bundlePath, err := p.API.GetBundlePath()
	if err != nil {
		return "", errors.Wrap(err, "couldn't get bundle path")
	}

	profileImage, err := ioutil.ReadFile(filepath.Join(bundlePath, "assets", "profile.png"))
	if err != nil {
		return "", errors.Wrap(err, "couldn't read profile image")
	}

	appErr := p.API.SetProfileImage(botID, profileImage)
	if appErr != nil {
		return "", errors.Wrap(appErr, "couldn't set profile image")
	}

	return botID, nil
}

func (p *Plugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	switch strings.TrimSpace(args.Command) {
	case "/" + commandSubscribe:
		return p.executeSubscribe(c, args)
	case "/" + commandUnsubscribe:
		return p.executeUnsubscribe(c, args)
	default:
		return nil, nil
	}
}

func (p *Plugin) executeSubscribe(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	subscriptions, err := GetSubscribedChannels()
	if err != nil {
		return nil, model.NewAppError("", "", nil, err.Error(), 500)
	}

	if subscriptions == nil {
		subscriptions = map[string]int{}
	}

	if _, exists := subscriptions[args.ChannelId]; exists {
		return &model.CommandResponse{
			Text:      "Channel is already subscribed",
			ChannelId: args.ChannelId,
		}, nil
	}

	subscriptions[args.ChannelId] = 0
	if err := SetSubscribedChannels(subscriptions); err != nil {
		return nil, model.NewAppError("", "", nil, err.Error(), 500)
	}

	puzzle, err := chess.Chess{}.GetDailyPuzzle()
	if err != nil {
		return nil, model.NewAppError("", "", nil, err.Error(), 500)
	}

	_ = puzzle.Post(args.ChannelId, BotUserID, p.API)

	return &model.CommandResponse{
		Text:      "Subscribed successfully.",
		ChannelId: args.ChannelId,
	}, nil
}

func (p *Plugin) executeUnsubscribe(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	subscriptions, err := GetSubscribedChannels()
	if err != nil {
		return nil, model.NewAppError("", "", nil, err.Error(), 500)
	}

	if subscriptions == nil {
		subscriptions = map[string]int{}
	}

	if _, exists := subscriptions[args.ChannelId]; !exists {
		return &model.CommandResponse{
			Text:      "Channel is already unsubscribed",
			ChannelId: args.ChannelId,
		}, nil
	}

	delete(subscriptions, args.ChannelId)
	if err := SetSubscribedChannels(subscriptions); err != nil {
		return nil, model.NewAppError("", "", nil, err.Error(), 500)
	}

	return &model.CommandResponse{
		Text:      "Unsubscribed successfully.",
		ChannelId: args.ChannelId,
	}, nil
}

func (p *Plugin) Run() error {
	if p.job != nil {
		if err := p.job.Close(); err != nil {
			return err
		}
	}

	job, err := cluster.Schedule(
		p.API,
		"DailyChessPuzzleJob",
		cluster.MakeWaitForInterval(30*time.Minute),
		func() {
			_ = Runner()
		},
	)

	if err != nil {
		p.API.LogError(fmt.Sprintf("Unable to schedule job for standup reports. Error: {%s}", err.Error()))
		return err
	}

	p.job = job
	return nil
}

func Runner() error {
	puzzle, err := chess.Chess{}.GetDailyPuzzle()
	if err != nil {
		return err
	}

	subscriptions, err := GetSubscribedChannels()
	if err != nil {
		return err
	}

	for channelID, lastPublishedPuzzle := range subscriptions {
		if lastPublishedPuzzle == puzzle.PublishTime {
			continue
		}

		if err := puzzle.Post(channelID, BotUserID, API); err != nil {
			continue
		}

		subscriptions[channelID] = puzzle.PublishTime
	}

	_ = SetSubscribedChannels(subscriptions)

	return nil
}

func GetSubscribedChannels() (map[string]int, error) {
	var channels map[string]int
	if _, err := Helpers.KVGetJSON(keySubscribedChannels, &channels); err != nil {
		fmt.Println(err.Error())
		return nil, err
	}

	return channels, nil
}

func SetSubscribedChannels(subscription map[string]int) error {
	data, err := json.Marshal(subscription)
	if err != nil {
		fmt.Println(err.Error())
		return err
	}

	if err := API.KVSet(keySubscribedChannels, data); err != nil {
		fmt.Println(err.Error())
		return errors.New(err.Error())
	}

	return nil
}

// See https://developers.mattermost.com/extend/plugins/server/reference/

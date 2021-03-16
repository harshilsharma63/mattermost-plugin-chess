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
	"github.com/harshilsharma63/mattermost-plugin-chess/server/puzzle"
	"github.com/mattermost/mattermost-plugin-api/cluster"
	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/mattermost/mattermost-server/v5/plugin"
	"github.com/pkg/errors"
)

const (
	commandTrigger = "chesspuzzlesubscribe"

	keyLastPublishedPuzzle = "last_puzzle_timestamp"
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

	job     *cluster.Job
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
	return p.API.RegisterCommand(&model.Command{
		Trigger: commandTrigger,
		AutoComplete: true,
	})
}

func (p *Plugin) ensureBot() (string, error) {
	bot := &model.Bot{
		Username: "chesspuzzlebot",
		DisplayName: "Chess Puzzle Bot",
		Description: "For now I'm a friendly bot who posts daily chess puzzzles from chess.com. But one day I'll take over the world 🤫",
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
	if strings.TrimSpace(args.Command) == "/" + commandTrigger {
		return p.executeSubscribe(c, args)
	}

	return nil, nil



	// puzzle, err := chess.Chess{}.GetDailyPuzzle()
	// if err != nil {
	// 	fmt.Println(err.Error())
	// 	return nil, model.NewAppError("while fetching puzzle", "", nil, err.Error(), 500)
	// }

	// post := puzzle.ToPost(args.ChannelId, BotUserID)
	// createdPost, appErr := p.API.CreatePost(post)
	// if appErr != nil {
	// 	fmt.Println(appErr.Error())
	// 	return nil, model.NewAppError("while posting puzzle to channel", "", nil, appErr.Error(), 500)
	// }

	// _, appErr = p.API.AddReaction(&model.Reaction{
	// 	UserId: BotUserID,
	// 	PostId: createdPost.Id,
	// 	EmojiName: "white_check_mark",
	// })

	// if appErr != nil {
	// 	fmt.Println(appErr.Error())
	// 	return nil, model.NewAppError("while adding reaction on created post", "", nil, appErr.Error(), 500)
	// }

	// return &model.CommandResponse{
	// 	Text: "Posted",
	// 	ChannelId: args.ChannelId,
	// }, nil
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
			Text: "Channel is already subscribed",
			ChannelId: args.ChannelId,
		}, nil
	}

	subscriptions[args.ChannelId] = 0
	if err := SetSubscribedChannels(subscriptions); err != nil {
		return nil, model.NewAppError("", "", nil, err.Error(), 500)
	}

	return &model.CommandResponse{
		Text: "Subscribed successfully.",
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
		cluster.MakeWaitForInterval(30 * time.Second),
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

	// lastPublishedPuzzle, err :=  GetLastPublishedPuzzle()
	// if err != nil {
	// 	return err
	// }

	// if lastPublishedPuzzle == puzzle.PublishTime {
	// 	// new puzzle is not available
	// 	return nil
	// }

	subscriptions, err := GetSubscribedChannels()
	if err != nil {
		return err
	}

	for channelID, lastPublishedPuzzle := range subscriptions {
		if lastPublishedPuzzle == puzzle.PublishTime {
			continue
		}

		post := puzzle.ToPost(channelID, BotUserID)
		createdPost, appErr := API.CreatePost(post)
		if appErr != nil {
			fmt.Println(appErr.Error())
			continue
		}

		subscriptions[channelID] = puzzle.PublishTime

		_, appErr = API.AddReaction(&model.Reaction{
			UserId: BotUserID,
			PostId: createdPost.Id,
			EmojiName: "white_check_mark",
		})

		if appErr != nil {
			fmt.Println(appErr.Error())
			continue
		}
	}

	_ = SetSubscribedChannels(subscriptions)

	return nil
}

func GetLastPublishedPuzzle() (int, error) {
	data, appErr := API.KVGet(keyLastPublishedPuzzle)
	if appErr != nil {
		fmt.Println(appErr.Error())
		return 0, appErr
	}

	if data == nil {
		data = []byte("0")
	}

	var timestamp int

	if err := json.Unmarshal(data, &timestamp); err != nil {
		fmt.Println(err.Error())
		return 0, err
	}

	return timestamp, nil
}

func SetLastPublishedPuzzle(puzzle puzzle.Puzzle) error {
	data, err := json.Marshal(puzzle.PublishTime)
	if err != nil {
		fmt.Println(err.Error())
		return err
	}

	return API.KVSet(keyLastPublishedPuzzle, data)
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

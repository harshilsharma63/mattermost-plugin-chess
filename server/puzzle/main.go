package puzzle

import (
	"fmt"
	"strconv"
	"time"

	"github.com/mattermost/mattermost-server/v5/model"
)

const (
	puzzlePostTemplate = "### Daily Puzzle - %s\n" +
			"##### %s\n" +
			"##### [Solve On Chess.com :arrow_heading_up: ](%s)\n" +
			"![](%s)"
)

// Puzzle represents a chess.com puzzle
type Puzzle struct {
	Title string `json:"title"`
	URL string `json:"url"`
	PublishTime int `json:"publish_time"`
	FEN string `json:"fen"`
	PGN string `json:"pgn"`
	Image string `json:"image"`
}

// ToPost prepapres a Mattermost post for the puzzle object
func (p Puzzle) ToPost(channelID, userID string) *model.Post {
	i, err := strconv.ParseInt("1405544146", 10, 64)
    if err != nil {
        panic(err)
    }
    puzzleDate := time.Unix(i, 0)

	msg := fmt.Sprintf(
		puzzlePostTemplate,
		puzzleDate.Format("1/2/2006"),
		p.Title,
		p.URL,
		p.Image,
	)


	return &model.Post{
		ChannelId: channelID,
		UserId: userID,
		Message: msg,
	}
}

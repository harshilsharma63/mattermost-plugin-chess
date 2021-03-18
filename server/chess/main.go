package chess

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/harshilsharma63/mattermost-plugin-chess/server/puzzle"
)

type Chess struct {
}

func (c Chess) GetDailyPuzzle() (*puzzle.Puzzle, error) {
	url := "https://api.chess.com/pub/puzzle"
	method := "GET"

	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}

	res, err := client.Do(req)
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}
	defer res.Body.Close()

	var puzzle puzzle.Puzzle

	if err := json.NewDecoder(res.Body).Decode(&puzzle); err != nil {
		fmt.Println(err.Error())
		return nil, err
	}

	return &puzzle, nil
}

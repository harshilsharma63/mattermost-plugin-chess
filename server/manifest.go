// This file is automatically generated. Do not modify it manually.

package main

import (
	"strings"

	"github.com/mattermost/mattermost-server/v5/model"
)

var manifest *model.Manifest

const manifestStr = `
{
  "id": "com.harshilsharma63.chess",
  "name": "Chess.com Puzzles",
  "description": "This plugin shares daily puzzles from chess.com",
  "homepage_url": "https://github.com/mattermost/mattermost-plugin-starter-template",
  "support_url": "https://github.com/mattermost/mattermost-plugin-starter-template/issues",
  "release_notes_url": "https://github.com/mattermost/mattermost-plugin-starter-template/releases/tag/v0.1.0",
  "icon_path": "assets/starter-template-icon.svg",
  "version": "1.0.0",
  "min_server_version": "5.32.1",
  "server": {
    "executables": {
      "linux-amd64": "server/dist/plugin-linux-amd64",
      "darwin-amd64": "server/dist/plugin-darwin-amd64",
      "windows-amd64": "server/dist/plugin-windows-amd64.exe"
    },
    "executable": ""
  },
  "settings_schema": {
    "header": "",
    "footer": "",
    "settings": []
  }
}
`

func init() {
	manifest = model.ManifestFromJson(strings.NewReader(manifestStr))
}

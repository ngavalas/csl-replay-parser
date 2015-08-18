package main

import (
	"bytes"
	"fmt"
	"os"

	"net/http"
	"encoding/json"
	"io/ioutil"
)

type ExportedStats struct {
	Teams    map[string][]*PlayerStats `json:"teams"`
	Duration float64 `json:"duration_in_minutes"`
	Winner   string  `json:"winner"`
	MatchId  uint32  `json:"match_id"`
}

func ReportAndClean(finishedQueue <-chan *FileFinished, conf *Config) {
	for {
		file := <-finishedQueue

		out, err := json.Marshal(exportMatchStats(file.Stats))

		if err != nil {
			fmt.Println(err)
		}

		resp := ""
		tries := 0

		for resp != "ok" && tries < conf.MaxTries {
			resp = sendJSON(out, conf.RequestUrl)
			tries = tries + 1
		}

		if tries < conf.MaxTries && conf.DeleteOldFiles {
			// success, so delete
			DebugPrint("deleting %s", file.Name)
			os.Remove(file.Path)
		} else {
			// failed, so ?
		}
	}
}

func exportMatchStats(stats *MatchStats) *ExportedStats {
	radiant := make([]*PlayerStats, 0, 5)
	dire    := make([]*PlayerStats, 0, 5)

	for _, player := range stats.Players {
		if player.Slot < 4 {
			radiant = append(radiant, player)
		} else {
			dire = append(dire, player)
		}
	}

	return &ExportedStats {
		Teams: map[string][]*PlayerStats {
			"radiant": radiant,
			"dire":    dire,
		},
		Duration: stats.Duration,
		Winner: stats.Winner,
		MatchId: stats.MatchId,
	}
}

func sendJSON(data []byte, requestUrl string) string {
	req, err := http.NewRequest("POST", requestUrl, bytes.NewBuffer(data))
    req.Header.Set("Content-Type", "application/json")

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
    	return "ok"
    }
    defer resp.Body.Close()

    body, _ := ioutil.ReadAll(resp.Body)

    return string(body)
}
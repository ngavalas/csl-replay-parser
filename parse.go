package main

import (
	"fmt"
	"time"

	"github.com/dotabuff/yasha"
	"github.com/dotabuff/yasha/dota"

	"github.com/davecgh/go-spew/spew"
)

type PlayerStats struct {
	SteamId  uint64 `json:"steam_id"`
	HeroName string `json:"raw_hero_name"`
	Slot     uint   `json:"-"`
	Kills    uint   `json:"kills"`
	Deaths   uint   `json:"deaths"`
	Assists	 uint   `json:"assist"`
	Gold     uint   `json:"total_gold"`
	Xp       uint   `json:"total_experience"`
}

type MatchStats struct {
	Players  map[uint64]*PlayerStats
	Duration float64
	Winner   string
	MatchId  uint32
}

var pp = spew.Dump

func Parse(parseQueue <-chan *FilePending, finishedQueue chan<- *FileFinished) {
	for {
		file := <-parseQueue

		DebugPrint("parsing: %s at %s\n", file.Name, file.Path)
		match := &MatchStats{}
		match.Players = make(map[uint64]*PlayerStats)

		parser := yasha.ParserFromFile(file.Path)

		parsed := false

		var totalTime time.Duration
		var gameEndTime, gameStartTime float64

		parser.OnEntityPreserved = func(e *yasha.PacketEntity) {
			// once we get the data once, it won't change, so we can skip it after one load
			if parsed {
				return
			}

			if e.Name == "DT_DOTAGamerulesProxy" {
				gameEndTime = e.Values["DT_DOTAGamerules.m_flGameEndTime"].(float64)
				gameStartTime = e.Values["DT_DOTAGamerules.m_flGameStartTime"].(float64)
				totalTime = time.Duration(gameEndTime-gameStartTime) * time.Second
			}

			// the ancient has fallen, gather final stats
			if gameEndTime > 0 {
				if e.Name == "DT_DOTA_PlayerResource" {
					for i := 0; i < 10; i++ {
						suffix := fmt.Sprintf("%04d", i)
						id := e.Values["m_iPlayerSteamIDs."+suffix].(uint64)

						match.Players[id] = &PlayerStats{
							SteamId:  id,
							Slot:     uint(i),
							Kills:    e.Values["m_iKills."+suffix].(uint),
							Deaths:   e.Values["m_iDeaths."+suffix].(uint),
							Assists:  e.Values["m_iAssists."+suffix].(uint),
							Gold:     e.Values["m_iTotalEarnedGold."+suffix].(uint),
							Xp:       e.Values["m_iTotalEarnedXP."+suffix].(uint),
						}
					}

					match.Duration = totalTime.Minutes()
					parsed = true
				}
			}
		}

		parser.OnFileInfo = func(fileinfo *dota.CDemoFileInfo) {
			gameInfo := fileinfo.GetGameInfo().GetDota()

			match.MatchId = gameInfo.GetMatchId()

			// game winner team IDs are hard-coded because reasons
			if gameInfo.GetGameWinner() == 2 {
				match.Winner = "radiant"
			} else {
				match.Winner = "dire"
			}

			for _, pls := range gameInfo.GetPlayerInfo() {
				match.Players[pls.GetSteamid()].HeroName = pls.GetHeroName()
			}
		}

		parser.Parse()

		//spew.Dump(match)
		DebugPrint("done parsing: %s\n", file.Name)

		finishedQueue <- &FileFinished{Path: file.Path, Name: file.Name, Stats: match}
	}
}

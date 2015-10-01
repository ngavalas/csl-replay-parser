package main

import (
	"fmt"
	"time"

	"github.com/dotabuff/manta"
	"github.com/dotabuff/manta/dota"

	"github.com/davecgh/go-spew/spew"
)

type PlayerStats struct {
	SteamId  uint64 `json:"steam_id"`
	HeroName string `json:"raw_hero_name"`
	Slot     uint   `json:"-"`
	Kills    int32  `json:"kills"`
	Deaths   int32  `json:"deaths"`
	Assists  int32  `json:"assist"`
	Gold     int32  `json:"total_gold"`
	Xp       int32  `json:"total_experience"`
}

type MatchStats struct {
	Players  map[uint64]*PlayerStats
	Duration float64
	Winner   string
	MatchId  uint32
}

var pp = spew.Dump

func ignoreError(v int32, e bool) int32 {
	return v
}

func Parse(parseQueue <-chan *FilePending, finishedQueue chan<- *FileFinished) {
	for {
		file := <-parseQueue

		DebugPrint("parsing: %s at %s\n", file.Name, file.Path)
		match := &MatchStats{}
		match.Players = make(map[uint64]*PlayerStats)

		parser, _ := manta.NewParserFromFile(file.Path)

		parsed := false

		var totalTime time.Duration
		var gameEndTime, gameStartTime float32

		parser.OnPacketEntity(func(e *manta.PacketEntity, pet manta.EntityEventType) error {
			// once we get the data once, it won't change, so we can skip it after one load
			if parsed {
				return nil
			}

			if e.ClassName == "CDOTAGamerulesProxy" {
				gameEndTime, _ = e.FetchFloat32("CDOTAGamerules.m_flGameEndTime")
				gameStartTime, _ = e.FetchFloat32("CDOTAGamerules.m_flGameStartTime")
				totalTime = time.Duration(gameEndTime-gameStartTime) * time.Second
			}

			// the ancient has fallen, gather final stats
			if gameEndTime > 0 {
				if e.ClassName == "CDOTA_PlayerResource" {
					for i := 0; i < 10; i++ {
						suffix := fmt.Sprintf("%04d", i)
						id, _ := e.FetchUint64("m_iPlayerSteamIDs." + suffix)

						match.Players[id] = &PlayerStats{
							SteamId: id,
							Slot:    uint(i),
							Kills:   ignoreError(e.FetchInt32("m_iKills." + suffix)),
							Deaths:  ignoreError(e.FetchInt32("m_iDeaths." + suffix)),
							Assists: ignoreError(e.FetchInt32("m_iAssists." + suffix)),
							Gold:    ignoreError(e.FetchInt32("m_iTotalEarnedGold." + suffix)),
							Xp:      ignoreError(e.FetchInt32("m_iTotalEarnedXP." + suffix)),
						}
					}

					match.Duration = totalTime.Minutes()
					parsed = true
				}
			}

			return nil
		})

		parser.Callbacks.OnCDemoFileInfo(func(fileinfo *dota.CDemoFileInfo) error {
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

			return nil
		})

		_ = parser.Start()

		//spew.Dump(match)
		DebugPrint("done parsing: %s\n", file.Name)

		finishedQueue <- &FileFinished{Path: file.Path, Name: file.Name, Stats: match}
	}
}

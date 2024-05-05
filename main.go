package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gorilla/websocket"
	"github.com/markus-wa/demoinfocs-golang/common"
	ex "github.com/markus-wa/demoinfocs-golang/v4/examples"
	dem "github.com/markus-wa/demoinfocs-golang/v4/pkg/demoinfocs"
	"github.com/markus-wa/demoinfocs-golang/v4/pkg/demoinfocs/msgs2"
)

type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

type Direction struct {
	X float32 `json:"x"`
	Y float32 `json:"y"`
	Z float32 `json:"z"`
}

type PlayerEventInfo struct {
	Position     Position  `json:"position"`
	Team         string    `json:"team"`
	Name         string    `json:"name"`
	IsAlive      bool      `json:"isAlive"`
	Kills        int       `json:"kills"`
	Deaths       int       `json:"deaths"`
	Assists      int       `json:"assists"`
	Health       int       `json:"health"`
	Money        int       `json:"money"`
	Weapon       string    `json:"weapon"`
	PlayerID     int64     `json:"playerID"`
	Direction    Direction `json:"direction"`
	Equipment    []string  `json:"equipment"`
	IsBlinded    bool      `json:"isBlinded"`
	HasDefuseKit bool      `json:"hasDefuseKit"`
}

type GrenadeEventInfo struct {
	Position Position `json:"position"`
	Name     string   `json:"name"`
}

type RoundEventInfo struct {
	RoundsPlayed int      `json:"roundsPlayed"`
	RoundTime    int      `json:"roundTime"`
	TeamScoreCT  int      `json:"teamScoreCT"`
	TeamScoreT   int      `json:"teamScoreT"`
	BombTimer    int      `json:"bombTimer"`
	BombPosition Position `json:"bombPosition"`
	BombCarrier  string   `json:"bombCarrier"`
}

type EventInfo struct {
	Type        string           `json:"type"`
	PlayerInfo  PlayerEventInfo  `json:"playerEventInfo"`
	RoundInfo   RoundEventInfo   `json:"roundEventInfo"`
	GrenadeInfo GrenadeEventInfo `json:"grenadeEventInfo"`
	ServerInfo  ServerInfo       `json:"serverInfo"`
}

type ServerInfo struct {
	MapData ex.Map `json:"mapData"`
}

var (
	upgrader        = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	clients         = make(map[*websocket.Conn]bool)
	startProcessing = make(chan bool, 1)
	mapData         ex.Map
)

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()
	clients[conn] = true
	startProcessing <- true // Signal to start processing once a client connects

	for {
		_, _, err := conn.ReadMessage() // Read messages from client (not used here)
		if err != nil {
			log.Printf("error: %v", err)
			delete(clients, conn)
			break
		}
	}
}

func broadcast(event []EventInfo) {
	for client := range clients {
		err := client.WriteJSON(event)
		if err != nil {
			log.Printf("error: %v", err)
			client.Close()
			delete(clients, client)
		}
	}
}

// func serveMapImage(w http.ResponseWriter, r *http.Request) {
// http.ServeFile(w, r, mapData.ImagePath)
// }

func main() {
	http.HandleFunc("/ws", wsHandler)
	// http.HandleFunc("/map_image", serveMapImage) // Route to access the map image
	go http.ListenAndServe(":8080", nil)

	<-startProcessing // Wait for a client to connect before proceeding

	demoFile, err := os.Open("ancient4k.dem")
	if err != nil {
		log.Panic("failed to open demo file: ", err)
	}
	defer demoFile.Close()

	p := dem.NewParser(demoFile)
	defer p.Close()

	var mapCodes = map[string]uint32{
		"de_mirage":   1936772555,
		"de_anubis":   3934213780,
		"de_nuke":     4081488007,
		"de_inferno":  3201302029,
		"de_ancient":  4262714479,
		"de_overpass": 2863184063,
		"de_vertigo":  970160341,
	}

	p.RegisterNetMessageHandler(func(msg *msgs2.CSVCMsg_ServerInfo) {
		mapName := msg.GetMapName()
		crc := mapCodes[mapName]
		mapData = ex.GetMapMetadata(mapName, crc)
		// radar := ex.GetMapRadar(mapName, crc)
		// // Save the radar image for use in the frontend
		// filePath := fmt.Sprintf("%s_radar.jpg", mapName)
		// file, err := os.Create(filePath)
		// if err != nil {
		// 	log.Fatal("Could not create map image file: ", err)
		// }
		// defer file.Close()
		// jpeg.Encode(file, radar, &jpeg.Options{Quality: 95})

		// mapData.ImagePath = filePath
		broadcast([]EventInfo{
			{
				Type: "Server",
				ServerInfo: ServerInfo{
					MapData: mapData,
				},
			},
		})
	})

	p.RegisterEventHandler(func(e any) {
		if !p.GameState().IsMatchStarted() {
			return
		}

		var tickEvents = []EventInfo{}
		var participants = p.GameState().Participants().Playing()
		for _, player := range participants {
			var position = player.Position()
			translatedX, translatedY := mapData.TranslateScale(position.X, position.Y)

			// Get Player Equipment
			var weaponType string
			equipment := []string{}
			for _, item := range player.Inventory {
				equipmentName := item.Type.String()
				equipment = append(equipment, equipmentName)
			}

			// Player
			var event = EventInfo{
				Type: "Player",
				PlayerInfo: PlayerEventInfo{
					Position: Position{X: translatedX, Y: translatedY},
					Team:     getTeam(common.Team(player.Team)),
					Name:     player.Name,
					IsAlive:  player.IsBlinded(),
					Kills:    player.Kills(),
					Deaths:   player.Deaths(),
					Assists:  player.Assists(),
					Health:   player.Health(),
					Money:    player.Money(),
					Weapon:   weaponType,
					PlayerID: int64(player.SteamID64),
					Direction: Direction{
						X: player.ViewDirectionX(),
						Y: player.ViewDirectionY(),
					},
					IsBlinded:    player.IsBlinded(),
					Equipment:    equipment,
					HasDefuseKit: player.HasDefuseKit(),
				},
			}

			tickEvents = append(tickEvents, event)
		}

		// Projectile
		var grenades = p.GameState().GrenadeProjectiles()
		for _, grenade := range grenades {
			var position = grenade.Position()
			translatedX, translatedY := mapData.TranslateScale(position.X, position.Y)

			var grenadeType = grenade.WeaponInstance.Type
			var event = EventInfo{
				Type: "Grenade",
				GrenadeInfo: GrenadeEventInfo{
					Position: Position{X: translatedX, Y: translatedY},
					Name:     grenadeType.String(),
				},
			}
			tickEvents = append(tickEvents, event)
		}

		// Round
		var roundsPlayed = p.GameState().TotalRoundsPlayed()
		var roundTime = p.GameState().IngameTick()
		var teamScoreCT = p.GameState().TeamCounterTerrorists().Score()
		var teamScoreT = p.GameState().TeamTerrorists().Score()
		var bomb = p.GameState().Bomb()
		var bombPosition = bomb.Position()

		translatedX, translatedY := mapData.TranslateScale(bombPosition.X, bombPosition.Y)

		var event = EventInfo{
			Type: "Round",
			RoundInfo: RoundEventInfo{
				RoundsPlayed: roundsPlayed,
				RoundTime:    roundTime,
				TeamScoreCT:  teamScoreCT,
				TeamScoreT:   teamScoreT,
				BombPosition: Position{X: translatedX, Y: translatedY},
			},
		}
		tickEvents = append(tickEvents, event)

		// fmt.Println(tickEvents)
		broadcast(tickEvents)
	})

	p.ParseToEnd()
}

func getTeam(team common.Team) string {
	switch team {
	case common.TeamTerrorists:
		return "T"
	case common.TeamCounterTerrorists:
		return "CT"
	default:
		return "NA"
	}
}

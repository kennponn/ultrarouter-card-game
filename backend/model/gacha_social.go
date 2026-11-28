package model

import (
	cryptorand "crypto/rand"
	"encoding/binary"
	"errors"
	"math"
	"math/rand/v2"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const GachaArenaDailyLimit = 5
const gachaPowerVersion = 8

var (
	ErrGachaShopLimit    = errors.New("personal shop stock is unavailable")
	ErrGachaCapacity     = errors.New("resource capacity would be exceeded")
	ErrGachaArenaLimit   = errors.New("daily arena limit reached")
	ErrGachaOpponent     = errors.New("arena opponent is unavailable")
	ErrGachaArenaPending = errors.New("arena is not open")
)

type GachaArenaMatch struct {
	ID         string `json:"id"`
	OpponentID int    `json:"opponent_id"`
	Name       string `json:"name"`
	Win        bool   `json:"win"`
	Draw       bool   `json:"draw"`
	Points     int    `json:"points"`
	PlayedAt   int64  `json:"played_at"`
	StakeE     int    `json:"stake_e"`
	DeltaQuota int    `json:"delta_quota"`
}
type GachaArenaState struct {
	Points  int               `json:"points"`
	Wins    int               `json:"wins"`
	Played  int               `json:"played"`
	History []GachaArenaMatch `json:"history"`
}
type GachaArenaResult struct {
	GachaArenaMatch
	Report *GachaBattleReport `json:"report"`
}
type GachaPublicGirl struct {
	ID    string `json:"id"`
	Level int    `json:"level"`
	Star  int    `json:"star"`
}
type GachaCompetitor struct {
	UserID      int               `json:"user_id"`
	Name        string            `json:"name"`
	Power       int               `json:"power"`
	ArenaPoints int               `json:"arena_points"`
	Squad       []GachaPublicGirl `json:"squad" gorm:"-"`
	State       string            `json:"-"`
}
type GachaSocialView struct {
	ArenaEnabled bool              `json:"arena_enabled"`
	PowerBoard   []GachaCompetitor `json:"power_board"`
	ArenaBoard   []GachaCompetitor `json:"arena_board"`
	Opponents    []GachaCompetitor `json:"opponents"`
	PowerRank    int64             `json:"power_rank"`
	ArenaRank    int64             `json:"arena_rank"`
	ArenaLimit   int               `json:"arena_limit"`
}

// Power includes combat stats and a bounded estimate of each awakening route's utility.
func GachaSquadPower(state *GachaGameState) (int, error) {
	if state == nil {
		return 0, ErrGachaTeam
	}
	if len(state.Squad) == 0 {
		return 0, nil
	}
	units, err := gachaSquadUnits(state, "ally", "a")
	if err != nil {
		return 0, err
	}
	power := 0
	for _, u := range units {
		critExpectation := 1 + float64(u.CR)*float64(u.CD)/10_000
		base := float64(u.Atk)*2.2*critExpectation + float64(u.MaxHP)*0.35 + float64(u.Def)*1.6 + float64(u.Spd)*3 + float64(u.EHR)*2 + float64(u.Res)*2
		power += int(math.Round(base * (1 + float64(u.Awakening)*gachaAwakeningPowerWeights[u.Branch])))
	}
	return power, nil
}

// Backfill only derived indexes. A revision condition protects concurrent saves;
// gameplay JSON and resources are never rewritten by this migration.
func InitializeGachaPower() error {
	var cursor int
	for {
		var players []GachaPlayer
		if err := DB.Where("power_version < ? AND user_id > ?", gachaPowerVersion, cursor).Order("user_id ASC").Limit(100).Find(&players).Error; err != nil {
			return err
		}
		if len(players) == 0 {
			return initializeGachaFleetIndexes()
		}
		for _, player := range players {
			var state GachaGameState
			power := 0
			if common.Unmarshal([]byte(player.State), &state) == nil && migrateGachaEffort(&state) == nil {
				power, _ = GachaSquadPower(&state)
			}
			if err := DB.Model(&GachaPlayer{}).Where("user_id = ? AND revision = ? AND power_version < ?", player.UserID, player.Revision, gachaPowerVersion).
				UpdateColumns(map[string]any{"power": power, "power_version": gachaPowerVersion, "arena_points": state.Arena.Points}).Error; err != nil {
				return err
			}
			cursor = player.UserID
		}
	}
}

func gachaCompetitors(tx *gorm.DB) *gorm.DB {
	return tx.Model(&GachaPlayer{}).Joins("JOIN users ON users.id = gacha_players.user_id").
		Where("users.status = ? AND users.deleted_at IS NULL AND gacha_players.power > ?", common.UserStatusEnabled, 0)
}

func GetGachaSocial(userID int) (*GachaSocialView, error) {
	var user User
	if err := DB.Select("id", "status").First(&user, userID).Error; err != nil {
		return nil, err
	}
	if userID <= 0 || user.Status != common.UserStatusEnabled {
		return nil, ErrGachaIntent
	}
	var player GachaPlayer
	if err := DB.First(&player, "user_id = ?", userID).Error; err != nil {
		return nil, err
	}
	view := &GachaSocialView{PowerBoard: []GachaCompetitor{}, ArenaBoard: []GachaCompetitor{}, Opponents: []GachaCompetitor{}, ArenaLimit: GachaArenaDailyLimit}
	view.ArenaEnabled = GachaArenaEnabled()
	for i, target := range []*[]GachaCompetitor{&view.PowerBoard} {
		query := gachaCompetitors(DB).Select("gacha_players.user_id, gacha_players.power, gacha_players.arena_points, gacha_players.state, COALESCE(users.display_name, '') AS name")
		switch i {
		case 0:
			query = query.Order("gacha_players.power DESC, gacha_players.user_id ASC").Limit(50)
		}
		if err := query.Scan(target).Error; err != nil {
			return nil, err
		}
		for j := range *target {
			entry := &(*target)[j]
			entry.Name = fleetPublicName(entry.Name, entry.UserID)
			var state GachaGameState
			if err := common.Unmarshal([]byte(entry.State), &state); err != nil {
				return nil, err
			}
			entry.Squad = []GachaPublicGirl{}
			for _, id := range state.Squad {
				if own := state.Girls[id]; own != nil {
					entry.Squad = append(entry.Squad, GachaPublicGirl{ID: id, Level: own.Level, Star: own.Star})
				}
			}
			entry.State = ""
		}
	}
	if player.Power > 0 {
		if err := gachaCompetitors(DB).Where("gacha_players.power > ? OR (gacha_players.power = ? AND gacha_players.user_id < ?)", player.Power, player.Power, userID).Count(&view.PowerRank).Error; err != nil {
			return nil, err
		}
		view.PowerRank++
		if player.ArenaPoints > 0 {
			if err := gachaCompetitors(DB).Where("gacha_players.arena_points > ? OR (gacha_players.arena_points = ? AND (gacha_players.power > ? OR (gacha_players.power = ? AND gacha_players.user_id < ?)))", player.ArenaPoints, player.ArenaPoints, player.Power, player.Power, userID).Count(&view.ArenaRank).Error; err != nil {
				return nil, err
			}
			view.ArenaRank++
		}
	}
	return view, nil
}

func SimulateGachaArena(attack, defense *GachaGameState) (*GachaBattleReport, error) {
	var seed [16]byte
	if _, err := cryptorand.Read(seed[:]); err != nil {
		return nil, err
	}
	return simulateGachaArena(attack, defense, rand.New(rand.NewPCG(binary.LittleEndian.Uint64(seed[:8]), binary.LittleEndian.Uint64(seed[8:]))))
}

func simulateGachaArena(attack, defense *GachaGameState, rng *rand.Rand) (*GachaBattleReport, error) {
	allies, err := gachaSquadUnits(attack, "ally", "a")
	if err != nil {
		return nil, ErrGachaTeam
	}
	enemies, err := gachaSquadUnits(defense, "enemy", "e")
	if err != nil {
		return nil, ErrGachaOpponent
	}
	if rng == nil {
		return nil, ErrGachaIntent
	}
	report := &GachaBattleReport{Allies: []GachaBattleUnit{}, Waves: [][]GachaBattleUnit{{}}, Events: []GachaBattleEvent{}}
	for _, u := range allies {
		report.Allies = append(report.Allies, u.GachaBattleUnit)
	}
	for _, u := range enemies {
		report.Waves[0] = append(report.Waves[0], u.GachaBattleUnit)
	}
	b := &gachaCombat{allies: allies, waves: [][]*gachaCombatUnit{enemies}, rng: rng, formula: gachaCatalog.Formula, report: report, pvp: true}
	return b.fight(60), nil
}

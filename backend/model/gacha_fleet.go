package model

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const fleetBoardLimit = 50
const fleetDirectoryLimit = 12
const fleetEdition = 3
const fleetNeedsIndex = "(fleet_indexed = ? OR fleet_indexed IS NULL OR fleet_edition < ? OR fleet_edition IS NULL)"

var fleetStoryPoints = func() map[string]bool {
	points := map[string]bool{}
	for _, chapter := range gachaVoyage.Chapters {
		for _, difficulty := range []string{"story", "hard", "nightmare"} {
			for _, id := range chapter.Routes[difficulty] {
				points[id] = true
			}
		}
	}
	return points
}()

type GachaFleetClaims struct {
	Week string `json:"week"`
	Mask int    `json:"mask"`
}

type fleetCargoSpec struct {
	ID, Name, Resource, CostText string
	Cost                         int
}

var fleetCargo = [...]fleetCargoSpec{
	{"charts", "勘绘航标", "stamina", "体力 5", 5},
	{"timber", "修整浮桥", "gold", "金币 45", 45},
	{"lamps", "点亮引航灯", "mat_r", "微光结晶 2", 2},
}

type fleetBeaconSpec struct {
	Name            string
	Goal, Gold, Ore int
}

var fleetBeacons = [...]fleetBeaconSpec{
	{"泊船浅湾", 1, 100, 2}, {"双灯渡口", 3, 180, 4}, {"极星母港", 6, 300, 8},
}

type GachaFleetCargoLine struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Cost     string `json:"cost"`
	Done     int    `json:"done"`
	Eligible bool   `json:"eligible"`
}

type GachaFleetBeacon struct {
	Name     string `json:"name"`
	Tier     int    `json:"tier"`
	Goal     int    `json:"goal"`
	Gold     int    `json:"gold"`
	Ore      int    `json:"ore"`
	Eligible bool   `json:"eligible"`
	Claimed  bool   `json:"claimed"`
}

type GachaFleetHarbor struct {
	Lines     []GachaFleetCargoLine `json:"lines"`
	Beacons   []GachaFleetBeacon    `json:"beacons"`
	LineLimit int                   `json:"line_limit"`
	Score     int                   `json:"score"`
	MyCargo   int                   `json:"my_cargo"`
	Supplied  bool                  `json:"supplied"`
}

func fleetCargoIndex(id string) int {
	for index, rule := range fleetCargo {
		if rule.ID == id {
			return index
		}
	}
	return -1
}

func fleetResource(state *GachaGameState, resource string) int {
	switch resource {
	case "gold":
		return state.Gold
	case "stamina":
		return state.Stamina
	default:
		return state.Mats[resource]
	}
}

func fleetScore(guild *GachaGuildState) int {
	return guild.TotalDamage/100 + (guild.Cargo[0]+guild.Cargo[1]+guild.Cargo[2])*2
}

func fleetHarborView(state *GachaGameState, guild *GachaGuildState, week string) GachaFleetHarbor {
	view := GachaFleetHarbor{LineLimit: guildCargoLimit(guild), Score: fleetScore(guild), Supplied: state.Daily.GuildSupplied}
	if state.Guild.ContributionWeek == week {
		view.MyCargo = state.Guild.MyCargo
	}
	for index, rule := range fleetCargo {
		view.Lines = append(view.Lines, GachaFleetCargoLine{rule.ID, rule.Name, rule.CostText, guild.Cargo[index],
			!view.Supplied && guild.Cargo[index] < view.LineLimit && fleetResource(state, rule.Resource) >= rule.Cost})
	}
	for index, rule := range fleetBeacons {
		claimed := state.FleetClaims != nil && state.FleetClaims.Week == week && state.FleetClaims.Mask&(1<<index) != 0
		ready := min(guild.Cargo[0], guild.Cargo[1], guild.Cargo[2]) >= rule.Goal
		view.Beacons = append(view.Beacons, GachaFleetBeacon{rule.Name, index + 1, rule.Goal, rule.Gold, rule.Ore, ready && !claimed && view.MyCargo > 0, claimed})
	}
	return view
}

func applyFleetCargo(state *GachaGameState, owner *GachaGuildState, intent GachaActionIntent, result *GachaActionResult, now time.Time) error {
	week := gachaWeekKey(now)
	if intent.Action == "guild_supply" {
		index := fleetCargoIndex(*intent.ItemID)
		rule := fleetCargo[index]
		if state.Daily.GuildSupplied || owner.Cargo[index] >= guildCargoLimit(owner) {
			return ErrGachaGuildLimit
		}
		if fleetResource(state, rule.Resource) < rule.Cost {
			return ErrGachaFunds
		}
		if err := guildAddProgress(state, owner, 40, 35, now); err != nil {
			return err
		}
		switch rule.Resource {
		case "stamina":
			if err := spendGachaStamina(state, rule.Cost, now, &result.Drops); err != nil {
				return err
			}
		case "gold":
			state.Gold -= rule.Cost
		default:
			state.Mats[rule.Resource] -= rule.Cost
		}
		state.Daily.GuildSupplied = true
		state.Guild.MyCargo++
		owner.Cargo[index]++
		if err := recordGuildSharedActivity(state, owner, "supply", 1, now); err != nil {
			return err
		}
		result.Drops = append(result.Drops, rule.Name+"已完成，公会航绩 +2 · 公会经验 +40 · 公会币 +35")
		return nil
	}
	index := *intent.Count - 1
	beacon := fleetHarborView(state, owner, week).Beacons[index]
	if !beacon.Eligible {
		return ErrGachaGuildLimit
	}
	if state.Gold > gachaMaxResource-beacon.Gold || state.Mats["forge_ore"] > gachaMaxResource-beacon.Ore {
		return ErrGachaCapacity
	}
	if state.FleetClaims == nil || state.FleetClaims.Week != week {
		state.FleetClaims = &GachaFleetClaims{Week: week}
	}
	state.FleetClaims.Mask |= 1 << index
	state.Gold += beacon.Gold
	state.Mats["forge_ore"] += beacon.Ore
	result.Drops = []string{fmt.Sprintf("%s：金币 +%d · 星锻矿 +%d", beacon.Name, beacon.Gold, beacon.Ore)}
	return nil
}

func fleetPublicName(name string, id int) string {
	if display := strings.TrimSpace(name); display != "" {
		return display
	}
	return fmt.Sprintf("航海员 %04X", id)
}

type GachaFleetPortrait struct {
	Level int               `json:"level,omitempty"`
	Team  []GachaPublicGirl `json:"team,omitempty"`
}

func gachaFleetIndex(userID int, state *GachaGameState) (map[string]any, error) {
	owned := 0
	for _, girl := range gachaCatalog.Girls {
		if gachaCardOwned(state.Girls[girl.ID]) {
			owned++
		}
	}
	stars := 0
	for id := range fleetStoryPoints {
		if state.Cleared[id] {
			stars += min(gachaMaxStars, max(0, state.Stars[id]))
		}
	}
	portrait := GachaFleetPortrait{}
	if state.Captain != nil {
		portrait.Level = min(gachaCatalog.Growth.MaxLevel, max(1, state.Captain.Level))
	}
	for index, id := range state.Squad {
		if len(portrait.Team) == GachaSquadLimit {
			break
		}
		if card := state.Girls[id]; gachaGirl(id) != nil && gachaCardOwned(card) && !slices.Contains(state.Squad[:index], id) {
			portrait.Team = append(portrait.Team, GachaPublicGirl{ID: id, Level: card.Level, Star: card.Star})
		}
	}
	encoded, err := common.Marshal(portrait)
	if err != nil {
		return nil, err
	}
	columns := map[string]any{"fleet_indexed": true, "fleet_edition": fleetEdition, "fleet_stars": stars, "fleet_wins": max(0, state.Arena.Wins), "fleet_portrait": string(encoded), "collection_count": owned, "fleet_leader": 0,
		"fleet_members": 0, "fleet_week": "", "fleet_score": 0, "fleet_name": ""}
	if guild := state.Guild; guild != nil {
		columns["fleet_leader"] = guild.LeaderID
		if guild.Listed && validGachaGuildOwner(guild, userID, fmt.Sprintf("%08X", userID)) {
			columns["fleet_members"], columns["fleet_name"] = len(guild.Members), guild.Name
			columns["fleet_week"], columns["fleet_score"] = guild.Week, fleetScore(guild)
		}
	}
	return columns, nil
}

func initializeGachaFleetIndexes() error {
	cursor := 0
	for {
		var batch []GachaPlayer
		if err := DB.Select("user_id", "revision", "state").Where(fleetNeedsIndex+" AND user_id > ?", false, fleetEdition, cursor).Order("user_id ASC").Limit(100).Find(&batch).Error; err != nil {
			return err
		}
		if len(batch) == 0 {
			return nil
		}
		for _, row := range batch {
			cursor = row.UserID
			var state GachaGameState
			if err := common.Unmarshal([]byte(row.State), &state); err != nil || state.Girls == nil {
				continue
			}
			columns, err := gachaFleetIndex(row.UserID, &state)
			if err != nil {
				return err
			}
			if err := DB.Model(&GachaPlayer{}).Where("user_id = ? AND revision = ? AND "+fleetNeedsIndex, row.UserID, row.Revision, false, fleetEdition).
				UpdateColumns(columns).Error; err != nil {
				return err
			}
		}
	}
}

type GachaFleetEntry struct {
	GachaFleetPortrait
	GuildLevel  int    `json:"guild_level,omitempty"`
	MemberLimit int    `json:"member_limit,omitempty"`
	Name        string `json:"name"`
	Value       int    `json:"value"`
	Mine        bool   `json:"mine,omitempty"`
	Members     int    `json:"members,omitempty"`
	Code        string `json:"code,omitempty"`
}

type GachaFleetBoard struct {
	Board       string            `json:"board"`
	Week        string            `json:"week"`
	Entries     []GachaFleetEntry `json:"entries"`
	MyRank      int64             `json:"my_rank"`
	MyValue     int               `json:"my_value"`
	MyName      string            `json:"my_name,omitempty"`
	MyLevel     int               `json:"my_level,omitempty"`
	StarLimit   int               `json:"star_limit"`
	CatalogSize int               `json:"catalog_size"`
	MemberLimit int               `json:"member_limit"`
	Next        int               `json:"next,omitempty"`
}

func fleetEligible() *gorm.DB {
	return DB.Model(&GachaPlayer{}).Joins("JOIN users ON users.id = gacha_players.user_id").
		Where("users.status = ? AND users.deleted_at IS NULL AND gacha_players.fleet_indexed = ? AND gacha_players.fleet_edition = ?", common.UserStatusEnabled, true, fleetEdition)
}

func GetGachaFleet(userID int, board string, after int, now time.Time) (*GachaFleetBoard, error) {
	if userID < 1 || after < 0 || board != "directory" && after != 0 {
		return nil, ErrGachaIntent
	}
	week := gachaWeekKey(now)
	column, name := "", "COALESCE(users.display_name, '')"
	criteria := func() *gorm.DB { return fleetEligible() }
	switch board {
	case "directory":
		column, name = "gacha_players.fleet_score", "gacha_players.fleet_name"
		criteria = func() *gorm.DB {
			return fleetEligible().Where("gacha_players.fleet_members BETWEEN 1 AND ?", 54)
		}
	case "guild":
		column, name = "gacha_players.fleet_score", "gacha_players.fleet_name"
		criteria = func() *gorm.DB {
			return fleetEligible().Where("gacha_players.fleet_week = ? AND gacha_players.fleet_members > 0", week)
		}
	case "collection":
		column = "gacha_players.collection_count"
	case "power":
		column = "gacha_players.power"
	case "stars":
		column = "gacha_players.fleet_stars"
	case "arena":
		column = "gacha_players.fleet_wins"
	default:
		return nil, ErrGachaIntent
	}
	var viewer struct {
		UserID, FleetLeader int
		Name, Portrait      string
	}
	requester := fleetEligible().Select("gacha_players.user_id, gacha_players.fleet_leader, gacha_players.fleet_portrait AS portrait, COALESCE(users.display_name, '') AS name").Where("gacha_players.user_id = ?", userID).Limit(1).Find(&viewer)
	if requester.Error != nil {
		return nil, requester.Error
	}
	if requester.RowsAffected != 1 {
		return nil, ErrGachaIntent
	}
	view := &GachaFleetBoard{Board: board, Week: week, Entries: []GachaFleetEntry{}, CatalogSize: len(gachaCatalog.Girls), MemberLimit: 55, StarLimit: len(fleetStoryPoints) * gachaMaxStars}
	ownerID := userID
	if board == "directory" || board == "guild" {
		ownerID = viewer.FleetLeader
	} else {
		view.MyName = fleetPublicName(viewer.Name, userID)
		var portrait GachaFleetPortrait
		if common.Unmarshal([]byte(viewer.Portrait), &portrait) == nil {
			view.MyLevel = portrait.Level
		}
	}
	type fleetBoardRow struct {
		UserID, Value, Members      int
		Name, Week, Portrait, State string
	}
	var rows []fleetBoardRow
	projection := "gacha_players.user_id, " + name + " AS name, " + column + " AS value"
	if board == "directory" || board == "guild" {
		projection += ", gacha_players.fleet_members AS members, gacha_players.fleet_week AS week, gacha_players.state AS state"
	}
	if board != "directory" {
		projection += ", gacha_players.fleet_portrait AS portrait"
	}
	if board == "directory" {
		// Capacity depends on the owner's persistent XP. Fill the public page
		// after checking that authority, so full low-level guilds cannot create
		// empty pages in front of guilds that are actually recruiting.
		cursor := after
		for len(rows) <= fleetDirectoryLimit {
			var batch []fleetBoardRow
			if err := criteria().Select(projection).Where("gacha_players.user_id > ?", cursor).
				Order("gacha_players.user_id ASC").Limit(fleetDirectoryLimit + 1).Scan(&batch).Error; err != nil {
				return nil, err
			}
			for _, row := range batch {
				cursor = row.UserID
				var owner GachaGameState
				if common.Unmarshal([]byte(row.State), &owner) != nil || !validGachaGuildOwner(owner.Guild, row.UserID, fmt.Sprintf("%08X", row.UserID)) {
					continue
				}
				if !owner.Guild.Listed || len(owner.Guild.Members) >= guildCapacity(owner.Guild) {
					continue
				}
				rows = append(rows, row)
				if len(rows) > fleetDirectoryLimit {
					break
				}
			}
			if len(batch) < fleetDirectoryLimit+1 {
				break
			}
		}
	} else {
		if err := criteria().Select(projection).Where(column + " > 0").
			Order(column + " DESC, gacha_players.user_id ASC").Limit(fleetBoardLimit).Scan(&rows).Error; err != nil {
			return nil, err
		}
	}
	if board == "directory" && len(rows) > fleetDirectoryLimit {
		view.Next = rows[fleetDirectoryLimit-1].UserID
		rows = rows[:fleetDirectoryLimit]
	}
	for index, row := range rows {
		entry := GachaFleetEntry{Name: row.Name, Value: row.Value, Mine: row.UserID == ownerID}
		if board != "directory" && row.Portrait != "" {
			if err := common.Unmarshal([]byte(row.Portrait), &entry.GachaFleetPortrait); err != nil {
				entry.GachaFleetPortrait = GachaFleetPortrait{}
			}
		}
		if board == "directory" || board == "guild" {
			var owner GachaGameState
			if common.Unmarshal([]byte(row.State), &owner) != nil || !validGachaGuildOwner(owner.Guild, row.UserID, fmt.Sprintf("%08X", row.UserID)) {
				continue
			}
			entry.GuildLevel, entry.MemberLimit = guildLevel(owner.Guild), guildCapacity(owner.Guild)
			if board == "directory" && row.Members >= entry.MemberLimit {
				continue
			}
			entry.Code, entry.Members = fmt.Sprintf("%08X", row.UserID), len(owner.Guild.Members)
			entry.Name = owner.Guild.Name
			if row.Week != week {
				entry.Value = 0
			}
		} else {
			entry.Name = fleetPublicName(row.Name, row.UserID)
		}
		if entry.Mine && board != "directory" {
			view.MyRank, view.MyValue, view.MyName, view.MyLevel = int64(index+1), entry.Value, entry.Name, entry.Level
		}
		view.Entries = append(view.Entries, entry)
	}
	if board != "directory" && ownerID > 0 && view.MyRank == 0 {
		fields := column + " AS value"
		if board == "guild" {
			fields += ", gacha_players.fleet_name AS name, gacha_players.fleet_portrait AS portrait"
		}
		var standing struct {
			Value          int
			Name, Portrait string
		}
		if err := criteria().Select(fields).Where("gacha_players.user_id = ?", ownerID).Limit(1).Scan(&standing).Error; err != nil {
			return nil, err
		}
		view.MyValue = standing.Value
		if view.MyValue > 0 {
			if board == "guild" {
				view.MyName = standing.Name
				var portrait GachaFleetPortrait
				if common.Unmarshal([]byte(standing.Portrait), &portrait) == nil {
					view.MyLevel = portrait.Level
				}
			}
			if err := criteria().Where("("+column+" > ? OR ("+column+" = ? AND gacha_players.user_id < ?))", view.MyValue, view.MyValue, ownerID).Count(&view.MyRank).Error; err != nil {
				return nil, err
			}
			view.MyRank++
		}
	}
	return view, nil
}

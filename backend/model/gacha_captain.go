package model

import (
	_ "embed"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

var ErrGachaCaptainClaim = errors.New("captain reward not available")

const gachaCaptainLootVersion = 8
const gachaStaminaXPVersion = 9

type GachaCaptain struct {
	Level int `json:"level"`
	XP    int `json:"xp"`
}

type GachaCaptainDay struct {
	Edition     int             `json:"edition"`
	Progress    map[string]int  `json:"progress"`
	Claimed     map[string]bool `json:"claimed"`
	Supplies    map[string]bool `json:"supplies"`
	Chests      map[string]bool `json:"chests,omitempty"`
	SkillReview bool            `json:"skill_review,omitempty"`
	GearReview  bool            `json:"gear_review,omitempty"`
}

type GachaCaptainTask struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Goal   string `json:"goal"`
	Target int    `json:"target"`
	XP     int    `json:"xp"`
	Gold   int    `json:"gold"`
	Points int    `json:"points"`
}

type GachaCaptainChest struct {
	ID        string         `json:"id"`
	Points    int            `json:"points"`
	Resources map[string]int `json:"resources"`
}

type GachaCaptainSupply struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Start   int    `json:"start"`
	End     int    `json:"end"`
	Stamina int    `json:"stamina"`
}

type GachaCaptainRules struct {
	XPBase   int                  `json:"xp_base"`
	XPStep   int                  `json:"xp_step"`
	Tasks    []GachaCaptainTask   `json:"tasks"`
	Supplies []GachaCaptainSupply `json:"supplies"`
	Chests   []GachaCaptainChest  `json:"chests"`
}

//go:embed gacha_captain.json
var gachaCaptainRulesJSON []byte

var gachaCaptainRules = func() GachaCaptainRules {
	var rules GachaCaptainRules
	if err := common.Unmarshal(gachaCaptainRulesJSON, &rules); err != nil {
		panic(err)
	}
	if rules.XPBase <= 0 || rules.XPStep <= 0 || len(rules.Tasks) != 9 || len(rules.Chests) == 0 || len(rules.Supplies) != 3 {
		panic("invalid captain rules")
	}
	points, last := 0, 0
	seen := map[string]bool{}
	for _, task := range rules.Tasks {
		if task.ID == "" || seen[task.ID] || task.Target <= 0 || task.Points <= 0 || task.XP < 1 || task.Gold < 1 {
			panic("invalid captain task")
		}
		points += task.Points
		seen[task.ID] = true
	}
	for _, chest := range rules.Chests {
		if chest.Points <= last || chest.ID != strconv.Itoa(chest.Points) || len(chest.Resources) == 0 {
			panic("invalid captain chest")
		}
		for _, amount := range chest.Resources {
			if amount < 1 || amount > gachaMaxResource {
				panic("invalid captain chest resource")
			}
		}
		last = chest.Points
	}
	if last != points {
		panic("invalid captain point total")
	}
	return rules
}()

func gachaCaptainLevel(state *GachaGameState) int {
	if state.Captain == nil {
		return 1
	}
	return state.Captain.Level
}

func gachaCaptainCost(level int) int {
	return gachaCaptainRules.XPBase + level*gachaCaptainRules.XPStep
}

func gachaCaptainTask(id string) *GachaCaptainTask {
	for index := range gachaCaptainRules.Tasks {
		if gachaCaptainRules.Tasks[index].ID == id {
			return &gachaCaptainRules.Tasks[index]
		}
	}
	return nil
}

func gachaCaptainSupply(id string) *GachaCaptainSupply {
	for index := range gachaCaptainRules.Supplies {
		if gachaCaptainRules.Supplies[index].ID == id {
			return &gachaCaptainRules.Supplies[index]
		}
	}
	return nil
}

func gachaCaptainChest(id string) *GachaCaptainChest {
	for index := range gachaCaptainRules.Chests {
		if gachaCaptainRules.Chests[index].ID == id {
			return &gachaCaptainRules.Chests[index]
		}
	}
	return nil
}

func gachaCaptainPoints(log *GachaCaptainDay) int {
	points := 0
	for _, task := range gachaCaptainRules.Tasks {
		if log.Progress[task.ID] >= task.Target {
			points += task.Points
		}
	}
	return points
}

func syncGachaCaptain(state *GachaGameState) error {
	peak := 1
	for _, own := range state.Girls {
		if own != nil && own.Star > 0 {
			peak = max(peak, gachaPeakLevel(own))
		}
	}
	if state.Captain == nil {
		state.Captain = &GachaCaptain{Level: peak}
	}
	captain := state.Captain
	limit := gachaCatalog.Growth.MaxLevel
	if captain.Level < peak || captain.Level > limit || captain.XP < 0 || captain.XP >= gachaCaptainCost(captain.Level) || captain.Level == limit && captain.XP != 0 {
		return ErrGachaRunState
	}
	if state.Daily.CaptainLog == nil {
		state.Daily.CaptainLog = &GachaCaptainDay{Edition: 1, Progress: map[string]int{"rollcall": 1}, Claimed: map[string]bool{}, Supplies: map[string]bool{}}
	}
	log := state.Daily.CaptainLog
	if log.Edition < 0 || log.Edition > 1 || log.Progress == nil || log.Claimed == nil || log.Supplies == nil || log.Progress["rollcall"] != 1 {
		return ErrGachaRunState
	}
	for id, value := range log.Progress {
		if task := gachaCaptainTask(id); task == nil || value < 0 || value > task.Target {
			return ErrGachaRunState
		}
	}
	for id, claimed := range log.Claimed {
		if task := gachaCaptainTask(id); task == nil || !claimed || log.Progress[id] != task.Target {
			return ErrGachaRunState
		}
	}
	for id, claimed := range log.Supplies {
		if !claimed || gachaCaptainSupply(id) == nil {
			return ErrGachaRunState
		}
	}
	for id, claimed := range log.Chests {
		if chest := gachaCaptainChest(id); !claimed || chest == nil || gachaCaptainPoints(log) < chest.Points {
			return ErrGachaRunState
		}
	}
	return nil
}

func migrateGachaCaptainDay(tx *gorm.DB, userID int, state *GachaGameState, now time.Time) error {
	log := state.Daily.CaptainLog
	if log.Edition == 1 {
		return nil
	}
	day, err := gachaCalendarDay(state.Daily.Date)
	if err != nil {
		return err
	}
	var played int64
	if err := tx.Model(&GachaLadderBattle{}).Where("attacker_id = ? AND played_at >= ? AND played_at <= ?", userID, day.UnixMilli(), now.UnixMilli()).Count(&played).Error; err != nil {
		return err
	}
	if played > 0 {
		log.Progress["ladder"] = int(min(played, int64(gachaCaptainTask("ladder").Target)))
	}
	log.Edition = 1
	return nil
}

func grantGachaCaptainXP(state *GachaGameState, amount int, drops *[]string) {
	captain := state.Captain
	if amount <= 0 || captain.Level >= gachaCatalog.Growth.MaxLevel {
		return
	}
	levelBefore := captain.Level
	captain.XP += amount
	for captain.Level < gachaCatalog.Growth.MaxLevel && captain.XP >= gachaCaptainCost(captain.Level) {
		captain.XP -= gachaCaptainCost(captain.Level)
		captain.Level++
	}
	if captain.Level == gachaCatalog.Growth.MaxLevel {
		amount -= captain.XP
		captain.XP = 0
	}
	*drops = append(*drops, fmt.Sprintf("船长经验 +%d", amount))
	if levelBefore != captain.Level {
		*drops = append(*drops, fmt.Sprintf("船长升至 Lv.%d · 卡牌上限同步提升", captain.Level))
	}
}

func spendGachaStamina(state *GachaGameState, amount int, now time.Time, drops *[]string) error {
	if amount < 0 || amount > gachaMaxResource || state.Captain == nil {
		return ErrGachaRunState
	}
	if amount == 0 {
		return nil
	}
	if state.Stamina < amount {
		return ErrGachaFunds
	}
	if state.Stamina >= state.StaminaMax {
		state.StaminaTs = now.UnixMilli()
	}
	state.Stamina -= amount
	grantGachaCaptainXP(state, amount, drops)
	return nil
}

func advanceGachaCaptainTask(state *GachaGameState, id string) {
	advanceGachaCaptainTaskBy(state, id, 1)
}

func advanceGachaCaptainTaskBy(state *GachaGameState, id string, amount int) {
	if log := state.Daily.CaptainLog; log != nil && amount > 0 {
		log.Progress[id] = min(gachaCaptainTask(id).Target, log.Progress[id]+amount)
	}
}

func gachaCaptainMastery(state *GachaGameState) (bool, bool) {
	owned, gear, skills := false, true, true
	for _, girl := range state.Girls {
		if !gachaCardOwned(girl) {
			continue
		}
		owned = true
		if gear {
			for _, slot := range gachaEquipment.Slots {
				if gachaEquipmentPiece(girl, slot.ID).Level < len(gachaEquipment.LevelCosts) {
					gear = false
					break
				}
			}
		}
		if skills && girl.Level > 0 {
			for _, line := range []string{"normal", "rage", "passive"} {
				if girl.Training == nil || girl.Training.Skills[line] < girl.Level {
					skills = false
					break
				}
			}
		}
		if !gear && !skills {
			break
		}
	}
	return owned && gear, owned && skills
}

func reconcileGachaCaptainDay(state *GachaGameState) {
	log := state.Daily.CaptainLog
	if log == nil {
		return
	}
	log.GearReview, log.SkillReview = gachaCaptainMastery(state)
	if state.Progression != nil {
		for _, day := range state.Progression.Days {
			if day.Date == state.Daily.Date {
				if day.TrainingLeft < day.Rules.TrainingPerDay {
					log.Progress["drill"] = 1
				}
				if len(day.WishTargets) > 0 {
					log.Progress["wish"] = 1
				}
			}
		}
	}
	if festival := state.Festival; festival != nil && festival.Day >= 0 && 2*festival.Day+1 < len(festivalEvents) {
		first, second := festivalEvents[2*festival.Day].ID, festivalEvents[2*festival.Day+1].ID
		if festival.Stamps[first] || festival.Stamps[second] || festival.Tries[first] == 3 && festival.Tries[second] == 3 {
			log.Progress["festival"] = 1
		}
	}
}

func settleGachaCaptainRun(state *GachaGameState, stage *GachaStage, snapshot *GachaRunSnapshot, result *GachaFinishResult) {
	if snapshot.RulesVersion == gachaCaptainLootVersion {
		grantGachaCaptainXP(state, stage.Cost, &result.Drops)
	} else if snapshot.RulesVersion >= gachaStaminaXPVersion {
		result.Drops = append(result.Drops, snapshot.StaminaDrops...)
	}
	if snapshot.RulesVersion < gachaCaptainLootVersion || !result.Win || stage.Cost <= 0 && snapshot.Mode != "training" {
		return
	}
	advanceGachaCaptainTask(state, "voyage")
	if stage.Cost > 0 {
		gear, skills := gachaCaptainMastery(state)
		if gear {
			advanceGachaCaptainTask(state, "equipment")
		}
		if skills {
			advanceGachaCaptainTask(state, "skill")
		}
		if gear || skills {
			advanceGachaCaptainTask(state, "outfit")
		}
	}
	if snapshot.Mode == "training" {
		advanceGachaCaptainTask(state, "drill")
	}
}

func recordGachaCaptainAction(state *GachaGameState, action string) {
	switch action {
	case "wish":
		advanceGachaCaptainTask(state, "wish")
	case "card_feed", "use_manual", "card_breakthrough", "effort_train", "effort_train_max", "gear_upgrade", "gear_auto", "gear_star", "weapon_upgrade", "carry_enhance", "skill_point", "skill_auto":
		advanceGachaCaptainTask(state, "outfit")
	}
}

func validateGachaCaptainIntent(intent GachaActionIntent) error {
	if intent.ItemID == nil || len(*intent.ItemID) > 24 || intent.GirlID != nil || intent.Count != nil || intent.Squad != nil || intent.Enabled != nil || intent.OpponentID != nil || intent.StakeE != nil || intent.QueueID != nil || intent.RoomID != nil || intent.Config != nil || intent.Choice != nil || intent.Materials != nil {
		return ErrGachaIntent
	}
	day, id, ok := strings.Cut(*intent.ItemID, ":")
	if _, err := gachaCalendarDay(day); !ok || err != nil {
		return ErrGachaIntent
	}
	if intent.Action == "captain_task" && gachaCaptainTask(id) != nil || intent.Action == "captain_supply" && gachaCaptainSupply(id) != nil || intent.Action == "captain_chest" && gachaCaptainChest(id) != nil {
		return nil
	}
	return ErrGachaIntent
}

func applyGachaCaptainClaim(state *GachaGameState, intent GachaActionIntent, result *GachaActionResult, now time.Time) error {
	day, id, _ := strings.Cut(*intent.ItemID, ":")
	log := state.Daily.CaptainLog
	if day != state.Daily.Date || day != gachaDay(now) || log == nil {
		return ErrGachaCaptainClaim
	}
	if intent.Action == "captain_chest" {
		chest := gachaCaptainChest(id)
		if chest == nil || log.Chests[id] || gachaCaptainPoints(log) < chest.Points {
			return ErrGachaCaptainClaim
		}
		if err := grantGachaDirectResources(state, chest.Resources, result, now); err != nil {
			return err
		}
		if log.Chests == nil {
			log.Chests = map[string]bool{}
		}
		log.Chests[id] = true
		return nil
	}
	if intent.Action == "captain_supply" {
		supply := gachaCaptainSupply(id)
		hour := now.In(gameZone).Hour()
		if supply == nil || hour < supply.Start || hour >= supply.End || log.Supplies[id] {
			return ErrGachaCaptainClaim
		}
		if state.Stamina > gachaMaxResource-supply.Stamina {
			return ErrGachaCapacity
		}
		state.Stamina += supply.Stamina
		if state.Stamina >= state.StaminaMax {
			state.StaminaTs = now.UnixMilli()
		}
		log.Supplies[id] = true
		result.Drops = append(result.Drops, fmt.Sprintf("%s · 体力 +%d", supply.Name, supply.Stamina))
		return nil
	}
	task := gachaCaptainTask(id)
	if task == nil || log.Claimed[id] || log.Progress[id] < task.Target {
		return ErrGachaCaptainClaim
	}
	if state.Gold > gachaMaxResource-task.Gold {
		return ErrGachaCapacity
	}
	state.Gold += task.Gold
	grantGachaCaptainXP(state, task.XP, &result.Drops)
	log.Claimed[id] = true
	result.Drops = append(result.Drops, fmt.Sprintf("%s · 金币 +%d", task.Name, task.Gold))
	return nil
}

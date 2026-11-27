package model

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrGachaCaptainClaim = errors.New("captain reward not available")

const gachaCaptainLootVersion = 8
const gachaStaminaXPVersion = 9

type GachaCaptain struct {
	Level int `json:"level"`
	XP    int `json:"xp"`
}

type GachaCaptainDay struct {
	Progress map[string]int  `json:"progress"`
	Claimed  map[string]bool `json:"claimed"`
	Supplies map[string]bool `json:"supplies"`
}

type GachaCaptainTask struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Goal   string `json:"goal"`
	Target int    `json:"target"`
	XP     int    `json:"xp"`
	Gold   int    `json:"gold"`
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
}

var gachaCaptainRules = GachaCaptainRules{
	XPBase: 20, XPStep: 10,
	Tasks: []GachaCaptainTask{
		{"rollcall", "港口点名", "今日抵达母港", 1, 40, 50},
		{"voyage", "三程巡航", "赢得消耗体力的副本或经验训练", 3, 100, 160},
		{"drill", "甲板晨练", "完成一次经验训练", 1, 60, 80},
		{"wish", "向星许愿", "为一位伙伴定向许愿", 1, 50, 60},
		{"outfit", "整装备航", "研读经验书、突破、努力训练或强化装备一次", 1, 60, 80},
	},
	Supplies: []GachaCaptainSupply{
		{"morning", "晨航便当", 9, 12, 30}, {"noon", "正午炊烟", 12, 21, 30}, {"night", "夜航热汤", 21, 24, 30},
	},
}

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
		state.Daily.CaptainLog = &GachaCaptainDay{Progress: map[string]int{"rollcall": 1}, Claimed: map[string]bool{}, Supplies: map[string]bool{}}
	}
	log := state.Daily.CaptainLog
	if log.Progress == nil || log.Claimed == nil || log.Supplies == nil || log.Progress["rollcall"] != 1 {
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
	if log := state.Daily.CaptainLog; log != nil {
		log.Progress[id] = min(gachaCaptainTask(id).Target, log.Progress[id]+1)
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
	if snapshot.Mode == "training" {
		advanceGachaCaptainTask(state, "drill")
	}
}

func recordGachaCaptainAction(state *GachaGameState, action string) {
	switch action {
	case "wish":
		advanceGachaCaptainTask(state, "wish")
	case "card_feed", "use_manual", "card_breakthrough", "effort_train", "effort_train_max", "gear_upgrade", "gear_auto", "gear_star", "weapon_upgrade":
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
	if intent.Action == "captain_task" && gachaCaptainTask(id) != nil || intent.Action == "captain_supply" && gachaCaptainSupply(id) != nil {
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

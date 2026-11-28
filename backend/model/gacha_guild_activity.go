package model

import (
	"fmt"
	"maps"
	"slices"
	"time"
)

// Personal daily and weekly entitlements remain with the account across guilds.
type GachaGuildActivity struct {
	Day           string          `json:"day"`
	Week          string          `json:"week"`
	Progress      map[string]int  `json:"progress"`
	Claimed       map[string]bool `json:"claimed"`
	MissionClaims int             `json:"mission_claims"`
	Contributions map[string]int  `json:"contributions"`
}

type GachaGuildMissions struct {
	Week     string          `json:"week"`
	Progress map[string]int  `json:"progress"`
	Awarded  map[string]bool `json:"awarded"`
}

type GachaGuildActivityTask struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Goal      string         `json:"goal"`
	Target    int            `json:"target"`
	Progress  int            `json:"progress"`
	XP        int            `json:"xp"`
	Coins     int            `json:"coins"`
	Resources map[string]int `json:"resources,omitempty"`
	Claimed   bool           `json:"claimed"`
	CanClaim  bool           `json:"can_claim"`
}

type GachaGuildXPSource struct {
	Name   string `json:"name"`
	Detail string `json:"detail"`
	XP     int    `json:"xp"`
	Coins  int    `json:"coins"`
}

type GachaGuildActivityView struct {
	Day          string                   `json:"day"`
	Week         string                   `json:"week"`
	Tasks        []GachaGuildActivityTask `json:"tasks"`
	Missions     []GachaGuildActivityTask `json:"missions"`
	Sources      []GachaGuildXPSource     `json:"sources"`
	Contribution int                      `json:"contribution"`
}

var guildDailyTasks = []GachaGuildActivityTask{
	{ID: "checkin", Name: "母港签到", Goal: "每日回到公会报到", Target: 1, XP: 20, Coins: gachaMarket.Guild.Checkin},
	{ID: "sortie", Name: "协力巡航", Goal: "赢得 3 场消耗体力的副本战斗", Target: 3, XP: 60, Coins: gachaMarket.Guild.Sortie},
	{ID: "training", Name: "整备伙伴", Goal: "完成 3 次消耗材料的伙伴或装备培养", Target: 3, XP: 40, Coins: gachaMarket.Guild.Training},
	{ID: "arena", Name: "演武交流", Goal: "完成 2 场排名竞技挑战，胜负均计", Target: 2, XP: 50, Coins: gachaMarket.Guild.Arena},
}

var guildWeeklyMissions = []GachaGuildActivityTask{
	{ID: "supply", Name: "物资集结", Goal: "全体成员累计交付 20 份共建物资", Target: 20, XP: 800, Coins: 150, Resources: map[string]int{"training_ink": 20, "training_manual": 6}},
	{ID: "raid", Name: "并肩讨伐", Goal: "全体成员累计挑战公会首领 30 次", Target: 30, XP: 1200, Coins: 250, Resources: map[string]int{"forge_ore": 20, "break_seal": 4}},
	{ID: "active", Name: "百舸争流", Goal: "全体成员累计领取 50 项巡航、整备或演武任务", Target: 50, XP: 1800, Coins: 400, Resources: map[string]int{"expedition_atlas": 8, "weapon_core": 3}},
}

func syncGuildActivity(state *GachaGameState, now time.Time) error {
	if state.GuildProgress == nil {
		return nil
	}
	p := state.GuildProgress
	if p.Activity == nil {
		p.Activity = &GachaGuildActivity{}
	}
	a := p.Activity
	day, week := gachaDay(now), gachaWeekKey(now)
	if a.Day != "" {
		if _, err := gachaCalendarDay(a.Day); err != nil || a.Day > day {
			return ErrGachaRunState
		}
	}
	if a.Week != "" && (!validGachaWeek(a.Week) || a.Week > week) {
		return ErrGachaRunState
	}
	if a.MissionClaims < 0 || a.MissionClaims > 7 || len(a.Contributions) > 64 {
		return ErrGachaRunState
	}
	for id, count := range a.Progress {
		index := slices.IndexFunc(guildDailyTasks, func(rule GachaGuildActivityTask) bool { return rule.ID == id && id != "checkin" })
		if index < 0 || count < 0 || count > guildDailyTasks[index].Target {
			return ErrGachaRunState
		}
	}
	for id := range a.Claimed {
		if !slices.ContainsFunc(guildDailyTasks, func(rule GachaGuildActivityTask) bool { return rule.ID == id }) {
			return ErrGachaRunState
		}
	}
	for code, count := range a.Contributions {
		if gachaGuildLeaderFromCode(code) <= 0 || count < 0 || count > 10000 {
			return ErrGachaRunState
		}
	}
	if a.Day != day {
		a.Day = day
		a.Progress = map[string]int{}
		a.Claimed = map[string]bool{}
	}
	if a.Week != week {
		a.Week = week
		a.MissionClaims = 0
		a.Contributions = map[string]int{}
	}
	if a.Progress == nil {
		a.Progress = map[string]int{}
	}
	if a.Claimed == nil {
		a.Claimed = map[string]bool{}
	}
	if a.Contributions == nil {
		a.Contributions = map[string]int{}
	}
	return nil
}

func syncGuildMissions(owner *GachaGuildState, now time.Time) error {
	week := gachaWeekKey(now)
	if owner.Missions == nil {
		owner.Missions = &GachaGuildMissions{}
	}
	m := owner.Missions
	if m.Week != "" && (!validGachaWeek(m.Week) || m.Week > week) {
		return ErrGachaRunState
	}
	for id, count := range m.Progress {
		i := slices.IndexFunc(guildWeeklyMissions, func(rule GachaGuildActivityTask) bool { return rule.ID == id })
		if i < 0 || count < 0 || count > guildWeeklyMissions[i].Target {
			return ErrGachaRunState
		}
	}
	for id := range m.Awarded {
		if !slices.ContainsFunc(guildWeeklyMissions, func(rule GachaGuildActivityTask) bool { return rule.ID == id }) {
			return ErrGachaRunState
		}
	}
	if m.Week != week {
		m.Week = week
		m.Progress = map[string]int{}
		m.Awarded = map[string]bool{}
	}
	if m.Progress == nil {
		m.Progress = map[string]int{}
	}
	if m.Awarded == nil {
		m.Awarded = map[string]bool{}
	}
	return nil
}

// Called only after a successful, authoritative gameplay outcome, never from
// client counters. Shared guild XP is transferred by the later locked claim.
func recordGuildPersonalActivity(state *GachaGameState, kind string, now time.Time) error {
	if state.Guild == nil {
		return nil
	}
	guildProgress(state, now)
	if err := syncGuildActivity(state, now); err != nil {
		return err
	}
	i := slices.IndexFunc(guildDailyTasks, func(rule GachaGuildActivityTask) bool { return rule.ID == kind && kind != "checkin" })
	if i < 0 {
		return ErrGachaRunState
	}
	a := state.GuildProgress.Activity
	a.Progress[kind] = min(guildDailyTasks[i].Target, a.Progress[kind]+1)
	return nil
}

func recordGuildTrainingAction(state *GachaGameState, action string, now time.Time) error {
	if slices.Contains([]string{"card_feed", "use_manual", "card_breakthrough", "effort_train", "effort_train_max", "effort_promote", "gear_upgrade", "gear_auto", "gear_star", "weapon_upgrade", "carry_enhance"}, action) {
		return recordGuildPersonalActivity(state, "training", now)
	}
	return nil
}

// Shared missions have a permanent-in-this-week XP marker; individual reward
// collection never grants shared XP again. Both live in the caller transaction.
func recordGuildSharedActivity(state *GachaGameState, owner *GachaGuildState, kind string, amount int, now time.Time) error {
	guildProgress(state, now)
	if err := syncGuildActivity(state, now); err != nil {
		return err
	}
	if err := syncGuildMissions(owner, now); err != nil {
		return err
	}
	if amount < 1 || amount > 100 {
		return ErrGachaRunState
	}
	a := state.GuildProgress.Activity
	a.Contributions[owner.Code] = min(10000, a.Contributions[owner.Code]+amount)
	for _, rule := range guildWeeklyMissions {
		if rule.ID != kind {
			continue
		}
		owner.Missions.Progress[kind] = min(rule.Target, owner.Missions.Progress[kind]+amount)
		if owner.Missions.Progress[kind] >= rule.Target && !owner.Missions.Awarded[kind] {
			owner.XP = min(guildLevelXP[len(guildLevelXP)-1], owner.XP+rule.XP)
			owner.Missions.Awarded[kind] = true
		}
		return nil
	}
	if kind == "checkin" {
		return nil
	}
	return ErrGachaRunState
}

func guildActivityView(state *GachaGameState, owner *GachaGuildState, now time.Time) *GachaGuildActivityView {
	guildProgress(state, now)
	if syncGuildActivity(state, now) != nil || syncGuildMissions(owner, now) != nil {
		return nil
	}
	a := state.GuildProgress.Activity
	v := &GachaGuildActivityView{Day: a.Day, Week: a.Week, Contribution: a.Contributions[owner.Code], Tasks: []GachaGuildActivityTask{}, Missions: []GachaGuildActivityTask{}}
	for _, rule := range guildDailyTasks {
		rule.Progress = a.Progress[rule.ID]
		if rule.ID == "checkin" {
			rule.Progress = 1
		}
		rule.Claimed = a.Claimed[rule.ID]
		rule.CanClaim = !rule.Claimed && rule.Progress >= rule.Target
		v.Tasks = append(v.Tasks, rule)
	}
	for i, rule := range guildWeeklyMissions {
		rule.Resources = maps.Clone(rule.Resources)
		rule.Progress = owner.Missions.Progress[rule.ID]
		rule.Claimed = a.MissionClaims&(1<<i) != 0
		rule.CanClaim = rule.Progress >= rule.Target && !rule.Claimed && v.Contribution > 0
		v.Missions = append(v.Missions, rule)
	}
	v.Sources = []GachaGuildXPSource{
		{Name: "每日签到", Detail: "每账号每日一次", XP: 20, Coins: gachaMarket.Guild.Checkin},
		{Name: "母港共建", Detail: "每日任选一种物资交付", XP: 40, Coins: gachaMarket.Guild.Contribution},
		{Name: "协力巡航", Detail: "三场付费副本胜利后领取", XP: 60, Coins: gachaMarket.Guild.Sortie},
		{Name: "整备伙伴", Detail: "三次消耗材料的培养后领取", XP: 40, Coins: gachaMarket.Guild.Training},
		{Name: "演武交流", Detail: "两场排名挑战后领取", XP: 50, Coins: gachaMarket.Guild.Arena},
		{Name: "首领讨伐", Detail: "每次挑战即得，每日两次", XP: gachaMarket.Guild.RaidXP, Coins: gachaMarket.Guild.Raid},
		{Name: "集体周目标", Detail: "三项目标额外提供 3800 公会经验", XP: 3800},
	}
	return v
}

func validateGuildActivityIntent(intent GachaActionIntent) error {
	if intent.GirlID != nil || intent.Squad != nil || intent.Count != nil || intent.OpponentID != nil || intent.Enabled != nil || intent.Choice != nil || intent.StakeE != nil || intent.QueueID != nil || intent.RoomID != nil || intent.Config != nil || intent.Materials != nil {
		return ErrGachaIntent
	}
	if intent.Action == "guild_checkin" && intent.ItemID == nil {
		return nil
	}
	if intent.ItemID == nil {
		return ErrGachaIntent
	}
	if intent.Action == "guild_activity_claim" && slices.ContainsFunc(guildDailyTasks, func(rule GachaGuildActivityTask) bool { return rule.ID == *intent.ItemID && rule.ID != "checkin" }) {
		return nil
	}
	if intent.Action == "guild_mission_claim" && slices.ContainsFunc(guildWeeklyMissions, func(rule GachaGuildActivityTask) bool { return rule.ID == *intent.ItemID }) {
		return nil
	}
	return ErrGachaIntent
}

func applyGuildActivity(state *GachaGameState, owner *GachaGuildState, intent GachaActionIntent, result *GachaActionResult, now time.Time) error {
	if err := validateGuildActivityIntent(intent); err != nil {
		return err
	}
	guildProgress(state, now)
	if err := syncGuildActivity(state, now); err != nil {
		return err
	}
	if err := syncGuildMissions(owner, now); err != nil {
		return err
	}
	a := state.GuildProgress.Activity
	if intent.Action == "guild_mission_claim" {
		for i, rule := range guildWeeklyMissions {
			if rule.ID != *intent.ItemID {
				continue
			}
			if a.MissionClaims&(1<<i) != 0 || a.Contributions[owner.Code] == 0 || owner.Missions.Progress[rule.ID] < rule.Target {
				return ErrGachaGuildLimit
			}
			if state.GuildProgress.Coins > gachaMaxResource-rule.Coins {
				return ErrGachaCapacity
			}
			prizes := make([]GachaVoyagePrize, 0, len(rule.Resources))
			for id, count := range rule.Resources {
				prizes = append(prizes, GachaVoyagePrize{Item: id, Amount: count})
			}
			drops, err := grantGachaVoyagePrize(state, prizes)
			if err != nil {
				return err
			}
			state.GuildProgress.Coins += rule.Coins
			a.MissionClaims |= 1 << i
			result.Drops = append(result.Drops, drops...)
			result.Drops = append(result.Drops, fmt.Sprintf("%s · 公会币 +%d", rule.Name, rule.Coins))
			return nil
		}
		return ErrGachaIntent
	}
	id := "checkin"
	if intent.ItemID != nil {
		id = *intent.ItemID
	}
	for _, rule := range guildDailyTasks {
		if rule.ID != id {
			continue
		}
		if a.Claimed[id] || id != "checkin" && a.Progress[id] < rule.Target {
			return ErrGachaGuildLimit
		}
		if err := guildAddProgress(state, owner, rule.XP, rule.Coins, now); err != nil {
			return err
		}
		kind := "active"
		if id == "checkin" {
			kind = id
		}
		if err := recordGuildSharedActivity(state, owner, kind, 1, now); err != nil {
			return err
		}
		a.Claimed[id] = true
		result.Drops = append(result.Drops, fmt.Sprintf("%s · 公会经验 +%d · 公会币 +%d", rule.Name, rule.XP, rule.Coins))
		return nil
	}
	return ErrGachaIntent
}

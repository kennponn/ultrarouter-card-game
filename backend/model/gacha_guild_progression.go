package model

import (
	cryptorand "crypto/rand"
	"encoding/binary"
	"fmt"
	"maps"
	"math/rand/v2"
	"slices"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// Personal balances and limits survive leaving, joining and recreating a guild.
type GachaGuildProgress struct {
	Coins         int                 `json:"coins"`
	Week          string              `json:"week"`
	Claims        int                 `json:"claims"`
	Bought        map[string]int      `json:"bought"`
	Participation map[string]int      `json:"participation"`
	Day           string              `json:"day"`
	Attempts      int                 `json:"attempts"`
	Activity      *GachaGuildActivity `json:"activity,omitempty"`
}

var guildLevelXP = [...]int{0, 1800, 6000, 14000, 28000, 50000, 82000, 126000, 184000, 260000}
var guildLegacyLevelXP = [...]int{0, 100, 250, 450, 700, 1000, 1400, 1850, 2350, 3000}

const guildRaidVersion = 3

type guildRaidRule struct {
	Name        string
	Level       int
	HP          int
	Attack      int
	Defense     int
	Coins       int
	XP          int
	Resources   map[string]int
	Mechanic    string
	Description string
}

var guildRaids = [...]guildRaidRule{
	{Name: "暗礁守卫", Level: 1, HP: 1300000, Attack: 550, Defense: 80, Coins: 300, XP: 200, Resources: map[string]int{"training_manual": 12, "training_ink": 30, "break_seal": 4, "mat_sr": 12}, Mechanic: "fury", Description: "每回合攻击增强；组织轮番进攻，在狂怒压垮阵容前积累伤害。"},
	{Name: "深潮统领", Level: 3, HP: 8500000, Attack: 1800, Defense: 160, Coins: 500, XP: 400, Resources: map[string]int{"training_manual": 20, "training_ink": 50, "break_seal": 8, "forge_ore": 35, "mat_ssr": 8}, Mechanic: "venom", Description: "普通攻击可能附加持续侵蚀；治疗与抗性能够延长输出时间。"},
	{Name: "深渊主宰", Level: 5, HP: 42000000, Attack: 5000, Defense: 280, Coins: 800, XP: 700, Resources: map[string]int{"expedition_atlas": 15, "training_ink": 80, "break_seal": 15, "weapon_core": 6, "mat_ssr": 15}, Mechanic: "reflection", Description: "受到伤害会反射给攻击者；搭配护盾和治疗维持全队。"},
	{Name: "永夜司钟", Level: 7, HP: 135000000, Attack: 10000, Defense: 450, Coins: 1200, XP: 1100, Resources: map[string]int{"expedition_atlas": 25, "training_ink": 120, "break_seal": 25, "weapon_core": 12, "gem_ticket": 8, "mat_ssr": 25}, Mechanic: "silence", Description: "偶数回合的普通攻击削减目标怒气；依靠协同蓄怒争取技能释放。"},
	{Name: "终焉潮皇", Level: 9, HP: 360000000, Attack: 18000, Defense: 700, Coins: 1800, XP: 1600, Resources: map[string]int{"expedition_atlas": 40, "training_ink": 180, "break_seal": 40, "weapon_core": 20, "chip_ticket": 12, "mat_ssr": 40}, Mechanic: "fury", Description: "随回合不断狂化的最终首领；集结高阶队伍共同攻破三亿六千万生命。"},
}

func guildRaidTotalHP() int {
	total := 0
	for _, rule := range guildRaids {
		total += rule.HP
	}
	return total
}

type GachaGuildRaidView struct {
	Stage         int            `json:"stage"`
	Name          string         `json:"name"`
	RequiredLevel int            `json:"required_level"`
	HP            int            `json:"hp"`
	MaxHP         int            `json:"max_hp"`
	Cleared       bool           `json:"cleared"`
	Unlocked      bool           `json:"unlocked"`
	Active        bool           `json:"active"`
	Participated  bool           `json:"participated"`
	Claimed       bool           `json:"claimed"`
	CanClaim      bool           `json:"can_claim"`
	Token         int            `json:"token"`
	Material      string         `json:"material"`
	MaterialCount int            `json:"material_count"`
	Coins         int            `json:"coins"`
	Gold          int            `json:"gold"`
	Resources     map[string]int `json:"resources"`
	Mechanic      string         `json:"mechanic"`
	Description   string         `json:"description"`
}

type GachaGuildRaidContribution struct {
	Name     string `json:"name"`
	Damage   int    `json:"damage"`
	Attempts int    `json:"attempts"`
}

type GachaGuildRaidRankingEntry struct {
	Rank     int    `json:"rank"`
	UserID   int    `json:"user_id"`
	Name     string `json:"name"`
	Damage   int    `json:"damage"`
	Attempts int    `json:"attempts"`
	Member   bool   `json:"member"`
}

type GachaGuildRaidRanking struct {
	Week       string                       `json:"week"`
	Stage      int                          `json:"stage"`
	StageName  string                       `json:"stage_name"`
	AllCleared bool                         `json:"all_cleared"`
	Entries    []GachaGuildRaidRankingEntry `json:"entries"`
}

func validGuildRaidContributions(owner *GachaGuildState) bool {
	for stage, rows := range owner.RaidDamage {
		if stage < 1 || stage > len(guildRaids) {
			return false
		}
		total := 0
		for userID, row := range rows {
			if userID <= 0 || row.Name == "" || row.Damage < 0 || row.Damage > guildRaids[stage-1].HP || row.Attempts < 1 || row.Attempts > 14 {
				return false
			}
			total += row.Damage
			if total > guildRaids[stage-1].HP {
				return false
			}
		}
	}
	return true
}

func recordGuildRaidContribution(owner *GachaGuildState, stage, userID, damage int) error {
	index := slices.IndexFunc(owner.Members, func(member GachaGuildMember) bool { return member.UserID == userID })
	if index < 0 || stage < 1 || stage > len(guildRaids) || damage < 0 || damage > guildRaids[stage-1].HP {
		return ErrGachaGuild
	}
	if owner.RaidDamage == nil {
		owner.RaidDamage = map[int]map[int]GachaGuildRaidContribution{}
	}
	if owner.RaidDamage[stage] == nil {
		owner.RaidDamage[stage] = map[int]GachaGuildRaidContribution{}
	}
	row := owner.RaidDamage[stage][userID]
	if row.Attempts >= 14 || row.Damage > guildRaids[stage-1].HP-damage {
		return ErrGachaRunState
	}
	row.Name = fleetPublicName(owner.Members[index].Name, userID)
	row.Damage += damage
	row.Attempts++
	owner.RaidDamage[stage][userID] = row
	return nil
}

func guildRaidRanking(owner *GachaGuildState) *GachaGuildRaidRanking {
	active := guildActiveStage(owner)
	stage := min(active+1, len(guildRaids))
	view := &GachaGuildRaidRanking{Week: owner.Week, Stage: stage, StageName: guildRaids[stage-1].Name, AllCleared: active == len(guildRaids), Entries: []GachaGuildRaidRankingEntry{}}
	for userID, row := range owner.RaidDamage[stage] {
		member := slices.ContainsFunc(owner.Members, func(member GachaGuildMember) bool { return member.UserID == userID })
		view.Entries = append(view.Entries, GachaGuildRaidRankingEntry{UserID: userID, Name: row.Name, Damage: row.Damage, Attempts: row.Attempts, Member: member})
	}
	slices.SortFunc(view.Entries, func(a, b GachaGuildRaidRankingEntry) int {
		if a.Damage != b.Damage {
			return b.Damage - a.Damage
		}
		if a.UserID < b.UserID {
			return -1
		}
		if a.UserID > b.UserID {
			return 1
		}
		return 0
	})
	for i := range view.Entries {
		view.Entries[i].Rank = i + 1
	}
	return view
}

type GachaGuildShopItem struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Level     int    `json:"level"`
	Cost      int    `json:"cost"`
	Resource  string `json:"resource"`
	Amount    int    `json:"amount"`
	Limit     int    `json:"limit"`
	Bought    int    `json:"bought"`
	Remaining int    `json:"remaining"`
	Unlocked  bool   `json:"unlocked"`
	CanBuy    bool   `json:"can_buy"`
}

var guildShop = [...]GachaGuildShopItem{
	{ID: "tokens", Name: "招募代币补给", Level: 1, Cost: 80, Resource: "token", Amount: 40, Limit: 3},
	{ID: "ore", Name: "星锻矿箱", Level: 1, Cost: 75, Resource: "forge_ore", Amount: 15, Limit: 2},
	{ID: "crystal", Name: "元素结晶箱", Level: 2, Cost: 60, Resource: "mat_sr", Amount: 3, Limit: 3},
	{ID: "stardust", Name: "星屑珍藏", Level: 4, Cost: 100, Resource: "mat_ssr", Amount: 2, Limit: 2},
	{ID: "manual", Name: "经验书补给", Level: 1, Cost: 60, Resource: "training_manual", Amount: 6, Limit: 5},
	{ID: "ink", Name: "训练墨匣", Level: 1, Cost: 80, Resource: "training_ink", Amount: 20, Limit: 5},
	{ID: "seal", Name: "晋阶印章匣", Level: 2, Cost: 100, Resource: "break_seal", Amount: 5, Limit: 4},
	{ID: "core", Name: "武装核心匣", Level: 4, Cost: 160, Resource: "weapon_core", Amount: 3, Limit: 3},
}

type GachaGuildBattleResult struct {
	Report    *GachaBattleReport `json:"report"`
	Stage     int                `json:"stage"`
	StageName string             `json:"stage_name"`
	Damage    int                `json:"damage"`
	BossHP    int                `json:"boss_hp"`
	BossMaxHP int                `json:"boss_max_hp"`
	Cleared   bool               `json:"cleared"`
	Drops     []string           `json:"drops"`
}

func guildLevel(guild *GachaGuildState) int {
	xp, _ := guildGrowthXP(guild)
	level := 1
	for i, threshold := range guildLevelXP {
		if xp >= threshold {
			level = i + 1
		}
	}
	return level
}

// Read-only callers must see the same level before and after the saved migration.
func guildGrowthXP(guild *GachaGuildState) (int, bool) {
	if guild == nil || guild.XP < 0 {
		return 0, false
	}
	if guild.GrowthVersion == 2 {
		return guild.XP, guild.XP <= guildLevelXP[len(guildLevelXP)-1]
	}
	if guild.GrowthVersion != 0 || guild.XP > guildLegacyLevelXP[len(guildLegacyLevelXP)-1] {
		return 0, false
	}
	for i := len(guildLegacyLevelXP) - 1; i >= 0; i-- {
		if guild.XP >= guildLegacyLevelXP[i] {
			if i == len(guildLegacyLevelXP)-1 {
				return guildLevelXP[i], true
			}
			progress := guild.XP - guildLegacyLevelXP[i]
			return guildLevelXP[i] + progress*(guildLevelXP[i+1]-guildLevelXP[i])/(guildLegacyLevelXP[i+1]-guildLegacyLevelXP[i]), true
		}
	}
	return 0, false
}

// Validate old persisted counters before mapping. A live boss always retains at
// least one HP; rounded damage can never award an unearned stage clear.
func migrateGuildOwner(guild *GachaGuildState) bool {
	xp, valid := guildGrowthXP(guild)
	if !valid || guild.RaidVersion < 0 || guild.RaidVersion > guildRaidVersion {
		return false
	}
	original := guild
	if guild.RaidVersion < guildRaidVersion {
		legacyHP := []int{24000, 60000, 120000}
		if guild.RaidVersion == 2 {
			legacyHP = []int{300000, 900000, 2400000, 5000000, 9000000}
		}
		legacyTotal := 0
		for _, hp := range legacyHP {
			legacyTotal += hp
		}
		if guild.RaidCleared < 0 || guild.RaidCleared > (1<<len(legacyHP))-1 || guild.RaidCleared&(guild.RaidCleared+1) != 0 || guild.TotalDamage < 0 || guild.TotalDamage > legacyTotal {
			return false
		}
		migrated := *guild
		guild = &migrated
		stage := guildActiveStage(guild)
		if guild.RaidVersion == 0 {
			if guild.RaidCleared != 0 || guild.BossMaxHP != legacyHP[0] || guild.BossHP < 0 || guild.BossHP > legacyHP[0] {
				return false
			}
			if guild.BossHP == 0 {
				guild.RaidCleared, stage = 1, 1
				guild.BossHP, guild.BossMaxHP = legacyHP[1], legacyHP[1]
			}
		} else if stage == len(legacyHP) {
			if guild.BossHP != 0 || guild.BossMaxHP != legacyHP[len(legacyHP)-1] {
				return false
			}
		} else if guild.BossHP <= 0 || guild.BossHP > legacyHP[stage] || guild.BossMaxHP != legacyHP[stage] {
			return false
		}
		if stage == len(guildRaids) {
			guild.BossHP, guild.BossMaxHP = 0, guildRaids[len(guildRaids)-1].HP
		} else if stage == len(legacyHP) {
			guild.BossHP = guildRaids[stage].HP
			guild.BossMaxHP = guild.BossHP
		} else {
			guild.BossHP = int((int64(guild.BossHP)*int64(guildRaids[stage].HP) + int64(legacyHP[stage]) - 1) / int64(legacyHP[stage]))
			guild.BossMaxHP = guildRaids[stage].HP
		}
		guild.RaidVersion = guildRaidVersion
	}
	guild.XP, guild.GrowthVersion = xp, 2
	*original = *guild
	return true
}

func guildCapacity(guild *GachaGuildState) int {
	return 10 + 5*(guildLevel(guild)-1)
}

func guildCargoLimit(guild *GachaGuildState) int {
	return guildCapacity(guild) * 7
}

func guildRole(guild *GachaGuildState, id int) string {
	if guild.LeaderID == id {
		return "leader"
	}
	for _, member := range guild.Members {
		if member.UserID == id {
			if member.Role == "deputy" || member.Role == "elder" {
				return member.Role
			}
			return "member"
		}
	}
	return ""
}

func guildCanManage(guild *GachaGuildState, id int) bool {
	return slices.Contains([]string{"leader", "deputy"}, guildRole(guild, id))
}

func guildCanKick(guild *GachaGuildState, actor, target int) bool {
	if guild == nil || actor == target || target == guild.LeaderID {
		return false
	}
	role, targetRole := guildRole(guild, actor), guildRole(guild, target)
	return targetRole != "" && (role == "leader" || role == "deputy" && (targetRole == "member" || targetRole == "elder"))
}

func applyGuildKick(owner *GachaGuildState, actor, target int, state *GachaGameState) error {
	if !guildCanKick(owner, actor, target) || state.Guild == nil || state.Guild.LeaderID != owner.LeaderID || state.Guild.Code != owner.Code {
		return ErrGachaGuild
	}
	owner.Members = slices.DeleteFunc(owner.Members, func(member GachaGuildMember) bool { return member.UserID == target })
	state.Guild = nil
	return nil
}

func guildActiveStage(guild *GachaGuildState) int {
	for i := range guildRaids {
		if guild.RaidCleared&(1<<i) == 0 {
			return i
		}
	}
	return len(guildRaids)
}

func guildProgress(state *GachaGameState, now time.Time) *GachaGuildProgress {
	legacy := state.GuildProgress == nil
	if legacy {
		state.GuildProgress = &GachaGuildProgress{}
	}
	p := state.GuildProgress
	week, day := gachaWeekKey(now), gachaDay(now)
	if p.Week != week {
		p.Week, p.Claims = week, 0
		p.Bought = map[string]int{}
		p.Participation = map[string]int{}
	}
	if p.Bought == nil {
		p.Bought = map[string]int{}
	}
	if p.Participation == nil {
		p.Participation = map[string]int{}
	}
	if state.GuildRewardWeek == week {
		p.Claims |= 1
	}
	if p.Day != day {
		p.Day, p.Attempts = day, 0
		if legacy && state.Daily.Date == day && state.Daily.GuildAttacked {
			p.Attempts = 1
		}
	}
	if legacy && state.Guild != nil && state.Guild.ContributionWeek == week && state.Guild.MyDamage > 0 {
		p.Participation[state.Guild.Code] |= 1
	}
	return p
}

func syncGachaGuildProgress(state *GachaGameState, now time.Time) error {
	if state.Guild != nil && len(state.Guild.Members) > 0 && !migrateGuildOwner(state.Guild) {
		return ErrGachaRunState
	}
	p := state.GuildProgress
	if p == nil {
		if state.Guild != nil || state.GuildRewardWeek != "" {
			guildProgress(state, now)
		}
		return syncGuildActivity(state, now)
	}
	if p.Coins < 0 || p.Coins > gachaMaxResource || p.Claims < 0 || p.Claims > 31 || p.Attempts < 0 || p.Attempts > 2 || len(p.Participation) > 64 {
		return ErrGachaRunState
	}
	if !validGachaWeek(p.Week) || p.Week > gachaWeekKey(now) {
		return ErrGachaRunState
	}
	if _, err := time.Parse("2006-01-02", p.Day); err != nil || p.Day > gachaDay(now) {
		return ErrGachaRunState
	}
	for id, count := range p.Bought {
		index := slices.IndexFunc(guildShop[:], func(item GachaGuildShopItem) bool { return item.ID == id })
		if index < 0 || count < 0 || count > guildShop[index].Limit {
			return ErrGachaRunState
		}
	}
	for code, mask := range p.Participation {
		if gachaGuildLeaderFromCode(code) == 0 || code != strings.ToUpper(code) || mask < 0 || mask > 31 {
			return ErrGachaRunState
		}
	}
	guildProgress(state, now)
	return syncGuildActivity(state, now)
}

func validateGachaGuildIntent(intent GachaActionIntent) error {
	if slices.Contains([]string{"guild_checkin", "guild_activity_claim", "guild_mission_claim"}, intent.Action) {
		return validateGuildActivityIntent(intent)
	}
	if intent.Materials != nil || intent.GirlID != nil || intent.Squad != nil || intent.StakeE != nil || intent.QueueID != nil || intent.RoomID != nil || intent.Choice != nil || intent.Config != nil {
		return ErrGachaIntent
	}
	if intent.Action != "guild_role" && intent.Action != "guild_kick" && intent.OpponentID != nil {
		return ErrGachaIntent
	}
	if intent.Action != "guild_create" && intent.Action != "guild_listing" && intent.Enabled != nil {
		return ErrGachaIntent
	}
	if intent.Action != "guild_claim" && intent.Action != "guild_supply_claim" && intent.Count != nil {
		return ErrGachaIntent
	}
	switch intent.Action {
	case "guild_create":
		if intent.ItemID != nil && validGachaGuildName(*intent.ItemID) {
			return nil
		}
	case "guild_join":
		if intent.ItemID != nil && gachaGuildLeaderFromCode(*intent.ItemID) > 0 {
			return nil
		}
	case "guild_listing":
		if intent.ItemID == nil && intent.Enabled != nil {
			return nil
		}
	case "guild_supply":
		if intent.ItemID != nil && fleetCargoIndex(*intent.ItemID) >= 0 {
			return nil
		}
	case "guild_supply_claim":
		if intent.ItemID == nil && intent.Count != nil && *intent.Count >= 1 && *intent.Count <= len(fleetBeacons) {
			return nil
		}
	case "guild_claim":
		if intent.ItemID == nil && (intent.Count == nil || *intent.Count >= 1 && *intent.Count <= len(guildRaids)) {
			return nil
		}
	case "guild_leave", "guild_attack":
		if intent.ItemID == nil {
			return nil
		}
	case "guild_shop":
		if intent.ItemID != nil && slices.ContainsFunc(guildShop[:], func(item GachaGuildShopItem) bool { return item.ID == *intent.ItemID }) {
			return nil
		}
	case "guild_role":
		if intent.ItemID != nil && slices.Contains([]string{"deputy", "elder", "member"}, *intent.ItemID) && intent.OpponentID != nil && *intent.OpponentID > 0 {
			return nil
		}
	case "guild_kick":
		if intent.ItemID == nil && intent.OpponentID != nil && *intent.OpponentID > 0 {
			return nil
		}
	}
	return ErrGachaIntent
}

func guildAddProgress(state *GachaGameState, owner *GachaGuildState, xp, coins int, now time.Time) error {
	p := guildProgress(state, now)
	currentXP, valid := guildGrowthXP(owner)
	if !valid || xp < 0 || coins < 0 {
		return ErrGachaRunState
	}
	if p.Coins < 0 || p.Coins > gachaMaxResource-coins {
		return ErrGachaCapacity
	}
	p.Coins += coins
	owner.XP, owner.GrowthVersion = min(guildLevelXP[len(guildLevelXP)-1], currentXP+xp), 2
	return nil
}

func guildRaidViews(state *GachaGameState, owner *GachaGuildState, now time.Time) []GachaGuildRaidView {
	p := guildProgress(state, now)
	active := guildActiveStage(owner)
	views := make([]GachaGuildRaidView, 0, len(guildRaids))
	for i, rule := range guildRaids {
		cleared := owner.RaidCleared&(1<<i) != 0
		hp := rule.HP
		if cleared {
			hp = 0
		} else if i == active {
			hp = owner.BossHP
		}
		participated := p.Participation[owner.Code]&(1<<i) != 0
		claimed := p.Claims&(1<<i) != 0
		unlocked := guildLevel(owner) >= rule.Level
		views = append(views, GachaGuildRaidView{
			Stage: i + 1, Name: rule.Name, RequiredLevel: rule.Level, HP: hp, MaxHP: rule.HP,
			Cleared: cleared, Unlocked: unlocked, Active: i == active, Participated: participated,
			Claimed: claimed, CanClaim: cleared && participated && !claimed && unlocked,
			Resources: maps.Clone(rule.Resources), Coins: rule.Coins, Mechanic: rule.Mechanic, Description: rule.Description,
		})
	}
	return views
}

func guildShopViews(state *GachaGameState, owner *GachaGuildState, now time.Time) []GachaGuildShopItem {
	p := guildProgress(state, now)
	view := make([]GachaGuildShopItem, 0, len(guildShop))
	for _, item := range guildShop {
		item.Bought = p.Bought[item.ID]
		item.Remaining = max(0, item.Limit-item.Bought)
		item.Unlocked = guildLevel(owner) >= item.Level
		item.CanBuy = item.Unlocked && item.Remaining > 0 && p.Coins >= item.Cost
		view = append(view, item)
	}
	return view
}

func applyGuildRole(owner *GachaGuildState, userID, target int, role string) error {
	if userID != owner.LeaderID || target == owner.LeaderID {
		return ErrGachaGuild
	}
	index := slices.IndexFunc(owner.Members, func(member GachaGuildMember) bool { return member.UserID == target })
	if index < 0 {
		return ErrGachaGuild
	}
	if !slices.Contains([]string{"member", "deputy", "elder"}, role) {
		return ErrGachaIntent
	}
	limit := map[string]int{"deputy": 2, "elder": 4}[role]
	if role != "member" {
		count := 0
		for _, member := range owner.Members {
			if member.UserID != target && guildRole(owner, member.UserID) == role {
				count++
			}
		}
		if count >= limit {
			return ErrGachaGuildLimit
		}
	}
	owner.Members[index].Role = role
	return nil
}

func applyGuildShop(state *GachaGameState, owner *GachaGuildState, id string, result *GachaActionResult, now time.Time) error {
	p := guildProgress(state, now)
	for _, item := range guildShopViews(state, owner, now) {
		if item.ID != id {
			continue
		}
		if !item.CanBuy {
			return ErrGachaGuildLimit
		}
		value := state.Mats[item.Resource]
		if item.Resource == "token" {
			value = state.Token
		}
		if value > gachaMaxResource-item.Amount {
			return ErrGachaCapacity
		}
		p.Coins -= item.Cost
		p.Bought[id]++
		if item.Resource == "token" {
			state.Token += item.Amount
		} else {
			state.Mats[item.Resource] += item.Amount
		}
		result.Drops = append(result.Drops, fmt.Sprintf("%s · %s +%d · 公会币 -%d", item.Name, gachaMailResourceName(item.Resource), item.Amount, item.Cost))
		return nil
	}
	return ErrGachaIntent
}

func applyGuildClaim(state *GachaGameState, owner *GachaGuildState, tier *int, result *GachaActionResult, now time.Time) error {
	p := guildProgress(state, now)
	for i, view := range guildRaidViews(state, owner, now) {
		if tier != nil && *tier != i+1 {
			continue
		}
		if !view.CanClaim {
			if tier != nil {
				return ErrGachaGuildLimit
			}
			continue
		}
		rule := guildRaids[i]
		if p.Coins > gachaMaxResource-rule.Coins {
			return ErrGachaCapacity
		}
		keys := make([]string, 0, len(rule.Resources))
		for resource, amount := range rule.Resources {
			if state.Mats[resource] < 0 || state.Mats[resource] > gachaMaxResource-amount {
				return ErrGachaCapacity
			}
			keys = append(keys, resource)
		}
		slices.Sort(keys)
		for _, resource := range keys {
			state.Mats[resource] += rule.Resources[resource]
			result.Drops = append(result.Drops, fmt.Sprintf("%s +%d", gachaMailResourceName(resource), rule.Resources[resource]))
		}
		p.Coins += rule.Coins
		p.Claims |= 1 << i
		if i == 0 {
			state.GuildRewardWeek = gachaWeekKey(now)
		}
		result.Drops = append(result.Drops, fmt.Sprintf("%s · 公会币 +%d", rule.Name, rule.Coins))
		return nil
	}
	return ErrGachaGuildLimit
}

func simulateGuildRaid(state *GachaGameState, owner *GachaGuildState, index int, rng *rand.Rand) (*GachaBattleReport, error) {
	if index < 0 || index >= len(guildRaids) || owner.BossHP <= 0 || rng == nil || gachaCatalog == nil || len(gachaCatalog.Enemies) == 0 {
		return nil, ErrGachaGuildLimit
	}
	allies, err := gachaSquadUnits(state, "ally", "a")
	if err != nil {
		return nil, err
	}
	rule := guildRaids[index]
	template := gachaCatalog.Enemies[len(gachaCatalog.Enemies)-1]
	boss := &gachaCombatUnit{GachaBattleUnit: GachaBattleUnit{
		GachaBattleSnapshot: GachaBattleSnapshot{UID: "e0", HP: owner.BossHP, Alive: true},
		Slot:                1, Side: "enemy", Name: rule.Name, Img: template.Img, AtkType: template.AtkType,
		Elem: weeklyBossElement(owner.Week), Boss: true, Level: 20 + index*15, MaxHP: rule.HP,
		Atk: rule.Attack, Def: rule.Defense, Spd: 90 + index*10, CR: 5, CD: 130,
	}}
	report := &GachaBattleReport{
		Allies: []GachaBattleUnit{}, Waves: [][]GachaBattleUnit{{boss.GachaBattleUnit}}, Events: []GachaBattleEvent{},
	}
	for _, unit := range allies {
		report.Allies = append(report.Allies, unit.GachaBattleUnit)
	}
	battle := &gachaCombat{
		allies: allies, waves: [][]*gachaCombatUnit{{boss}}, rng: rng, formula: gachaCatalog.Formula, report: report,
		mechanic: rule.Mechanic, roundLimit: 20,
	}
	return battle.fight(20), nil
}

func guildRaidDamage(report *GachaBattleReport) int {
	if report == nil || len(report.Waves) != 1 || len(report.Waves[0]) != 1 {
		return 0
	}
	initial := report.Waves[0][0].HP
	remaining := initial
	for _, event := range report.Events {
		for _, unit := range event.Units {
			if unit.UID == "e0" {
				remaining = unit.HP
			}
		}
	}
	return max(0, min(initial, initial-remaining))
}

func applyGuildRaid(state *GachaGameState, owner *GachaGuildState, userID int, result *GachaActionResult, now time.Time) error {
	p := guildProgress(state, now)
	index := guildActiveStage(owner)
	if index >= len(guildRaids) || guildLevel(owner) < guildRaids[index].Level || p.Attempts >= 2 {
		return ErrGachaGuildLimit
	}
	if len(state.Squad) == 0 {
		return ErrGachaTeam
	}
	if state.Stamina < 10 {
		return ErrGachaFunds
	}
	if p.Coins > gachaMaxResource-25 || state.Mats["training_ink"] > gachaMaxResource-3 {
		return ErrGachaCapacity
	}
	var seed [16]byte
	if _, err := cryptorand.Read(seed[:]); err != nil {
		return err
	}
	rng := rand.New(rand.NewPCG(binary.LittleEndian.Uint64(seed[:8]), binary.LittleEndian.Uint64(seed[8:])))
	report, err := simulateGuildRaid(state, owner, index, rng)
	if err != nil {
		return err
	}
	damage := guildRaidDamage(report)
	if err := recordGuildRaidContribution(owner, index+1, userID, damage); err != nil {
		return err
	}
	if err := spendGachaStamina(state, 10, now, &result.Drops); err != nil {
		return err
	}
	p.Attempts++
	state.Daily.GuildAttacked = p.Attempts >= 2
	p.Participation[owner.Code] |= 1 << index
	state.Guild.MyDamage += damage
	owner.BossHP -= damage
	owner.TotalDamage += damage
	state.Mats["training_ink"] += 3
	if err := guildAddProgress(state, owner, 30, 25, now); err != nil {
		return err
	}
	cleared := owner.BossHP == 0
	result.GuildBattle = &GachaGuildBattleResult{
		Report: report, Stage: index + 1, StageName: guildRaids[index].Name, Damage: damage,
		BossHP: owner.BossHP, BossMaxHP: guildRaids[index].HP, Cleared: cleared,
	}
	result.Drops = append(result.Drops, fmt.Sprintf("对%s造成 %d 伤害 · 训练墨 +3 · 公会币 +25 · 公会经验 +30", guildRaids[index].Name, damage))
	if cleared {
		owner.RaidCleared |= 1 << index
		owner.XP = min(guildLevelXP[len(guildLevelXP)-1], owner.XP+guildRaids[index].XP)
		result.Drops = append(result.Drops, "阶段已通关，参战成员可领取本周首通奖励")
		next := guildActiveStage(owner)
		if next < len(guildRaids) {
			owner.BossHP = guildRaids[next].HP
			owner.BossMaxHP = owner.BossHP
		}
	}
	if err := recordGuildSharedActivity(state, owner, "raid", 1, now); err != nil {
		return err
	}
	result.GuildBattle.Drops = slices.Clone(result.Drops)
	return nil
}

// Read-only batch lookup. No row locks, including when called by ranked arena.
// A membership pointer alone never proves membership: the owner's roster does.
func gachaGuildNames(tx *gorm.DB, ids []int) (map[int]string, error) {
	names := map[int]string{}
	if len(ids) == 0 {
		return names, nil
	}
	var players []GachaPlayer
	if err := tx.Select("user_id", "state").Where("user_id IN ?", ids).Find(&players).Error; err != nil {
		return nil, err
	}
	members := map[int]*GachaGuildState{}
	leaders := []int{}
	for _, player := range players {
		var state GachaGameState
		if common.Unmarshal([]byte(player.State), &state) == nil && state.Guild != nil {
			members[player.UserID] = state.Guild
			if !slices.Contains(leaders, state.Guild.LeaderID) {
				leaders = append(leaders, state.Guild.LeaderID)
			}
		}
	}
	if len(leaders) == 0 {
		return names, nil
	}
	var owners []GachaPlayer
	if err := tx.Select("user_id", "state").Where("user_id IN ?", leaders).Find(&owners).Error; err != nil {
		return nil, err
	}
	for _, player := range owners {
		var state GachaGameState
		if common.Unmarshal([]byte(player.State), &state) != nil || !validGachaGuildOwner(state.Guild, player.UserID, fmt.Sprintf("%08X", player.UserID)) {
			continue
		}
		for id, member := range members {
			if member.LeaderID == player.UserID && member.Code == state.Guild.Code && guildRole(state.Guild, id) != "" {
				names[id] = state.Guild.Name
			}
		}
	}
	return names, nil
}

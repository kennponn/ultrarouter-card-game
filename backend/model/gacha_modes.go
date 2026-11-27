package model

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

var (
	ErrGachaMode         = errors.New("game mode is unavailable")
	ErrGachaModeLimit    = errors.New("weekly challenge limit reached")
	ErrGachaTalentLocked = errors.New("talent requires three stars")
	ErrGachaGuild        = errors.New("guild is unavailable")
	ErrGachaGuildLimit   = errors.New("guild action limit reached")
)

const (
	gachaExpeditionFloors = 10
	gachaGuildMemberLimit = 10
)

type GachaExpeditionState struct {
	Week  string `json:"week"`
	Floor int    `json:"floor"`
	Best  int    `json:"best"`
	Route string `json:"route,omitempty"`
}

type GachaWeeklyBossState struct {
	Week       string `json:"week"`
	Element    string `json:"element"`
	Attempts   int    `json:"attempts"`
	BestDamage int    `json:"best_damage"`
	Cleared    bool   `json:"cleared"`
	Mechanic   string `json:"mechanic,omitempty"`
}

type GachaGuildMember struct {
	UserID  int    `json:"user_id"`
	Name    string `json:"name"`
	Role    string `json:"role,omitempty"`
	CanKick bool   `json:"can_kick,omitempty"`
}

// The founder's locked player row owns the roster and shared boss. Other players
// store membership and personal contribution only. Invite codes are public IDs.
type GachaGuildState struct {
	XP               int                                        `json:"xp,omitempty"`
	GrowthVersion    int                                        `json:"growth_version,omitempty"`
	Missions         *GachaGuildMissions                        `json:"missions,omitempty"`
	RaidVersion      int                                        `json:"raid_version,omitempty"`
	RaidCleared      int                                        `json:"raid_cleared,omitempty"`
	RaidDamage       map[int]map[int]GachaGuildRaidContribution `json:"raid_damage,omitempty"`
	Code             string                                     `json:"code"`
	Name             string                                     `json:"name"`
	LeaderID         int                                        `json:"leader_id"`
	Members          []GachaGuildMember                         `json:"members,omitempty"`
	Week             string                                     `json:"week"`
	BossHP           int                                        `json:"boss_hp"`
	BossMaxHP        int                                        `json:"boss_max_hp"`
	TotalDamage      int                                        `json:"total_damage"`
	ContributionWeek string                                     `json:"contribution_week"`
	MyDamage         int                                        `json:"my_damage"`
	Listed           bool                                       `json:"listed,omitempty"`
	Cargo            [3]int                                     `json:"cargo,omitempty"`
	MyCargo          int                                        `json:"my_cargo,omitempty"`
}

type GachaGuildView struct {
	Level             int                     `json:"level"`
	XP                int                     `json:"xp"`
	NextXP            int                     `json:"next_xp"`
	MemberLimit       int                     `json:"member_limit"`
	Role              string                  `json:"role"`
	CanManage         bool                    `json:"can_manage"`
	CanAssign         bool                    `json:"can_assign"`
	Activity          *GachaGuildActivityView `json:"activity,omitempty"`
	Coins             int                     `json:"coins"`
	AttemptsToday     int                     `json:"attempts_today"`
	AttemptLimit      int                     `json:"attempt_limit"`
	RaidStages        []GachaGuildRaidView    `json:"raid_stages"`
	RaidDamageRanking *GachaGuildRaidRanking  `json:"raid_damage_ranking"`
	Shop              []GachaGuildShopItem    `json:"shop"`
	Code              string                  `json:"code"`
	Name              string                  `json:"name"`
	LeaderID          int                     `json:"leader_id"`
	Members           []GachaGuildMember      `json:"members"`
	BossWeek          string                  `json:"boss_week"`
	BossHP            int                     `json:"boss_hp"`
	BossMaxHP         int                     `json:"boss_max_hp"`
	TotalDamage       int                     `json:"total_damage"`
	MyDamage          int                     `json:"my_damage"`
	AttackedToday     bool                    `json:"attacked_today"`
	CanClaim          bool                    `json:"can_claim"`
	RewardClaimed     bool                    `json:"reward_claimed"`
	BossElement       string                  `json:"boss_element"`
	Listed            bool                    `json:"listed"`
	Harbor            GachaFleetHarbor        `json:"harbor"`
}

func gachaWeekKey(now time.Time) string {
	year, week := now.In(gameZone).ISOWeek()
	return fmt.Sprintf("%04d-W%02d", year, week)
}

func validGachaWeek(week string) bool {
	if len(week) != 8 || week[4:6] != "-W" {
		return false
	}
	year, err := strconv.Atoi(week[:4])
	if err != nil || year < 2000 || year > 9999 {
		return false
	}
	number, err := strconv.Atoi(week[6:])
	if err != nil || number < 1 || number > 53 {
		return false
	}
	_, lastWeek := time.Date(year, time.December, 28, 0, 0, 0, 0, gameZone).ISOWeek()
	return number <= lastWeek
}

func weeklyBossElement(week string) string {
	elements := [...]string{"fire", "plant", "water", "star", "dark", "time"}
	number, _ := strconv.Atoi(week[6:])
	return elements[(max(1, number)-1)%len(elements)]
}

func weeklyBossMechanic(week string) string {
	return map[string]string{"fire": "fury", "plant": "regrowth", "water": "barrier", "star": "reflection", "dark": "venom", "time": "silence"}[weeklyBossElement(week)]
}

func resetGachaWeeklyModes(state *GachaGameState, now time.Time) {
	week := gachaWeekKey(now)
	if state.Expedition.Week != week {
		state.Expedition = GachaExpeditionState{Week: week, Floor: 1}
	}
	if state.WeeklyBoss.Week != week {
		state.WeeklyBoss = GachaWeeklyBossState{Week: week}
	}
	state.WeeklyBoss.Element = weeklyBossElement(week)
	state.WeeklyBoss.Mechanic = weeklyBossMechanic(week)
	if state.Guild != nil && state.Guild.ContributionWeek != week {
		state.Guild.ContributionWeek, state.Guild.MyDamage = week, 0
		state.Guild.MyCargo = 0
	}
}

func specialGachaStage(stageID string, state *GachaGameState, now time.Time) (*GachaStage, GachaRunSnapshot, error) {
	resetGachaWeeklyModes(state, now)
	snapshot := GachaRunSnapshot{Version: 1, Mode: stageID, Week: gachaWeekKey(now)}
	switch stageID {
	case "expedition":
		if state.Expedition.Floor < 1 || state.Expedition.Floor > gachaExpeditionFloors ||
			!slices.Contains([]string{"assault", "bastion", "fortune"}, state.Expedition.Route) {
			return nil, snapshot, ErrGachaMode
		}
		snapshot.Floor, snapshot.Route = state.Expedition.Floor, state.Expedition.Route
	case "weekly_boss":
		if state.WeeklyBoss.Attempts >= 3 || state.WeeklyBoss.Cleared {
			return nil, snapshot, ErrGachaModeLimit
		}
	default:
		return nil, snapshot, ErrGachaStage
	}
	stage, err := specialGachaStageFromSnapshot(snapshot, stageID)
	return stage, snapshot, err
}

func specialGachaStageFromSnapshot(snapshot GachaRunSnapshot, stageID string) (*GachaStage, error) {
	if !validGachaWeek(snapshot.Week) || snapshot.Mode != stageID {
		return nil, ErrGachaRunState
	}
	if isGachaProgressionStage(stageID) {
		stage := snapshot.ProgressionStage
		if snapshot.ProgressionVersion < 1 || stage == nil || stage.ID != stageID || stage.Exp < 0 || stage.Exp > 10_000 || stage.Cost < 0 || stage.Cost > 24 ||
			stage.TokenRange != [2]int{} || stage.Gold != [2]int{} || stage.Mat != [2]int{} {
			return nil, ErrGachaRunState
		}
		return stage, nil
	}
	switch snapshot.Mode {
	case "expedition":
		floor := snapshot.Floor
		if floor < 1 || floor > gachaExpeditionFloors || !slices.Contains([]string{"assault", "bastion", "fortune"}, snapshot.Route) {
			return nil, ErrGachaRunState
		}
		stage := &GachaStage{ID: stageID, Name: fmt.Sprintf("星穹远征 · 第 %d 层", floor), Img: "assets/d06-observatory.webp", Cost: 12 + floor,
			EnemyLv: 10 + floor*3, Waves: 1, EnemyCount: 2 + floor/4, StarRound: 4 + floor, Exp: 20 + floor*10,
			HasBoss: floor%5 == 0, TokenRange: [2]int{8 + floor*2, 12 + floor*2}, Gold: [2]int{40 + floor*15, 60 + floor*15}, Mat: [2]int{1, 2}, MatKey: "mat_r"}
		if floor >= 4 && floor <= 6 {
			stage.MatKey = "mat_sr"
		} else if floor >= 7 {
			stage.MatKey = "mat_ssr"
		}
		if stage.HasBoss {
			stage.EnemyCount, stage.BossName, stage.BossElem = 1, "星穹守关者", weeklyBossElement(snapshot.Week)
			stage.BossHPM, stage.BossAtkM = 1+float64(floor)*0.12, 1+float64(floor)*0.04
		}
		return stage, nil
	case "weekly_boss":
		if snapshot.Floor != 0 || snapshot.Route != "" {
			return nil, ErrGachaRunState
		}
		return &GachaStage{ID: stageID, Name: "每周元素首领 · 星蚀巨像", Img: "assets/d06-observatory.webp", Cost: 20, EnemyLv: 42,
			Waves: 1, EnemyCount: 1, StarRound: 12, HasBoss: true, BossElem: weeklyBossElement(snapshot.Week), BossName: "星蚀巨像", BossHPM: 2.2, BossAtkM: 1.15, Mechanic: weeklyBossMechanic(snapshot.Week)}, nil
	default:
		return nil, ErrGachaRunState
	}
}

// Special modes award game resources only; ordinary dungeon wallet rewards stay
// on their existing path. Settlement uses the issued week, even across rollover.
func settleGachaMode(state *GachaGameState, stage *GachaStage, snapshot *GachaRunSnapshot, report *GachaBattleReport, result *GachaFinishResult, abandon bool) error {
	if snapshot.Mode == "weekly_boss" {
		if !abandon && state.WeeklyBoss.Week == snapshot.Week {
			state.WeeklyBoss.BestDamage = max(state.WeeklyBoss.BestDamage, gachaReportDamage(report))
		}
		if !result.Win {
			return nil
		}
		if state.Gold > gachaMaxResource-500 || state.Mats["mat_ssr"] > gachaMaxResource-8 {
			return ErrGachaFunds
		}
		state.Gold += 500
		state.Mats["mat_ssr"] += 8
		if state.WeeklyBoss.Week == snapshot.Week {
			state.WeeklyBoss.Cleared = true
		}
		result.Drops = append(result.Drops, "首领首通奖励：金币 +500、星屑 x8")
		return nil
	}
	if snapshot.Mode != "expedition" {
		return ErrGachaRunState
	}
	if !result.Win {
		return nil
	}
	if state.Expedition.Week == snapshot.Week && state.Expedition.Floor != snapshot.Floor {
		return ErrGachaRunState
	}
	token, err := gachaRandomRange(stage.TokenRange)
	if err != nil {
		return err
	}
	gold, err := gachaRandomRange(stage.Gold)
	if err != nil {
		return err
	}
	if snapshot.Route == "fortune" {
		token = (token*125 + 99) / 100
		gold = (gold*125 + 99) / 100
	}
	if snapshot.MultiUsed {
		token *= 2
	}
	token = min(token, max(0, gachaCatalog.TokenBudget-state.Daily.TokenGained))
	material, err := gachaRandomRange(stage.Mat)
	if err != nil {
		return err
	}
	if state.Token > gachaMaxResource-token || state.Gold > gachaMaxResource-gold || state.Mats[stage.MatKey] > gachaMaxResource-material {
		return ErrGachaFunds
	}
	state.Token += token
	state.Daily.TokenGained += token
	state.Gold += gold
	state.Mats[stage.MatKey] += material
	if token > 0 {
		result.Drops = append(result.Drops, fmt.Sprintf("游戏代币 +%d", token))
	}
	result.Drops = append(result.Drops, fmt.Sprintf("金币 +%d", gold))
	matName := map[string]string{"mat_r": "微光结晶", "mat_sr": "元素结晶", "mat_ssr": "星屑"}[stage.MatKey]
	result.Drops = append(result.Drops, fmt.Sprintf("%s x%d", matName, material))
	if err := awardGachaExperience(state, snapshot.Squad, stage.Exp, &result.Drops); err != nil {
		return err
	}
	if state.Expedition.Week == snapshot.Week {
		state.Expedition.Best = max(state.Expedition.Best, snapshot.Floor)
		state.Expedition.Floor++
	}
	if snapshot.Floor == gachaExpeditionFloors {
		gemIDs := []string{"ruby", "jade", "sapphire", "wind", "amethyst"}
		index, err := gachaRandom(len(gemIDs))
		if err != nil {
			return err
		}
		gemID := gemIDs[index]
		if state.GemInventory[gemID] >= gachaMaxResource {
			return ErrGachaFunds
		}
		state.GemInventory[gemID]++
		result.Drops = append(result.Drops, gachaGemSpecs[gemID].Name+" x1 · 远征全通")
	}
	return nil
}

func gachaReportDamage(report *GachaBattleReport) int {
	damage := 0
	for _, event := range report.Events {
		for _, target := range event.Targets {
			if strings.HasPrefix(target.UID, "e") {
				damage += target.Damage
			}
		}
	}
	return damage
}

func gachaGuildLeaderFromCode(code string) int {
	if len(code) != 8 {
		return 0
	}
	number, err := strconv.ParseUint(code, 16, 32)
	if err != nil || number == 0 {
		return 0
	}
	return int(number)
}

func validGachaGuildName(name string) bool {
	name = strings.TrimSpace(name)
	if !utf8.ValidString(name) || utf8.RuneCountInString(name) < 2 || utf8.RuneCountInString(name) > 12 {
		return false
	}
	return !strings.ContainsFunc(name, unicode.IsControl)
}

func resetGuildBoss(guild *GachaGuildState, now time.Time) {
	if !migrateGuildOwner(guild) {
		return
	}
	week := gachaWeekKey(now)
	if guild.Week != week {
		guild.Week, guild.BossMaxHP, guild.BossHP, guild.TotalDamage = week, guildRaids[0].HP, guildRaids[0].HP, 0
		guild.Cargo = [3]int{}
		guild.RaidCleared = 0
		guild.RaidDamage = nil
		guild.RaidVersion = guildRaidVersion
	}
}

func validGachaGuildOwner(guild *GachaGuildState, leaderID int, code string) bool {
	if guild == nil {
		return false
	}
	copy := *guild
	if !migrateGuildOwner(&copy) {
		return false
	}
	guild = &copy
	if guild == nil || guild.LeaderID != leaderID || guild.Code != code || len(guild.Members) < 1 || len(guild.Members) > guildCapacity(guild) {
		return false
	}
	if !slices.Contains([]int{0, 1, 3, 7, 15, 31}, guild.RaidCleared) {
		return false
	}
	if !validGuildRaidContributions(guild) {
		return false
	}
	if !validGachaWeek(guild.Week) || guild.BossHP < 0 || guild.TotalDamage < 0 || guild.TotalDamage > guildRaidTotalHP() {
		return false
	}
	stage := guildActiveStage(guild)
	if stage == len(guildRaids) {
		if guild.BossHP != 0 || guild.BossMaxHP != guildRaids[len(guildRaids)-1].HP {
			return false
		}
	} else if guild.BossHP <= 0 || guild.BossHP > guildRaids[stage].HP || guild.BossMaxHP != guildRaids[stage].HP {
		return false
	}
	seen := make(map[int]bool, len(guild.Members))
	deputies, elders := 0, 0
	for _, member := range guild.Members {
		if member.UserID <= 0 || seen[member.UserID] || !slices.Contains([]string{"", "leader", "deputy", "elder", "member"}, member.Role) || member.Role == "leader" && member.UserID != leaderID {
			return false
		}
		if member.UserID == leaderID && member.Role != "" && member.Role != "leader" {
			return false
		}
		if member.Role == "deputy" {
			deputies++
		}
		if member.Role == "elder" {
			elders++
		}
		seen[member.UserID] = true
	}
	return seen[leaderID] && deputies <= 2 && elders <= 4
}

func getGachaGuildView(tx *gorm.DB, userID int, state *GachaGameState, now time.Time) (*GachaGuildView, error) {
	if state.Guild == nil {
		return nil, nil
	}
	owner := state.Guild
	if owner.LeaderID != userID {
		var player GachaPlayer
		if err := tx.Select("user_id", "state").Where("user_id = ?", owner.LeaderID).First(&player).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, nil
			}
			return nil, err
		}
		var leaderState GachaGameState
		if err := common.Unmarshal([]byte(player.State), &leaderState); err != nil {
			return nil, err
		}
		owner = leaderState.Guild
	}
	if !validGachaGuildOwner(owner, state.Guild.LeaderID, state.Guild.Code) || !slices.ContainsFunc(owner.Members, func(member GachaGuildMember) bool { return member.UserID == userID }) {
		return nil, nil
	}
	viewOwner := *owner
	resetGuildBoss(&viewOwner, now)
	owner = &viewOwner
	week := gachaWeekKey(now)
	myDamage := 0
	if state.Guild.ContributionWeek == week {
		myDamage = state.Guild.MyDamage
	}
	progress := guildProgress(state, now)
	raids := guildRaidViews(state, &viewOwner, now)
	canClaim := slices.ContainsFunc(raids, func(stage GachaGuildRaidView) bool { return stage.CanClaim })
	claimed := progress.Claims&1 != 0
	level, nextXP := guildLevel(&viewOwner), 0
	if level < len(guildLevelXP) {
		nextXP = guildLevelXP[level]
	}
	members := slices.Clone(owner.Members)
	for i := range members {
		members[i].Role = guildRole(owner, members[i].UserID)
		members[i].CanKick = guildCanKick(owner, userID, members[i].UserID)
	}
	return &GachaGuildView{Code: owner.Code, Name: owner.Name, LeaderID: owner.LeaderID, Members: members,
		Level: level, XP: owner.XP, NextXP: nextXP, MemberLimit: guildCapacity(owner), Role: guildRole(owner, userID),
		CanManage: guildCanManage(owner, userID), CanAssign: owner.LeaderID == userID,
		Activity:          guildActivityView(state, owner, now),
		RaidDamageRanking: guildRaidRanking(owner),
		Coins:             progress.Coins, AttemptsToday: progress.Attempts, AttemptLimit: 2, RaidStages: raids, Shop: guildShopViews(state, owner, now),
		BossWeek: week, BossHP: viewOwner.BossHP, BossMaxHP: viewOwner.BossMaxHP, TotalDamage: viewOwner.TotalDamage,
		MyDamage: myDamage, AttackedToday: progress.Attempts >= 2, RewardClaimed: claimed, Listed: owner.Listed,
		Harbor:   fleetHarborView(state, &viewOwner, week),
		CanClaim: canClaim, BossElement: weeklyBossElement(week)}, nil
}

func applyGachaGuildAction(userID int, intent GachaActionIntent, now time.Time) (*GachaActionResult, error) {
	if intent.Action == "guild_kick" {
		return applyGachaGuildKickAction(userID, intent, now)
	}
	intent.RequestID = strings.ToLower(intent.RequestID)
	hash, err := gachaIntentHash(intent)
	if err != nil {
		return nil, err
	}
	leaderID := 0
	if intent.Action == "guild_join" {
		leaderID = gachaGuildLeaderFromCode(*intent.ItemID)
	} else if intent.Action != "guild_create" {
		// Locate the second lock before opening a transaction, including with a
		// one-connection pool. Membership is rechecked after both rows are locked.
		var current GachaPlayer
		lookup := DB.Select("state").Where("user_id = ?", userID).First(&current).Error
		if lookup != nil && !errors.Is(lookup, gorm.ErrRecordNotFound) {
			return nil, lookup
		}
		if lookup == nil {
			var saved GachaGameState
			if err := common.Unmarshal([]byte(current.State), &saved); err != nil {
				return nil, err
			}
			if saved.Guild != nil {
				leaderID = saved.Guild.LeaderID
			}
		}
	}
	result := &GachaActionResult{}
	err = gachaTransaction(func(tx *gorm.DB) error {
		var ownerPlayer GachaPlayer
		var ownerState GachaGameState
		var ownerMissing bool
		loadOwner := func() error {
			if leaderID <= 0 || leaderID == userID {
				return nil
			}
			err := lockForUpdate(tx).Where("user_id = ?", leaderID).First(&ownerPlayer).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				ownerMissing = true
				return nil
			}
			if err != nil {
				return err
			}
			if err := decodeGachaCipherDocument([]byte(ownerPlayer.State), &ownerState); err != nil {
				return err
			}
			if err := migrateGachaEffort(&ownerState); err != nil {
				return err
			}
			resetGachaWeeklyModes(&ownerState, now)
			if err := syncGachaGuildProgress(&ownerState, now); err != nil {
				return err
			}
			return nil
		}
		if leaderID > 0 && leaderID < userID {
			if err := loadOwner(); err != nil {
				return err
			}
		}
		player, state, err := loadGachaPlayer(tx, userID, now)
		if err != nil {
			return err
		}
		if leaderID > userID {
			if err := loadOwner(); err != nil {
				return err
			}
		}
		if replay, err := gachaReplayResult(tx, userID, intent.RequestID, hash, result); replay || err != nil {
			return err
		}
		*result = GachaActionResult{}
		owner := &ownerState
		if leaderID == userID {
			owner = state
		}
		if intent.Action != "guild_create" && intent.Action != "guild_join" && state.Guild != nil && state.Guild.LeaderID != leaderID {
			return ErrGachaConcurrent
		}
		week := gachaWeekKey(now)
		ownerChanged := false
		switch intent.Action {
		case "guild_create":
			if state.Guild != nil || uint64(userID) > uint64(^uint32(0)) {
				return ErrGachaGuild
			}
			var user User
			if err := tx.Select("display_name").First(&user, userID).Error; err != nil {
				return err
			}
			state.Guild = &GachaGuildState{Code: fmt.Sprintf("%08X", userID), Name: strings.TrimSpace(*intent.ItemID), LeaderID: userID,
				Listed: intent.Enabled != nil && *intent.Enabled, GrowthVersion: 2, RaidVersion: guildRaidVersion, Members: []GachaGuildMember{{UserID: userID, Name: fleetPublicName(user.DisplayName, userID), Role: "leader"}}, Week: week, BossHP: guildRaids[0].HP, BossMaxHP: guildRaids[0].HP, ContributionWeek: week}
		case "guild_join":
			code := strings.ToUpper(*intent.ItemID)
			if state.Guild != nil || ownerMissing || leaderID == userID || !validGachaGuildOwner(owner.Guild, leaderID, code) {
				return ErrGachaGuild
			}
			if len(owner.Guild.Members) >= guildCapacity(owner.Guild) {
				return ErrGachaGuildLimit
			}
			var leader User
			if err := tx.Select("status").First(&leader, leaderID).Error; err != nil {
				return err
			}
			if leader.Status != common.UserStatusEnabled {
				return ErrGachaGuild
			}
			if slices.ContainsFunc(owner.Guild.Members, func(member GachaGuildMember) bool { return member.UserID == userID }) {
				return ErrGachaGuild
			}
			var user User
			if err := tx.Select("display_name").First(&user, userID).Error; err != nil {
				return err
			}
			owner.Guild.Members = append(owner.Guild.Members, GachaGuildMember{UserID: userID, Name: fleetPublicName(user.DisplayName, userID), Role: "member"})
			state.Guild = &GachaGuildState{Code: code, Name: owner.Guild.Name, LeaderID: leaderID, ContributionWeek: week}
			ownerChanged = true
		case "guild_leave":
			if state.Guild == nil {
				return ErrGachaGuild
			}
			if !ownerMissing && validGachaGuildOwner(owner.Guild, leaderID, state.Guild.Code) {
				if leaderID == userID && len(owner.Guild.Members) > 1 {
					return ErrGachaGuildLimit
				}
				if leaderID != userID {
					owner.Guild.Members = slices.DeleteFunc(owner.Guild.Members, func(member GachaGuildMember) bool { return member.UserID == userID })
					ownerChanged = true
				}
			}
			state.Guild = nil
		case "guild_attack", "guild_claim", "guild_listing", "guild_supply", "guild_supply_claim", "guild_shop", "guild_role", "guild_checkin", "guild_activity_claim", "guild_mission_claim":
			if state.Guild == nil || ownerMissing || !validGachaGuildOwner(owner.Guild, leaderID, state.Guild.Code) ||
				!slices.ContainsFunc(owner.Guild.Members, func(member GachaGuildMember) bool { return member.UserID == userID }) {
				return ErrGachaGuild
			}
			resetGuildBoss(owner.Guild, now)
			switch intent.Action {
			case "guild_checkin", "guild_activity_claim", "guild_mission_claim":
				if err := applyGuildActivity(state, owner.Guild, intent, result, now); err != nil {
					return err
				}
				ownerChanged = true
			case "guild_listing":
				if !guildCanManage(owner.Guild, userID) {
					return ErrGachaGuild
				}
				owner.Guild.Listed = *intent.Enabled
				ownerChanged = true
			case "guild_role":
				if err := applyGuildRole(owner.Guild, userID, *intent.OpponentID, *intent.ItemID); err != nil {
					return err
				}
				ownerChanged = true
			case "guild_supply", "guild_supply_claim":
				if err := applyFleetCargo(state, owner.Guild, intent, result, now); err != nil {
					return err
				}
				ownerChanged = intent.Action == "guild_supply"
			case "guild_shop":
				if err := applyGuildShop(state, owner.Guild, *intent.ItemID, result, now); err != nil {
					return err
				}
			case "guild_claim":
				if err := applyGuildClaim(state, owner.Guild, intent.Count, result, now); err != nil {
					return err
				}
			case "guild_attack":
				if err := applyGuildRaid(state, owner.Guild, userID, result, now); err != nil {
					return err
				}
				ownerChanged = true
			}
		default:
			return ErrGachaIntent
		}
		if ownerChanged && leaderID != userID {
			if err := storeGachaPlayer(tx, &ownerPlayer, owner, now); err != nil {
				return err
			}
		}
		if intent.Action == "guild_attack" {
			recordGachaProgressionActivity(state, "weekly", now)
		}
		if err := storeGachaPlayer(tx, player, state, now); err != nil {
			return err
		}
		result.Save, result.Revision = state, player.Revision
		result.Guild, err = getGachaGuildView(tx, userID, state, now)
		if err != nil {
			return err
		}
		return recordGachaAction(tx, userID, intent.RequestID, hash, result, now)
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Kicking changes three players when a deputy acts. Lock every distinct row in
// user-ID order before reading authority or membership, matching join/leave.
func applyGachaGuildKickAction(userID int, intent GachaActionIntent, now time.Time) (*GachaActionResult, error) {
	intent.RequestID = strings.ToLower(intent.RequestID)
	hash, err := gachaIntentHash(intent)
	if err != nil {
		return nil, err
	}
	targetID := *intent.OpponentID
	var hinted GachaPlayer
	if err := DB.Select("state").Where("user_id = ?", userID).First(&hinted).Error; err != nil {
		return nil, ErrGachaGuild
	}
	var hint GachaGameState
	if err := common.Unmarshal([]byte(hinted.State), &hint); err != nil {
		return nil, err
	}
	leaderID := 0
	if hint.Guild != nil {
		leaderID = hint.Guild.LeaderID
	}
	ids := []int{userID, targetID}
	if leaderID > 0 {
		ids = append(ids, leaderID)
	}
	slices.Sort(ids)
	ids = slices.Compact(ids)
	result := &GachaActionResult{}
	err = gachaTransaction(func(tx *gorm.DB) error {
		players := map[int]*GachaPlayer{}
		for _, id := range ids {
			player := &GachaPlayer{}
			if err := lockForUpdate(tx).Where("user_id = ?", id).First(player).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) && id != userID {
					continue
				}
				return err
			}
			players[id] = player
		}
		player, state, err := loadGachaPlayer(tx, userID, now)
		if err != nil {
			return err
		}
		if replay, err := gachaReplayResult(tx, userID, intent.RequestID, hash, result); replay || err != nil {
			return err
		}
		if state.Guild == nil || state.Guild.LeaderID != leaderID || players[leaderID] == nil || players[targetID] == nil || targetID == userID {
			return ErrGachaGuild
		}
		players[userID] = player
		states := map[int]*GachaGameState{userID: state}
		for _, id := range ids {
			if id == userID {
				continue
			}
			other := &GachaGameState{}
			if err := decodeGachaCipherDocument([]byte(players[id].State), other); err != nil {
				return err
			}
			if err := migrateGachaEffort(other); err != nil {
				return err
			}
			resetGachaWeeklyModes(other, now)
			if err := syncGachaGuildProgress(other, now); err != nil {
				return err
			}
			states[id] = other
		}
		owner := states[leaderID].Guild
		if !validGachaGuildOwner(owner, leaderID, state.Guild.Code) || guildRole(owner, userID) == "" {
			return ErrGachaGuild
		}
		resetGuildBoss(owner, now)
		if err := applyGuildKick(owner, userID, targetID, states[targetID]); err != nil {
			return err
		}
		for _, id := range ids {
			if err := storeGachaPlayer(tx, players[id], states[id], now); err != nil {
				return err
			}
		}
		*result = GachaActionResult{Save: state, Revision: player.Revision, Drops: []string{"已移出该成员，个人公会币与领奖记录保留"}}
		result.Guild, err = getGachaGuildView(tx, userID, state, now)
		if err != nil {
			return err
		}
		return recordGachaAction(tx, userID, intent.RequestID, hash, result, now)
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

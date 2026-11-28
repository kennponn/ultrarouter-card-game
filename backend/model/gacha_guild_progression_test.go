package model

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func guildTestState(now time.Time) (*GachaGameState, *GachaGuildState) {
	owner := &GachaGuildState{
		Code: "00000001", Name: "测试母港", LeaderID: 1, XP: 28000, GrowthVersion: 2,
		Week: gachaWeekKey(now), RaidVersion: guildRaidVersion, BossHP: guildRaids[0].HP, BossMaxHP: guildRaids[0].HP,
		Members: []GachaGuildMember{{UserID: 1, Role: "leader"}, {UserID: 2, Role: "member"}},
	}
	state := battleTestState([]string{"c08", "c06", "c01", "c12"}, 30, 3)
	state.Daily.Date = gachaDay(now)
	state.Guild = &GachaGuildState{Code: owner.Code, LeaderID: owner.LeaderID, ContributionWeek: owner.Week}
	guildProgress(state, now)
	return state, owner
}

func TestGuildLevelsPersistWhileWeeklyRaidAndPersonalLimitsReset(t *testing.T) {
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, gameZone)
	state, owner := guildTestState(now)
	assert.Equal(t, 5, guildLevel(owner))
	assert.Equal(t, 30, guildCapacity(owner))
	assert.Equal(t, 210, guildCargoLimit(owner), "every member can supply the same cargo lane for seven days")
	owner.RaidCleared, owner.BossHP, owner.TotalDamage = 3, guildRaids[2].HP, guildRaids[0].HP+guildRaids[1].HP
	owner.Cargo = [3]int{4, 3, 2}
	p := state.GuildProgress
	p.Coins, p.Claims, p.Attempts = 160, 3, 2
	p.Bought["tokens"], p.Participation[owner.Code] = 3, 3
	state.Guild = nil
	require.NoError(t, syncGachaGuildProgress(state, now))
	assert.Equal(t, 3, p.Claims, "leaving must not restore claims")
	assert.Equal(t, 3, p.Bought["tokens"], "leaving must not restore stock")
	assert.Equal(t, 2, p.Attempts, "leaving must not restore attempts")
	future := now.AddDate(0, 0, 7)
	resetGuildBoss(owner, future)
	require.NoError(t, syncGachaGuildProgress(state, future))
	assert.Equal(t, 28000, owner.XP)
	assert.Equal(t, guildRaids[0].HP, owner.BossHP)
	assert.Zero(t, owner.RaidCleared)
	assert.Equal(t, [3]int{}, owner.Cargo)
	assert.Equal(t, 160, p.Coins)
	assert.Zero(t, p.Claims)
	assert.Zero(t, p.Attempts)
	assert.Empty(t, p.Bought)
	assert.Empty(t, p.Participation)
}

func TestGuildMarketStockSurvivesMidnightAndRefreshesSeparately(t *testing.T) {
	sunday := time.Date(2026, 11, 29, 21, 1, 0, 0, gameZone)
	monday := sunday.Add(4 * time.Hour)
	state, owner := guildTestState(sunday)
	state.GuildProgress.Coins, state.Token = 1000, 100
	require.NoError(t, syncGachaMarket(state, sunday))
	state.Market.Bought["gold"] = 2
	for range 3 {
		require.NoError(t, applyGuildShop(state, owner, "tokens", &GachaActionResult{}, sunday))
	}
	require.NoError(t, syncGachaMarket(state, monday))
	require.NoError(t, syncGachaGuildProgress(state, monday))
	require.ErrorIs(t, applyGuildShop(state, owner, "tokens", &GachaActionResult{}, monday), ErrGachaGuildLimit)
	shop, period := "guild", strconv.FormatInt(state.Market.Slot, 10)
	intent := GachaActionIntent{Action: "shop_refresh", ItemID: &shop, Choice: &period}
	before := state.Token
	require.NoError(t, validateGachaMarketIntent(intent))
	require.NoError(t, applyGachaMarketIntent(state, intent, &GachaActionResult{}, monday))
	assert.Equal(t, before-gachaMarket.RefreshPrice, state.Token)
	assert.Empty(t, state.GuildProgress.Bought)
	assert.Equal(t, 2, state.Market.Bought["gold"])
	require.ErrorIs(t, applyGachaMarketIntent(state, intent, &GachaActionResult{}, monday), ErrGachaShopLimit)
	require.NoError(t, applyGuildShop(state, owner, "tokens", &GachaActionResult{}, monday))
	opening := time.Date(2026, 11, 30, 9, 0, 0, 0, gameZone)
	require.NoError(t, syncGachaGuildProgress(state, opening))
	assert.Empty(t, state.GuildProgress.Bought)
}

func TestGuildRoleAuthorityAndPositionLimits(t *testing.T) {
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, gameZone)
	_, owner := guildTestState(now)
	for id := 3; id <= 9; id++ {
		owner.Members = append(owner.Members, GachaGuildMember{UserID: id})
	}
	require.ErrorIs(t, applyGuildRole(owner, 2, 3, "deputy"), ErrGachaGuild)
	require.ErrorIs(t, applyGuildRole(owner, 1, 1, "member"), ErrGachaGuild)
	require.ErrorIs(t, applyGuildRole(owner, 1, 99, "elder"), ErrGachaGuild)
	require.NoError(t, applyGuildRole(owner, 1, 2, "deputy"))
	require.NoError(t, applyGuildRole(owner, 1, 3, "deputy"))
	require.ErrorIs(t, applyGuildRole(owner, 1, 4, "deputy"), ErrGachaGuildLimit)
	for id := 4; id <= 7; id++ {
		require.NoError(t, applyGuildRole(owner, 1, id, "elder"))
	}
	require.ErrorIs(t, applyGuildRole(owner, 1, 8, "elder"), ErrGachaGuildLimit)
	assert.True(t, guildCanManage(owner, 2))
	assert.False(t, guildCanManage(owner, 4))
	require.NoError(t, applyGuildRole(owner, 1, 2, "member"))
	require.NoError(t, applyGuildRole(owner, 1, 4, "deputy"))
	assert.False(t, guildCanManage(owner, 2))
	assert.True(t, validGachaGuildOwner(owner, 1, owner.Code))
}

func TestGuildClaimsRequireThisGuildStageAndStayClaimedAcrossMembership(t *testing.T) {
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, gameZone)
	state, owner := guildTestState(now)
	owner.RaidCleared = 3
	state.GuildProgress.Participation[owner.Code] = 2
	state.Guild.MyDamage = 900
	one, two := 1, 2
	require.ErrorIs(t, applyGuildClaim(state, owner, &one, &GachaActionResult{}, now), ErrGachaGuildLimit)
	owner.XP = 0
	require.ErrorIs(t, applyGuildClaim(state, owner, &two, &GachaActionResult{}, now), ErrGachaGuildLimit)
	owner.XP = 28000
	gold, token := state.Gold, state.Token
	require.NoError(t, applyGuildClaim(state, owner, &two, &GachaActionResult{}, now))
	assert.Equal(t, token, state.Token)
	assert.Equal(t, gold, state.Gold)
	assert.Equal(t, 16, state.Mats["mat_ssr"])
	assert.Equal(t, 1000, state.GuildProgress.Coins)
	other := *owner
	other.Code, other.LeaderID = "00000003", 3
	state.Guild = &GachaGuildState{Code: other.Code, LeaderID: 3, ContributionWeek: owner.Week}
	state.GuildProgress.Participation[other.Code] = 2
	require.ErrorIs(t, applyGuildClaim(state, &other, &two, &GachaActionResult{}, now), ErrGachaGuildLimit)
}

func TestGuildShopDebitsOnlyAfterCapacityAndRetainsWeeklyStock(t *testing.T) {
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, gameZone)
	state, owner := guildTestState(now)
	state.GuildProgress.Coins = 500
	state.Token = gachaMaxResource
	require.ErrorIs(t, applyGuildShop(state, owner, "tokens", &GachaActionResult{}, now), ErrGachaCapacity)
	assert.Equal(t, 500, state.GuildProgress.Coins)
	assert.Zero(t, state.GuildProgress.Bought["tokens"])
	state.Token = 0
	for range 3 {
		require.NoError(t, applyGuildShop(state, owner, "tokens", &GachaActionResult{}, now))
	}
	assert.Equal(t, 120, state.Token)
	assert.Equal(t, 260, state.GuildProgress.Coins)
	owner.Code = "00000007"
	require.ErrorIs(t, applyGuildShop(state, owner, "tokens", &GachaActionResult{}, now), ErrGachaGuildLimit)
}

func TestGuildRaidUsesReportHealthAndDoesNotMutateSimulationInputs(t *testing.T) {
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, gameZone)
	state, owner := guildTestState(now)
	owner.BossHP = 901
	beforeHP, beforeStamina := owner.BossHP, state.Stamina
	report, err := simulateGuildRaid(state, owner, 0, battleTestRNG())
	require.NoError(t, err)
	require.NotEmpty(t, report.Events)
	assert.Equal(t, beforeHP, report.Waves[0][0].HP)
	assert.Equal(t, gachaBattleReportVersion, report.Version)
	assert.LessOrEqual(t, report.Rounds, 20)
	assert.Greater(t, report.DurationMS, 0)
	assert.Equal(t, beforeHP, owner.BossHP)
	assert.Equal(t, beforeStamina, state.Stamina)
	lastHP := beforeHP
	for _, event := range report.Events {
		for _, unit := range event.Units {
			if unit.UID == "e0" {
				lastHP = unit.HP
			}
		}
	}
	assert.Equal(t, beforeHP-lastHP, guildRaidDamage(report))
	assert.Positive(t, guildRaidDamage(report))
	assert.LessOrEqual(t, guildRaidDamage(report), beforeHP)
}

func TestGuildProgressRejectsMalformedCountersBeforeReset(t *testing.T) {
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, gameZone)
	for _, mutate := range []func(*GachaGuildProgress){
		func(p *GachaGuildProgress) { p.Coins = -1 },
		func(p *GachaGuildProgress) { p.Claims = 32 },
		func(p *GachaGuildProgress) { p.Attempts = -1 },
		func(p *GachaGuildProgress) { p.Bought["tokens"] = 4 },
		func(p *GachaGuildProgress) { p.Bought["unknown"] = 1 },
		func(p *GachaGuildProgress) { p.Participation["00000001"] = 32 },
		func(p *GachaGuildProgress) { p.Week = "2099-W01" },
	} {
		state, _ := guildTestState(now)
		mutate(state.GuildProgress)
		require.ErrorIs(t, syncGachaGuildProgress(state, now), ErrGachaRunState)
	}
}

func TestGuildLegacyClaimAndContributionMigrateOnce(t *testing.T) {
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, gameZone)
	state, owner := guildTestState(now)
	state.GuildProgress = nil
	state.Guild.MyDamage = 12
	state.GuildRewardWeek = owner.Week
	state.Daily.GuildAttacked = true
	require.NoError(t, syncGachaGuildProgress(state, now))
	assert.Equal(t, 1, state.GuildProgress.Claims)
	assert.Equal(t, 1, state.GuildProgress.Attempts)
	assert.Equal(t, 1, state.GuildProgress.Participation[owner.Code])
	state.Guild.Code = "00000007"
	require.NoError(t, syncGachaGuildProgress(state, now))
	assert.Zero(t, state.GuildProgress.Participation[state.Guild.Code], "new membership cannot reuse old contribution")
}

func TestGuildDailyResetAndCompletedLegacyBossAdvanceOnlyOnce(t *testing.T) {
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, gameZone)
	state, owner := guildTestState(now)
	p := state.GuildProgress
	p.Attempts, p.Claims, p.Coins = 2, 1, 80
	p.Bought["tokens"] = 2
	state.Daily.GuildAttacked = true
	tomorrow := now.AddDate(0, 0, 1)
	require.NoError(t, syncGachaGuildProgress(state, tomorrow))
	assert.Zero(t, p.Attempts, "old day flag cannot consume a new day's attempt")
	assert.Equal(t, 1, p.Claims)
	assert.Equal(t, 2, p.Bought["tokens"])
	assert.Equal(t, 80, p.Coins)
	owner.RaidVersion, owner.RaidCleared, owner.BossHP = 0, 0, 0
	owner.BossMaxHP = 24000
	require.True(t, validGachaGuildOwner(owner, 1, owner.Code))
	resetGuildBoss(owner, now)
	assert.Equal(t, 1, owner.RaidCleared)
	assert.Equal(t, guildRaids[1].HP, owner.BossHP)
	owner.BossHP = 50000
	resetGuildBoss(owner, now)
	assert.Equal(t, 50000, owner.BossHP, "migration must not heal the next stage repeatedly")
	assert.True(t, validGachaGuildOwner(owner, 1, owner.Code))
	owner.RaidCleared = 2
	assert.False(t, validGachaGuildOwner(owner, 1, owner.Code), "stages cannot be cleared out of order")
}

func TestGuildNewIntentRejectsExtraFieldsAndUnrecognizedRoles(t *testing.T) {
	member, role, item, invalid, count := 2, "deputy", "tokens", "leader", 1
	require.NoError(t, validateGachaGuildIntent(GachaActionIntent{Action: "guild_role", OpponentID: &member, ItemID: &role}))
	require.NoError(t, validateGachaGuildIntent(GachaActionIntent{Action: "guild_shop", ItemID: &item}))
	require.ErrorIs(t, validateGachaGuildIntent(GachaActionIntent{Action: "guild_role", OpponentID: &member, ItemID: &invalid}), ErrGachaIntent)
	require.ErrorIs(t, validateGachaGuildIntent(GachaActionIntent{Action: "guild_shop", ItemID: &item, OpponentID: &member}), ErrGachaIntent)
	require.ErrorIs(t, validateGachaGuildIntent(GachaActionIntent{Action: "guild_attack", Count: &count}), ErrGachaIntent)
	require.NoError(t, validateGachaGuildIntent(GachaActionIntent{Action: "guild_kick", OpponentID: &member}))
	require.ErrorIs(t, validateGachaGuildIntent(GachaActionIntent{Action: "guild_kick", OpponentID: &member, ItemID: &role}), ErrGachaIntent)
	require.ErrorIs(t, validateGachaGuildIntent(GachaActionIntent{Action: "guild_kick"}), ErrGachaIntent)
}

func TestGuildGrowthAndRaidMigrationPreservesLevelAndUnfinishedProgress(t *testing.T) {
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, gameZone)
	for _, test := range []struct{ xp, mapped, level int }{{0, 0, 1}, {100, 1800, 2}, {175, 3900, 2}, {700, 28000, 5}, {3000, 260000, 10}} {
		_, owner := guildTestState(now)
		owner.GrowthVersion, owner.XP = 0, test.xp
		owner.RaidVersion, owner.BossHP, owner.BossMaxHP = 1, 12000, 24000
		assert.Equal(t, test.level, guildLevel(owner))
		require.True(t, validGachaGuildOwner(owner, 1, owner.Code))
		assert.Equal(t, 0, owner.GrowthVersion, "read-only checks do not mutate persisted state")
		require.True(t, migrateGuildOwner(owner))
		assert.Equal(t, test.mapped, owner.XP)
		assert.Equal(t, guildRaids[0].HP/2, owner.BossHP)
		assert.Equal(t, 2, owner.GrowthVersion)
		assert.Equal(t, guildRaidVersion, owner.RaidVersion)
		assert.Zero(t, owner.RaidCleared)
		require.True(t, migrateGuildOwner(owner))
		assert.Equal(t, guildRaids[0].HP/2, owner.BossHP)
	}
	_, owner := guildTestState(now)
	owner.RaidVersion, owner.BossHP, owner.BossMaxHP = 1, 1, 24000
	require.True(t, migrateGuildOwner(owner))
	assert.Equal(t, (guildRaids[0].HP+23999)/24000, owner.BossHP, "round toward life rather than a free clear")
	owner.RaidVersion, owner.RaidCleared, owner.BossHP, owner.BossMaxHP = 1, 7, 0, 120000
	require.True(t, migrateGuildOwner(owner))
	assert.Equal(t, 7, owner.RaidCleared)
	assert.Equal(t, guildRaids[3].HP, owner.BossHP)
	assert.Equal(t, 3, guildActiveStage(owner))
	for _, stage := range []int{1, 2} {
		_, migrated := guildTestState(now)
		oldHP := []int{24000, 60000, 120000}[stage]
		migrated.RaidVersion, migrated.RaidCleared = 1, (1<<stage)-1
		migrated.BossHP, migrated.BossMaxHP = oldHP/2, oldHP
		require.True(t, migrateGuildOwner(migrated))
		assert.Equal(t, guildRaids[stage].HP/2, migrated.BossHP)
		assert.Equal(t, (1<<stage)-1, migrated.RaidCleared)
	}
}

func TestGuildRaidParticipationAwardsSurviveAnUnfinishedBoss(t *testing.T) {
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, gameZone)
	state, owner := guildTestState(now)
	state.StaminaMax = 120
	state.StaminaTs = now.UnixMilli()
	state.Captain = &GachaCaptain{Level: 1}
	beforeGold, beforeStamina, beforeXP := state.Gold, state.Stamina, owner.XP
	result := &GachaActionResult{}
	require.NoError(t, applyGuildRaid(state, owner, 2, result, now))
	require.NotNil(t, result.GuildBattle)
	assert.LessOrEqual(t, result.GuildBattle.Report.Rounds, 20)
	assert.Equal(t, beforeGold, state.Gold)
	assert.Equal(t, beforeStamina-10, state.Stamina)
	assert.Equal(t, 50, state.GuildProgress.Coins)
	assert.Equal(t, 8, state.Mats["training_ink"])
	assert.Equal(t, beforeXP+60, owner.XP)
	assert.Equal(t, 1, state.GuildProgress.Attempts)
	assert.Equal(t, 1, state.GuildProgress.Participation[owner.Code])
	assert.Equal(t, 1, owner.Missions.Progress["raid"])
	assert.Equal(t, 1, state.GuildProgress.Activity.Contributions[owner.Code])
	assert.Equal(t, result.GuildBattle.Damage, owner.TotalDamage)
	assert.Equal(t, result.GuildBattle.Damage, owner.RaidDamage[1][2].Damage)
	assert.Equal(t, 1, owner.RaidDamage[1][2].Attempts)
}

func TestGuildRaidCapacityIsCheckedBeforeSpendingStamina(t *testing.T) {
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, gameZone)
	for _, balances := range [][2]int{{gachaMaxResource - 49, 0}, {0, gachaMaxResource - 7}} {
		state, owner := guildTestState(now)
		state.GuildProgress.Coins, state.Mats["training_ink"] = balances[0], balances[1]
		stamina, hp, xp := state.Stamina, owner.BossHP, owner.XP
		require.ErrorIs(t, applyGuildRaid(state, owner, 2, &GachaActionResult{}, now), ErrGachaCapacity)
		assert.Equal(t, stamina, state.Stamina)
		assert.Equal(t, hp, owner.BossHP)
		assert.Equal(t, xp, owner.XP)
		assert.Zero(t, state.GuildProgress.Attempts)
		assert.Equal(t, balances[0], state.GuildProgress.Coins)
		assert.Equal(t, balances[1], state.Mats["training_ink"])
	}
}

func TestGuildKickPermissionAndTargetMembershipAreCheckedBeforeMutation(t *testing.T) {
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, gameZone)
	state, owner := guildTestState(now)
	owner.Members = append(owner.Members, GachaGuildMember{UserID: 3, Role: "deputy"}, GachaGuildMember{UserID: 4, Role: "elder"}, GachaGuildMember{UserID: 5, Role: "deputy"})
	assert.True(t, guildCanKick(owner, 1, 3))
	assert.True(t, guildCanKick(owner, 3, 2))
	assert.True(t, guildCanKick(owner, 3, 4))
	assert.False(t, guildCanKick(owner, 3, 5))
	assert.False(t, guildCanKick(owner, 3, 1))
	assert.False(t, guildCanKick(owner, 1, 1))
	assert.False(t, guildCanKick(owner, 4, 2))
	assert.False(t, guildCanKick(owner, 1, 999))
	state.GuildProgress.Coins, state.GuildProgress.Claims, state.GuildProgress.Attempts = 777, 31, 2
	state.GuildProgress.Bought["manual"] = 5
	state.Guild.Code, state.Guild.LeaderID = "00000009", 9
	require.ErrorIs(t, applyGuildKick(owner, 3, 2, state), ErrGachaGuild)
	assert.Equal(t, 5, len(owner.Members))
	assert.Equal(t, "00000009", state.Guild.Code)
	state.Guild.Code, state.Guild.LeaderID = owner.Code, owner.LeaderID
	require.NoError(t, applyGuildKick(owner, 3, 2, state))
	assert.Nil(t, state.Guild)
	assert.Equal(t, 4, len(owner.Members))
	assert.Equal(t, 777, state.GuildProgress.Coins)
	assert.Equal(t, 31, state.GuildProgress.Claims)
	assert.Equal(t, 2, state.GuildProgress.Attempts)
	assert.Equal(t, 5, state.GuildProgress.Bought["manual"])
	require.ErrorIs(t, applyGuildKick(owner, 3, 2, state), ErrGachaGuild)
}

func TestGuildFiveStageRewardsAreDirectAndCapacityFailureIsAtomic(t *testing.T) {
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, gameZone)
	state, owner := guildTestState(now)
	owner.XP, owner.RaidCleared = 260000, 31
	owner.BossHP, owner.BossMaxHP = 0, guildRaids[4].HP
	state.GuildProgress.Participation[owner.Code] = 31
	state.GuildProgress.Claims = 7
	four, five := 4, 5
	state.Mats["weapon_core"] = gachaMaxResource
	require.ErrorIs(t, applyGuildClaim(state, owner, &four, &GachaActionResult{}, now), ErrGachaCapacity)
	assert.Equal(t, 7, state.GuildProgress.Claims)
	assert.Zero(t, state.GuildProgress.Coins)
	assert.Zero(t, state.Mats["training_ink"])
	state.Mats["weapon_core"] = 0
	require.NoError(t, applyGuildClaim(state, owner, &four, &GachaActionResult{}, now))
	require.NoError(t, applyGuildClaim(state, owner, &five, &GachaActionResult{}, now))
	assert.Equal(t, 31, state.GuildProgress.Claims)
	assert.Equal(t, 6000, state.GuildProgress.Coins)
	assert.Equal(t, 600, state.Mats["training_ink"])
	assert.Equal(t, 64, state.Mats["weapon_core"])
	assert.Equal(t, 24, state.Mats["chip_ticket"])
	require.ErrorIs(t, applyGuildClaim(state, owner, &five, &GachaActionResult{}, now), ErrGachaGuildLimit)
	assert.Equal(t, 6000, state.GuildProgress.Coins)
	require.NoError(t, syncGachaGuildProgress(state, now))
}

func TestGuildRaidRankingAggregatesSortsAndKeepsDepartedMembers(t *testing.T) {
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, gameZone)
	_, owner := guildTestState(now)
	owner.Members[0].Name, owner.Members[1].Name = "船长", "离会前昵称"
	owner.Members = append(owner.Members, GachaGuildMember{UserID: 3, Name: "同伤害队员"})
	require.NoError(t, recordGuildRaidContribution(owner, 1, 2, 150))
	require.NoError(t, recordGuildRaidContribution(owner, 1, 2, 250))
	require.NoError(t, recordGuildRaidContribution(owner, 1, 3, 400))
	require.NoError(t, recordGuildRaidContribution(owner, 1, 1, 50))
	view := guildRaidRanking(owner)
	require.Len(t, view.Entries, 3)
	assert.Equal(t, owner.Week, view.Week)
	assert.Equal(t, 1, view.Stage)
	assert.Equal(t, guildRaids[0].Name, view.StageName)
	assert.Equal(t, []int{2, 3, 1}, []int{view.Entries[0].UserID, view.Entries[1].UserID, view.Entries[2].UserID})
	assert.Equal(t, 400, view.Entries[0].Damage)
	assert.Equal(t, 2, view.Entries[0].Attempts)
	assert.Equal(t, 1, view.Entries[0].Rank)
	assert.True(t, view.Entries[0].Member)
	target := &GachaGameState{Guild: &GachaGuildState{Code: owner.Code, LeaderID: owner.LeaderID}}
	require.NoError(t, applyGuildKick(owner, 1, 2, target))
	view = guildRaidRanking(owner)
	assert.Equal(t, "离会前昵称", view.Entries[0].Name)
	assert.False(t, view.Entries[0].Member)
	assert.Equal(t, 400, view.Entries[0].Damage)
	owner.RaidCleared = 1
	assert.Equal(t, 2, guildRaidRanking(owner).Stage)
	assert.Empty(t, guildRaidRanking(owner).Entries, "next boss has its own ranking")
	assert.Equal(t, 400, owner.RaidDamage[1][2].Damage, "previous stage remains archived this week")
	owner.RaidCleared = 31
	require.NoError(t, recordGuildRaidContribution(owner, 5, 1, 900))
	view = guildRaidRanking(owner)
	assert.Equal(t, 5, view.Stage)
	assert.True(t, view.AllCleared)
	require.Len(t, view.Entries, 1)
	assert.Equal(t, 900, view.Entries[0].Damage)
}

func TestGuildRaidRankingResetsAtBeijingMondayAndPreservesResources(t *testing.T) {
	sunday := time.Date(2026, 11, 29, 23, 59, 59, 0, gameZone)
	state, owner := guildTestState(sunday)
	state.GuildProgress.Coins = 4321
	beforeXP, beforeGold := owner.XP, state.Gold
	require.NoError(t, recordGuildRaidContribution(owner, 1, 2, 900))
	resetGuildBoss(owner, sunday)
	require.Len(t, guildRaidRanking(owner).Entries, 1)
	monday := sunday.Add(time.Second)
	resetGuildBoss(owner, monday)
	require.NoError(t, syncGachaGuildProgress(state, monday))
	view := guildRaidRanking(owner)
	assert.Equal(t, gachaWeekKey(monday), view.Week)
	assert.Equal(t, 1, view.Stage)
	assert.Empty(t, owner.RaidDamage)
	assert.Empty(t, view.Entries)
	assert.Equal(t, guildRaids[0].HP, owner.BossHP)
	assert.Zero(t, owner.RaidCleared)
	assert.Equal(t, beforeXP, owner.XP)
	assert.Equal(t, beforeGold, state.Gold)
	assert.Equal(t, 4321, state.GuildProgress.Coins)
}

func TestGuildRaidRankingMigrationNeverInventsAttribution(t *testing.T) {
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, gameZone)
	_, owner := guildTestState(now)
	owner.RaidVersion, owner.BossHP, owner.BossMaxHP, owner.TotalDamage = 1, 12000, 24000, 12000
	require.True(t, migrateGuildOwner(owner))
	assert.Empty(t, guildRaidRanking(owner).Entries)
	assert.Equal(t, 12000, owner.TotalDamage)
	require.NoError(t, recordGuildRaidContribution(owner, 1, 2, 0))
	assert.Equal(t, 1, guildRaidRanking(owner).Entries[0].Attempts, "a zero-damage attempt is still counted")
	assert.Zero(t, guildRaidRanking(owner).Entries[0].Damage)
	assert.True(t, validGuildRaidContributions(owner))
	owner.RaidDamage[1][2] = GachaGuildRaidContribution{Name: "非法计数", Damage: 0, Attempts: 15}
	assert.False(t, validGuildRaidContributions(owner))
}

func TestGuildRaidVersionTwoMigratesRemainingLifeWithoutInflatingDamageOrClaims(t *testing.T) {
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, gameZone)
	oldHP := []int{300000, 900000, 2400000, 5000000, 9000000}
	for stage, hp := range oldHP {
		state, owner := guildTestState(now)
		owner.RaidVersion, owner.RaidCleared = 2, (1<<stage)-1
		owner.BossHP, owner.BossMaxHP = hp/2, hp
		owner.TotalDamage, owner.MyDamage = 12000, 7000
		owner.RaidDamage = map[int]map[int]GachaGuildRaidContribution{stage + 1: {2: {Name: "真实旧伤害", Damage: 7000, Attempts: 2}}}
		state.Guild = owner
		p := state.GuildProgress
		p.Claims, p.Coins, p.Attempts, p.Participation[owner.Code] = 7, 456, 2, 31
		p.Bought["ink"] = 2
		require.True(t, validGachaGuildOwner(owner, 1, owner.Code))
		assert.Equal(t, 2, owner.RaidVersion, "read-only owner checks do not migrate the saved object")
		require.NoError(t, syncGachaGuildProgress(state, now))
		assert.Equal(t, guildRaidVersion, owner.RaidVersion)
		assert.Equal(t, guildRaids[stage].HP/2, owner.BossHP)
		assert.Equal(t, guildRaids[stage].HP, owner.BossMaxHP)
		assert.Equal(t, (1<<stage)-1, owner.RaidCleared)
		assert.Equal(t, 12000, owner.TotalDamage)
		assert.Equal(t, 7000, owner.MyDamage)
		assert.Equal(t, GachaGuildRaidContribution{Name: "真实旧伤害", Damage: 7000, Attempts: 2}, owner.RaidDamage[stage+1][2])
		assert.Equal(t, 7, p.Claims)
		assert.Equal(t, 456, p.Coins)
		assert.Equal(t, 2, p.Attempts)
		assert.Equal(t, 31, p.Participation[owner.Code])
		assert.Equal(t, 2, p.Bought["ink"])
		require.True(t, migrateGuildOwner(owner))
		assert.Equal(t, guildRaids[stage].HP/2, owner.BossHP, "repeated normalization does not rescale HP")
	}
	_, owner := guildTestState(now)
	owner.RaidVersion, owner.RaidCleared, owner.BossHP, owner.BossMaxHP = 2, 31, 0, oldHP[4]
	owner.TotalDamage = 17600000
	owner.RaidDamage = map[int]map[int]GachaGuildRaidContribution{5: {2: {Name: "通关队员", Damage: 9000000, Attempts: 14}}}
	require.True(t, migrateGuildOwner(owner))
	require.True(t, validGachaGuildOwner(owner, 1, owner.Code))
	assert.Zero(t, owner.BossHP)
	assert.Equal(t, guildRaids[4].HP, owner.BossMaxHP)
	assert.Equal(t, 31, owner.RaidCleared)
	assert.Equal(t, 17600000, owner.TotalDamage)
	assert.Equal(t, 9000000, guildRaidRanking(owner).Entries[0].Damage)
}

func TestGuildRaidVersionTwoMigrationKeepsLiveBossAndRejectsInconsistentState(t *testing.T) {
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, gameZone)
	_, owner := guildTestState(now)
	owner.RaidVersion, owner.BossHP, owner.BossMaxHP = 2, 1, 300000
	require.True(t, migrateGuildOwner(owner))
	assert.Equal(t, (guildRaids[0].HP+299999)/300000, owner.BossHP)
	assert.Zero(t, owner.RaidCleared)
	for _, mutate := range []func(*GachaGuildState){
		func(g *GachaGuildState) { g.BossHP = 0 },
		func(g *GachaGuildState) { g.BossMaxHP = 24000 },
		func(g *GachaGuildState) { g.RaidCleared = 2 },
		func(g *GachaGuildState) { g.TotalDamage = 17600001 },
		func(g *GachaGuildState) { g.RaidVersion = guildRaidVersion + 1 },
	} {
		_, broken := guildTestState(now)
		broken.RaidVersion, broken.BossHP, broken.BossMaxHP = 2, 150000, 300000
		mutate(broken)
		before := *broken
		assert.False(t, migrateGuildOwner(broken))
		assert.Equal(t, before, *broken, "failed migration must not partially alter state")
	}
}

package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGuildDailyActivitiesRequirePlayAndPreserveClaimsAcrossGuilds(t *testing.T) {
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, gameZone)
	state, owner := guildTestState(now)
	xp, coins := owner.XP, state.GuildProgress.Coins
	id := "sortie"
	claim := GachaActionIntent{Action: "guild_activity_claim", ItemID: &id}
	receipt := &GachaActionResult{}
	require.ErrorIs(t, applyGuildActivity(state, owner, claim, receipt, now), ErrGachaGuildLimit)
	for i := 0; i < 4; i++ {
		require.NoError(t, recordGuildPersonalActivity(state, id, now))
	}
	assert.Equal(t, 3, state.GuildProgress.Activity.Progress[id])
	require.NoError(t, applyGuildActivity(state, owner, claim, receipt, now))
	assert.Equal(t, xp+60, owner.XP)
	assert.Equal(t, coins+45, state.GuildProgress.Coins)
	assert.Equal(t, 1, owner.Missions.Progress["active"])
	require.ErrorIs(t, applyGuildActivity(state, owner, claim, receipt, now), ErrGachaGuildLimit)
	state.Guild.Code = "00000002"
	other := *owner
	other.Code = state.Guild.Code
	require.ErrorIs(t, applyGuildActivity(state, &other, claim, receipt, now), ErrGachaGuildLimit)
	require.NoError(t, syncGuildActivity(state, now.AddDate(0, 0, 1)))
	assert.Empty(t, state.GuildProgress.Activity.Progress)
	assert.Empty(t, state.GuildProgress.Activity.Claimed)
	assert.Equal(t, coins+45, state.GuildProgress.Coins)
}

func TestGuildWeeklyMissionXPAwardsOnceAndMaterialClaimsAreAtomic(t *testing.T) {
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, gameZone)
	state, owner := guildTestState(now)
	before := owner.XP
	for i := 0; i < 21; i++ {
		require.NoError(t, recordGuildSharedActivity(state, owner, "supply", 1, now))
	}
	assert.Equal(t, before+800, owner.XP)
	assert.Equal(t, 20, owner.Missions.Progress["supply"])
	id := "supply"
	intent := GachaActionIntent{Action: "guild_mission_claim", ItemID: &id}
	state.Mats["training_ink"] = gachaMaxResource
	coins := state.GuildProgress.Coins
	require.ErrorIs(t, applyGuildActivity(state, owner, intent, &GachaActionResult{}, now), ErrGachaCapacity)
	assert.Zero(t, state.GuildProgress.Activity.MissionClaims)
	assert.Equal(t, coins, state.GuildProgress.Coins)
	state.Mats["training_ink"] -= 20
	require.NoError(t, applyGuildActivity(state, owner, intent, &GachaActionResult{}, now))
	assert.Equal(t, gachaMaxResource, state.Mats["training_ink"])
	assert.Equal(t, coins+150, state.GuildProgress.Coins)
	assert.Equal(t, before+800, owner.XP, "personal claims never repeat shared XP")
	require.ErrorIs(t, applyGuildActivity(state, owner, intent, &GachaActionResult{}, now), ErrGachaGuildLimit)
	state.Guild.Code = "00000002"
	other := *owner
	other.Code = state.Guild.Code
	require.NoError(t, recordGuildSharedActivity(state, &other, "checkin", 1, now))
	require.ErrorIs(t, applyGuildActivity(state, &other, intent, &GachaActionResult{}, now), ErrGachaGuildLimit)
	require.NoError(t, syncGuildActivity(state, now.AddDate(0, 0, 7)))
	assert.Zero(t, state.GuildProgress.Activity.MissionClaims)
	assert.Empty(t, state.GuildProgress.Activity.Contributions)
	assert.Equal(t, coins+150, state.GuildProgress.Coins)
}

func TestGuildActivityDoesNotCountOutsideMembershipAndRejectsForgedIntents(t *testing.T) {
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, gameZone)
	state, owner := guildTestState(now)
	state.Guild = nil
	require.NoError(t, recordGuildPersonalActivity(state, "sortie", now))
	assert.Nil(t, state.GuildProgress.Activity)
	state.Guild = &GachaGuildState{Code: owner.Code, LeaderID: owner.LeaderID}
	require.NoError(t, recordGuildTrainingAction(state, "squad", now))
	assert.Nil(t, state.GuildProgress.Activity)
	require.NoError(t, recordGuildTrainingAction(state, "gear_upgrade", now))
	assert.Equal(t, 1, state.GuildProgress.Activity.Progress["training"])
	count := 9
	require.ErrorIs(t, validateGuildActivityIntent(GachaActionIntent{Action: "guild_checkin", Count: &count}), ErrGachaIntent)
	id := "unknown"
	require.ErrorIs(t, validateGuildActivityIntent(GachaActionIntent{Action: "guild_activity_claim", ItemID: &id}), ErrGachaIntent)
	state.GuildProgress.Activity.Progress["sortie"] = 4
	require.ErrorIs(t, syncGuildActivity(state, now), ErrGachaRunState)
}

func TestGuildCheckinIsImmediateDoesNotAdvanceActiveMissionAndKeepsOldMail(t *testing.T) {
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, gameZone)
	state, owner := guildTestState(now)
	require.NoError(t, sendGachaMail(state, "old-gift", "旧礼包", "", map[string]int{"gold": 5}, 0, now))
	state.GuildProgress.Coins = gachaMaxResource
	intent := GachaActionIntent{Action: "guild_checkin"}
	require.ErrorIs(t, applyGuildActivity(state, owner, intent, &GachaActionResult{}, now), ErrGachaCapacity)
	assert.False(t, state.GuildProgress.Activity.Claimed["checkin"])
	state.GuildProgress.Coins -= 20
	require.NoError(t, applyGuildActivity(state, owner, intent, &GachaActionResult{}, now))
	assert.Equal(t, gachaMaxResource, state.GuildProgress.Coins)
	assert.Equal(t, 1, state.GuildProgress.Activity.Contributions[owner.Code])
	assert.Zero(t, owner.Missions.Progress["active"])
	require.Len(t, state.Mailbox.Messages, 1)
	assert.False(t, state.Mailbox.Messages[0].Claimed)
	require.ErrorIs(t, applyGuildActivity(state, owner, intent, &GachaActionResult{}, now), ErrGachaGuildLimit)
}

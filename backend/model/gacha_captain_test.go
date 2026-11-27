package model

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func captainTestIntent(day, action, id string) GachaActionIntent {
	tag := day + ":" + id
	return GachaActionIntent{Action: action, ItemID: &tag, RequestID: uuid.NewString()}
}

func TestGachaCaptainXPTracksActualStaminaDebitsAndIssuedRuns(t *testing.T) {
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, gameZone)
	state := newGachaState(now)
	require.NoError(t, syncGachaCaptain(state))
	var drops []string
	require.NoError(t, spendGachaStamina(state, 0, now, &drops))
	require.ErrorIs(t, spendGachaStamina(state, -1, now, &drops), ErrGachaRunState)
	require.ErrorIs(t, spendGachaStamina(state, state.Stamina+1, now, &drops), ErrGachaFunds)
	assert.Equal(t, 120, state.Stamina)
	assert.Empty(t, drops)
	assert.Equal(t, GachaCaptain{Level: 1}, *state.Captain)
	state.Captain.XP = gachaCaptainCost(1) - 1
	stage := &GachaStage{Cost: 6, Exp: 2000}
	snapshot := GachaRunSnapshot{RulesVersion: gachaStaminaXPVersion}
	require.NoError(t, spendGachaStamina(state, stage.Cost, now, &snapshot.StaminaDrops))
	assert.Equal(t, 114, state.Stamina)
	assert.Equal(t, GachaCaptain{Level: 2, XP: 5}, *state.Captain)
	regenGachaState(state, now.Add(time.Hour))
	assert.Equal(t, 120, state.Stamina)
	for _, won := range []bool{true, false} {
		result := &GachaFinishResult{Win: won}
		settleGachaCaptainRun(state, stage, &snapshot, result)
		assert.Equal(t, snapshot.StaminaDrops, result.Drops)
		assert.Equal(t, GachaCaptain{Level: 2, XP: 5}, *state.Captain)
	}
	for _, won := range []bool{true, false} {
		legacy := newGachaState(now)
		legacy.Stamina -= stage.Cost
		result := &GachaFinishResult{Win: won}
		settleGachaCaptainRun(legacy, stage, &GachaRunSnapshot{RulesVersion: gachaCaptainLootVersion}, result)
		assert.Equal(t, GachaCaptain{Level: 1, XP: stage.Cost}, *legacy.Captain)
		assert.Equal(t, 114, legacy.Stamina)
		assert.Equal(t, []string{"船长经验 +6"}, result.Drops)
	}
	for _, version := range []int{gachaExperienceLootVersion, gachaCaptainLootVersion, gachaStaminaXPVersion} {
		result := &GachaFinishResult{Win: true}
		settleGachaCaptainRun(state, &GachaStage{Exp: 2000}, &GachaRunSnapshot{Mode: "training", RulesVersion: version}, result)
		assert.Empty(t, result.Drops)
		assert.Equal(t, GachaCaptain{Level: 2, XP: 5}, *state.Captain)
	}
}

func TestGachaCaptainMigrationAndExperienceCeilings(t *testing.T) {
	state := battleTestState([]string{"c01", "c08"}, 12, 1)
	state.Girls["c01"].PeakLevel = 36
	require.NoError(t, syncGachaCaptain(state))
	assert.Equal(t, 36, gachaCaptainLevel(state))
	state.Captain.XP = 10
	require.NoError(t, syncGachaCaptain(state))
	assert.Equal(t, 10, state.Captain.XP)
	partner := state.Girls["c08"]
	partner.Level, partner.Exp = 36, 73
	var drops []string
	require.NoError(t, awardGachaExperience(state, []string{"c08"}, 10_000, &drops))
	assert.Equal(t, 73, partner.Exp)
	state.Mats["training_manual"] = 8
	partnerID := "c08"
	feed := GachaActionIntent{Action: "card_feed", GirlID: &partnerID, Materials: map[string]int{"training_manual": 8}}
	require.ErrorIs(t, applyGachaFeed(state, feed, &GachaActionResult{}), ErrGachaCultivationLocked)
	require.ErrorIs(t, applyGachaCultivationAction(state, GachaActionIntent{Action: "use_manual", GirlID: &partnerID}, &GachaActionResult{}), ErrGachaCultivationLocked)
	assert.Equal(t, 8, state.Mats["training_manual"])
	state.Captain.XP = gachaCaptainCost(36) - 1
	receipt := &GachaActionResult{}
	grantGachaCaptainXP(state, 1, &receipt.Drops)
	assert.Equal(t, GachaCaptain{Level: 37}, *state.Captain)
	partner.Exp = gachaLevelCost(36) - 301
	require.NoError(t, applyGachaFeed(state, feed, receipt))
	assert.Equal(t, 301, receipt.Feed.Experience)
	assert.Equal(t, map[string]int{"training_manual": 1}, receipt.Feed.Spent)
	assert.Equal(t, 7, state.Mats["training_manual"])
	assert.Equal(t, 37, partner.Level)
	assert.Zero(t, partner.Exp)
	state.Captain.Level, state.Captain.XP = 149, gachaCaptainCost(149)-5
	grantGachaCaptainXP(state, 300, &receipt.Drops)
	assert.Equal(t, GachaCaptain{Level: 150}, *state.Captain)
	assert.Contains(t, receipt.Drops, "船长经验 +5")
	state.Captain.Level = 2
	require.ErrorIs(t, syncGachaCaptain(state), ErrGachaRunState)
}

func TestGachaCaptainDailyTasksRequireConfirmedPlay(t *testing.T) {
	now := time.Date(2026, 11, 27, 11, 0, 0, 0, time.UTC)
	state := newGachaState(now)
	require.NoError(t, syncGachaCaptain(state))
	intent := captainTestIntent(gachaDay(now), "captain_task", "voyage")
	receipt := &GachaActionResult{}
	require.NoError(t, ValidateGachaAction(intent))
	require.ErrorIs(t, applyGachaCaptainClaim(state, intent, receipt, now), ErrGachaCaptainClaim)
	for _, sample := range []struct {
		mode string
		cost int
		win  bool
	}{{"", 2, false}, {"trial_c01_1", 0, true}, {"", 2, true}, {"training", 0, true}, {"", 2, true}, {"", 2, true}} {
		settleGachaCaptainRun(state, &GachaStage{Cost: sample.cost, Exp: 2000}, &GachaRunSnapshot{Mode: sample.mode, RulesVersion: gachaStaminaXPVersion}, &GachaFinishResult{Win: sample.win})
	}
	previous := *state.Captain
	settleGachaCaptainRun(state, &GachaStage{Cost: 30}, &GachaRunSnapshot{RulesVersion: gachaExperienceLootVersion}, &GachaFinishResult{Win: true})
	require.Equal(t, previous, *state.Captain)
	log := state.Daily.CaptainLog
	assert.Equal(t, map[string]int{"rollcall": 1, "voyage": 3, "drill": 1}, log.Progress)
	gold := state.Gold
	require.NoError(t, applyGachaCaptainClaim(state, intent, receipt, now))
	assert.Equal(t, gold+gachaCaptainTask("voyage").Gold, state.Gold)
	assert.True(t, log.Claimed["voyage"])
	assert.False(t, log.Claimed["rollcall"], "claiming one task must not claim another")
	require.ErrorIs(t, applyGachaCaptainClaim(state, intent, receipt, now), ErrGachaCaptainClaim)
	for _, action := range []string{"squad", "wish", "gear_upgrade", "card_feed"} {
		recordGachaCaptainAction(state, action)
	}
	assert.Equal(t, 1, log.Progress["wish"])
	assert.Equal(t, 1, log.Progress["outfit"])
	next := now.AddDate(0, 0, 1)
	regenGachaState(state, next)
	require.NoError(t, syncGachaCaptain(state))
	assert.Equal(t, map[string]int{"rollcall": 1}, state.Daily.CaptainLog.Progress)
	require.ErrorIs(t, applyGachaCaptainClaim(state, intent, receipt, next), ErrGachaCaptainClaim)
	state.Gold = gachaMaxResource
	rollcall := captainTestIntent(gachaDay(next), "captain_task", "rollcall")
	require.ErrorIs(t, applyGachaCaptainClaim(state, rollcall, receipt, next), ErrGachaCapacity)
	assert.False(t, state.Daily.CaptainLog.Claimed["rollcall"])
	state.Gold -= gachaCaptainTask("rollcall").Gold
	require.NoError(t, applyGachaCaptainClaim(state, rollcall, receipt, next))
	assert.True(t, state.Daily.CaptainLog.Claimed["rollcall"])
	assert.Equal(t, gachaMaxResource, state.Gold)
	for _, bad := range []GachaActionIntent{captainTestIntent("2026-02-30", "captain_task", "rollcall"), captainTestIntent(gachaDay(now), "captain_supply", "rollcall"), captainTestIntent(gachaDay(now), "captain_task", "extra")} {
		require.ErrorIs(t, ValidateGachaAction(bad), ErrGachaIntent)
	}
	forged := 10
	intent.Count = &forged
	require.ErrorIs(t, ValidateGachaAction(intent), ErrGachaIntent)
}

func TestGachaCaptainBeijingMealBoundariesAndOverflow(t *testing.T) {
	base := time.Date(2026, 11, 27, 0, 0, 0, 0, gameZone).UTC()
	for _, check := range []struct {
		id     string
		minute int
		open   bool
	}{{"morning", 539, false}, {"morning", 540, true}, {"morning", 719, true}, {"morning", 720, false}, {"noon", 719, false}, {"noon", 720, true}, {"noon", 1259, true}, {"noon", 1260, false}, {"night", 1259, false}, {"night", 1260, true}, {"night", 1439, true}, {"night", 1440, false}} {
		now := base.Add(time.Duration(check.minute) * time.Minute)
		state := newGachaState(now)
		require.NoError(t, syncGachaCaptain(state))
		intent := captainTestIntent(gachaDay(base), "captain_supply", check.id)
		require.NoError(t, ValidateGachaAction(intent))
		err := applyGachaCaptainClaim(state, intent, &GachaActionResult{}, now)
		if !check.open {
			require.ErrorIs(t, err, ErrGachaCaptainClaim)
			assert.Equal(t, 120, state.Stamina)
			continue
		}
		require.NoError(t, err)
		assert.Equal(t, 150, state.Stamina)
		require.ErrorIs(t, applyGachaCaptainClaim(state, intent, &GachaActionResult{}, now), ErrGachaCaptainClaim)
		state.Token = 100
		require.ErrorIs(t, buyGachaShopItem(state, "stamina"), ErrGachaCapacity)
		assert.Equal(t, 100, state.Token)
		regenGachaState(state, now.Add(8*time.Hour))
		assert.Equal(t, 150, state.Stamina)
	}
}

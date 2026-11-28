package model_test

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sweepCommand(id, day string, count int) model.GachaActionIntent {
	return model.GachaActionIntent{RequestID: uuid.NewString(), Action: "voyage_sweep", ItemID: &id, Count: &count, Choice: &day}
}

func requireGachaTestMail(t *testing.T, state *model.GachaGameState, id string) *model.GachaMailMessage {
	t.Helper()
	require.NotNil(t, state.Mailbox)
	var found []*model.GachaMailMessage
	for _, message := range state.Mailbox.Messages {
		if message.ID == id {
			found = append(found, message)
		}
	}
	require.Len(t, found, 1)
	return found[0]
}

func claimGachaTestMail(t *testing.T, userID int, message *model.GachaMailMessage, now time.Time) *model.GachaGameState {
	t.Helper()
	require.False(t, message.Claimed)
	claim := model.GachaActionIntent{Action: "mail_claim", RequestID: uuid.NewString(), ItemID: &message.ID}
	receipt, err := model.ApplyGachaAction(userID, claim, now)
	require.NoError(t, err)
	require.True(t, requireGachaTestMail(t, receipt.Save, message.ID).Claimed)
	return receipt.Save
}

func TestGachaVoyageSweepIntent(t *testing.T) {
	day := "2026-11-27"
	for _, point := range []string{"s1", "v1h1", "res_manifest"} {
		for _, count := range []int{1, 3} {
			require.NoError(t, model.ValidateGachaAction(sweepCommand(point, day, count)))
		}
	}
	for _, intent := range []model.GachaActionIntent{sweepCommand("v1n1", day, 1), sweepCommand("r1", day, 1), sweepCommand("n1", day, 1), sweepCommand("weekly_boss", day, 1), sweepCommand("s1", "2026-11-31", 1), sweepCommand("s1", day, 0), sweepCommand("s1", day, 2), sweepCommand("s1", day, 4)} {
		require.ErrorIs(t, model.ValidateGachaAction(intent), model.ErrGachaIntent)
	}
	intent := sweepCommand("s1", day, 1)
	intent.Choice = nil
	require.ErrorIs(t, model.ValidateGachaAction(intent), model.ErrGachaIntent)
	intent = sweepCommand("s1", day, 3)
	intent.Materials = map[string]int{"gold": 100}
	require.ErrorIs(t, model.ValidateGachaAction(intent), model.ErrGachaIntent)
}

func testGachaVoyageSweepDatabase(t *testing.T, clock time.Time) {
	zone := time.FixedZone("Beijing", 8*3600)
	local := clock.In(zone)
	now := time.Date(local.Year(), local.Month(), local.Day(), 23, 56, 0, 0, zone)
	day := now.Format("2006-01-02")
	user := gachaUser(t)
	const stage = "res_manifest"
	profile, err := model.GetGachaProfile(user.Id, now)
	require.NoError(t, err)
	point := profile.Voyage.Stages[stage]
	require.NotNil(t, point)
	assert.Equal(t, point.Prize, point.SweepPrize)
	assert.Empty(t, profile.Voyage.Stages["v1n1"].SweepPrize)
	assert.Equal(t, []model.GachaVoyagePrize{{Item: "forge_ore", Amount: 2}, {Item: "training_ink", Amount: 1}}, profile.Voyage.Stages["s1"].SweepPrize)
	state := profile.Save
	state.Captain.Level = 5
	state.Cleared[stage], state.Stars[stage], state.MultiOn = true, 3, true
	seedGachaState(t, user.Id, state)
	_, err = model.ApplyGachaAction(user.Id, sweepCommand(stage, day, 1), now)
	require.ErrorIs(t, err, model.ErrGachaLocked)
	state.Cleared["s1"], state.Stars[stage] = true, 2
	seedGachaState(t, user.Id, state)
	_, err = model.ApplyGachaAction(user.Id, sweepCommand(stage, day, 1), now)
	require.ErrorIs(t, err, model.ErrGachaSweepStars)
	state.Stars[stage] = 3
	seedGachaState(t, user.Id, state)
	run, err := model.StartGachaRun(user.Id, stage, uuid.NewString(), now)
	require.NoError(t, err)
	assert.Equal(t, state.Captain.XP+point.Cost, run.Save.Captain.XP)
	_, err = model.ApplyGachaAction(user.Id, sweepCommand(stage, day, 1), now)
	require.ErrorIs(t, err, model.ErrGachaRunState)
	abandoned, err := model.FinishGachaRun(user.Id, run.ID, true, now)
	require.NoError(t, err)
	assert.Equal(t, run.Save.Captain, abandoned.Save.Captain)
	_, err = model.ApplyGachaAction(user.Id, sweepCommand(stage, day, 3), now)
	require.ErrorIs(t, err, model.ErrGachaVoyageLimit)
	one := sweepCommand(stage, day, 1)
	first, err := model.ApplyGachaAction(user.Id, one, now)
	require.NoError(t, err)
	for _, prize := range point.Prize {
		assert.Equal(t, abandoned.Save.Mats[prize.Item]+prize.Amount, first.Save.Mats[prize.Item])
	}
	assert.Equal(t, abandoned.Save.Stamina-point.Cost, first.Save.Stamina)
	assert.Equal(t, 2, first.Save.Daily.VoyageAttempts[stage])
	assert.Equal(t, abandoned.Save.Captain.XP+point.Cost, first.Save.Captain.XP)
	assert.Equal(t, 1, first.Save.Daily.CaptainLog.Progress["voyage"])
	assert.Zero(t, first.Save.Daily.CaptainLog.Progress["drill"])
	assert.Equal(t, abandoned.Save.Girls, first.Save.Girls)
	assert.Equal(t, abandoned.Save.Multi, first.Save.Multi)
	assert.Equal(t, abandoned.Save.Token, first.Save.Token)
	assert.Equal(t, abandoned.Save.Stars, first.Save.Stars)
	replay, err := model.ApplyGachaAction(user.Id, one, now)
	require.NoError(t, err)
	assert.Equal(t, first.Revision, replay.Revision)
	assert.Equal(t, first.Save, replay.Save)
	assert.Nil(t, ladderPublicMail(first.Save, "action:"+one.RequestID), "a completed sweep is received directly")
	last, err := model.ApplyGachaAction(user.Id, sweepCommand(stage, day, 1), now)
	require.NoError(t, err)
	assert.Equal(t, 3, last.Save.Daily.VoyageAttempts[stage])
	_, err = model.StartGachaRun(user.Id, stage, uuid.NewString(), now)
	require.ErrorIs(t, err, model.ErrGachaVoyageLimit)
	var runs int64
	require.NoError(t, model.DB.Model(&model.GachaRun{}).Where("user_id = ?", user.Id).Count(&runs).Error)
	assert.EqualValues(t, 1, runs, "only the actual battle generated a report")
	var viewer model.User
	require.NoError(t, model.DB.Select("quota").First(&viewer, user.Id).Error)
	assert.Equal(t, user.Quota, viewer.Quota)

	tomorrow := now.Add(4 * time.Minute)
	nextDay := tomorrow.Format("2006-01-02")
	_, err = model.ApplyGachaAction(user.Id, sweepCommand(stage, day, 3), tomorrow)
	require.ErrorIs(t, err, model.ErrGachaSweepDate)
	unspent := gachaState(t, user.Id, tomorrow)
	assert.Zero(t, unspent.Daily.VoyageAttempts[stage])
	unspent.Mats[point.Prize[1].Item], unspent.Stamina = 1_000_000_000, 1
	seedGachaState(t, user.Id, unspent)
	_, err = model.ApplyGachaAction(user.Id, sweepCommand(stage, nextDay, 3), tomorrow)
	require.ErrorIs(t, err, model.ErrGachaFunds)
	unspent.Stamina = 120
	unspent.Activity = model.GachaRunActivity{Starts: 10, WindowStartedAt: tomorrow.UnixMilli()}
	seedGachaState(t, user.Id, unspent)
	_, err = model.ApplyGachaAction(user.Id, sweepCommand(stage, nextDay, 3), tomorrow)
	require.ErrorIs(t, err, model.ErrGachaSweepBurst)
	unchanged := gachaState(t, user.Id, tomorrow)
	assert.Equal(t, unspent.Activity, unchanged.Activity)
	assert.Equal(t, unspent.Stamina, unchanged.Stamina)
	assert.Equal(t, unspent.Captain, unchanged.Captain)
	unchanged.Activity.Starts = 9
	seedGachaState(t, user.Id, unchanged)
	batchCommand := sweepCommand(stage, nextDay, 3)
	batch, err := model.ApplyGachaAction(user.Id, batchCommand, tomorrow)
	require.ErrorIs(t, err, model.ErrGachaCapacity)
	rolledBack := gachaState(t, user.Id, tomorrow)
	assert.Equal(t, unchanged.Stamina, rolledBack.Stamina)
	assert.Equal(t, unchanged.Captain, rolledBack.Captain)
	assert.Equal(t, unchanged.Daily.VoyageAttempts, rolledBack.Daily.VoyageAttempts)
	assert.Equal(t, unchanged.Activity, rolledBack.Activity)
	rolledBack.Mats[point.Prize[1].Item] -= 3 * point.Prize[1].Amount
	seedGachaState(t, user.Id, rolledBack)
	batch, err = model.ApplyGachaAction(user.Id, batchCommand, tomorrow)
	require.NoError(t, err)
	assert.Equal(t, 3, batch.Save.Daily.VoyageAttempts[stage])
	assert.Equal(t, unchanged.Stamina-3*point.Cost, batch.Save.Stamina)
	assert.Equal(t, unchanged.Captain.XP+3*point.Cost, batch.Save.Captain.XP)
	assert.Equal(t, 1_000_000_000, batch.Save.Mats[point.Prize[1].Item])
	batchAgain, err := model.ApplyGachaAction(user.Id, batchCommand, tomorrow)
	require.NoError(t, err)
	assert.Equal(t, batch, batchAgain, "retry cannot charge stamina or award materials twice")
	assert.Greater(t, batch.Save.Activity.RestUntil, tomorrow.UnixMilli())
	batch.Save.Stars["s1"] = 3
	seedGachaState(t, user.Id, batch.Save)
	_, err = model.ApplyGachaAction(user.Id, sweepCommand("s1", nextDay, 1), tomorrow)
	require.ErrorIs(t, err, model.ErrGachaRest)
}

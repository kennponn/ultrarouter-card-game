package model_test

import (
	"slices"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testGachaMailDatabase(t *testing.T, _ time.Time) {
	const maxResource = 1_000_000_000
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, time.FixedZone("Beijing", 8*3600))
	command := func(action, item string) model.GachaActionIntent {
		intent := model.GachaActionIntent{Action: action, RequestID: uuid.NewString()}
		if item != "" {
			intent.ItemID = &item
		}
		return intent
	}
	t.Run("gift_ownership_replay_and_deletion", func(t *testing.T) {
		owner, other := gachaUser(t), gachaUser(t)
		before, otherBefore := gachaState(t, owner.Id, now), gachaState(t, other.Id, now)
		redeem := command("redeem_code", " starfrost2026 ")
		before.Gold = maxResource - 299
		seedGachaState(t, owner.Id, before)
		_, err := model.ApplyGachaAction(owner.Id, redeem, now)
		require.ErrorIs(t, err, model.ErrGachaCapacity)
		unpaid := gachaState(t, owner.Id, now)
		assert.Equal(t, before.Gold, unpaid.Gold)
		assert.Equal(t, before.Stamina, unpaid.Stamina)
		assert.Equal(t, before.Mats, unpaid.Mats)
		assert.False(t, unpaid.Mailbox.Redeemed["welcome-2026"])
		assert.Empty(t, unpaid.Mailbox.Messages)
		before.Gold = 600
		seedGachaState(t, owner.Id, before)
		sent, err := model.ApplyGachaAction(owner.Id, redeem, now)
		require.NoError(t, err)
		assert.Empty(t, sent.Save.Mailbox.Messages)
		assert.True(t, sent.Save.Mailbox.Redeemed["welcome-2026"])
		assert.Equal(t, before.Gold+300, sent.Save.Gold)
		assert.Equal(t, before.Stamina+30, sent.Save.Stamina)
		assert.Equal(t, before.Mats["field_notes"]+2, sent.Save.Mats["field_notes"])
		assert.Equal(t, before.Captain, sent.Save.Captain)
		again, err := model.ApplyGachaAction(owner.Id, redeem, now)
		require.NoError(t, err)
		assert.Equal(t, sent, again)
		_, err = model.ApplyGachaAction(owner.Id, command("redeem_code", "STARFROST2026"), now)
		require.ErrorIs(t, err, model.ErrGachaRedeemCode)
		// Simulate a pre-upgrade unclaimed gift: its entitlement is already marked
		// redeemed but the attachments have not entered inventory yet.
		legacy := gachaState(t, owner.Id, now)
		gift := &model.GachaMailMessage{ID: "gift:legacy-test", Title: "旧版欢迎礼包", Sender: "星霜港航务署", CreatedAt: now.UnixMilli(), Resources: map[string]int{"gold": 300, "stamina": 30, "field_notes": 2}}
		legacy.Mailbox.Messages = append(legacy.Mailbox.Messages, gift)
		seedGachaState(t, owner.Id, legacy)
		claim := command("mail_claim", gift.ID)
		_, err = model.ApplyGachaAction(other.Id, claim, now)
		require.ErrorIs(t, err, model.ErrGachaMailUnavailable)
		untouched := gachaState(t, other.Id, now)
		assert.Equal(t, otherBefore.Mailbox, untouched.Mailbox)
		assert.Equal(t, otherBefore.Gold, untouched.Gold)
		assert.False(t, requireGachaTestMail(t, gachaState(t, owner.Id, now), gift.ID).Claimed)
		paid, err := model.ApplyGachaAction(owner.Id, claim, now)
		require.NoError(t, err)
		assert.True(t, requireGachaTestMail(t, paid.Save, gift.ID).Claimed)
		assert.Equal(t, legacy.Gold+300, paid.Save.Gold)
		assert.Equal(t, legacy.Stamina+30, paid.Save.Stamina)
		assert.Equal(t, legacy.Mats["field_notes"]+2, paid.Save.Mats["field_notes"])
		assert.Equal(t, before.Captain, paid.Save.Captain)
		again, err = model.ApplyGachaAction(owner.Id, claim, now)
		require.NoError(t, err)
		assert.Equal(t, paid, again)
		_, err = model.ApplyGachaAction(owner.Id, command("mail_claim", gift.ID), now)
		require.ErrorIs(t, err, model.ErrGachaMailUnavailable)
		cleaned, err := model.ApplyGachaAction(owner.Id, command("mail_delete_read", ""), now)
		require.NoError(t, err)
		assert.False(t, slices.ContainsFunc(cleaned.Save.Mailbox.Messages, func(message *model.GachaMailMessage) bool { return message.ID == gift.ID }))
		assert.True(t, cleaned.Save.Mailbox.Redeemed["welcome-2026"])
		_, err = model.ApplyGachaAction(owner.Id, command("redeem_code", "STARFROST2026"), now)
		require.ErrorIs(t, err, model.ErrGachaRedeemCode)
		again, err = model.ApplyGachaAction(owner.Id, redeem, now)
		require.NoError(t, err)
		assert.Equal(t, sent, again)
		current := gachaState(t, owner.Id, now)
		assert.Equal(t, cleaned.Save.Mailbox, current.Mailbox)
		assert.Equal(t, paid.Save.Gold, current.Gold)
		assert.Equal(t, paid.Save.Stamina, current.Stamina)
		assert.Equal(t, paid.Save.Mats, current.Mats)
		independent, err := model.ApplyGachaAction(other.Id, redeem, now)
		require.NoError(t, err)
		assert.Equal(t, otherBefore.Gold+300, independent.Save.Gold)
		assert.Equal(t, otherBefore.Stamina+30, independent.Save.Stamina)
		assert.Empty(t, independent.Save.Mailbox.Messages)
	})
	t.Run("claim_all_capacity_rollback_and_retry", func(t *testing.T) {
		owner := gachaUser(t)
		before := gachaState(t, owner.Id, now)
		// A pre-upgrade automatic daily attachment remains claimable; new daily
		// tasks are collected immediately through captain_task instead.
		before.Mailbox.Messages = append(before.Mailbox.Messages, &model.GachaMailMessage{ID: "daily:2026-11-27:rollcall", Title: "旧版港口点名", Sender: "星霜港航务署", CreatedAt: now.UnixMilli(), Resources: map[string]int{"gold": 50}, CaptainXP: 40})
		before.Daily.CaptainLog.Claimed["rollcall"] = true
		seedGachaState(t, owner.Id, before)
		rollcall := requireGachaTestMail(t, before, "daily:2026-11-27:rollcall")
		require.Positive(t, rollcall.CaptainXP)
		before.Mailbox.Messages = append(before.Mailbox.Messages,
			&model.GachaMailMessage{ID: "gift:welcome-2026", Title: "旧版欢迎礼包", Sender: "测试", CreatedAt: now.UnixMilli(), Resources: map[string]int{"gold": 300, "stamina": 30, "field_notes": 2}},
			&model.GachaMailMessage{ID: "gift:voyage-2026", Title: "旧版探索礼包", Sender: "测试", CreatedAt: now.UnixMilli(), Resources: map[string]int{"gold": 100, "training_ink": 2, "gear_rivet": 5}})
		before.Mailbox.Redeemed["welcome-2026"], before.Mailbox.Redeemed["voyage-2026"] = true, true
		seedGachaState(t, owner.Id, before)
		state := gachaState(t, owner.Id, now)
		require.Len(t, state.Mailbox.Messages, 3)
		gold := 400 + rollcall.Resources["gold"]
		state.Gold = 1_000_000_000 - gold + 1
		seedGachaState(t, owner.Id, state)
		all := command("mail_all", "")
		_, err := model.ApplyGachaAction(owner.Id, all, now)
		require.ErrorIs(t, err, model.ErrGachaCapacity)
		unpaid := gachaState(t, owner.Id, now)
		assert.Equal(t, state.Gold, unpaid.Gold)
		assert.Equal(t, state.Stamina, unpaid.Stamina)
		assert.Equal(t, state.Mats, unpaid.Mats)
		assert.Equal(t, state.Captain, unpaid.Captain)
		assert.Equal(t, state.Mailbox, unpaid.Mailbox)
		unpaid.Gold--
		seedGachaState(t, owner.Id, unpaid)
		paid, err := model.ApplyGachaAction(owner.Id, all, now)
		require.NoError(t, err)
		assert.Equal(t, 1_000_000_000, paid.Save.Gold)
		assert.Equal(t, before.Stamina+30, paid.Save.Stamina)
		for id, amount := range map[string]int{"field_notes": 2, "training_ink": 2, "gear_rivet": 5} {
			assert.Equal(t, before.Mats[id]+amount, paid.Save.Mats[id], id)
		}
		assert.Equal(t, &model.GachaCaptain{Level: 2, XP: 10}, paid.Save.Captain)
		for _, message := range paid.Save.Mailbox.Messages {
			assert.True(t, message.Claimed)
			assert.True(t, message.Read)
		}
		again, err := model.ApplyGachaAction(owner.Id, all, now)
		require.NoError(t, err)
		assert.Equal(t, paid, again)
		_, err = model.ApplyGachaAction(owner.Id, command("mail_all", ""), now)
		require.ErrorIs(t, err, model.ErrGachaMailUnavailable)
	})
	t.Run("star_chest_capacity_rolls_back_claim_until_resources_fit", func(t *testing.T) {
		owner := gachaUser(t)
		state := gachaState(t, owner.Id, now)
		state.Gold, state.Mats["training_manual"] = maxResource, maxResource
		state.Cleared["s1"], state.Cleared["s5"], state.Cleared["s2"] = true, true, true
		state.Stars["s1"], state.Stars["s5"], state.Stars["s2"] = 3, 3, 2
		seedGachaState(t, owner.Id, state)
		const chestID = "c1-story-1"
		open := command("voyage_chest", chestID)
		_, err := model.ApplyGachaAction(owner.Id, open, now)
		require.ErrorIs(t, err, model.ErrGachaCapacity)
		unpaid := gachaState(t, owner.Id, now)
		assert.False(t, unpaid.VoyageClaims[chestID])
		assert.Equal(t, state.Gold, unpaid.Gold)
		assert.Equal(t, state.Mats, unpaid.Mats)
		unpaid.Gold -= 150
		seedGachaState(t, owner.Id, unpaid)
		_, err = model.ApplyGachaAction(owner.Id, open, now)
		require.ErrorIs(t, err, model.ErrGachaCapacity)
		unpaid = gachaState(t, owner.Id, now)
		assert.False(t, unpaid.VoyageClaims[chestID])
		assert.Equal(t, maxResource-150, unpaid.Gold)
		assert.Equal(t, maxResource, unpaid.Mats["training_manual"])
		unpaid.Mats["training_manual"]--
		seedGachaState(t, owner.Id, unpaid)
		paid, err := model.ApplyGachaAction(owner.Id, open, now)
		require.NoError(t, err)
		assert.True(t, paid.Save.VoyageClaims[chestID])
		assert.Equal(t, maxResource, paid.Save.Gold)
		assert.Equal(t, maxResource, paid.Save.Mats["training_manual"])
		again, err := model.ApplyGachaAction(owner.Id, open, now)
		require.NoError(t, err)
		assert.Equal(t, paid, again)
		_, err = model.ApplyGachaAction(owner.Id, command("voyage_chest", chestID), now)
		require.ErrorIs(t, err, model.ErrGachaVoyageChest)
	})
	t.Run("sweep_capacity_failure_preserves_stamina_and_daily_attempts", func(t *testing.T) {
		const stage = "res_manifest"
		owner := gachaUser(t)
		state := gachaState(t, owner.Id, now)
		state.Cleared["s1"], state.Cleared[stage], state.Stars[stage] = true, true, 3
		state.Captain.Level, state.Mats["field_notes"] = 5, maxResource
		seedGachaState(t, owner.Id, state)
		sweep := sweepCommand(stage, "2026-11-27", 3)
		sent, err := model.ApplyGachaAction(owner.Id, sweep, now)
		require.ErrorIs(t, err, model.ErrGachaCapacity)
		unpaid := gachaState(t, owner.Id, now)
		assert.Equal(t, state.Stamina, unpaid.Stamina)
		assert.Equal(t, state.Captain, unpaid.Captain)
		assert.Equal(t, state.Mats, unpaid.Mats)
		assert.Equal(t, state.Daily.VoyageAttempts, unpaid.Daily.VoyageAttempts)
		unpaid.Mats["field_notes"] -= 6
		seedGachaState(t, owner.Id, unpaid)
		sent, err = model.ApplyGachaAction(owner.Id, sweep, now)
		require.NoError(t, err)
		assert.Equal(t, state.Stamina-18, sent.Save.Stamina)
		assert.Equal(t, state.Captain.XP+18, sent.Save.Captain.XP)
		assert.Equal(t, 3, sent.Save.Daily.VoyageAttempts[stage])
		assert.Equal(t, maxResource, sent.Save.Mats["field_notes"])
		assert.Equal(t, state.Mats["training_manual"]+3, sent.Save.Mats["training_manual"])
		again, err := model.ApplyGachaAction(owner.Id, sweep, now)
		require.NoError(t, err)
		assert.Equal(t, sent, again)
		unpaid = gachaState(t, owner.Id, now)
		assert.Equal(t, sent.Save.Stamina, unpaid.Stamina)
		assert.Equal(t, sent.Save.Captain, unpaid.Captain)
		assert.Equal(t, sent.Save.Daily.VoyageAttempts, unpaid.Daily.VoyageAttempts)
		var battles int64
		require.NoError(t, model.DB.Model(&model.GachaRun{}).Where("user_id = ?", owner.Id).Count(&battles).Error)
		assert.Zero(t, battles)
	})
	t.Run("winning_resource_battle_capacity_is_atomic_and_finish_retries_once", func(t *testing.T) {
		const stage = "res_manifest"
		owner := gachaUser(t)
		state := gachaState(t, owner.Id, now)
		strongGachaState(state)
		state.Cleared["s1"], state.Mats["field_notes"] = true, maxResource
		seedGachaState(t, owner.Id, state)
		run, err := model.StartGachaRun(owner.Id, stage, uuid.NewString(), now)
		require.NoError(t, err)
		require.True(t, run.Result.Win)
		assert.Equal(t, state.Stamina-6, run.Save.Stamina)
		assert.Equal(t, state.Captain.XP+6, run.Save.Captain.XP)
		finishAt := time.UnixMilli(run.ReadyAt + 1)
		out, err := model.FinishGachaRun(owner.Id, run.ID, false, finishAt)
		require.ErrorIs(t, err, model.ErrGachaCapacity)
		unpaid := gachaState(t, owner.Id, finishAt)
		assert.Equal(t, state.Mats, unpaid.Mats)
		assert.Equal(t, run.Save.Stamina, unpaid.Stamina)
		assert.Equal(t, run.Save.Captain, unpaid.Captain)
		var record model.GachaRun
		require.NoError(t, model.DB.First(&record, "id = ?", run.ID).Error)
		assert.Equal(t, "running", record.Status)
		unpaid.Mats["field_notes"] -= 2
		seedGachaState(t, owner.Id, unpaid)
		out, err = model.FinishGachaRun(owner.Id, run.ID, false, finishAt)
		require.NoError(t, err)
		require.True(t, out.Win)
		assert.True(t, out.Save.Cleared[stage])
		assert.Equal(t, run.Save.Stamina, out.Save.Stamina)
		assert.Equal(t, run.Save.Captain, out.Save.Captain)
		assert.Equal(t, maxResource, out.Save.Mats["field_notes"])
		assert.Equal(t, state.Mats["training_manual"]+1, out.Save.Mats["training_manual"])
		assert.Zero(t, out.Quota)
		assert.Equal(t, owner.Quota, out.WalletQuota)
		again, err := model.FinishGachaRun(owner.Id, run.ID, false, finishAt)
		require.NoError(t, err)
		assert.Equal(t, out, again)
		unpaid = gachaState(t, owner.Id, finishAt)
		assert.Equal(t, out.Save.Mats, unpaid.Mats)
		assert.Equal(t, out.Save.Mailbox, unpaid.Mailbox)
	})
	t.Run("receiving_earned_shards_unlocks_only_once", func(t *testing.T) {
		owner := gachaUser(t)
		state := gachaState(t, owner.Id, now)
		cardID := "c23"
		require.Nil(t, state.Girls[cardID])
		state.Girls[cardID] = &model.GachaOwnedGirl{Shards: 49, Gems: make([]string, 3)}
		week := &state.Progression.Weeks[0]
		week.Completed, week.Activities, week.Rules.ShardsPerWeek = 5, []string{"training", "wish"}, 2
		seedGachaState(t, owner.Id, state)
		weekly := command("weekly_claim", week.Week)
		weekly.GirlID = &cardID
		sent, err := model.ApplyGachaAction(owner.Id, weekly, now)
		require.NoError(t, err)
		unlocked := sent.Save.Girls[cardID]
		assert.Equal(t, 2, unlocked.Star)
		assert.Equal(t, 1, unlocked.Level)
		assert.Equal(t, 1, unlocked.Shards)
		again, err := model.ApplyGachaAction(owner.Id, weekly, now)
		require.NoError(t, err)
		assert.Equal(t, sent, again)
		weekly.RequestID = uuid.NewString()
		_, err = model.ApplyGachaAction(owner.Id, weekly, now)
		require.Error(t, err)
		assert.Equal(t, unlocked, gachaState(t, owner.Id, now).Girls[cardID])
	})
}

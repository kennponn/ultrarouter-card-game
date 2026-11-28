package model_test

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func runGachaDailyEconomyMatrix(t *testing.T, clock time.Time) {
	local := clock.In(time.FixedZone("Beijing", 8*3600))
	now := time.Date(local.Year(), local.Month(), local.Day(), 12, 0, 0, 0, local.Location())
	command := func(action, id string) model.GachaActionIntent {
		intent := model.GachaActionIntent{Action: action, RequestID: uuid.NewString()}
		if id != "" {
			intent.ItemID = &id
		}
		return intent
	}
	t.Run("daily_real_gear_and_skill_actions_to_direct_task_and_chest_rewards", func(t *testing.T) {
		owner, other := gachaUser(t), gachaUser(t)
		profile, err := model.GetGachaProfile(owner.Id, now)
		require.NoError(t, err)
		state := profile.Save
		girl, line := "c01", "normal"
		slot := profile.Equipment.Slots[1]
		state.Girls[girl].Level = 5
		state.Gold, state.SkillPool.Points, state.SkillPool.TickAt = 0, 5, now.UnixMilli()
		state.Mats[slot.LevelMat] = 0
		seedGachaState(t, owner.Id, state)
		gear, skill := command("gear_auto", slot.ID), command("skill_auto", "")
		gear.GirlID, skill.GirlID, skill.Choice = &girl, &girl, &line
		for _, intent := range []model.GachaActionIntent{gear, skill} {
			_, err = model.ApplyGachaAction(owner.Id, intent, now)
			require.ErrorIs(t, err, model.ErrGachaFunds)
		}
		state = gachaState(t, owner.Id, now)
		log := state.Daily.CaptainLog
		require.Zero(t, log.Progress["skill"])
		require.Zero(t, log.Progress["equipment"])
		chest := command("captain_chest", state.Daily.Date+":25")
		_, err = model.ApplyGachaAction(owner.Id, chest, now)
		require.ErrorIs(t, err, model.ErrGachaCaptainClaim)
		state.Gold = 100_000
		for _, cost := range profile.Equipment.LevelCosts[:5] {
			state.Mats[slot.LevelMat] += cost
		}
		seedGachaState(t, owner.Id, state)
		for _, test := range []struct {
			intent model.GachaActionIntent
			task   string
		}{{gear, "equipment"}, {skill, "skill"}} {
			out, err := model.ApplyGachaAction(owner.Id, test.intent, now)
			require.NoError(t, err)
			require.Equal(t, 5, out.Save.Daily.CaptainLog.Progress[test.task])
			require.False(t, out.Save.Daily.CaptainLog.Claimed[test.task], "completing the task must still require an explicit claim")
			replay, err := model.ApplyGachaAction(owner.Id, test.intent, now)
			require.NoError(t, err)
			require.Equal(t, out.Revision, replay.Revision)
			claim := command("captain_task", out.Save.Daily.Date+":"+test.task)
			claimed, err := model.ApplyGachaAction(owner.Id, claim, now)
			require.NoError(t, err)
			require.True(t, claimed.Save.Daily.CaptainLog.Claimed[test.task])
			require.Greater(t, claimed.Save.Gold, out.Save.Gold)
			require.Len(t, claimed.Save.Mailbox.Messages, len(out.Save.Mailbox.Messages))
			again, err := model.ApplyGachaAction(owner.Id, claim, now)
			require.NoError(t, err)
			require.Equal(t, claimed, again)
		}
		before := gachaState(t, owner.Id, now)
		out, err := model.ApplyGachaAction(owner.Id, chest, now)
		require.NoError(t, err)
		require.True(t, out.Save.Daily.CaptainLog.Chests["25"])
		chestRules := profile.CaptainRules.Chests[0]
		require.Equal(t, before.Mats["stamina_pack_30"]+chestRules.Resources["stamina_pack_30"], out.Save.Mats["stamina_pack_30"])
		require.Equal(t, before.Token+chestRules.Resources["token"], out.Save.Token)
		for _, letter := range out.Save.Mailbox.Messages {
			require.NotEqual(t, "daily:"+before.Daily.Date+":chest-25", letter.ID)
		}
		retry, err := model.ApplyGachaAction(owner.Id, chest, now)
		require.NoError(t, err)
		require.Equal(t, out.Revision, retry.Revision)
		chest.RequestID = uuid.NewString()
		_, err = model.ApplyGachaAction(owner.Id, chest, now)
		require.ErrorIs(t, err, model.ErrGachaCaptainClaim)
		tomorrow := now.AddDate(0, 0, 1)
		_, err = model.ApplyGachaAction(owner.Id, chest, tomorrow)
		require.ErrorIs(t, err, model.ErrGachaCaptainClaim)
		otherState := gachaState(t, other.Id, tomorrow)
		require.Zero(t, otherState.Mats["stamina_pack_30"])
		fresh := gachaState(t, owner.Id, tomorrow)
		require.Equal(t, out.Save.Mats["stamina_pack_30"], fresh.Mats["stamina_pack_30"])
		require.Equal(t, out.Save.Token, fresh.Token)
		require.Empty(t, fresh.Daily.CaptainLog.Chests)
		require.Zero(t, fresh.Daily.CaptainLog.Progress["skill"])

	})
	t.Run("daily_two_free_pools_explicit_prices_replay_and_midnight", func(t *testing.T) {
		owner, other := gachaUser(t), gachaUser(t)
		profile, err := model.GetGachaProfile(owner.Id, now)
		require.NoError(t, err)
		state := profile.Save
		price := profile.Cultivation.Artifacts.DrawToken
		state.Token = price
		state.Mats["chip_ticket"], state.Mats["gem_ticket"] = 10, 1
		seedGachaState(t, owner.Id, state)
		draw := func(pool, method, day string, count int) model.GachaActionIntent {
			intent := command(pool, day)
			intent.Count, intent.Choice = &count, &method
			return intent
		}
		gem := draw("draw_gems", "free", state.Daily.Date, 1)
		_, err = model.ApplyGachaAction(owner.Id, draw("draw_gems", "ticket", "", 1), now)
		require.ErrorIs(t, err, model.ErrGachaArtifactFreeFirst)
		out, err := model.ApplyGachaAction(owner.Id, gem, now)
		require.NoError(t, err)
		require.Equal(t, &model.GachaArtifactPayment{Resource: "free"}, out.ArtifactPayment)
		require.Equal(t, price, out.Save.Token)
		require.Equal(t, 1, out.Save.Mats["gem_ticket"])
		require.Equal(t, map[string]bool{"gem_ticket": true}, out.Save.Daily.ArtifactFree)
		retry, err := model.ApplyGachaAction(owner.Id, gem, now)
		require.NoError(t, err)
		require.Equal(t, out.ArtifactLoot, retry.ArtifactLoot)
		gem.RequestID = uuid.NewString()
		_, err = model.ApplyGachaAction(owner.Id, gem, now)
		require.ErrorIs(t, err, model.ErrGachaArtifactFree)
		_, err = model.ApplyGachaAction(other.Id, gem, now)
		require.NoError(t, err)
		paid, err := model.ApplyGachaAction(owner.Id, draw("draw_gems", "token", "", 1), now)
		require.NoError(t, err)
		require.Equal(t, &model.GachaArtifactPayment{Resource: "token", Amount: price}, paid.ArtifactPayment)
		require.Zero(t, paid.Save.Token)
		_, err = model.ApplyGachaAction(owner.Id, draw("draw_gems", "token", "", 1), now)
		require.ErrorIs(t, err, model.ErrGachaFunds)
		require.Equal(t, 1, gachaState(t, owner.Id, now).Mats["gem_ticket"])
		_, err = model.ApplyGachaAction(owner.Id, draw("draw_chips", "free", state.Daily.Date, 1), now)
		require.NoError(t, err)
		chips, err := model.ApplyGachaAction(owner.Id, draw("draw_chips", "ticket", "", 10), now)
		require.NoError(t, err)
		require.Equal(t, &model.GachaArtifactPayment{Resource: "chip_ticket", Amount: 10}, chips.ArtifactPayment)
		require.Zero(t, chips.Save.Mats["chip_ticket"])
		tomorrow := now.AddDate(0, 0, 1)
		_, err = model.ApplyGachaAction(owner.Id, gem, tomorrow)
		require.ErrorIs(t, err, model.ErrGachaArtifactFree)
		fresh := gachaState(t, owner.Id, tomorrow)
		require.Empty(t, fresh.Daily.ArtifactFree)
		_, err = model.ApplyGachaAction(owner.Id, draw("draw_gems", "free", fresh.Daily.Date, 1), tomorrow)
		require.NoError(t, err)
	})
}

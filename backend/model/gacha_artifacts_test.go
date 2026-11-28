package model_test

import (
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func artifactSlot(action, girl string, slot int) model.GachaActionIntent {
	intent := cultivationIntent(action, girl)
	intent.Count = &slot
	return intent
}

func TestGachaArtifactIntentBoundaries(t *testing.T) {
	chip, wrong, bad, ticket, count, enabled := "edge_1", "edge_2", "edge_01", "gem_ticket", 10, true
	valid := artifactSlot("chip_equip", "c01", 1)
	valid.ItemID = &chip
	require.NoError(t, model.ValidateGachaAction(valid))
	for _, id := range []*string{nil, &wrong, &bad} {
		invalid := valid
		invalid.ItemID = id
		assert.ErrorIs(t, model.ValidateGachaAction(invalid), model.ErrGachaIntent)
	}
	valid.Enabled = &enabled
	assert.ErrorIs(t, model.ValidateGachaAction(valid), model.ErrGachaIntent)
	lock := artifactSlot("chip_lock", "c01", 6)
	assert.ErrorIs(t, model.ValidateGachaAction(lock), model.ErrGachaIntent)
	lock.Enabled = &enabled
	require.NoError(t, model.ValidateGachaAction(lock))
	for _, action := range []string{"draw_gems", "draw_chips"} {
		draw := model.GachaActionIntent{RequestID: uuid.NewString(), Action: action, Count: &count}
		require.NoError(t, model.ValidateGachaAction(draw))
		draw.ItemID = &chip
		assert.ErrorIs(t, model.ValidateGachaAction(draw), model.ErrGachaIntent)
		draw.ItemID, draw.GirlID = nil, &wrong
		assert.ErrorIs(t, model.ValidateGachaAction(draw), model.ErrGachaIntent)
		for _, invalidCount := range []int{-1, 0, 2, 9, 11} {
			draw.GirlID, draw.Count = nil, &invalidCount
			assert.ErrorIs(t, model.ValidateGachaAction(draw), model.ErrGachaIntent)
		}
		free, token, invalid, day := "free", "token", "gold", "2026-11-28"
		one := 1
		draw.Count, draw.Choice, draw.ItemID = &count, &free, &day
		require.ErrorIs(t, model.ValidateGachaAction(draw), model.ErrGachaIntent)
		draw.Count = &one
		require.NoError(t, model.ValidateGachaAction(draw))
		draw.Choice = &token
		require.ErrorIs(t, model.ValidateGachaAction(draw), model.ErrGachaIntent)
		draw.ItemID = nil
		require.NoError(t, model.ValidateGachaAction(draw))
		draw.Choice = &invalid
		require.ErrorIs(t, model.ValidateGachaAction(draw), model.ErrGachaIntent)
		draw.Choice = &free
		require.ErrorIs(t, model.ValidateGachaAction(draw), model.ErrGachaIntent)
	}
	convert := model.GachaActionIntent{RequestID: uuid.NewString(), Action: "convert_tickets", ItemID: &ticket}
	require.NoError(t, model.ValidateGachaAction(convert))
	convert.Count = &count
	assert.ErrorIs(t, model.ValidateGachaAction(convert), model.ErrGachaIntent)
	convert.Count, convert.ItemID = nil, &chip
	assert.ErrorIs(t, model.ValidateGachaAction(convert), model.ErrGachaIntent)
	for _, slot := range []int{0, 10} {
		assert.ErrorIs(t, model.ValidateGachaAction(artifactSlot("gem_socket", "c01", slot)), model.ErrGachaIntent)
	}
}

func runGachaArtifactMatrix(t *testing.T, now time.Time) {
	t.Run("free_draws_token_and_ticket_choices_are_daily_personal_and_replay_safe", func(t *testing.T) {
		user := gachaUser(t)
		profile, err := model.GetGachaProfile(user.Id, now)
		require.NoError(t, err)
		state := profile.Save
		price := profile.Cultivation.Artifacts.DrawToken
		require.Positive(t, price)
		state.Token, state.Mats["gem_ticket"] = 10*price, 1
		seedGachaState(t, user.Id, state)
		one, ten := 1, 10
		free, token, ticket, day := "free", "token", "ticket", state.Daily.Date
		for _, action := range []string{"draw_gems", "draw_chips"} {
			for _, cost := range []struct {
				choice *string
				count  *int
			}{{nil, &ten}, {&ticket, &one}, {&ticket, &ten}, {&token, &one}, {&token, &ten}} {
				_, err := model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: action, Count: cost.count, Choice: cost.choice}, now)
				require.ErrorIs(t, err, model.ErrGachaArtifactFreeFirst)
			}
		}
		unchanged := gachaState(t, user.Id, now)
		require.Equal(t, state.Token, unchanged.Token)
		require.Equal(t, state.Mats, unchanged.Mats)
		require.Empty(t, unchanged.Daily.ArtifactFree)
		require.Empty(t, unchanged.GemInventory)
		require.Empty(t, unchanged.ChipInventory)
		first := model.GachaActionIntent{RequestID: uuid.NewString(), Action: "draw_gems", Count: &one, Choice: &free, ItemID: &day}
		var commands [2]model.GachaActionIntent
		var outputs [2]*model.GachaActionResult
		var errors [2]error
		var workers sync.WaitGroup
		for index := range commands {
			commands[index] = first
			commands[index].RequestID = uuid.NewString()
			workers.Add(1)
			go func(index int) {
				defer workers.Done()
				outputs[index], errors[index] = model.ApplyGachaAction(user.Id, commands[index], now)
			}(index)
		}
		workers.Wait()
		winner := 0
		if errors[0] != nil {
			winner = 1
		}
		require.NoError(t, errors[winner])
		require.ErrorIs(t, errors[1-winner], model.ErrGachaArtifactFree)
		out := outputs[winner]
		require.Equal(t, &model.GachaArtifactPayment{Resource: "free"}, out.ArtifactPayment)
		require.True(t, out.Save.Daily.ArtifactFree["gem_ticket"])
		require.False(t, out.Save.Daily.ArtifactFree["chip_ticket"])
		require.Equal(t, state.Token, out.Save.Token)
		require.Equal(t, 1, out.Save.Mats["gem_ticket"])
		replay, err := model.ApplyGachaAction(user.Id, commands[winner], now)
		require.NoError(t, err)
		require.Equal(t, out, replay)
		chipPaid := model.GachaActionIntent{RequestID: uuid.NewString(), Action: "draw_chips", Count: &one, Choice: &token}
		_, err = model.ApplyGachaAction(user.Id, chipPaid, now)
		require.ErrorIs(t, err, model.ErrGachaArtifactFreeFirst)
		first.Action = "draw_chips"
		out, err = model.ApplyGachaAction(user.Id, first, now)
		require.NoError(t, err)
		require.True(t, out.Save.Daily.ArtifactFree["chip_ticket"])
		paid := model.GachaActionIntent{RequestID: uuid.NewString(), Action: "draw_gems", Count: &ten, Choice: &token}
		out, err = model.ApplyGachaAction(user.Id, paid, now)
		require.NoError(t, err)
		require.Equal(t, &model.GachaArtifactPayment{Resource: "token", Amount: price * 10}, out.ArtifactPayment)
		require.Zero(t, out.Save.Token)
		require.Equal(t, 1, out.Save.Mats["gem_ticket"])
		paidBatch, batchReceipt := paid, out
		paid.Count, paid.RequestID = &one, uuid.NewString()
		_, err = model.ApplyGachaAction(user.Id, paid, now)
		require.ErrorIs(t, err, model.ErrGachaFunds)
		paid.Choice = nil
		out, err = model.ApplyGachaAction(user.Id, paid, now)
		require.NoError(t, err)
		require.Equal(t, &model.GachaArtifactPayment{Resource: "gem_ticket", Amount: 1}, out.ArtifactPayment)
		require.Zero(t, out.Save.Mats["gem_ticket"])
		tomorrow := now.AddDate(0, 0, 1)
		fresh := gachaState(t, user.Id, tomorrow)
		require.Empty(t, fresh.Daily.ArtifactFree)
		replay, err = model.ApplyGachaAction(user.Id, paidBatch, tomorrow)
		require.NoError(t, err)
		require.Equal(t, batchReceipt, replay)
		replay, err = model.ApplyGachaAction(user.Id, commands[winner], tomorrow)
		require.NoError(t, err)
		require.Equal(t, outputs[winner], replay)
		paidBatch.RequestID = uuid.NewString()
		_, err = model.ApplyGachaAction(user.Id, paidBatch, tomorrow)
		require.ErrorIs(t, err, model.ErrGachaArtifactFreeFirst)
		require.Empty(t, gachaState(t, user.Id, tomorrow).Daily.ArtifactFree)
		first.RequestID = uuid.NewString()
		_, err = model.ApplyGachaAction(user.Id, first, tomorrow)
		require.ErrorIs(t, err, model.ErrGachaArtifactFree)
		first.ItemID = &fresh.Daily.Date
		out, err = model.ApplyGachaAction(user.Id, first, tomorrow)
		require.NoError(t, err)
		require.True(t, out.Save.Daily.ArtifactFree["chip_ticket"])
		legacySingle := model.GachaActionIntent{RequestID: uuid.NewString(), Action: "draw_gems", Count: &one}
		out, err = model.ApplyGachaAction(user.Id, legacySingle, tomorrow)
		require.NoError(t, err)
		require.Equal(t, &model.GachaArtifactPayment{Resource: "free"}, out.ArtifactPayment)
		replay, err = model.ApplyGachaAction(user.Id, legacySingle, tomorrow)
		require.NoError(t, err)
		require.Equal(t, out, replay)
		legacySingle.RequestID = uuid.NewString()
		_, err = model.ApplyGachaAction(user.Id, legacySingle, tomorrow)
		require.ErrorIs(t, err, model.ErrGachaFunds)
	})
	t.Run("captain_upgrade_counters_and_chest_are_committed_with_the_paid_actions", func(t *testing.T) {
		user := gachaUser(t)
		state := gachaState(t, user.Id, now)
		state.Captain.Level, state.Girls["c01"].Level = 10, 10
		state.Gold, state.Mats["gear_thread"] = 0, 0
		seedGachaState(t, user.Id, state)
		hero, slot, line := "c01", "head", "normal"
		gear := model.GachaActionIntent{RequestID: uuid.NewString(), Action: "gear_auto", ItemID: &slot, GirlID: &hero}
		skill := model.GachaActionIntent{RequestID: uuid.NewString(), Action: "skill_auto", Choice: &line, GirlID: &hero}
		for _, command := range []model.GachaActionIntent{gear, skill} {
			_, err := model.ApplyGachaAction(user.Id, command, now)
			require.ErrorIs(t, err, model.ErrGachaFunds)
		}
		unchanged := gachaState(t, user.Id, now)
		require.Equal(t, map[string]int{"rollcall": 1}, unchanged.Daily.CaptainLog.Progress)
		state.Gold, state.Mats["gear_thread"] = 100000, 25
		seedGachaState(t, user.Id, state)
		out, err := model.ApplyGachaAction(user.Id, gear, now)
		require.NoError(t, err)
		require.Equal(t, 5, out.Save.Daily.CaptainLog.Progress["equipment"])
		require.Equal(t, 1, out.Save.Daily.CaptainLog.Progress["outfit"])
		out, err = model.ApplyGachaAction(user.Id, skill, now)
		require.NoError(t, err)
		require.Equal(t, 5, out.Save.Daily.CaptainLog.Progress["skill"])
		replay, err := model.ApplyGachaAction(user.Id, skill, now)
		require.NoError(t, err)
		require.Equal(t, out, replay)
		chestID := state.Daily.Date + ":25"
		chest := model.GachaActionIntent{RequestID: uuid.NewString(), Action: "captain_chest", ItemID: &chestID}
		out, err = model.ApplyGachaAction(user.Id, chest, now)
		require.NoError(t, err)
		require.Equal(t, state.Token+5, out.Save.Token)
		require.Equal(t, 1, out.Save.Mats["stamina_pack_30"])
		require.Empty(t, out.Save.Daily.CaptainLog.Claimed)
		replay, err = model.ApplyGachaAction(user.Id, chest, now)
		require.NoError(t, err)
		require.Equal(t, out, replay)
		chest.RequestID = uuid.NewString()
		_, err = model.ApplyGachaAction(user.Id, chest, now)
		require.ErrorIs(t, err, model.ErrGachaCaptainClaim)
	})
	t.Run("graded_artifact_exact_price_at_cap_and_slot_transfer", func(t *testing.T) {
		user := gachaUser(t)
		state := gachaState(t, user.Id, now)
		first, other := "c01", "c08"
		base := "ruby~red!119.11"
		state.Girls[first].Sockets, state.Girls[other].Sockets = 1, 1
		state.Girls[first].Gems[0] = base
		state.Mats["gear_thread"], state.Mats["forge_ore"], state.Mats["gear_rivet"], state.Mats["gem_ticket"] = 378, 5, 150, 15
		seedGachaState(t, user.Id, state)
		level := refinementIntent("gem_level", first, base, 1)
		_, err := model.ApplyGachaAction(user.Id, level, now)
		require.ErrorIs(t, err, model.ErrGachaFunds)
		state = gachaState(t, user.Id, now)
		assert.Equal(t, 378, state.Mats["gear_thread"])
		assert.Equal(t, base, state.Girls[first].Gems[0])
		state.Mats["forge_ore"] = 6
		seedGachaState(t, user.Id, state)
		out, err := model.ApplyGachaAction(user.Id, level, now)
		require.NoError(t, err)
		atMax := "ruby~red!120.11"
		assert.Equal(t, atMax, out.Save.Girls[first].Gems[0])
		assert.Zero(t, out.Save.Mats["gear_thread"])
		replayed, err := model.ApplyGachaAction(user.Id, level, now)
		require.NoError(t, err)
		assert.Equal(t, out, replayed)
		_, err = model.ApplyGachaAction(user.Id, refinementIntent("gem_level", first, atMax, 1), now)
		require.ErrorIs(t, err, model.ErrGachaCultivationLocked)
		rank := refinementIntent("gem_rank", first, atMax, 1)
		_, err = model.ApplyGachaAction(user.Id, rank, now)
		require.ErrorIs(t, err, model.ErrGachaFunds)
		state = gachaState(t, user.Id, now)
		assert.Equal(t, 150, state.Mats["gear_rivet"])
		state.Mats["gem_ticket"] = 16
		seedGachaState(t, user.Id, state)
		out, err = model.ApplyGachaAction(user.Id, rank, now)
		require.NoError(t, err)
		final := "ruby~red!120.12"
		assert.Equal(t, final, out.Save.Girls[first].Gems[0])
		assert.Zero(t, out.Save.Mats["gear_rivet"])
		assert.Zero(t, out.Save.Mats["gem_ticket"])
		assert.Zero(t, out.Save.GemInventory[final])
		replayed, err = model.ApplyGachaAction(user.Id, rank, now)
		require.NoError(t, err)
		assert.Equal(t, out, replayed)
		_, err = model.ApplyGachaAction(user.Id, refinementIntent("gem_rank", first, final, 1), now)
		require.ErrorIs(t, err, model.ErrGachaCultivationLocked)
		out, err = model.ApplyGachaAction(user.Id, artifactSlot("gem_remove", first, 1), now)
		require.NoError(t, err)
		assert.Equal(t, 1, out.Save.GemInventory[final])
		out, err = model.ApplyGachaAction(user.Id, refinementIntent("gem_equip", other, final, 1), now)
		require.NoError(t, err)
		assert.Zero(t, out.Save.GemInventory[final])
		assert.Equal(t, final, out.Save.Girls[other].Gems[0])
		assert.Empty(t, out.Save.Girls[first].Gems[0])
	})
	t.Run("casebook_old_save_daily_rollover_and_receipt_replay", func(t *testing.T) {
		user := gachaUser(t)
		state := gachaState(t, user.Id, now)
		state.Casebook = nil
		seedGachaState(t, user.Id, state)
		profile, err := model.GetGachaProfile(user.Id, now)
		require.NoError(t, err)
		require.NotNil(t, profile.Cases)
		require.NotNil(t, profile.Save.Casebook)
		require.Empty(t, profile.Save.Casebook.Evidence)
		id, lead := "silent_bell", "c01"
		open := model.GachaActionIntent{RequestID: uuid.NewString(), Action: "case_open", ItemID: &id, GirlID: &lead}
		opened, err := model.ApplyGachaAction(user.Id, open, now)
		require.NoError(t, err)
		require.Equal(t, open.RequestID, opened.Save.Casebook.Active.ID)
		step := 1
		choice := "survey"
		move := model.GachaActionIntent{RequestID: uuid.NewString(), Action: "case_step", ItemID: &open.RequestID, Count: &step, Choice: &choice}
		first, err := model.ApplyGachaAction(user.Id, move, now)
		require.NoError(t, err)
		dup, err := model.ApplyGachaAction(user.Id, move, now)
		require.NoError(t, err)
		assert.Equal(t, first, dup)
		move.RequestID = uuid.NewString()
		_, err = model.ApplyGachaAction(user.Id, move, now)
		require.ErrorIs(t, err, model.ErrGachaCultivationLocked)
		file := "file"
		close := model.GachaActionIntent{RequestID: uuid.NewString(), Action: "case_close", ItemID: &open.RequestID, Count: &step, Choice: &file}
		out, err := model.ApplyGachaAction(user.Id, close, now)
		require.NoError(t, err)
		dup, err = model.ApplyGachaAction(user.Id, close, now)
		require.NoError(t, err)
		assert.Equal(t, out, dup)
		assert.Equal(t, 2, out.Save.Casebook.Evidence["echo"])
		assert.Equal(t, 1, out.Save.Casebook.Endings[id]["filed"])
		other := "glass_clock"
		open.ItemID = &other
		open.RequestID = uuid.NewString()
		_, err = model.ApplyGachaAction(user.Id, open, now)
		require.NoError(t, err)
		state = gachaState(t, user.Id, now.AddDate(0, 0, 1))
		assert.Nil(t, state.Casebook.Active)
		assert.Empty(t, state.Casebook.Opened)
		assert.Equal(t, "withdrawn", state.Casebook.Last.Ending)
		assert.Equal(t, 2, state.Casebook.Evidence["echo"])
	})
	t.Run("artifacts_old_sockets_extend_without_free_openings_or_item_loss", func(t *testing.T) {
		user := gachaUser(t)
		id := "c01"
		state := gachaState(t, user.Id, now)
		own := state.Girls[id]
		own.Gems, own.Sockets = []string{"ruby", "", "jade"}, 3
		state.GemInventory["wind"] = 1
		state.Gold, state.Mats["mat_r"] = 20000, 500
		seedGachaState(t, user.Id, state)
		state = gachaState(t, user.Id, now)
		require.Len(t, state.Girls[id].Gems, 9)
		assert.Equal(t, []string{"ruby", "", "jade", "", "", "", "", "", ""}, state.Girls[id].Gems)
		assert.Equal(t, 3, state.Girls[id].Sockets)
		_, err := model.ApplyGachaAction(user.Id, artifactSlot("gem_socket", id, 4), now)
		require.ErrorIs(t, err, model.ErrGachaCultivationLocked)
		state.Girls[id].Star, state.Girls[id].Level = 10, 60
		state.Girls[id].Awakening, state.Girls[id].Branch = 3, model.GachaAwakeningBranches(id)[0].ID
		seedGachaState(t, user.Id, state)
		gold, mats := state.Gold, state.Mats["mat_r"]
		_, err = model.ApplyGachaAction(user.Id, artifactSlot("gem_socket", id, 5), now)
		require.ErrorIs(t, err, model.ErrGachaIntent)
		for slot := 4; slot <= 9; slot++ {
			intent := artifactSlot("gem_socket", id, slot)
			out, err := model.ApplyGachaAction(user.Id, intent, now)
			require.NoError(t, err)
			gold -= 1800 + (slot-4)*300
			mats -= 30 + (slot-4)*5
			assert.Equal(t, gold, out.Save.Gold)
			assert.Equal(t, mats, out.Save.Mats["mat_r"])
			assert.Equal(t, slot, out.Save.Girls[id].Sockets)
		}
		equip := artifactSlot("gem_equip", id, 9)
		wind := "wind"
		equip.ItemID = &wind
		out, err := model.ApplyGachaAction(user.Id, equip, now)
		require.NoError(t, err)
		assert.Equal(t, wind, out.Save.Girls[id].Gems[8])
		assert.Zero(t, out.Save.GemInventory[wind])
		out, err = model.ApplyGachaAction(user.Id, artifactSlot("gem_remove", id, 9), now)
		require.NoError(t, err)
		assert.Equal(t, 1, out.Save.GemInventory[wind])
		assert.Empty(t, out.Save.Girls[id].Gems[8])
		assert.Equal(t, "ruby", out.Save.Girls[id].Gems[0])
		assert.Equal(t, "jade", out.Save.Girls[id].Gems[2])
	})

	t.Run("artifacts_chip_slot_locks_replacement_and_capacity", func(t *testing.T) {
		user := gachaUser(t)
		id := "c01"
		state := gachaState(t, user.Id, now)
		state.ChipInventory = map[string]int{"edge_1": 1, "aegis_1": 1, "tempo_6": 1}
		seedGachaState(t, user.Id, state)
		equip := func(chip string, slot int) (*model.GachaActionResult, error) {
			intent := artifactSlot("chip_equip", id, slot)
			intent.ItemID = &chip
			return model.ApplyGachaAction(user.Id, intent, now)
		}
		lock := func(enabled bool) (*model.GachaActionResult, error) {
			intent := artifactSlot("chip_lock", id, 1)
			intent.Enabled = &enabled
			return model.ApplyGachaAction(user.Id, intent, now)
		}
		_, err := lock(true)
		require.ErrorIs(t, err, model.ErrGachaCultivationLocked)
		_, err = equip("tempo_6", 6)
		require.ErrorIs(t, err, model.ErrGachaCultivationLocked)
		_, err = equip("tempo_1", 1)
		require.ErrorIs(t, err, model.ErrGachaFunds)
		out, err := equip("edge_1", 1)
		require.NoError(t, err)
		assert.Zero(t, out.Save.ChipInventory["edge_1"])
		assert.Equal(t, "edge_1", out.Save.Girls[id].Chips[0])
		out, err = lock(true)
		require.NoError(t, err)
		assert.Equal(t, 1, out.Save.Girls[id].ChipLocks)
		_, err = equip("aegis_1", 1)
		require.ErrorIs(t, err, model.ErrGachaCultivationLocked)
		_, err = model.ApplyGachaAction(user.Id, artifactSlot("chip_remove", id, 1), now)
		require.ErrorIs(t, err, model.ErrGachaCultivationLocked)
		out, err = lock(false)
		require.NoError(t, err)
		state = out.Save
		state.ChipInventory["edge_1"] = 1_000_000_000
		seedGachaState(t, user.Id, state)
		_, err = equip("aegis_1", 1)
		require.ErrorIs(t, err, model.ErrGachaCapacity)
		state = gachaState(t, user.Id, now)
		assert.Equal(t, 1, state.ChipInventory["aegis_1"])
		assert.Equal(t, "edge_1", state.Girls[id].Chips[0])
		state.ChipInventory["edge_1"] = 0
		seedGachaState(t, user.Id, state)
		out, err = equip("aegis_1", 1)
		require.NoError(t, err)
		assert.Equal(t, 1, out.Save.ChipInventory["edge_1"])
		assert.Zero(t, out.Save.ChipInventory["aegis_1"])
		out, err = model.ApplyGachaAction(user.Id, artifactSlot("chip_remove", id, 1), now)
		require.NoError(t, err)
		assert.Equal(t, 1, out.Save.ChipInventory["aegis_1"])
		assert.Empty(t, out.Save.Girls[id].Chips[0])
	})

	t.Run("artifacts_ticket_conversion_draws_and_receipts_are_atomic", func(t *testing.T) {
		user := gachaUser(t)
		state := gachaState(t, user.Id, now)
		state.Gold, state.Token = 0, 0
		state.Mats["forge_ore"], state.Mats["gem_ticket"], state.Mats["chip_ticket"] = 16, 9, 9
		state.Daily.ArtifactFree = map[string]bool{"gem_ticket": true, "chip_ticket": true}
		seedGachaState(t, user.Id, state)
		for _, entry := range []struct{ ticket, action string }{{"gem_ticket", "draw_gems"}, {"chip_ticket", "draw_chips"}} {
			convert := model.GachaActionIntent{RequestID: uuid.NewString(), Action: "convert_tickets", ItemID: &entry.ticket}
			out, err := model.ApplyGachaAction(user.Id, convert, now)
			require.NoError(t, err)
			assert.Equal(t, 10, out.Save.Mats[entry.ticket])
			replay, err := model.ApplyGachaAction(user.Id, convert, now)
			require.NoError(t, err)
			assert.Equal(t, out, replay)
			count := 10
			draw := model.GachaActionIntent{RequestID: uuid.NewString(), Action: entry.action, Count: &count}
			out, err = model.ApplyGachaAction(user.Id, draw, now)
			require.NoError(t, err)
			inventory := out.Save.GemInventory
			if entry.action == "draw_chips" {
				inventory = out.Save.ChipInventory
			}
			assert.Zero(t, out.Save.Mats[entry.ticket])
			assert.Len(t, out.Drops, len(inventory))
			assert.Len(t, out.ArtifactLoot, len(inventory))
			for _, item := range out.ArtifactLoot {
				assert.Equal(t, inventory[item.ItemID], item.Quantity)
				assert.Positive(t, item.Quantity)
			}
			receiptTotal, itemTotal := 0, 0
			for _, quantity := range inventory {
				itemTotal += quantity
			}
			for _, receipt := range out.Drops {
				_, quantity, found := strings.Cut(receipt, " x")
				require.True(t, found)
				amount, err := strconv.Atoi(quantity)
				require.NoError(t, err)
				receiptTotal += amount
			}
			assert.Equal(t, count, itemTotal)
			assert.Equal(t, itemTotal, receiptTotal)
			replay, err = model.ApplyGachaAction(user.Id, draw, now)
			require.NoError(t, err)
			assert.Equal(t, out, replay)
			draw.RequestID = uuid.NewString()
			_, err = model.ApplyGachaAction(user.Id, draw, now)
			require.ErrorIs(t, err, model.ErrGachaFunds)
		}
		state = gachaState(t, user.Id, now)
		assert.Zero(t, state.Mats["forge_ore"])
		assert.Zero(t, state.Gold)
		assert.Zero(t, state.Token)
		var wallet model.User
		require.NoError(t, model.DB.Select("id", "quota").First(&wallet, user.Id).Error)
		assert.Equal(t, user.Quota, wallet.Quota)
		for _, gem := range []string{"ruby", "jade", "sapphire", "wind", "amethyst"} {
			for _, quality := range []string{"~white", "~green", "", "~purple", "~gold", "~orange", "~red"} {
				state.GemInventory[gem+quality] = 1_000_000_000
			}
		}
		state.Mats["gem_ticket"] = 1
		seedGachaState(t, user.Id, state)
		one := 1
		_, err := model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "draw_gems", Count: &one}, now)
		require.ErrorIs(t, err, model.ErrGachaCapacity)
		after := gachaState(t, user.Id, now)
		assert.Equal(t, 1, after.Mats["gem_ticket"])
		assert.True(t, after.Daily.ArtifactFree["gem_ticket"])
		tomorrow := now.AddDate(0, 0, 1)
		_, err = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "draw_gems", Count: &one}, tomorrow)
		require.ErrorIs(t, err, model.ErrGachaCapacity)
		after = gachaState(t, user.Id, tomorrow)
		assert.Equal(t, 1, after.Mats["gem_ticket"])
		assert.False(t, after.Daily.ArtifactFree["gem_ticket"])
	})
}

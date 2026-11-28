package model_test

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runGachaCarryMatrix(t *testing.T, now time.Time) {
	t.Run("keepsake_receipts_conserve_items_and_materials", func(t *testing.T) {
		user := gachaUser(t)
		profile, err := model.GetGachaProfile(user.Id, now)
		require.NoError(t, err)
		require.Len(t, profile.Carry.Items, 6)
		state := profile.Save
		state.Mats["route_badge"], state.Mats["forge_ore"] = 8, 4
		state.CarryInventory["keepsake_banner"] = 1
		seedGachaState(t, user.Id, state)
		lens, banner, hero, missing, amount := "keepsake_lens", "keepsake_banner", "c01", "c04", 2
		craft := model.GachaActionIntent{RequestID: uuid.NewString(), Action: "carry_craft", ItemID: &lens}
		equip := model.GachaActionIntent{RequestID: uuid.NewString(), Action: "carry_equip", ItemID: &lens, GirlID: &hero}
		remove := model.GachaActionIntent{RequestID: uuid.NewString(), Action: "carry_remove", GirlID: &hero}
		apply := func(intent model.GachaActionIntent) *model.GachaActionResult {
			out, err := model.ApplyGachaAction(user.Id, intent, now)
			require.NoError(t, err)
			return out
		}
		illegal := craft
		illegal.Count = &amount
		assert.ErrorIs(t, model.ValidateGachaAction(illegal), model.ErrGachaIntent)
		illegal = remove
		illegal.ItemID = &lens
		assert.ErrorIs(t, model.ValidateGachaAction(illegal), model.ErrGachaIntent)
		illegal = craft
		illegal.GirlID = &hero
		assert.ErrorIs(t, model.ValidateGachaAction(illegal), model.ErrGachaIntent)
		made := apply(craft)
		assert.Equal(t, 1, made.Save.CarryInventory[lens])
		assert.Zero(t, made.Save.Mats["route_badge"])
		assert.Zero(t, made.Save.Mats["forge_ore"])
		assert.Equal(t, made, apply(craft))
		badEquip := equip
		badEquip.GirlID = &missing
		_, err = model.ApplyGachaAction(user.Id, badEquip, now)
		assert.ErrorIs(t, err, model.ErrGachaIntent)
		worn := apply(equip)
		assert.Zero(t, worn.Save.CarryInventory[lens])
		assert.Equal(t, lens, worn.Save.Girls[hero].Carry)
		replace := equip
		replace.ItemID = &banner
		_, err = model.ApplyGachaAction(user.Id, replace, now)
		assert.ErrorIs(t, err, model.ErrGachaReplay)
		replace.RequestID = uuid.NewString()
		replaced := apply(replace)
		assert.Equal(t, 1, replaced.Save.CarryInventory[lens])
		assert.Zero(t, replaced.Save.CarryInventory[banner])
		assert.Equal(t, banner, replaced.Save.Girls[hero].Carry)
		removed := apply(remove)
		assert.Equal(t, 1, removed.Save.CarryInventory[banner])
		assert.Empty(t, removed.Save.Girls[hero].Carry)
		assert.Equal(t, removed, apply(remove))
		assert.Equal(t, profile.Save.Gold, removed.Save.Gold)
		craft.RequestID = uuid.NewString()
		_, err = model.ApplyGachaAction(user.Id, craft, now)
		assert.ErrorIs(t, err, model.ErrGachaFunds)
		assert.Equal(t, removed.Save.CarryInventory, gachaState(t, user.Id, now).CarryInventory)
	})
	t.Run("mastery_materials_follow_issued_rules_and_win_only", func(t *testing.T) {
		user := gachaUser(t)
		for _, attempt := range []struct {
			stage   string
			rules   int
			win     bool
			abandon bool
			amount  int
		}{
			{"s1", 5, true, false, 0}, {"s1", 6, false, false, 0}, {"s1", 6, true, true, 0},
			{"s1", 6, true, false, 1}, {"s3", 6, true, false, 2}, {"r1", 6, true, false, 3}, {"n1", 6, true, false, 4},
			{"r1", 9, true, false, 3},
		} {
			before := gachaState(t, user.Id, now).Mats
			snapshot, err := common.Marshal(model.GachaRunSnapshot{Version: 1, Squad: []string{"c01"}, RulesVersion: attempt.rules, MultiUsed: true})
			require.NoError(t, err)
			report, err := common.Marshal(model.GachaBattleReport{Win: attempt.win, Stars: 1, Rounds: 2, TotalN: 1, AliveN: 1})
			require.NoError(t, err)
			record := model.GachaRun{ID: uuid.NewString(), UserID: user.Id, Stage: attempt.stage, Day: now.Format("2006-01-02"), Status: "running", StartedAt: now.UnixMilli() - 1, ReadyAt: now.UnixMilli(), ExpireAt: now.Add(time.Hour).UnixMilli(), Snapshot: model.GachaDocument(snapshot), BattleReport: model.GachaDocument(report)}
			require.NoError(t, model.DB.Create(&record).Error)
			out, err := model.FinishGachaRun(user.Id, record.ID, attempt.abandon, now)
			require.NoError(t, err)
			saved := out.Save
			assert.Equal(t, before["route_badge"]+attempt.amount, saved.Mats["route_badge"])
			if attempt.amount == 0 {
				assert.Equal(t, before["training_ink"], out.Save.Mats["training_ink"])
				assert.Equal(t, before["break_seal"], out.Save.Mats["break_seal"])
			}
			replay, err := model.FinishGachaRun(user.Id, record.ID, attempt.abandon, now)
			require.NoError(t, err)
			assert.Equal(t, out.Save.Mats, replay.Save.Mats)
		}
	})
	t.Run("keepsake_capacity_includes_worn_copies_and_corrupt_saves_fail_closed", func(t *testing.T) {
		user := gachaUser(t)
		state := gachaState(t, user.Id, now)
		id := "keepsake_lens"
		state.Mats["route_badge"], state.Mats["forge_ore"] = 8, 4
		state.CarryInventory[id], state.Girls["c01"].Carry = 999_999_999, id
		seedGachaState(t, user.Id, state)
		_, err := model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "carry_craft", ItemID: &id}, now)
		require.ErrorIs(t, err, model.ErrGachaCapacity)
		assert.Equal(t, state.Mats, gachaState(t, user.Id, now).Mats)
		for _, invalid := range []map[string]int{{id: 1_000_000_000}, {id: -1}, {"unknown": 1}} {
			state.CarryInventory = invalid
			seedGachaState(t, user.Id, state)
			_, err := model.GetGachaProfile(user.Id, now)
			assert.ErrorIs(t, err, model.ErrGachaRunState)
		}
		state.CarryInventory = nil
		state.Girls["c04"] = &model.GachaOwnedGirl{Shards: 1, Carry: id}
		seedGachaState(t, user.Id, state)
		_, err = model.GetGachaProfile(user.Id, now)
		assert.ErrorIs(t, err, model.ErrGachaRunState)
	})
}

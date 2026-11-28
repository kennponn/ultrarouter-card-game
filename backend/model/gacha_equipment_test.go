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

func equipmentIntent(action, girl, slot string) model.GachaActionIntent {
	intent := cultivationIntent(action, girl)
	intent.ItemID = &slot
	return intent
}

func TestGachaBoundEquipmentIntent(t *testing.T) {
	count, choice, enabled, squad := 1, "head", true, []string{"c01"}
	for _, action := range []string{"gear_upgrade", "gear_auto", "gear_star"} {
		for _, slot := range []string{"weapon", "head", "hands", "body", "feet"} {
			require.NoError(t, model.ValidateGachaAction(equipmentIntent(action, "c01", slot)))
		}
		intent := equipmentIntent(action, "c01", "head")
		for _, change := range []func(*model.GachaActionIntent){
			func(value *model.GachaActionIntent) { value.GirlID = nil },
			func(value *model.GachaActionIntent) { value.ItemID = nil },
			func(value *model.GachaActionIntent) { value.Count = &count },
			func(value *model.GachaActionIntent) { value.Choice = &choice },
			func(value *model.GachaActionIntent) { value.Squad = &squad },
			func(value *model.GachaActionIntent) { value.Enabled = &enabled },
			func(value *model.GachaActionIntent) { value.Config = &model.GachaProgressionConfig{} },
			func(value *model.GachaActionIntent) { value.OpponentID = &count },
			func(value *model.GachaActionIntent) { value.StakeE = &count },
			func(value *model.GachaActionIntent) { value.RoomID = &choice },
			func(value *model.GachaActionIntent) { value.QueueID = &choice },
		} {
			bad := intent
			change(&bad)
			assert.ErrorIs(t, model.ValidateGachaAction(bad), model.ErrGachaIntent)
		}
		assert.ErrorIs(t, model.ValidateGachaAction(equipmentIntent(action, "missing", "head")), model.ErrGachaIntent)
		assert.ErrorIs(t, model.ValidateGachaAction(equipmentIntent(action, "c01", "amulet")), model.ErrGachaIntent)
	}
	assert.ErrorIs(t, model.ValidateGachaAction(equipmentIntent("gear_grant", "c01", "head")), model.ErrGachaIntent)
}

func TestGachaBoundEquipmentSetTiersInBothCombatModes(t *testing.T) {
	const id = "c01"
	own := &model.GachaOwnedGirl{Level: 60, Star: 6, Awakening: 1, Branch: model.GachaAwakeningBranches(id)[0].ID}
	state := &model.GachaGameState{Girls: map[string]*model.GachaOwnedGirl{id: own}, Squad: []string{id}}
	stage := model.GachaStage{Waves: 1, EnemyCount: 1, EnemyLv: 1, StarRound: 5}
	combatUnit := func(t *testing.T) model.GachaBattleUnit {
		t.Helper()
		report, err := model.SimulateGachaCombat(state, stage)
		require.NoError(t, err)
		return report.Allies[0]
	}
	unarmed := combatUnit(t)
	own.WeaponLevel = 10
	base := combatUnit(t)
	assert.InDelta(t, float64(unarmed.Atk-3)*1.20+3, base.Atk, 1)
	assert.Equal(t, unarmed.MaxHP, base.MaxHP)
	basePower, err := model.GachaSquadPower(state)
	require.NoError(t, err)
	type gear = model.GachaEquipmentPiece
	full := map[string]gear{"weapon": {Star: 5}, "head": {Level: 10, Star: 5}, "hands": {Level: 10, Star: 5}, "body": {Level: 10, Star: 5}, "feet": {Level: 10, Star: 5}}
	for _, test := range []struct {
		name   string
		pieces map[string]gear
		bonus  model.GachaStats
	}{
		{"one", map[string]gear{"weapon": {Star: 1}}, model.GachaStats{Atk: 2}},
		{"two", map[string]gear{"weapon": {Star: 1}, "head": {Level: 2, Star: 1}}, model.GachaStats{HP: 7, Atk: 2}},
		{"three", map[string]gear{"weapon": {Star: 3}, "head": {Level: 6, Star: 3}, "hands": {Level: 6, Star: 3}}, model.GachaStats{HP: 15, Atk: 18}},
		{"four", map[string]gear{"weapon": {Star: 3}, "head": {Level: 6, Star: 3}, "hands": {Level: 6, Star: 3}, "body": {Level: 6, Star: 3}}, model.GachaStats{HP: 15, Atk: 22, Def: 22}},
		{"five", full, model.GachaStats{HP: 23, Atk: 34, Def: 34, Spd: 20, CR: 4, CD: 10}},
	} {
		t.Run(test.name, func(t *testing.T) {
			own.Equipment = test.pieces
			unit := combatUnit(t)
			assert.InDelta(t, float64(base.MaxHP)*(1+float64(test.bonus.HP)/100), unit.MaxHP, 1)
			assert.InDelta(t, float64(base.Atk-3)*(1+float64(test.bonus.Atk)/100)+3, unit.Atk, 1)
			assert.InDelta(t, float64(base.Def)*(1+float64(test.bonus.Def)/100), unit.Def, 1)
			assert.Equal(t, base.Spd+test.bonus.Spd, unit.Spd)
			assert.Equal(t, base.CR+test.bonus.CR, unit.CR)
			assert.Equal(t, base.CD+test.bonus.CD, unit.CD)
		})
	}
	trained := combatUnit(t)
	arena, err := model.SimulateGachaArena(state, state)
	require.NoError(t, err)
	for _, unit := range []model.GachaBattleUnit{arena.Allies[0], arena.Waves[0][0]} {
		assert.Equal(t, [6]int{trained.MaxHP, trained.Atk, trained.Def, trained.Spd, trained.CR, trained.CD}, [6]int{unit.MaxHP, unit.Atk, unit.Def, unit.Spd, unit.CR, unit.CD})
	}
	power, err := model.GachaSquadPower(state)
	require.NoError(t, err)
	assert.Greater(t, power, basePower)
	own.Equipment = nil
	partner := &model.GachaOwnedGirl{Level: 60, Star: 6, Awakening: 1, Branch: model.GachaAwakeningBranches("c08")[0].ID, WeaponLevel: 10}
	state.Girls["c08"] = partner
	before := combatUnit(t)
	partner.Equipment = full
	assert.Equal(t, before, combatUnit(t))
}

func runGachaEquipmentMatrix(t *testing.T, now time.Time) {
	t.Run("equipment_bound_slots_costs_auto_stars_and_legacy_weapon", func(t *testing.T) {
		user := gachaUser(t)
		profile, err := model.GetGachaProfile(user.Id, now)
		require.NoError(t, err)
		require.NotNil(t, profile.Equipment)
		var slots []string
		for _, slot := range profile.Equipment.Slots {
			slots = append(slots, slot.ID)
		}
		assert.Equal(t, []string{"weapon", "head", "hands", "body", "feet"}, slots)
		assert.Equal(t, profile.Cultivation.WeaponOre, profile.Equipment.LevelCosts[:10])
		assert.Equal(t, []int{2, 4, 6, 9, 12, 18, 26, 38, 55, 80, 115, 165, 230, 320, 450}, profile.Equipment.StarCosts)
		state, id := profile.Save, "c01"
		state.Girls[id].Star, state.Girls[id].WeaponLevel, state.Girls[id].WeaponRune = 2, 3, "edge"
		state.Gold, state.Mats["forge_ore"], state.Mats["gear_plate"], state.Mats["gear_thread"], state.Mats["gear_rivet"] = 0, 13, 76, 16, 37
		seedGachaState(t, user.Id, state)
		apply := func(action, slot string) (*model.GachaActionResult, error) {
			return model.ApplyGachaAction(user.Id, equipmentIntent(action, id, slot), now)
		}
		_, err = model.ApplyGachaAction(user.Id, equipmentIntent("gear_upgrade", "c04", "head"), now)
		require.ErrorIs(t, err, model.ErrGachaIntent)
		_, err = apply("gear_star", "head")
		require.ErrorIs(t, err, model.ErrGachaCultivationLocked)
		auto := equipmentIntent("gear_auto", id, "head")
		out, err := model.ApplyGachaAction(user.Id, auto, now)
		require.NoError(t, err)
		assert.Equal(t, 3, out.Save.Girls[id].Equipment["head"].Level)
		assert.Equal(t, 4, out.Save.Mats["gear_thread"])
		replay, err := model.ApplyGachaAction(user.Id, auto, now)
		require.NoError(t, err)
		assert.Equal(t, out, replay)
		_, err = apply("gear_auto", "head")
		require.ErrorIs(t, err, model.ErrGachaFunds)
		out, err = apply("gear_upgrade", "feet")
		require.NoError(t, err)
		assert.Equal(t, 1, out.Save.Girls[id].Equipment["feet"].Level)
		assert.Equal(t, 1, out.Save.Mats["gear_thread"])
		out, err = apply("gear_star", "head")
		require.NoError(t, err)
		assert.Equal(t, 35, out.Save.Mats["gear_rivet"])
		_, err = apply("gear_star", "head")
		require.ErrorIs(t, err, model.ErrGachaCultivationLocked)
		out, err = model.ApplyGachaAction(user.Id, cultivationIntent("weapon_upgrade", id), now)
		require.NoError(t, err)
		assert.Equal(t, 4, out.Save.Girls[id].WeaponLevel)
		assert.Equal(t, 7, out.Save.Mats["forge_ore"])
		out, err = apply("gear_auto", "weapon")
		require.NoError(t, err)
		assert.Equal(t, 5, out.Save.Girls[id].WeaponLevel)
		assert.Zero(t, out.Save.Mats["forge_ore"])
		assert.NotContains(t, out.Save.Girls[id].Equipment, "weapon")
		out, err = apply("gear_star", "weapon")
		require.NoError(t, err)
		assert.Equal(t, model.GachaEquipmentPiece{Star: 1}, out.Save.Girls[id].Equipment["weapon"])
		assert.Equal(t, "edge", out.Save.Girls[id].WeaponRune)
		out, err = apply("gear_auto", "body")
		require.NoError(t, err)
		assert.Equal(t, 1, out.Save.Mats["gear_plate"])
		assert.Equal(t, 10, out.Save.Girls[id].Equipment["body"].Level)
		_, err = apply("gear_upgrade", "body")
		require.ErrorIs(t, err, model.ErrGachaFunds)
		out, err = apply("gear_star", "body")
		require.NoError(t, err)
		_, err = apply("gear_star", "body")
		require.ErrorIs(t, err, model.ErrGachaCultivationLocked)
		state = out.Save
		own := state.Girls[id]
		own.Level, own.Star, own.Awakening, own.Branch, own.WeaponAwake = 60, 6, 1, model.GachaAwakeningBranches(id)[0].ID, 1
		seedGachaState(t, user.Id, state)
		for tier, left := range []int{27, 21, 12, 0} {
			out, err = apply("gear_star", "body")
			require.NoError(t, err)
			assert.Equal(t, model.GachaEquipmentPiece{Level: 10, Star: tier + 2}, out.Save.Girls[id].Equipment["body"])
			assert.Equal(t, left, out.Save.Mats["gear_rivet"])
		}
		_, err = apply("gear_star", "body")
		require.ErrorIs(t, err, model.ErrGachaCultivationLocked)
		_, err = apply("gear_star", "weapon")
		require.ErrorIs(t, err, model.ErrGachaFunds)
		state = gachaState(t, user.Id, now)
		assert.Empty(t, state.Girls["c08"].Equipment)
		assert.Equal(t, "edge", state.Girls[id].WeaponRune)
		assert.Equal(t, 1, state.Girls[id].WeaponAwake)
		assert.Zero(t, state.Gold)
		var wallet model.User
		require.NoError(t, model.DB.Select("id", "quota").First(&wallet, user.Id).Error)
		assert.Equal(t, user.Quota, wallet.Quota)
	})

	t.Run("equipment_rejects_corrupt_pieces_without_remapping_old_saves", func(t *testing.T) {
		user := gachaUser(t)
		state := gachaState(t, user.Id, now)
		type gear = model.GachaEquipmentPiece
		for _, invalid := range []model.GachaOwnedGirl{
			{Equipment: map[string]gear{"head": {Level: 1}}},
			{Level: 1, Star: 1, Equipment: map[string]gear{"ring": {Level: 1}}},
			{Level: 1, Star: 1, Equipment: map[string]gear{"weapon": {Level: 2}}},
			{Level: 1, Star: 1, Equipment: map[string]gear{"body": {Level: -1}}},
			{Level: 1, Star: 1, Equipment: map[string]gear{"head": {Level: 31}}},
			{Level: 1, Star: 1, Equipment: map[string]gear{"feet": {Level: 1, Star: -1}}},
			{Level: 1, Star: 2, Equipment: map[string]gear{"weapon": {Star: 1}}},
			{Level: 1, Star: 1, Equipment: map[string]gear{"hands": {Level: 2, Star: 1}}},
			{Level: 1, Star: 5, Equipment: map[string]gear{"body": {Level: 10, Star: 6}}},
		} {
			state.Girls["c28"] = &invalid
			seedGachaState(t, user.Id, state)
			_, err := model.GetGachaProfile(user.Id, now)
			require.ErrorIs(t, err, model.ErrGachaRunState)
		}
		delete(state.Girls, "c28")
		seedGachaState(t, user.Id, state)
		assert.Empty(t, gachaState(t, user.Id, now).Girls["c01"].Equipment)
	})

	t.Run("equipment_materials_follow_issued_rules_win_and_capacity", func(t *testing.T) {
		user := gachaUser(t)
		run, err := model.StartGachaRun(user.Id, "s1", uuid.NewString(), now)
		require.NoError(t, err)
		var current model.GachaRun
		require.NoError(t, model.DB.First(&current, "id = ?", run.ID).Error)
		var issued model.GachaRunSnapshot
		require.NoError(t, common.Unmarshal([]byte(current.Snapshot), &issued))
		assert.Equal(t, 9, issued.RulesVersion)
		clock := time.UnixMilli(run.ReadyAt + 1)
		out, err := model.FinishGachaRun(user.Id, run.ID, true, clock)
		require.NoError(t, err)
		ids := []string{"gear_plate", "gear_thread", "gear_rivet"}
		for _, id := range ids {
			assert.Zero(t, out.Save.Mats[id])
		}
		issue := func(stage string, version int, win bool) string {
			snapshot, err := common.Marshal(model.GachaRunSnapshot{Version: 1, Squad: []string{"c01"}, RulesVersion: version, MultiUsed: true})
			require.NoError(t, err)
			report, err := common.Marshal(model.GachaBattleReport{Win: win, Stars: 3, Rounds: 1, TotalN: 1, AliveN: 1, MaterialBonus: 0.2})
			require.NoError(t, err)
			record := model.GachaRun{ID: uuid.NewString(), UserID: user.Id, Stage: stage, Day: clock.Format("2006-01-02"), Status: "running", StartedAt: clock.UnixMilli() - 1,
				ReadyAt: clock.UnixMilli(), ExpireAt: clock.Add(time.Hour).UnixMilli(), Snapshot: model.GachaDocument(snapshot), BattleReport: model.GachaDocument(report)}
			require.NoError(t, model.DB.Create(&record).Error)
			return record.ID
		}
		for _, test := range []struct {
			stage string
			rules int
			win   bool
			delta [3]int
		}{
			{"s3", 4, true, [3]int{}}, {"s3", 5, false, [3]int{}}, {"s1", 5, true, [3]int{2, 1, 0}},
			{"s3", 5, true, [3]int{3, 2, 1}}, {"r1", 5, true, [3]int{1, 4, 2}}, {"n1", 5, true, [3]int{2, 6, 3}},
			{"s3", issued.RulesVersion, true, [3]int{3, 2, 1}},
		} {
			before := gachaState(t, user.Id, clock).Mats
			runID := issue(test.stage, test.rules, test.win)
			out, err = model.FinishGachaRun(user.Id, runID, false, clock)
			require.NoError(t, err)
			saved := out.Save
			for index, id := range ids {
				assert.Equal(t, before[id]+test.delta[index], saved.Mats[id])
			}
			replay, err := model.FinishGachaRun(user.Id, runID, false, clock)
			require.NoError(t, err)
			assert.Equal(t, out.Revision, replay.Revision)
			assert.Equal(t, out.Save.Mats, replay.Save.Mats)
		}
		state := gachaState(t, user.Id, clock)
		state.Mats["gear_rivet"] = 1_000_000_000
		seedGachaState(t, user.Id, state)
		blocked := issue("s3", 5, true)
		_, err = model.FinishGachaRun(user.Id, blocked, false, clock)
		require.ErrorIs(t, err, model.ErrGachaCapacity)
		assert.Equal(t, state.Mats, gachaState(t, user.Id, clock).Mats)
		state.Mats["gear_rivet"]--
		seedGachaState(t, user.Id, state)
		out, err = model.FinishGachaRun(user.Id, blocked, false, clock)
		require.NoError(t, err)
		assert.Equal(t, 1_000_000_000, out.Save.Mats["gear_rivet"])
	})
}

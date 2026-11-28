package model_test

import (
	"math"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cultivationIntent(action, girlID string) model.GachaActionIntent {
	return model.GachaActionIntent{RequestID: uuid.NewString(), Action: action, GirlID: &girlID}
}

func TestGachaCultivationIntentFields(t *testing.T) {
	choice, count := "edge", 2
	for _, action := range []string{"weapon_upgrade", "weapon_awaken", "potential_unlock", "use_manual", "weapon_engrave", "iv_roll"} {
		intent := cultivationIntent(action, "c01")
		if action == "weapon_engrave" {
			assert.ErrorIs(t, model.ValidateGachaAction(intent), model.ErrGachaIntent)
			intent.Choice = &choice
		}
		require.NoError(t, model.ValidateGachaAction(intent))
		bad := intent
		bad.Count = &count
		assert.ErrorIs(t, model.ValidateGachaAction(bad), model.ErrGachaIntent)
		bad = intent
		bad.ItemID = &choice
		assert.ErrorIs(t, model.ValidateGachaAction(bad), model.ErrGachaIntent)
		bad = intent
		bad.GirlID = nil
		assert.ErrorIs(t, model.ValidateGachaAction(bad), model.ErrGachaIntent)
		bad = intent
		unknown := "unknown"
		bad.Choice = &unknown
		assert.ErrorIs(t, model.ValidateGachaAction(bad), model.ErrGachaIntent)
	}
}

func TestGachaIndividualLockIntent(t *testing.T) {
	enabled, stat, unknown := false, "hp", "power"
	lock := cultivationIntent("iv_lock", "c01")
	lock.Choice = &stat
	assert.ErrorIs(t, model.ValidateGachaAction(lock), model.ErrGachaIntent)
	lock.Enabled = &enabled
	require.NoError(t, model.ValidateGachaAction(lock))
	lock.Choice = &unknown
	assert.ErrorIs(t, model.ValidateGachaAction(lock), model.ErrGachaIntent)
	lock.Choice = nil
	assert.ErrorIs(t, model.ValidateGachaAction(lock), model.ErrGachaIntent)
	roll := cultivationIntent("iv_roll", "c01")
	roll.Enabled = &enabled
	assert.ErrorIs(t, model.ValidateGachaAction(roll), model.ErrGachaIntent)
}

func TestGachaIndividualBonusesInBothCombatModesAndPower(t *testing.T) {
	const id = "c01"
	card := &model.GachaOwnedGirl{Star: 1, Level: 1}
	state := &model.GachaGameState{Squad: []string{id}, Girls: map[string]*model.GachaOwnedGirl{id: card}}
	stage := model.GachaStage{Waves: 1, EnemyCount: 1, EnemyLv: 1, StarRound: 5}
	base, err := model.SimulateGachaCombat(state, stage)
	require.NoError(t, err)
	unit := base.Allies[0]
	card.IVValues = []int{4, 4, 4, 7, 7, 3}
	threshold, err := model.SimulateGachaCombat(state, stage)
	require.NoError(t, err)
	assert.Equal(t, unit, threshold.Allies[0])
	card.IVValues = []int{31, 19, 10, 16, 24, 31}
	card.Chips = []string{"edge_1"}
	rolled, err := model.SimulateGachaCombat(state, stage)
	require.NoError(t, err)
	arena, err := model.SimulateGachaArena(state, state)
	require.NoError(t, err)
	for _, trained := range []model.GachaBattleUnit{rolled.Allies[0], arena.Allies[0], arena.Waves[0][0]} {
		assert.Equal(t, int(math.Round(float64(unit.MaxHP)*1.06)), trained.MaxHP)
		assert.Equal(t, int(math.Round(float64(unit.Atk-3)*1.04))+3+5, trained.Atk)
		assert.Equal(t, int(math.Round(float64(unit.Def)*1.02)), trained.Def)
		assert.Equal(t, unit.Spd+2, trained.Spd)
		assert.Equal(t, unit.CR+3, trained.CR)
		assert.Equal(t, unit.CD+7, trained.CD)
	}
	trained := rolled.Allies[0]
	expected := float64(trained.Atk)*2.2*(1+float64(trained.CR*trained.CD)/10000) + float64(trained.MaxHP)*0.35 + float64(trained.Def)*1.6 + float64(trained.Spd)*3 + float64(trained.EHR+trained.Res)*2
	power, err := model.GachaSquadPower(state)
	require.NoError(t, err)
	assert.Equal(t, int(math.Round(expected)), power)
}

func runGachaCultivationMatrix(t *testing.T, now time.Time) {
	t.Run("cipher_confirmed_attempt_is_idempotent_and_cannot_cross_days", func(t *testing.T) {
		user := gachaUser(t)
		initial := gachaState(t, user.Id, now)
		require.NotNil(t, initial.Cipher)
		tag, guess, firstStep := initial.Cipher.Day+":0", "0123", 1
		intent := model.GachaActionIntent{Action: "cipher_guess", RequestID: uuid.NewString(), ItemID: &tag, Choice: &guess, Count: &firstStep}
		first, err := model.ApplyGachaAction(user.Id, intent, now)
		require.NoError(t, err)
		require.Len(t, first.Save.Cipher.Attempts, 1)
		assert.Equal(t, guess, first.Save.Cipher.Attempts[0].Code)
		reward := 0
		if first.Save.Cipher.Solved {
			reward = 1
		}
		assert.Equal(t, initial.Mats["training_ink"]+2*reward, first.Save.Mats["training_ink"])
		assert.Equal(t, initial.Mats["forge_ore"]+reward, first.Save.Mats["forge_ore"])
		replay, err := model.ApplyGachaAction(user.Id, intent, now)
		require.NoError(t, err)
		assert.Equal(t, first, replay)
		stale := intent
		different := "3254"
		stale.Choice = &different
		_, err = model.ApplyGachaAction(user.Id, stale, now)
		require.ErrorIs(t, err, model.ErrGachaReplay)
		stale.RequestID = uuid.NewString()
		_, err = model.ApplyGachaAction(user.Id, stale, now)
		require.ErrorIs(t, err, model.ErrGachaCultivationLocked)
		nextDay := now.Add(24 * time.Hour)
		next := gachaState(t, user.Id, nextDay)
		assert.NotEqual(t, initial.Cipher.Day, next.Cipher.Day)
		assert.Empty(t, next.Cipher.Attempts)
		_, err = model.ApplyGachaAction(user.Id, stale, nextDay)
		require.ErrorIs(t, err, model.ErrGachaCultivationLocked)
		assert.Equal(t, first.Save.Mats["training_ink"], next.Mats["training_ink"])
	})
	t.Run("picked_books_and_oaths_are_idempotent_server_spends", func(t *testing.T) {
		user := gachaUser(t)
		state := gachaState(t, user.Id, now)
		state.Captain.Level = 60 // Feeding requires room below the captain's cap.
		state.Mats["field_notes"], state.Mats["break_seal"] = 2, 10
		seedGachaState(t, user.Id, state)
		feed := cultivationIntent("card_feed", "c01")
		feed.Materials = map[string]int{"field_notes": 2}
		out, err := model.ApplyGachaAction(user.Id, feed, now)
		require.NoError(t, err)
		require.Equal(t, 2400, out.Feed.Experience)
		replay, err := model.ApplyGachaAction(user.Id, feed, now)
		require.NoError(t, err)
		require.Equal(t, out, replay)
		feed.Materials["field_notes"] = 1
		_, err = model.ApplyGachaAction(user.Id, feed, now)
		require.ErrorIs(t, err, model.ErrGachaReplay)
		bind := cultivationIntent("vigil_bind", "c01")
		choice := "ward"
		bind.Choice = &choice
		out, err = model.ApplyGachaAction(user.Id, bind, now)
		require.NoError(t, err)
		require.Equal(t, "ward", out.Save.Girls["c01"].Vigil.Path)
		replay, err = model.ApplyGachaAction(user.Id, bind, now)
		require.NoError(t, err)
		require.Equal(t, out, replay)
		_, err = model.ApplyGachaAction(user.Id, cultivationIntent("vigil_ascend", "c01"), now)
		require.ErrorIs(t, err, model.ErrGachaCultivationLocked)
		assert.Equal(t, 10, gachaState(t, user.Id, now).Mats["break_seal"])
	})
	runGachaArtifactMatrix(t, now)
	runGachaEquipmentMatrix(t, now)
	runGachaRefinementMatrix(t, now)
	runGachaCarryMatrix(t, now)
	t.Run("training_uses_one_account_pool_and_replays_without_double_spend", func(t *testing.T) {
		user := gachaUser(t)
		profile, err := model.GetGachaProfile(user.Id, now)
		require.NoError(t, err)
		require.NotNil(t, profile.Training)
		state := profile.Save
		require.Equal(t, 20, state.SkillPool.Points)
		state.SkillPool, state.Daily.SkillSpent = nil, 3
		seedGachaState(t, user.Id, state)
		state = gachaState(t, user.Id, now)
		assert.Equal(t, 17, state.SkillPool.Points)
		assert.Zero(t, state.Daily.SkillSpent)
		assert.Equal(t, now.UnixMilli(), state.SkillPool.TickAt)
		state.Girls["c01"].Level, state.Girls["c08"].Level = 30, 30
		pool := profile.Training.SkillPool
		secondLevel := pool.UpgradeGoldBase + pool.UpgradeGoldStep
		levelsTwoToFive := 4*pool.UpgradeGoldBase + 10*pool.UpgradeGoldStep
		state.Gold, state.SkillPool.Points = 2*pool.UpgradeGoldBase+secondLevel+5*pool.BuyGold+levelsTwoToFive+1, 2
		seedGachaState(t, user.Id, state)
		upgrade := func(action, id, choice string) model.GachaActionIntent {
			intent := cultivationIntent(action, id)
			intent.Choice = &choice
			return intent
		}
		single := upgrade("skill_point", "c01", "normal")
		first, err := model.ApplyGachaAction(user.Id, single, now)
		require.NoError(t, err)
		assert.Equal(t, 1, first.Save.SkillPool.Points)
		assert.Equal(t, state.Gold-pool.UpgradeGoldBase, first.Save.Gold)
		replay, err := model.ApplyGachaAction(user.Id, single, now.Add(19*time.Second))
		require.NoError(t, err)
		assert.Equal(t, first, replay)
		second, err := model.ApplyGachaAction(user.Id, upgrade("skill_auto", "c08", "rage"), now)
		require.NoError(t, err)
		assert.Zero(t, second.Save.SkillPool.Points)
		assert.Equal(t, first.Save.Gold-pool.UpgradeGoldBase, second.Save.Gold)
		_, err = model.ApplyGachaAction(user.Id, upgrade("skill_point", "c01", "normal"), now.Add(19999*time.Millisecond))
		require.ErrorIs(t, err, model.ErrGachaFunds)
		tick := now.Add(20 * time.Second)
		next, err := model.ApplyGachaAction(user.Id, upgrade("skill_point", "c01", "normal"), tick)
		require.NoError(t, err)
		assert.Equal(t, second.Save.Gold-secondLevel, next.Save.Gold)
		assert.Equal(t, tick.UnixMilli(), next.Save.SkillPool.TickAt)
		five := 5
		buy := model.GachaActionIntent{Action: "skill_buy", Count: &five, RequestID: uuid.NewString()}
		bought, err := model.ApplyGachaAction(user.Id, buy, tick)
		require.NoError(t, err)
		assert.Equal(t, next.Save.Gold-5*pool.BuyGold, bought.Save.Gold)
		assert.Equal(t, 5, bought.Save.SkillPool.Points)
		receipt, err := model.ApplyGachaAction(user.Id, buy, tick)
		require.NoError(t, err)
		assert.Equal(t, bought, receipt)
		other := buy
		one := 1
		other.Count = &one
		_, err = model.ApplyGachaAction(user.Id, other, tick)
		require.ErrorIs(t, err, model.ErrGachaReplay)
		up, err := model.ApplyGachaAction(user.Id, upgrade("skill_auto", "c08", "rage"), tick)
		require.NoError(t, err)
		assert.Equal(t, 1, up.Save.Gold)
		assert.Equal(t, 1, up.Save.SkillPool.Points)
		assert.Equal(t, 5, up.Save.Girls["c08"].Training.Skills["rage"])
		two := 2
		other.Count, other.RequestID = &two, uuid.NewString()
		_, err = model.ApplyGachaAction(user.Id, other, tick)
		require.ErrorIs(t, err, model.ErrGachaFunds)
		later := gachaState(t, user.Id, tick.Add(24*time.Hour))
		assert.Equal(t, 20, later.SkillPool.Points)
		assert.Equal(t, tick.Add(24*time.Hour).UnixMilli(), later.SkillPool.TickAt)
		_, err = model.ApplyGachaAction(user.Id, other, tick.Add(24*time.Hour))
		require.ErrorIs(t, err, model.ErrGachaCapacity)
	})
	t.Run("cultivation_individual_reroll_prices_locks_and_replays_once", func(t *testing.T) {
		user := gachaUser(t)
		id := "c01"
		profile, err := model.GetGachaProfile(user.Id, now)
		require.NoError(t, err)
		state := profile.Save
		prices := profile.Cultivation.Individual.LockTokens
		state.Squad = []string{"c08"}
		state.Mats["iv_solvent"], state.Gold, state.Token = 3, 480, prices[2]+prices[5]
		state.Girls[id].Chips, state.Girls[id].ChipLocks = []string{"edge_1"}, 1
		seedGachaState(t, user.Id, state)
		lock := func(stat string, enabled bool) (*model.GachaActionResult, error) {
			intent := cultivationIntent("iv_lock", id)
			intent.Choice, intent.Enabled = &stat, &enabled
			return model.ApplyGachaAction(user.Id, intent, now)
		}
		_, err = lock("hp", true)
		require.ErrorIs(t, err, model.ErrGachaCultivationLocked)
		_, err = model.ApplyGachaAction(user.Id, cultivationIntent("iv_roll", "c04"), now)
		require.ErrorIs(t, err, model.ErrGachaIntent)
		first := cultivationIntent("iv_roll", id)
		out, err := model.ApplyGachaAction(user.Id, first, now)
		require.NoError(t, err)
		initial := out.Save.Girls[id].IVValues
		require.Len(t, initial, 6)
		for _, value := range initial {
			assert.GreaterOrEqual(t, value, 0)
			assert.LessOrEqual(t, value, 31)
		}
		assert.Equal(t, 2, out.Save.Mats["iv_solvent"])
		assert.Equal(t, 320, out.Save.Gold)
		assert.Equal(t, state.Token, out.Save.Token)
		replayed, err := model.ApplyGachaAction(user.Id, first, now)
		require.NoError(t, err)
		assert.Equal(t, out, replayed)
		for _, stat := range []string{"hp", "cd"} {
			_, err = lock(stat, true)
			require.NoError(t, err)
		}
		out, err = model.ApplyGachaAction(user.Id, cultivationIntent("iv_roll", id), now)
		require.NoError(t, err)
		assert.Equal(t, 1, out.Save.Mats["iv_solvent"])
		assert.Equal(t, 160, out.Save.Gold)
		assert.Equal(t, prices[5], out.Save.Token)
		assert.Equal(t, 33, out.Save.Girls[id].IVLocks)
		assert.Equal(t, initial[0], out.Save.Girls[id].IVValues[0])
		assert.Equal(t, initial[5], out.Save.Girls[id].IVValues[5])
		for _, stat := range []string{"atk", "def", "spd", "cr"} {
			_, err = lock(stat, true)
			require.NoError(t, err)
		}
		_, err = model.ApplyGachaAction(user.Id, cultivationIntent("iv_roll", id), now)
		require.ErrorIs(t, err, model.ErrGachaCultivationLocked)
		out, err = lock("atk", false)
		require.NoError(t, err)
		previous := out.Save.Girls[id].IVValues
		assert.Equal(t, 61, out.Save.Girls[id].IVLocks)
		out, err = model.ApplyGachaAction(user.Id, cultivationIntent("iv_roll", id), now)
		require.NoError(t, err)
		assert.Zero(t, out.Save.Mats["iv_solvent"])
		assert.Zero(t, out.Save.Token)
		assert.Equal(t, 1, out.Save.Girls[id].ChipLocks)
		for index, value := range out.Save.Girls[id].IVValues {
			if index != 1 {
				assert.Equal(t, previous[index], value)
			}
		}
		_, err = model.ApplyGachaAction(user.Id, cultivationIntent("iv_roll", id), now)
		require.ErrorIs(t, err, model.ErrGachaFunds)
		state = gachaState(t, user.Id, now)
		assert.Equal(t, out.Save.Girls[id].IVValues, state.Girls[id].IVValues)
		assert.Zero(t, state.Gold)
		var wallet model.User
		require.NoError(t, model.DB.Select("id", "quota").First(&wallet, user.Id).Error)
		assert.Equal(t, user.Quota, wallet.Quota)
	})
	t.Run("cultivation_fixed_weapon_costs_gates_and_replay", func(t *testing.T) {
		user := gachaUser(t)
		state := gachaState(t, user.Id, now)
		id := "c01"
		own := state.Girls[id]
		own.Level, own.Star, own.Awakening, own.Branch = 60, 5, 1, model.GachaAwakeningBranches(id)[0].ID
		state.Gold, state.Mats["forge_ore"], state.Mats["weapon_core"] = 0, 3, 9
		seedGachaState(t, user.Id, state)
		apply := func(action string) (*model.GachaActionResult, error) {
			return model.ApplyGachaAction(user.Id, cultivationIntent(action, id), now)
		}
		engrave := func(choice string) (*model.GachaActionResult, error) {
			intent := cultivationIntent("weapon_engrave", id)
			intent.Choice = &choice
			return model.ApplyGachaAction(user.Id, intent, now)
		}
		_, err := apply("weapon_awaken")
		require.ErrorIs(t, err, model.ErrGachaCultivationLocked)
		_, err = engrave("edge")
		require.ErrorIs(t, err, model.ErrGachaCultivationLocked)
		_, err = model.ApplyGachaAction(user.Id, cultivationIntent("weapon_upgrade", "c04"), now)
		require.ErrorIs(t, err, model.ErrGachaIntent)
		intents := []model.GachaActionIntent{cultivationIntent("weapon_upgrade", id), cultivationIntent("weapon_upgrade", id)}
		var group sync.WaitGroup
		var results [2]*model.GachaActionResult
		var failures [2]error
		for index := range intents {
			group.Go(func() { results[index], failures[index] = model.ApplyGachaAction(user.Id, intents[index], now) })
		}
		group.Wait()
		successes := 0
		for index, failure := range failures {
			if failure != nil {
				require.ErrorIs(t, failure, model.ErrGachaFunds)
				continue
			}
			successes++
			replay, err := model.ApplyGachaAction(user.Id, intents[index], now)
			require.NoError(t, err)
			assert.Equal(t, results[index], replay)
			assert.Equal(t, 1, replay.Save.Girls[id].WeaponLevel)
			assert.Zero(t, replay.Save.Mats["forge_ore"])
		}
		require.Equal(t, 1, successes)
		state = gachaState(t, user.Id, now)
		state.Mats["forge_ore"] = 88
		seedGachaState(t, user.Id, state)
		oreLeft := 88
		for level := 2; level <= 10; level++ {
			out, err := apply("weapon_upgrade")
			require.NoError(t, err)
			oreLeft -= level + 2
			assert.Equal(t, oreLeft, out.Save.Mats["forge_ore"])
			assert.Equal(t, level, out.Save.Girls[id].WeaponLevel)
			if level == 3 {
				for _, rune := range []string{"edge", "tempo"} {
					out, err = engrave(rune)
					require.NoError(t, err)
					oreLeft -= 8
					assert.Equal(t, oreLeft, out.Save.Mats["forge_ore"])
					assert.Equal(t, rune, out.Save.Girls[id].WeaponRune)
					_, err = engrave(rune)
					require.ErrorIs(t, err, model.ErrGachaCultivationLocked)
				}
			}
			if level == 5 {
				out, err = apply("weapon_awaken")
				require.NoError(t, err)
				assert.Equal(t, 1, out.Save.Girls[id].WeaponAwake)
				assert.Equal(t, 6, out.Save.Mats["weapon_core"])
			}
		}
		_, err = apply("weapon_upgrade")
		require.ErrorIs(t, err, model.ErrGachaCultivationLocked)
		_, err = apply("weapon_awaken")
		require.ErrorIs(t, err, model.ErrGachaCultivationLocked)
		state = gachaState(t, user.Id, now)
		assert.Zero(t, state.Mats["forge_ore"])
		state.Girls[id].Awakening = 3
		seedGachaState(t, user.Id, state)
		out, err := apply("weapon_awaken")
		require.NoError(t, err)
		assert.Equal(t, 2, out.Save.Girls[id].WeaponAwake)
		assert.Equal(t, "tempo", out.Save.Girls[id].WeaponRune)
		assert.Zero(t, out.Save.Mats["weapon_core"])
		assert.Zero(t, out.Save.Gold)
		_, err = apply("weapon_awaken")
		require.ErrorIs(t, err, model.ErrGachaCultivationLocked)
	})

	t.Run("cultivation_advanced_stars_reserve_shards_and_unlock_potential", func(t *testing.T) {
		user := gachaUser(t)
		state := gachaState(t, user.Id, now)
		id := "c01"
		own := state.Girls[id]
		own.Level, own.Star, own.Shards = 60, 5, 110
		state.Mats["mat_r"], state.Mats["shard_dust"] = 1000, 2000
		seedGachaState(t, user.Id, state)
		apply := func(action string) (*model.GachaActionResult, error) {
			return model.ApplyGachaAction(user.Id, cultivationIntent(action, id), now)
		}
		decompose := func(count int) (*model.GachaActionResult, error) {
			intent := cultivationIntent("shard_decompose", id)
			intent.Count = &count
			return model.ApplyGachaAction(user.Id, intent, now)
		}
		_, err := decompose(20)
		require.ErrorIs(t, err, model.ErrGachaFunds)
		_, err = decompose(9)
		require.ErrorIs(t, err, model.ErrGachaIntent)
		out, err := decompose(10)
		require.NoError(t, err)
		assert.Equal(t, 100, out.Save.Girls[id].Shards)
		assert.Equal(t, 1, out.Save.Mats["forge_ore"])
		for node, cost := range []int{4, 8} {
			before := out.Save.Mats["mat_r"]
			out, err = apply("potential_unlock")
			require.NoError(t, err)
			assert.Equal(t, node+1, out.Save.Girls[id].Potential)
			assert.Equal(t, before-cost, out.Save.Mats["mat_r"])
		}
		_, err = apply("potential_unlock")
		require.ErrorIs(t, err, model.ErrGachaCultivationLocked)
		for tier, shards := range []int{100, 140, 180, 220, 260, 300, 360} {
			if tier < 3 {
				_, err = apply("star")
				require.ErrorIs(t, err, model.ErrGachaCultivationLocked)
				state = gachaState(t, user.Id, now)
				state.Girls[id].Awakening, state.Girls[id].Branch = tier+1, model.GachaAwakeningBranches(id)[0].ID
				seedGachaState(t, user.Id, state)
			}
			state = gachaState(t, user.Id, now)
			if tier > 0 {
				_, err = apply("star")
				require.ErrorIs(t, err, model.ErrGachaFunds, "dust cannot replace even one personal shard")
				state.Girls[id].Shards = shards
				seedGachaState(t, user.Id, state)
			}
			before := state
			out, err = apply("star")
			require.NoError(t, err)
			assert.Equal(t, 6+tier, out.Save.Girls[id].Star)
			assert.Equal(t, before.Mats["mat_r"]-20*(tier+2), out.Save.Mats["mat_r"])
			assert.Zero(t, out.Save.Girls[id].Shards)
			assert.Equal(t, 2000, out.Save.Mats["shard_dust"])
			if cost := map[int]int{0: 14, 2: 18, 4: 24, 6: 32}[tier]; cost > 0 {
				beforeMat := out.Save.Mats["mat_r"]
				out, err = apply("potential_unlock")
				require.NoError(t, err)
				assert.Equal(t, beforeMat-cost, out.Save.Mats["mat_r"])
			}
		}
		assert.Equal(t, 12, out.Save.Girls[id].Star)
		assert.Equal(t, 6, out.Save.Girls[id].Potential)
		for _, action := range []string{"star", "potential_unlock"} {
			_, err = apply(action)
			require.ErrorIs(t, err, model.ErrGachaCultivationLocked)
		}
		assert.Equal(t, out.Save.Mats, gachaState(t, user.Id, now).Mats)
	})

	t.Run("cultivation_training_manual_for_one_owned_card", func(t *testing.T) {
		user := gachaUser(t)
		state := gachaState(t, user.Id, now)
		state.Captain.Level = 4
		state.Squad = []string{"c08"}
		state.Mats["training_manual"] = 2
		seedGachaState(t, user.Id, state)
		intent := cultivationIntent("use_manual", "c01")
		out, err := model.ApplyGachaAction(user.Id, intent, now)
		require.NoError(t, err)
		assert.Equal(t, 4, out.Save.Girls["c01"].Level)
		assert.Zero(t, out.Save.Girls["c01"].Exp)
		assert.Equal(t, 1, out.Save.Girls["c08"].Level)
		assert.Equal(t, 1, out.Save.Mats["training_manual"])
		replay, err := model.ApplyGachaAction(user.Id, intent, now)
		require.NoError(t, err)
		assert.Equal(t, out, replay)
		state = gachaState(t, user.Id, now)
		state.Girls["c01"].Level = 150
		seedGachaState(t, user.Id, state)
		_, err = model.ApplyGachaAction(user.Id, cultivationIntent("use_manual", "c01"), now)
		require.ErrorIs(t, err, model.ErrGachaCultivationLocked)
		assert.Equal(t, 1, gachaState(t, user.Id, now).Mats["training_manual"])
	})

	t.Run("cultivation_invalid_card_fields_rejected_on_load", func(t *testing.T) {
		user := gachaUser(t)
		state := gachaState(t, user.Id, now)
		for _, invalid := range []model.GachaOwnedGirl{
			{WeaponLevel: 1}, {Level: 1, Star: 1, WeaponLevel: -1}, {Level: 1, Star: 1, WeaponLevel: 31},
			{Level: 60, Star: 13}, {Level: 60, Star: 9}, {Level: 1, Star: 1, Potential: 1}, {Level: 60, Star: 5, Potential: 7},
			{Level: 60, Star: 5, WeaponAwake: 1}, {Level: 60, Star: 5, WeaponLevel: 10, WeaponAwake: 3},
			{Level: 1, Star: 1, WeaponRune: "edge"}, {Level: 1, Star: 1, WeaponLevel: 4, WeaponRune: "unknown"},
			{Level: 1, Star: 1, IVLocks: 1}, {Level: 1, Star: 1, IVValues: []int{0, 1}}, {IVValues: make([]int, 6)},
			{Level: 1, Star: 1, IVValues: []int{0, 0, 0, 0, 0, 32}}, {Level: 1, Star: 1, IVValues: []int{-1, 0, 0, 0, 0, 0}},
			{Level: 1, Star: 1, IVValues: make([]int, 6), IVLocks: 64}, {Level: 1, Star: 1, IVValues: make([]int, 6), IVLocks: -1},
		} {
			state.Girls["c28"] = &invalid
			seedGachaState(t, user.Id, state)
			_, err := model.GetGachaProfile(user.Id, now)
			require.ErrorIs(t, err, model.ErrGachaRunState)
		}
		delete(state.Girls, "c28")
		seedGachaState(t, user.Id, state)
	})

	t.Run("cultivation_only_new_winning_campaign_and_rift_runs_award_materials_once", func(t *testing.T) {
		user := gachaUser(t)
		state := gachaState(t, user.Id, now)
		strongGachaState(state)
		state.Cleared["s4"], state.Cleared["s6"] = true, true
		state.MultiOn, state.Multi = true, 9
		seedGachaState(t, user.Id, state)
		clock := now
		start := func(stage string) *model.GachaRunResult {
			run, err := model.StartGachaRun(user.Id, stage, uuid.NewString(), clock)
			require.NoError(t, err)
			clock = time.UnixMilli(run.ReadyAt + 1)
			return run
		}
		check := func(run *model.GachaRunResult, abandon, checkReplay bool, ore, bosses, gemTickets, chipTickets int) {
			out, err := model.FinishGachaRun(user.Id, run.ID, abandon, clock)
			require.NoError(t, err)
			assert.Equal(t, !abandon, out.Win)
			expected := map[string]int{"forge_ore": ore, "weapon_core": bosses, "training_manual": bosses, "gem_ticket": gemTickets, "chip_ticket": chipTickets}
			saved := out.Save
			if checkReplay {
				replay, err := model.FinishGachaRun(user.Id, run.ID, abandon, clock)
				require.NoError(t, err)
				assert.Equal(t, out, replay)
			}
			for id, amount := range expected {
				assert.Equal(t, amount, saved.Mats[id], id)
			}
			if run.Stage == "training" || abandon {
				for _, material := range []string{"gear_plate", "gear_thread", "gear_rivet"} {
					assert.Zero(t, out.Save.Mats[material])
				}
			}
		}
		asVersion := func(run *model.GachaRunResult, version int) *model.GachaRunResult {
			var record model.GachaRun
			require.NoError(t, model.DB.First(&record, "id = ?", run.ID).Error)
			var snapshot model.GachaRunSnapshot
			require.NoError(t, common.Unmarshal([]byte(record.Snapshot), &snapshot))
			snapshot.RulesVersion = version
			oldRules, err := common.Marshal(snapshot)
			require.NoError(t, err)
			require.NoError(t, model.DB.Model(&record).Update("snapshot", model.GachaDocument(oldRules)).Error)
			return run
		}
		check(start("s1"), true, false, 0, 0, 0, 0)
		check(start("training"), false, false, 0, 0, 0, 0)
		check(asVersion(start("s1"), 2), false, false, 0, 0, 0, 0)
		check(start("s1"), false, true, 1, 0, 0, 0)
		boss := start("s3")
		check(boss, false, true, 3, 1, 1, 0)
		state = gachaState(t, user.Id, clock)
		state.Mats["training_manual"] = 1_000_000_000
		seedGachaState(t, user.Id, state)
		fullStore := start("s3")
		_, err := model.FinishGachaRun(user.Id, fullStore.ID, false, clock)
		require.ErrorIs(t, err, model.ErrGachaCapacity)
		state = gachaState(t, user.Id, clock)
		assert.Equal(t, 3, state.Mats["forge_ore"])
		assert.Equal(t, 1, state.Mats["weapon_core"])
		assert.Equal(t, 1_000_000_000, state.Mats["training_manual"])
		assert.Equal(t, 1, state.Mats["gem_ticket"])
		state.Mats["training_manual"] = 1
		seedGachaState(t, user.Id, state)
		paid, err := model.FinishGachaRun(user.Id, fullStore.ID, false, clock)
		require.NoError(t, err)
		require.True(t, paid.Win)
		for id, amount := range map[string]int{"forge_ore": 5, "weapon_core": 2, "training_manual": 2, "gem_ticket": 2, "chip_ticket": 0} {
			assert.Equal(t, amount, paid.Save.Mats[id], id)
		}
		replay, err := model.FinishGachaRun(user.Id, fullStore.ID, false, clock)
		require.NoError(t, err)
		assert.Equal(t, paid, replay)
		check(start("r1"), false, true, 8, 3, 2, 1)
		check(asVersion(start("s3"), 3), false, false, 10, 4, 2, 1)
	})
}

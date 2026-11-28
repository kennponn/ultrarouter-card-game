package model_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runGachaMarketMatrix(t *testing.T, clock time.Time) {
	zone := time.FixedZone("Asia/Shanghai", 8*3600)
	local := clock.In(zone)
	day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, zone)
	intent := func(action, item string) model.GachaActionIntent {
		return model.GachaActionIntent{RequestID: uuid.NewString(), Action: action, ItemID: &item}
	}
	t.Run("stock_changes_at_09_14_21_not_midnight", func(t *testing.T) {
		user := gachaUser(t)
		start := day.Add(-2 * time.Hour)
		state := gachaState(t, user.Id, start)
		state.Token = 600
		seedGachaState(t, user.Id, state)
		for _, hour := range []int{9, 14, 21} {
			boundary := day.Add(time.Duration(hour) * time.Hour)
			bought, err := model.ApplyGachaAction(user.Id, intent("shop", "multi"), start)
			require.NoError(t, err)
			assert.Equal(t, boundary.UnixMilli(), bought.Save.Market.Next)
			_, err = model.ApplyGachaAction(user.Id, intent("shop", "multi"), boundary.Add(-time.Millisecond))
			require.ErrorIs(t, err, model.ErrGachaShopLimit)
			fresh := gachaState(t, user.Id, boundary)
			assert.Empty(t, fresh.Market.Bought)
			assert.Equal(t, boundary.UnixMilli(), fresh.Market.Slot)
			assert.Equal(t, bought.Save.Token, fresh.Token, "timed restock is free")
			start = boundary
		}
	})
	t.Run("natural_stamina_timer_after_packs_and_direct_purchases", func(t *testing.T) {
		for _, sample := range []struct {
			action, id string
			start, end int
		}{
			{"shop", "stamina_supply", 90, 120}, {"shop", "stamina_supply", 45, 75},
			{"use_stamina_pack", "stamina_pack_60", 90, 150}, {"use_stamina_pack", "stamina_pack_30", 30, 60},
		} {
			user := gachaUser(t)
			state := gachaState(t, user.Id, local)
			anchor := local.Add(-2 * time.Minute).UnixMilli()
			state.Stamina, state.StaminaTs, state.Token = sample.start, anchor, 100
			if sample.action == "use_stamina_pack" {
				state.Mats[sample.id] = 1
			}
			seedGachaState(t, user.Id, state)
			out, err := model.ApplyGachaAction(user.Id, intent(sample.action, sample.id), local)
			require.NoError(t, err)
			assert.Equal(t, sample.end, out.Save.Stamina)
			if sample.end >= state.StaminaMax {
				assert.Equal(t, local.UnixMilli(), out.Save.StaminaTs)
			} else {
				assert.Equal(t, anchor, out.Save.StaminaTs)
			}
		}
	})
	t.Run("personal_currencies_packs_and_paid_restock", func(t *testing.T) {
		user, other := gachaUser(t), gachaUser(t)
		state := gachaState(t, user.Id, local)
		state.Token, state.Mats["arena_coin"], state.Mats["dungeon_coin"] = 100, 1000, 100
		seedGachaState(t, user.Id, state)
		profile, err := model.GetGachaProfile(user.Id, local)
		require.NoError(t, err)
		require.NotNil(t, profile.Market)
		for _, offer := range profile.Market.Offers {
			if value := profile.Market.Packs[offer.Resource]; value > 0 && offer.Shop == "dungeon" {
				for _, key := range profile.Voyage.Resources {
					point := profile.Voyage.Stages[key]
					for _, prize := range point.Prize {
						if prize.Item == "dungeon_coin" {
							assert.Less(t, value*prize.Amount, offer.Price*point.Cost)
						}
					}
				}
			}
		}
		balance := 1000
		for _, spec := range []struct {
			item        string
			value, cost int
		}{{"stamina_pack_30", 30, 32}, {"stamina_pack_60", 60, 62}, {"stamina_pack_120", 120, 120}} {
			offer := "arena_pack_" + strconv.Itoa(spec.value)
			command := intent("shop", offer)
			out, err := model.ApplyGachaAction(user.Id, command, local)
			require.NoError(t, err)
			balance -= spec.cost
			assert.Equal(t, balance, out.Save.Mats["arena_coin"])
			assert.Equal(t, 100, out.Save.Token)
			assert.Equal(t, 1, out.Save.Mats[spec.item])
			replay, err := model.ApplyGachaAction(user.Id, command, local)
			require.NoError(t, err)
			assert.Equal(t, out.Revision, replay.Revision)
			opened, err := model.ApplyGachaAction(user.Id, intent("use_stamina_pack", spec.item), local)
			require.NoError(t, err)
			assert.Equal(t, out.Save.Stamina+spec.value, opened.Save.Stamina)
			assert.Zero(t, opened.Save.Mats[spec.item])
			_, err = model.ApplyGachaAction(user.Id, intent("use_stamina_pack", spec.item), local)
			require.ErrorIs(t, err, model.ErrGachaFunds)
		}
		_, err = model.ApplyGachaAction(user.Id, intent("shop", "arena_pack_120"), local)
		require.ErrorIs(t, err, model.ErrGachaShopLimit)
		poor := gachaState(t, other.Id, local)
		assert.Empty(t, poor.Market.Bought)
		_, err = model.ApplyGachaAction(other.Id, intent("shop", "arena_pack_120"), local)
		require.ErrorIs(t, err, model.ErrGachaFunds, "another player has fresh stock but not the currency")
		before, err := model.ApplyGachaAction(user.Id, intent("shop", "dungeon_wash"), local)
		require.NoError(t, err)
		refresh := intent("shop_refresh", "arena")
		period := strconv.FormatInt(before.Save.Market.Slot, 10)
		refresh.Choice = &period
		restock, err := model.ApplyGachaAction(user.Id, refresh, local)
		require.NoError(t, err)
		assert.Equal(t, before.Save.Token-profile.Market.RefreshPrice, restock.Save.Token)
		assert.Equal(t, map[string]int{"dungeon_wash": 1}, restock.Save.Market.Bought)
		assert.Equal(t, before.Save.Mats["arena_coin"], restock.Save.Mats["arena_coin"])
		retry, err := model.ApplyGachaAction(user.Id, refresh, local)
		require.NoError(t, err)
		assert.Equal(t, restock.Revision, retry.Revision)
		refresh.RequestID = uuid.NewString()
		_, err = model.ApplyGachaAction(user.Id, refresh, local)
		require.ErrorIs(t, err, model.ErrGachaShopLimit, "a full shelf cannot charge twice")
		dungeon := "dungeon"
		refresh.ItemID = &dungeon
		_, err = model.ApplyGachaAction(user.Id, refresh, time.UnixMilli(restock.Save.Market.Next))
		require.ErrorIs(t, err, model.ErrGachaShopLimit, "a stale confirmation cannot charge after a free reset")
	})
	t.Run("carry_stone_advancement_is_per_copy", func(t *testing.T) {
		user := gachaUser(t)
		state := gachaState(t, user.Id, local)
		state.Mats["arena_coin"] = 300
		state.Girls["c01"].Level = 50
		state.CarryInventory["keepsake_lens"] = 1
		seedGachaState(t, user.Id, state)
		for _, item := range []string{"arena_lens", "arena_stone_blue"} {
			_, err := model.ApplyGachaAction(user.Id, intent("shop", item), local)
			require.NoError(t, err)
		}
		equip := intent("carry_equip", "keepsake_lens")
		hero, partner := "c01", "c08"
		equip.GirlID = &hero
		worn, err := model.ApplyGachaAction(user.Id, equip, local)
		require.NoError(t, err)
		require.Equal(t, 1, worn.Save.CarryInventory["keepsake_lens"])
		upgrade := intent("carry_enhance", "carry_stone_blue")
		upgrade.GirlID = &hero
		up, err := model.ApplyGachaAction(user.Id, upgrade, local)
		require.NoError(t, err)
		assert.Equal(t, 60, up.Save.Girls[hero].CarryXP)
		assert.Greater(t, up.Save.Power, worn.Save.Power)
		retry, err := model.ApplyGachaAction(user.Id, upgrade, local)
		require.NoError(t, err)
		assert.Equal(t, up.Revision, retry.Revision)
		upgrade.RequestID = uuid.NewString()
		_, err = model.ApplyGachaAction(user.Id, upgrade, local)
		require.ErrorIs(t, err, model.ErrGachaFunds)
		removed, err := model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "carry_remove", GirlID: &hero}, local)
		require.NoError(t, err)
		assert.Zero(t, removed.Save.Girls[hero].CarryXP)
		assert.Equal(t, 2, removed.Save.CarryInventory["keepsake_lens"])
		assert.Equal(t, 1, removed.Save.CarryStoredXP["keepsake_lens"][60])
		equip.RequestID, equip.GirlID = uuid.NewString(), &partner
		transferred, err := model.ApplyGachaAction(user.Id, equip, local)
		require.NoError(t, err)
		assert.Equal(t, 60, transferred.Save.Girls[partner].CarryXP)
		assert.Empty(t, transferred.Save.CarryStoredXP["keepsake_lens"])
		equip.RequestID, equip.GirlID = uuid.NewString(), &hero
		second, err := model.ApplyGachaAction(user.Id, equip, local)
		require.NoError(t, err)
		assert.Zero(t, second.Save.Girls[hero].CarryXP)
	})
	t.Run("effort_stones_cover_training_not_breakthrough", func(t *testing.T) {
		user := gachaUser(t)
		profile, err := model.GetGachaProfile(user.Id, local)
		require.NoError(t, err)
		state := profile.Save
		perSession := profile.Training.EffortStages[0].Ink
		gold := profile.Training.EffortGoldBase
		state.Gold = 0
		state.Mats["effort_stone"], state.Mats["training_ink"] = 2*perSession, 2*perSession
		seedGachaState(t, user.Id, state)
		hero, stat := "c01", "atk"
		train := model.GachaActionIntent{RequestID: uuid.NewString(), Action: "effort_train_max", GirlID: &hero, Choice: &stat}
		_, err = model.ApplyGachaAction(user.Id, train, local)
		require.ErrorIs(t, err, model.ErrGachaFunds)
		saved := gachaState(t, user.Id, local)
		assert.Equal(t, state.Mats, saved.Mats)
		saved.Gold = 3 * gold
		seedGachaState(t, user.Id, saved)
		out, err := model.ApplyGachaAction(user.Id, train, local)
		require.NoError(t, err)
		assert.Equal(t, 3, out.Save.Daily.EffortSpent[hero])
		assert.Zero(t, out.Save.Mats["effort_stone"])
		assert.Equal(t, perSession, out.Save.Mats["training_ink"])
		assert.Zero(t, out.Save.Gold)
		saved = out.Save
		saved.Mats["effort_stone"], saved.Mats["training_ink"], saved.Gold = 100, 0, 10000
		saved.Girls[hero].Level = 40
		seedGachaState(t, user.Id, saved)
		_, err = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "card_breakthrough", GirlID: &hero}, local)
		require.ErrorIs(t, err, model.ErrGachaFunds)
		assert.Equal(t, 100, gachaState(t, user.Id, local).Mats["effort_stone"])
	})
}

package model

import (
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGachaWorldPullDistributionAndCurrency(t *testing.T) {
	clock := time.Date(2026, 11, 23, 0, 0, 0, 0, gameZone)
	hero := gachaGirl(gachaWorldSchedule(clock).Hero)
	random := rand.New(rand.NewSource(20261123))
	roll := func(limit int) (int, error) { return random.Intn(limit), nil }
	shardTotal, cardsTotal := 0, 0
	for range 1000 {
		state := newGachaState(clock)
		state.Mats["limited_ticket"] = 100
		for range 10 {
			receipt := GachaActionResult{}
			require.NoError(t, rollGachaWorld(state, hero, 10, &receipt, roll))
			require.Len(t, receipt.Drops, 10)
			for _, drop := range receipt.Drops {
				if drop == hero.Name+"碎片 ×1" {
					shardTotal++
				} else {
					require.Contains(t, drop, "整卡 ×1")
					cardsTotal++
				}
			}
		}
		require.Zero(t, state.Mats["limited_ticket"])
		require.Zero(t, state.Girls[hero.ID].Star, "even 80 shards must not auto-summon")
		for id, owned := range state.Girls {
			if id != hero.ID {
				require.Equal(t, "R", gachaGirl(id).Rarity)
				require.Zero(t, owned.Shards, "whole duplicates stay whole")
			}
		}
		require.Zero(t, state.Pity)
		require.Zero(t, state.TotalPull)
		require.Equal(t, newGachaState(clock).Gold, state.Gold)
		require.ErrorIs(t, rollGachaWorld(state, hero, 1, &GachaActionResult{}, roll), ErrGachaFunds)
	}
	require.Equal(t, 100000, shardTotal+cardsTotal)
	assert.InDelta(t, 55000, shardTotal, 800)
	t.Logf("1000 players/100000 draws: %d single featured fragments, %d R whole cards", shardTotal, cardsTotal)
}

func TestGachaWorldCalendarSourcesAndManualSynthesis(t *testing.T) {
	monday := time.Date(2026, 11, 22, 16, 0, 0, 0, time.UTC)
	period := gachaWorldSchedule(monday)
	require.Equal(t, "2026-11-23", period.SeasonKey)
	require.Equal(t, monday.AddDate(0, 0, 5).UnixMilli(), period.ClosesAt)
	require.Equal(t, period, gachaWorldSchedule(monday.Add(7*24*time.Hour-time.Nanosecond)))
	require.NotEqual(t, period.Hero, gachaWorldSchedule(monday.AddDate(0, 0, 7)).Hero)
	state := newGachaState(monday)
	require.NoError(t, normalizeGachaProgression(state, monday, DefaultGachaProgressionConfig()))
	hero := gachaGirl(period.Hero)
	id := hero.ID
	receipt := &GachaActionResult{}
	for _, source := range []string{"summon", "voyage", "growth", "starter"} {
		require.False(t, gachaCardHasSource(hero, source))
		require.ErrorIs(t, grantGachaSourceShards(state, hero, source, 80), ErrGachaCardSource)
		_, err := grantGachaWholeCard(state, hero, source)
		require.ErrorIs(t, err, ErrGachaCardSource)
	}
	require.True(t, gachaCardHasSource(gachaGirl("c16"), "summon"))
	require.False(t, gachaCardHasSource(gachaGirl("c16"), "growth"))
	require.ErrorIs(t, applyGachaProgressionAction(nil, 1, state, GachaActionIntent{Action: "wish", GirlID: &id}, receipt, monday), ErrGachaCardSource)
	state.Progression.Weeks[0].Completed, state.Progression.Weeks[0].Activities = 5, []string{"training", "wish"}
	week := state.Progression.Weeks[0].Week
	require.ErrorIs(t, applyGachaProgressionAction(nil, 1, state, GachaActionIntent{Action: "weekly_claim", ItemID: &week, GirlID: &id}, receipt, monday), ErrGachaCardSource)
	require.False(t, state.Progression.Weeks[0].Claimed)
	require.Nil(t, state.Girls[id])
	require.NoError(t, grantGachaSourceShards(state, hero, "world", 79))
	combine := GachaActionIntent{Action: "shard_unlock", GirlID: &id}
	require.ErrorIs(t, applyGachaProgressionAction(nil, 1, state, combine, receipt, monday), ErrGachaFunds)
	require.NoError(t, grantGachaSourceShards(state, hero, "world", 3))
	require.Zero(t, state.Girls[id].Star)
	require.NoError(t, applyGachaProgressionAction(nil, 1, state, combine, receipt, monday))
	require.Equal(t, 3, state.Girls[id].Star)
	require.Equal(t, 2, state.Girls[id].Shards)
	require.ErrorIs(t, applyGachaProgressionAction(nil, 1, state, combine, receipt, monday), ErrGachaFunds)
	rCard := gachaGirl("c01")
	_, err := grantGachaWholeCard(state, rCard, "world")
	require.NoError(t, err)
	require.Equal(t, 1, state.Girls[rCard.ID].Copies)
	require.NoError(t, recycleGachaWholeCopy(state, rCard.ID, receipt))
	require.Equal(t, rCard.DupGain, state.Girls[rCard.ID].Shards)
	require.ErrorIs(t, recycleGachaWholeCopy(state, rCard.ID, receipt), ErrGachaFunds)
}

func TestGachaWorldIntentAllowlist(t *testing.T) {
	date, count := "2026-11-23", 1
	intent := GachaActionIntent{Action: "world_pull", RequestID: uuid.NewString(), ItemID: &date, Count: &count}
	require.NoError(t, ValidateGachaAction(intent))
	for _, extra := range []func(*GachaActionIntent){
		func(in *GachaActionIntent) { in.GirlID = &date }, func(in *GachaActionIntent) { in.OpponentID = &count },
		func(in *GachaActionIntent) { in.Materials = map[string]int{"limited_ticket": 1} }, func(in *GachaActionIntent) { in.Choice = &date },
		func(in *GachaActionIntent) { in.Count = nil }, func(in *GachaActionIntent) { in.Action = "world_other" },
	} {
		bad := intent
		extra(&bad)
		require.ErrorIs(t, ValidateGachaAction(bad), ErrGachaIntent)
	}
	for _, value := range []string{"2026-11-24", strings.Repeat("0", 100), "2001-13-00"} {
		intent.ItemID = &value
		require.ErrorIs(t, ValidateGachaAction(intent), ErrGachaIntent)
	}
}

func TestGachaWorldGenericRewardsCannotIssueRestrictedFragments(t *testing.T) {
	clock := time.Date(2026, 11, 23, 0, 0, 0, 0, gameZone)
	state := newGachaState(clock)
	require.NoError(t, syncGachaMailbox(state))
	hero := gachaWorldSchedule(clock).Hero
	key := "shard:" + hero
	rewards := map[string]int{key: 80, "gold": 1}
	before := state.Gold
	_, sellable := gachaMailResource(state, key)
	require.False(t, sellable)
	require.ErrorIs(t, grantGachaDirectResources(state, rewards, &GachaActionResult{}, clock), ErrGachaCardSource)
	require.Error(t, sendGachaMail(state, "new-gift", "通用礼包", "", rewards, 0, clock))
	require.Equal(t, before, state.Gold)
	require.Nil(t, state.Girls[hero])
	require.Empty(t, state.Mailbox.Messages)
	id := "legacy:earned"
	state.Mailbox.Messages = append(state.Mailbox.Messages, &GachaMailMessage{ID: id, Title: "已寄达的老邮件", Sender: "航务署", CreatedAt: clock.UnixMilli(), Resources: rewards})
	require.NoError(t, syncGachaMailbox(state))
	intent := GachaActionIntent{Action: "mail_claim", ItemID: &id}
	require.NoError(t, applyGachaMail(state, intent, &GachaActionResult{}, clock))
	require.Equal(t, 80, state.Girls[hero].Shards)
	require.Zero(t, state.Girls[hero].Star)
	require.ErrorIs(t, applyGachaMail(state, intent, &GachaActionResult{}, clock), ErrGachaMailUnavailable)
}

func TestGachaWorldWeeklyTicketPacing(t *testing.T) {
	taskTickets := 0
	for _, chest := range gachaCaptainRules.Chests {
		taskTickets += chest.Resources["limited_ticket"]
	}
	for _, scenario := range []struct {
		arena, world, weeks int
		minMean, maxMean    float64
	}{{1, 1, 1, 80, 88}, {2, 2, 1, 55, 70}, {20, 20, 1, 20, 32}, {101, 21, 1, 10, 17}, {101, 21, 3, 35, 50}} {
		tickets := (taskTickets + gachaLadderRewardTier(scenario.arena).Tickets) * 7 * scenario.weeks
		draws, guaranteed := tickets, gachaWorldRankShards(scenario.world)
		for _, chest := range gachaWorldChests {
			if draws >= chest.Points {
				draws += chest.Tickets
				guaranteed += chest.Shards
			}
		}
		mean := float64(draws*gachaWorldShardPercent)/100 + float64(guaranteed)
		assert.GreaterOrEqual(t, mean, scenario.minMean)
		assert.LessOrEqual(t, mean, scenario.maxMean)
		t.Logf("arena %d, world %d, saving %d week(s) incl. weekends: earned %d tickets -> %d draws, mean %.2f featured shards", scenario.arena, scenario.world, scenario.weeks, tickets, draws, mean)
	}
	require.Zero(t, gachaWorldRankShards(21))
	require.Equal(t, 20, gachaWorldTiers[len(gachaWorldTiers)-1].Through)
	require.Equal(t, 3, gachaWorldEarnedMask(8))
}

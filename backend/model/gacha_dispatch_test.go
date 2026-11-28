package model

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func dispatchTestState(t *testing.T) (*GachaGameState, time.Time) {
	t.Helper()
	now := time.Date(2026, 11, 26, 18, 0, 0, 0, gameZone)
	state := newGachaState(now)
	require.NoError(t, syncGachaDispatch(state, 17, now))
	return state, now
}
func dispatchTestIntent(action, id, choice string, count int) GachaActionIntent {
	intent := GachaActionIntent{Action: action, RequestID: uuid.NewString()}
	if id != "" {
		intent.ItemID = &id
	}
	if choice != "" {
		intent.Choice = &choice
	}
	if count > 0 {
		intent.Count = &count
	}
	return intent
}

func TestDispatchClocksHaveExactlyOneSolutionAndOneAward(t *testing.T) {
	for seed := range 128 {
		clock := dispatchClock(dispatchRNG(uint64(seed)))
		count := 0
		for _, order := range clockOrders {
			if clock.accepts(order) {
				count++
			}
		}
		require.Equal(t, 1, count)
		require.NotEmpty(t, clock.Hints)
	}
	state, now := dispatchTestState(t)
	day := state.Daily.Date
	clock := &state.Daily.Dispatch.Clock
	win, miss := "", ""
	for _, order := range clockOrders {
		if clock.accepts(order) {
			win = order
		} else {
			miss = order
		}
	}
	result := &GachaActionResult{}
	guess := dispatchTestIntent("clock_align", day, miss, 1)
	require.NoError(t, applyGachaDispatch(state, guess, result))
	require.Error(t, applyGachaDispatch(state, guess, result))
	require.Zero(t, state.Mats["gear_thread"])
	guess = dispatchTestIntent("clock_align", day, win, 2)
	state.Mats["gear_thread"] = gachaMaxResource
	require.ErrorIs(t, applyGachaDispatch(state, guess, result), ErrGachaCapacity)
	require.Len(t, clock.Guesses, 1)
	require.False(t, clock.Solved)
	state.Mats["gear_thread"] = 0
	require.NoError(t, applyGachaDispatch(state, guess, result))
	require.Error(t, applyGachaDispatch(state, guess, result))
	require.Equal(t, 3, state.Mats["gear_thread"])
	require.NoError(t, syncGachaDispatch(state, 17, now))
	tomorrow := now.AddDate(0, 0, 1)
	regenGachaState(state, tomorrow)
	require.NoError(t, syncGachaDispatch(state, 17, tomorrow))
	require.Empty(t, state.Daily.Dispatch.Clock.Guesses)
	require.Error(t, applyGachaDispatch(state, guess, result))
}
func TestDispatchRelicsRequireSpecificArchiveAndFreezeOnlyLeader(t *testing.T) {
	state, _ := dispatchTestState(t)
	stock := gachaEvidence(state)
	stock["echo"], stock["ember"] = 3, 1
	state.Casebook.Endings["silent_bell"] = map[string]int{"filed": 1}
	result := &GachaActionResult{}
	bind := dispatchTestIntent("relic_bind", "bell", "", 0)
	require.Error(t, applyGachaDispatch(state, bind, result))
	require.Equal(t, 3, stock["echo"])
	state.Casebook.Endings["silent_bell"]["revealed"] = 1
	require.NoError(t, applyGachaDispatch(state, bind, result))
	require.Error(t, applyGachaDispatch(state, bind, result))
	require.Zero(t, stock["echo"])
	require.Empty(t, state.Dispatch.Active)
	before, err := gachaSquadUnits(state, "ally", "a")
	require.NoError(t, err)
	require.NoError(t, applyGachaDispatch(state, dispatchTestIntent("relic_wear", "bell", "", 0), result))
	data, err := freezeGachaRoster(state)
	require.NoError(t, err)
	require.NotContains(t, string(data), `"casebook"`)
	require.NoError(t, applyGachaDispatch(state, dispatchTestIntent("relic_store", "", "", 0), result))
	saved, err := thawGachaRoster(GachaDocument(data))
	require.NoError(t, err)
	after, err := gachaSquadUnits(saved, "enemy", "x")
	require.NoError(t, err)
	assert.Equal(t, before[0].Rage+28, after[0].Rage)
	assert.Equal(t, before[0].MaxHP-before[0].MaxHP/10, after[0].HP)
	assert.Equal(t, before[1].Rage, after[1].Rage)
	assert.Equal(t, after[1].MaxHP, after[1].HP)
	saved.previousTraining = true
	old, err := gachaSquadUnits(saved, "enemy", "x")
	require.NoError(t, err)
	assert.Equal(t, before[0].Rage, old[0].Rage)
	for _, id := range []string{"shroud", "lens"} {
		saved.previousTraining = false
		saved.Dispatch.Relics[id], saved.Dispatch.Active = true, id
		units, err := gachaSquadUnits(saved, "ally", "x")
		require.NoError(t, err)
		if id == "shroud" {
			assert.Equal(t, before[0].rageBonus-10, units[0].rageBonus)
			assert.Equal(t, before[0].Shield+before[0].MaxHP*18/100, units[0].Shield)
		} else {
			assert.InDelta(t, before[0].girl.Skills.Normal.Mult*0.88, units[0].girl.Skills.Normal.Mult, 0.00001)
			assert.InDelta(t, before[0].skillBoost+0.18, units[0].skillBoost, 0.00001)
		}
	}
}
func TestDispatchContractUsesSignedBattleNotCurrentLineupOrOldRuns(t *testing.T) {
	state, now := dispatchTestState(t)
	id := "c01"
	stage := gachaStage("s1")
	old := GachaRunSnapshot{Squad: []string{id}}
	issueGachaDispatch(state, stage, &old)
	sign := dispatchTestIntent("contract_sign", state.Daily.Date, "courier", 0)
	sign.GirlID = &id
	result := &GachaActionResult{}
	require.NoError(t, applyGachaDispatch(state, sign, result))
	require.Error(t, applyGachaDispatch(state, sign, result))
	signed := state.Daily.Dispatch.Contract
	report := &GachaBattleReport{TotalN: 2, AliveN: 2}
	finish := &GachaFinishResult{Win: true}
	for _, snap := range []GachaRunSnapshot{old, {Squad: []string{"c08", id}}, {Squad: []string{id, "c08", "c12"}}, {Mode: "expedition", Squad: []string{id}}} {
		if snap.Squad[0] != id || len(snap.Squad) > 1 || snap.Mode != "" {
			issueGachaDispatch(state, stage, &snap)
		}
		settleGachaDispatch(state, stage, &snap, report, finish)
	}
	require.Zero(t, signed.Progress)
	snap := GachaRunSnapshot{Squad: []string{id, "c08"}}
	issueGachaDispatch(state, stage, &snap)
	report.AliveN = 1
	settleGachaDispatch(state, stage, &snap, report, finish)
	require.Zero(t, signed.Progress)
	report.AliveN = 2
	settleGachaDispatch(state, stage, &snap, report, &GachaFinishResult{})
	require.Zero(t, signed.Progress)
	state.Squad = []string{"c12"}
	for range 2 {
		settleGachaDispatch(state, stage, &snap, report, finish)
	}
	require.Equal(t, 2, signed.Progress)
	claim := dispatchTestIntent("dispatch_contract_claim", state.Daily.Date, "", 0)
	require.NoError(t, applyGachaDispatch(state, claim, result))
	require.Error(t, applyGachaDispatch(state, claim, result))
	require.Equal(t, 4, state.Mats["gear_plate"])
	tomorrow := now.AddDate(0, 0, 1)
	regenGachaState(state, tomorrow)
	require.NoError(t, syncGachaDispatch(state, 17, tomorrow))
	sign.ItemID = &state.Daily.Date
	require.NoError(t, applyGachaDispatch(state, sign, result))
	settleGachaDispatch(state, stage, &snap, report, finish)
	assert.Zero(t, state.Daily.Dispatch.Contract.Progress)
}
func TestDispatchRouteHasReachabilityCargoRiskAndAtomicBanking(t *testing.T) {
	state, _ := dispatchTestState(t)
	result := &GachaActionResult{}
	tiles := strings.Fields("cache fog spring vault spring beacon fog cache spring beacon vault spring")
	reset := func() { state.Daily.Dispatch.Route = GachaFogRoute{Tiles: tiles, Light: 5, Status: "open"} }
	act := func(action, choice string, count int) error {
		return applyGachaDispatch(state, dispatchTestIntent(action, state.Daily.Date, choice, count), result)
	}
	reset()
	require.NoError(t, act("route_step", "left", 1))
	require.Error(t, act("route_step", "right", 2))
	require.Error(t, act("route_bank", "", 2))
	state.Mats["forge_ore"] = gachaMaxResource
	require.ErrorIs(t, act("route_bank", "", 1), ErrGachaCapacity)
	assert.Equal(t, "open", state.Daily.Dispatch.Route.Status)
	state.Mats["forge_ore"] = 0
	require.NoError(t, act("route_bank", "", 1))
	assert.Equal(t, 3, state.Mats["forge_ore"])
	require.Error(t, act("route_step", "middle", 2))
	require.True(t, validDispatchRoute(state.Daily.Dispatch.Route))
	reset()
	require.NoError(t, act("route_step", "middle", 1))
	require.NoError(t, act("route_step", "left", 2))
	assert.Equal(t, "lost", state.Daily.Dispatch.Route.Status)
	require.Equal(t, 3, state.Mats["forge_ore"])
	require.Error(t, act("route_bank", "", 2))
	reset()
	for index, move := range []string{"right", "middle", "middle", "middle"} {
		require.NoError(t, act("route_step", move, index+1))
	}
	route := state.Daily.Dispatch.Route
	assert.Equal(t, "arrived", route.Status)
	assert.True(t, validDispatchRoute(route))
	assert.Equal(t, 3, state.Mats["route_badge"])
}
func TestDispatchMosaicsAreSolvableAndOnlyAdvanceOnce(t *testing.T) {
	state, now := dispatchTestState(t)
	desk := &state.Dispatch.Mosaic
	require.False(t, validDispatchMosaic([]int{0, 4, 8}, "aaa-bbb-ccc"))
	layouts := []string{}
	var fill func(string)
	fill = func(cells string) {
		if len(cells) == 9 {
			layouts = append(layouts, cells[:3]+"-"+cells[3:6]+"-"+cells[6:])
			return
		}
		for _, ink := range []string{"a", "b", "c"} {
			if strings.Count(cells, ink) < 3 {
				fill(cells + ink)
			}
		}
	}
	fill("")
	for page := 1; page <= 3; page++ {
		pos := slices.IndexFunc(layouts, func(layout string) bool { return validDispatchMosaic(desk.Marks, layout) })
		require.NotEqual(t, -1, pos)
		intent := dispatchTestIntent("mosaic_place", desk.Week, layouts[pos], page)
		require.NoError(t, applyGachaDispatch(state, intent, &GachaActionResult{}))
		require.Error(t, applyGachaDispatch(state, intent, &GachaActionResult{}))
	}
	assert.Empty(t, desk.Marks)
	assert.Equal(t, 6, state.Mats["gear_rivet"])
	require.NoError(t, syncGachaDispatch(state, 17, now))
	later := now.AddDate(0, 0, 7)
	regenGachaState(state, later)
	require.NoError(t, syncGachaDispatch(state, 17, later))
	require.Zero(t, desk.Cleared)
	require.Len(t, desk.Marks, 3)
}
func TestDispatchMirrorUsesFixedLoanersAndRealCombat(t *testing.T) {
	state, _ := dispatchTestState(t)
	desk := &state.Daily.Dispatch.Mirror
	scene := gachaDispatch.Mirrors[desk.Scene]
	pair := slices.Clone(scene.Pool[:2])
	intent := dispatchTestIntent("mirror_trial", state.Daily.Date, "spark", 1)
	intent.Squad = &pair
	expected, err := simulateDispatchMirror(scene, state.Daily.Date, pair, "spark")
	require.NoError(t, err)
	ordinary := slices.Clone(state.Squad)
	gold, stamina := state.Gold, state.Stamina
	require.NoError(t, applyGachaDispatch(state, intent, &GachaActionResult{}))
	require.Equal(t, expected, desk.Last)
	assert.Positive(t, desk.Last.Rounds)
	assert.Equal(t, mirrorObjective(scene, expected), desk.Won)
	assert.Equal(t, ordinary, state.Squad)
	assert.Equal(t, gold, state.Gold)
	assert.Equal(t, stamina, state.Stamina)
	require.Error(t, applyGachaDispatch(state, intent, &GachaActionResult{}))
	_, err = simulateDispatchMirror(scene, state.Daily.Date, []string{"c99", pair[1]}, "spark")
	require.Error(t, err)
}

func TestDispatchEveryMirrorHasAWinningLoanPlan(t *testing.T) {
	for _, scene := range gachaDispatch.Mirrors {
		t.Run(scene.Goal, func(t *testing.T) {
			found := false
			for _, lead := range scene.Pool {
				for _, partner := range scene.Pool {
					if lead == partner {
						continue
					}
					for _, tactic := range []string{"ward", "spark", "rush"} {
						log, err := simulateDispatchMirror(scene, "2026-11-26", []string{lead, partner}, tactic)
						require.NoError(t, err)
						if mirrorObjective(scene, log) {
							found = true
							break
						}
					}
					if found {
						break
					}
				}
				if found {
					break
				}
			}
			require.True(t, found, "a standard loaner party must be able to clear today's target")
		})
	}
}

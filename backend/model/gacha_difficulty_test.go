package model

import (
	"cmp"
	"fmt"
	"maps"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

var difficultyTestDay = time.Date(2026, 11, 23, 12, 0, 0, 0, gameZone)

func difficultyRNG(seed int) *rand.Rand {
	return rand.New(rand.NewPCG(uint64(seed+3), uint64(seed*17+127)))
}

func difficultyPlayer(team []string, level, stars, skill int, equipped bool) *GachaGameState {
	state := newGachaState(difficultyTestDay)
	state.Captain.Level = gachaCatalog.Growth.MaxLevel
	state.Squad = slices.Clone(team)
	for _, girl := range gachaCatalog.Girls {
		own := &GachaOwnedGirl{Level: 1, Star: gachaInitialStars(&girl), Gems: newGachaGemSlots()}
		if slices.Contains(team, girl.ID) {
			own.Level, own.Star = level, max(own.Star, stars)
			if own.Star >= gachaFirstAdvancedStar {
				own.Awakening = gachaCultivation.AdvancedAwakening[own.Star-gachaFirstAdvancedStar]
			}
			own.Training = &GachaCardTraining{Skills: map[string]int{"normal": skill, "rage": skill, "passive": skill}}
			if equipped {
				own.WeaponLevel = 20
				own.Equipment = map[string]GachaEquipmentPiece{"head": {Level: 15, Star: 1}, "body": {Level: 15, Star: 1}, "hands": {Level: 15, Star: 1}, "feet": {Level: 15, Star: 1}}
			}
		}
		state.Girls[girl.ID] = own
	}
	return state
}

func difficultyScene(t *testing.T, key string, team []string, week time.Time) (*GachaGameState, GachaStage, GachaRunSnapshot) {
	t.Helper()
	attack := []string{"c04", "c11", "c19", "c23"}
	reserve := []string{"c15", "c14", "c05", "c10"}
	players := team
	if strings.HasPrefix(key, "dual_trial:") {
		players = append(slices.Clone(attack), reserve...)
	}
	state := difficultyPlayer(players, 150, 12, 120, true)
	state.Squad = slices.Clone(team)
	snapshot := GachaRunSnapshot{Version: 1, Squad: slices.Clone(team)}
	stage := gachaStage(key)
	if stage == nil {
		mode, part, _ := strings.Cut(key, ":")
		number := 0
		if part != "" {
			var err error
			number, err = strconv.Atoi(part)
			require.NoError(t, err)
		}
		rules := DefaultGachaProgressionConfig()
		state.ProgressionRules = rules
		state.Progression = &GachaProgressionState{Days: []GachaProgressionDay{{Date: gachaDay(week), Rules: rules, TrainingLeft: 1}}}
		resetGachaWeeklyModes(state, week)
		switch mode {
		case "training":
			state.Cleared["s3"], state.Cleared["s6"] = number >= 2, number >= 3
		case "awakening":
			state.Girls[team[0]].Star = 5 // Awakening candidates have not unlocked advanced stars yet.
			state.Girls[team[0]].Awakening = number - 1
			mode = fmt.Sprintf("trial_%s_%d", team[0], number)
		case "dual_trial":
			state.Cleared["s6"] = true
			state.Squad, state.Progression.ReserveSquad = attack, reserve
			if number == 2 {
				state.Progression.Dual.Phase = 1
				state.Progression.Dual.Squads = [][]string{attack, reserve}
			}
		case "expedition":
			state.Expedition.Floor, state.Expedition.Route = number, "bastion"
		}
		var err error
		if isGachaProgressionStage(mode) {
			stage, snapshot, err = specialGachaProgressionStage(mode, state, week)
		} else {
			stage, snapshot, err = specialGachaStage(mode, state, week)
			snapshot.Squad = slices.Clone(state.Squad)
		}
		require.NoError(t, err)
		state.Squad = slices.Clone(snapshot.Squad)
		if mode == "expedition" {
			state.BattleBuff = snapshot.Route
		}
	}
	require.NotNil(t, stage)
	combat, err := issueGachaCombatStage(*stage, &snapshot, state.Cleared)
	require.NoError(t, err)
	unchanged := combat
	unchanged.difficulty, unchanged.EnemyElem, unchanged.EnemyLv, unchanged.Mechanic = stage.difficulty, stage.EnemyElem, stage.EnemyLv, stage.Mechanic
	require.Equal(t, *stage, unchanged, "cost, rewards, gates, star threshold and native boss fields are unchanged")
	require.Equal(t, key, snapshot.Difficulty.Key)
	require.Equal(t, gachaDifficulty.Stages[key], snapshot.Difficulty.Tier)
	return state, combat, snapshot
}

func TestGachaDifficultyCatalogAndFrozenReports(t *testing.T) {
	require.NoError(t, gachaDifficulty.validate())
	require.Len(t, gachaDifficulty.Stages, 89)
	bad := gachaDifficulty
	bad.Stages = maps.Clone(bad.Stages)
	delete(bad.Stages, "res_forge")
	bad.Stages["mirror_trial"] = 0
	require.Error(t, bad.validate())
	bad = gachaDifficulty
	bad.Encounters = maps.Clone(bad.Encounters)
	bad.Encounters["s1"] = GachaDifficultyEncounter{Level: 200}
	require.Error(t, bad.validate())
	for _, key := range []string{"mirror_trial", "guild_raid", "arena", "", "training:4", "expedition:11"} {
		require.Nil(t, gachaDifficulty.issue(key))
	}
	team := []string{"c28", "c24", "c11", "c19"}
	state, stage, snapshot := difficultyScene(t, "training:3", team, difficultyTestDay)
	report, err := simulateGachaCombat(state, stage, difficultyRNG(5))
	require.NoError(t, err)
	require.Equal(t, gachaDifficulty.Encounters["training:3"].Level, report.Waves[0][0].Level)
	require.Equal(t, 1, snapshot.ProgressionStage.EnemyLv, "the original scene/rewards were not changed")
	legacy, err := simulateGachaCombat(state, *snapshot.ProgressionStage, difficultyRNG(5))
	require.NoError(t, err)
	require.Equal(t, 1, legacy.Waves[0][0].Level)
	captured := *snapshot.Difficulty
	captured.Version += 99
	captured.Atk++
	snapshot.Difficulty = &captured
	for _, archived := range []*GachaIssuedDifficulty{&captured, nil} {
		snapshot.Difficulty = archived
		stored := report
		if archived == nil {
			stored = legacy
		}
		encodedReport, err := common.Marshal(stored)
		require.NoError(t, err)
		encoded, err := common.Marshal(snapshot)
		require.NoError(t, err)
		run := &GachaRun{Stage: "training", Snapshot: GachaDocument(encoded), BattleReport: GachaDocument(encodedReport), ReadyAt: 17}
		view, err := gachaRunView(run, state, 3)
		require.NoError(t, err)
		require.Equal(t, archived, view.Difficulty, "never re-resolve a historical tier, including pre-difficulty runs")
		reloaded, err := common.Marshal(view.Result)
		require.NoError(t, err)
		require.JSONEq(t, string(encodedReport), string(reloaded))
	}
}

func TestGachaDifficultyUsesFixedEnemiesAndPreservesNativeMechanics(t *testing.T) {
	base := *gachaStage("s1")
	base.EnemyElem, base.Mechanic, base.BossElem = "plant", "barrier", "water"
	base.HasBoss, base.BossName, base.Waves = true, "原始守卫", 2
	base.HPScale, base.AtkScale, base.DefScale = 2, 1.5, 1.3
	rating := &GachaIssuedDifficulty{GachaDifficultyTier: GachaDifficultyTier{HP: 200, Atk: 170, Def: 130}, GachaDifficultyEncounter: GachaDifficultyEncounter{Element: "fire", Mechanic: "venom"}}
	scaled := gachaCombatStage(base, rating)
	require.Equal(t, "plant", scaled.EnemyElem)
	require.Equal(t, "barrier", scaled.Mechanic)
	beginner := newGachaState(difficultyTestDay)
	strong := difficultyPlayer([]string{"c04", "c11", "c19", "c23"}, 150, 12, 120, true)
	old, err := simulateGachaCombat(beginner, base, difficultyRNG(30))
	require.NoError(t, err)
	weakRun, err := simulateGachaCombat(beginner, scaled, difficultyRNG(30))
	require.NoError(t, err)
	strongRun, err := simulateGachaCombat(strong, scaled, difficultyRNG(90))
	require.NoError(t, err)
	require.Equal(t, weakRun.Waves, strongRun.Waves)
	require.InDelta(t, 2*float64(old.Waves[0][0].MaxHP), weakRun.Waves[0][0].MaxHP, 1.5)
	require.InDelta(t, 1.7*float64(old.Waves[0][0].Atk), weakRun.Waves[0][0].Atk, 1.5)
	require.InDelta(t, 1.3*float64(old.Waves[0][0].Def), weakRun.Waves[0][0].Def, 1.5)
	boss := weakRun.Waves[1][0]
	require.Equal(t, "water", boss.Elem)
	require.Equal(t, "原始守卫", boss.Name)
	require.Positive(t, boss.Shield)
	require.Nil(t, base.difficulty)
	sweep := gachaVoyage.Stages["s7"].GachaStage
	issued := gachaCombatStage(sweep, gachaDifficulty.issue(sweep.ID))
	manual, err := simulateGachaCombat(strong, issued, difficultyRNG(11))
	require.NoError(t, err)
	swept, err := SimulateGachaCombat(strong, sweep)
	require.NoError(t, err)
	require.Equal(t, manual.Waves, swept.Waves, "sweep verification cannot face the old unscaled enemies")
}

func TestGachaDifficultySeededClearability(t *testing.T) {
	attack := []string{"c04", "c11", "c19", "c23"}
	sustain := []string{"c28", "c24", "c11", "c19"}
	for _, key := range slices.Sorted(maps.Keys(gachaDifficulty.Stages)) {
		t.Run(key, func(t *testing.T) {
			wins, sampled := 0, 0
			for _, team := range [][]string{attack, sustain} {
				state, stage, _ := difficultyScene(t, key, team, difficultyTestDay)
				for seed := range 3 {
					report, err := simulateGachaCombat(state, stage, difficultyRNG(seed))
					require.NoError(t, err)
					require.LessOrEqual(t, report.Rounds, 60)
					if report.Win {
						wins++
					}
					sampled++
				}
			}
			t.Logf("strong-team clear witnesses %d/%d", wins, sampled)
			require.Positive(t, wins, "no reachable win observed for capped candidate teams; inspect balance")
		})
	}
	rotations := map[string]bool{}
	for offset := 0; len(rotations) < 6 && offset < 53; offset++ {
		week := difficultyTestDay.AddDate(0, 0, offset*7)
		state, stage, snapshot := difficultyScene(t, "weekly_boss", sustain, week)
		if rotations[stage.BossElem] {
			continue
		}
		rotations[stage.BossElem] = true
		require.Equal(t, weeklyBossElement(gachaWeekKey(week)), stage.BossElem)
		require.Equal(t, weeklyBossMechanic(gachaWeekKey(week)), stage.Mechanic)
		require.Equal(t, stage.BossElem, snapshot.Difficulty.BossElement)
		require.Equal(t, stage.BossElem, snapshot.Difficulty.Element)
		require.Equal(t, stage.Mechanic, snapshot.Difficulty.Mechanic)
		wins := 0
		for seed := range 3 {
			report, err := simulateGachaCombat(state, stage, difficultyRNG(seed))
			require.NoError(t, err)
			if report.Win {
				wins++
			}
		}
		t.Logf("weekly %s/%s: wins=%d/3", stage.BossElem, stage.Mechanic, wins)
		require.Positive(t, wins)
	}
	require.Len(t, rotations, 6)
}

func difficultyCounterWithSupport(t *testing.T, stage GachaStage) []string {
	t.Helper()
	element := stage.EnemyElem
	if stage.HasBoss && stage.BossElem != "" {
		element = stage.BossElem
	}
	roster := slices.Clone(gachaCatalog.Girls)
	slices.SortStableFunc(roster, func(first, second GachaGirl) int {
		firstBonus, _ := gachaElementMultiplier(first.Elem, element)
		secondBonus, _ := gachaElementMultiplier(second.Elem, element)
		return cmp.Compare(secondBonus, firstBonus)
	})
	var ids []string
	for _, slot := range []struct {
		rarity string
		aid    bool
	}{{"SSR", false}, {"SR", true}, {"SR", false}, {"SR", false}} {
		for _, girl := range roster {
			if girl.Rarity == slot.rarity && (girl.Role == "support" || girl.Role == "heal") == slot.aid && !slices.Contains(ids, girl.ID) {
				ids = append(ids, girl.ID)
				break
			}
		}
	}
	require.Len(t, ids, 4)
	return ids
}

func TestGachaDifficultyAwakeningCandidates(t *testing.T) {
	for _, girl := range gachaCatalog.Girls {
		team := []string{girl.ID}
		for _, id := range []string{"c28", "c24", "c11", "c19"} {
			if len(team) < 4 && !slices.Contains(team, id) {
				team = append(team, id)
			}
		}
		for rank := 1; rank <= 3; rank++ {
			state, stage, snapshot := difficultyScene(t, "awakening:"+strconv.Itoa(rank), team, difficultyTestDay)
			require.Equal(t, girl.ID, snapshot.ProgressionGirl)
			require.Equal(t, girl.Elem, stage.EnemyElem)
			require.Contains(t, snapshot.Squad, girl.ID)
			cleared := false
			for seed := 0; seed < 3 && !cleared; seed++ {
				report, err := simulateGachaCombat(state, stage, difficultyRNG(seed))
				require.NoError(t, err)
				cleared = report.Win
			}
			require.True(t, cleared, "no clear witness for %s awakening rank %d", girl.ID, rank)
		}
	}
}

func TestGachaDifficultyGrowthPressure(t *testing.T) {
	attack := []string{"c04", "c11", "c19", "c23"}
	sustain := []string{"c28", "c24", "c11", "c19"}
	beginner := newGachaState(difficultyTestDay)
	counts := func(state *GachaGameState, id string) (wins int) {
		snapshot := GachaRunSnapshot{}
		stage, err := issueGachaCombatStage(*gachaStage(id), &snapshot, state.Cleared)
		require.NoError(t, err)
		for seed := range 12 {
			report, err := simulateGachaCombat(state, stage, difficultyRNG(seed))
			require.NoError(t, err)
			if report.Win {
				wins++
			}
		}
		return wins
	}
	first, late := counts(beginner, "s1"), counts(beginner, "v4n2")
	require.Positive(t, first, "a fresh account must have a plausible first win")
	require.Less(t, late, first, "late dungeons must put more pressure on the identical new squad")
	lateStage := gachaCombatStage(*gachaStage("v4n2"), gachaDifficulty.issue("v4n2"))
	for _, team := range [][]string{attack, sustain, difficultyCounterWithSupport(t, lateStage)} {
		for _, build := range []struct{ level, stars, skill int }{{30, 3, 13}, {70, 6, 31}, {125, 9, 100}} {
			state := difficultyPlayer(team, build.level, build.stars, build.skill, build.level >= 125)
			t.Logf("team=%v level=%d stars=%d s12=%d/12 v4n2=%d/12", team, build.level, build.stars, counts(state, "s12"), counts(state, "v4n2"))
		}
	}
}

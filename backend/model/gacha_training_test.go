package model

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func trainingTestIntent(action, choice string) GachaActionIntent {
	id := "c01"
	intent := GachaActionIntent{Action: action, GirlID: &id, RequestID: uuid.NewString()}
	if choice != "" {
		intent.Choice = &choice
	}
	return intent
}

func TestGachaTrainingAllGradeStepsAndNoPartialSpending(t *testing.T) {
	state := battleTestState([]string{"c01"}, 60, 12)
	state.Gold = 100_000
	state.Mats["training_ink"], state.Mats["break_seal"] = 99999, 999
	own := state.Girls["c01"]
	own.Exp = 325
	own.Awakening, own.Branch = 3, GachaAwakeningBranches("c01")[0].ID
	intent := trainingTestIntent("card_breakthrough", "")
	completed := 0
	for index, grade := range gachaTraining.Grades {
		require.Len(t, grade.PlusLevels, grade.MaxPlus)
		for step := 1; step <= grade.MaxPlus; step++ {
			for own.Level < grade.PlusLevels[step-1] {
				_, err := trainGachaLevel(own, gachaLevelCost(own.Level), gachaCatalog.Growth.MaxLevel)
				require.NoError(t, err)
			}
			level, xp := own.Level, own.Exp
			ink, seals := state.Mats["training_ink"], state.Mats["break_seal"]
			require.NoError(t, applyGachaTrainingAction(state, intent, &GachaActionResult{}))
			trained := state.Girls["c01"].Training
			assert.Equal(t, index, trained.Grade)
			assert.Equal(t, step, trained.Plus)
			assert.Equal(t, level, own.Level)
			assert.Equal(t, xp, own.Exp)
			assert.Equal(t, ink-(8+4*completed+completed*(completed+1)/2), state.Mats["training_ink"])
			completed++
			assert.Equal(t, seals-completed/3, state.Mats["break_seal"])
		}
		if index+1 < len(gachaTraining.Grades) {
			ink, seals := state.Mats["training_ink"], state.Mats["break_seal"]
			next := gachaTraining.Grades[index+1]
			if own.Level < next.Level {
				require.ErrorIs(t, applyGachaTrainingAction(state, intent, &GachaActionResult{}), ErrGachaCultivationLocked)
				for own.Level < next.Level {
					_, err := trainGachaLevel(own, gachaLevelCost(own.Level), gachaCatalog.Growth.MaxLevel)
					require.NoError(t, err)
				}
			}
			state.Mats["break_seal"] = (completed+1)/3 - 1
			require.ErrorIs(t, applyGachaTrainingAction(state, intent, &GachaActionResult{}), ErrGachaFunds)
			assert.Equal(t, ink, state.Mats["training_ink"])
			assert.Equal(t, index, state.Girls["c01"].Training.Grade)
			assert.GreaterOrEqual(t, own.Level, next.Level)
			state.Mats["break_seal"] = seals
			level, xp := own.Level, own.Exp
			require.NoError(t, applyGachaTrainingAction(state, intent, &GachaActionResult{}))
			assert.Equal(t, index+1, state.Girls["c01"].Training.Grade)
			assert.Zero(t, state.Girls["c01"].Training.Plus)
			assert.Equal(t, level, own.Level)
			assert.Equal(t, xp, own.Exp)
			assert.Zero(t, own.PeakLevel)
			assert.Equal(t, 12, own.Star)
			assert.Equal(t, 3, own.Awakening)
			require.True(t, validGachaCultivationState(own))
			assert.Equal(t, ink-(8+4*completed+completed*(completed+1)/2), state.Mats["training_ink"])
			completed++
			assert.Equal(t, seals-completed/3, state.Mats["break_seal"])
		}
	}
	require.ErrorIs(t, applyGachaTrainingAction(state, intent, &GachaActionResult{}), ErrGachaCultivationLocked)
	own.Level, own.PeakLevel = 60, 0
	for _, bad := range []GachaCardTraining{{Grade: -1}, {Grade: 7}, {Plus: 3}, {Stage: -1}, {Stage: 1}, {Effort: map[string]int{"hp": 161}}, {Skills: map[string]int{"rage": gachaCatalog.Growth.MaxLevel + 1}}, {Skills: map[string]int{"extra": 1}}} {
		state.Girls["c01"].Training = &bad
		assert.False(t, validGachaCardTraining(state.Girls["c01"]))
	}
}

func TestGachaTrainingGoldIsRequiredBeforeAnyMaterialOrProgressIsSpent(t *testing.T) {
	state := battleTestState([]string{"c01"}, 20, 2)
	state.Mats["training_ink"] = 30
	state.Gold = gachaTraining.BreakCost.GoldBase - 1
	intent := trainingTestIntent("card_breakthrough", "")
	require.ErrorIs(t, applyGachaTrainingAction(state, intent, &GachaActionResult{}), ErrGachaFunds)
	require.Equal(t, 30, state.Mats["training_ink"])
	require.Nil(t, state.Girls["c01"].Training)
	state.Gold++
	require.NoError(t, applyGachaTrainingAction(state, intent, &GachaActionResult{}))
	require.Zero(t, state.Gold)
	require.Equal(t, 1, state.Girls["c01"].Training.Plus)
	state.Gold = 3*gachaTraining.EffortGoldBase - 1
	require.NoError(t, applyGachaTrainingAction(state, trainingTestIntent("effort_train_max", "hp"), &GachaActionResult{}))
	require.Equal(t, 2, state.Daily.EffortSpent["c01"])
	require.Equal(t, gachaTraining.EffortGoldBase-1, state.Gold)
	require.ErrorIs(t, applyGachaTrainingAction(state, trainingTestIntent("effort_train", "hp"), &GachaActionResult{}), ErrGachaFunds)
	require.Equal(t, 2, state.Daily.EffortSpent["c01"])
}

func TestGachaGreenToBlueNeedsCurrentLevelNotStars(t *testing.T) {
	state := battleTestState([]string{"c01"}, 19, 1)
	own := state.Girls["c01"]
	own.PeakLevel, own.Exp = 64, 75
	own.Training = &GachaCardTraining{Grade: 1, Plus: 2, Skills: map[string]int{"normal": 7}}
	state.Mats["training_ink"], state.Mats["break_seal"] = 43, 2
	intent := trainingTestIntent("card_breakthrough", "")
	require.ErrorIs(t, applyGachaTrainingAction(state, intent, &GachaActionResult{}), ErrGachaCultivationLocked)
	assert.Equal(t, 43, state.Mats["training_ink"])
	own.Level = 20
	state.Mats["training_ink"]--
	require.ErrorIs(t, applyGachaTrainingAction(state, intent, &GachaActionResult{}), ErrGachaFunds)
	assert.Equal(t, 2, state.Mats["break_seal"])
	assert.Equal(t, 75, own.Exp)
	state.Mats["training_ink"]++
	require.NoError(t, applyGachaTrainingAction(state, intent, &GachaActionResult{}))
	assert.Equal(t, 2, own.Training.Grade)
	assert.Zero(t, own.Training.Plus)
	assert.Equal(t, 20, own.Level)
	assert.Equal(t, 75, own.Exp)
	assert.Equal(t, 7, own.Training.Skills["normal"])
	assert.Equal(t, 64, own.PeakLevel)
	assert.Zero(t, state.Mats["training_ink"])
	assert.Zero(t, state.Mats["break_seal"])
	assert.True(t, validGachaCardTraining(own))
}

func TestGachaTrainingIntentAndEffortFloors(t *testing.T) {
	for _, action := range []struct{ key, choice string }{{"card_breakthrough", ""}, {"effort_train", "hp"}, {"effort_train_max", "hp"}, {"effort_promote", ""}, {"skill_point", "rage"}} {
		intent := trainingTestIntent(action.key, action.choice)
		require.NoError(t, ValidateGachaAction(intent))
		bad := intent
		count := 1
		bad.Count = &count
		assert.ErrorIs(t, ValidateGachaAction(bad), ErrGachaIntent)
		bad = intent
		bad.Choice = &intent.Action
		assert.ErrorIs(t, ValidateGachaAction(bad), ErrGachaIntent)
		bad = intent
		bad.ItemID = &intent.Action
		assert.ErrorIs(t, ValidateGachaAction(bad), ErrGachaIntent)
	}
	state := battleTestState([]string{"c01"}, 40, 3)
	own := state.Girls["c01"]
	clock := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	state.Mats["training_ink"], state.Mats["break_seal"], state.Gold = 80, 1, 80*gachaTraining.EffortGoldBase
	for day := range 8 {
		regenGachaState(state, clock.AddDate(0, 0, day))
		for _, stat := range gachaTraining.EffortStats {
			if own.Training != nil && own.Training.Effort[stat.ID] == 160 {
				continue
			}
			if state.Daily.EffortSpent["c01"] == 10 {
				break
			}
			require.ErrorIs(t, applyGachaTrainingAction(state, trainingTestIntent("effort_promote", ""), &GachaActionResult{}), ErrGachaCultivationLocked)
			require.NoError(t, applyGachaTrainingAction(state, trainingTestIntent("effort_train_max", stat.ID), &GachaActionResult{}))
		}
		require.Equal(t, 10, state.Daily.EffortSpent["c01"])
		require.ErrorIs(t, applyGachaTrainingAction(state, trainingTestIntent("effort_train", "res"), &GachaActionResult{}), ErrGachaCultivationLocked)
	}
	require.NoError(t, applyGachaTrainingAction(state, trainingTestIntent("effort_promote", ""), &GachaActionResult{}))
	assert.Equal(t, 1, own.Training.Stage)
	for _, stat := range gachaTraining.EffortStats {
		assert.Equal(t, 160, own.Training.Effort[stat.ID])
	}
	assert.Zero(t, state.Mats["training_ink"])
	assert.Zero(t, state.Mats["break_seal"])
	assert.Zero(t, state.Gold)
	regenGachaState(state, clock.AddDate(0, 0, 8))
	require.ErrorIs(t, applyGachaTrainingAction(state, trainingTestIntent("effort_train", "spd"), &GachaActionResult{}), ErrGachaFunds)
	assert.Equal(t, 160, own.Training.Effort["spd"])
}

func TestGachaTrainingAffectsRealAttacksAndOldRostersStayFrozen(t *testing.T) {
	state := battleTestState([]string{"c01"}, 60, 4)
	state.previousTraining = true
	own := state.Girls["c01"]
	own.Training = &GachaCardTraining{Grade: 1, Plus: 2, Effort: map[string]int{"hp": 2, "atk": 2, "def": 2, "spd": 2, "res": 2}, Skills: map[string]int{"normal": 4, "rage": 4, "passive": 4}}
	for _, side := range []string{"ally", "enemy"} {
		state.previousMastery = false
		fresh, err := gachaSquadUnits(state, side, "a")
		require.NoError(t, err)
		state.previousMastery = true
		old, err := gachaSquadUnits(state, side, "a")
		require.NoError(t, err)
		assert.Greater(t, fresh[0].MaxHP, old[0].MaxHP)
		assert.Greater(t, fresh[0].Atk, old[0].Atk)
		assert.Equal(t, old[0].Spd+1, fresh[0].Spd)
		assert.Equal(t, old[0].Res+1, fresh[0].Res)
		assert.InDelta(t, old[0].girl.Skills.Normal.Mult*1.16, fresh[0].girl.Skills.Normal.Mult, 0.00001)
		assert.InDelta(t, old[0].girl.Skills.Skill.Mult*1.16, fresh[0].girl.Skills.Skill.Mult, 0.00001)
		assert.Zero(t, old[0].castRefund)
		assert.Equal(t, 8, fresh[0].castRefund)
	}
	state.previousMastery = false
	allies, err := gachaSquadUnits(state, "ally", "a")
	require.NoError(t, err)
	enemies, err := gachaSquadUnits(state, "enemy", "e")
	require.NoError(t, err)
	combat := &gachaCombat{allies: allies, waves: [][]*gachaCombatUnit{enemies}, rng: battleTestRNG(), formula: gachaCatalog.Formula, report: &GachaBattleReport{}, pvp: true}
	allies[0].Rage = 0
	combat.act(allies[0])
	assert.Equal(t, gachaCatalog.Formula.RagePerNormal+12, allies[0].Rage)
	allies[0].Rage = gachaCatalog.Formula.RageMax
	combat.act(allies[0])
	assert.Equal(t, 8, allies[0].Rage)
	require.Len(t, combat.report.Events, 2)
	assert.False(t, combat.report.Events[0].Skill)
	assert.True(t, combat.report.Events[1].Skill)
}

func TestGachaSkillClockDoesNotBankTimeAtCapacityOrOnClockRollback(t *testing.T) {
	base := time.Unix(1000, 0)
	state := newGachaState(base)
	state.SkillPool, state.Daily.SkillSpent = nil, 4
	require.NoError(t, regenGachaSkillPool(state, base))
	assert.Equal(t, 16, state.SkillPool.Points)
	state.SkillPool.Points = 0
	for _, check := range []struct {
		elapsed time.Duration
		points  int
		anchor  time.Duration
	}{{19999 * time.Millisecond, 0, 0}, {20 * time.Second, 1, 20 * time.Second}, {61 * time.Second, 3, 60 * time.Second}, {23 * time.Second, 3, 60 * time.Second}, {time.Hour, 20, time.Hour}} {
		require.NoError(t, regenGachaSkillPool(state, base.Add(check.elapsed)))
		assert.Equal(t, check.points, state.SkillPool.Points)
		assert.Equal(t, base.Add(check.anchor).UnixMilli(), state.SkillPool.TickAt)
	}
	state.Gold = gachaTraining.SkillPool.UpgradeGoldBase
	require.NoError(t, applyGachaSkillAction(state, trainingTestIntent("skill_point", "normal"), &GachaActionResult{}, base.Add(2*time.Hour)))
	require.NoError(t, regenGachaSkillPool(state, base.Add(2*time.Hour+19*time.Second)))
	assert.Equal(t, 19, state.SkillPool.Points)
	require.NoError(t, regenGachaSkillPool(state, base.Add(2*time.Hour+20*time.Second)))
	assert.Equal(t, 20, state.SkillPool.Points)
	state.SkillPool.Points = 21
	require.ErrorIs(t, regenGachaSkillPool(state, base), ErrGachaRunState)
}

func TestGachaSkillLevelZeroThroughMaximumAndRestoredCombat(t *testing.T) {
	state := battleTestState([]string{"c01"}, gachaCatalog.Growth.MaxLevel, 4)
	own := state.Girls["c01"]
	maxLevel := own.Level
	require.Greater(t, maxLevel, 120)
	own.PeakLevel = maxLevel
	own.Training = &GachaCardTraining{Grade: 1, Skills: map[string]int{"normal": maxLevel - 1, "rage": maxLevel, "passive": maxLevel}}
	state.SkillPool = &GachaSkillPool{Points: 7}
	state.Gold = gachaTraining.SkillPool.UpgradeGoldBase + gachaTraining.SkillPool.UpgradeGoldStep*(maxLevel-1)
	base := time.Unix(2000, 0)
	intent := trainingTestIntent("skill_auto", "normal")
	require.NoError(t, applyGachaSkillAction(state, intent, &GachaActionResult{}, base))
	assert.Equal(t, maxLevel, own.Training.Skills["normal"])
	assert.Equal(t, 6, state.SkillPool.Points)
	assert.Zero(t, state.Gold)
	assert.False(t, validGachaSkillRanks(own, false))
	for _, level := range []int{0, 1, 11, maxLevel} {
		own.Level, own.Exp = level, 0
		require.NoError(t, func() error { _, err := gachaSquadUnits(state, "enemy", "e"); return err }())
		units, err := gachaSquadUnits(state, "ally", "a")
		require.NoError(t, err)
		assert.InDelta(t, gachaGirl("c01").Skills.Normal.Mult*(1+float64(level)*0.005), units[0].girl.Skills.Normal.Mult, 0.00001)
		assert.Equal(t, level/5, units[0].castRefund)
		assert.Equal(t, level/4, units[0].rageBonus)
		require.ErrorIs(t, applyGachaSkillAction(state, intent, &GachaActionResult{}, base), ErrGachaCultivationLocked)
		assert.Equal(t, maxLevel, own.Training.Skills["normal"])
	}
	own.Training.Skills["normal"]++
	assert.False(t, validGachaSkillRanks(own, true))
}

package model

import (
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGachaLadderSixSlotGuardsAndLegacyDefense(t *testing.T) {
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, gameZone)
	for _, rank := range []int{1, GachaLadderBots} {
		state := gachaLadderBotState(now, rank)
		require.Len(t, state.Squad, GachaSquadLimit)
		entry := &GachaLadderEntry{ID: -rank, Rank: rank}
		require.NoError(t, gachaLadderDefense(entry, state))
		require.Positive(t, entry.Power)
		require.NoError(t, gachaLadderPublic(entry))
		require.Len(t, entry.Squad, GachaSquadLimit)
		legacy := battleTestState([]string{"c08", "c06", "c01", "c12"}, 40, 3)
		oldEntry := &GachaLadderEntry{ID: 1, Rank: 101}
		require.NoError(t, gachaLadderDefense(oldEntry, legacy))
		frozen := oldEntry.Defense
		require.NoError(t, gachaLadderPublic(oldEntry))
		require.Len(t, oldEntry.Squad, 4)
		report, err := simulateGachaArena(state, legacy, battleTestRNG())
		require.NoError(t, err)
		require.Len(t, report.Allies, GachaSquadLimit)
		require.Len(t, report.Waves[0], 4)
		assert.Equal(t, frozen, oldEntry.Defense, "reading or fighting a legacy defense must not rewrite it")
	}
}

func TestGachaSixSlotCombatAndArena(t *testing.T) {
	ids := []string{"c08", "c04", "c06", "c01", "c12", "c13"}
	state := battleTestState(ids, 40, 3)
	stage := GachaStage{EnemyLv: 40, Waves: 2, EnemyCount: GachaSquadLimit, HasBoss: true, StarRound: 30}
	report, err := simulateGachaCombat(state, stage, battleTestRNG())
	require.NoError(t, err)
	require.Len(t, report.Allies, GachaSquadLimit)
	require.Len(t, report.Waves[0], GachaSquadLimit)
	require.Len(t, report.Waves[1], 1)
	assertBattleReportConsistent(t, report, stage)
	for _, units := range append([][]GachaBattleUnit{report.Allies}, report.Waves...) {
		for slot, unit := range units {
			assert.Equal(t, slot, unit.Slot)
		}
	}
	assert.Equal(t, ids, state.Squad, "combat must never reorder the persisted formation")
	arena, err := simulateGachaArena(state, battleTestState(ids, 40, 3), battleTestRNG())
	require.NoError(t, err)
	require.Len(t, arena.Allies, GachaSquadLimit)
	require.Len(t, arena.Waves[0], GachaSquadLimit)
	assertBattleReportConsistent(t, arena, GachaStage{StarRound: 60})
	for slot, unit := range arena.Waves[0] {
		assert.Equal(t, slot, unit.Slot)
	}
	state.Squad = append(state.Squad, "c02")
	state.Girls["c02"] = &GachaOwnedGirl{Level: 40, Star: 3}
	_, err = gachaSquadUnits(state, "ally", "a")
	require.ErrorIs(t, err, ErrGachaTeam)
}

func TestGachaFormationTargetsKeepRearSlotsProtectedAndStable(t *testing.T) {
	actor := &gachaCombatUnit{GachaBattleUnit: GachaBattleUnit{Slot: 5}}
	foes := make([]*gachaCombatUnit, GachaSquadLimit)
	for slot := range foes {
		foes[slot] = &gachaCombatUnit{GachaBattleUnit: GachaBattleUnit{
			GachaBattleSnapshot: GachaBattleSnapshot{UID: "e" + strconv.Itoa(slot), Alive: true}, Slot: slot,
		}}
	}
	// Reversed iteration must not change target selection.
	reversed := slices.Clone(foes)
	slices.Reverse(reversed)
	for _, slot := range []int{2, 0, 1, 5, 3, 4} {
		target := gachaFormationTarget(actor, reversed)
		require.Same(t, foes[slot], target)
		assert.Equal(t, slot, target.Slot)
		target.Alive = false
	}
	assert.Nil(t, gachaFormationTarget(actor, reversed))
}

func TestGachaFormationTauntOverridesColumn(t *testing.T) {
	combat, caster, _ := battleTestSkillCombat(gachaGirl("c01"))
	enemy, taunter := combat.waves[0][0], combat.waves[0][1]
	caster.Slot, caster.Rage, enemy.Slot = 0, 0, 0
	taunter.Slot, taunter.Taunt = 5, 1
	combat.act(caster)
	require.Len(t, combat.report.Events, 1)
	require.Len(t, combat.report.Events[0].Targets, 1)
	assert.Equal(t, taunter.UID, combat.report.Events[0].Targets[0].UID)
}

func TestGachaFormationIntentAndDualRosterCompatibility(t *testing.T) {
	const requestID = "11111111-1111-4111-8111-111111111111"
	for count := 0; count <= 7; count++ {
		squad := make([]string, count)
		err := ValidateGachaAction(GachaActionIntent{RequestID: requestID, Action: "squad", Squad: &squad})
		if count <= GachaSquadLimit {
			require.NoError(t, err, "size %d must pass intent validation; ownership is checked transactionally", count)
		} else {
			require.ErrorIs(t, err, ErrGachaIntent)
		}
	}
	ids := []string{"c01", "c02", "c03", "c04", "c05", "c06", "c07", "c08", "c09", "c10", "c11", "c12"}
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, gameZone)
	for count := 7; count <= 13; count++ {
		squad := make([]string, count)
		err := ValidateGachaAction(GachaActionIntent{RequestID: requestID, Action: "teams_save", Squad: &squad})
		if count < 8 || count > 12 || count%2 != 0 {
			require.ErrorIs(t, err, ErrGachaIntent)
			continue
		}
		require.NoError(t, err)
		state := battleTestState(ids, 40, 3)
		state.Progression = &GachaProgressionState{}
		state.Cleared["s6"] = true
		squad = slices.Clone(ids[:count])
		require.NoError(t, applyGachaProgressionAction(nil, 1, state, GachaActionIntent{Action: "teams_save", Squad: &squad}, &GachaActionResult{}, now))
		assert.Equal(t, squad[:count/2], state.Squad)
		assert.Equal(t, squad[count/2:], state.Progression.ReserveSquad)
		_, snapshot, err := specialGachaProgressionStage("dual_trial", state, now)
		require.NoError(t, err)
		assert.Equal(t, state.Squad, snapshot.DualSquads[0])
		assert.Equal(t, state.Progression.ReserveSquad, snapshot.DualSquads[1])
		frozen := slices.Clone(snapshot.DualSquads[1])
		state.Progression.ReserveSquad[0] = state.Squad[0]
		state.Progression.Dual.Phase = 1
		_, next, err := specialGachaProgressionStage("dual_trial", state, now)
		require.NoError(t, err)
		assert.Equal(t, frozen, next.Squad, "second encounter uses the locked roster, including old four-person trials")
	}
}

package model

import (
	"github.com/QuantumNous/new-api/common"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestJoineryRealBattlePresentationFixture(t *testing.T) {
	state := battleTestState([]string{"xs039", "c13", "c01"}, 40, 5)
	state.Girls["xs039"].Talent = "swift"
	state.Girls["xs039"].WeaponLevel = 10
	state.Girls["xs039"].WeaponAwake = 2
	state.Girls["xs039"].Awakening = 3
	stage := GachaStage{ID: "", Name: "三榫弦场演练", EnemyLv: 10, Waves: 1, EnemyCount: 6, HPScale: 20, StarRound: 60}
	report, err := simulateGachaCombat(state, stage, battleTestRNG())
	require.NoError(t, err)
	ultimate, cut, rescue := 0, 0, 0
	for _, event := range report.Events {
		if event.SkillKey == "xs039-skill-2" {
			ultimate++
		}
		if event.Variant == "cut" {
			cut++
		}
		if event.Variant == "rescue" {
			rescue++
		}
	}
	require.Positive(t, ultimate)
	require.Positive(t, cut)
	t.Logf("server-generated presentation fixture: %d events, %d ultimates, %d cuts, %d rescues", len(report.Events), ultimate, cut, rescue)
	if directory := os.Getenv("GACHA_REVIEW_OUTPUT"); directory != "" {
		data, err := common.Marshal(map[string]any{"state": state, "report": report, "stage": stage})
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(directory, "xs039-real-battle.json"), data, 0644))
	}
}

func joineryBattle(t *testing.T) (*gachaCombat, *gachaCombatUnit) {
	t.Helper()
	card := *gachaGirl("xs039")
	b := &gachaCombat{formula: gachaCatalog.Formula, rng: rand.New(rand.NewPCG(39, 91)), rounds: 1, report: &GachaBattleReport{}}
	unit := func(id, side string, slot int) *gachaCombatUnit {
		return &gachaCombatUnit{GachaBattleUnit: GachaBattleUnit{GachaBattleSnapshot: GachaBattleSnapshot{UID: id, HP: 10000, Alive: true}, Slot: slot, Side: side, MaxHP: 10000, Atk: 100, Def: 0, Spd: 60, Level: 1, CD: 150}, healBoost: 1}
	}
	owner := unit("a0", "ally", 0)
	owner.GirlID = "xs039"
	owner.Name = card.Name
	owner.girl = &card
	owner.SpAtk = 100
	owner.Atk = 50
	b.allies = []*gachaCombatUnit{owner, unit("a1", "ally", 1), unit("a2", "ally", 2)}
	b.waves = [][]*gachaCombatUnit{{}}
	for i := 0; i < 6; i++ {
		b.waves[0] = append(b.waves[0], unit("e"+strconv.Itoa(i), "enemy", i))
	}
	for _, u := range b.allies {
		b.report.Allies = append(b.report.Allies, u.GachaBattleUnit)
	}
	b.report.Waves = [][]GachaBattleUnit{{}}
	for _, u := range b.waves[0] {
		b.report.Waves[0] = append(b.report.Waves[0], u.GachaBattleUnit)
	}
	b.initJoinery()
	return b, owner
}

func TestJoineryCatalogAndFiveStarSynthesis(t *testing.T) {
	girl := gachaGirl("xs039")
	require.NotNil(t, girl)
	require.Equal(t, 5, gachaInitialStars(girl))
	for _, source := range []string{"summon", "growth", "voyage", "starter", "crew"} {
		require.False(t, gachaCardHasSource(girl, source))
	}
	now := time.Date(2026, 11, 30, 12, 0, 0, 0, gameZone)
	state := newGachaState(now)
	require.NoError(t, normalizeGachaProgression(state, now, DefaultGachaProgressionConfig()))
	require.NoError(t, grantGachaSourceShards(state, girl, "world", 80))
	require.False(t, gachaCardOwned(state.Girls[girl.ID]))
	id := girl.ID
	require.NoError(t, applyGachaProgressionAction(nil, 1, state, GachaActionIntent{Action: "shard_unlock", GirlID: &id}, &GachaActionResult{}, now))
	require.Equal(t, 5, state.Girls[id].Star)
	require.Zero(t, state.Girls[id].Shards)
	state.Squad = []string{id}
	state.legacyArenaStats = true
	units, err := gachaSquadUnits(state, "ally", "a")
	require.NoError(t, err)
	u := units[0]
	require.Equal(t, 736, u.MaxHP)
	require.Equal(t, 46, u.Atk)
	require.Equal(t, 59, u.Def)
	require.Equal(t, 69, u.Spd)
	require.Equal(t, 78, u.SpAtk)
	require.Equal(t, 87, u.SpDef)
}

func TestJoineryRescueBranchTargetsSwapHealAndCleanse(t *testing.T) {
	for _, bound := range []bool{false, true} {
		t.Run(strconv.FormatBool(bound), func(t *testing.T) {
			b, u := joineryBattle(t)
			patient, other := b.allies[1], b.allies[2]
			patient.HP = 1000
			other.HP = 9000
			u.HP = 8000
			patient.StunT = 2
			patient.DotT = 2
			patient.Dot = 80
			if bound {
				patient.Joinery.Bound = 2
			}
			e := b.actJoinery(u, gachaLivingUnits(b.allies), gachaLivingUnits(b.waves[0]))
			require.Equal(t, "rescue", e.Variant)
			require.Equal(t, 1150, patient.HP)
			require.Zero(t, patient.StunT)
			require.Equal(t, 2, patient.DotT)
			require.Equal(t, 1, u.Joinery.Peg)
			require.Equal(t, 3, u.Joinery.Cooldown)
			if bound {
				require.Equal(t, 1, patient.Slot)
				require.Equal(t, 2, other.Slot)
			} else {
				require.Equal(t, 2, patient.Slot)
				require.Equal(t, 1, other.Slot)
			}
			for _, enemy := range b.waves[0] {
				require.Equal(t, 10000, enemy.HP)
				require.Empty(t, enemy.Joinery.Pulses)
			}
		})
	}
	b, u := joineryBattle(t)
	b.allies = b.allies[:1]
	u.HP = 1000
	u.StunT = 1
	e := b.actJoinery(u, b.allies, b.waves[0])
	require.Equal(t, "rescue", e.Variant)
	require.Equal(t, 1150, u.HP)
	require.Zero(t, u.StunT)
	require.Zero(t, u.Slot)
}

func TestJoineryCutCooldownAndPointEconomy(t *testing.T) {
	b, u := joineryBattle(t)
	b.waves[0][3].Atk = 200
	b.waves[0][4].Atk = 200
	e := b.actJoinery(u, b.allies, b.waves[0])
	require.Equal(t, "cut", e.Variant)
	require.Equal(t, "e3", e.Targets[0].UID)
	require.Equal(t, 160, e.Targets[0].Damage)
	require.Len(t, b.waves[0][3].Joinery.Pulses, 1)
	require.Equal(t, 2, b.waves[0][3].Joinery.Pulses[0].Countdown)
	u.Joinery.Peg = 0
	u.Rage = 0
	e = b.actJoinery(u, b.allies, b.waves[0])
	require.Equal(t, "normal", e.SkillKey)
	require.Equal(t, 25, u.Rage)
	require.Equal(t, 1, u.Joinery.Peg)
	b.actJoinery(u, b.allies, b.waves[0])
	require.Equal(t, 1, u.Joinery.Peg, "only one normal point per round")
	u.Joinery.Peg = 0
	u.Rage = 100
	e = b.actJoinery(u, b.allies, b.waves[0])
	require.Equal(t, "xs039-skill-2", e.SkillKey)
	require.Equal(t, "pierce", e.Variant)
	u.Joinery.Peg = 3
	u.Rage = 0
	u.Joinery.Cooldown = 3
	b.act(u)
	require.Equal(t, 2, u.Joinery.Cooldown)
	require.Equal(t, "normal", b.report.Events[len(b.report.Events)-1].SkillKey)
}

func TestJoineryUltimateColumnSpecialDefenseAndOneNullification(t *testing.T) {
	for _, pegs := range []int{0, 1, 2, 3} {
		t.Run(strconv.Itoa(pegs), func(t *testing.T) {
			b, u := joineryBattle(t)
			u.Rage = 100
			u.Joinery.Peg = pegs
			front, back := b.waves[0][0], b.waves[0][3]
			front.Joinery.Bound = 2
			front.Def = 999999
			front.SpDef = 100
			back.Joinery.Nullify = 1
			e := b.actJoinery(u, b.allies, b.waves[0])
			require.Len(t, e.Targets, 2)
			require.Equal(t, "e0", e.Targets[0].UID)
			require.Equal(t, "e3", e.Targets[1].UID)
			require.Equal(t, 201, e.Targets[0].Damage, "250 power mitigated by 75 special defense and 310 defense constant")
			require.True(t, e.Targets[1].Blocked)
			require.Zero(t, back.Joinery.Nullify)
			require.Equal(t, 10000, back.HP)
			for _, index := range []int{1, 2, 4, 5} {
				require.Equal(t, 10000, b.waves[0][index].HP)
			}
			require.Zero(t, u.Rage)
			if pegs == 3 {
				require.Equal(t, "field", e.Variant)
				require.Equal(t, 1, u.Joinery.Peg)
				require.Equal(t, 2, u.Joinery.FieldUntil)
			} else {
				require.Equal(t, "pierce", e.Variant)
				require.Equal(t, pegs, u.Joinery.Peg)
				require.Zero(t, u.Joinery.FieldUntil)
			}
		})
	}
}

func TestJoineryPulsesTickAfterDOTSpreadOnceAndKeepPower(t *testing.T) {
	b, u := joineryBattle(t)
	target := b.waves[0][1]
	require.True(t, b.joineryImplant(u, target, 100, false))
	u.SpAtk = 900
	target.Dot, target.DotT = 5, 2
	b.act(target)
	require.Equal(t, 9995, target.HP)
	require.Equal(t, 1, target.Joinery.Pulses[0].Countdown)
	b.act(target)
	require.Equal(t, 9890, target.HP)
	require.Equal(t, 9970, b.waves[0][0].HP)
	require.Equal(t, 9970, b.waves[0][2].HP)
	require.Equal(t, 10000, b.waves[0][4].HP, "diagonal/other row is not adjacent")
	var derived *gachaCombatUnit
	for _, enemy := range b.waves[0] {
		for _, pulse := range enemy.Joinery.Pulses {
			require.True(t, pulse.Derived)
			require.Equal(t, 100.0, pulse.Power)
			derived = enemy
		}
	}
	require.NotNil(t, derived)
	before := derived.HP
	b.joineryBeforeAction(derived)
	b.joineryBeforeAction(derived)
	require.Equal(t, before-70, derived.HP)
	for _, enemy := range b.waves[0] {
		require.Empty(t, enemy.Joinery.Pulses)
	}
}

func TestJoineryPulseCapsCleansingDeathAndSuppression(t *testing.T) {
	b, u := joineryBattle(t)
	foes := b.waves[0]
	for _, target := range []*gachaCombatUnit{foes[0], foes[0], foes[1], foes[1]} {
		require.True(t, b.joineryImplant(u, target, 100, false))
	}
	require.False(t, b.joineryImplant(u, foes[0], 100, false))
	require.False(t, b.joineryImplant(u, foes[2], 100, false))
	b.joineryCleanse(foes[0], 1)
	require.Len(t, foes[0].Joinery.Pulses, 1)
	require.Equal(t, 10000, foes[0].HP)
	u.Joinery.Suppressed = 2
	foes[0].HP = 0
	foes[0].Alive = false
	b.flushJoinery()
	require.Empty(t, foes[0].Joinery.Pulses)
	require.Equal(t, 9970, foes[1].HP)
	for _, target := range foes {
		for _, pulse := range target.Joinery.Pulses {
			require.False(t, pulse.Derived)
		}
	}
	u.Joinery.Suppressed = 0
	u.Alive = false
	b.flushJoinery()
	for _, friend := range b.allies {
		require.Nil(t, friend.Joinery.Mark)
	}
}

func TestJoineryFieldScheduleCooldownCostExpiryAndControl(t *testing.T) {
	b, u := joineryBattle(t)
	u.Joinery.Peg = 3
	u.Rage = 100
	b.allies[1].Joinery.Cooldown = 3
	b.allies[2].Joinery.Cooldown = 3
	b.joineryImplant(u, b.waves[0][2], 100, false)
	b.actJoinery(u, b.allies, b.waves[0])
	u.Joinery.Reduction = 25
	require.InDelta(t, .85, b.joineryReduction(u), .0001)
	b.joineryAfterAction(u, true)
	require.Equal(t, 2, b.allies[1].Joinery.Cooldown)
	require.Equal(t, 3, b.allies[2].Joinery.Cooldown)
	require.Equal(t, 1, b.waves[0][0].Joinery.Cooldown)
	require.Equal(t, 1, b.waves[0][2].Joinery.Pulses[0].Countdown)
	b.joineryAfterAction(u, true)
	require.Equal(t, 2, b.allies[1].Joinery.Cooldown, "at most once per round")
	b.joineryEndRound()
	require.Equal(t, 2, u.Joinery.FieldUntil)
	b.rounds = 2
	u.StunT = 1
	b.joineryAfterAction(u, false)
	require.Equal(t, 2, u.Joinery.FieldUntil)
	require.Equal(t, 2, b.allies[1].Joinery.Cooldown)
	b.joineryEndRound()
	require.Zero(t, u.Joinery.FieldUntil)
	u.Joinery.FieldUntil = 5
	u.Joinery.Suppressed = 1
	b.flushJoinery()
	require.Zero(t, u.Joinery.FieldUntil)
	// A legacy card delayed from 0 to 1 cooldown really loses its next skill window.
	legacy := b.waves[0][0]
	card := *gachaGirl("c04")
	legacy.girl = &card
	legacy.Rage = 100
	legacy.Joinery.Cooldown = 1
	b.act(legacy)
	last := b.report.Events[len(b.report.Events)-1]
	require.False(t, last.Skill)
	require.Equal(t, 100, legacy.Rage)
}

func TestJoineryLethalProtectionFiniteAndSubsequentHitsContinue(t *testing.T) {
	for _, mode := range []string{"normal", "no-points", "suppressed", "spent", "owner-dead"} {
		t.Run(mode, func(t *testing.T) {
			b, u := joineryBattle(t)
			patient := b.allies[1]
			require.NotNil(t, patient.Joinery.Mark)
			patient.HP = 20
			patient.Rage = 80
			patient.StunT = 3
			patient.Joinery.HealBlock = 3
			patient.DotT = 3
			switch mode {
			case "no-points":
				u.Joinery.Peg = 0
			case "suppressed":
				u.Joinery.Suppressed = 1
			case "spent":
				patient.Joinery.RescueUsed = true
			case "owner-dead":
				u.Alive = false
			}
			hit := b.damage(b.waves[0][0], patient, 100, 0)
			if mode != "normal" {
				require.False(t, patient.Alive)
				require.False(t, hit.Rescued)
				return
			}
			require.True(t, hit.Rescued)
			require.Equal(t, 4000, patient.HP)
			require.Equal(t, 80, patient.Rage)
			require.Equal(t, 1, u.Joinery.Peg)
			require.Nil(t, patient.Joinery.Mark)
			require.Zero(t, patient.StunT)
			require.Zero(t, patient.Joinery.HealBlock)
			require.Equal(t, 3, patient.DotT)
			require.Equal(t, 1, patient.Joinery.Guard)
			require.True(t, patient.Joinery.RescueUsed)
			second := b.damage(b.waves[0][0], patient, 100, 0)
			require.False(t, second.Rescued)
			require.False(t, patient.Alive)
		})
	}
	b, u := joineryBattle(t)
	u.HP = 1
	b.damage(b.waves[0][0], u, 10, 0)
	require.True(t, u.Alive, "owner's own stored mark can rescue before alive is cleared")
}

func TestJoineryDebuffFollowUpCountsFreshKindsAndHonorsControl(t *testing.T) {
	b, u := joineryBattle(t)
	patient := b.allies[2]
	patient.HP = 100
	patient.StunT = 2
	b.joineryNewDebuff(patient, false)
	patient.SlowT = 2
	patient.Slow = 20
	b.joineryNewDebuff(patient, false)
	patient.DotT = 2
	patient.Dot = 10
	b.joineryNewDebuff(patient, false)
	require.Zero(t, patient.StunT)
	require.Equal(t, 1, u.Joinery.CleanRound)
	patient.SlowT = 4
	patient.DotT = 4
	b.joineryNewDebuff(patient, true)
	require.Zero(t, u.Joinery.DebuffMeter)
	u.StunT = 2
	patient.DefBreakT = 2
	b.joineryNewDebuff(patient, false)
	require.Zero(t, u.Joinery.DebuffMeter)
	u.StunT = 0
	patient.StunT = 1
	b.joineryNewDebuff(patient, false)
	patient.StunT = 0 // A later hit cleansing the status cannot erase its application.
	require.Equal(t, 1, u.Joinery.DebuffMeter)
}

func TestJoinerySnapshotsAreImmutableAndBothPvPSidesWork(t *testing.T) {
	b, u := joineryBattle(t)
	b.joineryImplant(u, b.waves[0][0], 100, false)
	b.record(GachaBattleEvent{Kind: "status"})
	snapshot := b.report.Events[0].Units[len(b.allies)]
	b.waves[0][0].Joinery.Pulses[0].Countdown = 0
	require.Equal(t, 2, snapshot.Joinery.Pulses[0].Countdown)
	state := battleTestState([]string{"xs039", "c04", "c13", "c01", "c07", "c15"}, 50, 5)
	for seed := 0; seed < 120; seed++ {
		report, err := simulateGachaArena(state, state, rand.New(rand.NewPCG(uint64(seed+1), 39)))
		require.NoError(t, err)
		require.NotNil(t, report.Allies[0].Joinery.Mark)
		require.NotNil(t, report.Waves[0][0].Joinery.Mark)
		require.Equal(t, 2, report.Allies[0].Joinery.Peg)
		require.Equal(t, 2, report.Waves[0][0].Joinery.Peg)
		require.Less(t, len(report.Events), 10000)
		for _, event := range report.Events {
			positions := map[string]map[int]bool{"a": {}, "e": {}}
			for _, unit := range event.Units {
				require.GreaterOrEqual(t, unit.HP, 0)
				require.GreaterOrEqual(t, unit.Rage, 0)
				require.LessOrEqual(t, unit.Rage, 100)
				if unit.Position != nil {
					side := unit.UID[:1]
					require.False(t, positions[side][*unit.Position])
					positions[side][*unit.Position] = true
				}
				if unit.Joinery != nil {
					require.LessOrEqual(t, unit.Joinery.Peg, 3)
					require.LessOrEqual(t, len(unit.Joinery.Pulses), 2)
				}
			}
		}
	}
}

func TestWorldExplicitRotationRejectsInvalidPoolsAndKeepsHistoricalSchedule(t *testing.T) {
	start := time.Date(2026, 11, 30, 0, 0, 0, 0, gameZone)
	require.Equal(t, "xs039", gachaWorldSchedule(start).Hero)
	cycle := gachaWorldRotationAt(start).Heroes
	for i := 0; i < 400; i++ {
		require.Equal(t, cycle[i%len(cycle)], gachaWorldSchedule(start.AddDate(0, 0, i*7)).Hero)
	}
	for _, heroes := range [][]string{nil, {}, {"xs039", "xs039"}, {"missing"}, {"c01"}, {"c04"}} {
		require.False(t, validGachaWorldRotations([]GachaWorldRotation{{Starts: "2026-11-30", Heroes: heroes}}))
	}
	require.True(t, validGachaWorldRotations([]GachaWorldRotation{{Starts: "2026-11-30", Heroes: []string{"xs039"}}}))
	legacy := gachaWorldRotationAt(start.AddDate(0, 0, -7)).Heroes
	require.Equal(t, []string{"c03", "c15", "c27"}, legacy)
	require.False(t, slices.Contains(legacy, "xs039"))
}

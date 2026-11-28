package model

import (
	"math"
	"math/rand/v2"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func battleTestState(ids []string, level, star int) *GachaGameState {
	state := &GachaGameState{
		Squad: slices.Clone(ids), Girls: make(map[string]*GachaOwnedGirl),
		Stamina: 83, Gold: 1729, Multi: 2, MultiOn: true,
		Cleared: map[string]bool{"s1": true}, Stars: map[string]int{"s1": 3},
		Mats: map[string]int{"mat_r": 9}, History: []GachaPullHistory{{Name: "previous", Rarity: "R"}},
	}
	for _, id := range ids {
		state.Girls[id] = &GachaOwnedGirl{Level: level, Star: star, Exp: 17, Shards: 23}
	}
	return state
}

func battleTestRNG() *rand.Rand { return rand.New(rand.NewPCG(47, 91)) }

func TestGachaMatchmakingPowerGapIsSymmetricAndNeverExceedsTwentyPercent(t *testing.T) {
	for _, pair := range []struct {
		a, b    int
		allowed bool
	}{
		{10_000, 12_000, true}, {10_000, 12_001, false},
		{10_000, 8_334, true}, {10_000, 8_333, false},
		{1, 1, true}, {1, 2, false},
	} {
		for _, direction := range [][2]int{{pair.a, pair.b}, {pair.b, pair.a}} {
			minimum, maximum := gachaArenaPowerRange(direction[0])
			assert.Equal(t, pair.allowed, direction[1] >= minimum && direction[1] <= maximum, "%v", direction)
		}
	}
}

func TestGachaArenaUsesBothPlayerSkillSetsAndPrivateDeterministicCombat(t *testing.T) {
	attack := battleTestState([]string{"c08", "c13", "c06", "c01"}, 40, 3)
	defense := battleTestState([]string{"c08", "c13", "c06", "c01"}, 40, 3)
	report, err := simulateGachaArena(attack, defense, battleTestRNG())
	require.NoError(t, err)
	again, err := simulateGachaArena(attack, defense, battleTestRNG())
	require.NoError(t, err)
	assert.Equal(t, report, again)
	assert.Equal(t, battleTestState([]string{"c08", "c13", "c06", "c01"}, 40, 3), attack)
	assert.Equal(t, attack, defense, "neither saved lineup is mutated by combat")
	require.Len(t, report.Waves, 1)
	require.Len(t, report.Waves[0], 4)
	for i, u := range report.Waves[0] {
		assert.Equal(t, defense.Squad[i], u.GirlID)
		assert.Equal(t, report.Allies[i].MaxHP, u.MaxHP)
		assert.Equal(t, report.Allies[i].Atk, u.Atk)
		assert.Equal(t, "enemy", u.Side)
	}
	duration := 0
	allySkill, enemySkill := false, false
	for _, event := range report.Events {
		duration += event.DurationMS
		if event.Skill {
			allySkill = allySkill || event.Src[0] == 'a'
			enemySkill = enemySkill || event.Src[0] == 'e'
		}
	}
	assert.True(t, allySkill)
	assert.True(t, enemySkill, "defenders must cast their owned character skills")
	assert.Equal(t, duration, report.DurationMS)
	assert.LessOrEqual(t, report.Rounds, 60)
	strong := battleTestState([]string{"c04", "c07", "c09", "c03"}, 60, 5)
	weak := battleTestState([]string{"c01"}, 1, 1)
	win, err := simulateGachaArena(strong, weak, battleTestRNG())
	require.NoError(t, err)
	assert.True(t, win.Win)
	loss, err := simulateGachaArena(weak, strong, battleTestRNG())
	require.NoError(t, err)
	assert.False(t, loss.Win)
	assert.False(t, loss.TimedOut)
}

func TestGachaArenaNormalAttacksChargeRageEquallyOnBothSides(t *testing.T) {
	state := battleTestState([]string{"c08"}, 20, 2)
	allies, err := gachaSquadUnits(state, "ally", "a")
	require.NoError(t, err)
	enemies, err := gachaSquadUnits(state, "enemy", "e")
	require.NoError(t, err)
	b := &gachaCombat{allies: allies, waves: [][]*gachaCombatUnit{enemies}, rng: battleTestRNG(), formula: gachaCatalog.Formula, report: &GachaBattleReport{}, pvp: true}
	b.act(allies[0])
	b.act(enemies[0])
	assert.Equal(t, gachaCatalog.Formula.RagePerNormal, allies[0].Rage)
	assert.Equal(t, allies[0].Rage, enemies[0].Rage)
}

func TestGachaBattleDeterministicAndReadOnly(t *testing.T) {
	state := battleTestState([]string{"c04", "c07", "c09", "c03"}, 60, 5)
	before := battleTestState(slices.Clone(state.Squad), 60, 5)
	stage := *gachaStage("s6")
	first, err := simulateGachaCombat(state, stage, battleTestRNG())
	require.NoError(t, err)
	second, err := simulateGachaCombat(state, stage, battleTestRNG())
	require.NoError(t, err)
	assert.Equal(t, first, second, "same private seed must produce the same complete report")
	assert.Equal(t, before, state, "combat changed the authoritative input state")
}

func TestGachaBattleInitialUnits(t *testing.T) {
	state := battleTestState([]string{"c01", "c06"}, 7, 3)
	stage := GachaStage{EnemyLv: 11, Waves: 2, EnemyCount: 3, HasBoss: true, StarRound: 8}
	report, err := simulateGachaCombat(state, stage, battleTestRNG())
	require.NoError(t, err)
	for i, unit := range report.Allies {
		girl := gachaGirl(state.Squad[i])
		growth := gachaCatalog.Growth
		stat := func(base int, levelGrow, starGrow float64) int {
			return int(math.Round(float64(base) * (1 + 6*levelGrow) * (1 + 2*starGrow)))
		}
		if unit.UID != "a"+strconv.Itoa(i) || unit.Side != "ally" || unit.GirlID != girl.ID ||
			unit.Name != girl.Name || unit.Img != girl.Img || unit.AtkType != girl.AtkType || unit.Boss || unit.Level != 7 {
			require.Failf(t, "battle invariant failed", "incorrect ally identity: %+v", unit)
		}
		if unit.MaxHP != stat(girl.Base.HP, growth.LevelStatGrow.HP, growth.StarStatGrow.HP) ||
			unit.Atk != stat(girl.Base.Atk, growth.LevelStatGrow.Atk, growth.StarStatGrow.Atk)+6 ||
			unit.Def != stat(girl.Base.Def, growth.LevelStatGrow.Def, growth.StarStatGrow.Def) ||
			unit.Spd != stat(girl.Base.Spd, growth.LevelStatGrow.Spd, growth.StarStatGrow.Spd) ||
			unit.CR != girl.Base.CR || unit.CD != girl.Base.CD || unit.EHR != girl.Base.EHR || unit.Res != girl.Base.Res {
			require.Failf(t, "battle invariant failed", "incorrect grown ally stats: %+v", unit)
		}
		if unit.HP != unit.MaxHP || !unit.Alive || unit.Rage != []int{6, 5}[i] || unit.Shield != 0 {
			require.Failf(t, "battle invariant failed", "initial ally snapshot is not pristine: %+v", unit)
		}
	}
	if len(report.Waves) != 2 || len(report.Waves[0]) != 3 || len(report.Waves[1]) != 1 {
		require.Failf(t, "battle invariant failed", "wrong normal/boss wave sizes: %+v", report.Waves)
	}
	for wave, units := range report.Waves {
		for i, unit := range units {
			template := gachaCatalog.Enemies[(wave*stage.EnemyCount+i)%3]
			if wave == 1 {
				template = gachaCatalog.Enemies[len(gachaCatalog.Enemies)-1]
			}
			if unit.UID != "e"+strconv.Itoa(i) || unit.Side != "enemy" || unit.Name != template.Name ||
				unit.Img != template.Img || unit.AtkType != template.AtkType || unit.Boss != (wave == 1) ||
				unit.MaxHP != int(math.Round(260*(1+11*0.10)*template.HPM)) ||
				unit.Atk != int(math.Round(55*(1+11*0.08)*template.AtkM)) ||
				unit.Def != int(math.Round(15*(1+11*0.06)*template.DefM)) || unit.Spd != template.Spd ||
				unit.HP != unit.MaxHP || !unit.Alive || unit.Rage != 0 {
				require.Failf(t, "battle invariant failed", "incorrect initial enemy wave=%d: %+v", wave, unit)
			}
		}
	}
}

func TestGachaBattleRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name   string
		change func(*GachaGameState, *GachaStage)
	}{
		{"empty squad", func(s *GachaGameState, _ *GachaStage) { s.Squad = nil }},
		{"too many allies", func(s *GachaGameState, _ *GachaStage) {
			s.Squad = []string{"c01", "c02", "c03", "c04", "c05", "c06", "c07"}
		}},
		{"duplicate ally", func(s *GachaGameState, _ *GachaStage) { s.Squad = []string{"c01", "c01"} }},
		{"missing ownership", func(s *GachaGameState, _ *GachaStage) { delete(s.Girls, "c01") }},
		{"locked girl", func(s *GachaGameState, _ *GachaStage) { s.Girls["c01"].Star = 0 }},
		{"zero level", func(s *GachaGameState, _ *GachaStage) { s.Girls["c01"].Level = 0 }},
		{"over level", func(s *GachaGameState, _ *GachaStage) { s.Girls["c01"].Level = gachaCatalog.Growth.MaxLevel + 1 }},
		{"over star", func(s *GachaGameState, _ *GachaStage) { s.Girls["c01"].Star = gachaCatalog.Growth.StarMax + 1 }},
		{"unknown girl", func(s *GachaGameState, _ *GachaStage) {
			s.Squad[0] = "unknown"
			s.Girls["unknown"] = &GachaOwnedGirl{Level: 1, Star: 1}
		}},
		{"zero waves", func(_ *GachaGameState, s *GachaStage) { s.Waves = 0 }},
		{"excess waves", func(_ *GachaGameState, s *GachaStage) { s.Waves = 9 }},
		{"zero enemies", func(_ *GachaGameState, s *GachaStage) { s.EnemyCount = 0 }},
		{"excess enemies", func(_ *GachaGameState, s *GachaStage) { s.EnemyCount = GachaSquadLimit + 1 }},
		{"zero enemy level", func(_ *GachaGameState, s *GachaStage) { s.EnemyLv = 0 }},
		{"excess enemy level", func(_ *GachaGameState, s *GachaStage) { s.EnemyLv = 201 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := battleTestState([]string{"c01"}, 1, 1)
			stage := *gachaStage("s1")
			tt.change(state, &stage)
			if _, err := simulateGachaCombat(state, stage, battleTestRNG()); err == nil {
				require.Fail(t, "invalid input was accepted")
			}
		})
	}
	if _, err := simulateGachaCombat(nil, *gachaStage("s1"), battleTestRNG()); err == nil {
		require.Fail(t, "nil state was accepted")
	}
	if _, err := simulateGachaCombat(battleTestState([]string{"c01"}, 1, 1), *gachaStage("s1"), nil); err == nil {
		require.Fail(t, "nil random source was accepted")
	}
}

func assertBattleReportConsistent(t *testing.T, report *GachaBattleReport, stage GachaStage) {
	t.Helper()
	require.Equal(t, gachaBattleReportVersion, report.Version)
	if report.Rounds < 1 || report.Rounds > 60 || report.DurationMS <= 0 || report.DurationMS >= int(GachaRunTTL/time.Millisecond) || report.TotalN != len(report.Allies) {
		require.Failf(t, "battle invariant failed", "invalid combat limits: rounds=%d duration=%d allies=%d", report.Rounds, report.DurationMS, report.TotalN)
	}
	units := make(map[string]GachaBattleSnapshot)
	maxHP := make(map[string]int)
	for _, unit := range report.Allies {
		units[unit.UID], maxHP[unit.UID] = unit.GachaBattleSnapshot, unit.MaxHP
	}
	wave, timeline, lastRound := -1, 0, 0
	for index, event := range report.Events {
		require.Equal(t, index+1, event.Seq)
		minimumDuration := 450
		if event.Kind == "round" {
			minimumDuration = 0
		} else if event.Kind == "ailment" {
			minimumDuration = 250
		} else if event.Kind == "aura" {
			minimumDuration = 350
		}
		if event.Round < lastRound || event.Round > report.Rounds || event.DurationMS < minimumDuration {
			require.Failf(t, "battle invariant failed", "invalid event ordering or duration: %+v", event)
		}
		lastRound, timeline = event.Round, timeline+event.DurationMS
		if event.Kind == "wave" {
			if event.Wave != wave+1 || event.Wave >= len(report.Waves) {
				require.Failf(t, "battle invariant failed", "invalid wave transition: %d -> %d", wave, event.Wave)
			}
			wave = event.Wave
			for uid := range units {
				if uid[0] == 'e' {
					delete(units, uid)
					delete(maxHP, uid)
				}
			}
			for _, unit := range report.Waves[wave] {
				units[unit.UID], maxHP[unit.UID] = unit.GachaBattleSnapshot, unit.MaxHP
			}
		} else if event.Kind == "round" && event.Wave == wave {
			seenOrder := make(map[string]bool)
			for _, uid := range event.Order {
				require.True(t, units[uid].Alive, "round order contains an absent or dead unit: %s", uid)
				require.False(t, seenOrder[uid], "round order duplicates a unit: %s", uid)
				seenOrder[uid] = true
			}
			for uid, unit := range units {
				require.Equal(t, unit.Alive, seenOrder[uid], "round order must include every living unit")
			}
			require.Zero(t, event.DurationMS)
		} else if !slices.Contains([]string{"action", "ailment", "aura"}, event.Kind) || event.Wave != wave {
			require.Failf(t, "battle invariant failed", "unknown or misplaced action event: %+v", event)
		} else if src, ok := units[event.Src]; !ok || !src.Alive {
			require.Failf(t, "battle invariant failed", "action source is absent or dead: %s", event.Src)
		}
		if event.Kind == "action" && event.Skill {
			src := units[event.Src]
			src.Rage = 0
			units[event.Src] = src
		}
		for _, target := range event.Targets {
			unit, ok := units[target.UID]
			if !ok || target.Damage < 0 || target.Heal < 0 || target.Shield < 0 || target.Absorbed < 0 ||
				target.SelfHeal < 0 || target.Reflected < 0 || target.RageDelta < -gachaCatalog.Formula.RageMax || target.RageDelta > gachaCatalog.Formula.RageMax {
				require.Failf(t, "battle invariant failed", "invalid action target: %+v", target)
			}
			unit.HP += target.Heal - target.Damage
			unit.Shield += target.Shield - target.Absorbed
			unit.Rage += target.RageDelta
			units[target.UID] = unit
			if target.SelfHeal != 0 || target.Reflected != 0 {
				src := units[event.Src]
				src.HP += target.SelfHeal - target.Reflected
				units[event.Src] = src
			}
		}
		if len(event.Units) != len(units) {
			require.Failf(t, "battle invariant failed", "event snapshot omitted units: got=%d want=%d", len(event.Units), len(units))
		}
		seen := make(map[string]bool)
		for _, snapshot := range event.Units {
			previous, ok := units[snapshot.UID]
			if !ok || seen[snapshot.UID] || snapshot.HP != previous.HP || snapshot.Shield != previous.Shield ||
				snapshot.HP < 0 || snapshot.HP > maxHP[snapshot.UID] || snapshot.Shield < 0 ||
				snapshot.Alive != (snapshot.HP > 0) || snapshot.Rage < 0 || snapshot.Rage > gachaCatalog.Formula.RageMax ||
				((snapshot.UID != event.Src || event.Kind == "action" && event.Skill) && snapshot.Rage != previous.Rage) || snapshot.Dot < 0 || snapshot.DotT < 0 || snapshot.DotT > 2 ||
				snapshot.ReflectPct < 0 || snapshot.ReflectPct > 100 || snapshot.ReflectT < 0 || snapshot.ReflectT > 2 {
				require.Failf(t, "battle invariant failed", "target deltas and authoritative snapshot disagree: previous=%+v snapshot=%+v", previous, snapshot)
			}
			seen[snapshot.UID], units[snapshot.UID] = true, snapshot
		}
	}
	if report.DurationMS != timeline {
		require.Failf(t, "battle invariant failed", "report duration does not cover playback: %d timeline=%d", report.DurationMS, timeline)
	}
	alive, enemies := 0, 0
	for uid, unit := range units {
		if unit.Alive {
			if uid[0] == 'a' {
				alive++
			} else {
				enemies++
			}
		}
	}
	win := wave == len(report.Waves)-1 && enemies == 0 && alive > 0 && !report.TimedOut
	stars := 0
	if win {
		stars = 1
		if alive == report.TotalN {
			stars++
		}
		if report.Rounds <= stage.StarRound {
			stars++
		}
	}
	if report.AliveN != alive || report.Win != win || report.Stars != stars {
		require.Failf(t, "battle invariant failed", "final snapshots disagree with result: alive=%d enemies=%d win=%v stars=%d report=%+v", alive, enemies, win, stars, report)
	}
}

func TestGachaBattleShortVictoryHasNoSettlementPadding(t *testing.T) {
	stage := GachaStage{Waves: 1, EnemyCount: 1, EnemyLv: 1, StarRound: 4}
	report, err := simulateGachaCombat(battleTestState([]string{"c04"}, 60, 5), stage, battleTestRNG())
	require.NoError(t, err)
	require.True(t, report.Win)
	assertBattleReportConsistent(t, report, stage)
	assert.Less(t, report.DurationMS, 30_000, "a short victory must be ready when its final animation ends")
}

func TestGachaBattleResultsMatchSnapshots(t *testing.T) {
	teams := []struct {
		name        string
		ids         []string
		level, star int
	}{
		{"weak", []string{"c08"}, 1, 1},
		{"strong", []string{"c04", "c07", "c09", "c03"}, 60, 5},
		{"support", []string{"c02", "c05", "c11", "c12"}, 20, 3},
		{"builds", []string{"c01", "c08", "c06", "c13"}, 60, 5},
	}
	for _, stage := range gachaCatalog.Stages {
		for _, team := range teams {
			t.Run(stage.ID+"/"+team.name, func(t *testing.T) {
				state := battleTestState(team.ids, team.level, team.star)
				if team.name == "builds" {
					talents := []string{"blight", "guardian", "siphon", "oracle"}
					families := []string{"grove", "tide", "umbra", "chrono"}
					state.Sigils = map[string]*GachaSigil{}
					for i, id := range team.ids {
						sigilID := "sigil_0123456789abcdef0123456789abcde" + strconv.Itoa(i+1)
						state.Girls[id].Talent, state.Girls[id].Sigil = talents[i], sigilID
						state.Sigils[sigilID] = &GachaSigil{ID: sigilID, Family: families[i], Quality: 3, Affixes: map[string]int{"atk": 8, "cr": 5}}
					}
				}
				report, err := simulateGachaCombat(state, stage, battleTestRNG())
				require.NoError(t, err)
				assertBattleReportConsistent(t, report, stage)
				if stage.ID == "s6" && team.name == "weak" && (report.Win || report.Stars != 0) {
					require.Fail(t, "level-one solo support unexpectedly wins the hardest stage")
				}
				if stage.RelicTier == 0 && team.name == "strong" && (!report.Win || report.Stars != 3) {
					require.Fail(t, "maximum-level balanced squad did not achieve three stars")
				}
			})
		}
	}
	for _, stage := range []GachaStage{
		{Waves: 1, EnemyCount: 1, EnemyLv: 1, StarRound: 4},
		{Waves: 8, EnemyCount: GachaSquadLimit, EnemyLv: 200, StarRound: 60},
	} {
		report, err := simulateGachaCombat(battleTestState([]string{"c01"}, 1, 1), stage, battleTestRNG())
		if err != nil {
			require.Failf(t, "battle invariant failed", "legal stage boundary was rejected: %v", err)
		}
		assertBattleReportConsistent(t, report, stage)
	}
}

func TestGachaBattleRoundLimit(t *testing.T) {
	// A private, non-parallel catalog fixture makes a surviving stalemate certain.
	original := gachaCatalog
	catalog := *original
	catalog.Girls = slices.Clone(original.Girls)
	for i := range catalog.Girls {
		if catalog.Girls[i].ID == "c01" {
			catalog.Girls[i].Base = GachaStats{HP: 1_000_000_000, Spd: 1}
			catalog.Girls[i].Skills.Normal.Mult = 0
			catalog.Girls[i].Skills.Skill.Mult = 0
		}
	}
	gachaCatalog = &catalog
	defer func() { gachaCatalog = original }()
	stage := GachaStage{Waves: 1, EnemyCount: 1, EnemyLv: 1, StarRound: 60}
	report, err := simulateGachaCombat(battleTestState([]string{"c01"}, 1, 1), stage, battleTestRNG())
	require.NoError(t, err)
	assertBattleReportConsistent(t, report, stage)
	assert.Greater(t, report.DurationMS, 30_000, "long battles must retain their full playback duration")
	if !report.TimedOut || report.Rounds != 60 || report.Win || report.Stars != 0 || report.AliveN != 1 {
		require.Fail(t, "surviving stalemate must stop at round 60 without victory or rewards")
	}
}

func battleTestSkillCombat(girl *GachaGirl) (*gachaCombat, *gachaCombatUnit, *gachaCombatUnit) {
	unit := func(uid, side string, hp, atk int) *gachaCombatUnit {
		return &gachaCombatUnit{GachaBattleUnit: GachaBattleUnit{
			GachaBattleSnapshot: GachaBattleSnapshot{UID: uid, HP: hp, Alive: true},
			Side:                side, MaxHP: 10000, Atk: atk, Def: 40, Level: 1, EHR: 200,
		}}
	}
	caster, ally := unit("a0", "ally", 7000, 100), unit("a1", "ally", 3000, 200)
	caster.girl, caster.GirlID, caster.Rage = girl, girl.ID, gachaCatalog.Formula.RageMax
	enemies := []*gachaCombatUnit{unit("e0", "enemy", 9000, 60), unit("e1", "enemy", 8000, 60)}
	return &gachaCombat{
		allies: []*gachaCombatUnit{caster, ally}, waves: [][]*gachaCombatUnit{enemies},
		rng: battleTestRNG(), formula: gachaCatalog.Formula, report: &GachaBattleReport{},
	}, caster, ally
}

func TestGachaBattleCatalogSkills(t *testing.T) {
	for i := range gachaCatalog.Girls {
		girl := &gachaCatalog.Girls[i]
		t.Run(girl.ID, func(t *testing.T) {
			combat, caster, ally := battleTestSkillCombat(girl)
			if girl.CombatProfile == "joinery" {
				combat, caster = joineryBattle(t)
				caster.Rage = gachaCatalog.Formula.RageMax
				combat.act(caster)
				event := combat.report.Events[0]
				require.True(t, event.Skill)
				require.Equal(t, girl.Skills.JoineryUltimate.Name, event.SkillName)
				require.Equal(t, "xs039-skill-2", event.SkillKey)
				require.Zero(t, caster.Rage)
				require.Len(t, event.Targets, 2)
				for _, hit := range event.Targets {
					require.Positive(t, hit.Damage)
				}
				return
			}
			if girl.ID == "c13" {
				caster.DefBreak, caster.DefBreakT = 30, 3
				ally.StunT, ally.Slow, ally.SlowT = 1, 25, 2
			}
			combat.act(caster)
			if len(combat.report.Events) != 1 {
				require.Fail(t, "skill did not produce exactly one authoritative event")
			}
			event, skill := combat.report.Events[0], girl.Skills.Skill
			if !event.Skill || event.SkillName != skill.Name || caster.Rage != 0 {
				require.Failf(t, "battle invariant failed", "skill was not cast or rage was not spent: %+v", event)
			}
			wantTargets := max(1, skill.Hits)
			if skill.Target == "team" || skill.Target == "allEnemies" {
				wantTargets = 2
			}
			if len(event.Targets) != wantTargets {
				require.Failf(t, "battle invariant failed", "wrong target/hit count: got=%d want=%d", len(event.Targets), wantTargets)
			}
			for _, target := range event.Targets {
				if skill.Mult > 0 && target.Damage <= 0 || skill.HealMult > 0 && target.Heal <= 0 ||
					skill.ShieldMult > 0 && target.Shield <= 0 {
					require.Failf(t, "battle invariant failed", "catalog skill effect was not applied: %+v", target)
				}
			}
			if skill.Taunt && caster.Taunt != 1 || skill.SelfDefUp > 0 && (caster.DefUp != skill.SelfDefUp || caster.DefUpT != 1) {
				require.Fail(t, "new defensive skill expired before the next enemy action")
			}
			if skill.TeamAtkUp > 0 && (caster.AtkUp != skill.TeamAtkUp || ally.AtkUp != skill.TeamAtkUp || ally.AtkUpT != skill.BuffTurns) {
				require.Fail(t, "team attack buff was not applied")
			}
			if combat.report.GoldBonus != skill.GoldBonus || skill.ExtraTurn && !ally.Extra {
				require.Fail(t, "gold bonus or highest-attack ally's extra turn is missing")
			}
			if girl.ID == "c13" && (caster.DefBreakT != 0 || ally.StunT != 0 || ally.SlowT != 2) {
				require.Fail(t, "cleanse must remove one debuff per ally, not all debuffs")
			}
			for _, enemy := range combat.waves[0] {
				if skill.Slow > 0 && (enemy.Slow != skill.Slow || enemy.SlowT != skill.SlowTurns) {
					require.Fail(t, "area slow was not applied to every enemy")
				}
			}
			target := combat.waves[0][1]
			if skill.DefBreak > 0 && (target.DefBreak != skill.DefBreak || target.DefBreakT != skill.DefBreakTurns) ||
				skill.StunChance > 0 && target.StunT != skill.StunTurns {
				require.Fail(t, "single-target debuff was not applied")
			}
			combat, caster, _ = battleTestSkillCombat(girl)
			caster.Rage = 0
			combat.act(caster)
			if event := combat.report.Events[0]; event.Skill || len(event.Targets) != 1 || event.Targets[0].Damage <= 0 || caster.Rage != combat.formula.RagePerNormal {
				require.Failf(t, "battle invariant failed", "normal attack must deal damage and generate rage: %+v", event)
			}
		})
	}
}

func TestGachaBattleDamageAndStatus(t *testing.T) {
	combat, caster, ally := battleTestSkillCombat(gachaGirl("c04"))
	enemy := combat.waves[0][0]
	caster.CR, caster.CD, caster.AtkUp = 100, 150, 20
	enemy.DefUp, enemy.DefBreak, enemy.Shield = 30, 25, 50
	defense := float64(enemy.Def) * 1.3 * 0.75 * 0.8
	want := int(math.Round(120 * 2 * (1 - defense/(defense+float64(combat.formula.DefK+combat.formula.DefLv))) * 2.5))
	target := combat.damage(caster, enemy, 2, 20)
	if !target.Crit || target.Absorbed != 50 || target.Damage != want-50 || enemy.HP != 9000-(want-50) || enemy.Shield != 0 {
		require.Failf(t, "battle invariant failed", "damage/critical/defense/shield formula mismatch: %+v enemy=%+v", target, enemy)
	}
	ally.Taunt = 1
	combat.act(enemy)
	if event := combat.report.Events[0]; len(event.Targets) != 1 || event.Targets[0].UID != ally.UID {
		require.Fail(t, "enemy normal attack ignored the living taunter")
	}
	caster.StunT = 1
	combat.act(caster)
	if event := combat.report.Events[1]; !event.Skipped || len(event.Targets) != 0 || caster.StunT != 0 {
		require.Fail(t, "stun did not consume exactly one action")
	}
	caster.Rage, caster.Slow, caster.SlowT = 0, 25, 1
	caster.DefBreak, caster.DefBreakT, caster.AtkUp, caster.AtkUpT = 30, 1, 20, 1
	caster.DefUp, caster.DefUpT, caster.Taunt = 30, 1, 1
	combat.act(caster)
	if caster.Slow != 0 || caster.SlowT != 0 || caster.DefBreak != 0 || caster.DefBreakT != 0 ||
		caster.AtkUp != 0 || caster.AtkUpT != 0 || caster.DefUp != 0 || caster.DefUpT != 0 || caster.Taunt != 0 {
		require.Fail(t, "one-action statuses did not expire")
	}
}

func TestGachaTalentsAndBondsAffectCombatOnBothSides(t *testing.T) {
	state := battleTestState([]string{"c01"}, 1, 3)
	base, err := gachaSquadUnits(state, "ally", "a")
	require.NoError(t, err)
	for _, talent := range []string{"power", "guardian", "swift", "siphon", "blight", "oracle"} {
		t.Run(talent, func(t *testing.T) {
			own := state.Girls["c01"]
			own.Talent = talent
			allies, err := gachaSquadUnits(state, "ally", "a")
			require.NoError(t, err)
			enemies, err := gachaSquadUnits(state, "enemy", "e")
			require.NoError(t, err)
			assert.Equal(t, allies[0].Atk, enemies[0].Atk)
			assert.Equal(t, allies[0].MaxHP, enemies[0].MaxHP)
			switch talent {
			case "power":
				assert.Greater(t, allies[0].Atk, base[0].Atk)
				assert.Less(t, allies[0].MaxHP, base[0].MaxHP)
				assert.Equal(t, allies[0].Def, base[0].Def)
			case "guardian":
				assert.Greater(t, allies[0].MaxHP, base[0].MaxHP)
				assert.Greater(t, allies[0].Def, base[0].Def)
				assert.Less(t, allies[0].Atk, base[0].Atk)
			case "swift":
				assert.Equal(t, base[0].Spd+4, allies[0].Spd)
				assert.Equal(t, base[0].CR+3, allies[0].CR)
				assert.Less(t, allies[0].Res, base[0].Res)
			case "siphon":
				assert.Greater(t, allies[0].Atk, base[0].Atk)
				assert.Less(t, allies[0].Def, base[0].Def)
			case "blight":
				assert.Less(t, allies[0].Atk, base[0].Atk)
				assert.Greater(t, allies[0].EHR, base[0].EHR)
			case "oracle":
				assert.Greater(t, allies[0].MaxHP, base[0].MaxHP)
				assert.Greater(t, allies[0].Res, base[0].Res)
				assert.Less(t, allies[0].Atk, base[0].Atk)
			}
		})
	}
	one := battleTestState([]string{"c06"}, 1, 1)
	unbonded, err := gachaSquadUnits(one, "ally", "a")
	require.NoError(t, err)
	pair := battleTestState([]string{"c06", "c08"}, 1, 1)
	bonded, err := gachaSquadUnits(pair, "ally", "a")
	require.NoError(t, err)
	assert.Equal(t, 199, bonded[0].Atk, "formation grants six percent attack before the flat collection bonus")
	assert.Equal(t, 188, unbonded[0].Atk)
	pair.BattleBuff = "assault"
	boosted, err := gachaSquadUnits(pair, "ally", "a")
	require.NoError(t, err)
	assert.Greater(t, boosted[0].Atk, bonded[0].Atk)
	assert.Equal(t, bonded[0].MaxHP, boosted[0].MaxHP)
}

func TestGachaCultivationUsesWholeCollectionOnBothSides(t *testing.T) {
	state := battleTestState([]string{"c01"}, 60, 6)
	hero := state.Girls["c01"]
	hero.Awakening, hero.Branch = 1, "focus"
	state.Girls["c11"] = &GachaOwnedGirl{Shards: 250}
	attack := float64(gachaGirl("c01").Base.Atk) * (1 + 59*gachaCatalog.Growth.LevelStatGrow.Atk) * (1 + 5*gachaCatalog.Growth.StarStatGrow.Atk)
	unit := func(side string) *gachaCombatUnit {
		units, err := gachaSquadUnits(state, side, "test")
		require.NoError(t, err)
		return units[0]
	}
	assert.Equal(t, int(math.Round(attack))+3, unit("ally").Atk, "shards alone grant neither codex nor bond")
	state.Girls["c11"] = &GachaOwnedGirl{Level: 1, Star: 2}
	assert.Equal(t, int(math.Round(attack))+6, unit("ally").Atk, "two cards do not complete the crew")
	state.Girls["c04"] = &GachaOwnedGirl{Level: 40, Star: 5}
	assert.Equal(t, int(math.Round(attack*1.04))+9, unit("ally").Atk, "neither remaining member is deployed")
	state.Girls["c11"] = &GachaOwnedGirl{Level: 40, Star: 6, Awakening: 1, Branch: "focus"}
	assert.Equal(t, int(math.Round(attack*1.04))+9, unit("enemy").Atk)
	state.Girls["c04"].Star, state.Girls["c04"].Awakening = 6, 1
	state.Girls["c08"] = &GachaOwnedGirl{Level: 1, Star: 1}
	hero.Awakening, hero.WeaponLevel, hero.WeaponAwake, hero.Potential, hero.WeaponRune = 3, 10, 2, 3, "edge"
	expected := int(math.Round(attack*1.20*1.07*1.03*1.05)) + 9
	for _, side := range []string{"ally", "enemy"} {
		built := unit(side)
		assert.Equal(t, expected, built.Atk)
		assert.Equal(t, 26, built.Rage)
	}
	speed := unit("ally").Spd
	hero.WeaponRune = "tempo"
	assert.Equal(t, speed+4, unit("ally").Spd)
	assert.Less(t, unit("ally").Atk, expected, "re-engraving replaces the attack rune")
	hero.WeaponLevel = 31
	_, err := gachaSquadUnits(state, "enemy", "e")
	assert.ErrorIs(t, err, ErrGachaRunState)
}

func TestGachaCultivationFrozenRostersKeepTheirQuotedPower(t *testing.T) {
	state := battleTestState([]string{"c01"}, 30, 3)
	legacy, err := common.Marshal(state)
	require.NoError(t, err)
	oldDefense, err := thawGachaRoster(GachaDocument(legacy))
	require.NoError(t, err)
	legacyPower, err := GachaSquadPower(oldDefense)
	require.NoError(t, err)
	quotedPower, err := GachaSquadPower(state)
	require.NoError(t, err)
	assert.Less(t, legacyPower, quotedPower)
	saved, err := freezeGachaRoster(state)
	require.NoError(t, err)
	state.Girls["c11"] = &GachaOwnedGirl{Level: 1, Star: 2}
	state.Girls["c01"].WeaponLevel = 5
	newPower, err := GachaSquadPower(state)
	require.NoError(t, err)
	assert.Greater(t, newPower, quotedPower)
	frozen, err := thawGachaRoster(GachaDocument(saved))
	require.NoError(t, err)
	frozenPower, err := GachaSquadPower(frozen)
	require.NoError(t, err)
	assert.Equal(t, quotedPower, frozenPower)
}

func TestGachaV6SnapshotKeepsPairsAndExcludesMastery(t *testing.T) {
	state := battleTestState([]string{"c01"}, 40, 4)
	state.Girls["c11"] = &GachaOwnedGirl{Level: 1, Star: 2}
	state.Girls["c01"].Equipment = map[string]GachaEquipmentPiece{"head": {Level: 6, Star: 2}}
	encoded, err := common.Marshal(gachaFrozenRoster{state, 6})
	require.NoError(t, err)
	old, err := thawGachaRoster(GachaDocument(encoded))
	require.NoError(t, err)
	assert.True(t, old.currentAtelierRules())
	assert.False(t, old.masteryEnabled())
	assert.Equal(t, 4, old.collection().bonds["c01"].Atk)
	assert.Zero(t, state.collection().bonds["c01"].Atk)
	for _, side := range []string{"ally", "enemy"} {
		original, err := gachaSquadUnits(old, side, "v6")
		require.NoError(t, err)
		old.Girls["c01"].Carry = "keepsake_lens"
		unchanged, err := gachaSquadUnits(old, side, "v6")
		require.NoError(t, err)
		assert.Equal(t, original[0].GachaBattleUnit, unchanged[0].GachaBattleUnit)
		old.Girls["c01"].Carry = ""
	}
}

func TestGachaFrozenV6KeepsUnrefinedItemsAndOriginalGearCaps(t *testing.T) {
	state := battleTestState([]string{"c01"}, 60, 12)
	own := state.Girls["c01"]
	own.Awakening, own.Branch = 3, GachaAwakeningBranches("c01")[0].ID
	own.Sockets, own.Gems, own.Chips = 1, []string{"jade"}, []string{"tempo_1"}
	own.WeaponLevel = 10
	own.Equipment = map[string]GachaEquipmentPiece{"weapon": {Star: 5}, "head": {Level: 10, Star: 5}}
	state.previousMastery = true
	baseline, err := gachaSquadUnits(state, "enemy", "v6")
	require.NoError(t, err)
	own.Gems[0], own.Chips[0] = "jade@12", "tempo_1@12"
	own.WeaponLevel = 30
	own.Equipment["weapon"], own.Equipment["head"] = GachaEquipmentPiece{Star: 15}, GachaEquipmentPiece{Level: 30, Star: 15}
	unchanged, err := gachaSquadUnits(state, "enemy", "v6")
	require.NoError(t, err)
	assert.Equal(t, baseline[0].GachaBattleUnit, unchanged[0].GachaBattleUnit)
	state.previousMastery = false
	advanced, err := gachaSquadUnits(state, "enemy", "v6")
	require.NoError(t, err)
	assert.Greater(t, advanced[0].MaxHP, baseline[0].MaxHP)
	assert.Greater(t, advanced[0].Shield, baseline[0].Shield)
	assert.Greater(t, advanced[0].rageBonus, baseline[0].rageBonus)
}

func TestGachaCarryPassivesCountOnlyDistinctDeployedItems(t *testing.T) {
	state := battleTestState([]string{"c01", "c06", "c08"}, 1, 1)
	state.Girls["c01"].Carry, state.Girls["c06"].Carry = "keepsake_lens", "keepsake_lens"
	state.Girls["c08"].Carry = "keepsake_log"
	state.Girls["c12"] = &GachaOwnedGirl{Star: 1, Level: 1, Carry: "keepsake_reed"}
	for _, side := range []string{"ally", "enemy"} {
		members, err := gachaSquadUnits(state, side, "team")
		require.NoError(t, err)
		assert.Equal(t, 14, members[0].Rage)
		assert.Equal(t, 14, members[1].Rage)
		assert.Equal(t, 6, members[2].Rage)
		assert.Equal(t, 3, members[2].rageBonus)
		assert.Zero(t, members[0].skillBoost)
	}
	state.Girls["c06"].Carry = "keepsake_banner"
	state.Squad = append(state.Squad, "c12")
	members, err := gachaSquadUnits(state, "ally", "team")
	require.NoError(t, err)
	assert.InDelta(t, 0.08, members[0].skillBoost, 0.0001)
	assert.InDelta(t, 1.08, members[3].healBoost, 0.0001)
	assert.Zero(t, members[1].skillBoost)
	assert.Equal(t, members[1].MaxHP*4/100, members[1].Shield)
}

func TestGachaAtelierStarsUnlockCombatPassivesAndPreserveV5Defense(t *testing.T) {
	state := battleTestState([]string{"c04"}, 60, 12)
	state.Girls["c04"].Awakening, state.Girls["c04"].Branch = 3, "focus"
	state.Girls["c04"].Equipment = map[string]GachaEquipmentPiece{"body": {Level: 10}, "feet": {Level: 10}}
	for _, id := range []string{"c27", "c01", "c11", "c03", "c10", "c09", "c15"} {
		state.Girls[id] = &GachaOwnedGirl{Star: 6, Level: 40, Awakening: 1, Branch: GachaAwakeningBranches(id)[0].ID}
	}
	assert.Equal(t, 30, state.collection().bonds["c04"].Atk)
	oldData, err := common.Marshal(gachaFrozenRoster{state, 5})
	require.NoError(t, err)
	old, err := thawGachaRoster(GachaDocument(oldData))
	require.NoError(t, err)
	for _, side := range []string{"ally", "enemy"} {
		before, err := gachaSquadUnits(old, side, "old")
		require.NoError(t, err)
		after, err := gachaSquadUnits(state, side, "new")
		require.NoError(t, err)
		assert.Greater(t, after[0].Atk, before[0].Atk)
		assert.Greater(t, after[0].Def, before[0].Def)
		assert.Equal(t, before[0].Spd+10, after[0].Spd)
		assert.Equal(t, 5, after[0].Rage)
		assert.Equal(t, 10, after[0].rageBonus)
		assert.InDelta(t, 0.05, after[0].lifeSteal, 0.0001)
		assert.InDelta(t, 0.08, after[0].skillBoost, 0.0001)
		assert.Zero(t, before[0].Rage)
		assert.Zero(t, before[0].lifeSteal)
	}
	state = battleTestState([]string{"c07"}, 60, 11)
	state.Girls["c07"].Awakening, state.Girls["c07"].Branch = 3, "guard"
	before, err := gachaSquadUnits(state, "ally", "a")
	require.NoError(t, err)
	assert.Equal(t, before[0].MaxHP*4/100, before[0].Shield)
	state.Girls["c07"].Star++
	after, err := gachaSquadUnits(state, "ally", "a")
	require.NoError(t, err)
	assert.Equal(t, after[0].MaxHP*4/100+after[0].MaxHP*12/100, after[0].Shield)
}

func TestGachaStarMilestoneThresholdsAreCumulativeForAllFiveRoles(t *testing.T) {
	roles := []struct {
		role                        string
		rage, rageGain, shield      int
		skill, heal, counter, leech float64
	}{
		{"adps", 6, 6, 0, 0.20, 1, 0, 0},
		{"sdps", 5, 10, 0, 0.08, 1, 0, 0.05},
		{"tank", 15, 0, 160, 0, 1, 0.20, 0},
		{"heal", 5, 12, 80, 0, 1.10, 0, 0},
		{"support", 26, 5, 80, 0, 1, 0, 0},
	}
	for _, expected := range roles {
		t.Run(expected.role, func(t *testing.T) {
			at := func(star int) *gachaCombatUnit {
				unit := &gachaCombatUnit{healBoost: 1}
				unit.MaxHP = 1000
				applyGachaMilestones(unit, expected.role, star)
				return unit
			}
			for _, threshold := range []int{3, 6, 9, 12} {
				assert.NotEqual(t, at(threshold-1), at(threshold))
				if threshold < 12 {
					assert.Equal(t, at(threshold), at(threshold+2))
				}
			}
			full := at(12)
			assert.Equal(t, expected.rage, full.Rage)
			assert.Equal(t, expected.rageGain, full.rageBonus)
			assert.Equal(t, expected.shield, full.Shield)
			assert.InDelta(t, expected.skill, full.skillBoost, 0.0001)
			assert.InDelta(t, expected.heal, full.healBoost, 0.0001)
			assert.InDelta(t, expected.counter, full.counter, 0.0001)
			assert.InDelta(t, expected.leech, full.lifeSteal, 0.0001)
		})
	}
}

func TestGachaArtifactSetsApplyPerWearerOnBothSides(t *testing.T) {
	state := battleTestState([]string{"c01"}, 60, 12)
	hero := state.Girls["c01"]
	hero.Awakening, hero.Branch, hero.Sockets = 3, "focus", 9
	hero.Gems = []string{"ruby", "ruby", "ruby", "ruby", "ruby", "ruby", "ruby", "ruby", "ruby"}
	hero.Chips = []string{"edge_1", "edge_2", "edge_3", "edge_4", "edge_5", "edge_6"}
	state.GemInventory = map[string]int{"ruby": 90}
	state.ChipInventory = map[string]int{"edge_6": 60}
	base := gachaGirl("c01").Base
	for _, side := range []string{"ally", "enemy"} {
		units, err := gachaSquadUnits(state, side, "test")
		require.NoError(t, err)
		assert.Equal(t, base.CD+24, units[0].CD)
		assert.Equal(t, base.CR+6, units[0].CR)
	}
	full, err := GachaSquadPower(state)
	require.NoError(t, err)
	hero.ChipLocks = 63
	locked, err := GachaSquadPower(state)
	require.NoError(t, err)
	assert.Equal(t, full, locked)
	hero.ChipLocks, hero.Gems[8], hero.Chips[5] = 0, "", ""
	units, err := gachaSquadUnits(state, "ally", "a")
	require.NoError(t, err)
	assert.Equal(t, base.CD, units[0].CD, "both top-tier bonuses must disappear when the pieces return to the bag")
	partial, err := GachaSquadPower(state)
	require.NoError(t, err)
	assert.Less(t, partial, full)
}

func TestGachaSigilCritAffixesChangeMaxLevelPowerAndBothCombatSides(t *testing.T) {
	base := battleTestState([]string{"c01"}, 60, 5)
	state := battleTestState([]string{"c01"}, 60, 5)
	sigil := &GachaSigil{ID: "sigil_0123456789abcdef0123456789abcdef", Family: "astral", Quality: 3, Affixes: map[string]int{"cr": 8, "cd": 25}}
	state.Sigils = map[string]*GachaSigil{sigil.ID: sigil}
	state.Girls["c01"].Sigil = sigil.ID
	baseline, err := GachaSquadPower(base)
	require.NoError(t, err)
	power, err := GachaSquadPower(state)
	require.NoError(t, err)
	assert.Greater(t, power, baseline, "same max level need not imply the same power")
	allies, err := gachaSquadUnits(state, "ally", "a")
	require.NoError(t, err)
	enemies, err := gachaSquadUnits(state, "enemy", "e")
	require.NoError(t, err)
	assert.Equal(t, gachaGirl("c01").Base.CR+8, allies[0].CR)
	assert.Equal(t, gachaGirl("c01").Base.CD+45, allies[0].CD)
	assert.Equal(t, allies[0].CR, enemies[0].CR)
	assert.Equal(t, allies[0].CD, enemies[0].CD)
	assert.Equal(t, 60, state.Girls["c01"].Level)
	assert.Equal(t, 5, state.Girls["c01"].Star)
	assert.Equal(t, map[string]int{"cr": 8, "cd": 25}, state.Sigils[sigil.ID].Affixes)
}

func TestGachaLeechAndCounterUseRealHealthDamageAndDoNotRecurse(t *testing.T) {
	attack := battleTestState([]string{"c06"}, 60, 5)
	attack.Girls["c06"].Talent = "siphon"
	sigil := &GachaSigil{ID: "sigil_1123456789abcdef0123456789abcdef", Family: "umbra", Quality: 3, Affixes: map[string]int{"atk": 8, "hp": 12}}
	attack.Sigils = map[string]*GachaSigil{sigil.ID: sigil}
	attack.Girls["c06"].Sigil = sigil.ID
	defend := battleTestState([]string{"c08"}, 60, 5)
	defend.Girls["c08"].Talent = "guardian"
	allies, err := gachaSquadUnits(attack, "ally", "a")
	require.NoError(t, err)
	enemies, err := gachaSquadUnits(defend, "enemy", "e")
	require.NoError(t, err)
	b := &gachaCombat{allies: allies, waves: [][]*gachaCombatUnit{enemies}, formula: gachaCatalog.Formula, rng: battleTestRNG(), report: &GachaBattleReport{}, pvp: true}
	src, dst := allies[0], enemies[0]
	src.CR, src.HP = 0, src.MaxHP/2
	dst.Shield = 100_000
	before := src.HP
	blocked := b.damage(src, dst, 1, 0)
	assert.Zero(t, blocked.Damage)
	assert.Positive(t, blocked.Absorbed)
	assert.Zero(t, blocked.SelfHeal, "shields are not a lifesteal source")
	assert.Equal(t, before, src.HP)
	dst.Shield = 0
	impact := b.damage(src, dst, 1, 0)
	assert.Positive(t, impact.Damage)
	assert.Equal(t, int(math.Round(float64(impact.Damage)*0.23)), impact.SelfHeal)
	assert.Positive(t, impact.Reflected)
	assert.Equal(t, before+impact.SelfHeal-impact.Reflected, src.HP)
	assert.Equal(t, dst.MaxHP-impact.Damage, dst.HP)
	assert.True(t, src.Alive)
}

func TestGachaBlightBypassesShieldsCapsDamageAndSuppressesRegrowth(t *testing.T) {
	state := battleTestState([]string{"c01"}, 60, 5)
	state.Girls["c01"].Talent = "blight"
	units, err := gachaSquadUnits(state, "ally", "a")
	require.NoError(t, err)
	b, _, _ := battleTestSkillCombat(gachaGirl("c01"))
	src, dst := units[0], b.waves[0][0]
	b.allies[0] = src
	src.EHR = 100
	dst.Res, dst.Shield = 0, 100_000
	before := dst.HP
	impact := b.damage(src, dst, 1, 0)
	assert.Zero(t, impact.Damage)
	assert.Equal(t, 2, dst.DotT)
	assert.Equal(t, 300, dst.Dot)
	b.act(dst)
	assert.Equal(t, before-300, dst.HP, "invasion ticks are health damage even behind a shield")
	assert.Equal(t, "ailment", b.report.Events[0].Kind)
	b.mechanic = "regrowth"
	dst.HP, dst.Dot, dst.DotT = 5000, 300, 2
	b.act(dst)
	assert.Equal(t, 4900, dst.HP, "300 damage plus 200 suppressed regeneration")
	dst.Dot, dst.DotT = 0, 0
	b.act(dst)
	assert.Equal(t, 5700, dst.HP, "without invasion the full 800 regeneration returns")
	src.Atk, dst.MaxHP, dst.HP, dst.Dot, dst.DotT = 20, 100_000, 100_000, 0, 0
	b.damage(src, dst, 1, 0)
	assert.Equal(t, 36, dst.Dot, "large bosses cannot bypass the caster attack-based damage cap")
}

func TestGachaAreaSkillStopsWhenReflectionDefeatsCaster(t *testing.T) {
	b, caster, _ := battleTestSkillCombat(gachaGirl("c01"))
	b.mechanic = "reflection"
	caster.HP, caster.CR = 1, 0
	caster.Rage = b.formula.RageMax
	require.GreaterOrEqual(t, len(b.waves[0]), 2)
	untouched := b.waves[0][1].HP
	b.act(caster)
	require.Len(t, b.report.Events, 1)
	assert.True(t, b.report.Events[0].Skill)
	assert.Len(t, b.report.Events[0].Targets, 1)
	assert.False(t, caster.Alive)
	assert.Zero(t, caster.HP)
	assert.Equal(t, untouched, b.waves[0][1].HP, "a defeated caster cannot damage the remaining targets")
}

func TestGachaSwiftChronoComboProducesFasterSkillCycles(t *testing.T) {
	state := battleTestState([]string{"c01"}, 60, 5)
	state.Girls["c01"].Talent = "swift"
	sigil := &GachaSigil{ID: "sigil_2123456789abcdef0123456789abcdef", Family: "chrono", Quality: 3, Affixes: map[string]int{"hp": 12, "def": 14}}
	state.Sigils = map[string]*GachaSigil{sigil.ID: sigil}
	state.Girls["c01"].Sigil = sigil.ID
	units, err := gachaSquadUnits(state, "ally", "a")
	require.NoError(t, err)
	b, _, _ := battleTestSkillCombat(gachaGirl("c01"))
	b.allies[0] = units[0]
	for _, enemy := range b.waves[0] {
		enemy.HP, enemy.MaxHP = 1_000_000, 1_000_000
	}
	for range 4 {
		b.act(units[0])
	}
	flags := []bool{}
	for _, event := range b.report.Events {
		flags = append(flags, event.Skill)
	}
	assert.Equal(t, []bool{false, true, false, true}, flags)
}

func TestGachaOracleStrengthensHealingAndCleansesOnlyOneAilment(t *testing.T) {
	state := battleTestState([]string{"c13"}, 60, 5)
	state.Girls["c13"].Talent = "oracle"
	sigil := &GachaSigil{ID: "sigil_3123456789abcdef0123456789abcdef", Family: "tide", Quality: 3, Affixes: map[string]int{"hp": 12, "def": 14}}
	state.Sigils = map[string]*GachaSigil{sigil.ID: sigil}
	state.Girls["c13"].Sigil = sigil.ID
	units, err := gachaSquadUnits(state, "ally", "a")
	require.NoError(t, err)
	b, _, ally := battleTestSkillCombat(gachaGirl("c13"))
	b.allies[0] = units[0]
	units[0].Rage = gachaCatalog.Formula.RageMax
	ally.HP, ally.MaxHP, ally.Dot, ally.DotT, ally.StunT = 1, 100_000, 100, 2, 1
	b.act(units[0])
	event := b.report.Events[0]
	require.True(t, event.Skill)
	require.Len(t, event.Targets, 2)
	expected := int(math.Round(units[0].effectiveAttack()*gachaGirl("c13").Skills.Skill.HealMult*1.6)) / 2
	assert.Equal(t, expected, event.Targets[1].Heal)
	assert.Zero(t, ally.DotT)
	assert.Zero(t, ally.Dot)
	assert.Equal(t, 1, ally.StunT, "the single-cleanse skill must not remove every status at once")
}

func TestGachaAwakeningDamageRoutesTradeSingleTargetForAreaDamage(t *testing.T) {
	original := *gachaGirl("c04")
	damages := map[string]int{}
	for _, branch := range []string{"focus", "sweep"} {
		state := battleTestState([]string{"c04"}, 60, 5)
		state.Girls["c04"].Awakening, state.Girls["c04"].Branch = 3, branch
		units, err := gachaSquadUnits(state, "ally", "a")
		require.NoError(t, err)
		b, _, _ := battleTestSkillCombat(gachaGirl("c04"))
		b.allies[0] = units[0]
		units[0].Rage, units[0].CR, units[0].EHR = 100, 0, 100
		for _, enemy := range b.waves[0] {
			enemy.MaxHP, enemy.HP, enemy.Def, enemy.Res = 100_000, 100_000, 3000, 0
		}
		b.act(units[0])
		require.Len(t, b.report.Events, 1)
		event := b.report.Events[0]
		assert.True(t, event.Skill)
		want := 1
		if branch == "sweep" {
			want = 2
		}
		assert.Len(t, event.Targets, want)
		damages[branch] = event.Targets[0].Damage
		for _, enemy := range b.waves[0] {
			if branch == "sweep" {
				assert.Equal(t, 2, enemy.DotT)
				assert.Positive(t, enemy.Dot)
				assert.LessOrEqual(t, enemy.Dot, int(math.Round(float64(units[0].Atk)*0.8)))
			} else {
				assert.Zero(t, enemy.DotT)
			}
		}
	}
	assert.Greater(t, damages["focus"], damages["sweep"])
	assert.Equal(t, original, *gachaGirl("c04"), "a skill choice must not mutate another player's catalogue")
}

func TestGachaAwakeningShieldRoutesTradeProtectionForAbsorptionReflection(t *testing.T) {
	shields := map[string]int{}
	for _, branch := range []string{"guard", "mirror"} {
		state := battleTestState([]string{"c07"}, 60, 5)
		state.Girls["c07"].Awakening, state.Girls["c07"].Branch = 3, branch
		units, err := gachaSquadUnits(state, "ally", "a")
		require.NoError(t, err)
		b, _, ally := battleTestSkillCombat(gachaGirl("c07"))
		b.allies[0], units[0].Rage = units[0], 100
		b.act(units[0])
		shields[branch] = ally.Shield
		before := b.waves[0][0].HP
		impact := b.damage(b.waves[0][0], ally, 1, 0)
		assert.Zero(t, impact.Damage)
		assert.Positive(t, impact.Absorbed)
		assert.Zero(t, impact.SelfHeal)
		if branch == "mirror" {
			assert.Equal(t, impact.Absorbed/2, impact.Reflected)
			assert.Equal(t, before-impact.Reflected, b.waves[0][0].HP)
		} else {
			assert.Zero(t, impact.Reflected)
		}
	}
	assert.Greater(t, shields["guard"], shields["mirror"])
}

func TestGachaAwakeningHealerAndSupportRoutesChangeCleanseTargetsAndRage(t *testing.T) {
	for _, pair := range []struct{ id, branch string }{{"c28", "purify"}, {"c28", "renew"}, {"c09", "rally"}, {"c09", "suppress"}} {
		t.Run(pair.branch, func(t *testing.T) {
			state := battleTestState([]string{pair.id}, 60, 5)
			state.Girls[pair.id].Awakening, state.Girls[pair.id].Branch = 3, pair.branch
			units, err := gachaSquadUnits(state, "ally", "a")
			require.NoError(t, err)
			b, _, ally := battleTestSkillCombat(gachaGirl(pair.id))
			b.allies[0] = units[0]
			units[0].Rage, units[0].EHR = 100, 100
			ally.HP, ally.Dot, ally.DotT, ally.StunT = 1000, 100, 2, 1
			for _, enemy := range b.waves[0] {
				enemy.MaxHP, enemy.HP, enemy.Rage = 100_000, 100_000, 80
			}
			b.act(units[0])
			require.Len(t, b.report.Events, 1)
			event := b.report.Events[0]
			switch pair.branch {
			case "purify":
				assert.Len(t, event.Targets, 2)
				assert.Zero(t, ally.DotT)
				assert.Zero(t, ally.StunT)
				assert.Positive(t, ally.Shield)
			case "renew":
				require.Len(t, event.Targets, 1)
				assert.Equal(t, ally.UID, event.Targets[0].UID)
				assert.Equal(t, 2, ally.DotT)
				assert.Equal(t, 1, ally.StunT)
				assert.Equal(t, 30, ally.Rage)
				assert.Positive(t, ally.Shield)
			case "rally":
				assert.Len(t, event.Targets, 2)
				assert.Positive(t, ally.AtkUp)
				assert.Equal(t, 20, ally.Rage)
			case "suppress":
				require.Len(t, event.Targets, 1)
				target := b.waves[0][0]
				assert.Equal(t, -25, event.Targets[0].RageDelta)
				assert.Equal(t, 55, target.Rage)
				assert.Positive(t, target.DefBreak)
				assert.Positive(t, target.StunT)
				assert.Zero(t, ally.Rage)
			}
		})
	}
}

func TestGachaAwakeningArenaUsesFrozenRoutesOnBothSidesAndDoesNotRewriteSaves(t *testing.T) {
	state := battleTestState([]string{"c04", "c07", "c09", "c28"}, 60, 5)
	for _, id := range state.Squad {
		state.Girls[id].Awakening, state.Girls[id].Branch = 3, GachaAwakeningBranches(id)[1].ID
	}
	before, err := common.Marshal(state)
	require.NoError(t, err)
	basePower, err := GachaSquadPower(battleTestState(state.Squad, 60, 5))
	require.NoError(t, err)
	advancedPower, err := GachaSquadPower(state)
	require.NoError(t, err)
	assert.Greater(t, advancedPower, basePower)
	report, err := simulateGachaArena(state, state, battleTestRNG())
	require.NoError(t, err)
	for i, ally := range report.Allies {
		assert.Equal(t, 3, ally.Awakening)
		assert.Equal(t, ally.Branch, report.Waves[0][i].Branch)
	}
	after, err := common.Marshal(state)
	require.NoError(t, err)
	assert.Equal(t, before, after)
	assertBattleReportConsistent(t, report, GachaStage{Waves: 1, EnemyCount: 4, EnemyLv: 60, StarRound: 60})
}

func TestGachaProgressionCalendarBudgetCannotBeBoughtWithExistingStock(t *testing.T) {
	for weekday := range 7 {
		t.Run(strconv.Itoa(weekday), func(t *testing.T) {
			start := time.Date(2026, 11, 23+weekday, 12, 0, 0, 0, gameZone)
			state := newGachaState(start)
			rules := DefaultGachaProgressionConfig()
			state.Gold, state.Token, state.Mats["shard_dust"] = gachaMaxResource, gachaMaxResource, gachaMaxResource
			firstFour, firstEight := 0, 0
			for offset := range 56 {
				now := start.AddDate(0, 0, offset)
				state.ProgressionRules = rules
				require.NoError(t, normalizeGachaProgression(state, now, rules))
				for _, activity := range []string{"training", "training", "training", "wish", "wish"} {
					recordGachaProgressionActivity(state, activity, now)
				}
				day := gachaDay(now)
				require.NoError(t, applyGachaProgressionAction(nil, 1, state, GachaActionIntent{Action: "contract_claim", ItemID: &day}, &GachaActionResult{}, now))
				for _, week := range state.Progression.Weeks {
					if !week.Claimed && week.Completed == 5 && len(week.Activities) >= 2 {
						girl, key := "c04", week.Week
						require.NoError(t, applyGachaProgressionAction(nil, 1, state, GachaActionIntent{Action: "weekly_claim", ItemID: &key, GirlID: &girl}, &GachaActionResult{}, now))
					}
				}
				if offset < 2 {
					assert.LessOrEqual(t, state.Progression.Marks, 320)
				}
				if firstFour == 0 && state.Progression.Marks >= 1600 {
					firstFour = offset + 1
				}
				if firstEight == 0 && state.Progression.Marks >= 3200 {
					firstEight = offset + 1
				}
			}
			assert.GreaterOrEqual(t, firstFour, 25)
			assert.LessOrEqual(t, firstFour, 28)
			assert.GreaterOrEqual(t, firstEight, 53)
			assert.LessOrEqual(t, firstEight, 56)
			assert.Equal(t, gachaMaxResource, state.Gold)
			assert.Equal(t, gachaMaxResource, state.Token)
		})
	}
}

func TestGachaProgressionIntroUsesEarnedCaptainHeadroomWithoutGoldLevelling(t *testing.T) {
	now := time.Date(2026, 11, 23, 12, 0, 0, 0, gameZone)
	state := newGachaState(now)
	state.Captain.Level = 20
	require.NoError(t, syncGachaCaptain(state))
	rules := DefaultGachaProgressionConfig()
	state.ProgressionRules = rules
	require.NoError(t, normalizeGachaProgression(state, now, rules))
	for range 3 {
		stage, snapshot, err := specialGachaProgressionStage("training", state, now)
		require.NoError(t, err)
		snapshot.RulesVersion = gachaStaminaXPVersion
		report, err := simulateGachaCombat(state, *stage, battleTestRNG())
		require.NoError(t, err)
		require.True(t, report.Win)
		result := &GachaFinishResult{Win: true}
		settleGachaCaptainRun(state, stage, &snapshot, result)
		require.NoError(t, settleGachaProgressionMode(state, &snapshot, result, now))
	}
	assert.Equal(t, GachaCaptain{Level: 20}, *state.Captain)
	stage := gachaStage("s1")
	report, err := simulateGachaCombat(state, *stage, battleTestRNG())
	require.NoError(t, err)
	require.True(t, report.Win)
	snapshot := GachaRunSnapshot{Squad: slices.Clone(state.Squad), RulesVersion: gachaStaminaXPVersion}
	require.NoError(t, spendGachaStamina(state, stage.Cost, now, &snapshot.StaminaDrops))
	settleGachaCaptainRun(state, stage, &snapshot, &GachaFinishResult{Win: true})
	require.NoError(t, awardGachaGameRewards(state, stage, &snapshot, report, &GachaFinishResult{}))
	key := "s1"
	require.NoError(t, applyGachaProgressionAction(nil, 1, state, GachaActionIntent{Action: "mainline_claim", ItemID: &key}, &GachaActionResult{}, now))
	assert.Equal(t, GachaCaptain{Level: 20, XP: stage.Cost}, *state.Captain)
	for _, id := range state.Squad {
		assert.GreaterOrEqual(t, state.Girls[id].Level, 20)
		assert.LessOrEqual(t, state.Girls[id].Level, 25)
		assert.LessOrEqual(t, state.Girls[id].Level, state.Captain.Level)
	}
	for _, id := range []string{"s5", "s2"} {
		stage := gachaStage(id)
		report, err := simulateGachaCombat(state, *stage, battleTestRNG())
		require.NoError(t, err)
		require.True(t, report.Win, "introductory chapter must be possible with the guaranteed starter squad")
		require.NoError(t, awardGachaGameRewards(state, stage, &snapshot, report, &GachaFinishResult{}))
	}
	core := "c04"
	require.NoError(t, applyGachaProgressionAction(nil, 1, state, GachaActionIntent{Action: "starter_select", GirlID: &core}, &GachaActionResult{}, now))
	assert.Equal(t, 3, state.Girls[core].Star)
	assert.Equal(t, 20, state.Girls[core].Level)
	assert.Zero(t, state.Progression.Marks)
}

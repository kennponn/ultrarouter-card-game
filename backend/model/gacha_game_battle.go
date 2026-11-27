package model

import (
	cryptorand "crypto/rand"
	"encoding/binary"
	"errors"
	"math"
	"math/rand/v2"
	"slices"
	"strconv"
)

// Squad order is authoritative: slots 0..2 are the front row and 3..5 the back row.
const GachaSquadLimit = 6
const gachaFormationColumns = 3
const gachaBattleReportVersion = 2

type GachaBattleSnapshot struct {
	UID        string `json:"uid"`
	HP         int    `json:"hp"`
	Rage       int    `json:"rage"`
	Shield     int    `json:"shield"`
	Alive      bool   `json:"alive"`
	Slow       int    `json:"slow"`
	SlowT      int    `json:"slowT"`
	StunT      int    `json:"stunT"`
	Taunt      int    `json:"taunt"`
	DefBreak   int    `json:"defBreak"`
	DefBreakT  int    `json:"defBreakT"`
	AtkUp      int    `json:"atkUp"`
	AtkUpT     int    `json:"atkUpT"`
	DefUp      int    `json:"defUp"`
	DefUpT     int    `json:"defUpT"`
	Extra      bool   `json:"extra"`
	Dot        int    `json:"dot"`
	DotT       int    `json:"dotT"`
	ReflectPct int    `json:"reflectPct,omitempty"`
	ReflectT   int    `json:"reflectT,omitempty"`
}

type GachaBattleUnit struct {
	GachaBattleSnapshot
	Slot      int    `json:"slot"`
	Side      string `json:"side"`
	GirlID    string `json:"girlId"`
	Elem      string `json:"elem,omitempty"`
	Name      string `json:"name"`
	Img       string `json:"img"`
	AtkType   string `json:"atkType"`
	Boss      bool   `json:"boss"`
	Level     int    `json:"level"`
	MaxHP     int    `json:"maxHp"`
	Atk       int    `json:"atk"`
	Def       int    `json:"def"`
	Spd       int    `json:"spd"`
	CR        int    `json:"cr"`
	CD        int    `json:"cd"`
	EHR       int    `json:"ehr"`
	Res       int    `json:"res"`
	Awakening int    `json:"awakening,omitempty"`
	Branch    string `json:"branch,omitempty"`
}

type GachaBattleTarget struct {
	UID       string `json:"uid"`
	Damage    int    `json:"damage"`
	Heal      int    `json:"heal"`
	Shield    int    `json:"shield"`
	Absorbed  int    `json:"absorbed"`
	Crit      bool   `json:"crit"`
	Elemental string `json:"elemental,omitempty"`
	SelfHeal  int    `json:"self_heal,omitempty"`
	Reflected int    `json:"reflected,omitempty"`
	RageDelta int    `json:"rage_delta,omitempty"`
}

type GachaBattleEvent struct {
	Seq        int                   `json:"seq"`
	Order      []string              `json:"order,omitempty"`
	Kind       string                `json:"kind"`
	Wave       int                   `json:"wave"`
	Round      int                   `json:"round"`
	Src        string                `json:"src"`
	Skill      bool                  `json:"skill"`
	SkillName  string                `json:"skill_name"`
	Skipped    bool                  `json:"skipped"`
	Targets    []GachaBattleTarget   `json:"targets"`
	Units      []GachaBattleSnapshot `json:"units"`
	DurationMS int                   `json:"duration_ms"`
}

type GachaBattleReport struct {
	Version       int                 `json:"version"`
	Allies        []GachaBattleUnit   `json:"allies"`
	Waves         [][]GachaBattleUnit `json:"waves"`
	Events        []GachaBattleEvent  `json:"events"`
	Win           bool                `json:"win"`
	Stars         int                 `json:"stars"`
	Rounds        int                 `json:"rounds"`
	AliveN        int                 `json:"alive_n"`
	TotalN        int                 `json:"total_n"`
	DurationMS    int                 `json:"duration_ms"`
	MaterialBonus float64             `json:"material_bonus"`
	GoldBonus     float64             `json:"gold_bonus"`
	TimedOut      bool                `json:"timed_out"`
}

type gachaCombatUnit struct {
	vigilAspect string
	vigilValue  int
	GachaBattleUnit
	girl        *GachaGirl
	skillBoost  float64
	healBoost   float64
	lifeSteal   float64
	counter     float64
	dotChance   float64
	dotPct      float64
	dotCap      float64
	rageBonus   int
	castRefund  int
	activeSkill *GachaSkill
}

type gachaCombat struct {
	allies     []*gachaCombatUnit
	waves      [][]*gachaCombatUnit
	wave       int
	rounds     int
	roundLimit int
	rng        *rand.Rand
	formula    GachaFormula
	report     *GachaBattleReport
	pvp        bool
	mechanic   string
}

// The production entry point never accepts a client seed or combat result.
func SimulateGachaCombat(state *GachaGameState, stage GachaStage) (*GachaBattleReport, error) {
	var seed [16]byte
	if _, err := cryptorand.Read(seed[:]); err != nil {
		return nil, err
	}
	rng := rand.New(rand.NewPCG(binary.LittleEndian.Uint64(seed[:8]), binary.LittleEndian.Uint64(seed[8:])))
	return simulateGachaCombat(state, stage, rng)
}

func simulateGachaCombat(state *GachaGameState, stage GachaStage, rng *rand.Rand) (*GachaBattleReport, error) {
	if gachaCatalog == nil || state == nil || rng == nil {
		return nil, errors.New("gacha combat is not initialized")
	}
	if len(state.Squad) == 0 || len(state.Squad) > GachaSquadLimit {
		return nil, errors.New("invalid gacha squad")
	}
	if stage.Waves < 1 || stage.Waves > 8 || stage.EnemyCount < 1 || stage.EnemyCount > GachaSquadLimit || stage.EnemyLv < 1 || stage.EnemyLv > 200 || len(gachaCatalog.Enemies) < 4 {
		return nil, errors.New("invalid gacha stage")
	}
	formula := gachaCatalog.Formula
	if formula.DefK <= 0 || formula.DefLv < 0 || formula.RageMax <= 0 || formula.RagePerNormal <= 0 {
		return nil, errors.New("invalid gacha combat formula")
	}
	report := &GachaBattleReport{Allies: []GachaBattleUnit{}, Waves: [][]GachaBattleUnit{}, Events: []GachaBattleEvent{}}
	b := &gachaCombat{rng: rng, formula: formula, report: report, mechanic: stage.Mechanic}
	var err error
	b.allies, err = gachaSquadUnits(state, "ally", "a")
	if err != nil {
		return nil, err
	}
	for _, u := range b.allies {
		report.Allies = append(report.Allies, u.GachaBattleUnit)
	}
	for wave := range stage.Waves {
		bossWave := stage.HasBoss && wave == stage.Waves-1
		count := stage.EnemyCount
		if bossWave {
			count = 1
		}
		units := make([]*gachaCombatUnit, 0, count)
		initial := make([]GachaBattleUnit, 0, count)
		for i := range count {
			templateIndex := (wave*stage.EnemyCount + i) % 3
			if bossWave {
				templateIndex = len(gachaCatalog.Enemies) - 1
			}
			template := gachaCatalog.Enemies[templateIndex]
			level := float64(stage.EnemyLv)
			u := &gachaCombatUnit{GachaBattleUnit: GachaBattleUnit{
				GachaBattleSnapshot: GachaBattleSnapshot{UID: "e" + strconv.Itoa(i), Alive: true},
				Slot:                i,
				Side:                "enemy", Name: template.Name, Img: template.Img, AtkType: template.AtkType,
				Elem: template.Elem,
				Boss: bossWave, Level: stage.EnemyLv,
				MaxHP: int(math.Round(260 * (1 + level*0.10) * template.HPM * max(1, stage.BossHPM) * max(1, stage.HPScale))),
				Atk:   int(math.Round(55 * (1 + level*0.08) * template.AtkM * max(1, stage.BossAtkM) * max(1, stage.AtkScale))),
				Def:   int(math.Round(15 * (1 + level*0.06) * template.DefM * max(1, stage.DefScale))),
				Spd:   template.Spd, CR: 5, CD: 130,
			}}
			u.Res = stage.EnemyRes
			if stage.EnemyElem != "" {
				u.Elem = stage.EnemyElem
			}
			if stage.Mechanic == "barrier" {
				u.Shield = int(math.Round(float64(u.MaxHP) * 0.35))
			}
			if bossWave && stage.BossElem != "" {
				u.Elem = stage.BossElem
			}
			if bossWave && stage.BossName != "" {
				u.Name = stage.BossName
			}
			u.HP = u.MaxHP
			units = append(units, u)
			initial = append(initial, u.GachaBattleUnit)
		}
		b.waves = append(b.waves, units)
		report.Waves = append(report.Waves, initial)
	}

	return b.fight(stage.StarRound), nil
}

func gachaSquadUnits(state *GachaGameState, side, prefix string) ([]*gachaCombatUnit, error) {
	if state == nil || len(state.Squad) == 0 || len(state.Squad) > GachaSquadLimit {
		return nil, ErrGachaTeam
	}
	units := make([]*gachaCombatUnit, 0, len(state.Squad))
	growth := gachaCatalog.Growth
	var collection gachaCollection
	if !state.legacyArenaStats {
		collection = state.collection()
	}
	elemCounts, roleCounts := map[string]int{}, map[string]int{}
	for _, id := range state.Squad {
		if girl := gachaGirl(id); girl != nil {
			elemCounts[girl.Elem]++
			roleCounts[girl.Role]++
		}
	}
	seen := make(map[string]bool, len(state.Squad))
	for i, id := range state.Squad {
		own := state.Girls[id]
		if seen[id] || !gachaCardOwned(own) || own.Star > growth.StarMax {
			return nil, errors.New("invalid gacha squad ownership")
		}
		if !validGachaCultivationStateFor(own, state.advancedMasteryEnabled()) {
			return nil, ErrGachaRunState
		}
		seen[id] = true
		var girl *GachaGirl
		for j := range gachaCatalog.Girls {
			if gachaCatalog.Girls[j].ID == id {
				girl = &gachaCatalog.Girls[j]
				break
			}
		}
		if girl == nil {
			return nil, errors.New("unknown gacha girl")
		}
		if !validGachaArtifactCard(own) {
			return nil, ErrGachaRunState
		}
		gemCounts := make(map[string]int, 3)
		for _, gemID := range own.Gems {
			if gemID == "" {
				continue
			}
			if _, ok := gachaGem(gemID); !ok {
				return nil, errors.New("unknown gacha gem")
			}
			baseID, _, _ := splitGachaRefinement(gemID)
			gemCounts[baseID]++
		}
		base := girl.Base
		levelGrowth, starGrowth := growth.LevelStatGrow, growth.StarStatGrow
		level, star := float64(max(0, own.Level-1)), float64(own.Star-1)
		hp := float64(base.HP) * (1 + levelGrowth.HP*level) * (1 + starGrowth.HP*star) * (1 + 0.05*float64(gemCounts["jade"]))
		atk := float64(base.Atk) * (1 + levelGrowth.Atk*level) * (1 + starGrowth.Atk*star) * (1 + 0.05*float64(gemCounts["ruby"]))
		def := float64(base.Def) * (1 + levelGrowth.Def*level) * (1 + starGrowth.Def*star) * (1 + 0.05*float64(gemCounts["sapphire"]))
		spd := float64(base.Spd)*(1+levelGrowth.Spd*level)*(1+starGrowth.Spd*star) + float64(3*gemCounts["wind"])
		cr := base.CR + 3*gemCounts["amethyst"]
		cd, ehr, res := base.CD, base.EHR, base.Res
		if !state.legacyArenaStats {
			bonus := gachaArtifactBonuses(own, gemCounts)
			if state.currentAtelierRules() {
				addGachaEquipmentBonuses(&bonus, id, own, state.masteryEnabled())
			}
			if state.masteryEnabled() && own.Carry != "" {
				item, ok := gachaCarry.Items[own.Carry]
				if !ok {
					return nil, ErrGachaRunState
				}
				bonus.addCultivationStat(item.Stat, item.Value)
			}
			hp *= 1 + float64(bonus.HP)/100
			atk *= 1 + float64(bonus.Atk)/100
			def *= 1 + float64(bonus.Def)/100
			spd += float64(bonus.Spd)
			cr += bonus.CR
			cd += bonus.CD
		}
		if state.masteryEnabled() && own.Training != nil {
			percent, flat := gachaTrainingBonuses(own.Training, state.advancedMasteryEnabled())
			factor := 1 + float64(percent)/100
			hp, atk, def = hp*factor+float64(flat.HP), atk*factor+float64(flat.Atk), def*factor+float64(flat.Def)
			spd += float64(flat.Spd)
			res += flat.Res
		}
		var sigil *GachaSigil
		if own.Sigil != "" {
			sigil = state.Sigils[own.Sigil]
			if !validGachaSigil(sigil) {
				return nil, ErrGachaSigil
			}
			hp *= 1 + float64(sigil.Affixes["hp"])/100
			atk *= 1 + float64(sigil.Affixes["atk"])/100
			def *= 1 + float64(sigil.Affixes["def"])/100
			spd += float64(sigil.Affixes["spd"])
			cr += sigil.Affixes["cr"]
			cd += sigil.Affixes["cd"]
			ehr += sigil.Affixes["ehr"]
			res += sigil.Affixes["res"]
			if sigil.Family == "astral" {
				cd += 20
			}
		}
		talentRank := min(3, max(0, own.Star-2))
		switch own.Talent {
		case "power":
			atk *= 1 + 0.08*float64(talentRank)
			hp *= 1 - 0.04*float64(talentRank)
		case "guardian":
			hp *= 1 + 0.05*float64(talentRank)
			def *= 1 + 0.10*float64(talentRank)
			atk *= 1 - 0.04*float64(talentRank)
		case "swift":
			spd += float64(4 * talentRank)
			cr += 3 * talentRank
			res -= 2 * talentRank
		case "siphon":
			atk *= 1 + 0.04*float64(talentRank)
			def *= 1 - 0.03*float64(talentRank)
		case "blight":
			atk *= 1 - 0.06*float64(talentRank)
			ehr += 4 * talentRank
		case "oracle":
			hp *= 1 + 0.04*float64(talentRank)
			res += 4 * talentRank
			atk *= 1 - 0.02*float64(talentRank)
		}
		if state.BattleBuff == "assault" {
			atk *= 1.12
		} else if state.BattleBuff == "bastion" {
			hp *= 1.12
		}
		if elemCounts[girl.Elem] >= 2 {
			atk *= 1.06
		}
		if roleCounts[girl.Role] >= 3 {
			hp *= 1.07
		}
		if len(elemCounts) >= 4 {
			def *= 1.05
		}
		if !state.legacyArenaStats {
			hp, atk, def, spd = collection.stats(id, own, hp, atk, def, spd)
		}
		var refinementEffects []GachaRefinementEffect
		if state.masteryEnabled() {
			var flat GachaStats
			if state.advancedMasteryEnabled() {
				flat, refinementEffects = gachaEquippedArtifactGrades(own)
			} else {
				flat, refinementEffects = gachaEquippedRefinement(own)
			}
			hp, atk, def, spd = hp+float64(flat.HP), atk+float64(flat.Atk), def+float64(flat.Def), spd+float64(flat.Spd)
			cr, cd = cr+flat.CR, cd+flat.CD
		}
		combatGirl := *girl
		combatGirl.Skills.Skill = gachaAwakeningSkill(girl, own)
		u := &gachaCombatUnit{girl: &combatGirl, GachaBattleUnit: GachaBattleUnit{
			GachaBattleSnapshot: GachaBattleSnapshot{UID: prefix + strconv.Itoa(i), Alive: true},
			Slot:                i,
			Side:                side, GirlID: id, Elem: girl.Elem, Name: girl.Name, Img: girl.Img, AtkType: girl.AtkType, Level: own.Level,
			MaxHP: int(math.Round(hp)), Atk: int(math.Round(atk)), Def: int(math.Round(def)), Spd: int(math.Round(spd)),
			CR: min(cr, 100), CD: cd, EHR: min(100, ehr), Res: min(100, max(0, res)),
			Awakening: own.Awakening, Branch: own.Branch,
		}}
		u.healBoost = 1
		switch own.Talent {
		case "power":
			u.skillBoost = 0.05 * float64(talentRank)
		case "guardian":
			u.counter = 0.4 * float64(talentRank)
		case "swift":
			u.rageBonus = 15 * talentRank
			u.Rage = 10 * talentRank
		case "siphon":
			u.lifeSteal = 0.05 * float64(talentRank)
		case "blight":
			u.dotChance, u.dotPct, u.dotCap = 0.35+0.15*float64(talentRank), 0.01*float64(talentRank), 0.6*float64(talentRank)
		case "oracle":
			u.healBoost += 0.15 * float64(talentRank)
		}
		if sigil != nil {
			switch sigil.Family {
			case "ember":
				u.skillBoost += 0.12
			case "tide":
				u.healBoost += 0.15
			case "grove":
				u.dotChance, u.dotPct, u.dotCap = max(u.dotChance, 0.5), max(u.dotPct, 0.01), max(u.dotCap, 0.5)
			case "umbra":
				u.lifeSteal += 0.08
			case "chrono":
				u.rageBonus += 8
				u.Rage += 45
			}
		}
		if state.currentAtelierRules() {
			applyGachaMilestones(u, girl.Role, own.Star)
		}
		for _, effect := range refinementEffects {
			applyGachaCultivationEffect(u, effect.Effect, effect.Value)
		}
		if state.masteryEnabled() && own.Carry != "" {
			applyGachaCarryEffects(u, state, gachaCarry.Items[own.Carry])
		}
		if state.masteryEnabled() {
			applyGachaSkillTraining(u, own.Training, state.advancedMasteryEnabled())
		}
		u.Rage = min(gachaCatalog.Formula.RageMax, u.Rage+own.WeaponAwake*gachaCultivation.WeaponRagePerAwake)
		u.HP = u.MaxHP
		units = append(units, u)
	}
	if state.advancedMasteryEnabled() {
		applyGachaVigilTeam(state, units)
		applyGachaSealedRelic(state, units)
	}
	if state.mirrorTactic != "" {
		applyMirrorTactic(units, state.mirrorTactic)
	}
	return units, nil
}

func (b *gachaCombat) fight(starRound int) *GachaBattleReport {
	report := b.report
	report.Version = gachaBattleReportVersion
	roundLimit := b.roundLimit
	if roundLimit <= 0 || roundLimit > 60 {
		roundLimit = 60
	}
battle:
	for b.wave = 0; b.wave < len(b.waves); b.wave++ {
		b.record(GachaBattleEvent{Kind: "wave", Wave: b.wave, Round: b.rounds, DurationMS: 600, Targets: []GachaBattleTarget{}})
		for len(gachaLivingUnits(b.waves[b.wave])) > 0 && len(gachaLivingUnits(b.allies)) > 0 {
			// A healing/shield stalemate cannot generate an unbounded saved report.
			if b.rounds >= roundLimit {
				report.TimedOut = true
				break battle
			}
			b.rounds++
			order := append(gachaLivingUnits(b.allies), gachaLivingUnits(b.waves[b.wave])...)
			if b.pvp {
				b.rng.Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })
			}
			slices.SortStableFunc(order, func(a, c *gachaCombatUnit) int {
				aSpeed := a.Spd * (100 - a.Slow)
				cSpeed := c.Spd * (100 - c.Slow)
				if aSpeed > cSpeed {
					return -1
				}
				if aSpeed < cSpeed {
					return 1
				}
				return 0
			})
			turnOrder := make([]string, 0, len(order))
			for _, u := range order {
				turnOrder = append(turnOrder, u.UID)
			}
			// The browser replays this order; speed controls never recalculate it.
			b.record(GachaBattleEvent{Kind: "round", Wave: b.wave, Round: b.rounds, Order: turnOrder, Targets: []GachaBattleTarget{}})
			for _, u := range order {
				if !u.Alive {
					continue
				}
				b.act(u)
				if len(gachaLivingUnits(b.waves[b.wave])) == 0 || len(gachaLivingUnits(b.allies)) == 0 {
					break
				}
				if u.Extra {
					u.Extra = false
					b.act(u)
				}
			}
		}
		if len(gachaLivingUnits(b.allies)) == 0 {
			break
		}
	}
	report.Rounds = b.rounds
	report.AliveN = len(gachaLivingUnits(b.allies))
	report.TotalN = len(b.allies)
	report.Win = b.wave == len(b.waves) && report.AliveN > 0 && !report.TimedOut
	if report.Win {
		report.Stars = 1
		if report.AliveN == report.TotalN {
			report.Stars++
		}
		if report.Rounds <= starRound {
			report.Stars++
		}
	}
	report.MaterialBonus = b.materialBonus()
	return report
}

func gachaLivingUnits(units []*gachaCombatUnit) []*gachaCombatUnit {
	living := make([]*gachaCombatUnit, 0, len(units))
	for _, u := range units {
		if u.Alive {
			living = append(living, u)
		}
	}
	return living
}

// Normal attacks meet the opposing front row first. A matching column wins
// ties; a defeated front row exposes its rear row without moving any slots.
// Taunt is selected by act before this fallback. Skills retain their own targets.
func gachaFormationTarget(actor *gachaCombatUnit, foes []*gachaCombatUnit) *gachaCombatUnit {
	var target *gachaCombatUnit
	priority := func(unit *gachaCombatUnit) int {
		rank := unit.Slot
		if unit.Slot%gachaFormationColumns == actor.Slot%gachaFormationColumns {
			rank -= gachaFormationColumns
		}
		if unit.Slot >= gachaFormationColumns {
			rank += GachaSquadLimit
		}
		return rank
	}
	for _, unit := range foes {
		if unit.Alive && (target == nil || priority(unit) < priority(target)) {
			target = unit
		}
	}
	return target
}

func (b *gachaCombat) record(event GachaBattleEvent) {
	event.Seq = len(b.report.Events) + 1
	event.Units = make([]GachaBattleSnapshot, 0, len(b.allies)+len(b.waves[b.wave]))
	for _, u := range b.allies {
		event.Units = append(event.Units, u.GachaBattleSnapshot)
	}
	for _, u := range b.waves[b.wave] {
		event.Units = append(event.Units, u.GachaBattleSnapshot)
	}
	b.report.Events = append(b.report.Events, event)
	b.report.DurationMS += event.DurationMS
}

func (b *gachaCombat) materialBonus() float64 {
	bonus := 0.0
	for _, u := range b.allies {
		if u.Alive && u.girl != nil {
			bonus += u.girl.MatBonus
		}
	}
	return bonus
}

func (u *gachaCombatUnit) effectiveAttack() float64 {
	return float64(u.Atk) * (1 + float64(u.AtkUp)/100)
}

func (b *gachaCombat) damage(src, dst *gachaCombatUnit, mult float64, ignoreDef int) GachaBattleTarget {
	if src.activeSkill != nil && src.activeSkill.LowHPBonus > 0 && dst.HP*2 < dst.MaxHP {
		mult *= 1 + src.activeSkill.LowHPBonus
	}
	defense := float64(dst.Def) * (1 + float64(dst.DefUp)/100) * (1 - float64(dst.DefBreak)/100) * (1 - float64(ignoreDef)/100)
	defMultiplier := 1 - defense/(defense+float64(b.formula.DefK+b.formula.DefLv*src.Level))
	critical := b.rng.Float64()*100 < float64(src.CR)
	critMultiplier := 1.0
	if critical {
		critMultiplier = 1 + float64(src.CD)/100
	}
	bonus := b.materialBonus()
	if b.pvp {
		bonus = 0
		team := b.allies
		if src.Side == "enemy" {
			team = b.waves[b.wave]
		}
		for _, u := range team {
			if u.Alive && u.girl != nil {
				bonus += u.girl.MatBonus
			}
		}
	}
	elementMultiplier, elemental := gachaElementMultiplier(src.Elem, dst.Elem)
	damage := max(1, int(math.Round(src.effectiveAttack()*mult*defMultiplier*critMultiplier*(1+bonus)*elementMultiplier)))
	absorbed := min(dst.Shield, damage)
	dst.Shield -= absorbed
	healthDamage := min(dst.HP, damage-absorbed)
	dst.HP -= healthDamage
	dst.Alive = dst.HP > 0
	result := GachaBattleTarget{UID: dst.UID, Damage: healthDamage, Absorbed: absorbed, Crit: critical, Elemental: elemental}
	if src.Alive && healthDamage > 0 && src.lifeSteal > 0 {
		heal := int(math.Round(float64(healthDamage) * src.lifeSteal))
		if src.DotT > 0 {
			heal /= 2
		}
		result.SelfHeal = min(src.MaxHP-src.HP, heal)
		src.HP += result.SelfHeal
	}
	if (healthDamage > 0 || absorbed > 0) && src.Alive {
		counter := 0
		if healthDamage > 0 && dst.Alive && dst.counter > 0 {
			counter += int(math.Round(float64(dst.Def) * dst.counter))
		}
		if !b.pvp && dst.Side == "enemy" && b.mechanic == "reflection" {
			counter += int(math.Round(float64(healthDamage) * 0.12))
		}
		if dst.Alive && dst.ReflectT > 0 && dst.ReflectPct > 0 {
			counter += absorbed * dst.ReflectPct / 100
		}
		result.Reflected = min(src.HP, counter)
		src.HP -= result.Reflected
		src.Alive = src.HP > 0
	}
	if src.Alive && dst.Alive && src.dotChance > 0 && b.rng.Float64() < min(1, max(0, src.dotChance+float64(src.EHR-dst.Res)/200)) {
		dot := min(int(math.Round(float64(dst.MaxHP)*src.dotPct)), int(math.Round(src.effectiveAttack()*src.dotCap)))
		dst.Dot, dst.DotT = max(dst.Dot, max(1, dot)), 2
	}
	if src.activeSkill != nil {
		skill := src.activeSkill
		if src.Alive && dst.Alive && skill.DotPct > 0 && b.rng.Float64() < min(1, max(0, 1+float64(src.EHR-dst.Res)/200)) {
			dot := min(int(math.Round(float64(dst.MaxHP)*skill.DotPct)), int(math.Round(src.effectiveAttack()*skill.DotCap)))
			dst.Dot, dst.DotT = max(dst.Dot, max(1, dot)), 2
		}
		if dst.Alive && skill.RageDrain > 0 {
			result.RageDelta = -min(dst.Rage, skill.RageDrain)
			dst.Rage += result.RageDelta
		}
	}
	return result
}

func (b *gachaCombat) support(src, target *gachaCombatUnit, skill GachaSkill) GachaBattleTarget {
	result := GachaBattleTarget{UID: target.UID}
	wasLowHP := target.HP*2 < target.MaxHP
	if skill.HealMult > 0 {
		heal := int(math.Round(src.effectiveAttack() * skill.HealMult * max(1, src.healBoost)))
		if target.DotT > 0 {
			heal /= 2
		}
		result.Heal = min(target.MaxHP-target.HP, heal)
		target.HP += result.Heal
	}
	shieldMultiplier := skill.ShieldMult
	if wasLowHP {
		shieldMultiplier += skill.LowHPShield
	}
	if shieldMultiplier > 0 {
		result.Shield = int(math.Round(src.effectiveAttack() * shieldMultiplier * max(1, src.healBoost)))
		target.Shield += result.Shield
		if skill.ShieldReflect > 0 {
			target.ReflectPct, target.ReflectT = int(math.Round(skill.ShieldReflect*100)), 2
		}
	}
	if skill.TeamAtkUp > 0 {
		target.AtkUp, target.AtkUpT = skill.TeamAtkUp, max(1, skill.BuffTurns)
	}
	if skill.RageGain > 0 {
		result.RageDelta = min(b.formula.RageMax-target.Rage, skill.RageGain)
		target.Rage += result.RageDelta
	}
	cleanse := skill.Cleanse
	if src.GirlID == "c13" && src.Awakening == 0 {
		cleanse = max(cleanse, 1)
	}
	for range cleanse {
		switch {
		case target.DotT > 0:
			target.Dot, target.DotT = 0, 0
		case target.StunT > 0:
			target.StunT = 0
		case target.DefBreakT > 0:
			target.DefBreak, target.DefBreakT = 0, 0
		case target.SlowT > 0:
			target.Slow, target.SlowT = 0, 0
		}
	}
	return result
}

// The two three-element cycles are shared by PvE and PvP; unrelated elements are neutral.
func gachaElementMultiplier(attacker, defender string) (float64, string) {
	switch attacker {
	case "fire":
		if defender == "plant" {
			return 1.2, "advantage"
		}
		if defender == "water" {
			return 0.8, "disadvantage"
		}
	case "plant":
		if defender == "water" {
			return 1.2, "advantage"
		}
		if defender == "fire" {
			return 0.8, "disadvantage"
		}
	case "water":
		if defender == "fire" {
			return 1.2, "advantage"
		}
		if defender == "plant" {
			return 0.8, "disadvantage"
		}
	case "star":
		if defender == "dark" {
			return 1.2, "advantage"
		}
		if defender == "time" {
			return 0.8, "disadvantage"
		}
	case "dark":
		if defender == "time" {
			return 1.2, "advantage"
		}
		if defender == "star" {
			return 0.8, "disadvantage"
		}
	case "time":
		if defender == "star" {
			return 1.2, "advantage"
		}
		if defender == "dark" {
			return 0.8, "disadvantage"
		}
	}
	return 1, ""
}

func (b *gachaCombat) act(u *gachaCombatUnit) {
	if u.Alive && u.DotT > 0 {
		damage := min(u.HP, u.Dot)
		u.HP -= damage
		u.Alive = u.HP > 0
		u.DotT--
		if u.DotT == 0 {
			u.Dot = 0
		}
		b.record(GachaBattleEvent{Kind: "ailment", Wave: b.wave, Round: b.rounds, Src: u.UID, SkillName: "侵蚀", DurationMS: 250, Targets: []GachaBattleTarget{{UID: u.UID, Damage: damage}}})
	}
	foes, team := gachaLivingUnits(b.waves[b.wave]), gachaLivingUnits(b.allies)
	if u.Side == "enemy" {
		foes, team = team, foes
	}
	if !u.Alive || len(foes) == 0 {
		return
	}
	if !b.pvp && u.Side == "enemy" && u.girl == nil {
		switch b.mechanic {
		case "fury":
			u.AtkUp, u.AtkUpT = min(150, b.rounds*12), 2
		case "regrowth":
			heal := int(math.Round(float64(u.MaxHP) * 0.08))
			if u.DotT > 0 {
				heal /= 4
			}
			heal = min(u.MaxHP-u.HP, heal)
			if heal > 0 {
				u.HP += heal
				b.record(GachaBattleEvent{Kind: "aura", Wave: b.wave, Round: b.rounds, Src: u.UID, Skill: true, SkillName: "复苏", DurationMS: 350, Targets: []GachaBattleTarget{{UID: u.UID, Heal: heal}}})
			}
		case "barrier":
			if b.rounds%3 == 0 {
				shield := min(u.MaxHP-u.Shield, int(math.Round(float64(u.MaxHP)*0.12)))
				u.Shield += shield
				b.record(GachaBattleEvent{Kind: "aura", Wave: b.wave, Round: b.rounds, Src: u.UID, Skill: true, SkillName: "结界", DurationMS: 350, Targets: []GachaBattleTarget{{UID: u.UID, Shield: shield}}})
			}
		}
	}
	event := GachaBattleEvent{Kind: "action", Wave: b.wave, Round: b.rounds, Src: u.UID, DurationMS: 450, Targets: []GachaBattleTarget{}}
	if u.StunT > 0 {
		u.StunT--
		event.Skipped = true
		b.record(event)
		return
	}
	var taunter *gachaCombatUnit
	b.invokeVigil(u, team, foes, "ready")
	for _, f := range foes {
		if f.Taunt > 0 {
			taunter = f
			break
		}
	}
	if u.girl != nil && u.Rage >= b.formula.RageMax {
		u.Rage = min(b.formula.RageMax, u.castRefund)
		skill := u.girl.Skills.Skill
		u.activeSkill = &skill
		skill.Mult *= 1 + u.skillBoost
		event.Skill, event.SkillName, event.DurationMS = true, skill.Name, 750
		switch skill.Target {
		case "allEnemies":
			for _, f := range foes {
				if !u.Alive {
					break
				}
				event.Targets = append(event.Targets, b.damage(u, f, skill.Mult, skill.IgnoreDefPct))
				if f.Alive && skill.Slow > 0 {
					f.Slow, f.SlowT = skill.Slow, max(1, skill.SlowTurns)
				}
			}
		case "team", "self":
			targets := team
			if skill.Target == "self" {
				targets = []*gachaCombatUnit{u}
			}
			for _, target := range targets {
				event.Targets = append(event.Targets, b.support(u, target, skill))
			}
			// Two ticks retain newly cast one-turn defenses through enemy actions.
			if skill.Taunt {
				u.Taunt = 2
			}
			if skill.SelfDefUp > 0 {
				u.DefUp, u.DefUpT = skill.SelfDefUp, 2
			}
			b.report.GoldBonus = max(b.report.GoldBonus, skill.GoldBonus)
		case "ally":
			var target *gachaCombatUnit
			for _, candidate := range team {
				if skill.ExtraTurn {
					if candidate != u && (target == nil || candidate.effectiveAttack() > target.effectiveAttack()) {
						target = candidate
					}
				} else if target == nil || float64(candidate.HP)/float64(candidate.MaxHP) < float64(target.HP)/float64(target.MaxHP) {
					target = candidate
				}
			}
			if target != nil {
				result := b.support(u, target, skill)
				if skill.ExtraTurn {
					target.Extra = true
				}
				event.Targets = append(event.Targets, result)
			}
		default:
			target := taunter
			if target == nil {
				target = foes[0]
				for _, f := range foes[1:] {
					if f.HP < target.HP {
						target = f
					}
				}
			}
			hits := max(1, skill.Hits)
			for range hits {
				if !target.Alive || !u.Alive {
					break
				}
				event.Targets = append(event.Targets, b.damage(u, target, skill.Mult/float64(hits), skill.IgnoreDefPct))
			}
			event.DurationMS += 150 * (len(event.Targets) - 1)
			if target.Alive {
				if skill.DefBreak > 0 {
					target.DefBreak, target.DefBreakT = skill.DefBreak, max(1, skill.DefBreakTurns)
				}
				if skill.StunChance > 0 && b.rng.Float64() < min(1, max(0, skill.StunChance+float64(u.EHR-target.Res)/200)) {
					target.StunT = max(1, skill.StunTurns)
				}
			}
		}
	} else {
		u.Rage = min(b.formula.RageMax, u.Rage+b.formula.RagePerNormal+u.rageBonus)
		target := taunter
		if target == nil {
			target = gachaFormationTarget(u, foes)
		}
		mult := 1.0
		if u.girl != nil {
			mult = u.girl.Skills.Normal.Mult
		}
		impact := b.damage(u, target, mult, 0)
		if !b.pvp && u.Side == "enemy" && u.girl == nil && target.Alive {
			switch b.mechanic {
			case "venom":
				if b.rng.Float64() < max(0.1, 0.8-float64(target.Res)/150) {
					dot := min(int(math.Round(float64(target.MaxHP)*0.02)), int(math.Round(u.effectiveAttack()*0.75)))
					target.Dot, target.DotT = max(target.Dot, max(1, dot)), 2
				}
			case "silence":
				if b.rounds%2 == 0 {
					impact.RageDelta = -min(target.Rage, 15)
					target.Rage += impact.RageDelta
				}
			}
		}
		event.Targets = append(event.Targets, impact)
		if u.Side == "enemy" && u.girl == nil {
			u.Rage = min(b.formula.RageMax, u.Rage+30)
		}
	}
	u.activeSkill = nil
	if u.ReflectT > 0 {
		u.ReflectT--
		if u.ReflectT == 0 {
			u.ReflectPct = 0
		}
	}
	if u.SlowT > 0 {
		u.SlowT--
		if u.SlowT == 0 {
			u.Slow = 0
		}
	}
	if u.DefBreakT > 0 {
		u.DefBreakT--
		if u.DefBreakT == 0 {
			u.DefBreak = 0
		}
	}
	if u.Taunt > 0 {
		u.Taunt--
	}
	if u.AtkUpT > 0 {
		u.AtkUpT--
		if u.AtkUpT == 0 {
			u.AtkUp = 0
		}
	}
	if u.DefUpT > 0 {
		u.DefUpT--
		if u.DefUpT == 0 {
			u.DefUp = 0
		}
	}
	b.record(event)
	if event.Skill {
		b.invokeVigil(u, team, foes, "cast")
	} else {
		b.invokeVigil(u, team, foes, "strike")
	}
}

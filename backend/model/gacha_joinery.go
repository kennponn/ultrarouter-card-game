package model

import (
	"math"
	"slices"
)

// Joinery state belongs to one server battle. Reports carry immutable copies;
// accounts never accept resources, positions or pulse timers from the browser.
type GachaJoineryState struct {
	Peg           int                 `json:"peg,omitempty"`
	Cooldown      int                 `json:"cooldown,omitempty"`
	FieldUntil    int                 `json:"field_until,omitempty"`
	Pulses        []GachaJoineryPulse `json:"pulses,omitempty"`
	Mark          *GachaJoineryMark   `json:"mark,omitempty"`
	RescueUsed    bool                `json:"rescue_used,omitempty"`
	Guard         int                 `json:"guard,omitempty"`
	Reduction     int                 `json:"reduction,omitempty"`
	Suppressed    int                 `json:"suppressed,omitempty"`
	Confused      int                 `json:"confused,omitempty"`
	Bound         int                 `json:"bound,omitempty"`
	Heavy         int                 `json:"heavy,omitempty"`
	HealBlock     int                 `json:"heal_block,omitempty"`
	Invulnerable  int                 `json:"invulnerable,omitempty"`
	Nullify       int                 `json:"nullify,omitempty"`
	PegRound      int                 `json:"-"`
	ScheduleRound int                 `json:"-"`
	DebuffMeter   int                 `json:"-"`
	CleanRound    int                 `json:"-"`
}

type GachaJoineryPulse struct {
	ID        int     `json:"id"`
	Owner     string  `json:"owner"`
	Countdown int     `json:"countdown"`
	Power     float64 `json:"power"`
	Derived   bool    `json:"derived,omitempty"`
}

type GachaJoineryMark struct {
	Owner string `json:"owner"`
	HP    int    `json:"hp"`
	Rage  int    `json:"rage"`
}

func gachaInitialStars(girl *GachaGirl) int {
	if girl.InitialStars > 0 {
		return girl.InitialStars
	}
	return gachaCatalog.Rarity[girl.Rarity].BaseStar
}

func validGachaCardDefinitions(girls []GachaGirl) bool {
	seen := map[string]bool{}
	for _, girl := range girls {
		if girl.ID == "" || seen[girl.ID] || girl.InitialStars < 0 || girl.InitialStars > 12 {
			return false
		}
		seen[girl.ID] = true
		if girl.CombatProfile != "" && girl.CombatProfile != "joinery" {
			return false
		}
		if girl.CombatProfile == "joinery" && (girl.ID != "xs039" || girl.InitialStars != 5 || girl.StatProfile != "atlas-initial" || girl.Base.SpAtk <= 0 || girl.Base.SpDef <= 0 || girl.Skills.Skill.Cooldown != 3 || girl.Skills.JoineryUltimate.Name == "") {
			return false
		}
	}
	return true
}

func (u *gachaCombatUnit) effectiveSpecial() float64 {
	power := u.SpAtk
	if power <= 0 {
		power = u.Atk
	}
	return float64(power) * (1 + float64(u.AtkUp)/100)
}

func (b *gachaCombat) joineryUnits() []*gachaCombatUnit {
	return append(slices.Clone(b.allies), b.waves[b.wave]...)
}

func (b *gachaCombat) joineryTeam(u *gachaCombatUnit) (team, foes []*gachaCombatUnit) {
	team, foes = b.allies, b.waves[b.wave]
	if u.Side == "enemy" {
		team, foes = foes, team
	}
	return
}

func joineryRank(units []*gachaCombatUnit, score func(*gachaCombatUnit) float64) *gachaCombatUnit {
	var best *gachaCombatUnit
	for _, u := range units {
		if u.Alive && (best == nil || score(u) < score(best) || score(u) == score(best) && u.Slot < best.Slot) {
			best = u
		}
	}
	return best
}

func joineryWeakest(units []*gachaCombatUnit) *gachaCombatUnit {
	return joineryRank(units, func(u *gachaCombatUnit) float64 { return float64(u.HP) / float64(max(1, u.MaxHP)) })
}

func joineryStrongest(units []*gachaCombatUnit) *gachaCombatUnit {
	return joineryRank(units, func(u *gachaCombatUnit) float64 { return -u.effectiveAttack() })
}

func (b *gachaCombat) initJoinery() {
	all := slices.Clone(b.allies)
	for _, wave := range b.waves {
		all = append(all, wave...)
	}
	for _, u := range all {
		if u.girl != nil && u.girl.CombatProfile == "joinery" {
			b.joiners = append(b.joiners, u)
		}
	}
	if len(b.joiners) == 0 {
		return
	}
	for _, u := range all {
		u.Joinery = &GachaJoineryState{PegRound: -1, ScheduleRound: -1, CleanRound: -1}
	}
	for _, owner := range b.joiners {
		owner.Joinery.Peg = 2
		team := b.allies
		if owner.Side == "enemy" {
			for _, wave := range b.waves {
				if slices.Contains(wave, owner) {
					team = wave
					break
				}
			}
		}
		others := slices.DeleteFunc(slices.Clone(team), func(u *gachaCombatUnit) bool { return u == owner })
		for _, u := range []*gachaCombatUnit{owner, joineryStrongest(others)} {
			if u != nil && u.Joinery.Mark == nil && !u.Joinery.RescueUsed {
				u.Joinery.Mark = &GachaJoineryMark{Owner: owner.UID, HP: u.HP, Rage: u.Rage}
			}
		}
	}
	// Capture entry marks before the first event, including both PvP sides.
	for i, u := range b.allies {
		b.report.Allies[i] = u.GachaBattleUnit
		b.report.Allies[i].Joinery = cloneJoinery(u.Joinery)
	}
	for w, wave := range b.waves {
		for i, u := range wave {
			b.report.Waves[w][i] = u.GachaBattleUnit
			b.report.Waves[w][i].Joinery = cloneJoinery(u.Joinery)
		}
	}
}

func cloneJoinery(state *GachaJoineryState) *GachaJoineryState {
	if state == nil {
		return nil
	}
	copy := *state
	copy.Pulses = slices.Clone(state.Pulses)
	if state.Mark != nil {
		mark := *state.Mark
		copy.Mark = &mark
	}
	return &copy
}

func (b *gachaCombat) joineryOwner(uid string) *gachaCombatUnit {
	for _, owner := range b.joiners {
		if owner.UID == uid {
			return owner
		}
	}
	return nil
}

func joineryActive(owner *gachaCombatUnit) bool {
	return owner != nil && owner.Alive && owner.Joinery != nil && owner.Joinery.Suppressed <= 0
}

func (b *gachaCombat) joineryEvent(owner *gachaCombatUnit, name, kind string, hits []GachaBattleTarget) {
	duration := 180
	if kind == "ailment" {
		duration = 360
	}
	b.joineryPending = append(b.joineryPending, GachaBattleEvent{Kind: kind, Src: owner.UID, SkillName: name, Passive: true, Wave: b.wave, Round: b.rounds, Targets: hits, DurationMS: duration})
}

func (b *gachaCombat) joineryCleanse(u *gachaCombatUnit, count int) {
	if u.Joinery == nil {
		return
	}
	j := u.Joinery
	for _, field := range []*int{&u.StunT, &j.Confused, &j.HealBlock, &j.Bound, &j.Heavy, &u.DotT, &u.SlowT, &u.DefBreakT, &j.Suppressed} {
		if count > 0 && *field > 0 {
			*field = 0
			count--
		}
	}
	if u.DotT == 0 {
		u.Dot = 0
	}
	if u.SlowT == 0 {
		u.Slow = 0
	}
	if u.DefBreakT == 0 {
		u.DefBreak = 0
	}
	for count > 0 && len(j.Pulses) > 0 {
		j.Pulses = j.Pulses[1:]
		count--
	}
}

func (b *gachaCombat) joineryLethal(u *gachaCombatUnit) bool {
	j := u.Joinery
	if u.HP > 0 || j == nil || j.Mark == nil || j.RescueUsed {
		return false
	}
	owner := b.joineryOwner(j.Mark.Owner)
	if !joineryActive(owner) || owner.Joinery.Peg < 1 {
		return false
	}
	owner.Joinery.Peg--
	mark := j.Mark
	j.Mark = nil
	j.RescueUsed = true
	j.Guard = 1
	u.HP = max(1, min(mark.HP/2, u.MaxHP*40/100))
	u.Rage = min(100, max(u.Rage, mark.Rage))
	u.Alive = true
	b.joineryCleanse(u, 2)
	b.joineryEvent(owner, "衣角接坠 · 致命救助", "status", []GachaBattleTarget{{UID: u.UID, Rescued: true, HPSet: u.HP}})
	return true
}

func (b *gachaCombat) joineryReduction(u *gachaCombatUnit) float64 {
	if u.Joinery == nil {
		return 1
	}
	j := u.Joinery
	if j.Invulnerable > 0 {
		return 0
	}
	reduction := j.Reduction
	if j.Guard > 0 {
		reduction += 20
	}
	if j.FieldUntil > 0 && j.FieldUntil >= b.rounds {
		reduction -= 10
	}
	return 1 - float64(min(100, max(0, reduction)))/100
}

func (b *gachaCombat) joineryImplant(owner, target *gachaCombatUnit, power float64, derived bool) bool {
	if !target.Alive || target.Joinery == nil || len(target.Joinery.Pulses) >= 2 {
		return false
	}
	count := 0
	for _, u := range b.joineryUnits() {
		for _, pulse := range u.Joinery.Pulses {
			if pulse.Owner == owner.UID {
				count++
			}
		}
	}
	if count >= 4 {
		return false
	}
	b.pulseSequence++
	target.Joinery.Pulses = append(target.Joinery.Pulses, GachaJoineryPulse{ID: b.pulseSequence, Owner: owner.UID, Countdown: 2, Power: power, Derived: derived})
	return true
}

func (b *gachaCombat) joineryDetonate(target *gachaCombatUnit, id int) {
	index := slices.IndexFunc(target.Joinery.Pulses, func(p GachaJoineryPulse) bool { return p.ID == id })
	if index < 0 {
		return
	}
	pulse := target.Joinery.Pulses[index]
	target.Joinery.Pulses = slices.Delete(target.Joinery.Pulses, index, index+1)
	owner := b.joineryOwner(pulse.Owner)
	if owner == nil {
		return
	}
	hits := []GachaBattleTarget{}
	mult := 1.0
	if pulse.Derived {
		mult = .7
	}
	if target.Alive {
		hits = append(hits, b.damageOptions(owner, target, mult, 0, true, pulse.Power, true, false))
	}
	if !pulse.Derived {
		team, _ := b.joineryTeam(target)
		ordered := slices.Clone(team)
		slices.SortFunc(ordered, func(a, c *gachaCombatUnit) int { return a.Slot - c.Slot })
		for _, u := range ordered {
			if u != target && u.Alive && u.Slot/3 == target.Slot/3 && (u.Slot == target.Slot-1 || u.Slot == target.Slot+1) {
				hits = append(hits, b.damageOptions(owner, u, .3, 0, true, pulse.Power, true, false))
			}
		}
		if joineryActive(owner) {
			_, foes := b.joineryTeam(owner)
			candidates := slices.DeleteFunc(slices.Clone(foes), func(u *gachaCombatUnit) bool { return len(u.Joinery.Pulses) > 0 })
			if next := joineryWeakest(candidates); next != nil {
				b.joineryImplant(owner, next, pulse.Power, true)
			}
		}
	}
	name := "留震 · 原生脉冲"
	if pulse.Derived {
		name = "留震 · 衍生脉冲"
	}
	b.joineryEvent(owner, name, "ailment", hits)
}

func (b *gachaCombat) pruneJoinery() {
	for _, owner := range b.joiners {
		j := owner.Joinery
		if j.FieldUntil > 0 && (!joineryActive(owner) || b.rounds > j.FieldUntil) {
			j.FieldUntil = 0
			b.joineryEvent(owner, "三榫弦场结束", "status", nil)
		}
		if !owner.Alive {
			for _, u := range b.joineryUnits() {
				if u.Joinery.Mark != nil && u.Joinery.Mark.Owner == owner.UID {
					u.Joinery.Mark = nil
					b.joineryEvent(owner, "衣角榫印消失", "status", []GachaBattleTarget{{UID: u.UID}})
				}
			}
		}
	}
}

func (b *gachaCombat) flushJoinery() {
	if len(b.joiners) == 0 || b.joineryFlushing {
		return
	}
	b.joineryFlushing = true
	defer func() { b.joineryFlushing = false }()
	for {
		b.pruneJoinery()
		for _, e := range b.joineryPending {
			b.record(e)
		}
		b.joineryPending = nil
		var dead *gachaCombatUnit
		id := 0
		for _, u := range b.joineryUnits() {
			if !u.Alive {
				for _, p := range u.Joinery.Pulses {
					if id == 0 || p.ID < id {
						dead = u
						id = p.ID
					}
				}
			}
		}
		if dead == nil {
			break
		}
		b.joineryDetonate(dead, id)
	}
}

func (b *gachaCombat) joineryBeforeAction(u *gachaCombatUnit) bool {
	if u.Joinery == nil {
		return u.Alive
	}
	b.flushJoinery()
	if !u.Alive {
		return false
	}
	for _, pulse := range slices.Clone(u.Joinery.Pulses) {
		index := slices.IndexFunc(u.Joinery.Pulses, func(p GachaJoineryPulse) bool { return p.ID == pulse.ID })
		if index < 0 {
			continue
		}
		u.Joinery.Pulses[index].Countdown--
		if u.Joinery.Pulses[index].Countdown <= 0 {
			b.joineryDetonate(u, pulse.ID)
			b.flushJoinery()
		}
	}
	return u.Alive
}

func (b *gachaCombat) joineryAfterAction(u *gachaCombatUnit, acted bool) {
	if u.Joinery == nil {
		return
	}
	j := u.Joinery
	if acted && joineryActive(u) && slices.Contains(b.joiners, u) && j.FieldUntil > 0 && j.FieldUntil >= b.rounds && u.StunT <= 0 && j.Confused <= 0 && j.ScheduleRound != b.rounds {
		j.ScheduleRound = b.rounds
		team, foes := b.joineryTeam(u)
		changes := []GachaBattleTarget{}
		candidates := slices.DeleteFunc(slices.Clone(team), func(v *gachaCombatUnit) bool { return v == u || v.Joinery.Cooldown <= 0 })
		if friend := joineryRank(candidates, func(v *gachaCombatUnit) float64 { return -float64(v.Joinery.Cooldown) }); friend != nil {
			friend.Joinery.Cooldown--
			changes = append(changes, GachaBattleTarget{UID: friend.UID})
		}
		if foe := joineryStrongest(foes); foe != nil && foe.Joinery.Invulnerable <= 0 && foe.Joinery.Cooldown < 5 {
			foe.Joinery.Cooldown++
			changes = append(changes, GachaBattleTarget{UID: foe.UID})
		}
		var first *GachaJoineryPulse
		var target *gachaCombatUnit
		for _, v := range b.joineryUnits() {
			for i := range v.Joinery.Pulses {
				p := &v.Joinery.Pulses[i]
				if p.Owner == u.UID && (first == nil || p.ID < first.ID) {
					first = p
					target = v
				}
			}
		}
		if first != nil {
			first.Countdown--
		}
		b.joineryEvent(u, "三榫弦场 · 调度", "status", changes)
		if first != nil && first.Countdown <= 0 {
			b.joineryDetonate(target, first.ID)
		}
	}
	if j.Guard > 0 {
		j.Guard--
	}
	for _, field := range []*int{&j.Suppressed, &j.Confused, &j.Bound, &j.Heavy, &j.HealBlock, &j.Invulnerable} {
		if *field > 0 {
			*field--
		}
	}
	b.flushJoinery()
}

func (b *gachaCombat) joineryEndRound() {
	for _, u := range b.joiners {
		if u.Joinery.FieldUntil > 0 && b.rounds >= u.Joinery.FieldUntil {
			u.Joinery.FieldUntil = 0
			b.joineryEvent(u, "三榫弦场结束", "status", nil)
		}
	}
	b.flushJoinery()
}

func (b *gachaCombat) actJoinery(u *gachaCombatUnit, team, foes []*gachaCombatUnit) GachaBattleEvent {
	j := u.Joinery
	usable := joineryActive(u)
	e := GachaBattleEvent{Kind: "action", Wave: b.wave, Round: b.rounds, Src: u.UID, Targets: []GachaBattleTarget{}}
	switch {
	case usable && u.Rage >= 100:
		skill := u.girl.Skills.JoineryUltimate
		u.Rage = min(100, u.castRefund)
		e.Skill, e.SkillKey, e.SkillName, e.DurationMS = true, "xs039-skill-2", skill.Name, 2520
		e.Variant = "pierce"
		if j.Peg == 3 {
			j.Peg -= 2
			j.FieldUntil = b.rounds + 1
			e.Variant = "field"
		}
		main := gachaFormationTarget(u, foes)
		for _, foe := range foes {
			if foe.Taunt > 0 {
				main = foe
				break
			}
		}
		ordered := slices.Clone(foes)
		slices.SortFunc(ordered, func(a, c *gachaCombatUnit) int { return a.Slot - c.Slot })
		for _, target := range ordered {
			if target.Alive && target.Slot%3 == main.Slot%3 && u.Alive {
				mult := skill.Mult
				if target == main && (main.Joinery.Bound > 0 || main.Joinery.Heavy > 0) {
					mult += .4
				}
				e.Targets = append(e.Targets, b.damageOptions(u, target, mult*(1+u.skillBoost), int(math.Round(skill.IgnoreSpDef*100)), true, 0, false, true))
			}
		}
	case usable && j.Peg > 0 && j.Cooldown == 0:
		skill := u.girl.Skills.Skill
		j.Peg--
		j.Cooldown = skill.Cooldown
		e.Skill, e.SkillKey, e.SkillName, e.DurationMS = true, "skill", skill.Name, 1830
		patient := joineryWeakest(team)
		rescue := patient.HP*100 <= patient.MaxHP*45
		for _, friend := range team {
			rescue = rescue || friend.StunT > 0 || friend.Joinery.Confused > 0
		}
		if rescue {
			e.Variant = "rescue"
			others := slices.DeleteFunc(slices.Clone(team), func(v *gachaCombatUnit) bool { return v == patient })
			other := joineryRank(others, func(v *gachaCombatUnit) float64 { return -float64(v.HP) })
			if other != nil && patient.Joinery.Bound == 0 && other.Joinery.Bound == 0 {
				patient.Slot, other.Slot = other.Slot, patient.Slot
				e.Targets = append(e.Targets, GachaBattleTarget{UID: other.UID})
			}
			heal := int(math.Round(u.effectiveSpecial() * skill.HealMult * max(1, u.healBoost)))
			if patient.Joinery.HealBlock > 0 {
				heal /= 2
			}
			heal = min(patient.MaxHP-patient.HP, heal)
			patient.HP += heal
			b.joineryCleanse(patient, 1)
			e.Targets = append(e.Targets, GachaBattleTarget{UID: patient.UID, Heal: heal})
		} else {
			e.Variant = "cut"
			target := joineryStrongest(foes)
			hit := b.damageOptions(u, target, skill.Mult*(1+u.skillBoost), 0, true, 0, false, true)
			if !hit.Blocked && joineryActive(u) {
				b.joineryImplant(u, target, u.effectiveSpecial(), false)
			}
			e.Targets = append(e.Targets, hit)
		}
	default:
		e.SkillKey, e.SkillName, e.DurationMS = "normal", u.girl.Skills.Normal.Name, 680
		u.Rage = min(100, u.Rage+u.girl.Skills.Normal.RageGain+u.rageBonus)
		target := gachaFormationTarget(u, foes)
		for _, foe := range foes {
			if foe.Taunt > 0 {
				target = foe
				break
			}
		}
		e.Targets = append(e.Targets, b.damageOptions(u, target, u.girl.Skills.Normal.Mult, 0, true, 0, false, false))
		if usable && j.PegRound != b.rounds {
			j.Peg = min(3, j.Peg+1)
			j.PegRound = b.rounds
		}
	}
	return e
}

// Count application events, before another hit can cleanse/kill the recipient.
// Refreshing an existing status is not a new debuff. Queue playback until the
// action is recorded, while applying the cleanse immediately for following hits.
func (b *gachaCombat) joineryNewDebuff(target *gachaCombatUnit, wasActive bool) {
	if wasActive || !target.Alive || target.Joinery == nil {
		return
	}
	for _, owner := range b.joiners {
		if owner.Side != target.Side || !joineryActive(owner) || owner.StunT > 0 || owner.Joinery.Confused > 0 {
			continue
		}
		team, _ := b.joineryTeam(owner)
		j := owner.Joinery
		j.DebuffMeter++
		if j.DebuffMeter >= 3 && j.CleanRound != b.rounds {
			j.DebuffMeter -= 3
			j.CleanRound = b.rounds
			if patient := joineryWeakest(team); patient != nil {
				b.joineryCleanse(patient, 1)
				b.joineryEvent(owner, "衣角接坠 · 术后回访", "status", []GachaBattleTarget{{UID: patient.UID}})
			}
		}
	}
}

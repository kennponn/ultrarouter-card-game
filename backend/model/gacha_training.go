package model

import (
	_ "embed"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/QuantumNous/new-api/common"
)

type GachaBreakGrade struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	MaxPlus    int    `json:"max_plus"`
	Level      int    `json:"level"`
	PlusLevels []int  `json:"plus_levels"`
}

type GachaBreakCost struct {
	GoldBase     int `json:"gold_base"`
	GoldStep     int `json:"gold_step"`
	InkBase      int `json:"ink_base"`
	InkStep      int `json:"ink_step"`
	SealInterval int `json:"seal_interval"`
}

type GachaEffortStage struct {
	Name  string `json:"name"`
	Cap   int    `json:"cap"`
	Gain  int    `json:"gain"`
	Ink   int    `json:"ink"`
	Seals int    `json:"seals"`
}

type GachaEffortStat struct {
	EarlyStep int    `json:"early_step"`
	EarlyGain int    `json:"early_gain"`
	ID        string `json:"id"`
	Name      string `json:"name"`
	Step      int    `json:"step"`
	Gain      int    `json:"gain"`
}

type GachaTrainingRules struct {
	Experience     GachaExperienceRules      `json:"experience"`
	EffortDaily    int                       `json:"effort_daily"`
	EffortGoldBase int                       `json:"effort_gold_base"`
	EffortGoldStep int                       `json:"effort_gold_step"`
	EffortKnee     int                       `json:"effort_knee"`
	SkillPool      GachaSkillPoolRules       `json:"skill_pool"`
	Grades         []GachaBreakGrade         `json:"grades"`
	BreakPercent   int                       `json:"break_percent"`
	BreakCost      GachaBreakCost            `json:"break_cost"`
	EffortStages   []GachaEffortStage        `json:"effort_stages"`
	EffortStats    []GachaEffortStat         `json:"effort_stats"`
	SkillDaily     int                       `json:"skill_daily"`
	SkillLevels    []int                     `json:"skill_levels"`
	SkillGold      []int                     `json:"skill_gold"`
	NormalPercent  int                       `json:"normal_percent"`
	RagePercent    int                       `json:"rage_percent"`
	CastRefund     int                       `json:"cast_refund"`
	NormalRage     int                       `json:"normal_rage"`
	Drops          map[string]map[string]int `json:"drops"`
}

type GachaSkillPoolRules struct {
	Capacity        int `json:"capacity"`
	IntervalSeconds int `json:"interval_seconds"`
	BuyGold         int `json:"buy_gold"`
	UpgradeGoldBase int `json:"upgrade_gold_base"`
	UpgradeGoldStep int `json:"upgrade_gold_step"`
	NormalBPS       int `json:"normal_bps"`
	RageBPS         int `json:"rage_bps"`
	CastRefundEvery int `json:"cast_refund_every"`
	NormalRageEvery int `json:"normal_rage_every"`
}

type GachaSkillPool struct {
	Points int   `json:"points"`
	TickAt int64 `json:"tick_at"`
}

type GachaCardTraining struct {
	Grade  int            `json:"grade,omitempty"`
	Plus   int            `json:"plus,omitempty"`
	Stage  int            `json:"stage,omitempty"`
	Effort map[string]int `json:"effort,omitempty"`
	Skills map[string]int `json:"skills,omitempty"`
}

//go:embed gacha_training.json
var gachaTrainingJSON []byte

var gachaTraining = func() GachaTrainingRules {
	var rules GachaTrainingRules
	if err := common.Unmarshal(gachaTrainingJSON, &rules); err != nil {
		panic(err)
	}
	if rules.EffortGoldBase < 1 || rules.EffortGoldStep < 0 || rules.BreakCost.GoldBase < 1 || rules.BreakCost.GoldStep < 0 || rules.SkillPool.UpgradeGoldBase < 1 || rules.SkillPool.UpgradeGoldStep < 0 {
		panic("invalid training gold prices")
	}
	return rules
}()

func gachaTrainingStat(id string) *GachaEffortStat {
	for index := range gachaTraining.EffortStats {
		if stat := &gachaTraining.EffortStats[index]; stat.ID == id {
			return stat
		}
	}
	return nil
}

func gachaSkillLine(id string) bool { return id == "normal" || id == "rage" || id == "passive" }

func regenGachaSkillPool(state *GachaGameState, now time.Time) error {
	rules, nowMS := gachaTraining.SkillPool, now.UnixMilli()
	if state.Daily.SkillSpent < 0 || state.Daily.SkillSpent > gachaTraining.SkillDaily {
		return ErrGachaRunState
	}
	pool := state.SkillPool
	if pool == nil {
		state.SkillPool = &GachaSkillPool{Points: rules.Capacity - state.Daily.SkillSpent, TickAt: nowMS}
		state.Daily.SkillSpent = 0
		return nil
	}
	if pool.Points < 0 || pool.Points > rules.Capacity || pool.TickAt < 0 || state.Daily.SkillSpent != 0 {
		return ErrGachaRunState
	}
	if nowMS <= pool.TickAt {
		return nil
	}
	if pool.Points == rules.Capacity {
		pool.TickAt = nowMS
		return nil
	}
	interval := int64(rules.IntervalSeconds) * 1000
	ticks := min((nowMS-pool.TickAt)/interval, int64(rules.Capacity-pool.Points))
	pool.Points += int(ticks)
	pool.TickAt += ticks * interval
	if pool.Points == rules.Capacity {
		pool.TickAt = nowMS
	}
	return nil
}

func validateGachaSkillIntent(intent GachaActionIntent) error {
	if intent.ItemID != nil || intent.Enabled != nil || intent.Squad != nil || intent.Config != nil || intent.Materials != nil || intent.OpponentID != nil || intent.StakeE != nil || intent.QueueID != nil || intent.RoomID != nil {
		return ErrGachaIntent
	}
	switch intent.Action {
	case "skill_buy":
		if intent.GirlID == nil && intent.Choice == nil && intent.Count != nil && *intent.Count >= 1 && *intent.Count <= gachaTraining.SkillPool.Capacity {
			return nil
		}
	case "skill_point", "skill_auto":
		if intent.Count == nil && intent.GirlID != nil && gachaGirl(*intent.GirlID) != nil && intent.Choice != nil && gachaSkillLine(*intent.Choice) {
			return nil
		}
	}
	return ErrGachaIntent
}

func applyGachaSkillAction(state *GachaGameState, intent GachaActionIntent, result *GachaActionResult, now time.Time) error {
	if err := validateGachaSkillIntent(intent); err != nil {
		return err
	}
	rules, pool := gachaTraining.SkillPool, state.SkillPool
	if pool == nil || pool.Points < 0 || pool.Points > rules.Capacity {
		return ErrGachaRunState
	}
	if intent.Action == "skill_buy" {
		count := *intent.Count
		if count > rules.Capacity-pool.Points {
			return ErrGachaCapacity
		}
		cost := count * rules.BuyGold
		if state.Gold < cost {
			return ErrGachaFunds
		}
		state.Gold -= cost
		pool.Points += count
		if pool.Points == rules.Capacity {
			pool.TickAt = max(pool.TickAt, now.UnixMilli())
		}
		result.Drops = append(result.Drops, fmt.Sprintf("购买 %d 技能点 · 消耗 %d 金币", count, cost))
		return nil
	}
	own := state.Girls[*intent.GirlID]
	if !gachaCardOwned(own) || !validGachaCardTraining(own) {
		return ErrGachaCultivationLocked
	}
	line := *intent.Choice
	trained := own.Training
	starting := 0
	if trained != nil {
		starting = trained.Skills[line]
	}
	if starting >= own.Level {
		return ErrGachaCultivationLocked
	}
	limit := min(own.Level-starting, pool.Points)
	if intent.Action == "skill_point" {
		limit = min(limit, 1)
	}
	count, cost := 0, 0
	for count < limit {
		next := rules.UpgradeGoldBase + rules.UpgradeGoldStep*(starting+count)
		if next > state.Gold-cost {
			break
		}
		count++
		cost += next
	}
	if count == 0 {
		return ErrGachaFunds
	}
	if trained == nil {
		trained = &GachaCardTraining{}
		own.Training = trained
	}
	if trained.Skills == nil {
		trained.Skills = make(map[string]int, 3)
	}
	if pool.Points == rules.Capacity {
		pool.TickAt = max(pool.TickAt, now.UnixMilli())
	}
	trained.Skills[line] = starting + count
	pool.Points -= count
	state.Gold -= cost
	advanceGachaCaptainTaskBy(state, "skill", count)
	name := map[string]string{"normal": "普攻", "rage": "怒气技", "passive": "回怒"}[line]
	result.Drops = append(result.Drops, fmt.Sprintf("%s研修 %d → %d 级 · %d 技能点 · %d 金币", name, starting, starting+count, count, cost))
	return nil
}

func validGachaCardTraining(own *GachaOwnedGirl) bool {
	return validGachaCardTrainingFor(own, true)
}

func validGachaCardTrainingFor(own *GachaOwnedGirl, advanced bool) bool {
	trained := own.Training
	if trained == nil {
		return true
	}
	if !gachaCardOwned(own) || trained.Grade < 0 || trained.Grade >= len(gachaTraining.Grades) || !validGachaEffort(trained, advanced) {
		return false
	}
	grade := gachaTraining.Grades[trained.Grade]
	if trained.Plus < 0 || trained.Plus > grade.MaxPlus || gachaPeakLevel(own) < grade.Level {
		return false
	}
	return validGachaSkillRanks(own, advanced)
}

func validGachaEffort(trained *GachaCardTraining, advanced bool) bool {
	if trained.Stage < 0 || trained.Stage >= len(gachaTraining.EffortStages) || !advanced && trained.Stage >= 6 {
		return false
	}
	for key := range trained.Effort {
		if gachaTrainingStat(key) == nil {
			return false
		}
	}
	floor, ceiling := 0, gachaTraining.EffortStages[trained.Stage].Cap
	if !advanced {
		floor, ceiling = trained.Stage*2, trained.Stage*2+2
	} else if trained.Stage > 0 {
		floor = gachaTraining.EffortStages[trained.Stage-1].Cap
	}
	for _, stat := range gachaTraining.EffortStats {
		if value := trained.Effort[stat.ID]; value < floor || value > ceiling {
			return false
		}
	}
	return true
}

func migrateGachaEffort(state *GachaGameState) error {
	if state.EffortVersion == 1 {
		return nil
	}
	if state.EffortVersion != 0 || len(state.Daily.EffortSpent) != 0 {
		return ErrGachaRunState
	}
	for _, own := range state.Girls {
		if own == nil || own.Training != nil && !validGachaCardTrainingFor(own, false) {
			return ErrGachaRunState
		}
	}
	for _, own := range state.Girls {
		trained := own.Training
		if trained == nil {
			continue
		}
		floor := 0
		if trained.Stage > 0 {
			floor = gachaTraining.EffortStages[trained.Stage-1].Cap
		}
		span := gachaTraining.EffortStages[trained.Stage].Cap - floor
		for stat, old := range trained.Effort {
			trained.Effort[stat] = floor + (old-trained.Stage*2)*span/2
		}
	}
	state.EffortVersion = 1
	return nil
}

func validGachaSkillRanks(own *GachaOwnedGirl, advanced bool) bool {
	if own.Training == nil {
		return true
	}
	for key, level := range own.Training.Skills {
		if !gachaSkillLine(key) || level < 0 {
			return false
		}
		if advanced {
			if level > gachaPeakLevel(own) || level > gachaCatalog.Growth.MaxLevel {
				return false
			}
		} else if level > len(gachaTraining.SkillLevels) || level > 0 && own.Level < gachaTraining.SkillLevels[level-1] {
			return false
		}
	}
	return true
}

func validateGachaTrainingIntent(intent GachaActionIntent) error {
	if intent.GirlID == nil || gachaGirl(*intent.GirlID) == nil || intent.Count != nil || intent.ItemID != nil || intent.Enabled != nil || intent.Squad != nil || intent.Config != nil || intent.OpponentID != nil || intent.StakeE != nil || intent.QueueID != nil || intent.RoomID != nil {
		return ErrGachaIntent
	}
	switch intent.Action {
	case "effort_train", "effort_train_max":
		if intent.Choice != nil && gachaTrainingStat(*intent.Choice) != nil {
			return nil
		}
	case "card_breakthrough", "effort_promote":
		if intent.Choice == nil {
			return nil
		}
	}
	return ErrGachaIntent
}

func applyGachaTrainingAction(state *GachaGameState, intent GachaActionIntent, result *GachaActionResult) error {
	if err := validateGachaTrainingIntent(intent); err != nil {
		return err
	}
	own := state.Girls[*intent.GirlID]
	if !gachaCardOwned(own) || !validGachaCardTraining(own) {
		return ErrGachaCultivationLocked
	}
	trained := GachaCardTraining{}
	if own.Training != nil {
		trained = *own.Training
	}
	stage := gachaTraining.EffortStages[trained.Stage]
	ink, seals, gold, stones := 0, 0, 0, 0
	var receipt string
	sessions := 0
	switch intent.Action {
	case "card_breakthrough":
		grade := gachaTraining.Grades[trained.Grade]
		completed := trained.Plus
		for _, previous := range gachaTraining.Grades[:trained.Grade] {
			completed += previous.MaxPlus + 1
		}
		cost := gachaTraining.BreakCost
		gold = cost.GoldBase + cost.GoldStep*completed
		ink = cost.InkBase + cost.InkStep*completed + completed*(completed+1)/2
		seals = (completed + 1) / cost.SealInterval
		var requiredLevel int
		if trained.Plus < grade.MaxPlus {
			requiredLevel = grade.PlusLevels[trained.Plus]
			trained.Plus++
		} else {
			if trained.Grade+1 >= len(gachaTraining.Grades) {
				return ErrGachaCultivationLocked
			}
			grade = gachaTraining.Grades[trained.Grade+1]
			requiredLevel = grade.Level
			trained.Grade++
			trained.Plus = 0
		}
		if own.Level < requiredLevel {
			return ErrGachaCultivationLocked
		}
		receipt = fmt.Sprintf("卡牌突破至%s +%d · 研训墨 −%d", grade.Name, trained.Plus, ink)
		if seals > 0 {
			receipt += fmt.Sprintf(" · 晋阶章 −%d", seals)
		}
	case "effort_train", "effort_train_max":
		key := *intent.Choice
		used := state.Daily.EffortSpent[*intent.GirlID]
		if trained.Effort[key] >= stage.Cap || used < 0 || used >= gachaTraining.EffortDaily {
			return ErrGachaCultivationLocked
		}
		sessions = 1
		perSession := gachaTraining.EffortGoldBase + gachaTraining.EffortGoldStep*trained.Stage
		if intent.Action == "effort_train_max" {
			sessions = min(gachaTraining.EffortDaily-used, (stage.Cap-trained.Effort[key]+stage.Gain-1)/stage.Gain, (state.Mats["training_ink"]+state.Mats["effort_stone"])/stage.Ink, state.Gold/perSession)
			if sessions < 1 {
				return ErrGachaFunds
			}
		}
		ink = stage.Ink * sessions
		stones = min(state.Mats["effort_stone"], ink)
		ink -= stones
		gold = perSession * sessions
		trained.Effort = maps.Clone(trained.Effort)
		if trained.Effort == nil {
			trained.Effort = map[string]int{}
		}
		trained.Effort[key] = min(stage.Cap, trained.Effort[key]+stage.Gain*sessions)
		receipt = fmt.Sprintf("%s训练 %d 次 · 努力值 %d/%d", gachaTrainingStat(key).Name, sessions, trained.Effort[key], stage.Cap)
	case "effort_promote":
		if trained.Stage+1 >= len(gachaTraining.EffortStages) {
			return ErrGachaCultivationLocked
		}
		for _, stat := range gachaTraining.EffortStats {
			if trained.Effort[stat.ID] != stage.Cap {
				return ErrGachaCultivationLocked
			}
		}
		trained.Stage++
		next := gachaTraining.EffortStages[trained.Stage]
		seals = next.Seals
		receipt = "努力值晋段：" + next.Name
	}
	if state.Mats["training_ink"] < ink || state.Mats["break_seal"] < seals || state.Gold < gold {
		return ErrGachaFunds
	}
	if stones > 0 {
		state.Mats["effort_stone"] -= stones
		receipt += fmt.Sprintf(" · 努力培养石 −%d", stones)
	}
	if gold > 0 {
		state.Gold -= gold
		receipt += fmt.Sprintf(" · 金币 −%d", gold)
	}
	if ink > 0 {
		state.Mats["training_ink"] -= ink
		if sessions > 0 {
			receipt += fmt.Sprintf(" · 研训墨 −%d", ink)
		}
	}
	if seals > 0 {
		state.Mats["break_seal"] -= seals
	}
	own.Training = &trained
	if sessions > 0 {
		if state.Daily.EffortSpent == nil {
			state.Daily.EffortSpent = map[string]int{}
		}
		state.Daily.EffortSpent[*intent.GirlID] += sessions
	}
	result.Drops = append(result.Drops, receipt)
	return nil
}

func gachaTrainingBonuses(trained *GachaCardTraining, advanced bool) (int, GachaStats) {
	var flat GachaStats
	if trained == nil {
		return 0, flat
	}
	steps := trained.Plus
	for _, grade := range gachaTraining.Grades[:trained.Grade] {
		steps += grade.MaxPlus + 1
	}
	for _, stat := range gachaTraining.EffortStats {
		value := trained.Effort[stat.ID]
		gain := min(value, gachaTraining.EffortKnee)/stat.EarlyStep*stat.EarlyGain + max(0, value-gachaTraining.EffortKnee)/stat.Step*stat.Gain
		if !advanced {
			switch stat.ID {
			case "hp":
				gain = value * 18
			case "atk":
				gain = value * 3
			case "def":
				gain = value * 2
			default:
				gain = value / 2
			}
		}
		switch stat.ID {
		case "hp":
			flat.HP = gain
		case "atk":
			flat.Atk = gain
		case "def":
			flat.Def = gain
		case "spd":
			flat.Spd = gain
		case "res":
			flat.Res = gain
		}
	}
	return steps * gachaTraining.BreakPercent, flat
}

func applyGachaSkillTraining(unit *gachaCombatUnit, trained *GachaCardTraining, advanced bool) {
	if trained == nil {
		return
	}
	normal, rage, passive := trained.Skills["normal"], trained.Skills["rage"], trained.Skills["passive"]
	normalBonus, rageBonus := float64(normal*gachaTraining.NormalPercent)/100, float64(rage*gachaTraining.RagePercent)/100
	refund, extra := rage*gachaTraining.CastRefund, passive*gachaTraining.NormalRage
	if advanced {
		rules := gachaTraining.SkillPool
		normal, rage, passive = min(normal, unit.Level), min(rage, unit.Level), min(passive, unit.Level)
		normalBonus, rageBonus = float64(normal*rules.NormalBPS)/10000, float64(rage*rules.RageBPS)/10000
		refund, extra = rage/rules.CastRefundEvery, passive/rules.NormalRageEvery
	}
	unit.girl.Skills.Normal.Mult *= 1 + normalBonus
	boost := 1 + rageBonus
	skill := &unit.girl.Skills.Skill
	skill.Mult *= boost
	skill.HealMult *= boost
	skill.ShieldMult *= boost
	skill.LowHPShield *= boost
	unit.castRefund = refund
	unit.rageBonus += extra
}

func awardGachaTrainingLoot(state *GachaGameState, stage *GachaStage, result *GachaFinishResult) error {
	drops := gachaTraining.Drops[gachaMaterialStageKind(stage)]
	for id, amount := range drops {
		if state.Mats[id] > gachaMaxResource-amount {
			return ErrGachaCapacity
		}
	}
	for _, id := range slices.Sorted(maps.Keys(drops)) {
		state.Mats[id] += drops[id]
		result.Drops = append(result.Drops, fmt.Sprintf("%s x%d", gachaItems.Items[id].Name, drops[id]))
	}
	return nil
}

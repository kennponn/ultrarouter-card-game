package model

import (
	_ "embed"
	"fmt"
	"maps"
	"slices"

	"github.com/QuantumNous/new-api/common"
)

const gachaEquipmentLootVersion = 5

type GachaEquipmentSlot struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Stat     string `json:"stat"`
	PerLevel int    `json:"per_level"`
	PerStar  int    `json:"per_star"`
	LevelMat string `json:"level_mat"`
}

type GachaEquipmentTier struct {
	Name   string         `json:"name"`
	Pieces int            `json:"pieces"`
	Star   int            `json:"star"`
	Stats  map[string]int `json:"stats"`
}

type GachaEquipmentQuality struct {
	Level   int    `json:"level"`
	Quality string `json:"quality"`
}

type GachaEquipmentMaterial struct {
	Name string `json:"name"`
	From string `json:"from"`
	Use  string `json:"use"`
}

type GachaEquipmentRules struct {
	Slots         []GachaEquipmentSlot              `json:"slots"`
	LevelCosts    []int                             `json:"level_costs"`
	StarMat       string                            `json:"star_mat"`
	StarMarkMat   string                            `json:"star_mark_mat"`
	StarMarkCosts []int                             `json:"star_mark_costs"`
	StarCosts     []int                             `json:"star_costs"`
	StarLevels    []int                             `json:"star_levels"`
	StarCardStars []int                             `json:"star_card_stars"`
	SetTiers      []GachaEquipmentTier              `json:"set_tiers"`
	QualitySteps  []GachaEquipmentQuality           `json:"quality_steps"`
	Refinement    GachaRefinementRules              `json:"refinement"`
	Materials     map[string]GachaEquipmentMaterial `json:"materials"`
	Drops         map[string]map[string]int         `json:"drops"`
}

type GachaEquipmentPiece struct {
	Level int `json:"level,omitempty"`
	Star  int `json:"star,omitempty"`
}

//go:embed gacha_equipment.json
var gachaEquipmentJSON []byte

var gachaEquipment = func() GachaEquipmentRules {
	var rules GachaEquipmentRules
	if err := common.Unmarshal(gachaEquipmentJSON, &rules); err != nil {
		panic(err)
	}
	if len(rules.StarCosts) == 0 || len(rules.StarLevels) != len(rules.StarCosts) || len(rules.StarCardStars) != len(rules.StarCosts) || len(rules.StarMarkCosts) != len(rules.StarCosts) ||
		len(rules.LevelCosts) < len(gachaCultivation.WeaponOre) || !slices.Equal(rules.LevelCosts[:len(gachaCultivation.WeaponOre)], gachaCultivation.WeaponOre) || !validGachaRefinementRules(rules.Refinement) || slices.ContainsFunc(rules.LevelCosts, func(cost int) bool { return cost <= 0 }) ||
		slices.ContainsFunc(rules.StarCosts, func(cost int) bool { return cost <= 0 }) {
		panic("invalid gacha equipment costs")
	}
	return rules
}()

func gachaEquipmentSlot(id string) *GachaEquipmentSlot {
	for index := range gachaEquipment.Slots {
		if slot := &gachaEquipment.Slots[index]; slot.ID == id {
			return slot
		}
	}
	return nil
}

func gachaEquipmentPiece(own *GachaOwnedGirl, id string) GachaEquipmentPiece {
	piece := own.Equipment[id]
	if id == "weapon" {
		piece.Level = own.WeaponLevel
	}
	return piece
}

func validGachaEquipmentState(own *GachaOwnedGirl) bool {
	if len(own.Equipment) > len(gachaEquipment.Slots) || own.Star == 0 && len(own.Equipment) > 0 {
		return false
	}
	for id, stored := range own.Equipment {
		if gachaEquipmentSlot(id) == nil || id == "weapon" && stored.Level != 0 {
			return false
		}
		piece := gachaEquipmentPiece(own, id)
		if piece.Level < 0 || piece.Level > len(gachaEquipment.LevelCosts) || piece.Star < 0 || piece.Star > len(gachaEquipment.StarCosts) {
			return false
		}
		if piece.Star > 0 && (piece.Level < gachaEquipment.StarLevels[piece.Star-1] || own.Star < gachaEquipment.StarCardStars[piece.Star-1]) {
			return false
		}
	}
	return true
}

func validateGachaEquipmentIntent(intent GachaActionIntent) error {
	if intent.Count != nil || intent.Enabled != nil || intent.Choice != nil || intent.Squad != nil || intent.Config != nil || intent.QueueID != nil || intent.RoomID != nil || intent.OpponentID != nil || intent.StakeE != nil ||
		intent.GirlID == nil || gachaGirl(*intent.GirlID) == nil || intent.ItemID == nil || gachaEquipmentSlot(*intent.ItemID) == nil {
		return ErrGachaIntent
	}
	switch intent.Action {
	case "gear_upgrade", "gear_auto", "gear_star":
		return nil
	}
	return ErrGachaIntent
}

func applyGachaEquipmentAction(state *GachaGameState, girlID, id, action string, result *GachaActionResult) error {
	own := state.Girls[girlID]
	if !gachaCardOwned(own) {
		return ErrGachaIntent
	}
	slot := gachaEquipmentSlot(id)
	if slot == nil {
		return ErrGachaIntent
	}
	piece := gachaEquipmentPiece(own, id)
	levelBefore := piece.Level
	material, cost, markCost := slot.LevelMat, 0, 0
	var receipt string
	switch action {
	case "gear_upgrade", "gear_auto", "weapon_upgrade":
		if piece.Level >= len(gachaEquipment.LevelCosts) || action == "weapon_upgrade" && piece.Level >= len(gachaCultivation.WeaponOre) {
			return ErrGachaCultivationLocked
		}
		for piece.Level < len(gachaEquipment.LevelCosts) {
			nextCost := gachaEquipment.LevelCosts[piece.Level]
			if state.Mats[material]-cost < nextCost {
				break
			}
			cost += nextCost
			piece.Level++
			if action != "gear_auto" {
				break
			}
		}
		if cost == 0 {
			return ErrGachaFunds
		}
		receipt = fmt.Sprintf("%s强化 +%d", slot.Name, piece.Level)
	case "gear_star":
		tier := piece.Star
		if tier >= len(gachaEquipment.StarCosts) || piece.Level < gachaEquipment.StarLevels[tier] || own.Star < gachaEquipment.StarCardStars[tier] {
			return ErrGachaCultivationLocked
		}
		material, cost = gachaEquipment.StarMat, gachaEquipment.StarCosts[tier]
		markCost = gachaEquipment.StarMarkCosts[tier]
		if state.Mats[material] < cost || state.Mats[gachaEquipment.StarMarkMat] < markCost {
			return ErrGachaFunds
		}
		piece.Star++
		receipt = fmt.Sprintf("%s升至%d星", slot.Name, piece.Star)
	default:
		return ErrGachaIntent
	}
	state.Mats[material] -= cost
	advanceGachaCaptainTaskBy(state, "equipment", piece.Level-levelBefore)
	if markCost > 0 {
		state.Mats[gachaEquipment.StarMarkMat] -= markCost
	}
	if id == "weapon" {
		own.WeaponLevel = piece.Level
		piece.Level = 0
	}
	if piece != (GachaEquipmentPiece{}) {
		if own.Equipment == nil {
			own.Equipment = map[string]GachaEquipmentPiece{}
		}
		own.Equipment[id] = piece
	}
	if action == "weapon_upgrade" {
		receipt = fmt.Sprintf("%s · 专武强化 +%d", gachaGirl(girlID).Name, own.WeaponLevel)
	} else {
		name := gachaEquipment.Materials[material].Name
		if name == "" {
			name = gachaItems.Items[material].Name
		}
		receipt = fmt.Sprintf("%s · %s -%d", receipt, name, cost)
		if markCost > 0 {
			receipt += fmt.Sprintf(" · %s -%d", gachaItems.Items[gachaEquipment.StarMarkMat].Name, markCost)
		}
	}
	result.Drops = append(result.Drops, receipt)
	return nil
}

func (bonus *GachaStats) addCultivationStat(stat string, value int) {
	switch stat {
	case "hp":
		bonus.HP += value
	case "atk":
		bonus.Atk += value
	case "def":
		bonus.Def += value
	case "spd":
		bonus.Spd += value
	case "cr":
		bonus.CR += value
	case "cd":
		bonus.CD += value
	}
}

func addGachaEquipmentBonuses(bonus *GachaStats, id string, own *GachaOwnedGirl, mastery bool) {
	if len(own.Equipment) == 0 {
		return
	}
	for _, slot := range gachaEquipment.Slots {
		piece := gachaEquipmentPiece(own, slot.ID)
		if !mastery {
			piece.Level, piece.Star = min(10, piece.Level), min(5, piece.Star)
		}
		stat := slot.Stat
		if stat == "signature" {
			stat = gachaCultivation.Weapons[id].Stat
		}
		bonus.addCultivationStat(stat, slot.PerLevel*piece.Level+slot.PerStar*piece.Star)
	}
	for _, tier := range gachaEquipment.SetTiers {
		if !mastery && tier.Star > 5 {
			continue
		}
		pieces := 0
		for _, piece := range own.Equipment {
			if piece.Star >= tier.Star {
				pieces++
			}
		}
		if pieces >= tier.Pieces {
			for stat, amount := range tier.Stats {
				bonus.addCultivationStat(stat, amount)
			}
		}
	}
}

func awardGachaEquipmentMaterials(state *GachaGameState, stage *GachaStage, result *GachaFinishResult) error {
	drops := gachaEquipment.Drops[gachaMaterialStageKind(stage)]
	for id, amount := range drops {
		if state.Mats[id] > gachaMaxResource-amount {
			return ErrGachaCapacity
		}
	}
	for _, id := range slices.Sorted(maps.Keys(drops)) {
		state.Mats[id] += drops[id]
		result.Drops = append(result.Drops, fmt.Sprintf("%s x%d", gachaEquipment.Materials[id].Name, drops[id]))
	}
	return nil
}

package model

import (
	_ "embed"
	"fmt"
	"math/bits"
	"slices"

	"github.com/QuantumNous/new-api/common"
)

type GachaCultivationEffect struct {
	Name  string `json:"name"`
	Stat  string `json:"stat"`
	Value int    `json:"value,omitempty"`
}

type GachaCollectionBond struct {
	Name     string   `json:"name"`
	Members  []string `json:"members"`
	Stat     string   `json:"stat"`
	Base     int      `json:"base,omitempty"`
	Advanced int      `json:"advanced,omitempty"`
}

type GachaStarMilestone struct {
	Star   int    `json:"star"`
	Name   string `json:"name"`
	Effect string `json:"effect"`
	Value  int    `json:"value"`
}

type GachaShardRules struct {
	PerOre           map[string]int `json:"per_ore"`
	LegacyDustPerOre int            `json:"legacy_dust_per_ore"`
}

type GachaSocketRule struct {
	Star int `json:"star"`
	Gold int `json:"gold"`
	Mat  int `json:"mat"`
}

type GachaArtifactBonus struct {
	Count int    `json:"count,omitempty"`
	Stat  string `json:"stat"`
	Value int    `json:"value"`
}

type GachaArtifactSet struct {
	Name  string               `json:"name"`
	Base  *GachaArtifactBonus  `json:"base,omitempty"`
	Tiers []GachaArtifactBonus `json:"tiers"`
}

type GachaArtifactRules struct {
	GemSockets []GachaSocketRule           `json:"gem_sockets"`
	ChipStars  []int                       `json:"chip_stars"`
	GemSets    map[string]GachaArtifactSet `json:"gem_sets"`
	ChipSets   map[string]GachaArtifactSet `json:"chip_sets"`
	TicketOre  int                         `json:"ticket_ore"`
	DrawToken  int                         `json:"draw_token"`
}

type GachaIndividualStat struct {
	Stat string `json:"stat"`
	Step int    `json:"step"`
}

type GachaIndividualRules struct {
	MaxValue   int                   `json:"max_value"`
	Gold       int                   `json:"gold"`
	Solvent    int                   `json:"solvent"`
	LockTokens []int                 `json:"lock_tokens"`
	Stats      []GachaIndividualStat `json:"stats"`
}

type GachaCultivationRules struct {
	AdvancedShards         map[string][]int                  `json:"advanced_shards"`
	AdvancedAwakening      []int                             `json:"advanced_awakening"`
	AdvancedMatMultiplier  []int                             `json:"advanced_mat_multiplier"`
	PotentialStars         []int                             `json:"potential_stars"`
	PotentialMats          []int                             `json:"potential_mats"`
	PotentialPerNode       int                               `json:"potential_per_node"`
	WeaponOre              []int                             `json:"weapon_ore"`
	WeaponCores            []int                             `json:"weapon_cores"`
	WeaponLevelGates       []int                             `json:"weapon_level_gates"`
	WeaponAwakeningGates   []int                             `json:"weapon_awakening_gates"`
	WeaponPerLevel         int                               `json:"weapon_per_level"`
	WeaponRagePerAwake     int                               `json:"weapon_rage_per_awake"`
	WeaponVitalityPerAwake int                               `json:"weapon_vitality_per_awake"`
	WeaponRuneLevel        int                               `json:"weapon_rune_level"`
	WeaponRuneOre          int                               `json:"weapon_rune_ore"`
	Engravings             map[string]GachaCultivationEffect `json:"engravings"`
	ManualXP               int                               `json:"manual_xp"`
	CodexFlat              map[string]int                    `json:"codex_flat"`
	BondBase               int                               `json:"bond_base"`
	BondAdvanced           int                               `json:"bond_advanced"`
	BondStar               int                               `json:"bond_star"`
	Weapons                map[string]GachaCultivationEffect `json:"weapons"`
	Bonds                  []GachaCollectionBond             `json:"bonds"`
	Fellowships            []GachaCollectionBond             `json:"fellowships"`
	Crews                  []GachaCollectionBond             `json:"crews"`
	BondCap                int                               `json:"bond_cap"`
	Milestones             map[string][]GachaStarMilestone   `json:"milestones"`
	Shards                 GachaShardRules                   `json:"shards"`
	Artifacts              GachaArtifactRules                `json:"artifacts"`
	Individual             GachaIndividualRules              `json:"individual"`
}

//go:embed gacha_cultivation.json
var gachaCultivationJSON []byte

var gachaCultivation = func() GachaCultivationRules {
	var rules GachaCultivationRules
	if err := common.Unmarshal(gachaCultivationJSON, &rules); err != nil {
		panic(err)
	}
	individual := rules.Individual
	if individual.Gold < 1 || individual.Solvent < 1 || len(individual.Stats) != 6 || len(individual.LockTokens) != len(individual.Stats) || individual.LockTokens[0] != 0 {
		panic("invalid individual value prices")
	}
	for index := 1; index < len(individual.LockTokens); index++ {
		if individual.LockTokens[index] <= individual.LockTokens[index-1] || individual.LockTokens[index] > gachaMaxResource {
			panic("invalid individual value lock price")
		}
	}
	return rules
}()

func gachaIndividualIndex(stat string) int {
	return slices.IndexFunc(gachaCultivation.Individual.Stats, func(item GachaIndividualStat) bool { return item.Stat == stat })
}

func validGachaIndividualState(own *GachaOwnedGirl) bool {
	if len(own.IVValues) == 0 {
		return own.IVLocks == 0
	}
	rules := &gachaCultivation.Individual
	return own.Star > 0 && len(own.IVValues) == len(rules.Stats) && own.IVLocks >= 0 && own.IVLocks < 1<<len(rules.Stats) &&
		!slices.ContainsFunc(own.IVValues, func(value int) bool { return value < 0 || value > rules.MaxValue })
}

func applyGachaIndividualAction(state *GachaGameState, own *GachaOwnedGirl, intent GachaActionIntent, result *GachaActionResult) error {
	rules := &gachaCultivation.Individual
	if intent.Action == "iv_lock" {
		if len(own.IVValues) == 0 {
			return ErrGachaCultivationLocked
		}
		bit := 1 << gachaIndividualIndex(*intent.Choice)
		if *intent.Enabled {
			own.IVLocks |= bit
		} else {
			own.IVLocks &^= bit
		}
		return nil
	}
	locked := bits.OnesCount(uint(own.IVLocks))
	if locked >= len(rules.Stats) || locked >= len(rules.LockTokens) {
		return ErrGachaCultivationLocked
	}
	tokens := rules.LockTokens[locked]
	if state.Gold < rules.Gold || state.Token < tokens || state.Mats["iv_solvent"] < rules.Solvent {
		return ErrGachaFunds
	}
	values := make([]int, len(rules.Stats))
	copy(values, own.IVValues)
	for index := range values {
		if own.IVLocks&(1<<index) == 0 {
			value, err := gachaRandom(rules.MaxValue + 1)
			if err != nil {
				return err
			}
			values[index] = value
		}
	}
	state.Mats["iv_solvent"] -= rules.Solvent
	state.Gold -= rules.Gold
	state.Token -= tokens
	own.IVValues = values
	result.Drops = append(result.Drops, fmt.Sprintf("个体值洗练 · 金币 −%d · 洗练剂 −%d · 代币 −%d", rules.Gold, rules.Solvent, tokens))
	return nil
}

func gachaArtifactBonuses(own *GachaOwnedGirl, gemCounts map[string]int) GachaStats {
	rules := &gachaCultivation.Artifacts
	var stats GachaStats
	add := stats.addCultivationStat
	applySet := func(set GachaArtifactSet, count int) {
		if set.Base != nil {
			add(set.Base.Stat, set.Base.Value*count)
		}
		for _, tier := range set.Tiers {
			if count >= tier.Count {
				add(tier.Stat, tier.Value)
			}
		}
	}
	for gem, count := range gemCounts {
		applySet(rules.GemSets[gem], count)
	}
	chipCounts := map[string]int{}
	for _, chip := range own.Chips {
		if family, _, ok := parseGachaChip(chip); ok {
			chipCounts[family]++
		}
	}
	for family, count := range chipCounts {
		applySet(rules.ChipSets[family], count)
	}
	for index, value := range own.IVValues {
		stat := gachaCultivation.Individual.Stats[index]
		add(stat.Stat, value/stat.Step)
	}
	return stats
}

type gachaFrozenRoster struct {
	*GachaGameState
	PowerVersion int `json:"arena_power_version"`
}

func freezeGachaRoster(state *GachaGameState) ([]byte, error) {
	roster := *state
	roster.Casebook, roster.Cipher, roster.Daily.Dispatch = nil, nil, nil
	roster.Dispatch = nil
	if state.Dispatch != nil && state.Dispatch.Active != "" {
		id := state.Dispatch.Active
		roster.Dispatch = &GachaDispatchState{Active: id, Relics: map[string]bool{id: state.Dispatch.Relics[id]}}
	}
	return common.Marshal(gachaFrozenRoster{&roster, gachaPowerVersion})
}

func thawGachaRoster(document GachaDocument) (*GachaGameState, error) {
	roster := gachaFrozenRoster{GachaGameState: &GachaGameState{}}
	if err := common.Unmarshal([]byte(document), &roster); err != nil {
		return nil, err
	}
	switch roster.PowerVersion {
	case 0:
		roster.legacyArenaStats = true
	case 5:
		roster.previousAtelier = true
	case 6:
		roster.previousMastery = true
	case 7:
		roster.previousTraining = true
	case gachaPowerVersion:
	default:
		return nil, ErrGachaRunState
	}
	return roster.GachaGameState, nil
}

type gachaCollection struct {
	flat         GachaStats
	bonds        map[string]GachaStats
	legacyWeapon bool
}

func (state *GachaGameState) currentAtelierRules() bool {
	return !state.legacyArenaStats && !state.previousAtelier
}

func (state *GachaGameState) masteryEnabled() bool {
	return state.currentAtelierRules() && !state.previousMastery
}

func (state *GachaGameState) advancedMasteryEnabled() bool {
	return state.masteryEnabled() && !state.previousTraining
}

func (state *GachaGameState) collection() gachaCollection {
	girls := state.Girls
	collection := gachaCollection{bonds: make(map[string]GachaStats), legacyWeapon: !state.masteryEnabled()}
	for id, weapon := range gachaCultivation.Weapons {
		if own := girls[id]; own == nil || own.Star < 1 {
			continue
		}
		amount := gachaCultivation.CodexFlat[weapon.Stat]
		switch weapon.Stat {
		case "atk":
			collection.flat.Atk += amount
		case "hp":
			collection.flat.HP += amount
		case "def":
			collection.flat.Def += amount
		}
	}
	bonds := gachaCultivation.Bonds
	if state.masteryEnabled() {
		bonds = gachaCultivation.Crews
	} else if state.currentAtelierRules() {
		bonds = gachaCultivation.Fellowships
	}
	for _, bond := range bonds {
		lowest := gachaCatalog.Growth.StarMax
		for _, id := range bond.Members {
			if girls[id] == nil {
				lowest = 0
				break
			}
			lowest = min(lowest, girls[id].Star)
		}
		if lowest < 1 {
			continue
		}
		bonus := gachaCultivation.BondBase
		if bond.Base > 0 {
			bonus = bond.Base
		}
		if lowest >= gachaCultivation.BondStar {
			bonus = gachaCultivation.BondAdvanced
			if bond.Advanced > 0 {
				bonus = bond.Advanced
			}
		}
		for _, id := range bond.Members {
			stats := collection.bonds[id]
			switch bond.Stat {
			case "hp":
				stats.HP = min(gachaCultivation.BondCap, stats.HP+bonus)
			case "atk":
				stats.Atk = min(gachaCultivation.BondCap, stats.Atk+bonus)
			case "def":
				stats.Def = min(gachaCultivation.BondCap, stats.Def+bonus)
			}
			collection.bonds[id] = stats
		}
	}
	return collection
}

func (collection gachaCollection) stats(id string, own *GachaOwnedGirl, hp, atk, def, spd float64) (float64, float64, float64, float64) {
	apply := func(stat string, value int) {
		multiplier := 1 + float64(value)/100
		switch stat {
		case "hp":
			hp *= multiplier
		case "atk":
			atk *= multiplier
		case "def":
			def *= multiplier
		case "spd":
			spd += float64(value)
		}
	}
	weaponLevel := own.WeaponLevel
	if collection.legacyWeapon {
		weaponLevel = min(weaponLevel, len(gachaCultivation.WeaponOre))
	}
	apply(gachaCultivation.Weapons[id].Stat, weaponLevel*gachaCultivation.WeaponPerLevel)
	apply("hp", own.WeaponAwake*gachaCultivation.WeaponVitalityPerAwake)
	bond := collection.bonds[id]
	apply("hp", bond.HP)
	apply("atk", bond.Atk)
	apply("def", bond.Def)
	potential := 1 + float64(own.Potential*gachaCultivation.PotentialPerNode)/100
	hp, atk, def = hp*potential, atk*potential, def*potential
	rune := gachaCultivation.Engravings[own.WeaponRune]
	apply(rune.Stat, rune.Value)
	return hp + float64(collection.flat.HP), atk + float64(collection.flat.Atk), def + float64(collection.flat.Def), spd
}

func applyGachaMilestones(unit *gachaCombatUnit, role string, star int) {
	for _, milestone := range gachaCultivation.Milestones[role] {
		if star < milestone.Star {
			continue
		}
		applyGachaCultivationEffect(unit, milestone.Effect, milestone.Value)
	}
}

func applyGachaCultivationEffect(unit *gachaCombatUnit, effect string, value int) {
	switch effect {
	case "rage":
		unit.Rage += value
	case "rage_gain":
		unit.rageBonus += value
	case "shield":
		unit.Shield += unit.MaxHP * value / 100
	case "skill":
		unit.skillBoost += float64(value) / 100
	case "heal":
		unit.healBoost += float64(value) / 100
	case "counter":
		unit.counter += float64(value) / 100
	case "leech":
		unit.lifeSteal += float64(value) / 100
	}
}

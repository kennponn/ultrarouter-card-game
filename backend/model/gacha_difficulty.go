package model

import (
	_ "embed"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

type GachaDifficultyTier struct {
	Name string `json:"name"`
	HP   int    `json:"hp"`
	Atk  int    `json:"atk"`
	Def  int    `json:"def"`
}

type GachaDifficultyEncounter struct {
	Element  string `json:"element,omitempty"`
	Mechanic string `json:"mechanic,omitempty"`
	Level    int    `json:"level,omitempty"`
}

type GachaDifficultyCatalog struct {
	Version    int                                 `json:"version"`
	Tiers      []GachaDifficultyTier               `json:"tiers"`
	Stages     map[string]int                      `json:"stages"`
	Encounters map[string]GachaDifficultyEncounter `json:"encounters,omitempty"`
}

type GachaIssuedDifficulty struct {
	Version     int    `json:"version"`
	Key         string `json:"key"`
	Tier        int    `json:"tier"`
	BossElement string `json:"boss_element,omitempty"`
	GachaDifficultyTier
	GachaDifficultyEncounter
}

//go:embed gacha_difficulty.json
var gachaDifficultyData []byte

var gachaDifficulty = func() GachaDifficultyCatalog {
	var catalog GachaDifficultyCatalog
	if err := common.Unmarshal(gachaDifficultyData, &catalog); err != nil {
		panic(err)
	}
	if err := catalog.validate(); err != nil {
		panic(err)
	}
	return catalog
}()

func (catalog GachaDifficultyCatalog) validate() error {
	if catalog.Version < 1 || len(catalog.Tiers) == 0 {
		return fmt.Errorf("missing dungeon difficulty edition or tiers")
	}
	for index, tier := range catalog.Tiers {
		if strings.TrimSpace(tier.Name) == "" || min(tier.HP, tier.Atk, tier.Def) <= 0 || max(tier.HP, tier.Atk, tier.Def) > 100_000 {
			return fmt.Errorf("invalid dungeon difficulty tier %d", index)
		}
	}
	expected := map[string]bool{"weekly_boss": true}
	for _, stage := range gachaCatalog.Stages {
		expected[stage.ID] = true
	}
	for key := range gachaVoyage.Stages {
		expected[key] = true
	}
	for prefix, count := range map[string]int{"training": 3, "awakening": 3, "dual_trial": 2, "expedition": gachaExpeditionFloors} {
		for index := 1; index <= count; index++ {
			expected[prefix+":"+strconv.Itoa(index)] = true
		}
	}
	if len(catalog.Stages) != len(expected) {
		return fmt.Errorf("dungeon difficulty defines %d stages; expected %d", len(catalog.Stages), len(expected))
	}
	for key := range expected {
		if index, exists := catalog.Stages[key]; !exists || index < 0 || index >= len(catalog.Tiers) {
			return fmt.Errorf("missing or invalid dungeon difficulty for %s", key)
		}
	}
	for key, encounter := range catalog.Encounters {
		if !expected[key] || encounter == (GachaDifficultyEncounter{}) ||
			encounter.Level != 0 && (encounter.Level < 1 || encounter.Level > 200 || key != "training:2" && key != "training:3") ||
			!slices.Contains([]string{"", "fire", "plant", "water", "star", "dark", "time"}, encounter.Element) ||
			!slices.Contains([]string{"", "fury", "regrowth", "barrier", "reflection", "venom", "silence"}, encounter.Mechanic) {
			return fmt.Errorf("invalid dungeon encounter %s", key)
		}
	}
	return nil
}

func gachaTrainingTier(cleared map[string]bool) int {
	if cleared["s6"] {
		return 3
	}
	if cleared["s3"] {
		return 2
	}
	return 1
}

func gachaDifficultyKey(stageID string, snapshot GachaRunSnapshot, cleared map[string]bool) string {
	switch snapshot.Mode {
	case "training":
		return "training:" + strconv.Itoa(gachaTrainingTier(cleared))
	case "dual_trial", "expedition":
		return snapshot.Mode + ":" + strconv.Itoa(snapshot.Floor)
	default:
		if snapshot.ProgressionGirl != "" {
			return "awakening:" + strconv.Itoa(snapshot.ProgressionRank)
		}
		return stageID
	}
}

func (catalog GachaDifficultyCatalog) issue(key string) *GachaIssuedDifficulty {
	index, ok := catalog.Stages[key]
	if !ok {
		return nil
	}
	return &GachaIssuedDifficulty{Version: catalog.Version, Key: key, Tier: index,
		GachaDifficultyTier: catalog.Tiers[index], GachaDifficultyEncounter: catalog.Encounters[key]}
}

func issueGachaCombatStage(stage GachaStage, snapshot *GachaRunSnapshot, cleared map[string]bool) (GachaStage, error) {
	issued := gachaDifficulty.issue(gachaDifficultyKey(stage.ID, *snapshot, cleared))
	if issued == nil {
		return GachaStage{}, ErrGachaStage
	}
	combat := gachaCombatStage(stage, issued)
	issued.Level, issued.Element, issued.Mechanic = combat.EnemyLv, combat.EnemyElem, combat.Mechanic
	if combat.HasBoss {
		issued.BossElement = combat.BossElem
		if combat.Waves == 1 && combat.BossElem != "" {
			issued.Element = combat.BossElem
		}
	}
	snapshot.Difficulty = issued
	return combat, nil
}

func gachaCombatStage(stage GachaStage, difficulty *GachaIssuedDifficulty) GachaStage {
	stage.difficulty = difficulty
	if difficulty != nil {
		if stage.EnemyElem == "" {
			stage.EnemyElem = difficulty.Element
		}
		if stage.Mechanic == "" {
			stage.Mechanic = difficulty.Mechanic
		}
		if difficulty.Level > 0 {
			stage.EnemyLv = difficulty.Level
		}
	}
	return stage
}

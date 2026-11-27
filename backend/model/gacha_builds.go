package model

import (
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	gachaSigilCapacity     = 160
	gachaRelicDailyLimit   = 30
	gachaBurstRunLimit     = 12
	gachaBurstRunWindow    = 10 * time.Minute
	gachaBurstRestDuration = 2 * time.Minute
)

var (
	ErrGachaSigil         = errors.New("sigil is unavailable or equipped")
	ErrGachaRest          = errors.New("rapid dungeon runs require a rest")
	gachaTalentIDs        = []string{"power", "guardian", "swift", "siphon", "blight", "oracle"}
	gachaSigilAffixIDs    = []string{"atk", "hp", "def", "spd", "cr", "cd", "ehr", "res"}
	gachaSigilDustReward  = [3]int{2, 4, 8}
	gachaSigilAffixBounds = map[string][3][2]int{
		"atk": {{2, 5}, {5, 8}, {8, 12}}, "hp": {{3, 8}, {8, 12}, {12, 18}},
		"def": {{3, 8}, {8, 14}, {14, 20}}, "spd": {{2, 4}, {4, 8}, {8, 12}},
		"cr": {{1, 3}, {3, 5}, {5, 8}}, "cd": {{4, 9}, {9, 16}, {16, 25}},
		"ehr": {{2, 5}, {5, 8}, {8, 12}}, "res": {{2, 5}, {5, 8}, {8, 12}},
	}
)

type GachaSigil struct {
	ID      string         `json:"id"`
	Family  string         `json:"family"`
	Quality int            `json:"quality"`
	Affixes map[string]int `json:"affixes"`
}

type GachaSigilFamily struct {
	Name    string
	Element string
	MatName string
	GemPool []string
}

var gachaSigilFamilies = map[string]GachaSigilFamily{
	"ember":  {Name: "\u70c8\u7130\u7075\u5370", Element: "fire", MatName: "\u708e\u5fc3\u7cbe\u534e", GemPool: []string{"ruby", "amethyst"}},
	"tide":   {Name: "\u6f6e\u6c50\u7075\u5370", Element: "water", MatName: "\u6f6e\u6c50\u7cbe\u534e", GemPool: []string{"jade", "sapphire"}},
	"grove":  {Name: "\u68ee\u606f\u7075\u5370", Element: "plant", MatName: "\u68ee\u606f\u7cbe\u534e", GemPool: []string{"jade", "sapphire"}},
	"astral": {Name: "\u661f\u8f89\u7075\u5370", Element: "star", MatName: "\u661f\u8f89\u7cbe\u534e", GemPool: []string{"amethyst", "ruby"}},
	"umbra":  {Name: "\u591c\u8680\u7075\u5370", Element: "dark", MatName: "\u591c\u8680\u7cbe\u534e", GemPool: []string{"amethyst", "sapphire"}},
	"chrono": {Name: "\u65f6\u5e8f\u7075\u5370", Element: "time", MatName: "\u65f6\u5e8f\u7cbe\u534e", GemPool: []string{"wind", "amethyst"}},
}

type GachaRunActivity struct {
	WindowStartedAt int64 `json:"window_started_at"`
	Starts          int   `json:"starts"`
	RestUntil       int64 `json:"rest_until"`
}

type GachaRestError struct{ Until int64 }

func (e *GachaRestError) Error() string        { return ErrGachaRest.Error() }
func (e *GachaRestError) Is(target error) bool { return target == ErrGachaRest }

func validGachaSigilID(id string) bool {
	if len(id) != 38 || !strings.HasPrefix(id, "sigil_") {
		return false
	}
	_, err := hex.DecodeString(id[6:])
	return err == nil
}

func validGachaSigil(sigil *GachaSigil) bool {
	if sigil == nil || !validGachaSigilID(sigil.ID) || sigil.Quality < 1 || sigil.Quality > 3 || len(sigil.Affixes) != 2 {
		return false
	}
	if _, ok := gachaSigilFamilies[sigil.Family]; !ok {
		return false
	}
	for key, value := range sigil.Affixes {
		bounds, ok := gachaSigilAffixBounds[key]
		if !ok || value < bounds[sigil.Quality-1][0] || value > bounds[sigil.Quality-1][1] {
			return false
		}
	}
	return true
}

func validateGachaBuildState(state *GachaGameState) error {
	if state.Sigils == nil {
		state.Sigils = map[string]*GachaSigil{}
	}
	if len(state.Sigils) > gachaSigilCapacity || state.Daily.RelicRuns < 0 || state.Daily.RelicRuns > gachaRelicDailyLimit ||
		state.Activity.Starts < 0 || state.Activity.Starts > gachaBurstRunLimit || state.Activity.WindowStartedAt < 0 || state.Activity.RestUntil < 0 {
		return ErrGachaRunState
	}
	for id, sigil := range state.Sigils {
		if !validGachaSigil(sigil) || id != sigil.ID {
			return ErrGachaRunState
		}
	}
	equipped := make(map[string]bool)
	for _, own := range state.Girls {
		if own == nil {
			return ErrGachaRunState
		}
		if own.Sigil != "" {
			if own.Star < 1 || state.Sigils[own.Sigil] == nil || equipped[own.Sigil] {
				return ErrGachaRunState
			}
			equipped[own.Sigil] = true
		}
	}
	for _, count := range state.Mats {
		if count < 0 || count > gachaMaxResource {
			return ErrGachaRunState
		}
	}
	if err := validateGachaTeams(state); err != nil {
		return err
	}
	return nil
}

// validateGachaTeams rejects tampered presets: at most four squads of four
// distinct owned girls each, and the active slot must match the live Squad.
func validateGachaTeams(state *GachaGameState) error {
	if len(state.Teams) != 4 {
		return ErrGachaRunState
	}
	if state.TeamOn < 1 || state.TeamOn > 4 {
		return ErrGachaRunState
	}
	seen := map[string]int{}
	for index, team := range state.Teams {
		if len(team) > 4 {
			return ErrGachaRunState
		}
		for i, id := range team {
			own := state.Girls[id]
			if own == nil || own.Star < 1 || own.Level < 1 || gachaGirl(id) == nil || slices.Contains(team[:i], id) {
				return ErrGachaRunState
			}
			// One girl, one preset: owning her twice across slots is a shared
			// roster, which the four-preset feature explicitly forbids.
			if _, ok := seen[id]; ok {
				return ErrGachaRunState
			}
			seen[id] = index
		}
	}
	if !slices.Equal(state.Teams[state.TeamOn-1], state.Squad) {
		return ErrGachaRunState
	}
	return nil
}

func createGachaSigil(family string, quality int) (*GachaSigil, error) {
	if _, ok := gachaSigilFamilies[family]; !ok || quality < 1 || quality > 3 {
		return nil, ErrGachaIntent
	}
	id, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}
	available := slices.Clone(gachaSigilAffixIDs)
	sigil := &GachaSigil{ID: "sigil_" + strings.ReplaceAll(id.String(), "-", ""), Family: family, Quality: quality, Affixes: map[string]int{}}
	for range 2 {
		index, err := gachaRandom(len(available))
		if err != nil {
			return nil, err
		}
		key := available[index]
		available = slices.Delete(available, index, index+1)
		value, err := gachaRandomRange(gachaSigilAffixBounds[key][quality-1])
		if err != nil {
			return nil, err
		}
		sigil.Affixes[key] = value
	}
	return sigil, nil
}

func gachaSigilName(sigil *GachaSigil) string {
	if !validGachaSigil(sigil) {
		return "\u7075\u5370"
	}
	quality := [3]string{"\u666e\u901a", "\u7a00\u6709", "\u53f2\u8bd7"}[sigil.Quality-1]
	return quality + "\u00b7" + gachaSigilFamilies[sigil.Family].Name
}

func applyGachaSigilAction(state *GachaGameState, intent GachaActionIntent, result *GachaActionResult) error {
	if state.Sigils == nil {
		state.Sigils = map[string]*GachaSigil{}
	}
	switch intent.Action {
	case "sigil_equip":
		own := state.Girls[*intent.GirlID]
		sigil := state.Sigils[*intent.ItemID]
		if own == nil || own.Star < 1 || !validGachaSigil(sigil) {
			return ErrGachaSigil
		}
		for id, equipped := range state.Girls {
			if id != *intent.GirlID && equipped.Sigil == sigil.ID {
				return ErrGachaSigil
			}
		}
		own.Sigil = sigil.ID
		result.Drops = append(result.Drops, gachaSigilName(sigil)+"\u5df2\u88c5\u5907")
	case "sigil_remove":
		own := state.Girls[*intent.GirlID]
		if own == nil || own.Star < 1 || own.Sigil == "" {
			return ErrGachaSigil
		}
		own.Sigil = ""
	case "sigil_salvage", "sigil_salvage_low":
		equipped := map[string]bool{}
		for _, own := range state.Girls {
			if own.Sigil != "" {
				equipped[own.Sigil] = true
			}
		}
		ids := []string{}
		if intent.Action == "sigil_salvage" {
			sigil := state.Sigils[*intent.ItemID]
			if !validGachaSigil(sigil) || equipped[sigil.ID] {
				return ErrGachaSigil
			}
			ids = append(ids, sigil.ID)
		} else {
			for id, sigil := range state.Sigils {
				if sigil.Quality == 1 && !equipped[id] {
					ids = append(ids, id)
				}
			}
			if len(ids) == 0 {
				return ErrGachaSigil
			}
			slices.Sort(ids)
		}
		dust := 0
		for _, id := range ids {
			dust += gachaSigilDustReward[state.Sigils[id].Quality-1]
		}
		if state.Mats["sigil_dust"] > gachaMaxResource-dust {
			return ErrGachaCapacity
		}
		for _, id := range ids {
			delete(state.Sigils, id)
		}
		state.Mats["sigil_dust"] += dust
		result.Drops = append(result.Drops, fmt.Sprintf("\u7075\u5370\u7c89\u5c18 x%d", dust))
	case "sigil_craft":
		family := *intent.ItemID
		mat := "essence_" + family
		if _, ok := gachaSigilFamilies[family]; !ok {
			return ErrGachaIntent
		}
		if len(state.Sigils) >= gachaSigilCapacity {
			return ErrGachaCapacity
		}
		if state.Gold < 800 || state.Mats[mat] < 40 || state.Mats["sigil_dust"] < 15 {
			return ErrGachaFunds
		}
		sigil, err := createGachaSigil(family, 3)
		if err != nil {
			return err
		}
		state.Gold -= 800
		state.Mats[mat] -= 40
		state.Mats["sigil_dust"] -= 15
		state.Sigils[sigil.ID] = sigil
		result.Sigil = sigil
		result.Drops = append(result.Drops, gachaSigilName(sigil)+" x1")
	default:
		return ErrGachaIntent
	}
	return nil
}

func checkGachaRunRest(state *GachaGameState, now time.Time) error {
	if state.Activity.RestUntil > now.UnixMilli() {
		return &GachaRestError{Until: state.Activity.RestUntil}
	}
	return nil
}

func trackGachaRunStart(state *GachaGameState, now time.Time, readyAt int64) {
	activity := &state.Activity
	if activity.WindowStartedAt == 0 || now.UnixMilli()-activity.WindowStartedAt >= gachaBurstRunWindow.Milliseconds() || activity.Starts >= gachaBurstRunLimit {
		activity.WindowStartedAt, activity.Starts, activity.RestUntil = now.UnixMilli(), 0, 0
	}
	activity.Starts++
	if activity.Starts == gachaBurstRunLimit {
		activity.RestUntil = readyAt + gachaBurstRestDuration.Milliseconds()
	}
}

func awardGachaSigilLoot(state *GachaGameState, stage *GachaStage, snapshot *GachaRunSnapshot, result *GachaFinishResult) error {
	if stage.RelicTier == 0 {
		return nil
	}
	if !snapshot.RelicEligible {
		result.Drops = append(result.Drops, "\u4eca\u65e5\u7075\u5370\u8865\u7ed9\u5df2\u7528\u5b8c\uff0c\u666e\u901a\u5956\u52b1\u4fdd\u7559")
		return nil
	}
	family, ok := gachaSigilFamilies[stage.LootFamily]
	if !ok {
		return ErrGachaStage
	}
	count, err := gachaRandomRange(stage.Essence)
	if err != nil {
		return err
	}
	mat := "essence_" + stage.LootFamily
	if state.Mats[mat] > gachaMaxResource-count {
		return ErrGachaCapacity
	}
	state.Mats[mat] += count
	result.Drops = append(result.Drops, fmt.Sprintf("%s x%d", family.MatName, count))
	drop, err := gachaChance(stage.RelicChance)
	if err != nil || !drop {
		return err
	}
	quality := 1
	if stage.RelicTier == 2 {
		quality = 2
	}
	promote, err := gachaChance(0.30)
	if err != nil {
		return err
	}
	if promote {
		quality++
	}
	if state.Sigils == nil {
		state.Sigils = map[string]*GachaSigil{}
	}
	if len(state.Sigils) >= gachaSigilCapacity {
		dust := gachaSigilDustReward[quality-1]
		if state.Mats["sigil_dust"] > gachaMaxResource-dust {
			return ErrGachaCapacity
		}
		state.Mats["sigil_dust"] += dust
		result.Drops = append(result.Drops, fmt.Sprintf("\u7075\u5370\u80cc\u5305\u5df2\u6ee1\uff0c\u81ea\u52a8\u8f6c\u4e3a\u7c89\u5c18 x%d", dust))
		return nil
	}
	sigil, err := createGachaSigil(stage.LootFamily, quality)
	if err != nil {
		return err
	}
	state.Sigils[sigil.ID] = sigil
	result.Drops = append(result.Drops, gachaSigilName(sigil)+" x1")
	return nil
}

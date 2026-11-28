package model

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
)

const gachaArtifactLootVersion = 4

var (
	ErrGachaArtifactFree      = errors.New("daily free draw already used or day changed")
	ErrGachaArtifactFreeFirst = errors.New("daily free single draw must be taken before a paid draw")
)

type GachaArtifactPayment struct {
	Resource string `json:"resource"`
	Amount   int    `json:"amount"`
}

var gachaArtifactActions = []string{"chip_equip", "chip_remove", "chip_lock", "gem_refine", "chip_refine", "gem_level", "chip_level", "gem_rank", "chip_rank", "draw_gems", "draw_chips", "convert_tickets", "salvage_dust"}
var gachaArtifactTicketNames = map[string]string{"gem_ticket": "宝石召唤券", "chip_ticket": "芯片召唤券"}

func newGachaGemSlots() []string { return make([]string, len(gachaCultivation.Artifacts.GemSockets)) }

func parseGachaChip(id string) (string, int, bool) {
	id, _, valid := splitGachaRefinement(id)
	if !valid {
		return "", 0, false
	}
	family, index, found := strings.Cut(id, "_")
	if !found || len(index) != 1 || index[0] < '1' || index[0] > '9' {
		return "", 0, false
	}
	slot := int(index[0] - '0')
	_, known := gachaCultivation.Artifacts.ChipSets[family]
	return family, slot, known && slot <= len(gachaCultivation.Artifacts.ChipStars)
}

func validGachaArtifactCard(own *GachaOwnedGirl) bool {
	rules := &gachaCultivation.Artifacts
	if own == nil || own.Sockets < 0 || own.Sockets > len(rules.GemSockets) || len(own.Gems) > len(rules.GemSockets) ||
		len(own.Chips) > len(rules.ChipStars) || own.ChipLocks < 0 || own.ChipLocks >= 1<<len(own.Chips) {
		return false
	}
	if own.Sockets > 0 && own.Star < rules.GemSockets[own.Sockets-1].Star {
		return false
	}
	for index, gem := range own.Gems {
		if gem != "" {
			if _, ok := gachaGem(gem); !ok || index >= own.Sockets {
				return false
			}
		}
	}
	for index, chip := range own.Chips {
		if chip == "" {
			if own.ChipLocks&(1<<index) != 0 {
				return false
			}
			continue
		}
		if _, slot, ok := parseGachaChip(chip); !ok || slot != index+1 || own.Star < rules.ChipStars[index] {
			return false
		}
	}
	return true
}

func normalizeGachaArtifacts(state *GachaGameState) error {
	for ticket, used := range state.Daily.ArtifactFree {
		if !used || gachaArtifactTicketNames[ticket] == "" {
			return ErrGachaRunState
		}
	}
	if state.GemInventory == nil {
		state.GemInventory = map[string]int{}
	}
	if state.ChipInventory == nil {
		state.ChipInventory = map[string]int{}
	}
	for id, count := range state.GemInventory {
		if _, ok := gachaGem(id); !ok || count < 0 || count > gachaMaxResource {
			return ErrGachaRunState
		}
	}
	for id, count := range state.ChipInventory {
		if _, _, ok := parseGachaChip(id); !ok || count < 0 || count > gachaMaxResource {
			return ErrGachaRunState
		}
	}
	for _, own := range state.Girls {
		if !validGachaArtifactCard(own) {
			return ErrGachaRunState
		}
		if missing := len(gachaCultivation.Artifacts.GemSockets) - len(own.Gems); missing > 0 {
			own.Gems = append(own.Gems, make([]string, missing)...)
		}
	}
	return nil
}

func validateGachaArtifactIntent(intent GachaActionIntent) error {
	drawing := intent.Action == "draw_gems" || intent.Action == "draw_chips"
	if intent.Squad != nil || intent.Config != nil || !drawing && intent.Choice != nil || intent.RoomID != nil || intent.StakeE != nil || intent.QueueID != nil || intent.OpponentID != nil {
		return ErrGachaIntent
	}
	switch intent.Action {
	case "gem_refine", "chip_refine", "gem_level", "chip_level", "gem_rank", "chip_rank":
		if intent.Enabled != nil {
			return ErrGachaIntent
		}
		return validateGachaRefinementIntent(intent)
	case "draw_gems", "draw_chips":
		if intent.GirlID != nil || intent.Enabled != nil || intent.Count == nil || (*intent.Count != 1 && *intent.Count != 10) {
			return ErrGachaIntent
		}
		if intent.Choice != nil && *intent.Choice == "free" {
			if *intent.Count != 1 || intent.ItemID == nil {
				return ErrGachaIntent
			}
			if _, err := gachaCalendarDay(*intent.ItemID); err != nil {
				return ErrGachaIntent
			}
		} else if intent.ItemID != nil || intent.Choice != nil && *intent.Choice != "ticket" && *intent.Choice != "token" {
			return ErrGachaIntent
		}
	case "convert_tickets":
		if intent.GirlID != nil || intent.Count != nil || intent.Enabled != nil || intent.ItemID == nil || gachaArtifactTicketNames[*intent.ItemID] == "" {
			return ErrGachaIntent
		}
	case "salvage_dust":
		if intent.GirlID != nil || intent.Count != nil || intent.Enabled != nil || intent.ItemID == nil || !slices.Contains([]string{"shard_dust", "shard_dust_r", "shard_dust_sr", "shard_dust_ssr"}, *intent.ItemID) {
			return ErrGachaIntent
		}
	case "chip_equip", "chip_remove", "chip_lock":
		if intent.GirlID == nil || gachaGirl(*intent.GirlID) == nil || intent.Count == nil || *intent.Count < 1 || *intent.Count > len(gachaCultivation.Artifacts.ChipStars) ||
			(intent.Action == "chip_lock") != (intent.Enabled != nil) || (intent.Action == "chip_equip") != (intent.ItemID != nil) {
			return ErrGachaIntent
		}
		if intent.ItemID != nil {
			if _, slot, ok := parseGachaChip(*intent.ItemID); !ok || slot != *intent.Count {
				return ErrGachaIntent
			}
		}
	default:
		return ErrGachaIntent
	}
	return nil
}

func applyGachaArtifactAction(state *GachaGameState, intent GachaActionIntent, result *GachaActionResult) error {
	switch intent.Action {
	case "gem_refine", "chip_refine", "gem_level", "chip_level", "gem_rank", "chip_rank":
		return advanceGachaArtifact(state, intent, result)
	case "salvage_dust":
		key, cost := *intent.ItemID, gachaCultivation.Shards.LegacyDustPerOre
		if state.Mats[key] < cost {
			return ErrGachaFunds
		}
		if state.Mats["forge_ore"] >= gachaMaxResource {
			return ErrGachaCapacity
		}
		state.Mats[key] -= cost
		state.Mats["forge_ore"]++
		result.Drops = append(result.Drops, "旧尘回收 · 星锻矿 ×1")
		return nil
	case "convert_tickets":
		ticket, ore := *intent.ItemID, gachaCultivation.Artifacts.TicketOre
		if state.Mats["forge_ore"] < ore {
			return ErrGachaFunds
		}
		if state.Mats[ticket] >= gachaMaxResource {
			return ErrGachaCapacity
		}
		state.Mats["forge_ore"] -= ore
		state.Mats[ticket]++
		result.Drops = append(result.Drops, gachaArtifactTicketNames[ticket]+" x1")
		return nil
	case "draw_gems", "draw_chips":
		return drawGachaArtifacts(state, intent, result)
	}
	own := state.Girls[*intent.GirlID]
	index := *intent.Count - 1
	if own == nil || own.Star < 1 {
		return ErrGachaIntent
	}
	if own.Star < gachaCultivation.Artifacts.ChipStars[index] {
		return ErrGachaCultivationLocked
	}
	var old string
	if index < len(own.Chips) {
		old = own.Chips[index]
	}
	bit := 1 << index
	if intent.Action == "chip_lock" {
		if old == "" {
			return ErrGachaCultivationLocked
		}
		if *intent.Enabled {
			own.ChipLocks |= bit
		} else {
			own.ChipLocks &^= bit
		}
		return nil
	}
	if own.ChipLocks&bit != 0 {
		return ErrGachaCultivationLocked
	}
	var next string
	if intent.Action == "chip_equip" {
		next = *intent.ItemID
		if next == old {
			return nil
		}
		if state.ChipInventory[next] < 1 {
			return ErrGachaFunds
		}
	} else if intent.Action != "chip_remove" || old == "" {
		return ErrGachaIntent
	}
	if old != "" && state.ChipInventory[old] >= gachaMaxResource {
		return ErrGachaCapacity
	}
	if missing := len(gachaCultivation.Artifacts.ChipStars) - len(own.Chips); missing > 0 {
		own.Chips = append(own.Chips, make([]string, missing)...)
	}
	if old != "" {
		state.ChipInventory[old]++
	}
	if next != "" {
		state.ChipInventory[next]--
	}
	own.Chips[index] = next
	return nil
}

func drawGachaArtifacts(state *GachaGameState, intent GachaActionIntent, result *GachaActionResult) error {
	if err := validateGachaArtifactIntent(intent); err != nil {
		return err
	}
	count := *intent.Count
	ticket, inventory := "gem_ticket", state.GemInventory
	var pool []string
	if intent.Action == "draw_chips" {
		ticket, inventory = "chip_ticket", state.ChipInventory
		for _, family := range slices.Sorted(maps.Keys(gachaCultivation.Artifacts.ChipSets)) {
			for index := range gachaCultivation.Artifacts.ChipStars {
				pool = append(pool, family+"_"+strconv.Itoa(index+1))
			}
		}
	} else {
		pool = slices.Sorted(maps.Keys(gachaGemSpecs))
	}
	payment := GachaArtifactPayment{Resource: ticket, Amount: count}
	freeUsed := state.Daily.ArtifactFree[ticket]
	switch {
	case intent.Choice != nil && *intent.Choice == "free":
		if *intent.ItemID != state.Daily.Date || freeUsed {
			return ErrGachaArtifactFree
		}
		payment.Resource, payment.Amount = "free", 0
	case !freeUsed:
		if intent.Choice != nil || count != 1 {
			return ErrGachaArtifactFreeFirst
		}
		payment.Resource, payment.Amount = "free", 0
	case intent.Choice != nil && *intent.Choice == "token":
		price := gachaCultivation.Artifacts.DrawToken
		if price < 1 || price > gachaMaxResource/count {
			return ErrGachaRunState
		}
		payment.Resource, payment.Amount = "token", price*count
		if state.Token < payment.Amount {
			return ErrGachaFunds
		}
	default:
		if state.Mats[ticket] < count {
			return ErrGachaFunds
		}
	}
	counts := map[string]int{}
	for range count {
		index, err := gachaRandom(len(pool))
		if err != nil {
			return err
		}
		roll, err := gachaRandom(10000)
		if err != nil {
			return err
		}
		item, _ := gachaArtifactTemplate(pool[index])
		item.Quality = gachaArtifactQualityAt(roll)
		id := item.id()
		counts[id]++
		if inventory[id] > gachaMaxResource-counts[id] {
			return ErrGachaCapacity
		}
	}
	switch payment.Resource {
	case "free":
		if state.Daily.ArtifactFree == nil {
			state.Daily.ArtifactFree = map[string]bool{}
		}
		state.Daily.ArtifactFree[ticket] = true
	case "token":
		state.Token -= payment.Amount
	default:
		state.Mats[ticket] -= payment.Amount
	}
	result.ArtifactPayment = &payment
	for _, id := range slices.Sorted(maps.Keys(counts)) {
		total := counts[id]
		inventory[id] += total
		item, _ := parseGachaArtifactKey(id)
		result.ArtifactLoot = append(result.ArtifactLoot, GachaArtifactLoot{ItemID: id, Quantity: total})
		result.Drops = append(result.Drops, fmt.Sprintf("%s x%d", item.name(), total))
	}
	return nil
}

func awardGachaArtifactTicket(state *GachaGameState, stage *GachaStage, result *GachaFinishResult) error {
	var ticket string
	if stage.RelicTier > 0 {
		ticket = "chip_ticket"
	} else if stage.HasBoss {
		ticket = "gem_ticket"
	} else {
		return nil
	}
	if state.Mats[ticket] >= gachaMaxResource {
		return ErrGachaCapacity
	}
	state.Mats[ticket]++
	result.Drops = append(result.Drops, gachaArtifactTicketNames[ticket]+" x1")
	return nil
}

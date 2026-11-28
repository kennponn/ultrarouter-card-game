package model

import (
	_ "embed"
	"fmt"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
)

type GachaShopItem struct {
	ID       string `json:"id"`
	Shop     string `json:"shop"`
	Resource string `json:"resource"`
	Amount   int    `json:"amount"`
	Price    int    `json:"price"`
	Limit    int    `json:"limit"`
}

type GachaMarketRules struct {
	RefreshPrice       int                     `json:"refresh_price"`
	DungeonCoins       []int                   `json:"dungeon_coins"`
	DungeonStamina     []int                   `json:"dungeon_stamina"`
	Guild              GachaMarketGuildRules   `json:"guild"`
	ExpeditionBase     int                     `json:"expedition_base"`
	ExpeditionPerFloor int                     `json:"expedition_per_floor"`
	ChallengeWin       int                     `json:"challenge_win"`
	ChallengeLoss      int                     `json:"challenge_loss"`
	LadderRewards      []GachaLadderRewardTier `json:"ladder_rewards"`
	Packs              map[string]int          `json:"packs"`
	Offers             []GachaShopItem         `json:"offers"`
}

type GachaMarketGuildRules struct {
	Checkin      int `json:"checkin"`
	Sortie       int `json:"sortie"`
	Training     int `json:"training"`
	Arena        int `json:"arena"`
	Contribution int `json:"contribution"`
	Raid         int `json:"raid"`
	RaidXP       int `json:"raid_xp"`
	RaidInk      int `json:"raid_ink"`
	RaidClear    int `json:"raid_clear_multiplier"`
	TokenPrice   int `json:"token_price"`
	TokenAmount  int `json:"token_amount"`
	TokenLimit   int `json:"token_limit"`
}

type GachaMarketStock struct {
	Slot   int64          `json:"slot"`
	Next   int64          `json:"next"`
	Bought map[string]int `json:"bought"`
}

//go:embed gacha_market.json
var gachaMarketJSON []byte

var gachaMarket = func() GachaMarketRules {
	var rules GachaMarketRules
	if err := common.Unmarshal(gachaMarketJSON, &rules); err != nil {
		panic(err)
	}
	if rules.RefreshPrice < 1 || rules.ChallengeLoss < 1 || rules.ChallengeWin < rules.ChallengeLoss || len(rules.DungeonCoins) != 6 || len(rules.DungeonStamina) != len(rules.DungeonCoins) {
		panic("invalid market currency rules")
	}
	for index, cost := range rules.DungeonStamina {
		if cost < 1 || cost > 120 || rules.DungeonCoins[index] < 1 || rules.DungeonCoins[index] > gachaMaxResource {
			panic("invalid resource dungeon economy")
		}
	}
	guild := rules.Guild
	if guild.RaidClear < 1 || guild.RaidClear > 10 {
		panic("invalid guild raid clear multiplier")
	}
	for _, amount := range []int{guild.Checkin, guild.Sortie, guild.Training, guild.Arena, guild.Contribution, guild.Raid, guild.RaidXP, guild.RaidInk, guild.TokenPrice, guild.TokenAmount, guild.TokenLimit} {
		if amount < 1 || amount > gachaMaxResource {
			panic("invalid guild economy")
		}
	}
	seen := make(map[string]bool, len(rules.Offers))
	for _, offer := range rules.Offers {
		currency := gachaShopCurrency(offer.Shop)
		if offer.ID == "" || seen[offer.ID] || currency == "" || offer.Resource == "" || offer.Resource == currency || offer.Amount < 1 || offer.Amount > gachaMaxResource || offer.Price < 1 || offer.Price > gachaMaxResource || offer.Limit < 1 {
			panic("invalid market offer")
		}
		seen[offer.ID] = true
	}
	return rules
}()

func gachaMarketWindow(now time.Time) (int64, int64) {
	local := now.In(gameZone)
	day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, gameZone)
	for _, hours := range [][2]int{{21, 33}, {14, 21}, {9, 14}, {-3, 9}} {
		start := day.Add(time.Duration(hours[0]) * time.Hour)
		if !local.Before(start) {
			return start.UnixMilli(), day.Add(time.Duration(hours[1]) * time.Hour).UnixMilli()
		}
	}
	panic("invalid game shop window")
}

func gachaShopItem(id string) *GachaShopItem {
	for index := range gachaMarket.Offers {
		if gachaMarket.Offers[index].ID == id {
			return &gachaMarket.Offers[index]
		}
	}
	return nil
}

func gachaShopCurrency(shop string) string {
	switch shop {
	case "token":
		return "token"
	case "arena", "dungeon", "expedition":
		return shop + "_coin"
	}
	return ""
}

func syncGachaMarket(state *GachaGameState, now time.Time) error {
	slot, next := gachaMarketWindow(now)
	if state.Market.Slot > slot || state.Market.Slot < 0 {
		return ErrGachaRunState
	}
	if state.Market.Slot != 0 {
		for id, used := range state.Market.Bought {
			item := gachaShopItem(id)
			if item == nil || used < 0 || used > item.Limit {
				return ErrGachaRunState
			}
		}
	}
	if state.Market.Slot != slot || state.Market.Bought == nil {
		stock := map[string]int{}
		if state.Market.Slot == 0 && state.Daily.Date == gachaDay(now) {
			for id, used := range state.Daily.ShopBought {
				if id == "stamina" {
					id = "stamina_supply"
				}
				item := gachaShopItem(id)
				if item == nil || item.Shop != "token" || used < 0 || used > 20 {
					return ErrGachaRunState
				}
				stock[id] = min(item.Limit, used)
			}
		}
		state.Market.Bought = stock
	}
	state.Daily.ShopBought = nil
	state.Market.Slot, state.Market.Next = slot, next
	return nil
}

func restoreMarketStamina(state *GachaGameState, amount, capacity int, now time.Time) error {
	if amount < 1 || state.Stamina < 0 || state.Stamina > capacity-amount {
		return ErrGachaCapacity
	}
	state.Stamina += amount
	if state.Stamina >= state.StaminaMax {
		state.StaminaTs = now.UnixMilli()
	}
	return nil
}

func buyGachaShopItem(state *GachaGameState, id string, now time.Time) error {
	item := gachaShopItem(id)
	if item == nil {
		return ErrGachaIntent
	}
	if state.Market.Bought[id] >= item.Limit {
		return ErrGachaShopLimit
	}
	currency := gachaShopCurrency(item.Shop)
	balance, ok := gachaMailResource(state, currency)
	if !ok || balance < item.Price {
		return ErrGachaFunds
	}
	if item.Resource == "stamina" {
		if err := restoreMarketStamina(state, item.Amount, state.StaminaMax, now); err != nil {
			return err
		}
	} else if _, carry := gachaCarry.Items[item.Resource]; carry {
		if err := grantGachaCarry(state, item.Resource, item.Amount); err != nil {
			return err
		}
	} else {
		value, known := gachaMailResource(state, item.Resource)
		if !known || value < 0 || value > gachaMaxResource-item.Amount {
			return ErrGachaCapacity
		}
		setGachaMailResource(state, item.Resource, value+item.Amount)
	}
	setGachaMailResource(state, currency, balance-item.Price)
	if state.Market.Bought == nil {
		state.Market.Bought = map[string]int{}
	}
	state.Market.Bought[id]++
	return nil
}

func validateGachaMarketIntent(intent GachaActionIntent) error {
	if intent.ItemID == nil || intent.Count != nil || intent.GirlID != nil || intent.Squad != nil || intent.Enabled != nil || intent.OpponentID != nil || intent.StakeE != nil || intent.QueueID != nil || intent.RoomID != nil || intent.Config != nil || intent.Materials != nil {
		return ErrGachaIntent
	}
	if intent.Action == "use_stamina_pack" && intent.Choice == nil && gachaMarket.Packs[*intent.ItemID] > 0 {
		return nil
	}
	if intent.Action == "shop_refresh" && (gachaShopCurrency(*intent.ItemID) != "" || *intent.ItemID == "guild") && intent.Choice != nil {
		slot, err := strconv.ParseInt(*intent.Choice, 10, 64)
		if err == nil && slot > 0 && strconv.FormatInt(slot, 10) == *intent.Choice {
			return nil
		}
	}
	return ErrGachaIntent
}

func applyGachaMarketIntent(state *GachaGameState, intent GachaActionIntent, result *GachaActionResult, now time.Time) error {
	id := *intent.ItemID
	if intent.Action == "use_stamina_pack" {
		amount := gachaMarket.Packs[id]
		if amount <= 0 || state.Mats[id] < 1 {
			return ErrGachaFunds
		}
		if err := restoreMarketStamina(state, amount, gachaMaxResource, now); err != nil {
			return err
		}
		state.Mats[id]--
		result.Drops = append(result.Drops, fmt.Sprintf("体力 +%d", amount))
		return nil
	}
	if *intent.Choice != strconv.FormatInt(state.Market.Slot, 10) {
		return ErrGachaShopLimit
	}
	if state.Token < gachaMarket.RefreshPrice {
		return ErrGachaFunds
	}
	used := false
	if id == "guild" {
		if state.Guild == nil {
			return ErrGachaGuild
		}
		progress := guildProgress(state, now)
		for _, count := range progress.Bought {
			used = used || count > 0
		}
		if used {
			clear(progress.Bought)
		}
	} else {
		for key, count := range state.Market.Bought {
			if item := gachaShopItem(key); item != nil && item.Shop == id && count > 0 {
				used = true
				delete(state.Market.Bought, key)
			}
		}
	}
	if !used {
		return ErrGachaShopLimit
	}
	state.Token -= gachaMarket.RefreshPrice
	result.Drops = append(result.Drops, fmt.Sprintf("本店个人库存已补满 · 代币 -%d", gachaMarket.RefreshPrice))
	return nil
}

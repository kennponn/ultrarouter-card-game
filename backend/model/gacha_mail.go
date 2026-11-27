package model

import (
	_ "embed"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

var (
	ErrGachaMailUnavailable = errors.New("mail attachment unavailable")
	ErrGachaMailboxFull     = errors.New("mailbox has too many unclaimed attachments")
	ErrGachaRedeemCode      = errors.New("gift code unavailable")
)

type GachaMailMessage struct {
	ID        string         `json:"id"`
	Title     string         `json:"title"`
	Body      string         `json:"body"`
	Sender    string         `json:"sender"`
	CreatedAt int64          `json:"created_at"`
	Resources map[string]int `json:"resources"`
	CaptainXP int            `json:"captain_xp,omitempty"`
	Read      bool           `json:"read"`
	Claimed   bool           `json:"claimed"`
}

type GachaMailbox struct {
	Messages []*GachaMailMessage `json:"messages"`
	Redeemed map[string]bool     `json:"redeemed"`
}

type gachaGiftCode struct {
	ID        string         `json:"id"`
	Code      string         `json:"code"`
	Name      string         `json:"name"`
	Starts    string         `json:"starts"`
	Ends      string         `json:"ends"`
	Resources map[string]int `json:"resources"`
}

//go:embed gacha_mail.json
var gachaGiftJSON []byte

var gachaGiftCodes = func() []gachaGiftCode {
	var codes []gachaGiftCode
	if err := common.Unmarshal(gachaGiftJSON, &codes); err != nil {
		panic(err)
	}
	return codes
}()

func syncGachaMailbox(state *GachaGameState) error {
	if state.Mailbox == nil {
		state.Mailbox = &GachaMailbox{Messages: []*GachaMailMessage{}, Redeemed: map[string]bool{}}
	}
	if state.Mailbox.Messages == nil || state.Mailbox.Redeemed == nil || len(state.Mailbox.Messages) > 400 {
		return ErrGachaRunState
	}
	seen := map[string]bool{}
	for _, message := range state.Mailbox.Messages {
		if message == nil || message.ID == "" || seen[message.ID] || message.CreatedAt <= 0 || message.CaptainXP < 0 || message.CaptainXP > gachaMaxResource {
			return ErrGachaRunState
		}
		seen[message.ID] = true
		for resource, amount := range message.Resources {
			if _, ok := gachaMailResource(state, resource); !ok || amount < 1 || amount > gachaMaxResource {
				return ErrGachaRunState
			}
		}
	}
	return nil
}

func gachaMailResource(state *GachaGameState, id string) (int, bool) {
	switch id {
	case "gold":
		return state.Gold, true
	case "stamina":
		return state.Stamina, true
	case "token":
		return state.Token, true
	case "multi":
		return state.Multi, true
	case "permits":
		if state.Progression != nil {
			return state.Progression.Permits, true
		}
		return 0, true
	case "marks":
		if state.Progression != nil {
			return state.Progression.Marks, true
		}
		return 0, true
	}
	if cardID, shard := strings.CutPrefix(id, "shard:"); shard {
		if gachaGirl(cardID) == nil {
			return 0, false
		}
		if own := state.Girls[cardID]; own != nil {
			return own.Shards, true
		}
		return 0, true
	}
	_, exists := gachaItems.Items[id]
	return state.Mats[id], exists
}

func setGachaMailResource(state *GachaGameState, id string, amount int) {
	switch id {
	case "gold":
		state.Gold = amount
	case "stamina":
		state.Stamina = amount
	case "token":
		state.Token = amount
	case "multi":
		state.Multi = amount
	case "permits":
		state.Progression.Permits = amount
	case "marks":
		state.Progression.Marks = amount
	default:
		if cardID, shard := strings.CutPrefix(id, "shard:"); shard {
			if state.Girls[cardID] == nil {
				state.Girls[cardID] = &GachaOwnedGirl{Gems: newGachaGemSlots()}
			}
			state.Girls[cardID].Shards = amount
		} else {
			state.Mats[id] = amount
		}
	}
}

func gachaMailResourceName(id string) string {
	if name := map[string]string{"gold": "金币", "stamina": "体力", "token": "代币", "multi": "双倍卡", "permits": "招募券", "marks": "契约印"}[id]; name != "" {
		return name
	}
	if cardID, shard := strings.CutPrefix(id, "shard:"); shard {
		return gachaGirl(cardID).Name + "碎片"
	}
	if item := gachaItems.Items[id]; item.Name != "" {
		return item.Name
	}
	return id
}

func sendGachaMail(state *GachaGameState, id, title, body string, resources map[string]int, captainXP int, now time.Time) error {
	if err := syncGachaMailbox(state); err != nil {
		return err
	}
	if captainXP < 0 || captainXP > gachaMaxResource {
		return ErrGachaRunState
	}
	for resource, amount := range resources {
		if _, ok := gachaMailResource(state, resource); !ok || amount < 1 || amount > gachaMaxResource {
			return ErrGachaRunState
		}
	}
	pending := 0
	for _, message := range state.Mailbox.Messages {
		if message.ID == id {
			return ErrGachaMailUnavailable
		}
		if !message.Claimed {
			pending++
		}
	}
	if pending >= 200 {
		if strings.HasPrefix(id, "daily:") || strings.HasPrefix(id, "battle:") || strings.HasPrefix(id, "ladder:") {
			return mergeGachaAutomaticMail(state, resources, captainXP, now)
		}
		return ErrGachaMailboxFull
	}
	if len(state.Mailbox.Messages) >= 400 {
		for index, message := range state.Mailbox.Messages {
			if message.Claimed {
				state.Mailbox.Messages = slices.Delete(state.Mailbox.Messages, index, index+1)
				break
			}
		}
	}
	state.Mailbox.Messages = append(state.Mailbox.Messages, &GachaMailMessage{ID: id, Title: title, Body: body, Sender: "星霜港航务署", CreatedAt: now.UnixMilli(), Resources: maps.Clone(resources), CaptainXP: captainXP})
	return nil
}

func mergeGachaAutomaticMail(state *GachaGameState, resources map[string]int, captainXP int, now time.Time) error {
	var summary *GachaMailMessage
	for _, message := range state.Mailbox.Messages {
		if message.ID == "system:overflow" && !message.Claimed {
			summary = message
			break
		}
	}
	if summary == nil {
		state.Mailbox.Messages = slices.DeleteFunc(state.Mailbox.Messages, func(message *GachaMailMessage) bool { return message.ID == "system:overflow" && message.Claimed })
		if len(state.Mailbox.Messages) >= 400 {
			for index, message := range state.Mailbox.Messages {
				if message.Claimed {
					state.Mailbox.Messages = slices.Delete(state.Mailbox.Messages, index, index+1)
					break
				}
			}
		}
		summary = &GachaMailMessage{ID: "system:overflow", Title: "系统与排名奖励汇总", Body: "待领邮件较多，后续自动发放的奖励合并保存在此。跨日仍可收取。", Sender: "星霜港航务署", CreatedAt: now.UnixMilli(), Resources: map[string]int{}}
		state.Mailbox.Messages = append(state.Mailbox.Messages, summary)
	}
	if summary.CaptainXP > gachaMaxResource-captainXP {
		return ErrGachaCapacity
	}
	for resource, amount := range resources {
		if summary.Resources[resource] > gachaMaxResource-amount {
			return ErrGachaCapacity
		}
	}
	for resource, amount := range resources {
		summary.Resources[resource] += amount
	}
	summary.CaptainXP += captainXP
	summary.Read = false
	return nil
}

func validateGachaMailIntent(intent GachaActionIntent) error {
	if intent.Count != nil || intent.GirlID != nil || intent.Squad != nil || intent.Enabled != nil || intent.OpponentID != nil || intent.StakeE != nil || intent.QueueID != nil || intent.RoomID != nil || intent.Config != nil || intent.Choice != nil || intent.Materials != nil {
		return ErrGachaIntent
	}
	if intent.Action == "mail_all" || intent.Action == "mail_delete_read" {
		if intent.ItemID == nil {
			return nil
		}
		return ErrGachaIntent
	}
	if intent.ItemID == nil || len(*intent.ItemID) < 1 || len(*intent.ItemID) > 100 {
		return ErrGachaIntent
	}
	if intent.Action == "mail_read" || intent.Action == "mail_claim" {
		return nil
	}
	if intent.Action == "redeem_code" && len(*intent.ItemID) <= 40 {
		return nil
	}
	return ErrGachaIntent
}

func applyGachaMail(state *GachaGameState, intent GachaActionIntent, receipt *GachaActionResult, now time.Time) error {
	if err := syncGachaMailbox(state); err != nil {
		return err
	}
	if intent.Action == "redeem_code" {
		code := strings.ToUpper(strings.TrimSpace(*intent.ItemID))
		for _, gift := range gachaGiftCodes {
			if gift.Code != code {
				continue
			}
			day := gachaDay(now)
			if day < gift.Starts || day > gift.Ends || state.Mailbox.Redeemed[gift.ID] {
				return ErrGachaRedeemCode
			}
			if err := grantGachaDirectResources(state, gift.Resources, receipt, now); err != nil {
				return err
			}
			state.Mailbox.Redeemed[gift.ID] = true
			receipt.Drops = append(receipt.Drops, "兑换成功，礼包已直接到账")
			return nil
		}
		return ErrGachaRedeemCode
	}
	if intent.Action == "mail_delete_read" {
		state.Mailbox.Messages = slices.DeleteFunc(state.Mailbox.Messages, func(message *GachaMailMessage) bool {
			return message.Read && (message.Claimed || len(message.Resources) == 0 && message.CaptainXP == 0)
		})
		return nil
	}
	var selected []*GachaMailMessage
	for _, message := range state.Mailbox.Messages {
		if intent.ItemID != nil && message.ID != *intent.ItemID {
			continue
		}
		if intent.Action == "mail_read" {
			message.Read = true
			return nil
		}
		if !message.Claimed && (len(message.Resources) > 0 || message.CaptainXP > 0) {
			selected = append(selected, message)
		}
	}
	if len(selected) == 0 {
		return ErrGachaMailUnavailable
	}
	resources := map[string]int{}
	xp := 0
	for _, message := range selected {
		if xp > gachaMaxResource-message.CaptainXP {
			return ErrGachaCapacity
		}
		xp += message.CaptainXP
		for resource, amount := range message.Resources {
			if resources[resource] > gachaMaxResource-amount {
				return ErrGachaCapacity
			}
			resources[resource] += amount
		}
	}
	if err := grantGachaDirectResources(state, resources, receipt, now); err != nil {
		return err
	}
	grantGachaCaptainXP(state, xp, &receipt.Drops)
	for _, message := range selected {
		message.Claimed, message.Read = true, true
	}
	return nil
}

// Validate the whole bundle before changing any balance. Used by explicit
// claims and gift redemption; old mail attachments keep the same semantics.
func grantGachaDirectResources(state *GachaGameState, resources map[string]int, receipt *GachaActionResult, now time.Time) error {
	for resource, amount := range resources {
		current, ok := gachaMailResource(state, resource)
		if !ok || amount < 1 || amount > gachaMaxResource || current < 0 || current > gachaMaxResource-amount {
			return ErrGachaCapacity
		}
	}
	keys := make([]string, 0, len(resources))
	for resource := range resources {
		keys = append(keys, resource)
	}
	sort.Strings(keys)
	for _, resource := range keys {
		current, _ := gachaMailResource(state, resource)
		setGachaMailResource(state, resource, current+resources[resource])
		receipt.Drops = append(receipt.Drops, fmt.Sprintf("%s +%d", gachaMailResourceName(resource), resources[resource]))
		if cardID, shard := strings.CutPrefix(resource, "shard:"); shard {
			girl, own := gachaGirl(cardID), state.Girls[cardID]
			rarity := gachaCatalog.Rarity[girl.Rarity]
			if own.Star == 0 && own.Shards >= rarity.ShardToUnlock {
				own.Shards -= rarity.ShardToUnlock
				own.Star, own.Level = rarity.BaseStar, 1
				receipt.Drops = append(receipt.Drops, "新少女解锁："+girl.Name)
			}
		}
	}
	if resources["stamina"] > 0 && state.Stamina >= state.StaminaMax {
		state.StaminaTs = now.UnixMilli()
	}
	return nil
}

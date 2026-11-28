package model

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const GachaLadderDailyLimit = 5
const GachaLadderBots = 100

var ErrGachaLadder = errors.New("ranked arena state changed or action unavailable")
var ErrGachaLadderLimit = errors.New("ranked arena daily limit or cooldown reached")

// All rank changes, defense changes and daily settlement share this SQL gate.
// Settling before every rank mutation preserves the actual 21:00 ordering even
// if the host worker was offline: no later rank can overwrite an unpaid day.
type GachaLadderGate struct {
	ID             int   `gorm:"primaryKey;autoIncrement:false"`
	Revision       int64 `gorm:"not null"`
	SettledThrough int64 `gorm:"not null"`
}

type GachaLadderEntry struct {
	ID             int               `gorm:"primaryKey;autoIncrement:false" json:"id"` // user ID; negative IDs are NPCs
	Rank           int               `gorm:"column:position;not null;uniqueIndex" json:"rank"`
	BestRank       int               `json:"best_rank"`
	BestRewardRank int               `gorm:"not null;default:0" json:"-"`
	Name           string            `gorm:"type:varchar(128);not null" json:"name"`
	Power          int               `json:"power"`
	Defense        GachaDocument     `gorm:"not null" json:"-"`
	JoinedAt       int64             `json:"joined_at"`
	Day            string            `gorm:"type:varchar(10)" json:"-"`
	Used           int               `json:"-"`
	CooldownUntil  int64             `json:"cooldown_until"`
	Squad          []GachaPublicGirl `gorm:"-" json:"squad"`
	GuildName      string            `gorm:"-" json:"guild_name"`
}

type GachaLadderMaterials struct {
	R   int `json:"mat_r"`
	SR  int `json:"mat_sr"`
	SSR int `json:"mat_ssr"`
}
type GachaLadderRewardTier struct {
	MaxRank   int                  `json:"max_rank"`
	Materials GachaLadderMaterials `json:"materials"`
	Token     int                  `json:"token"`
	ArenaCoin int                  `json:"arena_coin"`
	Tickets   int                  `json:"limited_ticket"`
}

// These are game tokens. Ranked rewards never touch the host wallet or E quota.
var gachaLadderRewards = gachaMarket.LadderRewards

type GachaLadderBestRewardTier struct {
	MaxRank     int   `json:"max_rank"`
	Token       int   `json:"token"`
	Claimed     bool  `json:"claimed"`
	CanClaim    bool  `json:"can_claim"`
	DeliveredAt int64 `json:"delivered_at"`
}

var gachaLadderBestRewards = []GachaLadderBestRewardTier{
	{MaxRank: 100, Token: 200}, {MaxRank: 50, Token: 400}, {MaxRank: 20, Token: 600},
	{MaxRank: 10, Token: 1000}, {MaxRank: 3, Token: 1500}, {MaxRank: 1, Token: 2500},
}

func gachaLadderBestTier(id string) (GachaLadderBestRewardTier, bool) {
	for _, tier := range gachaLadderBestRewards {
		if id == fmt.Sprintf("best:%d", tier.MaxRank) {
			return tier, true
		}
	}
	return GachaLadderBestRewardTier{}, false
}

type GachaLadderMail struct {
	UserID      int                  `gorm:"primaryKey;autoIncrement:false" json:"-"`
	Day         string               `gorm:"type:varchar(10);primaryKey" json:"day"`
	Rank        int                  `json:"rank"`
	Materials   GachaLadderMaterials `gorm:"embedded;embeddedPrefix:reward_" json:"materials"`
	Kind        string               `gorm:"type:varchar(16);not null;default:daily" json:"kind"`
	ArenaCoin   int                  `gorm:"not null;default:0" json:"arena_coin"`
	Tickets     int                  `gorm:"not null;default:0" json:"limited_ticket"`
	Token       int                  `gorm:"not null;default:0" json:"token"`
	SettledAt   int64                `json:"settled_at"`
	Claimed     bool                 `gorm:"not null;default:false" json:"claimed"`
	DeliveredAt int64                `gorm:"not null;default:0" json:"delivered_at"`
}

type GachaLadderBattle struct {
	ID             string        `gorm:"type:varchar(36);primaryKey" json:"id"`
	AttackerID     int           `gorm:"index" json:"attacker_id"`
	DefenderID     int           `gorm:"index" json:"defender_id"`
	AttackerName   string        `gorm:"type:varchar(128)" json:"attacker_name"`
	DefenderName   string        `gorm:"type:varchar(128)" json:"defender_name"`
	AttackerBefore int           `json:"attacker_before"`
	AttackerAfter  int           `json:"attacker_after"`
	DefenderBefore int           `json:"defender_before"`
	DefenderAfter  int           `json:"defender_after"`
	AttackWin      bool          `json:"attack_win"`
	PlayedAt       int64         `gorm:"index" json:"played_at"`
	Report         GachaDocument `gorm:"not null" json:"-"`
}

type GachaLadderResult struct {
	GachaLadderBattle
	ArenaCoin int                `json:"arena_coin,omitempty"`
	Ladder    bool               `json:"ladder"`
	Name      string             `json:"name"`
	Win       bool               `json:"win"`
	OwnBefore int                `json:"own_before"`
	OwnAfter  int                `json:"own_after"`
	Report    *GachaBattleReport `json:"report"`
}

type GachaLadderView struct {
	Me             *GachaLadderEntry           `json:"me"`
	Remaining      int                         `json:"remaining"`
	Limit          int                         `json:"limit"`
	Total          int64                       `json:"total"`
	NextSettlement int64                       `json:"next_settlement"`
	NextReset      int64                       `json:"next_reset"`
	ServerNow      int64                       `json:"server_now"`
	Opponents      []GachaLadderEntry          `json:"opponents"`
	Board          []GachaLadderEntry          `json:"board"`
	History        []GachaLadderBattle         `json:"history"`
	Mail           []GachaLadderMail           `json:"mail"`
	Rewards        []GachaLadderRewardTier     `json:"rewards"`
	BestRank       int                         `json:"best_rank"`
	BestRewards    []GachaLadderBestRewardTier `json:"best_rewards"`
}

func gachaLadderBoundary(now time.Time, hour int) time.Time {
	local := now.In(gameZone)
	boundary := time.Date(local.Year(), local.Month(), local.Day(), hour, 0, 0, 0, gameZone)
	if now.Before(boundary) {
		boundary = boundary.AddDate(0, 0, -1)
	}
	return boundary
}
func GachaLadderReward(rank int) GachaLadderMaterials {
	return gachaLadderRewardTier(rank).Materials
}
func gachaLadderRewardTier(rank int) GachaLadderRewardTier {
	for _, tier := range gachaLadderRewards {
		if rank > 0 && (tier.MaxRank == 0 || rank <= tier.MaxRank) {
			return tier
		}
	}
	return GachaLadderRewardTier{}
}

func gachaLadderBestRank(best, rank int) int {
	if best <= 0 {
		return rank
	}
	return min(best, rank)
}

// Only a ladder-gate owner creates outbox rows. Read before inserting so an
// existing row is never write-locked by ON CONFLICT while a profile transaction
// holds that player's lock and is updating the delivery marker.
func insertGachaLadderMail(tx *gorm.DB, mail *GachaLadderMail) error {
	var count int64
	if err := tx.Model(&GachaLadderMail{}).Where("user_id = ? AND day = ?", mail.UserID, mail.Day).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	return tx.Create(mail).Error
}

// Gate owners only: historical rows record manual-claim entitlements, not mail.
// Already delivered legacy rows keep their marker and remain in the old inbox.
// The progress marker avoids rescanning every previously reached tier forever;
// the stable primary keys remain the final duplicate barrier during upgrades.
func queueGachaLadderBestRewards(tx *gorm.DB, now time.Time) error {
	var entries []GachaLadderEntry
	if err := tx.Select("id", "best_rank").Where("id > 0 AND best_rank > 0 AND (best_reward_rank = 0 OR best_rank < best_reward_rank)").Find(&entries).Error; err != nil {
		return err
	}
	for _, entry := range entries {
		for _, tier := range gachaLadderBestRewards {
			if entry.BestRank > tier.MaxRank {
				continue
			}
			mail := GachaLadderMail{UserID: entry.ID, Day: fmt.Sprintf("best:%d", tier.MaxRank), Kind: "best", Rank: tier.MaxRank, Token: tier.Token, SettledAt: now.UnixMilli()}
			if err := insertGachaLadderMail(tx, &mail); err != nil {
				return err
			}
		}
		if err := tx.Model(&GachaLadderEntry{}).Where("id = ?", entry.ID).Update("best_reward_rank", entry.BestRank).Error; err != nil {
			return err
		}
	}
	return nil
}

func gachaLadderMailID(mail GachaLadderMail) string {
	if mail.Kind == "best" {
		return "ladder:" + mail.Day
	}
	return "ladder:daily:" + mail.Day
}

// Called only after the caller locks this player. It never reacquires the ladder
// gate or loads another player, so ordinary profile/mail actions cannot invert
// the gate -> player lock order. Mailbox append and delivery marker commit in
// the same outer transaction; deleting a claimed public message cannot reissue it.
func syncGachaLadderMailbox(tx *gorm.DB, userID int, state *GachaGameState, now time.Time) (bool, error) {
	var pending []GachaLadderMail
	// The existing player lock serializes delivery for this user. A plain read
	// also avoids MySQL next-key locks blocking newly queued reward inserts.
	if err := tx.Where("user_id = ? AND claimed = ? AND delivered_at = ? AND kind <> ? AND day NOT LIKE ?", userID, false, 0, "best", "best:%").Order("settled_at ASC, day ASC").Find(&pending).Error; err != nil {
		return false, err
	}
	changed := false
	for _, mail := range pending {
		// A ladder request may have established a repeatable-read snapshot before
		// waiting for the player lock. The conditional current write prevents a
		// stale delivered_at=0 read from reissuing a since-claimed/deleted letter.
		reserved := tx.Model(&GachaLadderMail{}).Where("user_id = ? AND day = ? AND claimed = ? AND delivered_at = ?", userID, mail.Day, false, 0).Update("delivered_at", now.UnixMilli())
		if reserved.Error != nil {
			return false, reserved.Error
		}
		if reserved.RowsAffected == 0 {
			continue
		}
		id := gachaLadderMailID(mail)
		exists := slices.ContainsFunc(state.Mailbox.Messages, func(message *GachaMailMessage) bool { return message.ID == id })
		if !exists {
			resources := map[string]int{}
			for key, amount := range map[string]int{"token": mail.Token, "mat_r": mail.Materials.R, "mat_sr": mail.Materials.SR, "mat_ssr": mail.Materials.SSR, "arena_coin": mail.ArenaCoin, "limited_ticket": mail.Tickets} {
				if amount < 0 || amount > gachaMaxResource {
					return false, ErrGachaRunState
				}
				if amount > 0 {
					resources[key] = amount
				}
			}
			title := "竞技场每日排名奖励"
			body := fmt.Sprintf("%s 北京时间21:00结算，排名第%d名。奖励已寄达，请领取附件。", mail.Day, mail.Rank)
			if err := sendGachaMail(state, id, title, body, resources, 0, time.UnixMilli(mail.SettledAt)); err != nil {
				if errors.Is(err, ErrGachaMailboxFull) || errors.Is(err, ErrGachaCapacity) {
					// A full mailbox must not prevent login. Keep this durable outbox
					// row for the next player load after attachments are collected.
					if reset := tx.Model(&GachaLadderMail{}).Where("user_id = ? AND day = ?", userID, mail.Day).Update("delivered_at", 0).Error; reset != nil {
						return false, reset
					}
					return changed, nil
				}
				return false, err
			}
		}
		changed = true
	}
	return changed, nil
}

// Safe to call again after fresh or upgrade migrations. It never resets ranks.
func InitializeGachaLadder(now time.Time) error {
	return gachaTransaction(func(tx *gorm.DB) error {
		gate := GachaLadderGate{ID: 1, SettledThrough: gachaLadderBoundary(now, 21).UnixMilli()}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&gate).Error; err != nil {
			return err
		}
		if err := lockGachaLadder(tx); err != nil {
			return err
		}
		for rank := 1; rank <= GachaLadderBots; rank++ {
			var count int64
			if err := tx.Model(&GachaLadderEntry{}).Where("id = ?", -rank).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				continue
			}
			state := gachaLadderBotState(now, rank)
			entry := GachaLadderEntry{ID: -rank, Rank: rank, BestRank: rank, Name: fmt.Sprintf("星霜守卫 %03d", rank), JoinedAt: now.UnixMilli()}
			if err := gachaLadderDefense(&entry, state); err != nil {
				return err
			}
			if err := tx.Create(&entry).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// New guards fill all six positions. Existing defenses are never rewritten by
// initialization, so upgrades preserve old four-card guards and player rosters.
func gachaLadderBotState(now time.Time, rank int) *GachaGameState {
	state := newGachaState(now)
	state.Squad = append(state.Squad, "c02", "c05")
	level := 1 + (GachaLadderBots-rank)*59/GachaLadderBots
	for _, id := range state.Squad {
		if state.Girls[id] == nil {
			state.Girls[id] = &GachaOwnedGirl{Gems: newGachaGemSlots()}
		}
		state.Girls[id].Level, state.Girls[id].Star = level, 1+(level-1)/15
	}
	return state
}

func lockGachaLadder(tx *gorm.DB) error {
	r := tx.Model(&GachaLadderGate{}).Where("id = ?", 1).Update("revision", gorm.Expr("revision + 1"))
	if r.Error != nil {
		return r.Error
	}
	if r.RowsAffected != 1 {
		return ErrGachaLadder
	}
	return nil
}

func settleGachaLadder(tx *gorm.DB, now time.Time) error {
	var inactive []int
	enabled := tx.Model(&User{}).Select("id").Where("status = ?", common.UserStatusEnabled)
	if err := tx.Model(&GachaLadderEntry{}).Where("position > 0 AND id > 0 AND id NOT IN (?)", enabled).Order("position ASC").Pluck("id", &inactive).Error; err != nil {
		return err
	}
	if err := removeGachaLadderSeats(tx, inactive); err != nil {
		return err
	}
	if err := queueGachaLadderBestRewards(tx, now); err != nil {
		return err
	}
	var gate GachaLadderGate
	if err := tx.First(&gate, "id = ?", 1).Error; err != nil {
		return err
	}
	due := gachaLadderBoundary(now, 21)
	if gate.SettledThrough >= due.UnixMilli() {
		return nil
	}
	var players []GachaLadderEntry
	if err := tx.Select("id", "position", "joined_at").Where("id > 0 AND position > 0").Find(&players).Error; err != nil {
		return err
	}
	for at := time.UnixMilli(gate.SettledThrough).In(gameZone).AddDate(0, 0, 1); !at.After(due); at = at.AddDate(0, 0, 1) {
		for _, player := range players {
			if player.JoinedAt > at.UnixMilli() {
				continue
			}
			reward := gachaLadderRewardTier(player.Rank)
			mail := GachaLadderMail{UserID: player.ID, Day: at.Format("2006-01-02"), Kind: "daily", Rank: player.Rank,
				Materials: reward.Materials, Token: reward.Token, ArenaCoin: reward.ArenaCoin, Tickets: reward.Tickets, SettledAt: at.UnixMilli()}
			if err := insertGachaLadderMail(tx, &mail); err != nil {
				return err
			}
		}
	}
	return tx.Model(&gate).Update("settled_through", due.UnixMilli()).Error
}

func removeGachaLadderSeats(tx *gorm.DB, ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	for _, id := range ids {
		if err := tx.Model(&GachaLadderEntry{}).Where("id = ?", id).Updates(map[string]any{"position": -id, "defense": GachaDocument("{}"), "power": 0, "name": ""}).Error; err != nil {
			return err
		}
	}
	var active []GachaLadderEntry
	if err := tx.Select("id", "position", "best_rank").Where("position > 0").Order("position ASC").Find(&active).Error; err != nil {
		return err
	}
	for offset, entry := range active {
		rank := offset + 1
		if entry.Rank != rank {
			if err := tx.Model(&entry).Updates(map[string]any{"position": rank, "best_rank": gachaLadderBestRank(entry.BestRank, rank)}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// Call from a host-owned background job (e.g. every minute). Multiple workers
// are safe. Request-time catch-up is a second line of defense, not a client clock.
func SettleGachaLadder(ctx context.Context, now time.Time) error {
	started := time.Now()
	return gachaTransaction(func(tx *gorm.DB) error {
		tx = tx.WithContext(ctx)
		if err := lockGachaLadder(tx); err != nil {
			return err
		}
		return settleGachaLadder(tx, now.Add(time.Since(started)))
	})
}

func gachaLadderDefense(entry *GachaLadderEntry, state *GachaGameState) error {
	power, err := GachaSquadPower(state)
	if err != nil || power <= 0 {
		return ErrGachaTeam
	}
	data, err := common.Marshal(state)
	if err != nil {
		return err
	}
	entry.Defense, entry.Power = GachaDocument(data), power
	return nil
}
func gachaLadderPublic(entry *GachaLadderEntry) error {
	var state GachaGameState
	if err := common.Unmarshal([]byte(entry.Defense), &state); err != nil {
		return err
	}
	entry.Squad = []GachaPublicGirl{}
	for _, id := range state.Squad {
		if girl := state.Girls[id]; girl != nil {
			entry.Squad = append(entry.Squad, GachaPublicGirl{ID: id, Level: girl.Level, Star: girl.Star})
		}
	}
	return nil
}
func gachaLadderIdentity(tx *gorm.DB, userID int) (string, error) {
	if userID <= 0 {
		return "", ErrGachaIntent
	}
	var user User
	if err := tx.Select("id", "status", "username", "display_name").First(&user, userID).Error; err != nil {
		return "", err
	}
	if user.Status != common.UserStatusEnabled {
		return "", ErrGachaIntent
	}
	if user.DisplayName != "" {
		return user.DisplayName, nil
	}
	return user.Username, nil
}

func GetGachaLadder(userID int, now time.Time) (*GachaLadderView, error) {
	view := &GachaLadderView{}
	started := time.Now()
	err := gachaTransaction(func(tx *gorm.DB) error {
		if err := lockGachaLadder(tx); err != nil {
			return err
		}
		// Include time spent waiting for the gate, including transaction retries.
		// A request queued before 21:00 must not mutate yesterday's ordering after
		// acquiring the gate on the other side of the settlement boundary.
		now := now.Add(time.Since(started))
		if _, err := gachaLadderIdentity(tx, userID); err != nil {
			return err
		}
		if err := settleGachaLadder(tx, now); err != nil {
			return err
		}
		*view = GachaLadderView{Limit: GachaLadderDailyLimit, Remaining: GachaLadderDailyLimit,
			ServerNow: now.UnixMilli(), NextSettlement: gachaLadderBoundary(now, 21).AddDate(0, 0, 1).UnixMilli(),
			NextReset: gachaLadderBoundary(now, 5).AddDate(0, 0, 1).UnixMilli(), Rewards: slices.Clone(gachaLadderRewards), BestRewards: slices.Clone(gachaLadderBestRewards),
			Opponents: []GachaLadderEntry{}, Board: []GachaLadderEntry{}, History: []GachaLadderBattle{}, Mail: []GachaLadderMail{}}
		if err := tx.Model(&GachaLadderEntry{}).Where("position > 0").Count(&view.Total).Error; err != nil {
			return err
		}
		if err := tx.Where("position > 0").Order("position ASC").Limit(50).Find(&view.Board).Error; err != nil {
			return err
		}
		var me GachaLadderEntry
		err := tx.First(&me, "id = ?", userID).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			view.BestRank = me.BestRank
		}
		if err == nil && me.Rank > 0 {
			view.Me = &me
			if me.Day == gachaLadderBoundary(now, 5).Format("2006-01-02") {
				view.Remaining = max(0, GachaLadderDailyLimit-me.Used)
			}
			if err := gachaLadderPublic(&me); err != nil {
				return err
			}
			// Server-chosen rank bands; refresh samples one entry per band.
			low := max(1, me.Rank*40/100)
			high := min(int(view.Total), max(6, me.Rank+2))
			count := high - low
			bands := min(5, count)
			positions := make([]int, 0, bands)
			for band := 0; band < bands; band++ {
				start, end := band*count/bands, (band+1)*count/bands
				pick, err := gachaRandom(end - start)
				if err != nil {
					return err
				}
				position := low + start + pick
				if position >= me.Rank {
					position++
				}
				positions = append(positions, position)
			}
			if len(positions) > 0 {
				if err := tx.Where("position IN ?", positions).Order("position ASC").Find(&view.Opponents).Error; err != nil {
					return err
				}
			}
		}
		for _, entries := range [][]GachaLadderEntry{view.Board, view.Opponents} {
			for i := range entries {
				if err := gachaLadderPublic(&entries[i]); err != nil {
					return err
				}
			}
		}
		ids := []int{userID}
		for _, entries := range [][]GachaLadderEntry{view.Board, view.Opponents} {
			for _, entry := range entries {
				if entry.ID > 0 {
					ids = append(ids, entry.ID)
				}
			}
		}
		guilds, err := gachaGuildNames(tx, ids)
		if err != nil {
			return err
		}
		if view.Me != nil {
			view.Me.GuildName = guilds[userID]
		}
		for _, entries := range [][]GachaLadderEntry{view.Board, view.Opponents} {
			for i := range entries {
				entries[i].GuildName = guilds[entries[i].ID]
			}
		}
		if err := tx.Select("id", "attacker_id", "defender_id", "attacker_name", "defender_name", "attacker_before", "attacker_after", "defender_before", "defender_after", "attack_win", "played_at").Where("attacker_id = ? OR defender_id = ?", userID, userID).Order("played_at DESC, id DESC").Limit(20).Find(&view.History).Error; err != nil {
			return err
		}
		if _, _, err := loadGachaPlayer(tx, userID, now); err != nil {
			return err
		}
		var bestRewards []GachaLadderMail
		if err := tx.Where("user_id = ? AND kind = ?", userID, "best").Find(&bestRewards).Error; err != nil {
			return err
		}
		for i := range view.BestRewards {
			tier := &view.BestRewards[i]
			for _, reward := range bestRewards {
				if reward.Day != fmt.Sprintf("best:%d", tier.MaxRank) {
					continue
				}
				tier.Claimed, tier.DeliveredAt = reward.Claimed, reward.DeliveredAt
				tier.CanClaim = view.BestRank > 0 && view.BestRank <= tier.MaxRank && !reward.Claimed && reward.DeliveredAt == 0
				break
			}
		}
		return tx.Where("user_id = ? AND kind <> ?", userID, "best").Order("day DESC").Limit(60).Find(&view.Mail).Error
	})
	return view, err
}

func validateGachaLadderIntent(intent GachaActionIntent) error {
	if intent.GirlID != nil || intent.Squad != nil || intent.Enabled != nil || intent.StakeE != nil || intent.QueueID != nil || intent.RoomID != nil || intent.Choice != nil || intent.Config != nil {
		return ErrGachaIntent
	}
	switch intent.Action {
	case "ladder_join", "ladder_defense", "ladder_leave":
		if intent.OpponentID != nil || intent.Count != nil || intent.ItemID != nil {
			return ErrGachaIntent
		}
	case "ladder_challenge":
		if intent.OpponentID == nil || *intent.OpponentID == 0 || intent.Count == nil || *intent.Count < 1 || intent.ItemID != nil {
			return ErrGachaIntent
		}
	case "ladder_claim":
		if intent.OpponentID != nil || intent.Count != nil || intent.ItemID == nil {
			return ErrGachaIntent
		}
		if day, err := time.Parse("2006-01-02", *intent.ItemID); err != nil || day.Format("2006-01-02") != *intent.ItemID {
			return ErrGachaIntent
		}
	case "ladder_best_claim":
		if intent.OpponentID != nil || intent.Count != nil || intent.ItemID == nil {
			return ErrGachaIntent
		}
		if _, ok := gachaLadderBestTier(*intent.ItemID); !ok {
			return ErrGachaIntent
		}
	default:
		return ErrGachaIntent
	}
	return nil
}

func applyGachaLadderAction(userID int, intent GachaActionIntent, now time.Time) (*GachaActionResult, error) {
	intent.RequestID = strings.ToLower(intent.RequestID)
	hash, err := gachaIntentHash(intent)
	if err != nil {
		return nil, err
	}
	result := &GachaActionResult{}
	started := time.Now()
	err = gachaTransaction(func(tx *gorm.DB) error {
		if err := lockGachaLadder(tx); err != nil {
			return err
		}
		now := now.Add(time.Since(started))
		name, err := gachaLadderIdentity(tx, userID)
		if err != nil {
			return err
		}
		if err := settleGachaLadder(tx, now); err != nil {
			return err
		}
		player, state, err := loadGachaPlayer(tx, userID, now)
		if err != nil {
			return err
		}
		if replay, err := gachaReplayResult(tx, userID, intent.RequestID, hash, result); replay || err != nil {
			return err
		}
		*result = GachaActionResult{}
		var me GachaLadderEntry
		err = tx.First(&me, "id = ?", userID).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		active := err == nil && me.Rank > 0
		switch intent.Action {
		case "ladder_join":
			if active {
				return ErrGachaLadder
			}
			var last GachaLadderEntry
			if err := tx.Select("position").Order("position DESC").First(&last).Error; err != nil {
				return err
			}
			me.ID, me.Rank, me.Name, me.JoinedAt = userID, last.Rank+1, name, now.UnixMilli()
			me.BestRank = gachaLadderBestRank(me.BestRank, me.Rank)
			if err := gachaLadderDefense(&me, state); err != nil {
				return err
			}
			if err := tx.Save(&me).Error; err != nil {
				return err
			}
		case "ladder_leave":
			if !active {
				return ErrGachaLadder
			}
			if err := removeGachaLadderSeats(tx, []int{userID}); err != nil {
				return err
			}
		case "ladder_defense":
			if !active {
				return ErrGachaLadder
			}
			me.Name = name
			if err := gachaLadderDefense(&me, state); err != nil {
				return err
			}
			if err := tx.Save(&me).Error; err != nil {
				return err
			}
		case "ladder_challenge":
			if !active {
				return ErrGachaLadder
			}
			me.Name = name
			result.Ladder, err = challengeGachaLadder(tx, &me, state, *intent.OpponentID, *intent.Count, now)
			if err != nil {
				return err
			}
			coins := gachaMarket.ChallengeLoss
			if result.Ladder.Win {
				coins = gachaMarket.ChallengeWin
			}
			if state.Mats["arena_coin"] > gachaMaxResource-coins {
				return ErrGachaCapacity
			}
			state.Mats["arena_coin"] += coins
			result.Ladder.ArenaCoin = coins
			if err := recordGuildPersonalActivity(state, "arena", now); err != nil {
				return err
			}
			advanceGachaCaptainTask(state, "ladder")
		case "ladder_claim":
			// Cached clients may still send the old action. It only ensures mailbox
			// delivery; rewards are claimed exclusively through the public inbox.
			var mail GachaLadderMail
			if err := tx.First(&mail, "user_id = ? AND day = ?", userID, *intent.ItemID).Error; err != nil || mail.Claimed {
				return ErrGachaLadder
			}
		case "ladder_best_claim":
			tier, ok := gachaLadderBestTier(*intent.ItemID)
			if !ok || me.BestRank <= 0 || me.BestRank > tier.MaxRank {
				return ErrGachaLadder
			}
			var reward GachaLadderMail
			if err := tx.First(&reward, "user_id = ? AND day = ? AND kind = ?", userID, *intent.ItemID, "best").Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrGachaLadder
				}
				return err
			}
			if reward.Claimed || reward.DeliveredAt != 0 {
				return ErrGachaLadder
			}
			if state.Token > gachaMaxResource-tier.Token {
				return ErrGachaCapacity
			}
			claimed := tx.Model(&GachaLadderMail{}).Where("user_id = ? AND day = ? AND claimed = ? AND delivered_at = ?", userID, reward.Day, false, 0).Update("claimed", true)
			if claimed.Error != nil {
				return claimed.Error
			}
			if claimed.RowsAffected != 1 {
				return ErrGachaLadder
			}
			state.Token += tier.Token
			result.Drops = append(result.Drops, fmt.Sprintf("历史前%d名 · 代币 ×%d", tier.MaxRank, tier.Token))
		}
		if err := queueGachaLadderBestRewards(tx, now); err != nil {
			return err
		}
		if _, err := syncGachaLadderMailbox(tx, userID, state, now); err != nil {
			return err
		}
		if err := storeGachaPlayer(tx, player, state, now); err != nil {
			return err
		}
		result.Save, result.Revision = state, player.Revision
		return recordGachaAction(tx, userID, intent.RequestID, hash, result, now)
	})
	return result, err
}

func challengeGachaLadder(tx *gorm.DB, me *GachaLadderEntry, attack *GachaGameState, opponentID, expectedRank int, now time.Time) (*GachaLadderResult, error) {
	if opponentID == me.ID {
		return nil, ErrGachaIntent
	}
	day := gachaLadderBoundary(now, 5).Format("2006-01-02")
	if me.Day != day {
		me.Day, me.Used = day, 0
	}
	if me.Used >= GachaLadderDailyLimit || now.UnixMilli() < me.CooldownUntil {
		return nil, ErrGachaLadderLimit
	}
	var opponent GachaLadderEntry
	if err := tx.First(&opponent, "id = ?", opponentID).Error; err != nil {
		return nil, ErrGachaLadder
	}
	// Clients can challenge only the same neighborhood offered by the view.
	if opponent.Rank != expectedRank || opponent.Rank < max(1, me.Rank*40/100) || opponent.Rank > max(6, me.Rank+2) {
		return nil, ErrGachaLadder
	}
	if opponentID > 0 {
		name, err := gachaLadderIdentity(tx, opponentID)
		if err != nil {
			return nil, ErrGachaLadder
		}
		opponent.Name = name
	}
	var defense GachaGameState
	if err := common.Unmarshal([]byte(opponent.Defense), &defense); err != nil {
		return nil, err
	}
	report, err := SimulateGachaArena(attack, &defense)
	if err != nil {
		return nil, err
	}
	battle := GachaLadderBattle{ID: uuid.NewString(), AttackerID: me.ID, DefenderID: opponent.ID,
		AttackerName: me.Name, DefenderName: opponent.Name, AttackerBefore: me.Rank, DefenderBefore: opponent.Rank,
		AttackWin: report.Win && !report.TimedOut, PlayedAt: now.UnixMilli()}
	if battle.AttackWin && opponent.Rank < me.Rank {
		// Temporary rank zero avoids unique-index collisions in every dialect.
		if err := tx.Model(&GachaLadderEntry{}).Where("id = ?", me.ID).Update("position", 0).Error; err != nil {
			return nil, err
		}
		me.Rank, opponent.Rank = opponent.Rank, me.Rank
		me.BestRank = gachaLadderBestRank(me.BestRank, me.Rank)
		if err := tx.Save(&opponent).Error; err != nil {
			return nil, err
		}
	}
	me.Used++
	me.CooldownUntil = now.Add(10 * time.Second).UnixMilli()
	if err := tx.Save(me).Error; err != nil {
		return nil, err
	}
	battle.AttackerAfter, battle.DefenderAfter = me.Rank, opponent.Rank
	data, err := common.Marshal(report)
	if err != nil {
		return nil, err
	}
	battle.Report = GachaDocument(data)
	if err := tx.Create(&battle).Error; err != nil {
		return nil, err
	}
	return gachaLadderResult(battle, me.ID)
}

func gachaLadderResult(battle GachaLadderBattle, userID int) (*GachaLadderResult, error) {
	var report GachaBattleReport
	if err := common.Unmarshal([]byte(battle.Report), &report); err != nil {
		return nil, err
	}
	result := &GachaLadderResult{GachaLadderBattle: battle, Ladder: true, Name: battle.DefenderName, Win: battle.AttackWin,
		OwnBefore: battle.AttackerBefore, OwnAfter: battle.AttackerAfter, Report: &report}
	if userID == battle.DefenderID {
		result.Name, result.Win, result.OwnBefore, result.OwnAfter = battle.AttackerName, !battle.AttackWin, battle.DefenderBefore, battle.DefenderAfter
	}
	return result, nil
}
func GetGachaLadderReplay(userID int, id string) (*GachaLadderResult, error) {
	if !validateGachaUUID(id) {
		return nil, ErrGachaIntent
	}
	if _, err := gachaLadderIdentity(DB, userID); err != nil {
		return nil, err
	}
	var battle GachaLadderBattle
	if err := DB.Where("id = ? AND (attacker_id = ? OR defender_id = ?)", strings.ToLower(id), userID, userID).First(&battle).Error; err != nil {
		return nil, ErrGachaIntent
	}
	return gachaLadderResult(battle, userID)
}

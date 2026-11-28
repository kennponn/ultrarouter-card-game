package model

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrGachaCardSource = errors.New("card unavailable from this source")
var ErrGachaWorldClosed = errors.New("world summon is closed or season changed")
var ErrGachaWorldReward = errors.New("world reward unavailable")
var ErrGachaWorldTickets = fmt.Errorf("limited tickets required: %w", ErrGachaFunds)

func gachaCardHasSource(girl *GachaGirl, source string) bool {
	return girl != nil && (girl.Sources == nil && source == "summon" || slices.Contains(girl.Sources, source))
}

func validGachaCardSources(girls []GachaGirl) bool {
	crew := []string{"c01", "c06", "c08", "c12"}
	pools := map[string]int{}
	for _, girl := range girls {
		if girl.Sources != nil && len(girl.Sources) == 0 || slices.Contains(crew, girl.ID) != gachaCardHasSource(&girl, "crew") {
			return false
		}
		for index, source := range girl.Sources {
			if slices.Contains(girl.Sources[:index], source) || !slices.Contains([]string{"summon", "world", "voyage", "growth", "starter", "crew"}, source) || source == "world" && girl.Rarity == "SR" || source == "starter" && girl.Rarity != "SSR" {
				return false
			}
		}
		for _, source := range []string{"summon", "world", "voyage"} {
			if gachaCardHasSource(&girl, source) {
				pools[source+girl.Rarity]++
			}
		}
	}
	return pools["summonR"] > 0 && pools["summonSR"] > 0 && pools["summonSSR"] > 0 && pools["worldR"] > 0 && pools["worldSSR"] > 0 && pools["voyageR"] > 0 && pools["voyageSR"] > 0 && pools["voyageSSR"] > 0
}

func pickGachaSourceGirl(rarity, source, element string, random func(int) (int, error)) (*GachaGirl, error) {
	var pool, preferred []*GachaGirl
	for index := range gachaCatalog.Girls {
		girl := &gachaCatalog.Girls[index]
		if girl.Rarity == rarity && gachaCardHasSource(girl, source) {
			pool = append(pool, girl)
			if element != "" && girl.Elem == element {
				preferred = append(preferred, girl)
			}
		}
	}
	if len(preferred) > 0 {
		pool = preferred
	}
	if len(pool) == 0 {
		return nil, ErrGachaCardSource
	}
	index, err := random(len(pool))
	if err != nil {
		return nil, err
	}
	return pool[index], nil
}

func grantGachaSourceShards(state *GachaGameState, girl *GachaGirl, source string, amount int) error {
	if !gachaCardHasSource(girl, source) {
		return ErrGachaCardSource
	}
	key := "shard:" + girl.ID
	current, ok := gachaIssuedResource(state, key)
	if !ok || amount < 1 || amount > gachaMaxResource || current < 0 || current > gachaMaxResource-amount {
		return ErrGachaCapacity
	}
	setGachaMailResource(state, key, current+amount)
	return nil
}

func grantGachaWholeCard(state *GachaGameState, girl *GachaGirl, source string) (GachaPullResult, error) {
	var pull GachaPullResult
	if !slices.Contains([]string{"summon", "starter", "world"}, source) || !gachaCardHasSource(girl, source) || source == "world" && girl.Rarity != "R" {
		return pull, ErrGachaCardSource
	}
	own := state.Girls[girl.ID]
	pull.GirlID, pull.IsNew = girl.ID, !gachaCardOwned(own)
	if pull.IsNew {
		if own == nil {
			own = &GachaOwnedGirl{Gems: newGachaGemSlots()}
			state.Girls[girl.ID] = own
		}
		own.Level, own.Star = 1, gachaCatalog.Rarity[girl.Rarity].BaseStar
		return pull, nil
	}
	pull.Dup = true
	if source == "world" {
		if own.Copies >= gachaMaxResource {
			return pull, ErrGachaCapacity
		}
		own.Copies++
	} else {
		if err := grantGachaSourceShards(state, girl, source, girl.DupGain); err != nil {
			return pull, err
		}
		pull.Shards = girl.DupGain
	}
	return pull, nil
}

func recycleGachaWholeCopy(state *GachaGameState, id string, result *GachaActionResult) error {
	girl, own := gachaGirl(id), state.Girls[id]
	if !gachaCardOwned(own) || own.Copies <= 0 || girl == nil || girl.Rarity != "R" {
		return ErrGachaFunds
	}
	if err := grantGachaSourceShards(state, girl, "world", girl.DupGain); err != nil {
		return err
	}
	own.Copies--
	result.Drops = append(result.Drops, fmt.Sprintf("%s重复整卡 −1，专属碎片 +%d", girl.Name, girl.DupGain))
	return nil
}

type GachaWorldSeason struct {
	SeasonKey string `gorm:"type:varchar(10);primaryKey" json:"week"`
	Edition   int    `gorm:"not null;default:1" json:"edition"`
	Hero      string `gorm:"type:varchar(16);not null" json:"hero"`
	ClosesAt  int64  `gorm:"not null;index:idx_world_close,priority:2" json:"closes_at"`
	Settled   bool   `gorm:"not null;default:false;index:idx_world_close,priority:1" json:"settled"`
	Revision  int64  `gorm:"not null;default:0" json:"-"`
}

type GachaWorldEntry struct {
	SeasonKey   string            `gorm:"type:varchar(10);primaryKey;index:idx_world_board,priority:1" json:"week"`
	UserID      int               `gorm:"primaryKey;autoIncrement:false;index:idx_world_board,priority:4;index:idx_world_pending,priority:1" json:"-"`
	Hero        string            `gorm:"type:varchar(16);not null" json:"hero"`
	Name        string            `gorm:"type:varchar(128);not null" json:"-"`
	Points      int               `gorm:"not null;index:idx_world_board,priority:2,sort:desc" json:"points"`
	ReachedAt   int64             `gorm:"not null;index:idx_world_board,priority:3" json:"-"`
	ChestMask   int               `gorm:"not null;default:0" json:"chest_mask"`
	FinalRank   int               `gorm:"not null;default:0" json:"final_rank"`
	PrizeShards int               `gorm:"not null;default:0" json:"prize_shards"`
	RankClaimed bool              `gorm:"not null;default:false" json:"rank_claimed"`
	Pending     bool              `gorm:"not null;default:false;index:idx_world_pending,priority:2" json:"-"`
	Edition     int               `gorm:"not null;default:1" json:"edition"`
	Chests      []GachaWorldChest `gorm:"-" json:"chests,omitempty"`
}

type GachaWorldChest struct {
	Points  int `json:"points"`
	Shards  int `json:"shards"`
	Tickets int `json:"tickets"`
}

type GachaWorldTier struct {
	Through int `json:"through"`
	Shards  int `json:"shards"`
}

type gachaWorldRules struct {
	ShardPercent int               `json:"shard_percent"`
	Chests       []GachaWorldChest `json:"chests"`
	Tiers        []GachaWorldTier  `json:"tiers"`
}

type gachaWorldRulebook struct {
	Edition  int                     `json:"edition"`
	Editions map[int]gachaWorldRules `json:"editions"`
}

//go:embed gacha_world.json
var gachaWorldJSON []byte

var gachaWorldBook = func() gachaWorldRulebook {
	var book gachaWorldRulebook
	if err := common.Unmarshal(gachaWorldJSON, &book); err != nil {
		panic(err)
	}
	if _, ok := book.Editions[book.Edition]; !ok {
		panic("missing active world edition")
	}
	for edition, rules := range book.Editions {
		if edition < 1 || rules.ShardPercent < 1 || rules.ShardPercent > 99 || len(rules.Chests) == 0 || len(rules.Chests) > 24 || len(rules.Tiers) == 0 {
			panic("invalid world rules")
		}
		lastPoints, lastRank, lastShards := 0, 0, gachaMaxResource
		for _, chest := range rules.Chests {
			if chest.Points <= lastPoints || chest.Points > gachaMaxResource || chest.Shards < 1 || chest.Shards > gachaMaxResource || chest.Tickets < 0 || chest.Tickets >= chest.Points-lastPoints {
				panic("invalid world chest")
			}
			lastPoints = chest.Points
		}
		for _, tier := range rules.Tiers {
			if tier.Through <= lastRank || tier.Through > 20 || tier.Shards < 1 || tier.Shards > lastShards {
				panic("invalid world prize")
			}
			lastRank, lastShards = tier.Through, tier.Shards
		}
		if lastRank != 20 {
			panic("world prizes must cover top twenty")
		}
	}
	return book
}()

var gachaWorldShardPercent = gachaWorldBook.Editions[gachaWorldBook.Edition].ShardPercent
var gachaWorldChests = gachaWorldBook.Editions[gachaWorldBook.Edition].Chests
var gachaWorldTiers = gachaWorldBook.Editions[gachaWorldBook.Edition].Tiers

func gachaWorldRankShards(rank int) int {
	return gachaWorldBook.Editions[gachaWorldBook.Edition].rankShards(rank)
}

func (rules gachaWorldRules) rankShards(rank int) int {
	for _, tier := range rules.Tiers {
		if rank > 0 && rank <= tier.Through {
			return tier.Shards
		}
	}
	return 0
}

func gachaWorldEarnedMask(points int) int {
	return gachaWorldBook.Editions[gachaWorldBook.Edition].earnedMask(points)
}

func (rules gachaWorldRules) earnedMask(points int) int {
	mask := 0
	for index, chest := range rules.Chests {
		if points >= chest.Points {
			mask |= 1 << index
		}
	}
	return mask
}

func gachaWorldSchedule(now time.Time) GachaWorldSeason {
	monday := gachaMonday(now)
	var heroes []string
	for _, girl := range gachaCatalog.Girls {
		if girl.Rarity == "SSR" && gachaCardHasSource(&girl, "world") {
			heroes = append(heroes, girl.ID)
		}
	}
	slices.Sort(heroes)
	week := (monday.Unix() - time.Date(2026, 1, 5, 0, 0, 0, 0, gameZone).Unix()) / (7 * 86400)
	cycle := int64(len(heroes))
	return GachaWorldSeason{SeasonKey: gachaDay(monday), Hero: heroes[(week%cycle+cycle)%cycle], Edition: gachaWorldBook.Edition, ClosesAt: monday.AddDate(0, 0, 5).UnixMilli()}
}

func validateGachaWorldIntent(intent GachaActionIntent) error {
	if intent.Squad != nil || intent.Materials != nil || intent.Enabled != nil || intent.OpponentID != nil || intent.StakeE != nil || intent.QueueID != nil || intent.RoomID != nil || intent.Choice != nil || intent.Config != nil {
		return ErrGachaIntent
	}
	if intent.Action == "card_recycle_copy" {
		if intent.Count != nil || intent.ItemID != nil || intent.GirlID == nil || gachaGirl(*intent.GirlID) == nil {
			return ErrGachaIntent
		}
		return nil
	}
	if intent.ItemID == nil || intent.GirlID != nil {
		return ErrGachaIntent
	}
	start, err := gachaCalendarDay(*intent.ItemID)
	if err != nil || start.Weekday() != time.Monday {
		return ErrGachaIntent
	}
	switch intent.Action {
	case "world_pull":
		if intent.Count == nil || *intent.Count != 1 && *intent.Count != 10 {
			return ErrGachaIntent
		}
	case "world_chest":
		if intent.Count != nil {
			for _, rules := range gachaWorldBook.Editions {
				if slices.ContainsFunc(rules.Chests, func(chest GachaWorldChest) bool { return chest.Points == *intent.Count }) {
					return nil
				}
			}
		}
		return ErrGachaIntent
	case "world_rank":
		if intent.Count != nil {
			return ErrGachaIntent
		}
	default:
		return ErrGachaIntent
	}
	return nil
}

func rollGachaWorld(state *GachaGameState, hero *GachaGirl, count int, receipt *GachaActionResult, random func(int) (int, error)) error {
	return gachaWorldBook.Editions[gachaWorldBook.Edition].roll(state, hero, count, receipt, random)
}

func (rules gachaWorldRules) roll(state *GachaGameState, hero *GachaGirl, count int, receipt *GachaActionResult, random func(int) (int, error)) error {
	if count != 1 && count != 10 || !gachaCardHasSource(hero, "world") || hero.Rarity != "SSR" {
		return ErrGachaIntent
	}
	if state.Mats["limited_ticket"] < count {
		return ErrGachaWorldTickets
	}
	for range count {
		roll, err := random(100)
		if err != nil {
			return err
		}
		if roll < rules.ShardPercent {
			if err := grantGachaSourceShards(state, hero, "world", 1); err != nil {
				return err
			}
			receipt.Drops = append(receipt.Drops, hero.Name+"碎片 ×1")
			continue
		}
		girl, err := pickGachaSourceGirl("R", "world", "", random)
		if err != nil {
			return err
		}
		outcome, err := grantGachaWholeCard(state, girl, "world")
		if err != nil {
			return err
		}
		label := "整卡 ×1（新伙伴）"
		if outcome.Dup {
			label = "整卡 ×1（重复整卡已留存）"
		}
		receipt.Drops = append(receipt.Drops, girl.Name+label)
	}
	state.Mats["limited_ticket"] -= count
	return nil
}

func lockGachaWorldSeason(tx *gorm.DB, key string) (*GachaWorldSeason, error) {
	locked := tx.Model(&GachaWorldSeason{}).Where("season_key = ?", key).UpdateColumn("revision", gorm.Expr("revision + 1"))
	if locked.Error != nil {
		return nil, locked.Error
	}
	if locked.RowsAffected != 1 {
		return nil, ErrGachaWorldReward
	}
	var season GachaWorldSeason
	err := lockForUpdate(tx).Where("season_key = ?", key).Take(&season).Error
	return &season, err
}

func gachaWorldBoardQuery(tx *gorm.DB, season *GachaWorldSeason) *gorm.DB {
	query := tx.Model(&GachaWorldEntry{}).Where("season_key = ?", season.SeasonKey)
	if season.Settled {
		return query.Where("final_rank > 0").Order("final_rank ASC").Limit(20)
	}
	enabled := tx.Session(&gorm.Session{NewDB: true}).Model(&User{}).Select("id").Where("status = ?", common.UserStatusEnabled)
	return query.Where("points > 0 AND user_id IN (?)", enabled).Order("points DESC, reached_at ASC, user_id ASC").Limit(20)
}

func settleLockedGachaWorld(tx *gorm.DB, season *GachaWorldSeason, now time.Time) error {
	if season.Settled || now.UnixMilli() < season.ClosesAt {
		return nil
	}
	var winners []GachaWorldEntry
	rules, ok := gachaWorldBook.Editions[season.Edition]
	if !ok {
		return ErrGachaRunState
	}
	if err := lockForUpdate(gachaWorldBoardQuery(tx, season)).Find(&winners).Error; err != nil {
		return err
	}
	for index, winner := range winners {
		if err := tx.Model(&winner).Updates(map[string]any{"final_rank": index + 1, "prize_shards": rules.rankShards(index + 1), "pending": true}).Error; err != nil {
			return err
		}
	}
	if err := tx.Model(season).Update("settled", true).Error; err != nil {
		return err
	}
	season.Settled = true
	return nil
}

func SettleGachaWorld(ctx context.Context, now time.Time) error {
	var keys []string
	if err := DB.WithContext(ctx).Model(&GachaWorldSeason{}).Where("settled = ? AND closes_at <= ?", false, now.UnixMilli()).Order("closes_at ASC").Limit(16).Pluck("season_key", &keys).Error; err != nil {
		return err
	}
	for _, key := range keys {
		err := gachaTransaction(func(tx *gorm.DB) error {
			tx = tx.WithContext(ctx)
			season, err := lockGachaWorldSeason(tx, key)
			if err != nil {
				return err
			}
			return settleLockedGachaWorld(tx, season, now)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func applyGachaWorld(tx *gorm.DB, userID int, state *GachaGameState, intent GachaActionIntent, result *GachaActionResult, now, started time.Time) error {
	key := *intent.ItemID
	if intent.Action == "world_pull" {
		candidate := gachaWorldSchedule(now)
		if candidate.SeasonKey != key || now.UnixMilli() >= candidate.ClosesAt {
			return ErrGachaWorldClosed
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&candidate).Error; err != nil {
			return err
		}
	}
	season, err := lockGachaWorldSeason(tx, key)
	if err != nil {
		return err
	}
	now = now.Add(time.Since(started))
	if intent.Action == "world_pull" && (season.Settled || now.UnixMilli() >= season.ClosesAt) {
		return ErrGachaWorldClosed
	}
	if err := settleLockedGachaWorld(tx, season, now); err != nil {
		return err
	}
	rules, known := gachaWorldBook.Editions[season.Edition]
	if !known {
		return ErrGachaRunState
	}
	var entry GachaWorldEntry
	err = lockForUpdate(tx).Where("season_key = ? AND user_id = ?", key, userID).Take(&entry).Error
	newEntry := errors.Is(err, gorm.ErrRecordNotFound)
	if newEntry {
		if intent.Action != "world_pull" {
			return ErrGachaWorldReward
		}
		var user User
		if err := tx.Select("id", "display_name").First(&user, userID).Error; err != nil {
			return err
		}
		entry = GachaWorldEntry{SeasonKey: key, UserID: userID, Hero: season.Hero, Edition: season.Edition, Name: fleetPublicName(user.DisplayName, userID)}
	} else if err != nil {
		return err
	} else if entry.Edition != season.Edition || entry.Hero != season.Hero {
		return ErrGachaRunState
	}
	hero := gachaGirl(season.Hero)
	switch intent.Action {
	case "world_pull":
		if entry.Points > gachaMaxResource-*intent.Count {
			return ErrGachaCapacity
		}
		if err := rules.roll(state, hero, *intent.Count, result, gachaRandom); err != nil {
			return err
		}
		entry.Points += *intent.Count
		entry.ReachedAt = max(now.UnixMilli(), entry.ReachedAt)
	case "world_chest":
		index := slices.IndexFunc(rules.Chests, func(chest GachaWorldChest) bool { return chest.Points == *intent.Count })
		if index < 0 || entry.Points < *intent.Count || entry.ChestMask&(1<<index) != 0 {
			return ErrGachaWorldReward
		}
		chest := rules.Chests[index]
		rewards := map[string]int{"shard:" + season.Hero: chest.Shards}
		if chest.Tickets > 0 {
			rewards["limited_ticket"] = chest.Tickets
		}
		if err := grantGachaIssuedResources(state, rewards, result, now); err != nil {
			return err
		}
		entry.ChestMask |= 1 << index
	case "world_rank":
		if !season.Settled || entry.FinalRank < 1 || entry.FinalRank > 20 || entry.PrizeShards < 1 || entry.RankClaimed {
			return ErrGachaWorldReward
		}
		if err := grantGachaIssuedResources(state, map[string]int{"shard:" + season.Hero: entry.PrizeShards}, result, now); err != nil {
			return err
		}
		entry.RankClaimed = true
	}
	entry.Pending = entry.ChestMask != rules.earnedMask(entry.Points) || entry.FinalRank > 0 && !entry.RankClaimed
	if newEntry {
		return tx.Create(&entry).Error
	}
	updated := tx.Model(&entry).Select("points", "reached_at", "chest_mask", "rank_claimed", "pending").Updates(&entry)
	if updated.Error == nil && updated.RowsAffected != 1 {
		return ErrGachaConcurrent
	}
	return updated.Error
}

type GachaWorldStanding struct {
	Position int    `json:"rank"`
	Name     string `json:"name"`
	Points   int    `json:"points"`
	Shards   int    `json:"shards"`
	Self     bool   `json:"self"`
}

type GachaWorldView struct {
	GachaWorldSeason
	NextOpen      int64                `json:"next_open"`
	Open          bool                 `json:"open"`
	ShardPercent  int                  `json:"shard_percent"`
	Chests        []GachaWorldChest    `json:"chests"`
	Tiers         []GachaWorldTier     `json:"tiers"`
	Me            GachaWorldEntry      `json:"me"`
	Board         []GachaWorldStanding `json:"board"`
	Previous      *GachaWorldSeason    `json:"previous,omitempty"`
	PreviousBoard []GachaWorldStanding `json:"previous_board,omitempty"`
	Unclaimed     []GachaWorldEntry    `json:"unclaimed"`
}

func readGachaWorldBoard(tx *gorm.DB, season *GachaWorldSeason, userID int) ([]GachaWorldStanding, error) {
	rules, ok := gachaWorldBook.Editions[season.Edition]
	if !ok {
		return nil, ErrGachaRunState
	}
	var rows []GachaWorldEntry
	if err := gachaWorldBoardQuery(tx, season).Find(&rows).Error; err != nil {
		return nil, err
	}
	board := make([]GachaWorldStanding, 0, len(rows))
	for index, entry := range rows {
		shards := rules.rankShards(index + 1)
		if season.Settled {
			shards = entry.PrizeShards
		}
		board = append(board, GachaWorldStanding{index + 1, entry.Name, entry.Points, shards, entry.UserID == userID})
	}
	return board, nil
}

func GetGachaWorld(userID int, now time.Time) (*GachaWorldView, error) {
	if userID < 1 {
		return nil, ErrGachaIntent
	}
	if err := SettleGachaWorld(context.Background(), now); err != nil {
		return nil, err
	}
	var view GachaWorldView
	err := gachaTransaction(func(tx *gorm.DB) error {
		if _, err := gachaLadderIdentity(tx, userID); err != nil {
			return err
		}
		season := gachaWorldSchedule(now)
		view = GachaWorldView{GachaWorldSeason: season, NextOpen: gachaMonday(now).AddDate(0, 0, 7).UnixMilli()}
		var stored GachaWorldSeason
		err := tx.Where("season_key = ?", season.SeasonKey).Take(&stored).Error
		if err == nil {
			view.GachaWorldSeason = stored
		} else if errors.Is(err, gorm.ErrRecordNotFound) && now.UnixMilli() >= view.ClosesAt {
			view.Settled = true
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		rules, ok := gachaWorldBook.Editions[view.Edition]
		if !ok {
			return ErrGachaRunState
		}
		view.ShardPercent, view.Chests, view.Tiers = rules.ShardPercent, rules.Chests, rules.Tiers
		view.Open = !view.Settled && now.UnixMilli() < view.ClosesAt
		err = tx.Where("season_key = ? AND user_id = ?", season.SeasonKey, userID).Take(&view.Me).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			view.Me = GachaWorldEntry{SeasonKey: season.SeasonKey, Hero: view.Hero, Edition: view.Edition}
		} else if err != nil {
			return err
		}
		view.Board, err = readGachaWorldBoard(tx, &view.GachaWorldSeason, userID)
		if err != nil {
			return err
		}
		var last GachaWorldSeason
		err = tx.Where("settled = ? AND season_key < ?", true, season.SeasonKey).Order("season_key DESC").Take(&last).Error
		if err == nil {
			view.Previous = &last
			view.PreviousBoard, err = readGachaWorldBoard(tx, &last, userID)
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := tx.Where("user_id = ? AND season_key < ? AND pending = ?", userID, season.SeasonKey, true).Order("season_key ASC").Limit(20).Find(&view.Unclaimed).Error; err != nil {
			return err
		}
		for index := range view.Unclaimed {
			rules, ok := gachaWorldBook.Editions[view.Unclaimed[index].Edition]
			if !ok {
				return ErrGachaRunState
			}
			view.Unclaimed[index].Chests = rules.Chests
		}
		return nil
	})
	return &view, err
}

package model

import (
	"errors"
	"slices"
	"time"

	"gorm.io/gorm"
)

var (
	ErrGachaSweepStars = errors.New("sweep requires a three-star clearance")
	ErrGachaSweepDate  = errors.New("sweep day has changed")
	ErrGachaSweepBurst = errors.New("sweep batch would cross the rest threshold")
)

func gachaSweepPrizes(point *GachaVoyagePoint) []GachaVoyagePrize {
	if point.RelicTier != 0 || !slices.Contains([]string{"story", "hard", "resources"}, point.Kind) {
		return nil
	}
	if len(point.Prize) > 0 {
		return slices.Clone(point.Prize)
	}
	if point.Kind == "story" {
		return []GachaVoyagePrize{{"forge_ore", 2}, {"training_ink", 1}}
	}
	return nil
}

func validateGachaSweepIntent(intent GachaActionIntent) error {
	if intent.ItemID == nil || intent.Count == nil || *intent.Count != 1 && *intent.Count != 3 || intent.Choice == nil || intent.Materials != nil || intent.GirlID != nil || intent.Squad != nil || intent.Enabled != nil || intent.OpponentID != nil || intent.StakeE != nil || intent.QueueID != nil || intent.RoomID != nil || intent.Config != nil {
		return ErrGachaIntent
	}
	if _, err := gachaCalendarDay(*intent.Choice); err != nil {
		return ErrGachaIntent
	}
	if point := gachaVoyage.Stages[*intent.ItemID]; point == nil || len(point.SweepPrize) == 0 {
		return ErrGachaIntent
	}
	return nil
}

func requireGachaRunIdle(tx *gorm.DB, userID int, now time.Time) error {
	var active GachaRun
	query := tx.Select("id").Where("user_id = ? AND status = ? AND expire_at > ? AND ready_at > ?", userID, "running", now.UnixMilli(), 0).Limit(1).Find(&active)
	if query.Error != nil {
		return query.Error
	}
	if query.RowsAffected > 0 {
		return ErrGachaRunState
	}
	return nil
}

func spendGachaVoyageAttempts(state *GachaGameState, point *GachaVoyagePoint, count int) error {
	if point == nil || point.DailyLimit == 0 {
		return nil
	}
	used := state.Daily.VoyageAttempts[point.ID]
	if used < 0 || count < 1 || used > point.DailyLimit-count {
		return ErrGachaVoyageLimit
	}
	if state.Daily.VoyageAttempts == nil {
		state.Daily.VoyageAttempts = map[string]int{}
	}
	state.Daily.VoyageAttempts[point.ID] = used + count
	return nil
}

func sweepGachaVoyage(tx *gorm.DB, userID int, state *GachaGameState, intent GachaActionIntent, result *GachaActionResult, now time.Time) error {
	if *intent.Choice != state.Daily.Date || state.Daily.Date != gachaDay(now) {
		return ErrGachaSweepDate
	}
	point, count := gachaVoyage.Stages[*intent.ItemID], *intent.Count
	if err := checkGachaVoyagePoint(state, point); err != nil {
		return err
	}
	if !state.Cleared[point.ID] || state.Stars[point.ID] != gachaMaxStars {
		return ErrGachaSweepStars
	}
	if err := checkGachaRunRest(state, now); err != nil {
		return err
	}
	if err := requireGachaRunIdle(tx, userID, now); err != nil {
		return err
	}
	if point.Cost < 1 || state.Stamina/count < point.Cost {
		return ErrGachaFunds
	}
	budget := GachaGameState{Activity: state.Activity}
	for range count {
		if checkGachaRunRest(&budget, now) != nil {
			return ErrGachaSweepBurst
		}
		trackGachaRunStart(&budget, now, now.UnixMilli())
	}
	prizes := slices.Clone(point.SweepPrize)
	seen := make(map[string]bool, len(prizes))
	for index, prize := range prizes {
		if seen[prize.Item] || prize.Amount < 1 || prize.Amount > gachaMaxResource/count {
			return ErrGachaRunState
		}
		seen[prize.Item] = true
		prizes[index].Amount *= count
	}
	if err := spendGachaVoyageAttempts(state, point, count); err != nil {
		return err
	}
	notices, err := verifyGachaDispatchSweep(state, point, count)
	if err != nil {
		return err
	}
	drops, err := grantGachaVoyagePrize(state, prizes)
	if err != nil {
		return err
	}
	if err := spendGachaStamina(state, point.Cost*count, now, &drops); err != nil {
		return err
	}
	state.Activity = budget.Activity
	advanceGachaCaptainTaskBy(state, "voyage", count)
	result.Drops = append(drops, notices...)
	return nil
}

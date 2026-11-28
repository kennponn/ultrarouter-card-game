package model_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func runGachaWorldMatrix(t *testing.T) {
	t.Run("world_ticket_replay_week_close_top20_and_permanent_prizes", func(t *testing.T) {
		zone := time.FixedZone("Beijing", 8*3600)
		monday := time.Date(2028, 1, 3, 0, 0, 0, 0, zone)
		closeAt, next := monday.AddDate(0, 0, 5), monday.AddDate(0, 0, 7)
		user := gachaUser(t)
		state := gachaState(t, user.Id, monday)
		state.Token, state.Gold, state.Progression.Permits = 5000, 5000, 5000
		seedGachaState(t, user.Id, state)
		tables := []any{&model.GachaWorldEntry{}, &model.GachaWorldSeason{}}
		require.NoError(t, model.DB.Migrator().DropTable(tables...))
		require.NoError(t, model.DB.AutoMigrate(tables...))
		recorder := &gachaSchemaRecorder{}
		require.NoError(t, model.DB.Session(&gorm.Session{Logger: recorder}).AutoMigrate(tables...))
		require.Empty(t, recorder.ddl)
		view, err := model.GetGachaWorld(user.Id, monday)
		require.NoError(t, err)
		require.True(t, view.Open)
		require.Empty(t, view.Board)
		require.Equal(t, monday.Format("2006-01-02"), view.SeasonKey)
		pinned := view.GachaWorldSeason
		pinned.Hero = "c03"
		if pinned.Hero == view.Hero {
			pinned.Hero = "c15"
		}
		require.NoError(t, model.DB.Create(&pinned).Error)
		view, err = model.GetGachaWorld(user.Id, monday)
		require.NoError(t, err)
		require.Equal(t, pinned.Hero, view.Hero)
		command := func(action string, count int) model.GachaActionIntent {
			intent := model.GachaActionIntent{Action: action, RequestID: uuid.NewString(), ItemID: &view.SeasonKey}
			if count != 0 {
				intent.Count = &count
			}
			return intent
		}
		draw := command("world_pull", 10)
		_, err = model.ApplyGachaAction(user.Id, draw, monday)
		require.ErrorIs(t, err, model.ErrGachaFunds)
		state.Mats["limited_ticket"] = 10
		seedGachaState(t, user.Id, state)
		callback := "world-rollback-" + uuid.NewString()
		failure := errors.New("isolated receipt failure")
		require.NoError(t, model.DB.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
			if action, ok := tx.Statement.Dest.(*model.GachaAction); ok && action.RequestID == draw.RequestID {
				tx.AddError(failure)
			}
		}))
		_, err = model.ApplyGachaAction(user.Id, draw, monday)
		require.NoError(t, model.DB.Callback().Create().Remove(callback))
		require.ErrorIs(t, err, failure)
		require.Equal(t, 10, gachaState(t, user.Id, monday).Mats["limited_ticket"])
		empty, err := model.GetGachaWorld(user.Id, monday)
		require.NoError(t, err)
		require.Empty(t, empty.Board)
		type attempt struct {
			out *model.GachaActionResult
			err error
		}
		start, replies := make(chan struct{}), make(chan attempt, 2)
		for range 2 {
			go func() {
				<-start
				out, err := model.ApplyGachaAction(user.Id, draw, monday)
				replies <- attempt{out, err}
			}()
		}
		close(start)
		first, second := <-replies, <-replies
		require.NoError(t, first.err)
		require.NoError(t, second.err)
		require.Equal(t, first.out, second.out)
		result := first.out
		require.Len(t, result.Drops, 10)
		require.Zero(t, result.Save.Mats["limited_ticket"])
		require.Equal(t, 5000, result.Save.Token)
		require.Equal(t, 5000, result.Save.Gold)
		require.Equal(t, 5000, result.Save.Progression.Permits)
		retry, err := model.ApplyGachaAction(user.Id, draw, monday)
		require.NoError(t, err)
		require.Equal(t, result, retry)
		reused := command("world_pull", 1)
		reused.RequestID = draw.RequestID
		_, err = model.ApplyGachaAction(user.Id, reused, monday)
		require.ErrorIs(t, err, model.ErrGachaReplay)
		low := 0
		for index := range 20 {
			rival := gachaUser(t)
			points := 1
			if index < 2 {
				points = 20 - index
			}
			row := model.GachaWorldEntry{SeasonKey: view.SeasonKey, UserID: rival.Id, Hero: view.Hero, Name: fmt.Sprintf("航海员 %02d", index), Points: points, ReachedAt: monday.UnixMilli()}
			require.NoError(t, model.DB.Create(&row).Error)
			low = rival.Id
		}
		live, err := model.GetGachaWorld(user.Id, closeAt.Add(-time.Minute))
		require.NoError(t, err)
		require.Len(t, live.Board, 20)
		require.True(t, live.Board[2].Self)
		require.Equal(t, pinned.Hero, live.Hero)
		require.Equal(t, pinned.Edition, live.Edition)
		require.Equal(t, 10, live.Me.Points)
		_, err = model.ApplyGachaAction(user.Id, command("world_rank", 0), closeAt.Add(-time.Minute))
		require.ErrorIs(t, err, model.ErrGachaWorldReward)
		continued, err := model.ApplyGachaAction(user.Id, draw, closeAt)
		require.NoError(t, err)
		require.Equal(t, result, continued)
		_, err = model.ApplyGachaAction(user.Id, command("world_pull", 1), closeAt)
		require.ErrorIs(t, err, model.ErrGachaWorldClosed)
		closed, err := model.GetGachaWorld(user.Id, closeAt)
		require.NoError(t, err)
		require.False(t, closed.Open)
		require.True(t, closed.Settled)
		require.Equal(t, 3, closed.Me.FinalRank)
		require.Equal(t, 15, closed.Me.PrizeShards)
		require.NoError(t, model.SettleGachaWorld(context.Background(), closeAt))
		_, err = model.ApplyGachaAction(low, command("world_rank", 0), closeAt)
		require.ErrorIs(t, err, model.ErrGachaWorldReward)
		rolled, err := model.GetGachaWorld(user.Id, next)
		require.NoError(t, err)
		require.Len(t, rolled.Unclaimed, 1)
		require.NotEqual(t, view.SeasonKey, rolled.SeasonKey)
		require.Empty(t, rolled.Board)
		require.Equal(t, closed.Board, rolled.PreviousBoard)
		var before int
		if own := result.Save.Girls[view.Hero]; own != nil {
			before = own.Shards
		}
		for _, claim := range []model.GachaActionIntent{command("world_chest", 3), command("world_chest", 8), command("world_rank", 0)} {
			paid, err := model.ApplyGachaAction(user.Id, claim, next)
			require.NoError(t, err)
			again, err := model.ApplyGachaAction(user.Id, claim, next)
			require.NoError(t, err)
			require.Equal(t, paid, again)
			claim.RequestID = uuid.NewString()
			_, err = model.ApplyGachaAction(user.Id, claim, next)
			require.ErrorIs(t, err, model.ErrGachaWorldReward)
		}
		state = gachaState(t, user.Id, next)
		require.Equal(t, before+2+3+15, state.Girls[view.Hero].Shards)
		require.Equal(t, 1, state.Mats["limited_ticket"])
		rolled, err = model.GetGachaWorld(user.Id, next)
		require.NoError(t, err)
		require.Empty(t, rolled.Unclaimed)
		_, err = model.ApplyGachaAction(user.Id, command("world_pull", 1), next)
		require.ErrorIs(t, err, model.ErrGachaWorldClosed)
	})
}

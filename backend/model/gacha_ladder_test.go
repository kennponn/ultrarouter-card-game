package model_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Invoked inside the existing disposable SQLite/MySQL/PostgreSQL host matrix.
// No substitute account model or in-memory database is used by these cases.
func runGachaLadderMatrix(t *testing.T, clock time.Time) {
	t.Run("ranked_arena_21h_material_settlement", func(t *testing.T) {
		zone := time.FixedZone("Asia/Shanghai", 8*3600)
		local := clock.In(zone)
		now := time.Date(local.Year(), local.Month(), local.Day(), 20, 59, 0, 0, zone)
		tables := []any{&model.GachaLadderGate{}, &model.GachaLadderEntry{}, &model.GachaLadderMail{}, &model.GachaLadderBattle{}}
		user := gachaUser(t)
		state := gachaState(t, user.Id, now)
		strongGachaState(state)
		seedGachaState(t, user.Id, state)
		var before model.GachaPlayer
		require.NoError(t, model.DB.First(&before, "user_id = ?", user.Id).Error)
		require.NoError(t, model.DB.Migrator().DropTable(tables...))
		require.NoError(t, model.DB.AutoMigrate(tables...))
		require.NoError(t, model.InitializeGachaLadder(now))
		require.NoError(t, model.DB.AutoMigrate(tables...))
		require.NoError(t, model.InitializeGachaLadder(now))
		var after model.GachaPlayer
		require.NoError(t, model.DB.First(&after, "user_id = ?", user.Id).Error)
		assert.Equal(t, before, after, "fresh/upgrade ladder migration preserves game resources")
		var count int64
		require.NoError(t, model.DB.Model(&model.GachaLadderEntry{}).Count(&count).Error)
		assert.EqualValues(t, 100, count, "idempotent NPC initialization")
		view, err := model.GetGachaLadder(user.Id, now)
		require.NoError(t, err)
		assert.Nil(t, view.Me)
		assert.Empty(t, view.Mail)
		join := model.GachaActionIntent{RequestID: uuid.NewString(), Action: "ladder_join"}
		_, err = model.ApplyGachaAction(user.Id, join, now)
		require.NoError(t, err)
		_, err = model.ApplyGachaAction(user.Id, join, now)
		require.NoError(t, err, "replayed registration does not append a second seat")
		view, err = model.GetGachaLadder(user.Id, now)
		require.NoError(t, err)
		assert.Equal(t, 101, view.Me.Rank)
		assert.Len(t, view.Opponents, 5)
		assert.Equal(t, 5, view.Remaining)
		assert.Equal(t, 21, time.UnixMilli(view.NextSettlement).In(zone).Hour())
		assert.Equal(t, 5, time.UnixMilli(view.NextReset).In(zone).Hour())
		assert.Equal(t, view.Me.Rank, view.Me.BestRank)

		other := gachaUser(t)
		gachaState(t, other.Id, now)
		_, err = model.ApplyGachaAction(other.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "ladder_join"}, now)
		require.NoError(t, err)
		outsider := gachaUser(t)
		gachaState(t, outsider.Id, now)
		for _, handler := range []func(){
			func() { assert.Equal(t, 403, gachaRequest(t, user.Id, false, controller.GetGachaLadder, nil).Code) },
			func() {
				assert.Equal(t, 403, gachaRequest(t, user.Id, false, controller.GetGachaLadderReplay, nil).Code)
			},
		} {
			handler()
		}
		for _, extra := range []string{"save", "rank", "materials", "win", "report"} {
			response := gachaRequest(t, user.Id, true, controller.ApplyGachaAction, map[string]any{"request_id": uuid.NewString(), "action": "ladder_join", extra: 1})
			assert.Equal(t, 400, response.Code)
		}
		stake := 1
		assert.ErrorIs(t, model.ValidateGachaAction(model.GachaActionIntent{RequestID: uuid.NewString(), Action: "ladder_join", StakeE: &stake}), model.ErrGachaIntent)
		for _, action := range []string{"arena", "arena_join", "shop"} {
			target := -100
			assert.Error(t, model.ValidateGachaAction(model.GachaActionIntent{RequestID: uuid.NewString(), Action: action, OpponentID: &target}))
		}

		target, rank := -100, 100
		challenge := model.GachaActionIntent{RequestID: uuid.NewString(), Action: "ladder_challenge", OpponentID: &target, Count: &rank}
		out, err := model.ApplyGachaAction(user.Id, challenge, now.Add(time.Second))
		require.NoError(t, err)
		require.NotNil(t, out.Ladder)
		require.True(t, out.Ladder.Win)
		assert.Equal(t, 101, out.Ladder.OwnBefore)
		assert.Equal(t, 100, out.Ladder.OwnAfter)
		assert.Equal(t, state.Token, out.Save.Token, "historical rewards wait for explicit direct collection")
		assert.Nil(t, ladderPublicMail(out.Save, "ladder:best:100"))
		var breakthrough model.GachaLadderMail
		require.NoError(t, model.DB.First(&breakthrough, "user_id = ? AND day = ?", user.Id, "best:100").Error)
		assert.False(t, breakthrough.Claimed)
		assert.Zero(t, breakthrough.DeliveredAt)
		assert.Equal(t, 200, breakthrough.Token)
		assert.Zero(t, out.Save.Daily.ArenaPlayed, "ranked challenges do not consume E matchmaking allowance")
		retry, err := model.ApplyGachaAction(user.Id, challenge, now.Add(2*time.Second))
		require.NoError(t, err)
		initialJSON, err := common.Marshal(out)
		require.NoError(t, err)
		replayJSON, err := common.Marshal(retry)
		require.NoError(t, err)
		assert.JSONEq(t, string(initialJSON), string(replayJSON))
		var attempts int
		require.NoError(t, model.DB.Model(&model.GachaLadderEntry{}).Where("id = ?", user.Id).Select("used").Scan(&attempts).Error)
		assert.Equal(t, 1, attempts)
		_, err = model.GetGachaLadderReplay(outsider.Id, out.Ladder.ID)
		assert.Error(t, err, "unrelated users cannot read private battle events")
		replay, err := model.GetGachaLadderReplay(user.Id, out.Ladder.ID)
		require.NoError(t, err)
		assert.Equal(t, out.Ladder.Report, replay.Report)
		defendedRank := 100
		defenseBattle, err := model.ApplyGachaAction(other.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "ladder_challenge", OpponentID: &user.Id, Count: &defendedRank}, now.Add(3*time.Second))
		require.NoError(t, err)
		require.False(t, defenseBattle.Ladder.Win, "weak attack cannot beat the registered strong defense")
		defenderReplay, err := model.GetGachaLadderReplay(user.Id, defenseBattle.Ladder.ID)
		require.NoError(t, err)
		assert.True(t, defenderReplay.Win)
		assert.Equal(t, 100, defenderReplay.OwnBefore)
		assert.Equal(t, 100, defenderReplay.OwnAfter)
		assert.Equal(t, defenseBattle.Ladder.Report, defenderReplay.Report, "both viewers replay the same attacker-left event stream")
		challenge.RequestID = uuid.NewString()
		_, err = model.ApplyGachaAction(user.Id, challenge, now.Add(2*time.Second))
		assert.ErrorIs(t, err, model.ErrGachaLadderLimit)
		_, err = model.ApplyGachaAction(user.Id, challenge, now.Add(12*time.Second))
		assert.ErrorIs(t, err, model.ErrGachaLadder, "stale rank quote must not consume an attempt")
		view, err = model.GetGachaLadder(user.Id, now.Add(12*time.Second))
		require.NoError(t, err)
		assert.Equal(t, 4, view.Remaining)

		// Ordinary squad edits must not silently replace the registered defense.
		defense := view.Me.Defense
		squad := []string{"c04"}
		_, err = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "squad", Squad: &squad}, now.Add(15*time.Second))
		require.NoError(t, err)
		view, err = model.GetGachaLadder(user.Id, now.Add(15*time.Second))
		require.NoError(t, err)
		assert.Equal(t, defense, view.Me.Defense)
		_, err = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "ladder_defense"}, now.Add(16*time.Second))
		require.NoError(t, err)
		view, err = model.GetGachaLadder(user.Id, now.Add(16*time.Second))
		require.NoError(t, err)
		assert.Len(t, view.Me.Squad, 1)

		// No worker ran at 21:00. A later challenge must pay the old rank first.
		boundary := now.Add(time.Minute)
		target, rank = -99, 99
		challenge.RequestID = uuid.NewString()
		out, err = model.ApplyGachaAction(user.Id, challenge, boundary.Add(time.Second))
		require.NoError(t, err)
		require.True(t, out.Ladder.Win)
		assert.Equal(t, 99, out.Ladder.OwnAfter)
		view, err = model.GetGachaLadder(user.Id, boundary.Add(2*time.Second))
		require.NoError(t, err)
		require.Len(t, view.Mail, 1)
		assert.Equal(t, 100, view.Mail[0].Rank, "21:00 snapshot precedes the post-21:00 rank change")
		assert.Equal(t, model.GachaLadderReward(100), view.Mail[0].Materials)
		assert.Equal(t, 80, view.Mail[0].Token)
		assert.Positive(t, view.Mail[0].DeliveredAt)
		require.NoError(t, model.SettleGachaLadder(context.Background(), boundary.Add(3*time.Second)))
		require.NoError(t, model.SettleGachaLadder(context.Background(), boundary.Add(3*time.Second)))
		day := view.Mail[0].Day
		oldR := out.Save.Mats["mat_r"]
		oldToken := out.Save.Token
		legacy, err := model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "ladder_claim", ItemID: &day}, boundary.Add(4*time.Second))
		require.NoError(t, err)
		assert.Equal(t, oldR, legacy.Save.Mats["mat_r"], "old claim buttons only deliver to the shared inbox")
		assert.Equal(t, oldToken, legacy.Save.Token)
		mailID := "ladder:daily:" + day
		claim := model.GachaActionIntent{RequestID: uuid.NewString(), Action: "mail_claim", ItemID: &mailID}
		claimed, err := model.ApplyGachaAction(user.Id, claim, boundary.Add(4*time.Second))
		require.NoError(t, err)
		assert.Equal(t, oldR+40, claimed.Save.Mats["mat_r"])
		assert.Equal(t, oldToken+80, claimed.Save.Token)
		again, err := model.ApplyGachaAction(user.Id, claim, boundary.Add(5*time.Second))
		require.NoError(t, err)
		assert.Equal(t, claimed, again, "claim retries do not credit twice")
		claim.RequestID = uuid.NewString()
		_, err = model.ApplyGachaAction(user.Id, claim, boundary.Add(6*time.Second))
		assert.ErrorIs(t, err, model.ErrGachaMailUnavailable)
		wallet := model.User{}
		require.NoError(t, model.DB.First(&wallet, user.Id).Error)
		assert.Equal(t, user.Quota, wallet.Quota, "ranked play and materials do not mutate E balance")

		for i := 0; i < 3; i++ {
			rank = 98 - i
			require.NoError(t, model.DB.Model(&model.GachaLadderEntry{}).Where("position = ?", rank).Select("id").Scan(&target).Error)
			challenge.RequestID = uuid.NewString()
			_, err = model.ApplyGachaAction(user.Id, challenge, boundary.Add(time.Duration(20+i*12)*time.Second))
			require.NoError(t, err)
		}
		challenge.RequestID = uuid.NewString()
		_, err = model.ApplyGachaAction(user.Id, challenge, boundary.Add(time.Minute))
		assert.ErrorIs(t, err, model.ErrGachaLadderLimit)
		reset := boundary.Add(8 * time.Hour)
		view, err = model.GetGachaLadder(user.Id, reset.Add(-time.Second))
		require.NoError(t, err)
		assert.Zero(t, view.Remaining)
		view, err = model.GetGachaLadder(user.Id, reset)
		require.NoError(t, err)
		assert.Equal(t, 5, view.Remaining, "attempts reset at 05:00 Beijing, not 21:00")

		// Offline catch-up and concurrent claiming stay exactly once.
		later := boundary.Add(48 * time.Hour)
		require.NoError(t, model.SettleGachaLadder(context.Background(), later))
		view, err = model.GetGachaLadder(user.Id, later)
		require.NoError(t, err)
		require.Len(t, view.Mail, 3)
		newDay := "ladder:daily:" + later.In(zone).Format("2006-01-02")
		state = gachaState(t, user.Id, later)
		oldR = state.Mats["mat_r"]
		var errs [2]error
		var wg sync.WaitGroup
		for i := range errs {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				_, errs[i] = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "mail_claim", ItemID: &newDay}, later)
			}(i)
		}
		wg.Wait()
		assert.NotEqual(t, errs[0] == nil, errs[1] == nil)
		assert.Equal(t, oldR+40, gachaState(t, user.Id, later).Mats["mat_r"])
		previousDay := later.Add(-24 * time.Hour).In(zone).Format("2006-01-02")
		previousMailID := "ladder:daily:" + previousDay
		state = gachaState(t, user.Id, later)
		state.Mats["mat_r"] = 1_000_000_000
		seedGachaState(t, user.Id, state)
		_, err = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "mail_claim", ItemID: &previousMailID}, later)
		assert.ErrorIs(t, err, model.ErrGachaCapacity)
		var mail model.GachaLadderMail
		require.NoError(t, model.DB.First(&mail, "user_id = ? AND day = ?", user.Id, previousDay).Error)
		assert.Positive(t, mail.DeliveredAt, "public inbox delivery is independent of asset capacity")
		state = gachaState(t, user.Id, later)
		for _, message := range state.Mailbox.Messages {
			if message.ID == previousMailID {
				assert.False(t, message.Claimed, "capacity failure leaves public attachments pending")
			}
		}
		t.Log("100 NPCs; swap 101 -> 100 -> 99; replay uses 1 attempt; rank-100 daily mail 40/20/5 + 80 tokens; best-100 entitlement 200 tokens; 05:00 reset; concurrent public claim exactly once; E wallet unchanged")
		require.NoError(t, model.InitializeGachaLadder(later))
		var positions []int
		require.NoError(t, model.DB.Model(&model.GachaLadderEntry{}).Where("position > 0").Order("position ASC").Pluck("position", &positions).Error)
		for i, position := range positions {
			assert.Equal(t, i+1, position, "swaps and re-initialization keep unique contiguous ranks")
		}
		require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", user.Id).Update("status", common.UserStatusDisabled).Error)
		_, err = model.GetGachaLadder(user.Id, later)
		assert.Error(t, err)
		_, err = model.GetGachaLadderReplay(user.Id, out.Ladder.ID)
		assert.Error(t, err)
	})
	runGachaLadderMailbox(t)
}

var ladderTestZone = time.FixedZone("UTC+8", 8*3600)

func nextLadderTestBoundary(t *testing.T) time.Time {
	t.Helper()
	var gate model.GachaLadderGate
	require.NoError(t, model.DB.First(&gate, 1).Error)
	return time.UnixMilli(gate.SettledThrough).In(ladderTestZone).Add(24 * time.Hour)
}

func seedGachaLadderLeaders(t *testing.T, when time.Time) (model.User, model.User) {
	t.Helper()
	leader, runnerUp := gachaUser(t), gachaUser(t)
	for offset, user := range []model.User{leader, runnerUp} {
		_, err := model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "ladder_join"}, when)
		require.NoError(t, err)
		require.NoError(t, model.DB.Transaction(func(tx *gorm.DB) error {
			var entry model.GachaLadderEntry
			if err := tx.First(&entry, user.Id).Error; err != nil {
				return err
			}
			originalRank := entry.Rank
			if err := tx.Model(&entry).Update("position", 0).Error; err != nil {
				return err
			}
			rank := offset + 1
			if err := tx.Model(&model.GachaLadderEntry{}).Where("position = ?", rank).Update("position", originalRank).Error; err != nil {
				return err
			}
			return tx.Model(&entry).Updates(map[string]any{"position": rank, "best_rank": rank}).Error
		}))
	}
	return leader, runnerUp
}

func ladderTestMail(t *testing.T, userID int, boundary time.Time) []model.GachaLadderMail {
	t.Helper()
	var mail []model.GachaLadderMail
	require.NoError(t, model.DB.Where("user_id = ? AND day = ?", userID, boundary.Format("2006-01-02")).Find(&mail).Error)
	// Delivery may legitimately happen later on an ordinary profile load. These
	// settlement assertions compare the immutable entitlement and cutoff only.
	for i := range mail {
		mail[i].DeliveredAt = 0
	}
	return mail
}

func runGachaLadderEligibility(t *testing.T) {
	for _, scenario := range []struct {
		name, removal string
		refresh       bool
	}{
		{name: "disabled_with_refresh", removal: "disabled", refresh: true},
		{name: "disabled_without_refresh", removal: "disabled"},
		{name: "deleted_without_refresh", removal: "deleted"},
		{name: "withdrawn", removal: "withdrawn"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			previous := nextLadderTestBoundary(t)
			leader, runnerUp := seedGachaLadderLeaders(t, previous.Add(-time.Minute))
			require.NoError(t, model.SettleGachaLadder(context.Background(), previous))
			leaderHistory, runnerHistory := ladderTestMail(t, leader.Id, previous), ladderTestMail(t, runnerUp.Id, previous)
			require.Len(t, leaderHistory, 1)
			require.Len(t, runnerHistory, 1)
			require.Equal(t, 2, runnerHistory[0].Rank)
			require.Equal(t, model.GachaLadderMaterials{R: 100, SR: 50, SSR: 15}, runnerHistory[0].Materials)
			boundary := previous.Add(24 * time.Hour)
			when := boundary.Add(-time.Minute)
			cooldown := when.Add(10 * time.Second).UnixMilli()
			require.NoError(t, model.DB.Model(&model.GachaLadderEntry{}).Where("id = ?", leader.Id).Updates(map[string]any{"used": 4, "day": when.Format("2006-01-02"), "cooldown_until": cooldown}).Error)
			switch scenario.removal {
			case "disabled":
				require.NoError(t, model.DB.Model(&leader).Update("status", common.UserStatusDisabled).Error)
			case "deleted":
				require.NoError(t, model.DB.Delete(&leader).Error)
			case "withdrawn":
				leave := model.GachaActionIntent{RequestID: uuid.NewString(), Action: "ladder_leave"}
				_, err := model.ApplyGachaAction(leader.Id, leave, when)
				require.NoError(t, err)
				_, err = model.ApplyGachaAction(leader.Id, leave, when)
				require.NoError(t, err)
			}
			if scenario.refresh {
				view, err := model.GetGachaLadder(runnerUp.Id, when.Add(time.Second))
				require.NoError(t, err)
				require.NotNil(t, view.Me)
				require.Equal(t, 1, view.Me.Rank)
			} else if scenario.removal != "withdrawn" {
				var before model.GachaLadderEntry
				require.NoError(t, model.DB.First(&before, runnerUp.Id).Error)
				require.Equal(t, 2, before.Rank, "no ladder request or worker has reclaimed the seat")
			}
			require.Empty(t, ladderTestMail(t, runnerUp.Id, boundary))
			require.NoError(t, model.SettleGachaLadder(context.Background(), boundary))
			var settledSeat model.GachaLadderEntry
			require.NoError(t, model.DB.First(&settledSeat, runnerUp.Id).Error)
			require.Equal(t, 1, settledSeat.Rank)
			require.Empty(t, ladderTestMail(t, leader.Id, boundary))
			mail := ladderTestMail(t, runnerUp.Id, boundary)
			require.Len(t, mail, 1)
			require.Equal(t, 1, mail[0].Rank)
			require.Equal(t, model.GachaLadderMaterials{R: 120, SR: 60, SSR: 20}, mail[0].Materials)
			require.Equal(t, boundary.UnixMilli(), mail[0].SettledAt)
			for _, retry := range []time.Time{boundary, boundary.Add(time.Minute)} {
				require.NoError(t, model.SettleGachaLadder(context.Background(), retry))
				require.Equal(t, mail, ladderTestMail(t, runnerUp.Id, boundary))
				require.Empty(t, ladderTestMail(t, leader.Id, boundary))
				require.Equal(t, leaderHistory, ladderTestMail(t, leader.Id, previous))
				require.Equal(t, runnerHistory, ladderTestMail(t, runnerUp.Id, previous))
			}
			view, err := model.GetGachaLadder(runnerUp.Id, boundary.Add(time.Minute))
			require.NoError(t, err)
			require.NotNil(t, view.Me)
			require.Equal(t, 1, view.Me.Rank)
			require.NotEmpty(t, view.Board)
			require.Equal(t, runnerUp.Id, view.Board[0].ID)
			require.Len(t, view.Opponents, 5)
			for _, entry := range view.Opponents {
				require.NotEqual(t, leader.Id, entry.ID)
			}
			if scenario.removal == "withdrawn" {
				_, err := model.ApplyGachaAction(leader.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "ladder_join"}, boundary)
				require.NoError(t, err)
				view, err := model.GetGachaLadder(leader.Id, boundary)
				require.NoError(t, err)
				require.EqualValues(t, view.Total, view.Me.Rank)
				require.Equal(t, 1, view.Remaining)
				require.Equal(t, cooldown, view.Me.CooldownUntil)
			}
			t.Logf("%s: board rank=1, mail rank=1 and 120/60/20; new rows A/B=0/1 after two retries; prior mail unchanged", scenario.name)
		})
	}
	t.Run("player_challenge_at_cutoff", func(t *testing.T) {
		for _, scenario := range []struct {
			name     string
			offset   time.Duration
			mailRank int
			award    model.GachaLadderMaterials
		}{
			{name: "before_21", offset: -5 * time.Second, mailRank: 1, award: model.GachaLadderMaterials{R: 120, SR: 60, SSR: 20}},
			{name: "after_21", offset: 5 * time.Second, mailRank: 2, award: model.GachaLadderMaterials{R: 100, SR: 50, SSR: 15}},
		} {
			t.Run(scenario.name, func(t *testing.T) {
				boundary := nextLadderTestBoundary(t)
				setup := boundary.Add(-time.Minute)
				leader, runnerUp := seedGachaLadderLeaders(t, setup)
				state := gachaState(t, runnerUp.Id, setup)
				strongGachaState(state)
				seedGachaState(t, runnerUp.Id, state)
				first := 1
				intent := model.GachaActionIntent{RequestID: uuid.NewString(), Action: "ladder_challenge", OpponentID: &leader.Id, Count: &first}
				result, err := model.ApplyGachaAction(runnerUp.Id, intent, boundary.Add(scenario.offset))
				require.NoError(t, err)
				require.NotNil(t, result.Ladder)
				require.True(t, result.Ladder.Win)
				require.Equal(t, 2, result.Ladder.OwnBefore)
				require.Equal(t, 1, result.Ladder.OwnAfter)
				require.NoError(t, model.SettleGachaLadder(context.Background(), boundary.Add(time.Minute)))
				challengerMail := ladderTestMail(t, runnerUp.Id, boundary)
				require.Len(t, challengerMail, 1)
				require.Equal(t, scenario.mailRank, challengerMail[0].Rank)
				require.Equal(t, scenario.award, challengerMail[0].Materials)
				defenderMail := ladderTestMail(t, leader.Id, boundary)
				require.Len(t, defenderMail, 1)
				require.Equal(t, 3-scenario.mailRank, defenderMail[0].Rank)
				require.Equal(t, model.GachaLadderReward(defenderMail[0].Rank), defenderMail[0].Materials)
				t.Logf("%s: real win 2 -> 1, credited cutoff rank=%d and %d/%d/%d", scenario.name, scenario.mailRank, scenario.award.R, scenario.award.SR, scenario.award.SSR)
			})
		}
	})
}

func ladderPublicMail(state *model.GachaGameState, id string) *model.GachaMailMessage {
	for _, message := range state.Mailbox.Messages {
		if message.ID == id {
			return message
		}
	}
	return nil
}

func runGachaLadderMailbox(t *testing.T) {
	t.Run("historical_tiers_claim_directly_after_retirement_and_survive_rejoin", func(t *testing.T) {
		now := nextLadderTestBoundary(t).Add(-10 * time.Minute)
		user := gachaUser(t)
		join := model.GachaActionIntent{RequestID: uuid.NewString(), Action: "ladder_join"}
		_, err := model.ApplyGachaAction(user.Id, join, now)
		require.NoError(t, err)
		// Upgraded best ranks unlock all crossed tiers without mailing or paying.
		require.NoError(t, model.DB.Model(&model.GachaLadderEntry{}).Where("id = ?", user.Id).Update("best_rank", 1).Error)
		before := gachaState(t, user.Id, now)
		view, err := model.GetGachaLadder(user.Id, now)
		require.NoError(t, err)
		require.Equal(t, 1, view.BestRank)
		require.Len(t, view.BestRewards, 6)
		state := gachaState(t, user.Id, now)
		assert.Equal(t, before.Token, state.Token)
		assert.Equal(t, before.Mailbox, state.Mailbox)
		var outbox []model.GachaLadderMail
		require.NoError(t, model.DB.Where("user_id = ? AND kind = ?", user.Id, "best").Find(&outbox).Error)
		require.Len(t, outbox, 6)
		_, err = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "ladder_leave"}, now)
		require.NoError(t, err)
		view, err = model.GetGachaLadder(user.Id, now)
		require.NoError(t, err)
		assert.Nil(t, view.Me)
		assert.Equal(t, 1, view.BestRank)
		total := 0
		for _, tier := range view.BestRewards {
			assert.True(t, tier.CanClaim, "retired players can still collect reached tiers")
			assert.False(t, tier.Claimed)
			assert.Zero(t, tier.DeliveredAt)
			id := "best:" + fmt.Sprint(tier.MaxRank)
			claim := model.GachaActionIntent{RequestID: uuid.NewString(), Action: "ladder_best_claim", ItemID: &id}
			claimed, err := model.ApplyGachaAction(user.Id, claim, now)
			require.NoError(t, err)
			total += tier.Token
			assert.Equal(t, before.Token+total, claimed.Save.Token)
			assert.Equal(t, before.Mailbox, claimed.Save.Mailbox)
			replay, err := model.ApplyGachaAction(user.Id, claim, now)
			require.NoError(t, err)
			assert.Equal(t, claimed, replay, "same request replays the exact receipt")
			claim.RequestID = uuid.NewString()
			_, err = model.ApplyGachaAction(user.Id, claim, now)
			assert.ErrorIs(t, err, model.ErrGachaLadder, "new requests cannot claim a tier twice")
		}
		assert.Equal(t, 6200, total)
		require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", user.Id).Update("display_name", "更名后的旅人").Error)
		join.RequestID = uuid.NewString()
		_, err = model.ApplyGachaAction(user.Id, join, now)
		require.NoError(t, err)
		require.NoError(t, model.DB.Model(&model.GachaLadderEntry{}).Where("id = ?", user.Id).Update("best_reward_rank", 0).Error)
		view, err = model.GetGachaLadder(user.Id, now)
		require.NoError(t, err)
		state = gachaState(t, user.Id, now)
		assert.Equal(t, before.Token+6200, state.Token)
		for _, tier := range view.BestRewards {
			assert.True(t, tier.Claimed)
			assert.False(t, tier.CanClaim)
			assert.Zero(t, tier.DeliveredAt)
			assert.Nil(t, ladderPublicMail(state, "ladder:best:"+fmt.Sprint(tier.MaxRank)))
		}
		var count int64
		require.NoError(t, model.DB.Model(&model.GachaLadderMail{}).Where("user_id = ? AND kind = ?", user.Id, "best").Count(&count).Error)
		assert.EqualValues(t, 6, count)
		var wallet model.User
		require.NoError(t, model.DB.First(&wallet, user.Id).Error)
		assert.Equal(t, user.Quota, wallet.Quota)
	})
	t.Run("best_claim_rejects_forged_tiers_and_unreached_ranks", func(t *testing.T) {
		now := nextLadderTestBoundary(t).Add(-10 * time.Minute)
		user := gachaUser(t)
		_, err := model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "ladder_join"}, now)
		require.NoError(t, err)
		for _, id := range []string{"", "best:0", "best:2", "best:0100", "best:+100", "best:100 ", "ladder:best:100", "2026-11-27"} {
			assert.ErrorIs(t, model.ValidateGachaAction(model.GachaActionIntent{RequestID: uuid.NewString(), Action: "ladder_best_claim", ItemID: &id}), model.ErrGachaIntent)
		}
		id, extra := "best:100", 200
		assert.ErrorIs(t, model.ValidateGachaAction(model.GachaActionIntent{RequestID: uuid.NewString(), Action: "ladder_best_claim", ItemID: &id, Count: &extra}), model.ErrGachaIntent)
		before := gachaState(t, user.Id, now)
		_, err = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "ladder_best_claim", ItemID: &id}, now)
		assert.ErrorIs(t, err, model.ErrGachaLadder)
		assert.Equal(t, before.Token, gachaState(t, user.Id, now).Token)
	})
	t.Run("best_claim_capacity_failure_leaves_entitlement_and_request_retryable", func(t *testing.T) {
		now := nextLadderTestBoundary(t).Add(-10 * time.Minute)
		user := gachaUser(t)
		_, err := model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "ladder_join"}, now)
		require.NoError(t, err)
		require.NoError(t, model.DB.Model(&model.GachaLadderEntry{}).Where("id = ?", user.Id).Update("best_rank", 100).Error)
		_, err = model.GetGachaLadder(user.Id, now)
		require.NoError(t, err)
		state := gachaState(t, user.Id, now)
		state.Token = 1_000_000_000 - 199
		seedGachaState(t, user.Id, state)
		id := "best:100"
		claim := model.GachaActionIntent{RequestID: uuid.NewString(), Action: "ladder_best_claim", ItemID: &id}
		_, err = model.ApplyGachaAction(user.Id, claim, now)
		assert.ErrorIs(t, err, model.ErrGachaCapacity)
		var reward model.GachaLadderMail
		require.NoError(t, model.DB.First(&reward, "user_id = ? AND day = ?", user.Id, id).Error)
		assert.False(t, reward.Claimed)
		assert.Zero(t, reward.DeliveredAt)
		view, err := model.GetGachaLadder(user.Id, now)
		require.NoError(t, err)
		assert.True(t, view.BestRewards[0].CanClaim)
		state = gachaState(t, user.Id, now)
		assert.Equal(t, 1_000_000_000-199, state.Token)
		assert.Nil(t, ladderPublicMail(state, "ladder:best:100"))
		state.Token--
		seedGachaState(t, user.Id, state)
		result, err := model.ApplyGachaAction(user.Id, claim, now)
		require.NoError(t, err, "a failed capacity check does not consume the idempotency key")
		assert.Equal(t, 1_000_000_000, result.Save.Token)
	})
	t.Run("concurrent_best_claims_pay_exactly_once", func(t *testing.T) {
		now := nextLadderTestBoundary(t).Add(-10 * time.Minute)
		user := gachaUser(t)
		_, err := model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "ladder_join"}, now)
		require.NoError(t, err)
		require.NoError(t, model.DB.Model(&model.GachaLadderEntry{}).Where("id = ?", user.Id).Update("best_rank", 100).Error)
		before := gachaState(t, user.Id, now)
		id := "best:100"
		var errs [2]error
		var wg sync.WaitGroup
		for i := range errs {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				_, errs[i] = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "ladder_best_claim", ItemID: &id}, now)
			}(i)
		}
		wg.Wait()
		assert.NotEqual(t, errs[0] == nil, errs[1] == nil)
		for _, err := range errs {
			if err != nil {
				assert.ErrorIs(t, err, model.ErrGachaLadder)
			}
		}
		assert.Equal(t, before.Token+200, gachaState(t, user.Id, now).Token)
	})
	t.Run("legacy_best_mail_remains_in_inbox_and_never_duplicates_direct_claim", func(t *testing.T) {
		now := nextLadderTestBoundary(t).Add(-10 * time.Minute)
		user := gachaUser(t)
		_, err := model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "ladder_join"}, now)
		require.NoError(t, err)
		require.NoError(t, model.DB.Model(&model.GachaLadderEntry{}).Where("id = ?", user.Id).Update("best_rank", 50).Error)
		state := gachaState(t, user.Id, now)
		beforeToken := state.Token
		mailID := "ladder:best:100"
		state.Mailbox.Messages = append(state.Mailbox.Messages, &model.GachaMailMessage{ID: mailID, Title: "旧历史排名奖励", Sender: "竞技场", CreatedAt: now.UnixMilli(), Resources: map[string]int{"token": 200}})
		seedGachaState(t, user.Id, state)
		legacy := model.GachaLadderMail{UserID: user.Id, Day: "best:100", Kind: "best", Rank: 100, Token: 200, SettledAt: now.UnixMilli(), DeliveredAt: now.UnixMilli()}
		pending := model.GachaLadderMail{UserID: user.Id, Day: "best:50", Kind: "best", Rank: 50, Token: 400, SettledAt: now.UnixMilli()}
		require.NoError(t, model.DB.Create(&legacy).Error)
		require.NoError(t, model.DB.Create(&pending).Error)
		view, err := model.GetGachaLadder(user.Id, now)
		require.NoError(t, err)
		assert.False(t, view.BestRewards[0].CanClaim)
		assert.Positive(t, view.BestRewards[0].DeliveredAt)
		assert.True(t, view.BestRewards[1].CanClaim, "undelivered old entitlements switch to direct collection")
		id := "best:100"
		_, err = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "ladder_best_claim", ItemID: &id}, now)
		assert.ErrorIs(t, err, model.ErrGachaLadder)
		_, err = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "mail_claim", ItemID: &mailID}, now)
		require.NoError(t, err)
		assert.Equal(t, beforeToken+200, gachaState(t, user.Id, now).Token)
		_, err = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "mail_delete_read"}, now)
		require.NoError(t, err)
		require.NoError(t, model.DB.Model(&model.GachaLadderEntry{}).Where("id = ?", user.Id).Update("best_reward_rank", 0).Error)
		_, err = model.GetGachaLadder(user.Id, now)
		require.NoError(t, err)
		_, err = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "ladder_best_claim", ItemID: &id}, now)
		assert.ErrorIs(t, err, model.ErrGachaLadder, "deleting the old letter cannot reopen direct collection")
		id = "best:50"
		result, err := model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "ladder_best_claim", ItemID: &id}, now)
		require.NoError(t, err)
		assert.Equal(t, beforeToken+600, result.Save.Token)
		assert.Nil(t, ladderPublicMail(result.Save, "ladder:best:50"))
	})
	t.Run("legacy_mail_migration_is_atomic_even_on_action_replay", func(t *testing.T) {
		now := nextLadderTestBoundary(t).Add(-5 * time.Minute)
		user := gachaUser(t)
		join := model.GachaActionIntent{RequestID: uuid.NewString(), Action: "ladder_join"}
		_, err := model.ApplyGachaAction(user.Id, join, now)
		require.NoError(t, err)
		before := gachaState(t, user.Id, now)
		day := now.AddDate(0, 0, -1).Format("2006-01-02")
		claimedDay := now.AddDate(0, 0, -2).Format("2006-01-02")
		legacy := model.GachaLadderMail{UserID: user.Id, Day: day, Rank: 50, Materials: model.GachaLadderMaterials{R: 60, SR: 30, SSR: 8}, SettledAt: now.AddDate(0, 0, -1).UnixMilli()}
		claimed := model.GachaLadderMail{UserID: user.Id, Day: claimedDay, Rank: 1, Materials: model.GachaLadderMaterials{R: 120, SR: 60, SSR: 20}, SettledAt: now.AddDate(0, 0, -2).UnixMilli(), Claimed: true}
		require.NoError(t, model.DB.Create(&legacy).Error)
		require.NoError(t, model.DB.Create(&claimed).Error)
		_, err = model.ApplyGachaAction(user.Id, join, now)
		require.NoError(t, err, "an idempotent old action still persists newly synchronized mail")
		state := gachaState(t, user.Id, now)
		message := ladderPublicMail(state, "ladder:daily:"+day)
		require.NotNil(t, message)
		assert.Equal(t, map[string]int{"mat_r": 60, "mat_sr": 30, "mat_ssr": 8}, message.Resources, "legacy attachment amounts are preserved without retroactive token grants")
		assert.Nil(t, ladderPublicMail(state, "ladder:daily:"+claimedDay), "already claimed legacy rows must not migrate again")
		assert.Equal(t, before.Token, state.Token)
		assert.Equal(t, before.Mats, state.Mats)
		require.NoError(t, model.DB.First(&legacy, "user_id = ? AND day = ?", user.Id, day).Error)
		assert.Positive(t, legacy.DeliveredAt)
		for i := 0; i < 3; i++ {
			_, err = model.GetGachaLadder(user.Id, now)
			require.NoError(t, err)
		}
		state = gachaState(t, user.Id, now)
		matches := 0
		for _, item := range state.Mailbox.Messages {
			if item.ID == message.ID {
				matches++
			}
		}
		assert.Equal(t, 1, matches)
	})
	t.Run("full_automatic_mailbox_keeps_pending_ranked_outbox", func(t *testing.T) {
		now := nextLadderTestBoundary(t).Add(-time.Minute)
		user := gachaUser(t)
		state := gachaState(t, user.Id, now)
		state.Token = 0
		state.Mailbox.Messages = []*model.GachaMailMessage{}
		for i := 0; i < 200; i++ {
			state.Mailbox.Messages = append(state.Mailbox.Messages, &model.GachaMailMessage{ID: fmt.Sprintf("existing:%d", i), Title: "待领邮件", Sender: "测试邮务署", CreatedAt: now.UnixMilli(), Resources: map[string]int{"gold": 1}})
		}
		state.Mailbox.Messages = append(state.Mailbox.Messages, &model.GachaMailMessage{ID: "system:overflow", Title: "奖励汇总", Sender: "测试邮务署", CreatedAt: now.UnixMilli(), Resources: map[string]int{"token": 1_000_000_000}})
		seedGachaState(t, user.Id, state)
		day := now.Format("2006-01-02")
		mail := model.GachaLadderMail{UserID: user.Id, Day: day, Rank: 1, Token: 300, SettledAt: now.UnixMilli()}
		require.NoError(t, model.DB.Create(&mail).Error)
		state = gachaState(t, user.Id, now)
		assert.Zero(t, state.Token, "mailbox capacity never blocks or silently grants account assets")
		require.NoError(t, model.DB.First(&mail, "user_id = ? AND day = ?", user.Id, day).Error)
		assert.Zero(t, mail.DeliveredAt)
		id := "system:overflow"
		_, err := model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "mail_claim", ItemID: &id}, now)
		require.NoError(t, err)
		state = gachaState(t, user.Id, now)
		summary := ladderPublicMail(state, id)
		require.NotNil(t, summary)
		assert.False(t, summary.Claimed)
		assert.Equal(t, 300, summary.Resources["token"])
		require.NoError(t, model.DB.First(&mail, "user_id = ? AND day = ?", user.Id, day).Error)
		assert.Positive(t, mail.DeliveredAt)
	})
}

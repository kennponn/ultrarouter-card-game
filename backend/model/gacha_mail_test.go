package model

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mailTestIntent(action, id string) GachaActionIntent {
	intent := GachaActionIntent{Action: action, RequestID: uuid.NewString()}
	if id != "" {
		intent.ItemID = &id
	}
	return intent
}

func TestGachaMailGiftIsServerDefinedAndRedeemsOnce(t *testing.T) {
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, gameZone)
	state := newGachaState(now)
	require.NoError(t, syncGachaCaptain(state))
	beforeGold, beforeStamina := state.Gold, state.Stamina
	intent := mailTestIntent("redeem_code", " starfrost2026 ")
	require.NoError(t, ValidateGachaAction(intent))
	receipt := &GachaActionResult{}
	require.NoError(t, applyGachaMail(state, intent, receipt, now))
	require.Empty(t, state.Mailbox.Messages)
	assert.ErrorIs(t, applyGachaMail(state, intent, receipt, now), ErrGachaRedeemCode)
	assert.ErrorIs(t, applyGachaMail(state, mailTestIntent("redeem_code", "NO-SUCH-GIFT"), receipt, now), ErrGachaRedeemCode)
	assert.ErrorIs(t, applyGachaMail(state, mailTestIntent("mail_claim", "other-user-mail"), receipt, now), ErrGachaMailUnavailable)
	assert.Equal(t, beforeGold+300, state.Gold)
	assert.Equal(t, beforeStamina+30, state.Stamina)
	assert.Equal(t, 2, state.Mats["field_notes"])
	assert.Equal(t, 1, state.Captain.Level)
	assert.Zero(t, state.Captain.XP)
	forged := 500
	intent.Count = &forged
	assert.ErrorIs(t, ValidateGachaAction(intent), ErrGachaIntent)
}

func TestGachaMailClaimAllIsAtomicAndUnreadAttachmentsCannotBeDeleted(t *testing.T) {
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, gameZone)
	state := newGachaState(now)
	require.NoError(t, syncGachaCaptain(state))
	require.NoError(t, sendGachaMail(state, "first", "首封", "", map[string]int{"gold": 100}, 40, now))
	require.NoError(t, sendGachaMail(state, "second", "次封", "", map[string]int{"gold": 200, "forge_ore": 3}, 0, now))
	receipt := &GachaActionResult{}
	require.NoError(t, applyGachaMail(state, mailTestIntent("mail_read", "first"), receipt, now))
	require.NoError(t, applyGachaMail(state, mailTestIntent("mail_delete_read", ""), receipt, now))
	require.Len(t, state.Mailbox.Messages, 2)
	state.Gold = gachaMaxResource - 299
	assert.ErrorIs(t, applyGachaMail(state, mailTestIntent("mail_all", ""), receipt, now), ErrGachaCapacity)
	assert.Equal(t, gachaMaxResource-299, state.Gold)
	assert.Zero(t, state.Mats["forge_ore"])
	assert.False(t, state.Mailbox.Messages[0].Claimed)
	assert.Zero(t, state.Captain.XP)
	state.Gold = 0
	require.NoError(t, applyGachaMail(state, mailTestIntent("mail_all", ""), receipt, now))
	assert.Equal(t, 300, state.Gold)
	assert.Equal(t, 3, state.Mats["forge_ore"])
	assert.Equal(t, GachaCaptain{Level: 2, XP: 10}, *state.Captain)
	require.NoError(t, applyGachaMail(state, mailTestIntent("mail_delete_read", ""), receipt, now))
	assert.Empty(t, state.Mailbox.Messages)
}

func TestGachaLegacyDailyMailSurvivesDirectClaimUpgrade(t *testing.T) {
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, gameZone)
	state := newGachaState(now)
	require.NoError(t, syncGachaCaptain(state))
	id := "daily:" + gachaDay(now) + ":rollcall"
	require.NoError(t, sendGachaMail(state, id, "旧任务奖励", "", map[string]int{"gold": 50}, 40, now))
	state.Daily.CaptainLog.Claimed["rollcall"] = true
	before := state.Gold
	require.ErrorIs(t, applyGachaCaptainClaim(state, captainTestIntent(gachaDay(now), "captain_task", "rollcall"), &GachaActionResult{}, now), ErrGachaCaptainClaim)
	next := now.AddDate(0, 0, 1)
	regenGachaState(state, next)
	require.NoError(t, syncGachaCaptain(state))
	require.NoError(t, applyGachaMail(state, mailTestIntent("mail_claim", id), &GachaActionResult{}, next))
	assert.Equal(t, before+50, state.Gold)
	require.NoError(t, applyGachaCaptainClaim(state, captainTestIntent(gachaDay(next), "captain_task", "rollcall"), &GachaActionResult{}, next))
	assert.Equal(t, before+100, state.Gold)
	require.Len(t, state.Mailbox.Messages, 1)
}

func TestGachaRankingOverflowAndDirectClaimsAreIndependent(t *testing.T) {
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, gameZone)
	state := newGachaState(now)
	require.NoError(t, syncGachaCaptain(state))
	for index := range 200 {
		require.NoError(t, sendGachaMail(state, fmt.Sprintf("pending-%d", index), "附件", "", map[string]int{"gold": 1}, 0, now))
	}
	assert.ErrorIs(t, sendGachaMail(state, "overflow", "未发放", "", map[string]int{"gold": 1}, 0, now), ErrGachaMailboxFull)
	before := state.Gold
	require.NoError(t, applyGachaCaptainClaim(state, captainTestIntent(gachaDay(now), "captain_task", "rollcall"), &GachaActionResult{}, now))
	assert.Equal(t, before+50, state.Gold)
	require.Len(t, state.Mailbox.Messages, 200)
	require.NoError(t, sendGachaMail(state, "ladder:daily:2026-11-27", "排名奖励", "", map[string]int{"token": 300}, 0, now))
	require.NoError(t, sendGachaMail(state, "ladder:best:1", "历史排名", "", map[string]int{"token": 2500}, 0, now))
	require.Len(t, state.Mailbox.Messages, 201)
	assert.Equal(t, 2800, state.Mailbox.Messages[200].Resources["token"])
	beforeToken := state.Token
	require.NoError(t, applyGachaMail(state, mailTestIntent("mail_all", ""), &GachaActionResult{}, now))
	assert.Equal(t, beforeToken+2800, state.Token)
	assert.Equal(t, before+250, state.Gold)
	assert.ErrorIs(t, applyGachaMail(state, mailTestIntent("mail_all", ""), &GachaActionResult{}, now), ErrGachaMailUnavailable)
}

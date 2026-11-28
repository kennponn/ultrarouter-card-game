package model_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func runGachaContractClaims(t *testing.T) {
	t.Run("daily_and_dispatch_claims_via_controller", func(t *testing.T) {
		user := gachaUser(t)
		now := time.Now()
		yesterday := now.AddDate(0, 0, -1)
		gachaState(t, user.Id, yesterday)
		state := gachaState(t, user.Id, now)
		for index := range state.Progression.Days {
			state.Progression.Days[index].Activities = []string{"training", "wish"}
		}
		require.Len(t, state.Progression.Days, 2)
		seedGachaState(t, user.Id, state)
		today, prior := state.Daily.Date, state.Progression.Days[0].Date
		marks := state.Progression.Marks
		mailBefore := len(state.Mailbox.Messages)
		claim := func(action, item string) model.GachaActionIntent {
			return model.GachaActionIntent{RequestID: uuid.NewString(), Action: action, ItemID: &item}
		}
		send := func(intent model.GachaActionIntent, success bool) {
			response := gachaRequest(t, user.Id, true, controller.ApplyGachaAction, intent)
			if success {
				require.Equal(t, http.StatusOK, response.Code)
			} else {
				require.NotEqual(t, http.StatusOK, response.Code)
			}
		}
		send(claim("dispatch_contract_claim", today), false)
		for _, day := range []string{today, prior} {
			intent := claim("contract_claim", day)
			send(intent, true)
			send(intent, true)
			send(claim("daily_contract_claim", day), false)
			state = gachaState(t, user.Id, now)
			require.Nil(t, state.Daily.Dispatch.Contract)
			marks += 40
			require.Equal(t, marks, state.Progression.Marks)
			require.Len(t, state.Mailbox.Messages, mailBefore)
			send(claim("mail_claim", "action:"+intent.RequestID), false)
		}
		state.Daily.Dispatch.Contract = &model.GachaSignedClause{ID: "courier", Leader: state.Squad[0], Progress: 2}
		seedGachaState(t, user.Id, state)
		notes, plates := state.Mats["field_notes"], state.Mats["gear_plate"]
		send(claim("dispatch_contract_claim", prior), false)
		intent := claim("dispatch_contract_claim", today)
		send(intent, true)
		send(intent, true)
		send(claim("dispatch_contract_claim", today), false)
		state = gachaState(t, user.Id, now)
		require.True(t, state.Daily.Dispatch.Contract.Claimed)
		require.Equal(t, marks, state.Progression.Marks)
		require.Equal(t, notes+2, state.Mats["field_notes"])
		require.Equal(t, plates+4, state.Mats["gear_plate"])
		require.Len(t, state.Mailbox.Messages, mailBefore)
		for _, day := range state.Progression.Days {
			require.True(t, day.MarksClaimed)
		}
		t.Log("daily current/prior marks=40/40 directly; no new mail; dispatch notes/plates=2/4; same and new UUID retries grant nothing")
	})
}

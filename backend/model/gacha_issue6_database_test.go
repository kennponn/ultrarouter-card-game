package model_test

import (
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func runGachaIssue6Matrix(t *testing.T, clock time.Time) {
	zone := time.FixedZone("Asia/Shanghai", 8*3600)
	local := clock.In(zone)
	for _, clause := range []string{"courier", "choir", "breach"} {
		t.Run("issue6_"+clause, func(t *testing.T) {
			now := time.Date(local.Year(), local.Month(), local.Day(), 10, 0, 0, 0, zone)
			user := gachaUser(t)
			profile, err := model.GetGachaProfile(user.Id, now)
			require.NoError(t, err)
			state := profile.Save
			strongGachaState(state)
			for _, hero := range state.Squad {
				state.Girls[hero].Level = 150
				state.Girls[hero].Training = &model.GachaCardTraining{Skills: map[string]int{"normal": 150, "rage": 150, "passive": 150}}
			}
			leader := "c04"
			duo := []string{leader, "c07"}
			trio := []string{leader, "c07", "c09"}
			state.Stamina = 500
			state.Squad = duo
			if clause != "courier" {
				state.Squad = trio
			}
			for _, gate := range []string{"s1", "s2", "s6", "v1s5", "v1h3"} {
				state.Cleared[gate] = true
			}
			seedGachaState(t, user.Id, state)
			setTeam := func(team []string) {
				t.Helper()
				_, err := model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "squad", Squad: &team}, now)
				require.NoError(t, err)
			}
			start := func(stage string) *model.GachaRunResult {
				t.Helper()
				run, err := model.StartGachaRun(user.Id, stage, uuid.NewString(), now)
				require.NoError(t, err)
				return run
			}
			finish := func(run *model.GachaRunResult, abandon bool, want int, notices ...string) *model.GachaFinishResult {
				t.Helper()
				now = time.UnixMilli(run.ReadyAt).Add(time.Second)
				out, err := model.FinishGachaRun(user.Id, run.ID, abandon, now)
				require.NoError(t, err)
				require.Equal(t, !abandon, out.Win, "real battle at %s", run.Stage)
				require.Equal(t, want, out.Save.Daily.Dispatch.Contract.Progress, "receipt for %s", run.Stage)
				for _, notice := range notices {
					require.Contains(t, strings.Join(out.Drops, " · "), notice, "receipt for %s", run.Stage)
				}
				replay, err := model.FinishGachaRun(user.Id, run.ID, abandon, now)
				require.NoError(t, err)
				require.Equal(t, out.Revision, replay.Revision)
				require.Equal(t, want, replay.Save.Daily.Dispatch.Contract.Progress)
				require.Equal(t, want, gachaState(t, user.Id, now).Daily.Dispatch.Contract.Progress, "independent profile reload")
				t.Logf("clause=%s stage=%s abandon=%t survivors=%d/%d rounds=%d progress=%d", clause, run.Stage, abandon, out.AliveN, out.TotalN, out.Rounds, want)
				now = now.Add(5 * time.Minute)
				return out
			}
			prior := start("v1s1")
			day := state.Daily.Date
			_, err = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "contract_sign", ItemID: &day, GirlID: &leader, Choice: &clause}, now)
			require.NoError(t, err)
			_, err = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "dispatch_contract_claim", ItemID: &day}, now)
			require.ErrorIs(t, err, model.ErrGachaCultivationLocked)
			require.Equal(t, 3, finish(prior, false, 0).Stars, "earn the three stars for the later sweep")
			finish(start("res_manifest"), false, 0)
			finish(start("training"), false, 0)
			sweep := func(want int, notice string) {
				t.Helper()
				stage, count := "v1s1", 3
				intent := model.GachaActionIntent{RequestID: uuid.NewString(), Action: "voyage_sweep", ItemID: &stage, Count: &count, Choice: &day}
				before := gachaState(t, user.Id, now)
				out, err := model.ApplyGachaAction(user.Id, intent, now)
				require.NoError(t, err)
				require.Equal(t, want, out.Save.Daily.Dispatch.Contract.Progress)
				require.Contains(t, strings.Join(out.Drops, " · "), notice)
				require.Equal(t, before.Stamina-count*profile.Voyage.Stages[stage].Cost, out.Save.Stamina)
				replay, err := model.ApplyGachaAction(user.Id, intent, now)
				require.NoError(t, err)
				require.Equal(t, out.Revision, replay.Revision)
				reloaded := gachaState(t, user.Id, now)
				require.Equal(t, out.Save.Daily.Dispatch.Contract, reloaded.Daily.Dispatch.Contract)
				require.Equal(t, out.Save.Stamina, reloaded.Stamina)
				t.Logf("clause=%s stage=%s sweep=%d progress=%d", clause, stage, count, want)
				now = now.Add(5 * time.Minute)
			}
			setTeam([]string{"c07", leader})
			sweep(0, "签约角色不在出战队伍第一位")
			switch clause {
			case "courier":
				finish(start("v1s1"), false, 0, "签约角色不在出战队伍第一位")
				setTeam(trio)
				finish(start("v1s1"), false, 0, "静默护送最多两人出战")
				setTeam(duo)
				finish(start("v1s1"), true, 0, "本场未取胜或已退出")
				issued := start("v1s1")
				setTeam([]string{"c07", leader})
				first := finish(issued, false, 1)
				require.Equal(t, 2, first.TotalN)
				require.Equal(t, first.TotalN, first.AliveN)
				setTeam(duo)
				sweep(2, "委托 2/2")
			case "choir":
				setTeam(duo)
				finish(start("s1"), false, 0, "出战队伍不足三种元素")
				setTeam(trio)
				sweep(1, "委托 1/3")
				sweep(1, "本主线关卡已计入")
				finish(start("v1h1"), false, 2)
				out := finish(start("v1n1"), false, 3)
				require.Equal(t, []string{"v1s1", "v1h1", "v1n1"}, out.Save.Daily.Dispatch.Contract.Stages)
			case "breach":
				setTeam([]string{leader, "c07", "c09", "c03"})
				sweep(0, "须挑战元素裂隙")
				finish(start("v1s1"), false, 0)
				finish(start("r1"), true, 0)
				require.True(t, finish(start("r1"), false, 1).Fast)
			}
			before := gachaState(t, user.Id, now)
			claim := model.GachaActionIntent{RequestID: uuid.NewString(), Action: "dispatch_contract_claim", ItemID: &day}
			claimed, err := model.ApplyGachaAction(user.Id, claim, now)
			require.NoError(t, err)
			require.True(t, claimed.Save.Daily.Dispatch.Contract.Claimed)
			for _, rule := range profile.Dispatch.Contracts {
				if rule.ID == clause {
					for item, amount := range rule.Reward {
						require.Equal(t, before.Mats[item]+amount, claimed.Save.Mats[item])
					}
				}
			}
			replay, err := model.ApplyGachaAction(user.Id, claim, now)
			require.NoError(t, err)
			require.Equal(t, claimed.Revision, replay.Revision)
			claim.RequestID = uuid.NewString()
			_, err = model.ApplyGachaAction(user.Id, claim, now)
			require.ErrorIs(t, err, model.ErrGachaCultivationLocked)
			require.Equal(t, claimed.Save.Mats, gachaState(t, user.Id, now).Mats)
			if clause == "courier" {
				now = now.AddDate(0, 0, 1)
				day = gachaState(t, user.Id, now).Daily.Date
				weak := "c12"
				setTeam([]string{weak})
				_, err = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "contract_sign", ItemID: &day, GirlID: &weak, Choice: &clause}, now)
				require.NoError(t, err)
				require.Equal(t, 3, gachaState(t, user.Id, now).Stars["v1s1"])
				sweep(0, "本场未取胜或已退出")
			}
		})
	}
}

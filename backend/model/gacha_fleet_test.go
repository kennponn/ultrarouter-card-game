package model_test

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGachaFleetIntentShapes(t *testing.T) {
	charts, unknown := "charts", "balance"
	tier, invalid, yes := 1, 4, true
	for _, intent := range []model.GachaActionIntent{{Action: "guild_listing", Enabled: &yes}, {Action: "guild_supply", ItemID: &charts}, {Action: "guild_supply_claim", Count: &tier}} {
		intent.RequestID = uuid.NewString()
		require.NoError(t, model.ValidateGachaAction(intent))
		intent.Choice = &charts
		require.ErrorIs(t, model.ValidateGachaAction(intent), model.ErrGachaIntent)
	}
	for _, intent := range []model.GachaActionIntent{{Action: "guild_listing"}, {Action: "guild_listing", Enabled: &yes, ItemID: &charts}, {Action: "guild_supply", ItemID: &unknown}, {Action: "guild_supply", ItemID: &charts, Count: &tier}, {Action: "guild_supply_claim", Count: &invalid}} {
		intent.RequestID = uuid.NewString()
		require.ErrorIs(t, model.ValidateGachaAction(intent), model.ErrGachaIntent)
	}
}

func testGachaFleetDatabase(t *testing.T, now time.Time) {
	leader, member, other, lamp, observer := gachaUser(t), gachaUser(t), gachaUser(t), gachaUser(t), gachaUser(t)
	newIntent := func(action string) model.GachaActionIntent {
		return model.GachaActionIntent{RequestID: uuid.NewString(), Action: action}
	}
	item := func(action, id string) model.GachaActionIntent {
		intent := newIntent(action)
		intent.ItemID = &id
		return intent
	}
	apply := func(id int, intent model.GachaActionIntent) *model.GachaActionResult {
		t.Helper()
		result, err := model.ApplyGachaAction(id, intent, now)
		require.NoError(t, err)
		return result
	}
	board := func(id int, key string, start int, date time.Time) *model.GachaFleetBoard {
		t.Helper()
		result, err := model.GetGachaFleet(id, key, start, date)
		require.NoError(t, err)
		return result
	}
	findCode := func(view *model.GachaFleetBoard, code string) int {
		return slices.IndexFunc(view.Entries, func(entry model.GachaFleetEntry) bool { return entry.Code == code })
	}
	yes := true
	created := apply(leader.Id, item("guild_create", "冬港工坊"))
	code := created.Guild.Code
	create := item("guild_create", "夏夜灯塔")
	create.Enabled = &yes
	otherCode := apply(other.Id, create).Guild.Code
	assert.Equal(t, -1, findCode(board(leader.Id, "directory", leader.Id-1, now), code), "existing invitation-only fleets remain private")
	list := newIntent("guild_listing")
	list.Enabled = &yes
	apply(leader.Id, list)
	for _, id := range []int{member.Id, lamp.Id, observer.Id} {
		apply(id, item("guild_join", code))
	}
	list.RequestID = uuid.NewString()
	_, err := model.ApplyGachaAction(member.Id, list, now)
	require.ErrorIs(t, err, model.ErrGachaGuild)
	directory := board(member.Id, "directory", leader.Id-1, now)
	index := findCode(directory, code)
	require.NotEqual(t, -1, index)
	assert.Equal(t, 4, directory.Entries[index].Members)
	assert.True(t, directory.Entries[index].Mine)
	assert.NotContains(t, fmt.Sprint(directory.Entries), leader.Username)
	assert.Equal(t, -1, findCode(board(member.Id, "directory", leader.Id, now), code))
	chart := item("guild_supply", "charts")
	charted := apply(leader.Id, chart)
	assert.Equal(t, &model.GachaCaptain{Level: 1, XP: 5}, charted.Save.Captain)
	assert.Equal(t, 115, charted.Save.Stamina)
	assert.Equal(t, charted, apply(leader.Id, chart))
	wood := apply(other.Id, item("guild_supply", "timber"))
	assert.Equal(t, &model.GachaCaptain{Level: 1}, wood.Save.Captain)
	row := board(member.Id, "guild", 0, now)
	index = findCode(row, code)
	require.NotEqual(t, -1, index)
	assert.Equal(t, index+1, findCode(row, otherCode))
	assert.EqualValues(t, index+1, row.MyRank)
	assert.Equal(t, 2, row.MyValue)
	supply := item("guild_supply", "timber")
	first := apply(member.Id, supply)
	replayed := apply(member.Id, supply)
	assert.Equal(t, first.Save.Gold, replayed.Save.Gold)
	assert.Equal(t, first.Revision, replayed.Revision)
	assert.Equal(t, 1, first.Guild.Harbor.Lines[1].Done)
	_, err = model.ApplyGachaAction(member.Id, item("guild_supply", "charts"), now)
	require.ErrorIs(t, err, model.ErrGachaGuildLimit)
	_, err = model.ApplyGachaAction(lamp.Id, item("guild_supply", "lamps"), now)
	require.ErrorIs(t, err, model.ErrGachaFunds)
	state := gachaState(t, lamp.Id, now)
	assert.False(t, state.Daily.GuildSupplied)
	state.Mats["mat_r"] = 2
	seedGachaState(t, lamp.Id, state)
	apply(lamp.Id, item("guild_supply", "lamps"))
	claim, tier := newIntent("guild_supply_claim"), 1
	claim.Count = &tier
	_, err = model.ApplyGachaAction(observer.Id, claim, now)
	require.ErrorIs(t, err, model.ErrGachaGuildLimit)
	awarded := apply(member.Id, claim)
	assert.Equal(t, first.Save.Gold+100, awarded.Save.Gold)
	assert.Equal(t, first.Save.Mats["forge_ore"]+2, awarded.Save.Mats["forge_ore"])
	if awarded.Save.Mailbox != nil {
		assert.False(t, slices.ContainsFunc(awarded.Save.Mailbox.Messages, func(message *model.GachaMailMessage) bool {
			return message.ID == "action:"+claim.RequestID
		}), "active claims must not create a second mailbox grant")
	}
	assert.True(t, awarded.Guild.Harbor.Beacons[0].Claimed)
	assert.Equal(t, awarded.Revision, apply(member.Id, claim).Revision)
	claim.RequestID = uuid.NewString()
	_, err = model.ApplyGachaAction(member.Id, claim, now)
	require.ErrorIs(t, err, model.ErrGachaGuildLimit)
	apply(member.Id, newIntent("guild_leave"))
	joined := apply(member.Id, item("guild_join", otherCode))
	assert.True(t, joined.Guild.Harbor.Beacons[0].Claimed)
	assert.Zero(t, joined.Guild.Harbor.MyCargo)
	_, err = model.ApplyGachaAction(member.Id, item("guild_supply", "charts"), now)
	require.ErrorIs(t, err, model.ErrGachaGuildLimit)
	future := now.AddDate(0, 0, 7)
	assert.Zero(t, board(other.Id, "guild", 0, future).MyRank)
	profile, err := model.GetGachaProfile(member.Id, future)
	require.NoError(t, err)
	assert.Zero(t, profile.Guild.Harbor.Score)
	assert.False(t, profile.Guild.Harbor.Beacons[0].Claimed)
	assert.False(t, profile.Guild.Harbor.Supplied)

	displays := []string{fmt.Sprintf("藏家甲%d", leader.Id), fmt.Sprintf("藏家乙%d", other.Id)}
	for index, id := range []int{leader.Id, other.Id} {
		require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", id).Update("display_name", displays[index]).Error)
		state := gachaState(t, id, now)
		for seq := 1; seq <= 27; seq++ {
			state.Girls[fmt.Sprintf("c%02d", seq)] = &model.GachaOwnedGirl{Star: 1, Level: 1}
		}
		state.Girls["c28"] = &model.GachaOwnedGirl{Shards: 1}
		state.Squad = []string{}
		if id == other.Id {
			state.Squad = []string{"c06", "c02"}
		}
		state.Arena.Wins, state.Arena.Played = 4+index, 6+index
		state.Stars["s1"], state.Stars["v2h1"], state.Stars["v3n1"], state.Stars["v4s5"] = 3, 2, 1, 3
		for _, id := range []string{"s1", "v2h1", "v3n1", "v4s5"} {
			state.Cleared[id] = true
		}
		state.Stars["res_manifest"], state.Stars["rift_fire"], state.Stars["not_a_stage"] = 3, 3, 3
		seedGachaState(t, id, state)
		gachaState(t, id, now)
	}
	collection := board(leader.Id, "collection", 0, now)
	index = slices.IndexFunc(collection.Entries, func(entry model.GachaFleetEntry) bool { return entry.Mine })
	require.GreaterOrEqual(t, index, 0)
	require.Less(t, index+1, len(collection.Entries))
	assert.Equal(t, displays[1], collection.Entries[index+1].Name)
	assert.Equal(t, 28, collection.CatalogSize)
	assert.Equal(t, 27, collection.MyValue)
	assert.EqualValues(t, index+1, collection.MyRank)
	assert.Zero(t, board(leader.Id, "power", 0, now).MyRank)
	stars := board(leader.Id, "stars", 0, now)
	assert.Equal(t, 156, stars.StarLimit)
	assert.Equal(t, 9, stars.MyValue)
	index = slices.IndexFunc(stars.Entries, func(entry model.GachaFleetEntry) bool { return entry.Mine })
	require.GreaterOrEqual(t, index, 0)
	require.Less(t, index+1, len(stars.Entries))
	assert.EqualValues(t, index+1, stars.MyRank)
	assert.Equal(t, displays[0], stars.MyName)
	assert.Equal(t, displays[1], stars.Entries[index+1].Name)
	assert.Equal(t, []model.GachaPublicGirl{{ID: "c06", Star: 1, Level: 1}, {ID: "c02", Star: 1, Level: 1}}, stars.Entries[index+1].Team)
	assert.Equal(t, 5, board(other.Id, "arena", 0, now).MyValue)
	public, err := common.Marshal(collection)
	require.NoError(t, err)
	for _, field := range []string{"state", "members", "user_id", "email", "quota", "squad", "username", "code"} {
		assert.NotContains(t, string(public), `"`+field+`":`)
	}
	var before, after, healthyMember model.GachaPlayer
	require.NoError(t, model.DB.First(&before, "user_id = ?", other.Id).Error)
	require.NoError(t, model.DB.First(&healthyMember, "user_id = ?", member.Id).Error)
	t.Cleanup(func() {
		assert.NoError(t, model.DB.Model(&healthyMember).Select("state", "power", "fleet_indexed", "fleet_edition", "fleet_stars", "fleet_wins", "collection_count", "fleet_members", "fleet_week", "fleet_score", "fleet_name").UpdateColumns(&healthyMember).Error)
	})
	staleName := fmt.Sprintf("航海员 %04X", member.Id)
	for pass, invalidState := range []string{`{"girls":`, `null`} {
		require.NoError(t, model.DB.Model(&model.GachaPlayer{}).Where("user_id = ?", member.Id).UpdateColumns(map[string]any{
			"state": invalidState, "fleet_indexed": pass == 1, "fleet_edition": 0, "fleet_stars": 156, "fleet_wins": 999, "collection_count": 28, "power": 100000000,
			"fleet_members": 2, "fleet_score": 999, "fleet_week": row.Week, "fleet_name": staleName,
		}).Error)
		var held, checked model.GachaPlayer
		require.NoError(t, model.DB.First(&held, "user_id = ?", member.Id).Error)
		require.NoError(t, model.DB.Model(&model.GachaPlayer{}).Where("user_id = ?", other.Id).UpdateColumns(map[string]any{"fleet_indexed": pass == 1, "fleet_edition": 0, "fleet_stars": 0, "fleet_wins": 0, "fleet_portrait": "", "collection_count": 0, "fleet_members": 0}).Error)
		require.NoError(t, model.InitializeGachaPower())
		require.NoError(t, model.InitializeGachaPower())
		require.NoError(t, model.DB.First(&checked, "user_id = ?", member.Id).Error)
		assert.True(t, held == checked, "an unreadable record must remain untouched for recovery")
		for _, key := range []string{"directory", "guild", "collection", "power", "stars", "arena"} {
			start := 0
			if key == "directory" {
				start = member.Id - 1
			}
			entries := board(leader.Id, key, start, now).Entries
			assert.False(t, slices.ContainsFunc(entries, func(entry model.GachaFleetEntry) bool { return entry.Name == staleName }), "unindexed record on %s", key)
		}
		assert.NotEqual(t, -1, findCode(board(leader.Id, "directory", member.Id-1, now), otherCode))
	}
	require.NoError(t, model.DB.First(&after, "user_id = ?", other.Id).Error)
	assert.Equal(t, before.State, after.State)
	assert.Equal(t, before.Revision, after.Revision)
	assert.Equal(t, before.UpdatedAt, after.UpdatedAt)
	assert.Equal(t, 27, after.CollectionCount)
	assert.Equal(t, 2, after.FleetMembers)
	assert.Greater(t, after.FleetEdition, 0)
	assert.Equal(t, 9, after.FleetStars)
	assert.Equal(t, 5, after.FleetWins)
	assert.Equal(t, before.FleetPortrait, after.FleetPortrait)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", other.Id).Update("status", common.UserStatusDisabled).Error)
	assert.Equal(t, -1, findCode(board(leader.Id, "directory", leader.Id-1, now), otherCode))
	assert.False(t, slices.ContainsFunc(board(leader.Id, "collection", 0, now).Entries, func(entry model.GachaFleetEntry) bool { return entry.Name == displays[1] }))
	assert.False(t, slices.ContainsFunc(board(leader.Id, "stars", 0, now).Entries, func(entry model.GachaFleetEntry) bool { return entry.Name == displays[1] }))
	_, err = model.GetGachaFleet(other.Id, "power", 0, now)
	assert.Error(t, err)
	t.Run("member_sees_guild_standing_beyond_top_fifty", func(t *testing.T) {
		state := gachaState(t, leader.Id, now)
		state.Captain.Level = 17
		seedGachaState(t, leader.Id, state)
		gachaState(t, leader.Id, now)
		var owner model.GachaPlayer
		require.NoError(t, model.DB.First(&owner, "user_id = ?", leader.Id).Error)
		baseline := board(observer.Id, "guild", 0, now)
		require.Positive(t, baseline.MyRank)
		var ahead []model.GachaPlayer
		var ids []int
		for range 50 {
			user := gachaUser(t)
			ids = append(ids, user.Id)
			guildState, err := common.Marshal(model.GachaGameState{Guild: &model.GachaGuildState{
				Code: fmt.Sprintf("%08X", user.Id), Name: fmt.Sprintf("灯塔%04X", user.Id), LeaderID: user.Id,
				Members: []model.GachaGuildMember{{UserID: user.Id, Role: "leader"}},
				Week:    owner.FleetWeek, BossHP: 24000, BossMaxHP: 24000, RaidVersion: 1, Listed: true,
			}})
			require.NoError(t, err)
			ahead = append(ahead, model.GachaPlayer{UserID: user.Id, State: model.GachaDocument(guildState), Revision: 1, FleetIndexed: true, FleetEdition: owner.FleetEdition,
				FleetLeader: user.Id, FleetMembers: 1, FleetName: fmt.Sprintf("灯塔%04X", user.Id), FleetWeek: owner.FleetWeek, FleetScore: owner.FleetScore + 1})
		}
		require.NoError(t, model.DB.CreateInBatches(&ahead, 20).Error)
		t.Cleanup(func() { assert.NoError(t, model.DB.Where("user_id IN ?", ids).Delete(&model.GachaPlayer{}).Error) })
		standing := board(observer.Id, "guild", 0, now)
		require.Len(t, standing.Entries, 50)
		assert.Equal(t, -1, findCode(standing, code))
		assert.Equal(t, baseline.MyRank+50, standing.MyRank)
		assert.Equal(t, owner.FleetScore, standing.MyValue)
		assert.Equal(t, created.Guild.Name, standing.MyName)
		assert.Equal(t, 17, standing.MyLevel)
		unranked := board(observer.Id, "guild", 0, now.AddDate(0, 0, 7))
		assert.Zero(t, unranked.MyRank)
		assert.Empty(t, unranked.MyName)
		assert.Zero(t, unranked.MyLevel)
	})
	t.Run("guild_kick_updates_roster_and_membership_atomically", func(t *testing.T) { testGachaGuildKickDatabase(t, now) })
}

func testGachaGuildKickDatabase(t *testing.T, now time.Time) {
	// Deliberately create the target before the leader and the deputy afterward
	// so the transaction must lock all three rows in a non-role-based order.
	target, leader, deputy, peer, elder, other := gachaUser(t), gachaUser(t), gachaUser(t), gachaUser(t), gachaUser(t), gachaUser(t)
	intent := func(action string, targetID int, item string) model.GachaActionIntent {
		request := model.GachaActionIntent{RequestID: uuid.NewString(), Action: action}
		if targetID > 0 {
			request.OpponentID = &targetID
		}
		if item != "" {
			request.ItemID = &item
		}
		return request
	}
	apply := func(id int, request model.GachaActionIntent) *model.GachaActionResult {
		t.Helper()
		result, err := model.ApplyGachaAction(id, request, now)
		require.NoError(t, err)
		return result
	}
	code := apply(leader.Id, intent("guild_create", 0, "移员测试港")).Guild.Code
	otherCode := apply(other.Id, intent("guild_create", 0, "另一测试港")).Guild.Code
	for _, id := range []int{target.Id, deputy.Id, peer.Id, elder.Id} {
		apply(id, intent("guild_join", 0, code))
	}
	apply(leader.Id, intent("guild_role", deputy.Id, "deputy"))
	apply(leader.Id, intent("guild_role", peer.Id, "deputy"))
	apply(leader.Id, intent("guild_role", elder.Id, "elder"))
	for _, blocked := range [][2]int{{deputy.Id, leader.Id}, {deputy.Id, peer.Id}, {elder.Id, target.Id}, {leader.Id, leader.Id}} {
		_, err := model.ApplyGachaAction(blocked[0], intent("guild_kick", blocked[1], ""), now)
		require.ErrorIs(t, err, model.ErrGachaGuild)
	}
	saved := gachaState(t, target.Id, now)
	saved.GuildProgress.Coins, saved.GuildProgress.Claims, saved.GuildProgress.Attempts = 515, 31, 2
	saved.GuildProgress.Bought["ink"] = 3
	saved.GuildProgress.Participation[code] = 31
	seedGachaState(t, target.Id, saved)
	kick := intent("guild_kick", target.Id, "")
	result := apply(deputy.Id, kick)
	assert.False(t, slices.ContainsFunc(result.Guild.Members, func(member model.GachaGuildMember) bool { return member.UserID == target.Id }))
	after := gachaState(t, target.Id, now)
	assert.Nil(t, after.Guild)
	assert.Equal(t, saved.GuildProgress, after.GuildProgress)
	apply(target.Id, intent("guild_join", 0, code))
	apply(deputy.Id, kick)
	assert.Equal(t, code, gachaState(t, target.Id, now).Guild.Code, "replaying an old successful kick cannot kick a rejoined player again")
	apply(target.Id, intent("guild_leave", 0, ""))
	apply(target.Id, intent("guild_join", 0, otherCode))
	_, err := model.ApplyGachaAction(deputy.Id, intent("guild_kick", target.Id, ""), now)
	require.ErrorIs(t, err, model.ErrGachaGuild)
	assert.Equal(t, otherCode, gachaState(t, target.Id, now).Guild.Code, "a stale request cannot clear another guild's membership")
	apply(deputy.Id, intent("guild_kick", elder.Id, ""))
	assert.Nil(t, gachaState(t, elder.Id, now).Guild)
	apply(leader.Id, intent("guild_kick", deputy.Id, ""))
	assert.Nil(t, gachaState(t, deputy.Id, now).Guild)
}

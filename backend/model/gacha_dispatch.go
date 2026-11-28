package model

import (
	_ "embed"
	"fmt"
	"hash/fnv"
	"math/rand/v2"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

type GachaSealedRelic struct {
	ID      string         `json:"id"`
	Name    string         `json:"name"`
	Quality string         `json:"quality"`
	Case    string         `json:"case"`
	Ending  string         `json:"ending"`
	Cost    map[string]int `json:"cost"`
	Gain    string         `json:"gain"`
	Loss    string         `json:"loss"`
	Value   int            `json:"value"`
	Price   int            `json:"price"`
}
type GachaDispatchClause struct {
	ID     string         `json:"id"`
	Name   string         `json:"name"`
	Rule   string         `json:"rule"`
	Target int            `json:"target"`
	Reward map[string]int `json:"reward"`
}
type GachaTrailTile struct {
	Name  string `json:"name"`
	Fuel  int    `json:"fuel"`
	Light int    `json:"light"`
	Cargo int    `json:"cargo"`
}
type GachaMirrorScene struct {
	Name     string   `json:"name"`
	Rule     string   `json:"rule"`
	Goal     string   `json:"goal"`
	Enemy    string   `json:"enemy"`
	Mechanic string   `json:"mechanic"`
	Level    int      `json:"level"`
	Enemies  int      `json:"enemies"`
	Pool     []string `json:"pool"`
}
type GachaDispatchRules struct {
	SweepReports bool                      `json:"sweep_reports"`
	Relics       []GachaSealedRelic        `json:"relics"`
	Symbols      map[string]string         `json:"symbols"`
	ClockTries   int                       `json:"clock_tries"`
	ClockReward  map[string]int            `json:"clock_reward"`
	Contracts    []GachaDispatchClause     `json:"contracts"`
	Tiles        map[string]GachaTrailTile `json:"tiles"`
	MosaicReward map[string]int            `json:"mosaic_reward"`
	Mirrors      []GachaMirrorScene        `json:"mirrors"`
	MirrorReward map[string]int            `json:"mirror_reward"`
}

//go:embed gacha_dispatch.json
var gachaDispatchJSON []byte
var gachaDispatch = func() GachaDispatchRules {
	var rules GachaDispatchRules
	if err := common.Unmarshal(gachaDispatchJSON, &rules); err != nil {
		panic(err)
	}
	for _, relic := range rules.Relics {
		if _, ok := gachaItems.Qualities[relic.Quality]; !ok || gachaCases.Files[relic.Case].Name == "" || gachaCases.Endings[relic.Ending] == "" || len(relic.Cost) == 0 || relic.Value < 1 || relic.Price < 1 || relic.Price > 100 {
			panic("invalid dispatch relic")
		}
		for id, amount := range relic.Cost {
			if gachaCases.Evidence[id] == "" || amount < 1 {
				panic("invalid dispatch evidence cost")
			}
		}
	}
	for _, mirror := range rules.Mirrors {
		if len(mirror.Pool) != 6 || !slices.Contains([]string{"survive", "rush", "cast"}, mirror.Goal) {
			panic("invalid dispatch mirror")
		}
		for index, id := range mirror.Pool {
			if gachaGirl(id) == nil || slices.Contains(mirror.Pool[:index], id) {
				panic("invalid dispatch loaner")
			}
		}
	}
	if len(rules.Mirrors) == 0 || rules.ClockTries != 2 {
		panic("invalid dispatch desks")
	}
	return rules
}()

type GachaDispatchState struct {
	Relics map[string]bool `json:"relics,omitempty"`
	Active string          `json:"active,omitempty"`
	Mosaic GachaMosaic     `json:"mosaic"`
}
type GachaDispatchDay struct {
	Clock    GachaFogClock      `json:"clock"`
	Route    GachaFogRoute      `json:"route"`
	Mirror   GachaMirrorDesk    `json:"mirror"`
	Contract *GachaSignedClause `json:"contract,omitempty"`
}
type GachaClockHint struct {
	A  string `json:"a"`
	Op string `json:"op"`
	B  string `json:"b,omitempty"`
}
type GachaFogClock struct {
	Hints   []GachaClockHint `json:"hints"`
	Guesses []string         `json:"guesses"`
	Solved  bool             `json:"solved"`
}
type GachaFogRoute struct {
	Tiles  []string `json:"tiles"`
	Track  []int    `json:"track"`
	Light  int      `json:"light"`
	Cargo  int      `json:"cargo"`
	Status string   `json:"status"`
}
type GachaSignedClause struct {
	ID       string   `json:"id"`
	Leader   string   `json:"leader"`
	Progress int      `json:"progress"`
	Claimed  bool     `json:"claimed"`
	Stages   []string `json:"stages,omitempty"`
}
type GachaMosaic struct {
	Week    string `json:"week"`
	Cleared int    `json:"cleared"`
	Marks   []int  `json:"marks"`
}
type GachaMirrorDesk struct {
	Scene    int             `json:"scene"`
	Roster   []string        `json:"roster"`
	Attempts int             `json:"attempts"`
	Won      bool            `json:"won"`
	Last     *GachaMirrorLog `json:"last,omitempty"`
}
type GachaMirrorLog struct {
	Squad  []string `json:"squad"`
	Choice string   `json:"choice"`
	Win    bool     `json:"win"`
	Alive  int      `json:"alive"`
	Rounds int      `json:"rounds"`
	Casts  int      `json:"casts"`
	Notes  []string `json:"notes"`
}

func dispatchSeed(parts ...string) uint64 {
	digest := fnv.New64a()
	_, _ = digest.Write([]byte(strings.Join(parts, "/")))
	return digest.Sum64()
}
func dispatchRNG(seed uint64) *rand.Rand { return rand.New(rand.NewPCG(seed, 0xcb8634b579e3f5ad)) }
func dispatchRelic(id string) *GachaSealedRelic {
	for index := range gachaDispatch.Relics {
		if gachaDispatch.Relics[index].ID == id {
			return &gachaDispatch.Relics[index]
		}
	}
	return nil
}
func dispatchClause(id string) *GachaDispatchClause {
	for index := range gachaDispatch.Contracts {
		if gachaDispatch.Contracts[index].ID == id {
			return &gachaDispatch.Contracts[index]
		}
	}
	return nil
}

var dispatchActions = []string{"relic_bind", "relic_wear", "relic_store", "clock_align", "contract_sign", "dispatch_contract_claim", "route_step", "route_bank", "mosaic_place", "mirror_trial"}
var clockOrders = []string{"ash-moon-tide", "ash-tide-moon", "moon-ash-tide", "moon-tide-ash", "tide-ash-moon", "tide-moon-ash"}

func (hint GachaClockHint) fits(order string) bool {
	marks := strings.Split(order, "-")
	first, second := slices.Index(marks, hint.A), slices.Index(marks, hint.B)
	if first < 0 {
		return false
	}
	switch hint.Op {
	case "before":
		return second >= 0 && first < second
	case "next":
		return second >= 0 && first == second+1
	case "apart":
		return second >= 0 && (first-second == 2 || second-first == 2)
	case "notfirst":
		return first != 0
	case "notlast":
		return first != 2
	}
	return false
}
func (clock *GachaFogClock) accepts(order string) bool {
	return slices.Contains(clockOrders, order) && len(clock.Hints) > 0 && !slices.ContainsFunc(clock.Hints, func(hint GachaClockHint) bool { return !hint.fits(order) })
}
func dispatchClock(rng *rand.Rand) GachaFogClock {
	answer := clockOrders[rng.IntN(len(clockOrders))]
	options := slices.Clone(clockOrders)
	hints := []GachaClockHint{}
	for _, first := range strings.Split(answer, "-") {
		for _, last := range strings.Split(answer, "-") {
			if first != last {
				for _, op := range []string{"before", "next", "apart"} {
					hints = append(hints, GachaClockHint{first, op, last})
				}
			}
		}
		hints = append(hints, GachaClockHint{A: first, Op: "notfirst"}, GachaClockHint{A: first, Op: "notlast"})
	}
	rng.Shuffle(len(hints), func(first, second int) { hints[first], hints[second] = hints[second], hints[first] })
	clock := GachaFogClock{Guesses: []string{}}
	for _, hint := range hints {
		if !hint.fits(answer) {
			continue
		}
		prior := len(options)
		options = slices.DeleteFunc(options, func(order string) bool { return !hint.fits(order) })
		if len(options) < prior {
			clock.Hints = append(clock.Hints, hint)
		}
		if len(options) == 1 {
			break
		}
	}
	return clock
}
func syncGachaDispatch(state *GachaGameState, userID int, now time.Time) error {
	if state.Dispatch == nil {
		state.Dispatch = &GachaDispatchState{}
	}
	book := state.Dispatch
	if book.Active != "" && !book.Relics[book.Active] {
		return ErrGachaRunState
	}
	for id, owned := range book.Relics {
		if !owned || dispatchRelic(id) == nil {
			return ErrGachaRunState
		}
	}
	week := gachaWeekKey(now)
	if book.Mosaic.Week != week {
		book.Mosaic = GachaMosaic{Week: week, Marks: dispatchMosaicMarks(week, 0)}
	}
	if book.Mosaic.Cleared < 0 || book.Mosaic.Cleared > 3 || !slices.Equal(book.Mosaic.Marks, dispatchMosaicMarks(week, book.Mosaic.Cleared)) {
		return ErrGachaRunState
	}
	if state.Daily.Dispatch == nil {
		rng := dispatchRNG(dispatchSeed("desk", state.Daily.Date, fmt.Sprint(userID)))
		tiles := make([]string, 0, 12)
		for row := range 4 {
			line := []string{"spring", "cache", "fog"}
			if row%2 == 1 {
				line[1] = "vault"
				line[2] = "beacon"
			}
			rng.Shuffle(3, func(first, second int) { line[first], line[second] = line[second], line[first] })
			tiles = append(tiles, line...)
		}
		scene := int(dispatchSeed("mirror", state.Daily.Date) % uint64(len(gachaDispatch.Mirrors)))
		state.Daily.Dispatch = &GachaDispatchDay{Clock: dispatchClock(rng), Route: GachaFogRoute{Tiles: tiles, Track: []int{}, Light: 5, Status: "open"}, Mirror: GachaMirrorDesk{Scene: scene, Roster: slices.Clone(gachaDispatch.Mirrors[scene].Pool)}}
	}
	day := state.Daily.Dispatch
	clock := day.Clock
	solutionCount := 0
	for _, order := range clockOrders {
		if clock.accepts(order) {
			solutionCount++
		}
	}
	if solutionCount != 1 || len(clock.Guesses) > gachaDispatch.ClockTries || day.Mirror.Scene < 0 || day.Mirror.Scene >= len(gachaDispatch.Mirrors) || !slices.Equal(day.Mirror.Roster, gachaDispatch.Mirrors[day.Mirror.Scene].Pool) || day.Mirror.Attempts < 0 || day.Mirror.Attempts > 3 {
		return ErrGachaRunState
	}
	solved := false
	for index, guess := range clock.Guesses {
		if solved || !slices.Contains(clockOrders, guess) || slices.Contains(clock.Guesses[:index], guess) {
			return ErrGachaRunState
		}
		solved = clock.accepts(guess)
	}
	if solved != clock.Solved || !validDispatchRoute(day.Route) {
		return ErrGachaRunState
	}
	if signed := day.Contract; signed != nil {
		rule := dispatchClause(signed.ID)
		if rule == nil || !gachaCardOwned(state.Girls[signed.Leader]) || signed.Progress < 0 || signed.Progress > rule.Target || signed.Claimed && signed.Progress != rule.Target || signed.ID == "choir" && len(signed.Stages) != signed.Progress {
			return ErrGachaRunState
		}
	}
	return nil
}
func validateGachaDispatchIntent(intent GachaActionIntent) error {
	if intent.Enabled != nil || intent.Materials != nil || intent.Config != nil || intent.RoomID != nil || intent.QueueID != nil || intent.StakeE != nil || intent.OpponentID != nil || intent.Action != "mirror_trial" && intent.Squad != nil || intent.Action != "contract_sign" && intent.GirlID != nil {
		return ErrGachaIntent
	}
	if intent.Action == "relic_store" && intent.ItemID == nil && intent.Choice == nil && intent.Count == nil {
		return nil
	}
	if intent.ItemID == nil {
		return ErrGachaIntent
	}
	id := *intent.ItemID
	if intent.Action == "relic_bind" || intent.Action == "relic_wear" {
		if dispatchRelic(id) != nil && intent.Choice == nil && intent.Count == nil {
			return nil
		}
		return ErrGachaIntent
	}
	if intent.Action == "mosaic_place" {
		if validGachaWeek(id) && intent.Choice != nil && len(*intent.Choice) == 11 && intent.Count != nil && *intent.Count >= 1 && *intent.Count <= 3 {
			return nil
		}
		return ErrGachaIntent
	}
	date, err := time.Parse("2006-01-02", id)
	if err != nil || date.Format("2006-01-02") != id {
		return ErrGachaIntent
	}
	switch intent.Action {
	case "contract_sign":
		if intent.Choice != nil && dispatchClause(*intent.Choice) != nil && intent.Count == nil && intent.GirlID != nil && gachaGirl(*intent.GirlID) != nil {
			return nil
		}
	case "dispatch_contract_claim":
		if intent.Choice == nil && intent.Count == nil {
			return nil
		}
	case "clock_align":
		if intent.Choice != nil && slices.Contains(clockOrders, *intent.Choice) && intent.Count != nil && *intent.Count >= 1 && *intent.Count <= gachaDispatch.ClockTries {
			return nil
		}
	case "route_step":
		if intent.Count != nil && *intent.Count >= 1 && *intent.Count <= 4 && intent.Choice != nil && slices.Contains([]string{"left", "middle", "right"}, *intent.Choice) {
			return nil
		}
	case "route_bank":
		if intent.Choice == nil && intent.Count != nil && *intent.Count >= 1 && *intent.Count <= 3 {
			return nil
		}
	case "mirror_trial":
		if intent.Choice != nil && slices.Contains([]string{"ward", "spark", "rush"}, *intent.Choice) && intent.Count != nil && *intent.Count >= 1 && *intent.Count <= 3 && intent.Squad != nil && len(*intent.Squad) == 2 && (*intent.Squad)[0] != (*intent.Squad)[1] && gachaGirl((*intent.Squad)[0]) != nil && gachaGirl((*intent.Squad)[1]) != nil {
			return nil
		}
	}
	return ErrGachaIntent
}
func grantGachaDispatch(state *GachaGameState, reward map[string]int, result *GachaActionResult) error {
	ids := make([]string, 0, len(reward))
	for id, count := range reward {
		if _, ok := gachaItems.Items[id]; !ok || count < 0 {
			return ErrGachaRunState
		}
		if state.Mats[id] > gachaMaxResource-count {
			return ErrGachaCapacity
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		state.Mats[id] += reward[id]
		result.Drops = append(result.Drops, fmt.Sprintf("%s ×%d", gachaItems.Items[id].Name, reward[id]))
	}
	return nil
}
func applyGachaDispatch(state *GachaGameState, intent GachaActionIntent, result *GachaActionResult) error {
	if err := validateGachaDispatchIntent(intent); err != nil {
		return err
	}
	book, day := state.Dispatch, state.Daily.Dispatch
	if book == nil || day == nil {
		return ErrGachaRunState
	}
	if intent.Action == "relic_store" {
		if book.Active == "" {
			return ErrGachaCultivationLocked
		}
		book.Active = ""
		result.Drops = append(result.Drops, "遗物已归柜，下次开战生效")
		return nil
	}
	id := *intent.ItemID
	if strings.HasPrefix(intent.Action, "relic_") {
		relic := dispatchRelic(id)
		if intent.Action == "relic_wear" {
			if !book.Relics[id] || book.Active == id {
				return ErrGachaCultivationLocked
			}
			book.Active = id
			result.Drops = append(result.Drops, "领队携行 · "+relic.Name)
			return nil
		}
		if book.Relics[id] || gachaCaseEndingCount(state, relic.Case, relic.Ending) == 0 {
			return ErrGachaCultivationLocked
		}
		evidence := gachaEvidence(state)
		for key, amount := range relic.Cost {
			if evidence[key] < amount {
				return ErrGachaFunds
			}
		}
		for key, amount := range relic.Cost {
			evidence[key] -= amount
		}
		if book.Relics == nil {
			book.Relics = map[string]bool{}
		}
		book.Relics[id] = true
		result.Drops = append(result.Drops, "封存成功 · "+relic.Name)
		return nil
	}
	if intent.Action == "mosaic_place" {
		return applyGachaMosaic(state, intent, result)
	}
	if id != state.Daily.Date {
		return ErrGachaCultivationLocked
	}
	switch intent.Action {
	case "clock_align":
		clock := &day.Clock
		guess := *intent.Choice
		if clock.Solved || len(clock.Guesses)+1 != *intent.Count || slices.Contains(clock.Guesses, guess) {
			return ErrGachaCultivationLocked
		}
		if clock.accepts(guess) {
			if err := grantGachaDispatch(state, gachaDispatch.ClockReward, result); err != nil {
				return err
			}
			clock.Solved = true
			result.Drops = append(result.Drops, "钟声合拍，校时完成")
		} else {
			result.Drops = append(result.Drops, "齿印未能咬合；请重新核对线索")
		}
		clock.Guesses = append(clock.Guesses, guess)
	case "contract_sign":
		if day.Contract != nil || !gachaCardOwned(state.Girls[*intent.GirlID]) {
			return ErrGachaCultivationLocked
		}
		day.Contract = &GachaSignedClause{ID: *intent.Choice, Leader: *intent.GirlID}
		result.Drops = append(result.Drops, "已立约 · "+dispatchClause(*intent.Choice).Name)
	case "dispatch_contract_claim":
		signed := day.Contract
		if signed == nil || signed.Claimed || signed.Progress != dispatchClause(signed.ID).Target {
			return ErrGachaCultivationLocked
		}
		if err := grantGachaDispatch(state, dispatchClause(signed.ID).Reward, result); err != nil {
			return err
		}
		signed.Claimed = true
	case "route_step", "route_bank":
		return applyGachaRoute(state, intent, result)
	case "mirror_trial":
		return applyGachaMirror(state, intent, result)
	default:
		return ErrGachaIntent
	}
	return nil
}

func issueGachaDispatch(state *GachaGameState, stage *GachaStage, snapshot *GachaRunSnapshot) {
	if state.Daily.Dispatch == nil || stage == nil {
		return
	}
	signed := state.Daily.Dispatch.Contract
	if signed == nil || signed.Claimed || signed.Progress >= dispatchClause(signed.ID).Target {
		return
	}
	snapshot.ContractDay = state.Daily.Date
	snapshot.ContractID = signed.ID
}

func dispatchEntryBlock(signed *GachaSignedClause, stage *GachaStage, snapshot *GachaRunSnapshot) string {
	if snapshot.Mode != "" {
		return "特殊玩法不是委托航线"
	}
	if len(snapshot.Squad) == 0 || snapshot.Squad[0] != signed.Leader {
		return "签约角色不在出战队伍第一位"
	}
	point := gachaVoyage.Stages[stage.ID]
	campaign := point != nil && (point.Kind == "story" || point.Kind == "hard" || point.Kind == "nightmare") && stage.RelicTier == 0
	switch signed.ID {
	case "courier":
		if len(snapshot.Squad) > 2 {
			return "静默护送最多两人出战"
		}
		if !campaign {
			return "须挑战主线，资源本和裂隙不计"
		}
	case "breach":
		if stage.RelicTier <= 0 {
			return "须挑战元素裂隙"
		}
	case "choir":
		if !campaign {
			return "须挑战主线，资源本和裂隙不计"
		}
		elements := map[string]bool{}
		for _, id := range snapshot.Squad {
			if girl := gachaGirl(id); girl != nil {
				elements[girl.Elem] = true
			}
		}
		if len(elements) < 3 {
			return "出战队伍不足三种元素"
		}
		if slices.Contains(signed.Stages, stage.ID) {
			return "本主线关卡已计入，须换一个关卡"
		}
	default:
		return "未知委托"
	}
	return ""
}

func settleGachaDispatch(state *GachaGameState, stage *GachaStage, snapshot *GachaRunSnapshot, report *GachaBattleReport, result *GachaFinishResult) {
	if state.Daily.Dispatch == nil || snapshot.ContractDay != state.Daily.Date || snapshot.ContractID == "" {
		return
	}
	signed := state.Daily.Dispatch.Contract
	if signed == nil || signed.ID != snapshot.ContractID || signed.Claimed || signed.Progress >= dispatchClause(signed.ID).Target {
		return
	}
	reason := dispatchEntryBlock(signed, stage, snapshot)
	if reason == "" {
		switch {
		case !result.Win:
			reason = "本场未取胜或已退出"
		case signed.ID == "courier" && (report.TotalN != len(snapshot.Squad) || report.AliveN != report.TotalN):
			reason = "须让开局的全体队员存活至胜利"
		case signed.ID == "breach" && (report.Rounds < 1 || report.Rounds > stage.StarRound):
			reason = "未在本关速通回合内取胜"
		}
	}
	if reason != "" {
		result.Drops = append(result.Drops, "委托未计入："+reason)
		return
	}
	if signed.ID == "choir" {
		signed.Stages = append(signed.Stages, stage.ID)
	}
	signed.Progress++
	result.Drops = append(result.Drops, fmt.Sprintf("%s · 委托 %d/%d", dispatchClause(signed.ID).Name, signed.Progress, dispatchClause(signed.ID).Target))
}

func verifyGachaDispatchSweep(state *GachaGameState, point *GachaVoyagePoint, count int) ([]string, error) {
	if !gachaDispatch.SweepReports {
		return nil, nil
	}
	snapshot := GachaRunSnapshot{Squad: slices.Clone(state.Squad)}
	issueGachaDispatch(state, &point.GachaStage, &snapshot)
	if snapshot.ContractID == "" {
		return nil, nil
	}
	signed := state.Daily.Dispatch.Contract
	if reason := dispatchEntryBlock(signed, &point.GachaStage, &snapshot); reason != "" {
		return []string{"委托未计入：" + reason}, nil
	}
	var notices []string
	for range count {
		report, err := SimulateGachaCombat(state, point.GachaStage)
		if err != nil {
			return nil, err
		}
		finish := GachaFinishResult{Win: report.Win}
		progress := signed.Progress
		settleGachaDispatch(state, &point.GachaStage, &snapshot, report, &finish)
		notices = append(notices, finish.Drops...)
		if signed.Progress == dispatchClause(signed.ID).Target || signed.ID == "choir" && signed.Progress > progress {
			break
		}
	}
	return notices, nil
}
func applyGachaSealedRelic(state *GachaGameState, units []*gachaCombatUnit) {
	if state.Dispatch == nil || len(units) == 0 {
		return
	}
	relic := dispatchRelic(state.Dispatch.Active)
	if relic == nil || !state.Dispatch.Relics[relic.ID] {
		return
	}
	lead := units[0]
	switch relic.ID {
	case "bell":
		lead.Rage = min(gachaCatalog.Formula.RageMax, lead.Rage+relic.Value)
		lead.HP = max(1, lead.HP-max(1, lead.MaxHP*relic.Price/100))
	case "shroud":
		lead.Shield += lead.MaxHP * relic.Value / 100
		lead.rageBonus -= relic.Price
	case "lens":
		lead.skillBoost += float64(relic.Value) / 100
		lead.girl.Skills.Normal.Mult *= 1 - float64(relic.Price)/100
	}
}

package model

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const gachaProgressionOption = "GachaProgressionConfig"

var (
	ErrGachaExperience       = errors.New("levels require earned experience")
	ErrGachaPermits          = errors.New("recruit permits are required")
	ErrGachaProgressionLimit = errors.New("no eligible banked opportunity")
	ErrGachaAwakening        = errors.New("awakening level, stars, trial or branch is missing")
	ErrGachaDualTeam         = errors.New("two distinct squads of four to six characters are required")
	ErrGachaProgressionAdmin = errors.New("progression configuration requires an administrator")
	ErrGachaConfigVersion    = errors.New("progression configuration version changed")
)

// Entitlements snapshot these budgets when issued. Changing an option cannot
// multiply already accrued opportunities or rewrite an in-flight reward.
type GachaProgressionConfig struct {
	Version            int    `json:"version"`
	TrainingPerDay     int    `json:"training_per_day"`
	TrainingXP         [3]int `json:"training_xp"`
	PermitsPerDay      int    `json:"permits_per_day"`
	PermitsPerWeek     int    `json:"permits_per_week"`
	MarksPerDay        int    `json:"marks_per_day"`
	MarksPerWeek       int    `json:"marks_per_week"`
	AwakeningCosts     [3]int `json:"awakening_costs"`
	WishesPerDay       int    `json:"wishes_per_day"`
	ShardsPerWish      int    `json:"shards_per_wish"`
	ShardsPerWeek      int    `json:"shards_per_week"`
	RefinementsPerWeek int    `json:"refinements_per_week"`
}

func DefaultGachaProgressionConfig() GachaProgressionConfig {
	return GachaProgressionConfig{Version: 1, TrainingPerDay: 3, TrainingXP: [3]int{2000, 4000, 6000},
		PermitsPerDay: 5, PermitsPerWeek: 10, MarksPerDay: 40, MarksPerWeek: 120,
		AwakeningCosts: [3]int{80, 120, 200}, WishesPerDay: 2, ShardsPerWish: 10,
		ShardsPerWeek: 40, RefinementsPerWeek: 2}
}

func validGachaProgressionConfig(c GachaProgressionConfig) bool {
	if c.Version < 1 || c.Version > 1_000_000 || c.TrainingPerDay < 1 || c.TrainingPerDay > 5 ||
		c.PermitsPerDay < 1 || c.PermitsPerDay > 10 || c.PermitsPerWeek < 0 || c.PermitsPerWeek > 30 ||
		c.MarksPerDay < 10 || c.MarksPerDay > 80 || c.MarksPerWeek < 0 || c.MarksPerWeek > 240 ||
		c.WishesPerDay < 1 || c.WishesPerDay > 4 || c.ShardsPerWish < 1 || c.ShardsPerWish > 20 ||
		c.ShardsPerWeek < 0 || c.ShardsPerWeek > 80 || c.RefinementsPerWeek < 1 || c.RefinementsPerWeek > 4 {
		return false
	}
	for i := range 3 {
		if c.TrainingXP[i] < 500 || c.TrainingXP[i] > 10_000 || c.AwakeningCosts[i] < 20 || c.AwakeningCosts[i] > 1000 {
			return false
		}
		if i > 0 && (c.TrainingXP[i] < c.TrainingXP[i-1] || c.AwakeningCosts[i] < c.AwakeningCosts[i-1]) {
			return false
		}
	}
	return true
}

func loadGachaProgressionConfig(tx *gorm.DB) (GachaProgressionConfig, error) {
	c := DefaultGachaProgressionConfig()
	var option Option
	err := tx.Select("value").Where(&Option{Key: gachaProgressionOption}).First(&option).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	var stored GachaProgressionConfig
	if err := common.UnmarshalJsonStr(option.Value, &stored); err != nil || !validGachaProgressionConfig(stored) {
		return c, ErrGachaRunState
	}
	return stored, nil
}

type GachaProgressionDay struct {
	Date           string                 `json:"date"`
	Rules          GachaProgressionConfig `json:"rules"`
	TrainingLeft   int                    `json:"training_left"`
	PermitsClaimed bool                   `json:"permits_claimed"`
	WishTargets    []string               `json:"wish_targets"`
	Activities     []string               `json:"activities"`
	MarksClaimed   bool                   `json:"marks_claimed"`
}

type GachaProgressionWeek struct {
	Monday          string                 `json:"monday"`
	Week            string                 `json:"week"`
	Rules           GachaProgressionConfig `json:"rules"`
	Activities      []string               `json:"activities"`
	Completed       int                    `json:"completed"`
	Claimed         bool                   `json:"claimed"`
	RefinementsLeft int                    `json:"refinements_left"`
}

type GachaDualTrialState struct {
	Week       string     `json:"week"`
	Squads     [][]string `json:"squads,omitempty"`
	Phase      int        `json:"phase"`
	Cleared    bool       `json:"cleared"`
	BestRounds int        `json:"best_rounds"`
	Rounds     int        `json:"rounds"`
}

type GachaProgressionState struct {
	Version        int                    `json:"version"`
	StartedDay     string                 `json:"started_day"`
	LastDay        string                 `json:"last_day"`
	LastMonday     string                 `json:"last_monday"`
	Legacy         bool                   `json:"legacy"`
	Days           []GachaProgressionDay  `json:"days"`
	Weeks          []GachaProgressionWeek `json:"weeks"`
	Permits        int                    `json:"permits"`
	Marks          int                    `json:"marks"`
	StarterClaimed bool                   `json:"starter_claimed"`
	MainlineClaims []string               `json:"mainline_claims"`
	TrialWins      map[string]int         `json:"trial_wins"`
	ReserveSquad   []string               `json:"reserve_squad"`
	Dual           GachaDualTrialState    `json:"dual"`
	Milestones     map[string]int64       `json:"milestones"`
	ActiveDays     []string               `json:"active_days"`
}

type GachaAwakeningBranch struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type GachaProgressionView struct {
	Rules           GachaProgressionConfig            `json:"rules"`
	Branches        map[string][]GachaAwakeningBranch `json:"branches"`
	CanConfigure    bool                              `json:"can_configure"`
	DailyBankDays   int                               `json:"daily_bank_days"`
	WeeklyBankWeeks int                               `json:"weekly_bank_weeks"`
	PowerWeights    map[string]float64                `json:"power_weights"`
}

var gachaAwakeningPowerWeights = map[string]float64{
	"focus": 0.07, "sweep": 0.065, "guard": 0.055, "mirror": 0.06,
	"purify": 0.05, "renew": 0.055, "rally": 0.06, "suppress": 0.065,
}

var gachaProgressionActivities = []string{"training", "wish", "campaign", "rift", "trial", "weekly", "dual"}
var gachaProgressionActions = []string{"daily_permits", "contract_claim", "wish", "weekly_claim", "starter_select", "shard_unlock", "mainline_claim", "awakening", "awakening_branch", "teams_save", "sigil_refine", "progression_config", "weapon_upgrade", "weapon_awaken", "weapon_engrave", "potential_unlock", "use_manual", "iv_roll", "iv_lock"}

func validateGachaProgressionIntent(intent GachaActionIntent) error {
	if (intent.Enabled != nil) != (intent.Action == "iv_lock") || intent.OpponentID != nil || intent.StakeE != nil || intent.QueueID != nil || intent.RoomID != nil {
		return ErrGachaIntent
	}
	if intent.Action != "teams_save" && intent.Squad != nil || intent.Action != "sigil_refine" && intent.Count != nil ||
		intent.Action != "progression_config" && intent.Config != nil {
		return ErrGachaIntent
	}
	usesGirl := slices.Contains([]string{"wish", "weekly_claim", "starter_select", "shard_unlock", "awakening", "awakening_branch", "weapon_upgrade", "weapon_awaken", "weapon_engrave", "potential_unlock", "use_manual", "iv_roll", "iv_lock"}, intent.Action)
	usesItem := slices.Contains([]string{"daily_permits", "contract_claim", "weekly_claim", "mainline_claim", "sigil_refine"}, intent.Action)
	usesChoice := slices.Contains([]string{"awakening", "awakening_branch", "sigil_refine", "weapon_engrave", "iv_lock"}, intent.Action)
	if !usesGirl && intent.GirlID != nil || usesGirl && (intent.GirlID == nil || gachaGirl(*intent.GirlID) == nil) ||
		!usesItem && intent.ItemID != nil || usesItem && intent.ItemID == nil || !usesChoice && intent.Choice != nil {
		return ErrGachaIntent
	}
	switch intent.Action {
	case "iv_lock":
		if intent.Choice == nil || gachaIndividualIndex(*intent.Choice) < 0 {
			return ErrGachaIntent
		}
	case "daily_permits", "contract_claim":
		if _, err := gachaCalendarDay(*intent.ItemID); err != nil {
			return ErrGachaIntent
		}
	case "weekly_claim":
		if !validGachaWeek(*intent.ItemID) {
			return ErrGachaIntent
		}
	case "mainline_claim":
		if !slices.Contains([]string{"s1", "s3", "s6"}, *intent.ItemID) {
			return ErrGachaIntent
		}
	case "starter_select":
		if !gachaCardHasSource(gachaGirl(*intent.GirlID), "starter") {
			return ErrGachaIntent
		}
	case "awakening_branch":
		if intent.Choice == nil || !validGachaBranch(*intent.GirlID, *intent.Choice) {
			return ErrGachaIntent
		}
	case "awakening":
		if intent.Choice != nil && !validGachaBranch(*intent.GirlID, *intent.Choice) {
			return ErrGachaIntent
		}
	case "weapon_engrave":
		if intent.Choice == nil {
			return ErrGachaIntent
		}
		if _, ok := gachaCultivation.Engravings[*intent.Choice]; !ok {
			return ErrGachaIntent
		}
	case "teams_save":
		if intent.Squad == nil || len(*intent.Squad) < 8 || len(*intent.Squad) > 2*GachaSquadLimit || len(*intent.Squad)%2 != 0 {
			return ErrGachaIntent
		}
	case "sigil_refine":
		if intent.Count == nil || *intent.Count < 1 || *intent.Count > 2 || !validGachaSigilID(*intent.ItemID) || intent.Choice == nil || !slices.Contains(gachaSigilAffixIDs, *intent.Choice) {
			return ErrGachaIntent
		}
	case "progression_config":
		if intent.Config == nil || !validGachaProgressionConfig(*intent.Config) {
			return ErrGachaIntent
		}
	}
	return nil
}

func gachaCalendarDay(day string) (time.Time, error) {
	t, err := time.ParseInLocation("2006-01-02", day, gameZone)
	if err != nil || t.Year() < 2000 || t.Year() > 9999 {
		return t, ErrGachaRunState
	}
	return t, nil
}

func gachaMonday(now time.Time) time.Time {
	local := now.In(gameZone)
	day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, gameZone)
	return day.AddDate(0, 0, -(int(day.Weekday())+6)%7)
}

func normalizeGachaProgression(state *GachaGameState, now time.Time, rules GachaProgressionConfig) error {
	today, _ := gachaCalendarDay(gachaDay(now))
	monday := gachaMonday(now)
	if state.Progression == nil {
		legacy := state.TotalPull > 0 || state.Cleared["s1"]
		for _, own := range state.Girls {
			legacy = legacy || own.Level > 1
		}
		state.Progression = &GachaProgressionState{Version: 1, StartedDay: gachaDay(now),
			LastDay: gachaDay(today.AddDate(0, 0, -1)), LastMonday: gachaDay(monday.AddDate(0, 0, -7)),
			Legacy: legacy, TrialWins: map[string]int{}, Milestones: map[string]int64{}}
	}
	p := state.Progression
	if p.Version != 1 || p.Permits < 0 || p.Permits > gachaMaxResource || p.Marks < 0 || p.Marks > gachaMaxResource || len(p.Days) > 7 || len(p.Weeks) > 2 {
		return ErrGachaRunState
	}
	start, err := gachaCalendarDay(p.StartedDay)
	if err != nil {
		return err
	}
	lastDay, err := gachaCalendarDay(p.LastDay)
	if err != nil {
		return err
	}
	lastMonday, err := gachaCalendarDay(p.LastMonday)
	if err != nil || lastMonday.Weekday() != time.Monday {
		return ErrGachaRunState
	}
	cutoff := today.AddDate(0, 0, -6)
	p.Days = slices.DeleteFunc(p.Days, func(d GachaProgressionDay) bool { return d.Date < gachaDay(cutoff) })
	first := lastDay.AddDate(0, 0, 1)
	if first.Before(cutoff) {
		first = cutoff
	}
	if first.Before(start) {
		first = start
	}
	for day := first; !day.After(today); day = day.AddDate(0, 0, 1) {
		p.Days = append(p.Days, GachaProgressionDay{Date: gachaDay(day), Rules: rules, TrainingLeft: rules.TrainingPerDay,
			WishTargets: []string{}, Activities: []string{}})
	}
	if today.After(lastDay) {
		p.LastDay = gachaDay(today)
	}
	weekCutoff := monday.AddDate(0, 0, -7)
	p.Weeks = slices.DeleteFunc(p.Weeks, func(w GachaProgressionWeek) bool { return w.Monday < gachaDay(weekCutoff) })
	firstWeek := lastMonday.AddDate(0, 0, 7)
	if firstWeek.Before(weekCutoff) {
		firstWeek = weekCutoff
	}
	if firstWeek.Before(gachaMonday(start)) {
		firstWeek = gachaMonday(start)
	}
	for week := firstWeek; !week.After(monday); week = week.AddDate(0, 0, 7) {
		p.Weeks = append(p.Weeks, GachaProgressionWeek{Monday: gachaDay(week), Week: gachaWeekKey(week), Rules: rules,
			RefinementsLeft: rules.RefinementsPerWeek, Activities: []string{}})
	}
	if monday.After(lastMonday) {
		p.LastMonday = gachaDay(monday)
	}
	if p.TrialWins == nil {
		p.TrialWins = map[string]int{}
	}
	if p.Milestones == nil {
		p.Milestones = map[string]int64{}
	}
	if p.Dual.Week != gachaWeekKey(now) {
		p.Dual = GachaDualTrialState{Week: gachaWeekKey(now)}
	}
	if len(p.Days) > 7 || len(p.Weeks) > 2 || len(p.MainlineClaims) > 3 || len(p.ReserveSquad) > GachaSquadLimit || len(p.ActiveDays) > 90 {
		return ErrGachaRunState
	}
	for i, d := range p.Days {
		if _, err := gachaCalendarDay(d.Date); err != nil || !validGachaProgressionConfig(d.Rules) || d.TrainingLeft < 0 || d.TrainingLeft > d.Rules.TrainingPerDay ||
			len(d.WishTargets) > d.Rules.WishesPerDay || len(d.Activities) > 2 || i > 0 && d.Date <= p.Days[i-1].Date {
			return ErrGachaRunState
		}
		for j, id := range d.WishTargets {
			if gachaGirl(id) == nil || slices.Contains(d.WishTargets[:j], id) {
				return ErrGachaRunState
			}
		}
		for j, kind := range d.Activities {
			if !slices.Contains(gachaProgressionActivities, kind) || slices.Contains(d.Activities[:j], kind) {
				return ErrGachaRunState
			}
		}
	}
	for i, w := range p.Weeks {
		date, err := gachaCalendarDay(w.Monday)
		if err != nil || date.Weekday() != time.Monday || gachaWeekKey(date) != w.Week || !validGachaProgressionConfig(w.Rules) ||
			w.Completed < 0 || w.Completed > 5 || len(w.Activities) > len(gachaProgressionActivities) || w.RefinementsLeft < 0 ||
			w.RefinementsLeft > w.Rules.RefinementsPerWeek || i > 0 && w.Monday <= p.Weeks[i-1].Monday {
			return ErrGachaRunState
		}
		for j, kind := range w.Activities {
			if !slices.Contains(gachaProgressionActivities, kind) || slices.Contains(w.Activities[:j], kind) {
				return ErrGachaRunState
			}
		}
	}
	for id, rank := range p.TrialWins {
		if gachaGirl(id) == nil || rank < 1 || rank > 3 {
			return ErrGachaRunState
		}
	}
	for id, own := range state.Girls {
		if own.Awakening < 0 || own.Awakening > 3 || own.Awakening == 0 && own.Branch != "" {
			return ErrGachaRunState
		}
		if own.Awakening > 0 && (gachaPeakLevel(own) < 30+own.Awakening*10 || own.Star < 2+own.Awakening || !validGachaBranch(id, own.Branch)) {
			return ErrGachaRunState
		}
	}
	return nil
}

func recordGachaProgressionActivity(state *GachaGameState, kind string, now time.Time) {
	p := state.Progression
	if p == nil || !slices.Contains(gachaProgressionActivities, kind) {
		return
	}
	for i := range p.Days {
		d := &p.Days[i]
		if !d.MarksClaimed && len(d.Activities) < 2 && !slices.Contains(d.Activities, kind) {
			d.Activities = append(d.Activities, kind)
			break
		}
	}
	for i := range p.Weeks {
		w := &p.Weeks[i]
		needsCount := w.Completed < 5
		addsVariety := len(w.Activities) < 2 && !slices.Contains(w.Activities, kind)
		if !w.Claimed && (needsCount || addsVariety) {
			w.Completed = min(5, w.Completed+1)
			if !slices.Contains(w.Activities, kind) {
				w.Activities = append(w.Activities, kind)
			}
			break
		}
	}
	day := gachaDay(now)
	if !slices.Contains(p.ActiveDays, day) {
		p.ActiveDays = append(p.ActiveDays, day)
		if len(p.ActiveDays) > 90 {
			p.ActiveDays = p.ActiveDays[len(p.ActiveDays)-90:]
		}
	}
}

func awardGachaExperience(state *GachaGameState, squad []string, amount int, drops *[]string) error {
	if amount < 0 || amount > 100_000 {
		return ErrGachaIntent
	}
	limit, total := gachaCaptainLevel(state), 0
	entries := make([]string, 0, len(squad))
	for _, id := range squad {
		own, girl := state.Girls[id], gachaGirl(id)
		if !gachaCardOwned(own) || girl == nil {
			return ErrGachaTeam
		}
		credited, err := trainGachaLevel(own, amount, limit)
		if err != nil {
			return err
		}
		total += credited
		entry := fmt.Sprintf("%s +%d", girl.Name, credited)
		if credited < amount {
			entry += "（已达等级上限）"
		}
		entries = append(entries, entry)
	}
	if len(entries) > 0 && amount > 0 {
		switch {
		case len(entries) > 1 && total == 0:
			*drops = append(*drops, fmt.Sprintf("伙伴经验 +0（全员已达 Lv.%d 上限）", limit))
		case len(entries) > 1 && total == len(entries)*amount:
			*drops = append(*drops, fmt.Sprintf("伙伴经验：每人 +%d", amount))
		default:
			*drops = append(*drops, "伙伴经验："+strings.Join(entries, "、"))
		}
	}
	return nil
}

func gachaStarShardCost(girl *GachaGirl, nextStar int) int {
	if nextStar < 2 || nextStar > gachaCatalog.Growth.StarMax || girl == nil {
		return 0
	}
	if nextStar >= gachaFirstAdvancedStar {
		return gachaCultivation.AdvancedShards[girl.Rarity][nextStar-gachaFirstAdvancedStar]
	}
	if girl.Rarity == "R" {
		return [4]int{20, 40, 60, 80}[nextStar-2]
	}
	if girl.Rarity == "SR" {
		return map[int]int{3: 40, 4: 80, 5: 120}[nextStar]
	}
	return gachaCatalog.Growth.StarCost[nextStar]
}

func GachaAwakeningBranches(girlID string) []GachaAwakeningBranch {
	g := gachaGirl(girlID)
	if g == nil {
		return nil
	}
	switch g.Role {
	case "sdps", "adps":
		names := [2]string{"\u7834\u52bf\u4e13\u7cbe", "\u71ce\u539f\u4e13\u7cbe"}
		if girlID == "c04" {
			names = [2]string{"\u7834\u7532\u9f99\u67aa", "\u71ce\u539f\u9f99\u67aa"}
		}
		return []GachaAwakeningBranch{{"focus", names[0], "\u5355\u4f53\u7a7f\u900f\uff1b\u4e09\u9636\u5bf9\u534a\u8840\u4ee5\u4e0b\u76ee\u6807\u8ffd\u52a0\u4f24\u5bb3"},
			{"sweep", names[1], "\u964d\u4f4e\u5355\u4f53\u500d\u7387\uff0c\u6539\u4e3a\u7fa4\u653b\u5e76\u9644\u52a0\u6301\u7eed\u4fb5\u8680\u707c\u70e7"}}
	case "tank":
		names := [2]string{"\u5b88\u62a4\u4e4b\u58c1", "\u8346\u68d8\u4e4b\u58c1"}
		if girlID == "c07" {
			names = [2]string{"\u5b88\u62a4\u661f\u58c1", "\u8346\u68d8\u661f\u58c1"}
		}
		return []GachaAwakeningBranch{{"guard", names[0], "\u7fa4\u4f53\u62a4\u76fe\u4e0e\u5632\u8bbd\uff1b\u4e8c\u9636\u51c0\u5316\uff0c\u4e09\u9636\u4fdd\u62a4\u4f4e\u8840\u961f\u53cb"},
			{"mirror", names[1], "\u62a4\u76fe\u91cf\u51cf\u5c11\uff0c\u5c06\u62a4\u76fe\u5438\u6536\u7684\u4e00\u90e8\u5206\u4f24\u5bb3\u53cd\u51fb\u7ed9\u653b\u51fb\u8005"}}
	case "heal":
		return []GachaAwakeningBranch{{"purify", "\u51c0\u5316\u6f6e\u751f", "\u7fa4\u4f53\u6cbb\u7597\u4e0e\u51c0\u5316\uff1b\u4e09\u9636\u4e3a\u4f4e\u8840\u961f\u53cb\u989d\u5916\u63d0\u4f9b\u62a4\u76fe"},
			{"renew", "\u56de\u54cd\u4e4b\u606f", "\u96c6\u4e2d\u6cbb\u7597\u8840\u91cf\u6700\u4f4e\u7684\u961f\u53cb\uff0c\u9644\u52a0\u62a4\u76fe\u4e0e\u6012\u6c14\uff0c\u653e\u5f03\u7fa4\u4f53\u51c0\u5316"}}
	default:
		return []GachaAwakeningBranch{{"rally", "\u534f\u594f\u661f\u6b4c", "\u7fa4\u4f53\u653b\u51fb\u4e0e\u6012\u6c14\u652f\u63f4\uff1b\u4e09\u9636\u4f4e\u8840\u961f\u53cb\u83b7\u5f97\u4fdd\u62a4"},
			{"suppress", "\u5c01\u7981\u56de\u54cd", "\u6539\u4e3a\u5355\u4f53\u63a7\u5236\u3001\u7834\u7532\u548c\u524a\u6012\uff0c\u653e\u5f03\u7fa4\u4f53\u653b\u51fb\u589e\u76ca"}}
	}
}

func validGachaBranch(girlID, branch string) bool {
	return slices.ContainsFunc(GachaAwakeningBranches(girlID), func(b GachaAwakeningBranch) bool { return b.ID == branch })
}

func gachaAwakeningSkill(girl *GachaGirl, own *GachaOwnedGirl) GachaSkill {
	skill := girl.Skills.Skill
	rank := own.Awakening
	if rank == 0 {
		return skill
	}
	for _, branch := range GachaAwakeningBranches(girl.ID) {
		if branch.ID == own.Branch {
			skill.Name = branch.Name
		}
	}
	switch own.Branch {
	case "focus":
		skill.Target, skill.Hits = "enemy", 1
		if girl.Role == "adps" {
			skill.Mult *= 1.6
		}
		skill.IgnoreDefPct = min(70, max(skill.IgnoreDefPct, 20+rank*8))
		skill.LowHPBonus = 0
		if rank == 3 {
			skill.LowHPBonus = 0.25
		}
	case "sweep":
		skill.Target, skill.Hits, skill.IgnoreDefPct, skill.StunChance, skill.DefBreak = "allEnemies", 1, 0, 0, 0
		if girl.Role == "sdps" {
			skill.Mult *= 0.48 + float64(rank)*0.025
		} else {
			skill.Mult *= 0.85
		}
		skill.DotPct, skill.DotCap = 0.01+float64(rank)*0.005, 0.5+float64(rank)*0.1
	case "guard", "mirror":
		skill.Target, skill.Taunt = "team", true
		baseShield := max(1.2, skill.ShieldMult)
		if own.Branch == "guard" {
			skill.ShieldMult = baseShield * (1 + float64(rank)*0.1)
			if rank >= 2 {
				skill.Cleanse = 1
			}
			if rank == 3 {
				skill.LowHPShield = 0.5
			}
		} else {
			skill.ShieldMult = baseShield * 0.65
			skill.ShieldReflect = 0.2 + float64(rank)*0.1
		}
	case "purify":
		skill.Target = "team"
		if girl.Skills.Skill.Target == "ally" {
			skill.HealMult *= 0.7
		}
		skill.Cleanse = 1
		if rank >= 2 {
			skill.Cleanse = 2
		}
		if rank == 3 {
			skill.LowHPShield = 0.5
		}
	case "renew":
		skill.Target, skill.Cleanse, skill.ExtraTurn = "ally", 0, false
		skill.HealMult *= 1.35
		skill.ShieldMult, skill.RageGain = 0.5, 15+rank*5
		if rank == 3 {
			skill.LowHPShield = 0.5
		}
	case "rally":
		skill.Target, skill.ExtraTurn = "team", false
		skill.TeamAtkUp, skill.BuffTurns, skill.RageGain = 20+rank*5, 2, 8+rank*4
		skill.ShieldMult = max(0.5, skill.ShieldMult*0.6)
		if rank == 3 {
			skill.LowHPShield = 0.5
		}
	case "suppress":
		skill.Target, skill.Mult, skill.HealMult, skill.ShieldMult, skill.TeamAtkUp = "enemy", 1.2+float64(rank)*0.12, 0, 0, 0
		skill.StunChance, skill.StunTurns = 0.35+float64(rank)*0.1, 1
		skill.DefBreak, skill.DefBreakTurns, skill.RageDrain = 15+rank*5, 2, 10+rank*5
	}
	return skill
}

func applyGachaProgressionAction(tx *gorm.DB, userID int, state *GachaGameState, intent GachaActionIntent, result *GachaActionResult, now time.Time) error {
	p := state.Progression
	switch intent.Action {
	case "weapon_upgrade", "weapon_awaken", "weapon_engrave", "potential_unlock", "use_manual", "iv_roll", "iv_lock":
		return applyGachaCultivationAction(state, intent, result)
	case "daily_permits", "contract_claim":
		for i := range p.Days {
			d := &p.Days[i]
			if d.Date != *intent.ItemID {
				continue
			}
			if intent.Action == "daily_permits" {
				if d.PermitsClaimed {
					return ErrGachaProgressionLimit
				}
				if p.Permits > gachaMaxResource-d.Rules.PermitsPerDay {
					return ErrGachaCapacity
				}
				p.Permits += d.Rules.PermitsPerDay
				d.PermitsClaimed = true
				result.Drops = append(result.Drops, fmt.Sprintf("\u62db\u52df\u5238 +%d", d.Rules.PermitsPerDay))
			} else {
				if d.MarksClaimed || len(d.Activities) < 2 {
					return ErrGachaProgressionLimit
				}
				if p.Marks > gachaMaxResource-d.Rules.MarksPerDay {
					return ErrGachaCapacity
				}
				p.Marks += d.Rules.MarksPerDay
				d.MarksClaimed = true
				result.Drops = append(result.Drops, fmt.Sprintf("\u5951\u7ea6\u5370 +%d", d.Rules.MarksPerDay))
			}
			return nil
		}
		return ErrGachaProgressionLimit
	case "wish":
		girl := gachaGirl(*intent.GirlID)
		if !gachaCardHasSource(girl, "growth") {
			return ErrGachaCardSource
		}
		for i := range p.Days {
			d := &p.Days[i]
			if len(d.WishTargets) >= d.Rules.WishesPerDay || slices.Contains(d.WishTargets, girl.ID) {
				continue
			}
			if err := grantGachaSourceShards(state, girl, "growth", d.Rules.ShardsPerWish); err != nil {
				return err
			}
			d.WishTargets = append(d.WishTargets, girl.ID)
			recordGachaProgressionActivity(state, "wish", now)
			result.Drops = append(result.Drops, fmt.Sprintf("%s\u4e13\u5c5e\u788e\u7247 +%d", girl.Name, d.Rules.ShardsPerWish))
			return nil
		}
		return ErrGachaProgressionLimit
	case "weekly_claim":
		for i := range p.Weeks {
			w := &p.Weeks[i]
			if w.Week != *intent.ItemID {
				continue
			}
			if w.Claimed || w.Completed < 5 || len(w.Activities) < 2 {
				return ErrGachaProgressionLimit
			}
			if p.Marks > gachaMaxResource-w.Rules.MarksPerWeek || p.Permits > gachaMaxResource-w.Rules.PermitsPerWeek {
				return ErrGachaCapacity
			}
			girl := gachaGirl(*intent.GirlID)
			if !gachaCardHasSource(girl, "growth") {
				return ErrGachaCardSource
			}
			if w.Rules.ShardsPerWeek > 0 {
				if err := grantGachaSourceShards(state, girl, "growth", w.Rules.ShardsPerWeek); err != nil {
					return err
				}
			}
			p.Marks += w.Rules.MarksPerWeek
			p.Permits += w.Rules.PermitsPerWeek
			w.Claimed = true
			result.Drops = append(result.Drops, fmt.Sprintf("\u5468\u5e38\uff1a\u5951\u7ea6\u5370 +%d\u3001\u62db\u52df\u5238 +%d\u3001%s\u788e\u7247 +%d", w.Rules.MarksPerWeek, w.Rules.PermitsPerWeek, girl.Name, w.Rules.ShardsPerWeek))
			return nil
		}
		return ErrGachaProgressionLimit
	case "starter_select":
		if p.StarterClaimed || !state.Cleared["s2"] {
			return ErrGachaProgressionLimit
		}
		girl := gachaGirl(*intent.GirlID)
		if _, err := grantGachaWholeCard(state, girl, "starter"); err != nil {
			return err
		}
		p.StarterClaimed = true
		result.Drops = append(result.Drops, "\u9996\u6838\u5fc3\u81ea\u9009\uff1a"+girl.Name+"\uff08\u5df2\u62e5\u6709\u65f6\u8f6c\u4e3a\u4e13\u5c5e\u788e\u7247\uff09")
		if err := awardGachaExperience(state, []string{girl.ID}, 19_000, &result.Drops); err != nil {
			return err
		}
	case "shard_unlock":
		girl := gachaGirl(*intent.GirlID)
		own := state.Girls[girl.ID]
		need := gachaCatalog.Rarity[girl.Rarity].ShardToUnlock
		if own == nil || own.Star != 0 || own.Level != 0 || need < 1 || own.Shards < need {
			return ErrGachaFunds
		}
		own.Shards -= need
		own.Star, own.Level = gachaInitialStars(girl), 1
		result.Drops = append(result.Drops, "\u788e\u7247\u62db\u52df\u6210\u529f\uff1a"+girl.Name)
	case "mainline_claim":
		stageID := *intent.ItemID
		if !state.Cleared[stageID] || slices.Contains(p.MainlineClaims, stageID) {
			return ErrGachaProgressionLimit
		}
		permits := map[string]int{"s1": 5, "s3": 10, "s6": 15}[stageID]
		xp := map[string]int{"s1": 14_000, "s3": 8000, "s6": 0}[stageID]
		gold := map[string]int{"s1": 500, "s3": 600, "s6": 1000}[stageID]
		if p.Permits > gachaMaxResource-permits || state.Gold > gachaMaxResource-gold {
			return ErrGachaCapacity
		}
		if len(state.Squad) < 1 {
			return ErrGachaTeam
		}
		result.Drops = append(result.Drops, fmt.Sprintf("主线首通里程碑：招募券 +%d、金币 +%d", permits, gold))
		if err := awardGachaExperience(state, state.Squad, xp, &result.Drops); err != nil {
			return err
		}
		p.Permits += permits
		state.Gold += gold
		p.MainlineClaims = append(p.MainlineClaims, stageID)
	case "awakening", "awakening_branch":
		own := state.Girls[*intent.GirlID]
		if own == nil || own.Star < 3 || own.Level < 40 {
			return ErrGachaAwakening
		}
		if intent.Action == "awakening_branch" {
			if own.Awakening < 1 || !validGachaBranch(*intent.GirlID, *intent.Choice) {
				return ErrGachaAwakening
			}
			if own.Branch == *intent.Choice {
				return nil
			}
			if state.Gold < 250 {
				return ErrGachaFunds
			}
			state.Gold -= 250
			own.Branch = *intent.Choice
			result.Drops = append(result.Drops, "\u6280\u80fd\u8def\u7ebf\u5df2\u6539\u9009\uff0c\u89c9\u9192\u9636\u7ea7\u4fdd\u7559")
			break
		}
		rank := own.Awakening + 1
		if rank > 3 || own.Star < rank+2 || own.Level < 30+rank*10 || p.TrialWins[*intent.GirlID] < rank {
			return ErrGachaAwakening
		}
		if rank == 1 && (intent.Choice == nil || !validGachaBranch(*intent.GirlID, *intent.Choice)) {
			return ErrGachaAwakening
		}
		if rank > 1 && intent.Choice != nil {
			return ErrGachaIntent
		}
		cost := state.ProgressionRules.AwakeningCosts[rank-1]
		if p.Marks < cost {
			return ErrGachaFunds
		}
		p.Marks -= cost
		own.Awakening = rank
		if rank == 1 {
			own.Branch = *intent.Choice
		}
		if p.Milestones["first_awakening"] == 0 {
			p.Milestones["first_awakening"] = now.UnixMilli()
		}
		result.Drops = append(result.Drops, fmt.Sprintf("%s\u89c9\u9192%d\u9636\uff0c\u5951\u7ea6\u5370 -%d", gachaGirl(*intent.GirlID).Name, rank, cost))
	case "teams_save":
		team := *intent.Squad
		if len(team) < 8 || len(team) > 2*GachaSquadLimit || len(team)%2 != 0 {
			return ErrGachaDualTeam
		}
		for i, id := range team {
			own := state.Girls[id]
			if !gachaCardOwned(own) || gachaGirl(id) == nil || slices.Contains(team[:i], id) {
				return ErrGachaDualTeam
			}
		}
		split := len(team) / 2
		state.Squad, p.ReserveSquad = slices.Clone(team[:split]), slices.Clone(team[split:])
		result.Drops = append(result.Drops, "两支常用队伍已保存；试炼开局后使用当时锁定的角色与站位")
	case "sigil_refine":
		sigil := state.Sigils[*intent.ItemID]
		if !validGachaSigil(sigil) {
			return ErrGachaSigil
		}
		keys := make([]string, 0, 2)
		for key := range sigil.Affixes {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		oldKey := keys[*intent.Count-1]
		newKey := *intent.Choice
		if newKey != oldKey && sigil.Affixes[newKey] > 0 {
			return ErrGachaIntent
		}
		bounds := gachaSigilAffixBounds[newKey][sigil.Quality-1]
		value := (bounds[0] + bounds[1] + 1) / 2
		if newKey == oldKey && sigil.Affixes[oldKey] >= value {
			return ErrGachaIntent
		}
		if state.Gold < 500 || state.Mats["sigil_dust"] < 10 {
			return ErrGachaFunds
		}
		var entitlement *GachaProgressionWeek
		for i := range p.Weeks {
			if p.Weeks[i].RefinementsLeft > 0 {
				entitlement = &p.Weeks[i]
				break
			}
		}
		if entitlement == nil {
			return ErrGachaProgressionLimit
		}
		entitlement.RefinementsLeft--
		state.Gold -= 500
		state.Mats["sigil_dust"] -= 10
		delete(sigil.Affixes, oldKey)
		sigil.Affixes[newKey] = value
		result.Sigil = sigil
		result.Drops = append(result.Drops, "\u7075\u5370\u5b9a\u5411\u6210\u529f\uff0c\u5bb6\u65cf\u4e0e\u53e6\u4e00\u6761\u5c5e\u6027\u4fdd\u7559")
	case "progression_config":
		var user User
		if err := tx.Select("role").First(&user, userID).Error; err != nil {
			return err
		}
		if user.Role < common.RoleAdminUser {
			return ErrGachaProgressionAdmin
		}
		defaults, err := common.Marshal(DefaultGachaProgressionConfig())
		if err != nil {
			return err
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&Option{Key: gachaProgressionOption, Value: string(defaults)}).Error; err != nil {
			return err
		}
		var option Option
		if err := lockForUpdate(tx).Where(&Option{Key: gachaProgressionOption}).First(&option).Error; err != nil {
			return err
		}
		var previous GachaProgressionConfig
		if err := common.UnmarshalJsonStr(option.Value, &previous); err != nil {
			return err
		}
		if intent.Config.Version != previous.Version+1 {
			return ErrGachaConfigVersion
		}
		data, err := common.Marshal(intent.Config)
		if err != nil {
			return err
		}
		updated := tx.Model(&Option{}).Where(map[string]any{"key": gachaProgressionOption, "value": option.Value}).Update("value", string(data))
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return ErrGachaConcurrent
		}
		state.ProgressionRules = *intent.Config
		result.ProgressionRules = intent.Config
		result.Drops = append(result.Drops, "\u6210\u957f\u914d\u7f6e\u5df2\u4fdd\u5b58\uff0c\u65b0\u9884\u7b97\u4ece\u4e0b\u6b21\u53d1\u653e\u673a\u4f1a\u5f00\u59cb\u751f\u6548")
	default:
		return ErrGachaIntent
	}
	return nil
}

func isGachaProgressionStage(id string) bool {
	if id == "training" || id == "dual_trial" {
		return true
	}
	parts := strings.Split(id, "_")
	return len(parts) == 3 && parts[0] == "trial" && gachaGirl(parts[1]) != nil && slices.Contains([]string{"1", "2", "3"}, parts[2])
}

func specialGachaProgressionStage(stageID string, state *GachaGameState, now time.Time) (*GachaStage, GachaRunSnapshot, error) {
	p := state.Progression
	snapshot := GachaRunSnapshot{Version: 1, Mode: stageID, Week: gachaWeekKey(now), ProgressionVersion: state.ProgressionRules.Version}
	stage := &GachaStage{ID: stageID, Img: "assets/d06-observatory.webp", Waves: 1, EnemyCount: 2, StarRound: 12}
	switch stageID {
	case "training":
		var day *GachaProgressionDay
		for i := range p.Days {
			if p.Days[i].TrainingLeft > 0 {
				day = &p.Days[i]
				break
			}
		}
		if day == nil {
			return nil, snapshot, ErrGachaProgressionLimit
		}
		tier := gachaTrainingTier(state.Cleared) - 1
		stage.Name, stage.EnemyLv, stage.Exp = "\u7ecf\u9a8c\u8bad\u7ec3", 1, day.Rules.TrainingXP[tier]
		snapshot.TrainingDay, snapshot.ProgressionVersion = day.Date, day.Rules.Version
		snapshot.Squad = slices.Clone(state.Squad)
	case "dual_trial":
		if !state.Cleared["s6"] || p.Dual.Cleared {
			return nil, snapshot, ErrGachaProgressionLimit
		}
		if len(p.Dual.Squads) == 0 {
			all := append(slices.Clone(state.Squad), p.ReserveSquad...)
			if len(state.Squad) < 4 || len(state.Squad) > GachaSquadLimit || len(p.ReserveSquad) < 4 || len(p.ReserveSquad) > GachaSquadLimit {
				return nil, snapshot, ErrGachaDualTeam
			}
			for i, id := range all {
				own := state.Girls[id]
				if own == nil || own.Level < 40 || own.Star < 3 || slices.Contains(all[:i], id) {
					return nil, snapshot, ErrGachaDualTeam
				}
			}
			p.Dual.Squads = [][]string{slices.Clone(state.Squad), slices.Clone(p.ReserveSquad)}
		}
		if p.Dual.Phase < 0 || p.Dual.Phase > 1 || len(p.Dual.Squads) != 2 {
			return nil, snapshot, ErrGachaRunState
		}
		snapshot.Floor, snapshot.DualSquads = p.Dual.Phase+1, [][]string{slices.Clone(p.Dual.Squads[0]), slices.Clone(p.Dual.Squads[1])}
		snapshot.Squad = slices.Clone(p.Dual.Squads[p.Dual.Phase])
		mechanics := []string{"barrier", "regrowth", "reflection", "silence", "venom", "fury"}
		variant := 0
		for _, ch := range snapshot.Week {
			variant += int(ch)
		}
		stage.Mechanic = mechanics[(variant+p.Dual.Phase*2)%len(mechanics)]
		stage.EnemyElem = weeklyBossElement(snapshot.Week)
		stage.Name, stage.Cost, stage.EnemyLv = fmt.Sprintf("\u53cc\u961f\u5951\u7ea6\u8bd5\u70bc\u00b7\u7b2c%d\u6218", snapshot.Floor), 14, 62
		stage.HPScale, stage.AtkScale, stage.EnemyRes = 2.2, 1.3, 20
		if snapshot.Floor == 1 {
			stage.Waves, stage.EnemyCount = 2, 3
		} else {
			stage.HasBoss, stage.HPScale = true, 3
		}
	default:
		if !isGachaProgressionStage(stageID) {
			return nil, snapshot, ErrGachaStage
		}
		parts := strings.Split(stageID, "_")
		rank := int(parts[2][0] - '0')
		girl := gachaGirl(parts[1])
		own := state.Girls[girl.ID]
		if own == nil || own.Level < 30+rank*10 || own.Star < rank+2 || own.Awakening < rank-1 {
			return nil, snapshot, ErrGachaAwakening
		}
		snapshot.ProgressionGirl, snapshot.ProgressionRank = girl.ID, rank
		snapshot.Squad = slices.Clone(state.Squad)
		if !slices.Contains(snapshot.Squad, girl.ID) {
			if len(snapshot.Squad) >= GachaSquadLimit {
				snapshot.Squad[GachaSquadLimit-1] = girl.ID
			} else {
				snapshot.Squad = append(snapshot.Squad, girl.ID)
			}
		}
		stage.Name = fmt.Sprintf("%s\u4e13\u5c5e\u8bd5\u70bc\u00b7%d\u9636", girl.Name, rank)
		stage.Cost, stage.EnemyLv, stage.HPScale = 0, 20+rank*10, 1+float64(rank)*0.25
		stage.EnemyElem = girl.Elem
		if rank >= 2 {
			stage.Mechanic = map[string]string{"tank": "reflection", "heal": "venom", "support": "silence", "sdps": "barrier", "adps": "regrowth"}[girl.Role]
			stage.EnemyCount, stage.AtkScale = 3, 1.1
		}
		if rank == 3 {
			stage.Waves, stage.StarRound, stage.EnemyRes = 2, 18, 15
		}
	}
	snapshot.ProgressionStage = stage
	return stage, snapshot, nil
}

func settleGachaProgressionMode(state *GachaGameState, snapshot *GachaRunSnapshot, result *GachaFinishResult, now time.Time) error {
	if !result.Win {
		return nil
	}
	p := state.Progression
	switch snapshot.Mode {
	case "training":
		found := false
		for i := range p.Days {
			d := &p.Days[i]
			if d.Date != snapshot.TrainingDay {
				continue
			}
			if d.TrainingLeft <= 0 || d.Rules.Version != snapshot.ProgressionVersion {
				return ErrGachaRunState
			}
			d.TrainingLeft--
			found = true
			break
		}
		if !found {
			// A last-bank-day battle may finish just after midnight. Its issued
			// credential still owns one reward; no new expired-day run can start.
			day, err := gachaCalendarDay(snapshot.TrainingDay)
			if err != nil || now.Before(day.AddDate(0, 0, 7)) || !now.Before(day.AddDate(0, 0, 7).Add(GachaRunTTL)) {
				return ErrGachaRunState
			}
		}
		if err := awardGachaExperience(state, snapshot.Squad, snapshot.ProgressionStage.Exp, &result.Drops); err != nil {
			return err
		}
		recordGachaProgressionActivity(state, "training", now)
		result.Drops = append(result.Drops, "训练不产出 E")
	case "dual_trial":
		if snapshot.Week != p.Dual.Week {
			if snapshot.Floor == 2 {
				return awardGachaDualCompletion(state, result, now)
			}
			return nil
		}
		if p.Dual.Cleared || snapshot.Floor != p.Dual.Phase+1 || len(snapshot.DualSquads) != 2 || !slices.Equal(snapshot.Squad, p.Dual.Squads[p.Dual.Phase]) {
			return ErrGachaRunState
		}
		p.Dual.Phase++
		p.Dual.Rounds += result.Rounds
		recordGachaProgressionActivity(state, "dual", now)
		if p.Dual.Phase == 2 {
			p.Dual.Cleared, p.Dual.BestRounds = true, p.Dual.Rounds
			if err := awardGachaDualCompletion(state, result, now); err != nil {
				return err
			}
		} else {
			result.Drops = append(result.Drops, "\u7b2c\u4e00\u961f\u4efb\u52a1\u5b8c\u6210\uff0c\u4e0b\u4e00\u6218\u7531\u9501\u5b9a\u7684\u7b2c\u4e8c\u961f\u4e0a\u573a")
		}
	default:
		id, rank := snapshot.ProgressionGirl, snapshot.ProgressionRank
		if gachaGirl(id) == nil || rank < 1 || rank > 3 || !slices.Contains(snapshot.Squad, id) {
			return ErrGachaRunState
		}
		if p.TrialWins[id] < rank {
			p.TrialWins[id] = rank
			recordGachaProgressionActivity(state, "trial", now)
			result.Drops = append(result.Drops, "\u4e13\u5c5e\u8bd5\u70bc\u901a\u8fc7\uff0c\u53ef\u4f7f\u7528\u5951\u7ea6\u5370\u89c9\u9192")
		} else {
			result.Drops = append(result.Drops, "\u7ec3\u4e60\u5b8c\u6210\uff0c\u4e0d\u91cd\u590d\u4ea7\u51fa\u5956\u52b1\u6216\u5951\u7ea6\u8fdb\u5ea6")
		}
	}
	return nil
}

func awardGachaDualCompletion(state *GachaGameState, result *GachaFinishResult, now time.Time) error {
	gold := gachaCatalog.ActivityPayouts.DualGold
	if state.Gold > gachaMaxResource-gold || state.Mats["sigil_dust"] > gachaMaxResource-20 {
		return ErrGachaCapacity
	}
	state.Gold += gold
	state.Mats["sigil_dust"] += 20
	if state.Progression.Milestones["first_dual_clear"] == 0 {
		state.Progression.Milestones["first_dual_clear"] = now.UnixMilli()
	}
	result.Drops = append(result.Drops, fmt.Sprintf("双队试炼完成：金币 +%d、灵印粉尘 +20", gold))
	return nil
}

package model

import (
	"errors"
	"fmt"
	"slices"
)

type GachaVoyagePrize struct {
	Item   string `json:"item"`
	Amount int    `json:"amount"`
}

type GachaVoyagePoint struct {
	GachaStage
	Kind        string             `json:"kind"`
	Description string             `json:"description"`
	Requires    []string           `json:"requires,omitempty"`
	ChapterGate string             `json:"chapter_gate,omitempty"`
	CaptainMin  int                `json:"captain_min,omitempty"`
	DailyLimit  int                `json:"daily_limit,omitempty"`
	Prize       []GachaVoyagePrize `json:"prize,omitempty"`
	SweepPrize  []GachaVoyagePrize `json:"sweep_prize,omitempty"`
}

type GachaVoyageChest struct {
	ID    string             `json:"id"`
	Stars int                `json:"stars"`
	Prize []GachaVoyagePrize `json:"prize"`
}

type GachaVoyageChapter struct {
	ID       string                        `json:"id"`
	Name     string                        `json:"name"`
	Landmark string                        `json:"landmark"`
	Routes   map[string][]string           `json:"routes"`
	Chests   map[string][]GachaVoyageChest `json:"chests"`
}

type GachaVoyageCatalog struct {
	Chapters   []GachaVoyageChapter         `json:"chapters"`
	Stages     map[string]*GachaVoyagePoint `json:"stages"`
	Resources  []string                     `json:"resources"`
	FirstClear []GachaVoyagePrize           `json:"first_clear"`
	FirstThree []GachaVoyagePrize           `json:"first_three"`
}

var gachaVoyage = buildGachaVoyage()
var ErrGachaVoyageLimit = errors.New("voyage attempts exhausted")
var ErrGachaVoyageChest = errors.New("voyage chest is unavailable")

func buildGachaVoyage() GachaVoyageCatalog {
	book := GachaVoyageCatalog{Stages: map[string]*GachaVoyagePoint{}, FirstClear: []GachaVoyagePrize{{"field_notes", 1}}, FirstThree: []GachaVoyagePrize{{"expedition_atlas", 1}}}
	regions := []struct {
		name, landmark, element, mechanic string
		stations, veteran, abyss          []string
	}{
		{"雾森边境", "雾灯港", "plant", "regrowth", []string{"芦岸浮标", "苔舟浅滩", "渡鸦风口", "双岬灯塔", "雾湾领港人"}, []string{"封航水闸", "失灯哨所", "雾角信号塔"}, []string{"枯潮树庭", "无光领航台"}},
		{"火与遗迹", "余烬船坞", "fire", "fury", []string{"砂岩栈桥", "风炉前哨", "铜帆码头", "余烬水道", "赤礁守门人"}, []string{"熔砂回湾", "火壶要港", "铁帆熔炉"}, []string{"逆风灰港", "沸海王座"}},
		{"潮汐与月影", "月汐航标", "water", "barrier", []string{"贝影浮桥", "银帆水道", "鲸歌曲湾", "冷月船厂", "满潮掌舵人"}, []string{"潮钟内海", "深水祭坛", "锁月港"}, []string{"镜潮回廊", "月蚀暗礁"}},
		{"星穹终境", "子午星盘", "star", "reflection", []string{"星尘栈道", "碎镜银滩", "霜帆航标", "天穹泊位", "星盘观测者"}, []string{"南极光隘口", "霜晷船台", "群星内港"}, []string{"逆照长夜", "落星罗盘"}},
	}
	for index, region := range regions {
		number := index + 1
		chapter := GachaVoyageChapter{ID: fmt.Sprintf("c%d", number), Name: region.name, Landmark: region.landmark, Routes: map[string][]string{}, Chests: map[string][]GachaVoyageChest{}}
		legacy := gachaCatalog.Stages[index*3 : index*3+3]
		for node, base := range legacy {
			point := &GachaVoyagePoint{GachaStage: base, Kind: "story"}
			if base.UnlockStage != "" {
				point.Requires = []string{base.UnlockStage}
			}
			if index > 0 && node == 0 {
				prior := book.Chapters[index-1].Routes["story"]
				point.ChapterGate = prior[len(prior)-1]
			}
			book.Stages[base.ID] = point
			chapter.Routes["story"] = append(chapter.Routes["story"], base.ID)
		}
		for tier, mode := range []string{"story", "hard", "nightmare"} {
			names := [][]string{region.stations, region.veteran, region.abyss}[tier]
			for node, name := range names {
				base := legacy[2]
				id := fmt.Sprintf("v%d%s%d", number, []string{"s", "h", "n"}[tier], node+1)
				level := base.EnemyLv + 2*node + tier*12
				point := &GachaVoyagePoint{GachaStage: GachaStage{ID: id, Name: name, Img: base.Img, Cost: base.Cost + tier*2, EnemyLv: level, Waves: 1 + tier, EnemyCount: 2 + index/2, StarRound: 6 + tier*2, Exp: base.Exp + 10*node + 45*tier, HasBoss: node == len(names)-1, EnemyElem: region.element}, Kind: mode, Description: "沿航标穿过" + region.landmark + "，夺回本段航道。", CaptainMin: []int{0, 8, 15}[tier], Prize: []GachaVoyagePrize{{"gold", 90 + 45*number + 60*tier}, {[]string{"training_ink", "gear_rivet", "break_seal"}[tier], number + 1}}}
				if tier > 0 || point.HasBoss {
					point.Mechanic = region.mechanic
				}
				if tier == 1 {
					point.DailyLimit = 3
				}
				route := chapter.Routes[mode]
				if len(route) > 0 {
					point.Requires = []string{route[len(route)-1]}
				} else {
					prior := chapter.Routes[[]string{"", "story", "hard"}[tier]]
					point.Requires = []string{prior[len(prior)-1]}
					if index > 0 {
						previous := book.Chapters[index-1].Routes[mode]
						point.Requires = append(point.Requires, previous[len(previous)-1])
					}
				}
				book.Stages[id] = point
				chapter.Routes[mode] = append(route, id)
			}
			length := len(chapter.Routes[mode])
			for step := 1; step <= 3; step++ {
				prizes := []GachaVoyagePrize{{"gold", 150 * number * step}, {[]string{"training_manual", "forge_ore", "route_badge"}[step-1], (tier + 1) * step}}
				chapter.Chests[mode] = append(chapter.Chests[mode], GachaVoyageChest{ID: fmt.Sprintf("%s-%s-%d", chapter.ID, mode, step), Stars: length * step, Prize: prizes})
			}
		}
		book.Chapters = append(book.Chapters, chapter)
	}
	resources := []struct {
		id, name, gate, elem, mechanic, description string
		cost, level, waves, enemies                 int
		prize                                       []GachaVoyagePrize
	}{
		{"res_manifest", "领航抄录所", "s1", "plant", "", "夺回漂流航志，为新伙伴备齐经验书。", 6, 12, 1, 2, []GachaVoyagePrize{{"field_notes", 2}, {"training_manual", 1}}},
		{"res_forge", "浮船锻造厂", "s5", "fire", "fury", "趁炉心过热前击退守卫，拖回强化与升星工料。", 8, 22, 2, 2, []GachaVoyagePrize{{"gear_plate", 6}, {"gear_thread", 6}, {"gear_rivet", 3}}},
		{"res_ink", "潮墨档案库", "s2", "water", "barrier", "破开水幕，取回训练卷宗与晋阶印章。", 9, 30, 1, 3, []GachaVoyagePrize{{"training_ink", 5}, {"break_seal", 1}}},
		{"res_caravan", "星砂护商线", "s4", "time", "silence", "在静默潮带护送商队归港，回收钱箱与行旅凭证。", 10, 38, 2, 3, []GachaVoyagePrize{{"gold", 1200}, {"route_badge", 3}}},
		{"res_mine", "深海星锻井", "s3", "dark", "venom", "深入侵蚀矿层，打捞洗练矿料与觉醒核心。", 12, 48, 2, 2, []GachaVoyagePrize{{"forge_ore", 7}, {"weapon_core", 1}}},
		{"res_salvage", "棱镜打捞湾", "s6", "star", "reflection", "避开反射屏障，从沉船保险舱取出宝芯凭券。", 14, 60, 2, 3, []GachaVoyagePrize{{"gem_ticket", 1}, {"chip_ticket", 1}}},
	}
	for index, supply := range resources {
		point := &GachaVoyagePoint{GachaStage: GachaStage{ID: supply.id, Name: supply.name, Img: gachaCatalog.Stages[index].Img, Cost: supply.cost, EnemyLv: supply.level, Waves: supply.waves, EnemyCount: supply.enemies, StarRound: 6 + supply.waves, Exp: supply.level * 3, EnemyElem: supply.elem, Mechanic: supply.mechanic, HasBoss: index >= 4}, Kind: "resources", Description: supply.description, Requires: []string{supply.gate}, DailyLimit: 3, Prize: supply.prize}
		book.Stages[supply.id] = point
		book.Resources = append(book.Resources, supply.id)
	}
	for _, point := range book.Stages {
		point.SweepPrize = gachaSweepPrizes(point)
	}
	return book
}

func checkGachaVoyagePoint(state *GachaGameState, point *GachaVoyagePoint) error {
	if point == nil {
		return nil
	}
	if point.CaptainMin > 0 && gachaCaptainLevel(state) < point.CaptainMin {
		return ErrGachaLocked
	}
	if point.UnlockStage != "" && !state.Cleared[point.UnlockStage] {
		return ErrGachaLocked
	}
	for _, previous := range point.Requires {
		if !state.Cleared[previous] {
			return ErrGachaLocked
		}
	}
	if point.ChapterGate != "" && !state.VoyagePasses[point.ID] && !state.Cleared[point.ID] && !state.Cleared[point.ChapterGate] {
		return ErrGachaLocked
	}
	if point.DailyLimit > 0 && state.Daily.VoyageAttempts[point.ID] >= point.DailyLimit {
		return ErrGachaVoyageLimit
	}
	return nil
}

func migrateGachaVoyage(state *GachaGameState) {
	// A star record is meaningful only after a confirmed clear. Never turn a
	// stray star value into a clear or use it to unlock stages and chests.
	for id, stars := range state.Stars {
		if !state.Cleared[id] || stars <= 0 {
			delete(state.Stars, id)
		} else if stars > gachaMaxStars {
			state.Stars[id] = gachaMaxStars
		}
	}
	if state.VoyageEdition != 0 {
		return
	}
	state.VoyageEdition = 1
	for _, chapter := range gachaVoyage.Chapters[1:] {
		id := chapter.Routes["story"][0]
		if state.Cleared[gachaVoyage.Stages[id].UnlockStage] {
			if state.VoyagePasses == nil {
				state.VoyagePasses = map[string]bool{}
			}
			state.VoyagePasses[id] = true
		}
	}
}

func grantGachaVoyagePrize(state *GachaGameState, prizes []GachaVoyagePrize) ([]string, error) {
	for _, prize := range prizes {
		current := state.Mats[prize.Item]
		if prize.Item == "gold" {
			current = state.Gold
		} else if gachaItems.Items[prize.Item].Name == "" {
			return nil, ErrGachaRunState
		}
		if prize.Amount < 1 || current < 0 || current > gachaMaxResource-prize.Amount {
			return nil, ErrGachaCapacity
		}
	}
	var drops []string
	for _, prize := range prizes {
		name := "金币"
		if prize.Item == "gold" {
			state.Gold += prize.Amount
		} else {
			state.Mats[prize.Item] += prize.Amount
			name = gachaItems.Items[prize.Item].Name
		}
		drops = append(drops, fmt.Sprintf("%s x%d", name, prize.Amount))
	}
	return drops, nil
}

func findGachaVoyageChest(id string) (*GachaVoyageChest, []string) {
	for _, chapter := range gachaVoyage.Chapters {
		for mode, chests := range chapter.Chests {
			for _, chest := range chests {
				if chest.ID == id {
					return &chest, chapter.Routes[mode]
				}
			}
		}
	}
	return nil, nil
}

func validateGachaVoyageIntent(intent GachaActionIntent) error {
	if intent.ItemID == nil || intent.Count != nil || intent.GirlID != nil || intent.Squad != nil || intent.Enabled != nil || intent.OpponentID != nil || intent.StakeE != nil || intent.QueueID != nil || intent.RoomID != nil || intent.Choice != nil || intent.Config != nil || intent.Materials != nil {
		return ErrGachaIntent
	}
	if chest, _ := findGachaVoyageChest(*intent.ItemID); chest == nil {
		return ErrGachaIntent
	}
	return nil
}

func claimGachaVoyageChest(state *GachaGameState, id string, result *GachaActionResult) error {
	chest, route := findGachaVoyageChest(id)
	if chest == nil {
		return ErrGachaIntent
	}
	stars := 0
	for _, point := range route {
		if state.Cleared[point] {
			stars += min(gachaMaxStars, max(0, state.Stars[point]))
		}
	}
	if state.VoyageClaims[id] || stars < chest.Stars {
		return ErrGachaVoyageChest
	}
	drops, err := grantGachaVoyagePrize(state, chest.Prize)
	if err != nil {
		return err
	}
	if state.VoyageClaims == nil {
		state.VoyageClaims = map[string]bool{}
	}
	state.VoyageClaims[id] = true
	result.Drops = drops
	return nil
}

func awardGachaVoyageMilestones(state *GachaGameState, point *GachaVoyagePoint, stars int, result *GachaFinishResult) error {
	if point == nil || point.Kind == "resources" {
		return nil
	}
	var prizes []GachaVoyagePrize
	if !state.Cleared[point.ID] {
		prizes = slices.Clone(gachaVoyage.FirstClear)
	}
	if stars == gachaMaxStars && state.Stars[point.ID] < gachaMaxStars {
		prizes = append(prizes, gachaVoyage.FirstThree...)
	}
	drops, err := grantGachaVoyagePrize(state, prizes)
	result.Drops = append(result.Drops, drops...)
	return err
}

func settleGachaVoyagePoint(state *GachaGameState, point *GachaVoyagePoint, snapshot *GachaRunSnapshot, report *GachaBattleReport, result *GachaFinishResult) error {
	if !result.Win || !report.Win {
		return nil
	}
	drops, err := grantGachaVoyagePrize(state, point.Prize)
	if err != nil {
		return err
	}
	result.Drops = append(result.Drops, drops...)
	if err := awardGachaExperience(state, snapshot.Squad, point.Exp, &result.Drops); err != nil {
		return err
	}
	result.Stars = report.Stars
	result.IsBest = report.Stars > state.Stars[point.ID]
	state.Cleared[point.ID] = true
	state.Stars[point.ID] = max(report.Stars, state.Stars[point.ID])
	settleGachaVigil(state, &point.GachaStage, snapshot, report, result)
	if point.Kind != "resources" {
		settleGachaDispatch(state, &point.GachaStage, snapshot, report, result)
	}
	return nil
}

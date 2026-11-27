package model

import (
	"context"
	crand "crypto/rand"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"math/big"
	"slices"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

// Credentials and browser saves are never accepted as game state. All writes
// serialize on this user's row, with a revision CAS also protecting SQLite.
type GachaPlayer struct {
	UserID       int    `gorm:"primaryKey;autoIncrement:false"`
	State        string `gorm:"type:text;not null"`
	Revision     int64  `gorm:"not null"`
	Power        int    `gorm:"not null;default:0;index:idx_gacha_power"`
	PowerVersion int    `gorm:"not null;default:0"`
	ArenaPoints  int    `gorm:"not null;default:0;index:idx_gacha_arena_points"`
	CreatedAt    int64
	UpdatedAt    int64
}

// Battle playback may exceed MySQL TEXT's 64 KiB limit.
type GachaDocument string

func (GachaDocument) GormDataType() string { return "string" }
func (GachaDocument) GormDBDataType(db *gorm.DB, _ *schema.Field) string {
	if db.Dialector.Name() == "mysql" {
		return "LONGTEXT"
	}
	return "TEXT"
}

type GachaAction struct {
	UserID     int           `gorm:"primaryKey;autoIncrement:false"`
	RequestID  string        `gorm:"type:varchar(36);primaryKey"`
	IntentHash string        `gorm:"type:varchar(64);not null"`
	Result     GachaDocument `gorm:"not null"`
	CreatedAt  int64
}

type GachaOwnedGirl struct {
	Star      int      `json:"star"`
	Level     int      `json:"level"`
	Exp       int      `json:"exp"`
	Shards    int      `json:"shards"`
	Sockets   int      `json:"sockets"`
	Gems      []string `json:"gems"`
	Talent    string   `json:"talent,omitempty"`
	Sigil     string   `json:"sigil,omitempty"`
	Awakening int      `json:"awakening,omitempty"`
	Branch    string   `json:"branch,omitempty"`
}
type GachaGameDaily struct {
	Date           string         `json:"date"`
	TokenGained    int            `json:"tokenGained"`
	ShopBought     map[string]int `json:"shopBought,omitempty"`
	ArenaOpponents []int          `json:"arenaOpponents,omitempty"`
	ArenaPlayed    int            `json:"arenaPlayed"`
	GuildAttacked  bool           `json:"guild_attacked"`
	RelicRuns      int            `json:"relic_runs"`
}
type GachaPullHistory struct {
	Name   string `json:"n"`
	Rarity string `json:"r"`
}
type GachaGameState struct {
	Stamina          int                        `json:"stamina"`
	StaminaMax       int                        `json:"staminaMax"`
	StaminaTs        int64                      `json:"staminaTs"`
	Token            int                        `json:"token"`
	Gold             int                        `json:"gold"`
	Multi            int                        `json:"multi"`
	MultiOn          bool                       `json:"multiOn"`
	Daily            GachaGameDaily             `json:"daily"`
	Pity             int                        `json:"pity"`
	TotalPull        int                        `json:"totalPull"`
	Girls            map[string]*GachaOwnedGirl `json:"girls"`
	Squad            []string                   `json:"squad"`
	Teams            [][]string                 `json:"teams"`
	TeamOn           int                        `json:"team_on"`
	Cleared          map[string]bool            `json:"cleared"`
	Stars            map[string]int             `json:"stars"`
	History          []GachaPullHistory         `json:"history"`
	Mats             map[string]int             `json:"mats"`
	GemInventory     map[string]int             `json:"gem_inventory"`
	Sigils           map[string]*GachaSigil     `json:"sigils"`
	Activity         GachaRunActivity           `json:"activity"`
	Power            int                        `json:"power"`
	Arena            GachaArenaState            `json:"arena"`
	Expedition       GachaExpeditionState       `json:"expedition,omitempty"`
	WeeklyBoss       GachaWeeklyBossState       `json:"weekly_boss,omitempty"`
	Guild            *GachaGuildState           `json:"guild,omitempty"`
	GuildRewardWeek  string                     `json:"guild_reward_week,omitempty"`
	Progression      *GachaProgressionState     `json:"progression,omitempty"`
	ProgressionRules GachaProgressionConfig     `json:"progression_rules"`
	BattleBuff       string                     `json:"-"`
}
type GachaStats struct {
	HP  int `json:"hp"`
	Atk int `json:"atk"`
	Def int `json:"def"`
	Spd int `json:"spd"`
	CR  int `json:"cr"`
	CD  int `json:"cd"`
	EHR int `json:"ehr"`
	Res int `json:"res"`
}

type GachaGemSpec struct {
	Name   string
	Effect string
}

var gachaGemSpecs = map[string]GachaGemSpec{
	"ruby":     {Name: "赤曜石", Effect: "攻击 +5%"},
	"jade":     {Name: "生命翡翠", Effect: "生命 +5%"},
	"sapphire": {Name: "坚壁蓝宝石", Effect: "防御 +5%"},
	"wind":     {Name: "风灵晶", Effect: "速度 +3"},
	"amethyst": {Name: "紫晶", Effect: "暴击 +3%"},
}

var gachaSocketGoldCosts = [3]int{200, 600, 1500}
var gachaSocketMaterialCosts = [3]int{5, 12, 25}

type GachaSkill struct {
	Name          string  `json:"name"`
	Target        string  `json:"target"`
	Mult          float64 `json:"mult"`
	HealMult      float64 `json:"healMult"`
	ShieldMult    float64 `json:"shieldMult"`
	StunChance    float64 `json:"stunChance"`
	GoldBonus     float64 `json:"goldBonus"`
	Hits          int     `json:"hits"`
	Slow          int     `json:"slow"`
	SlowTurns     int     `json:"slowTurns"`
	DefBreak      int     `json:"defBreak"`
	DefBreakTurns int     `json:"defBreakTurns"`
	StunTurns     int     `json:"stunTurns"`
	IgnoreDefPct  int     `json:"ignoreDefPct"`
	SelfDefUp     int     `json:"selfDefUp"`
	TeamAtkUp     int     `json:"teamAtkUp"`
	BuffTurns     int     `json:"buffTurns"`
	Taunt         bool    `json:"taunt"`
	ExtraTurn     bool    `json:"extraTurn"`
	Cleanse       int     `json:"cleanse,omitempty"`
	RageGain      int     `json:"rageGain,omitempty"`
	RageDrain     int     `json:"rageDrain,omitempty"`
	DotPct        float64 `json:"dotPct,omitempty"`
	DotCap        float64 `json:"dotCap,omitempty"`
	ShieldReflect float64 `json:"shieldReflect,omitempty"`
	LowHPBonus    float64 `json:"lowHPBonus,omitempty"`
	LowHPShield   float64 `json:"lowHPShield,omitempty"`
}
type GachaSkills struct {
	Normal GachaSkill `json:"normal"`
	Skill  GachaSkill `json:"skill"`
}
type GachaGirl struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Rarity   string      `json:"rarity"`
	Elem     string      `json:"elem"`
	Role     string      `json:"role"`
	AtkType  string      `json:"atkType"`
	Img      string      `json:"img"`
	Base     GachaStats  `json:"base"`
	MatBonus float64     `json:"matBonus"`
	DupGain  int         `json:"dupGain"`
	Skills   GachaSkills `json:"skills"`
}
type GachaStage struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Img         string  `json:"img"`
	Cost        int     `json:"cost"`
	EnemyLv     int     `json:"enemyLv"`
	Waves       int     `json:"waves"`
	EnemyCount  int     `json:"enemyCount"`
	StarRound   int     `json:"starRound"`
	Exp         int     `json:"exp"`
	HasBoss     bool    `json:"hasBoss"`
	TokenRange  [2]int  `json:"tokenRange"`
	Gold        [2]int  `json:"gold"`
	Mat         [2]int  `json:"mat"`
	ShardR      float64 `json:"shardR"`
	ShardSR     float64 `json:"shardSR"`
	ShardSSR    float64 `json:"shardSSR"`
	MatKey      string  `json:"matKey"`
	UnlockStage string  `json:"unlockStage"`
	BossElem    string  `json:"-"`
	BossName    string  `json:"-"`
	BossHPM     float64 `json:"-"`
	BossAtkM    float64 `json:"-"`
	LootFamily  string  `json:"lootFamily,omitempty"`
	Mechanic    string  `json:"mechanic,omitempty"`
	EnemyElem   string  `json:"enemyElem,omitempty"`
	HPScale     float64 `json:"hpScale,omitempty"`
	AtkScale    float64 `json:"atkScale,omitempty"`
	DefScale    float64 `json:"defScale,omitempty"`
	EnemyRes    int     `json:"enemyRes,omitempty"`
	RelicTier   int     `json:"relicTier,omitempty"`
	RelicChance float64 `json:"relicChance,omitempty"`
	Essence     [2]int  `json:"essence,omitempty"`
}
type GachaRarity struct {
	BaseStar      int `json:"baseStar"`
	ShardToUnlock int `json:"shardToUnlock"`
}
type GachaStatGrowth struct {
	HP  float64 `json:"hp"`
	Atk float64 `json:"atk"`
	Def float64 `json:"def"`
	Spd float64 `json:"spd"`
}
type GachaGrowth struct {
	ExpPerLevel   int             `json:"expPerLevel"`
	MaxLevel      int             `json:"maxLevel"`
	StarMax       int             `json:"starMax"`
	StarCost      map[int]int     `json:"starCost"`
	LevelStatGrow GachaStatGrowth `json:"levelStatGrow"`
	StarStatGrow  GachaStatGrowth `json:"starStatGrow"`
}
type GachaFormula struct {
	DefK          int     `json:"defK"`
	DefLv         int     `json:"defLv"`
	RagePerNormal int     `json:"ragePerNormal"`
	RageMax       int     `json:"rageMax"`
	CritBase      float64 `json:"critBase"`
}
type GachaEnemyTemplate struct {
	Name    string  `json:"name"`
	Img     string  `json:"img"`
	Elem    string  `json:"elem"`
	AtkType string  `json:"atkType"`
	HPM     float64 `json:"hpM"`
	AtkM    float64 `json:"atkM"`
	DefM    float64 `json:"defM"`
	Spd     int     `json:"spd"`
	Boss    bool    `json:"boss"`
}
type GachaSummonConfig struct {
	Cost1     int     `json:"cost1"`
	Cost10    int     `json:"cost10"`
	WeightR   float64 `json:"weightR"`
	WeightSR  float64 `json:"weightSR"`
	WeightSSR float64 `json:"weightSSR"`
	PitySSR   int     `json:"pitySSR"`
}
type GachaCatalog struct {
	Girls           []GachaGirl            `json:"GIRLS"`
	Stages          []GachaStage           `json:"STAGES"`
	Rarity          map[string]GachaRarity `json:"RARITY"`
	Gacha           GachaSummonConfig      `json:"GACHA"`
	Growth          GachaGrowth            `json:"GROWTH"`
	Formula         GachaFormula           `json:"FORMULA"`
	Enemies         []GachaEnemyTemplate   `json:"ENEMY_TPL"`
	MatByRarity     map[string]string      `json:"MAT_BY_RARITY"`
	MatStarCost     map[string]int         `json:"MAT_STAR_COST"`
	TokenBudget     int                    `json:"TOKEN_BUDGET"`
	StaminaRegenMin int                    `json:"STAMINA_REGEN_MIN"`
}

//go:embed gacha_catalog.json
var gachaCatalogJSON []byte
var gachaCatalog = func() *GachaCatalog {
	var catalog GachaCatalog
	if err := common.Unmarshal(gachaCatalogJSON, &catalog); err != nil {
		panic(err)
	}
	if len(catalog.Girls) != 28 || len(catalog.Stages) != 24 || catalog.StaminaRegenMin < 1 {
		panic("invalid gacha catalog")
	}
	return &catalog
}()

var (
	ErrGachaIntent     = errors.New("invalid game intent")
	ErrGachaFunds      = errors.New("not enough game resources")
	ErrGachaLocked     = errors.New("dungeon stage is locked")
	ErrGachaTeam       = errors.New("invalid owned team")
	ErrGachaTeamShared = errors.New("girl already fields in another preset team")
	ErrGachaNotReady   = errors.New("dungeon battle has not finished")
	ErrGachaReplay     = errors.New("request id was reused for a different intent")
	ErrGachaConcurrent = errors.New("game state changed concurrently; retry with the same request id")
)

const gachaMaxResource = 1_000_000_000

type GachaActionIntent struct {
	RequestID  string                  `json:"request_id"`
	Action     string                  `json:"action"`
	Count      *int                    `json:"count,omitempty"`
	GirlID     *string                 `json:"girl_id,omitempty"`
	Squad      *[]string               `json:"squad,omitempty"`
	Enabled    *bool                   `json:"enabled,omitempty"`
	ItemID     *string                 `json:"item_id,omitempty"`
	OpponentID *int                    `json:"opponent_id,omitempty"`
	StakeE     *int                    `json:"stake_e,omitempty"`
	QueueID    *string                 `json:"queue_id,omitempty"`
	RoomID     *string                 `json:"room_id,omitempty"`
	Choice     *string                 `json:"choice,omitempty"`
	Config     *GachaProgressionConfig `json:"config,omitempty"`
}
type GachaPullResult struct {
	GirlID string `json:"girl_id"`
	IsNew  bool   `json:"is_new"`
	Dup    bool   `json:"dup"`
	Shards int    `json:"shards"`
}
type GachaActionResult struct {
	Save             *GachaGameState            `json:"save"`
	Revision         int64                      `json:"revision"`
	Drops            []string                   `json:"drops,omitempty"`
	Guild            *GachaGuildView            `json:"guild,omitempty"`
	Results          []GachaPullResult          `json:"results,omitempty"`
	Arena            *GachaArenaResult          `json:"arena,omitempty"`
	Queue            *GachaArenaQueueView       `json:"queue,omitempty"`
	Room             *GachaArenaRoomView        `json:"room,omitempty"`
	Exchange         *GachaTokenExchangeReceipt `json:"exchange,omitempty"`
	Decomposition    *GachaShardDecomposition   `json:"decomposition,omitempty"`
	Sigil            *GachaSigil                `json:"sigil,omitempty"`
	ProgressionRules *GachaProgressionConfig    `json:"progression_rules,omitempty"`
}
type GachaShardDecomposition struct {
	GirlID string `json:"girl_id"`
	Shards int    `json:"shards"`
	Dust   int    `json:"dust"`
	Rarity string `json:"rarity,omitempty"`
}
type GachaRunResult struct {
	ID       string             `json:"id"`
	Stage    string             `json:"stage"`
	ReadyAt  int64              `json:"ready_at"`
	ExpireAt int64              `json:"expire_at"`
	Result   *GachaBattleReport `json:"result"`
	Save     *GachaGameState    `json:"save"`
	Revision int64              `json:"revision"`
	Scene    *GachaRunScene     `json:"scene,omitempty"`
}
type GachaRunScene struct {
	Name      string `json:"name"`
	Img       string `json:"img"`
	StarRound int    `json:"star_round"`
}
type GachaRunSnapshot struct {
	Version            int         `json:"version"`
	Squad              []string    `json:"squad"`
	MultiUsed          bool        `json:"multi_used"`
	Mode               string      `json:"mode,omitempty"`
	Week               string      `json:"week,omitempty"`
	Floor              int         `json:"floor,omitempty"`
	Route              string      `json:"route,omitempty"`
	RulesVersion       int         `json:"rules_version,omitempty"`
	RelicEligible      bool        `json:"relic_eligible,omitempty"`
	ProgressionVersion int         `json:"progression_version,omitempty"`
	ProgressionStage   *GachaStage `json:"progression_stage,omitempty"`
	ProgressionGirl    string      `json:"progression_girl,omitempty"`
	ProgressionRank    int         `json:"progression_rank,omitempty"`
	TrainingDay        string      `json:"training_day,omitempty"`
	DualSquads         [][]string  `json:"dual_squads,omitempty"`
}

func gachaGirl(id string) *GachaGirl {
	for i := range gachaCatalog.Girls {
		if gachaCatalog.Girls[i].ID == id {
			return &gachaCatalog.Girls[i]
		}
	}
	return nil
}

func gachaGem(id string) (GachaGemSpec, bool) {
	spec, ok := gachaGemSpecs[id]
	return spec, ok
}

func gachaStage(id string) *GachaStage {
	for i := range gachaCatalog.Stages {
		if gachaCatalog.Stages[i].ID == id {
			return &gachaCatalog.Stages[i]
		}
	}
	return nil
}
func newGachaState(now time.Time) *GachaGameState {
	s := &GachaGameState{Stamina: 120, StaminaMax: 120, StaminaTs: now.UnixMilli(), Gold: 600, Multi: 1,
		Daily: GachaGameDaily{Date: gachaDay(now)}, Girls: map[string]*GachaOwnedGirl{}, Squad: []string{"c08", "c06", "c01", "c12"},
		Cleared: map[string]bool{}, Stars: map[string]int{}, History: []GachaPullHistory{}, Mats: map[string]int{"mat_r": 0, "mat_sr": 0, "mat_ssr": 0, "shard_dust": 0}, GemInventory: map[string]int{}, Sigils: map[string]*GachaSigil{}}
	s.Teams = [][]string{slices.Clone(s.Squad), {}, {}, {}}
	s.TeamOn = 1
	for _, id := range s.Squad {
		s.Girls[id] = &GachaOwnedGirl{Star: 1, Level: 1, Gems: make([]string, 3)}
	}
	return s
}
// normalizeGachaTeams keeps four preset slots and mirrors the active slot into
// Squad, which remains the single source every combat path consumes.
func normalizeGachaTeams(s *GachaGameState) {
	if len(s.Teams) == 0 {
		// Legacy save without presets: the live squad becomes slot one.
		s.Teams = [][]string{slices.Clone(s.Squad)}
		s.TeamOn = 1
	}
	if len(s.Teams) > 4 {
		s.Teams = s.Teams[:4]
	}
	for len(s.Teams) < 4 {
		s.Teams = append(s.Teams, []string{})
	}
	for i, team := range s.Teams {
		if team == nil {
			s.Teams[i] = []string{}
		}
	}
	if s.TeamOn < 1 || s.TeamOn > 4 {
		s.TeamOn = 1
	}
	// Squad stays authoritative: every combat path reads only it, and any
	// legacy or out-of-band write lands there. Repair the active preset from
	// Squad so the two never disagree.
	s.Teams[s.TeamOn-1] = slices.Clone(s.Squad)
	dedupeGachaTeams(s)
}

// validateGachaTeamMembers rejects a squad the player cannot field: unknown,
// unowned, duplicated or under-levelled entries.
func validateGachaTeamMembers(s *GachaGameState, team []string) error {
	for i, id := range team {
		own := s.Girls[id]
		if own == nil || own.Star < 1 || own.Level < 1 || gachaGirl(id) == nil || slices.Contains(team[:i], id) {
			return ErrGachaTeam
		}
	}
	return nil
}

// gachaTeamConflict lists the girls a squad shares with any other preset.
func gachaTeamConflict(s *GachaGameState, slot int, team []string) []string {
	conflict := make([]string, 0, len(team))
	for _, id := range team {
		if gachaGirlInOtherTeam(s, slot, id) {
			conflict = append(conflict, id)
		}
	}
	return conflict
}

// setGachaActiveSquad is the one write path for the live preset. Every entry
// point (the squad editor, the growth page's dual-team save) goes through it so
// a girl can never be parked in two presets and then silently dropped on load.
func setGachaActiveSquad(s *GachaGameState, team []string) error {
	if s.TeamOn < 1 || s.TeamOn > 4 {
		s.TeamOn = 1
	}
	for len(s.Teams) < 4 {
		s.Teams = append(s.Teams, []string{})
	}
	if err := validateGachaTeamMembers(s, team); err != nil {
		return err
	}
	if len(gachaTeamConflict(s, s.TeamOn-1, team)) > 0 {
		return ErrGachaTeamShared
	}
	s.Teams[s.TeamOn-1] = slices.Clone(team)
	s.Squad = slices.Clone(team)
	return nil
}

// gachaGirlInOtherTeam reports whether another preset already fields the girl.
func gachaGirlInOtherTeam(s *GachaGameState, slot int, id string) bool {
	for i, team := range s.Teams {
		if i == slot {
			continue
		}
		if slices.Contains(team, id) {
			return true
		}
	}
	return false
}

// dedupeGachaTeams keeps a girl in exactly one preset. The active slot wins:
// a save that lists her twice keeps her where the player is actually fighting
// and repairs the other slot, so loading never strips the live team.
func dedupeGachaTeams(s *GachaGameState) {
	used := map[string]bool{}
	order := make([]int, 0, len(s.Teams))
	order = append(order, s.TeamOn-1)
	for i := range s.Teams {
		if i != s.TeamOn-1 {
			order = append(order, i)
		}
	}
	for _, i := range order {
		team := s.Teams[i]
		kept := make([]string, 0, len(team))
		for _, id := range team {
			// Drop stale entries before validation: an older save can still list
			// a girl who was later lost, and a hard validation error would lock
			// the account out of every action instead of repairing the roster.
			own := s.Girls[id]
			if id == "" || used[id] || own == nil || own.Star < 1 || own.Level < 1 || gachaGirl(id) == nil {
				continue
			}
			used[id] = true
			kept = append(kept, id)
		}
		if len(kept) > 4 {
			// Presets field four girls: a tampered save must not fail validation
			// and lock the account out, so clamp instead of rejecting.
			kept = kept[:4]
		}
		s.Teams[i] = kept
	}
	s.Squad = slices.Clone(s.Teams[s.TeamOn-1])
}
func regenGachaState(s *GachaGameState, now time.Time) {
	if s.Daily.Date != gachaDay(now) {
		s.Daily = GachaGameDaily{Date: gachaDay(now)}
	}
	nowMS := now.UnixMilli()
	if s.Stamina >= s.StaminaMax {
		s.StaminaTs = nowMS
		return
	}
	interval := int64(gachaCatalog.StaminaRegenMin) * 60_000
	if elapsed := nowMS - s.StaminaTs; elapsed >= interval {
		add := min(elapsed/interval, int64(s.StaminaMax-s.Stamina))
		s.Stamina += int(add)
		s.StaminaTs += add * interval
		if s.Stamina >= s.StaminaMax {
			s.StaminaTs = nowMS
		}
	}
}
func loadGachaPlayer(tx *gorm.DB, userID int, now time.Time) (*GachaPlayer, *GachaGameState, error) {
	if userID <= 0 {
		return nil, nil, ErrGachaIntent
	}
	initial, err := common.Marshal(newGachaState(now))
	if err != nil {
		return nil, nil, err
	}
	created := GachaPlayer{UserID: userID, State: string(initial), Revision: 1, CreatedAt: now.UnixMilli(), UpdatedAt: now.UnixMilli()}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&created).Error; err != nil {
		return nil, nil, err
	}
	var player GachaPlayer
	if err := lockForUpdate(tx).Where("user_id = ?", userID).First(&player).Error; err != nil {
		return nil, nil, err
	}
	// Establish MySQL's consistent-read snapshot only after serialization. A
	// pre-lock read could miss the action/run committed while this request waited.
	var user User
	if err := tx.Select("id", "status").First(&user, userID).Error; err != nil {
		return nil, nil, err
	}
	if user.Status != common.UserStatusEnabled {
		return nil, nil, ErrGachaIntent
	}
	var state GachaGameState
	if err := common.Unmarshal([]byte(player.State), &state); err != nil {
		return nil, nil, err
	}
	if state.Girls == nil || state.Mats == nil || state.Stars == nil || state.Cleared == nil || state.StaminaMax != 120 || state.Stamina < 0 || state.Stamina > 120 || state.Gold < 0 || state.Gold > gachaMaxResource || state.Token < 0 || state.Token > gachaMaxResource || state.Multi < 0 || state.Multi > gachaMaxResource {
		return nil, nil, ErrGachaRunState
	}
	if state.GemInventory == nil {
		state.GemInventory = map[string]int{}
	}
	normalizeGachaTeams(&state)
	regenGachaState(&state, now)
	if err := validateGachaBuildState(&state); err != nil {
		return nil, nil, err
	}
	resetGachaWeeklyModes(&state, now)
	if state.Expedition.Floor < 1 || state.Expedition.Floor > 11 || state.Expedition.Best < 0 || state.Expedition.Best > 10 || state.WeeklyBoss.Attempts < 0 || state.WeeklyBoss.Attempts > 3 || state.WeeklyBoss.BestDamage < 0 {
		return nil, nil, ErrGachaRunState
	}
	for id, count := range state.GemInventory {
		if _, ok := gachaGem(id); !ok || count < 0 || count > gachaMaxResource {
			return nil, nil, ErrGachaRunState
		}
	}
	for _, own := range state.Girls {
		if own == nil || own.Sockets < 0 || own.Sockets > 3 {
			return nil, nil, ErrGachaRunState
		}
		if own.Gems == nil {
			own.Gems = make([]string, 3)
		}
		if len(own.Gems) != 3 {
			return nil, nil, ErrGachaRunState
		}
		if own.Talent != "" && (!slices.Contains(gachaTalentIDs, own.Talent) || own.Star < 3) {
			return nil, nil, ErrGachaRunState
		}
		for slot, gemID := range own.Gems {
			if slot >= own.Sockets && gemID != "" {
				return nil, nil, ErrGachaRunState
			}
			if gemID != "" {
				if _, ok := gachaGem(gemID); !ok {
					return nil, nil, ErrGachaRunState
				}
			}
		}
	}
	if dust, ok := state.Mats["shard_dust"]; ok && (dust < 0 || dust > gachaMaxResource) {
		return nil, nil, ErrGachaRunState
	}
	if _, ok := state.Mats["shard_dust"]; !ok {
		state.Mats["shard_dust"] = 0
	}
	rules, err := loadGachaProgressionConfig(tx)
	if err != nil {
		return nil, nil, err
	}
	state.ProgressionRules = rules
	if err := normalizeGachaProgression(&state, now, rules); err != nil {
		return nil, nil, err
	}
	return &player, &state, nil
}
func storeGachaPlayer(tx *gorm.DB, player *GachaPlayer, state *GachaGameState, now time.Time) error {
	// Any write path that moves girls in or out (squad, dual-team save, story
	// recruits) updates Squad; mirror it back so the preset slots never drift.
	if len(state.Teams) == 4 && state.TeamOn >= 1 && state.TeamOn <= 4 {
		state.Teams[state.TeamOn-1] = slices.Clone(state.Squad)
	}
	if p := state.Progression; p != nil && !p.Legacy && p.Milestones["first_level60_squad"] == 0 && len(state.Squad) == 4 {
		complete := true
		for _, id := range state.Squad {
			complete = complete && state.Girls[id] != nil && state.Girls[id].Level >= 60
		}
		if complete {
			p.Milestones["first_level60_squad"] = now.UnixMilli()
		}
	}
	power, err := GachaSquadPower(state)
	if err != nil {
		return err
	}
	state.Power = power
	data, err := common.Marshal(state)
	if err != nil {
		return err
	}
	result := tx.Model(&GachaPlayer{}).Where("user_id = ? AND revision = ?", player.UserID, player.Revision).
		Updates(map[string]any{"state": string(data), "revision": player.Revision + 1, "updated_at": now.UnixMilli(),
			"power": power, "power_version": gachaPowerVersion, "arena_points": state.Arena.Points})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrGachaConcurrent
	}
	player.Revision++
	return nil
}
func gachaTransaction(run func(*gorm.DB) error) error {
	var err error
	for attempt := range 4 {
		err = DB.Transaction(run)
		if err == nil {
			return nil
		}
		text := strings.ToLower(err.Error())
		if !errors.Is(err, ErrGachaConcurrent) && !strings.Contains(text, "database is locked") && !strings.Contains(text, "deadlock") && !strings.Contains(text, "serialization failure") && !strings.Contains(text, "could not serialize") {
			return err
		}
		if attempt < 3 {
			time.Sleep(time.Duration(10*(attempt+1)) * time.Millisecond)
		}
	}
	return err
}
func validateGachaUUID(id string) bool {
	parsed, err := uuid.Parse(id)
	return err == nil && len(id) == 36 && parsed != uuid.Nil && parsed.String() == strings.ToLower(id)
}
func ValidateGachaAction(intent GachaActionIntent) error {
	if !validateGachaUUID(intent.RequestID) {
		return ErrGachaIntent
	}
	if slices.Contains(gachaProgressionActions, intent.Action) {
		return validateGachaProgressionIntent(intent)
	}
	if intent.Choice != nil || intent.Config != nil {
		return ErrGachaIntent
	}
	roomAction := slices.Contains([]string{"room_create", "room_join", "room_ready", "room_leave"}, intent.Action)
	itemAction := slices.Contains([]string{"shop", "gem_equip", "talent", "expedition_route", "guild_create", "guild_join", "sigil_equip", "sigil_salvage", "sigil_craft"}, intent.Action)
	if intent.OpponentID != nil || !itemAction && intent.ItemID != nil || intent.Action != "arena_join" && intent.Action != "room_create" && intent.StakeE != nil || intent.Action != "arena_cancel" && intent.QueueID != nil || !roomAction && intent.RoomID != nil {
		return ErrGachaIntent
	}
	switch intent.Action {
	case "token_exchange":
		if intent.Count == nil || *intent.Count < 1 || *intent.Count > gachaMaxResource || intent.GirlID != nil || intent.Squad != nil || intent.Enabled != nil {
			return ErrGachaIntent
		}
	case "room_create", "room_join", "room_ready", "room_leave":
		if intent.Count != nil || intent.GirlID != nil || intent.Squad != nil || intent.Enabled != nil {
			return ErrGachaIntent
		}
		if intent.Action == "room_create" {
			if intent.RoomID != nil || intent.StakeE == nil || *intent.StakeE < 1 || *intent.StakeE > 5 {
				return ErrGachaIntent
			}
		} else if intent.RoomID == nil || !validateGachaUUID(*intent.RoomID) {
			return ErrGachaIntent
		}
	case "shop":
		if intent.ItemID == nil || gachaShopItem(*intent.ItemID) == nil || intent.Count != nil || intent.GirlID != nil || intent.Squad != nil || intent.Enabled != nil {
			return ErrGachaIntent
		}
	case "arena_join", "arena_cancel":
		if intent.Count != nil || intent.GirlID != nil || intent.Squad != nil || intent.Enabled != nil {
			return ErrGachaIntent
		}
		if intent.Action == "arena_join" && (intent.StakeE == nil || *intent.StakeE < 1 || *intent.StakeE > 5) {
			return ErrGachaIntent
		}
		if intent.Action == "arena_cancel" && (intent.QueueID == nil || !validateGachaUUID(*intent.QueueID)) {
			return ErrGachaIntent
		}
	case "gacha":
		if intent.Count == nil || (*intent.Count != 1 && *intent.Count != 10) || intent.GirlID != nil || intent.Squad != nil || intent.Enabled != nil {
			return ErrGachaIntent
		}
	case "level", "star":
		if intent.GirlID == nil || gachaGirl(*intent.GirlID) == nil || intent.Count != nil || intent.Squad != nil || intent.Enabled != nil {
			return ErrGachaIntent
		}
	case "shard_decompose":
		if intent.GirlID == nil || gachaGirl(*intent.GirlID) == nil || intent.Count == nil || *intent.Count < 1 || *intent.Count > gachaMaxResource || intent.Squad != nil || intent.Enabled != nil {
			return ErrGachaIntent
		}
	case "gem_socket", "gem_remove":
		if intent.GirlID == nil || gachaGirl(*intent.GirlID) == nil || intent.Count == nil || *intent.Count < 1 || *intent.Count > 3 || intent.ItemID != nil || intent.Squad != nil || intent.Enabled != nil {
			return ErrGachaIntent
		}
	case "gem_equip":
		if intent.GirlID == nil || gachaGirl(*intent.GirlID) == nil || intent.Count == nil || *intent.Count < 1 || *intent.Count > 3 || intent.ItemID == nil || intent.Squad != nil || intent.Enabled != nil {
			return ErrGachaIntent
		}
		if _, ok := gachaGem(*intent.ItemID); !ok {
			return ErrGachaIntent
		}
	case "talent":
		if intent.GirlID == nil || gachaGirl(*intent.GirlID) == nil || intent.ItemID == nil || !slices.Contains(gachaTalentIDs, *intent.ItemID) || intent.Count != nil || intent.Squad != nil || intent.Enabled != nil {
			return ErrGachaIntent
		}
	case "sigil_equip", "sigil_remove":
		if intent.GirlID == nil || gachaGirl(*intent.GirlID) == nil || intent.Count != nil || intent.Squad != nil || intent.Enabled != nil {
			return ErrGachaIntent
		}
		if intent.Action == "sigil_equip" && (intent.ItemID == nil || !validGachaSigilID(*intent.ItemID)) || intent.Action == "sigil_remove" && intent.ItemID != nil {
			return ErrGachaIntent
		}
	case "sigil_salvage", "sigil_craft":
		if intent.ItemID == nil || intent.GirlID != nil || intent.Count != nil || intent.Squad != nil || intent.Enabled != nil {
			return ErrGachaIntent
		}
		if intent.Action == "sigil_salvage" && !validGachaSigilID(*intent.ItemID) {
			return ErrGachaIntent
		}
		if intent.Action == "sigil_craft" {
			if _, ok := gachaSigilFamilies[*intent.ItemID]; !ok {
				return ErrGachaIntent
			}
		}
	case "sigil_salvage_low":
		if intent.ItemID != nil || intent.GirlID != nil || intent.Count != nil || intent.Squad != nil || intent.Enabled != nil {
			return ErrGachaIntent
		}
	case "expedition_route":
		if intent.ItemID == nil || !slices.Contains([]string{"assault", "bastion", "fortune"}, *intent.ItemID) || intent.Count != nil || intent.GirlID != nil || intent.Squad != nil || intent.Enabled != nil {
			return ErrGachaIntent
		}
	case "guild_create":
		if intent.ItemID == nil || !validGachaGuildName(*intent.ItemID) || intent.Count != nil || intent.GirlID != nil || intent.Squad != nil || intent.Enabled != nil {
			return ErrGachaIntent
		}
	case "guild_join":
		if intent.ItemID == nil || gachaGuildLeaderFromCode(*intent.ItemID) == 0 || intent.Count != nil || intent.GirlID != nil || intent.Squad != nil || intent.Enabled != nil {
			return ErrGachaIntent
		}
	case "guild_leave", "guild_attack", "guild_claim":
		if intent.ItemID != nil || intent.Count != nil || intent.GirlID != nil || intent.Squad != nil || intent.Enabled != nil {
			return ErrGachaIntent
		}
	case "squad":
		if intent.Squad == nil || len(*intent.Squad) > 4 || intent.Count != nil || intent.GirlID != nil || intent.Enabled != nil {
			return ErrGachaIntent
		}
	case "multi":
		if intent.Enabled == nil || intent.Count != nil || intent.GirlID != nil || intent.Squad != nil {
			return ErrGachaIntent
		}
	case "team_select":
		if intent.Count == nil || *intent.Count < 1 || *intent.Count > 4 || intent.Squad != nil || intent.GirlID != nil || intent.Enabled != nil || intent.ItemID != nil {
			return ErrGachaIntent
		}
	default:
		return ErrGachaIntent
	}
	return nil
}
func gachaIntentHash(intent any) (string, error) {
	data, err := common.Marshal(intent)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}
func gachaReplayResult(tx *gorm.DB, userID int, requestID, hash string, result any) (bool, error) {
	var action GachaAction
	err := lockForUpdate(tx).Where("user_id = ? AND request_id = ?", userID, strings.ToLower(requestID)).First(&action).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if action.IntentHash != hash {
		return false, ErrGachaReplay
	}
	return true, common.Unmarshal([]byte(action.Result), result)
}
func recordGachaAction(tx *gorm.DB, userID int, requestID, hash string, result any, now time.Time) error {
	data, err := common.Marshal(result)
	if err != nil {
		return err
	}
	return tx.Create(&GachaAction{UserID: userID, RequestID: strings.ToLower(requestID), IntentHash: hash, Result: GachaDocument(data), CreatedAt: now.UnixMilli()}).Error
}
func gachaRandom(n int) (int, error) {
	if n <= 0 {
		return 0, ErrGachaIntent
	}
	value, err := crand.Int(crand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return 0, err
	}
	return int(value.Int64()), nil
}
func gachaRandomRange(bounds [2]int) (int, error) {
	if bounds[0] < 0 || bounds[1] < bounds[0] || bounds[1] > gachaMaxResource {
		return 0, ErrGachaIntent
	}
	n, err := gachaRandom(bounds[1] - bounds[0] + 1)
	return bounds[0] + n, err
}
func gachaChance(rate float64) (bool, error) {
	if math.IsNaN(rate) || math.IsInf(rate, 0) || rate < 0 || rate > 1 {
		return false, ErrGachaIntent
	}
	n, err := gachaRandom(1_000_000)
	return float64(n) < rate*1_000_000, err
}
func gachaRandomGirl(rarity string) (*GachaGirl, error) {
	pool := []*GachaGirl{}
	for i := range gachaCatalog.Girls {
		if gachaCatalog.Girls[i].Rarity == rarity {
			pool = append(pool, &gachaCatalog.Girls[i])
		}
	}
	n, err := gachaRandom(len(pool))
	if err != nil {
		return nil, err
	}
	return pool[n], nil
}
func ApplyGachaAction(userID int, intent GachaActionIntent, now time.Time) (*GachaActionResult, error) {
	if err := ValidateGachaAction(intent); err != nil {
		return nil, err
	}
	if intent.Action == "arena_join" || intent.Action == "arena_cancel" {
		return applyGachaQueueAction(userID, intent, now)
	}
	if strings.HasPrefix(intent.Action, "guild_") {
		return applyGachaGuildAction(userID, intent, now)
	}
	if strings.HasPrefix(intent.Action, "room_") {
		return applyGachaRoomAction(userID, intent, now)
	}
	intent.RequestID = strings.ToLower(intent.RequestID)
	hash, err := gachaIntentHash(intent)
	if err != nil {
		return nil, err
	}
	result := &GachaActionResult{}
	walletDelta := 0
	err = gachaTransaction(func(tx *gorm.DB) error {
		walletDelta = 0
		player, state, err := loadGachaPlayer(tx, userID, now)
		if err != nil {
			return err
		}
		if replay, err := gachaReplayResult(tx, userID, intent.RequestID, hash, result); replay || err != nil {
			return err
		}
		*result = GachaActionResult{}
		switch intent.Action {
		case "token_exchange":
			result.Exchange, err = exchangeGachaTokens(tx, userID, state, *intent.Count)
			if err != nil {
				return err
			}
			walletDelta = -result.Exchange.CostQuota
		case "shop":
			if err := buyGachaShopItem(state, *intent.ItemID); err != nil {
				return err
			}
		case "gacha":
			count := *intent.Count
			if state.Progression.Permits < count {
				return ErrGachaPermits
			}
			cost := gachaCatalog.Gacha.Cost1
			if count == 10 {
				cost = gachaCatalog.Gacha.Cost10
			}
			if state.Gold < cost || state.TotalPull > gachaMaxResource-count {
				return ErrGachaFunds
			}
			state.Gold -= cost
			state.Progression.Permits -= count
			tenGotSR := false
			for i := range count {
				rarity := "R"
				if state.Pity >= gachaCatalog.Gacha.PitySSR-1 {
					rarity = "SSR"
				} else {
					roll, err := gachaRandom(1_000_000)
					if err != nil {
						return err
					}
					if count == 10 && i == 9 && !tenGotSR {
						rarity = "SR"
						if float64(roll) < gachaCatalog.Gacha.WeightSSR/(1-gachaCatalog.Gacha.WeightR)*1_000_000 {
							rarity = "SSR"
						}
					} else if float64(roll) < gachaCatalog.Gacha.WeightSSR*1_000_000 {
						rarity = "SSR"
					} else if float64(roll) < (gachaCatalog.Gacha.WeightSSR+gachaCatalog.Gacha.WeightSR)*1_000_000 {
						rarity = "SR"
					}
				}
				state.Pity++
				state.TotalPull++
				if rarity == "SSR" {
					state.Pity = 0
				}
				if rarity != "R" {
					tenGotSR = true
				}
				girl, err := gachaRandomGirl(rarity)
				if err != nil {
					return err
				}
				own := state.Girls[girl.ID]
				pull := GachaPullResult{GirlID: girl.ID, IsNew: own == nil || own.Star == 0}
				if pull.IsNew {
					shards := 0
					if own != nil {
						shards = own.Shards
					}
					state.Girls[girl.ID] = &GachaOwnedGirl{Star: gachaCatalog.Rarity[rarity].BaseStar, Level: 1, Shards: shards, Gems: make([]string, 3)}
				} else {
					if own.Shards > gachaMaxResource-girl.DupGain {
						return ErrGachaFunds
					}
					own.Shards += girl.DupGain
					pull.Dup = true
					pull.Shards = girl.DupGain
				}
				result.Results = append(result.Results, pull)
			}
			history := make([]GachaPullHistory, 0, 12)
			for _, pull := range result.Results {
				girl := gachaGirl(pull.GirlID)
				history = append(history, GachaPullHistory{Name: girl.Name, Rarity: girl.Rarity})
			}
			state.History = append(history, state.History...)
			if len(state.History) > 12 {
				state.History = state.History[:12]
			}
		case "level", "star":
			girl := gachaGirl(*intent.GirlID)
			own := state.Girls[girl.ID]
			if own == nil || own.Star < 1 || own.Level < 1 {
				return ErrGachaIntent
			}
			if intent.Action == "level" {
				return ErrGachaExperience
			} else {
				if own.Star >= gachaCatalog.Growth.StarMax {
					return ErrGachaIntent
				}
				need := gachaStarShardCost(girl, own.Star+1)
				matKey := gachaCatalog.MatByRarity[girl.Rarity]
				needMat := gachaCatalog.MatStarCost[girl.Rarity]
				if need <= 0 || needMat <= 0 {
					return ErrGachaIntent
				}
				dustKey := gachaRarityDust(girl)
				if own.Shards+state.Mats[dustKey]+state.Mats["shard_dust"] < need || state.Mats[matKey] < needMat {
					return ErrGachaFunds
				}
				fromGirl := min(own.Shards, need)
				own.Shards -= fromGirl
				fromRarity := min(state.Mats[dustKey], need-fromGirl)
				state.Mats[dustKey] -= fromRarity
				state.Mats["shard_dust"] -= need - fromGirl - fromRarity
				state.Mats[matKey] -= needMat
				own.Star++
			}
		case "shard_decompose":
			girl := gachaGirl(*intent.GirlID)
			own := state.Girls[girl.ID]
			if own == nil {
				own = &GachaOwnedGirl{Gems: make([]string, 3)}
				state.Girls[girl.ID] = own
			}
			reserve := gachaCatalog.Rarity[girl.Rarity].ShardToUnlock
			if own.Star > 0 {
				reserve = 0
				if own.Star < gachaCatalog.Growth.StarMax {
					reserve = gachaStarShardCost(girl, own.Star+1)
				}
			}
			dustKey := gachaRarityDust(girl)
			if own.Shards < reserve || *intent.Count > own.Shards-reserve || state.Mats[dustKey] > gachaMaxResource-*intent.Count {
				return ErrGachaFunds
			}
			own.Shards -= *intent.Count
			state.Mats[dustKey] += *intent.Count
			result.Decomposition = &GachaShardDecomposition{GirlID: girl.ID, Shards: *intent.Count, Dust: *intent.Count, Rarity: girl.Rarity}
		case "gem_socket", "gem_equip", "gem_remove":
			girl := gachaGirl(*intent.GirlID)
			own := state.Girls[girl.ID]
			if own == nil || own.Star < 1 || own.Level < 1 || len(own.Gems) != 3 {
				return ErrGachaIntent
			}
			slot := *intent.Count - 1
			switch intent.Action {
			case "gem_socket":
				if slot != own.Sockets || own.Sockets >= len(own.Gems) {
					return ErrGachaIntent
				}
				goldCost := gachaSocketGoldCosts[slot]
				materialCost := gachaSocketMaterialCosts[slot]
				matKey := gachaCatalog.MatByRarity[girl.Rarity]
				if state.Gold < goldCost || state.Mats[matKey] < materialCost {
					return ErrGachaFunds
				}
				state.Gold -= goldCost
				state.Mats[matKey] -= materialCost
				own.Sockets++
			case "gem_equip":
				if slot >= own.Sockets {
					return ErrGachaIntent
				}
				oldGem := own.Gems[slot]
				newGem := *intent.ItemID
				if oldGem == newGem {
					break
				}
				if state.GemInventory[newGem] < 1 {
					return ErrGachaFunds
				}
				if oldGem != "" && state.GemInventory[oldGem] >= gachaMaxResource {
					return ErrGachaFunds
				}
				state.GemInventory[newGem]--
				if oldGem != "" {
					state.GemInventory[oldGem]++
				}
				own.Gems[slot] = newGem
			case "gem_remove":
				if slot >= own.Sockets || own.Gems[slot] == "" || state.GemInventory[own.Gems[slot]] >= gachaMaxResource {
					return ErrGachaIntent
				}
				state.GemInventory[own.Gems[slot]]++
				own.Gems[slot] = ""
			}
		case "sigil_equip", "sigil_remove", "sigil_salvage", "sigil_salvage_low", "sigil_craft":
			if err := applyGachaSigilAction(state, intent, result); err != nil {
				return err
			}
		case "talent":
			girl := gachaGirl(*intent.GirlID)
			own := state.Girls[girl.ID]
			if own == nil || own.Star < 3 || own.Level < 1 {
				return ErrGachaTalentLocked
			}
			if own.Talent == *intent.ItemID {
				break
			}
			cost := 500
			matCost := 5
			if own.Talent != "" {
				cost = 1000
				matCost = 10
			}
			matKey := gachaCatalog.MatByRarity[girl.Rarity]
			if state.Gold < cost || state.Mats[matKey] < matCost {
				return ErrGachaFunds
			}
			state.Gold -= cost
			state.Mats[matKey] -= matCost
			own.Talent = *intent.ItemID
		case "expedition_route":
			resetGachaWeeklyModes(state, now)
			if state.Expedition.Floor > 10 {
				return ErrGachaMode
			}
			var pending int64
			if err := tx.Model(&GachaRun{}).Where("user_id = ? AND status = ? AND expire_at > ?", userID, "running", now.UnixMilli()).Count(&pending).Error; err != nil {
				return err
			}
			if pending > 0 {
				return ErrGachaRunState
			}
			state.Expedition.Route = *intent.ItemID
		case "squad":
			team := slices.Clone(*intent.Squad)
			if team == nil {
				team = []string{}
			}
			if err := setGachaActiveSquad(state, team); err != nil {
				return err
			}
		case "team_select":
			if len(state.Teams) < 4 || *intent.Count < 1 || *intent.Count > 4 {
				return ErrGachaIntent
			}
			next := state.Teams[*intent.Count-1]
			for i, id := range next {
				own := state.Girls[id]
				if own == nil || own.Star < 1 || own.Level < 1 || gachaGirl(id) == nil || slices.Contains(next[:i], id) {
					return ErrGachaTeam
				}
			}
			state.TeamOn = *intent.Count
			state.Squad = slices.Clone(next)
		case "multi":
			if *intent.Enabled && state.Multi <= 0 {
				return ErrGachaFunds
			}
			state.MultiOn = *intent.Enabled
		default:
			if err := applyGachaProgressionAction(tx, userID, state, intent, result, now); err != nil {
				return err
			}
		}
		if err := storeGachaPlayer(tx, player, state, now); err != nil {
			return err
		}
		result.Save = state
		result.Revision = player.Revision
		return recordGachaAction(tx, userID, intent.RequestID, hash, result, now)
	})
	if err != nil {
		return nil, err
	}
	if walletDelta != 0 && common.RedisEnabled {
		if cacheErr := cacheIncrUserQuota(userID, int64(walletDelta)); cacheErr != nil {
			common.SysError(fmt.Sprintf("gacha token exchange cache sync failed for user %d: %v", userID, cacheErr))
		}
	}
	return result, nil
}

func gachaDailyLocked(tx *gorm.DB, userID int, day string, now time.Time) (*GachaDaily, error) {
	created := GachaDaily{Day: day, UserID: userID, UpdatedAt: now.UnixMilli()}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&created).Error; err != nil {
		return nil, err
	}
	var daily GachaDaily
	if err := lockForUpdate(tx).Where("day = ? AND user_id = ?", day, userID).First(&daily).Error; err != nil {
		return nil, err
	}
	return &daily, nil
}
func gachaRunView(run *GachaRun, state *GachaGameState, revision int64) (*GachaRunResult, error) {
	if run.Snapshot == "" || run.BattleReport == "" || run.ReadyAt <= 0 {
		return nil, ErrGachaRunState
	}
	var report GachaBattleReport
	if err := common.Unmarshal([]byte(run.BattleReport), &report); err != nil {
		return nil, err
	}
	view := &GachaRunResult{ID: run.ID, Stage: run.Stage, ReadyAt: run.ReadyAt, ExpireAt: run.ExpireAt, Result: &report, Save: state, Revision: revision}
	var snapshot GachaRunSnapshot
	if err := common.Unmarshal([]byte(run.Snapshot), &snapshot); err != nil {
		return nil, err
	}
	if snapshot.Mode != "" {
		stage, err := specialGachaStageFromSnapshot(snapshot, run.Stage)
		if err != nil {
			return nil, err
		}
		view.Scene = &GachaRunScene{Name: stage.Name, Img: stage.Img, StarRound: stage.StarRound}
	}
	return view, nil
}
func StartGachaRun(userID int, stageID, requestID string, now time.Time) (*GachaRunResult, error) {
	if gachaStage(stageID) == nil && stageID != "expedition" && stageID != "weekly_boss" && !isGachaProgressionStage(stageID) {
		return nil, ErrGachaStage
	}
	if !validateGachaUUID(requestID) {
		return nil, ErrGachaIntent
	}
	requestID = strings.ToLower(requestID)
	intent := struct{ Action, Stage string }{"start", stageID}
	hash, err := gachaIntentHash(intent)
	if err != nil {
		return nil, err
	}
	result := &GachaRunResult{}
	err = gachaTransaction(func(tx *gorm.DB) error {
		player, state, err := loadGachaPlayer(tx, userID, now)
		if err != nil {
			return err
		}
		if replay, err := gachaReplayResult(tx, userID, requestID, hash, result); replay || err != nil {
			return err
		}
		if err := checkGachaRunRest(state, now); err != nil {
			return err
		}
		stage := gachaStage(stageID)
		snapshot := GachaRunSnapshot{Version: 1, Squad: slices.Clone(state.Squad), MultiUsed: state.MultiOn && state.Multi > 0}
		if stage == nil {
			if isGachaProgressionStage(stageID) {
				stage, snapshot, err = specialGachaProgressionStage(stageID, state, now)
			} else {
				stage, snapshot, err = specialGachaStage(stageID, state, now)
				snapshot.Squad = slices.Clone(state.Squad)
			}
			if err != nil {
				return err
			}
			snapshot.MultiUsed = snapshot.Mode == "expedition" && state.MultiOn && state.Multi > 0
		}
		if stage.UnlockStage != "" && !state.Cleared[stage.UnlockStage] {
			return ErrGachaLocked
		}
		snapshot.RulesVersion = 2
		if snapshot.ProgressionVersion == 0 {
			snapshot.ProgressionVersion = state.ProgressionRules.Version
		}
		if stage.RelicTier > 0 && state.Daily.RelicRuns < gachaRelicDailyLimit {
			snapshot.RelicEligible = true
		}
		if len(snapshot.Squad) < 1 || len(snapshot.Squad) > 4 {
			return ErrGachaTeam
		}
		for i, id := range snapshot.Squad {
			own := state.Girls[id]
			if own == nil || own.Star < 1 || own.Level < 1 || slices.Contains(snapshot.Squad[:i], id) {
				return ErrGachaTeam
			}
		}
		if state.Stamina < stage.Cost {
			return ErrGachaFunds
		}
		var pending int64
		if err := tx.Model(&GachaRun{}).Where("user_id = ? AND status = ? AND expire_at > ? AND ready_at > ?", userID, "running", now.UnixMilli(), 0).Count(&pending).Error; err != nil {
			return err
		}
		if pending > 0 {
			return ErrGachaRunState
		}
		daily, err := gachaDailyLocked(tx, userID, gachaDay(now), now)
		if err != nil {
			return err
		}
		combatState := *state
		combatState.Squad = slices.Clone(snapshot.Squad)
		if snapshot.Mode == "expedition" {
			combatState.BattleBuff = snapshot.Route
			state.Expedition.Route = ""
		} else if snapshot.Mode == "weekly_boss" {
			state.WeeklyBoss.Attempts++
		}
		report, err := SimulateGachaCombat(&combatState, *stage)
		if err != nil {
			return err
		}
		if report.DurationMS <= 0 || report.DurationMS >= int(GachaRunTTL/time.Millisecond) {
			return ErrGachaRunState
		}
		if snapshot.MultiUsed {
			state.Multi--
			if state.Multi == 0 {
				state.MultiOn = false
			}
		}
		if state.Stamina == state.StaminaMax {
			state.StaminaTs = now.UnixMilli()
		}
		state.Stamina -= stage.Cost
		reportJSON, err := common.Marshal(report)
		if err != nil {
			return err
		}
		snapshotJSON, err := common.Marshal(snapshot)
		if err != nil {
			return err
		}
		id, err := uuid.NewRandom()
		if err != nil {
			return err
		}
		run := GachaRun{ID: "run_" + strings.ReplaceAll(id.String(), "-", ""), UserID: userID, Stage: stageID, Day: daily.Day, Status: "running", StartedAt: now.UnixMilli(), ReadyAt: now.UnixMilli() + int64(report.DurationMS), ExpireAt: now.Add(GachaRunTTL).UnixMilli(), Snapshot: GachaDocument(snapshotJSON), BattleReport: GachaDocument(reportJSON)}
		trackGachaRunStart(state, now, run.ReadyAt)
		if snapshot.RelicEligible {
			state.Daily.RelicRuns++
		}
		if err := tx.Create(&run).Error; err != nil {
			return err
		}
		if err := tx.Model(&GachaDaily{}).Where("day = ? AND user_id = ?", daily.Day, userID).Updates(map[string]any{"runs": gorm.Expr("runs + 1"), "updated_at": now.UnixMilli()}).Error; err != nil {
			return err
		}
		if err := storeGachaPlayer(tx, player, state, now); err != nil {
			return err
		}
		view, err := gachaRunView(&run, state, player.Revision)
		if err != nil {
			return err
		}
		*result = *view
		return recordGachaAction(tx, userID, requestID, hash, result, now)
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func awardGachaGameRewards(state *GachaGameState, stage *GachaStage, snapshot *GachaRunSnapshot, report *GachaBattleReport, result *GachaFinishResult) error {
	token, err := gachaRandomRange(stage.TokenRange)
	if err != nil {
		return err
	}
	if snapshot.MultiUsed {
		token *= 2
	}
	real := min(token, max(0, gachaCatalog.TokenBudget-state.Daily.TokenGained))
	if state.Token > gachaMaxResource-real {
		return ErrGachaFunds
	}
	state.Token += real
	state.Daily.TokenGained += real
	result.Drops = append(result.Drops, fmt.Sprintf("代币 x%d", real))
	gold, err := gachaRandomRange(stage.Gold)
	if err != nil {
		return err
	}
	if math.IsNaN(report.GoldBonus) || report.GoldBonus < 0 || report.GoldBonus > 0.3 {
		return ErrGachaRunState
	}
	gold = int(math.Round(float64(gold) * (1 + report.GoldBonus)))
	if state.Gold > gachaMaxResource-gold {
		return ErrGachaFunds
	}
	state.Gold += gold
	result.Drops = append(result.Drops, fmt.Sprintf("金币 x%d", gold))
	for _, id := range snapshot.Squad {
		own := state.Girls[id]
		if own == nil || own.Star < 1 {
			return ErrGachaRunState
		}
		if own.Level < gachaCatalog.Growth.MaxLevel {
			own.Exp += stage.Exp
			for own.Level < gachaCatalog.Growth.MaxLevel && own.Exp >= gachaCatalog.Growth.ExpPerLevel*own.Level {
				own.Exp -= gachaCatalog.Growth.ExpPerLevel * own.Level
				own.Level++
			}
			if own.Level >= gachaCatalog.Growth.MaxLevel {
				own.Exp = 0
			}
		}
	}
	result.Drops = append(result.Drops, fmt.Sprintf("经验 x%d", stage.Exp))
	material, err := gachaRandomRange(stage.Mat)
	if err != nil {
		return err
	}
	if math.IsNaN(report.MaterialBonus) || report.MaterialBonus < 0 || report.MaterialBonus > 0.20 {
		return ErrGachaRunState
	}
	material = int(math.Round(float64(material) * (1 + report.MaterialBonus)))
	if state.Mats[stage.MatKey] > gachaMaxResource-material {
		return ErrGachaFunds
	}
	state.Mats[stage.MatKey] += material
	matNames := map[string]string{"mat_r": "微光结晶", "mat_sr": "元素结晶", "mat_ssr": "星屑"}
	result.Drops = append(result.Drops, fmt.Sprintf("%s x%d", matNames[stage.MatKey], material))
	gemDrop, err := gachaChance(0.18)
	if err != nil {
		return err
	}
	if gemDrop {
		gemIDs := []string{"ruby", "jade", "sapphire", "wind", "amethyst"}
		if family, ok := gachaSigilFamilies[stage.LootFamily]; ok && snapshot.RulesVersion >= 2 {
			gemIDs = family.GemPool
		}
		gemIndex, err := gachaRandom(len(gemIDs))
		if err != nil {
			return err
		}
		gemID := gemIDs[gemIndex]
		if state.GemInventory[gemID] >= gachaMaxResource {
			return ErrGachaFunds
		}
		state.GemInventory[gemID]++
		result.Drops = append(result.Drops, gachaGemSpecs[gemID].Name+" x1")
	}
	for _, drop := range []struct {
		rarity string
		rate   float64
		count  int
	}{{"R", stage.ShardR, 5}, {"SR", stage.ShardSR, 3}, {"SSR", stage.ShardSSR, 2}} {
		ok, err := gachaChance(drop.rate)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		pool := []GachaGirl{}
		if family, ok := gachaSigilFamilies[stage.LootFamily]; ok && snapshot.RulesVersion >= 2 {
			for _, girl := range gachaCatalog.Girls {
				if girl.Rarity == drop.rarity && girl.Elem == family.Element {
					pool = append(pool, girl)
				}
			}
		}
		var girl *GachaGirl
		if len(pool) > 0 {
			index, pickErr := gachaRandom(len(pool))
			if pickErr != nil {
				return pickErr
			}
			girl = &pool[index]
		} else {
			girl, err = gachaRandomGirl(drop.rarity)
		}
		if err != nil {
			return err
		}
		own := state.Girls[girl.ID]
		if own == nil {
			own = &GachaOwnedGirl{Gems: make([]string, 3)}
			state.Girls[girl.ID] = own
		}
		if own.Shards > gachaMaxResource-drop.count {
			return ErrGachaFunds
		}
		own.Shards += drop.count
		if own.Star == 0 && own.Shards >= gachaCatalog.Rarity[drop.rarity].ShardToUnlock {
			own.Shards -= gachaCatalog.Rarity[drop.rarity].ShardToUnlock
			own.Star = gachaCatalog.Rarity[drop.rarity].BaseStar
			own.Level = 1
			result.Drops = append(result.Drops, "新少女解锁："+girl.Name)
		} else {
			result.Drops = append(result.Drops, fmt.Sprintf("%s碎片 x%d", girl.Name, drop.count))
		}
	}
	bonus, err := gachaChance(0.08)
	if err != nil {
		return err
	}
	if bonus {
		if state.Multi >= gachaMaxResource {
			return ErrGachaFunds
		}
		state.Multi++
		result.Drops = append(result.Drops, "双倍卡 x1")
	}
	if err := awardGachaSigilLoot(state, stage, snapshot, result); err != nil {
		return err
	}
	state.Cleared[stage.ID] = true
	result.IsBest = report.Stars > state.Stars[stage.ID]
	if result.IsBest {
		state.Stars[stage.ID] = report.Stars
	}
	return nil
}

func FinishGachaRun(userID int, runID string, abandon bool, now time.Time) (*GachaFinishResult, error) {
	if len(runID) == 0 || len(runID) > 40 {
		return nil, ErrGachaIntent
	}
	result := &GachaFinishResult{Drops: []string{}}
	credited := 0
	err := gachaTransaction(func(tx *gorm.DB) error {
		credited = 0
		player, state, err := loadGachaPlayer(tx, userID, now)
		if err != nil {
			return err
		}
		var run GachaRun
		if err := lockForUpdate(tx).Where("id = ? AND user_id = ?", runID, userID).First(&run).Error; err != nil {
			return ErrGachaRunState
		}
		if run.Result != "" {
			return common.Unmarshal([]byte(run.Result), result)
		}
		if run.Status != "running" || run.Snapshot == "" || run.BattleReport == "" || run.ReadyAt <= 0 {
			return ErrGachaRunState
		}
		if run.ExpireAt <= now.UnixMilli() {
			return ErrGachaRunState
		}
		if !abandon && now.UnixMilli() < run.ReadyAt {
			return ErrGachaNotReady
		}
		var snapshot GachaRunSnapshot
		var report GachaBattleReport
		if err := common.Unmarshal([]byte(run.Snapshot), &snapshot); err != nil {
			return err
		}
		if err := common.Unmarshal([]byte(run.BattleReport), &report); err != nil {
			return err
		}
		if snapshot.Version != 1 || len(snapshot.Squad) == 0 || report.Stars < 0 || report.Stars > 3 {
			return ErrGachaRunState
		}
		stage := gachaStage(run.Stage)
		rule, exists := gachaStageRules[run.Stage]
		if snapshot.Mode == "" {
			if stage == nil || !exists {
				return ErrGachaStage
			}
		} else {
			stage, err = specialGachaStageFromSnapshot(snapshot, run.Stage)
			if err != nil {
				return err
			}
		}
		daily, err := gachaDailyLocked(tx, userID, gachaDay(now), now)
		if err != nil {
			return err
		}
		*result = GachaFinishResult{Win: report.Win && !abandon, Drops: []string{}, Rounds: report.Rounds, AliveN: report.AliveN, TotalN: report.TotalN, Fast: report.Rounds <= stage.StarRound}
		if snapshot.Mode != "" {
			if result.Win {
				result.Stars = report.Stars
			}
			if isGachaProgressionStage(snapshot.Mode) {
				if err := settleGachaProgressionMode(state, &snapshot, result, now); err != nil {
					return err
				}
			} else {
				if err := settleGachaMode(state, stage, &snapshot, &report, result, abandon); err != nil {
					return err
				}
				if result.Win {
					recordGachaProgressionActivity(state, "weekly", now)
				}
			}
		} else if result.Win {
			result.Stars = report.Stars
			if err := awardGachaGameRewards(state, stage, &snapshot, &report, result); err != nil {
				return err
			}
			base, err := gachaRandomRange([2]int{rule.QuotaMin, rule.QuotaMax})
			if err != nil {
				return err
			}
			quota := common.QuotaRound(float64(base)*gachaStarMultiplier(report.Stars)/10) * 10
			result.Quota = max(quota, 0)
			if result.Quota > 0 {
				var user User
				if err := lockForUpdate(tx).First(&user, userID).Error; err != nil {
					return err
				}
				if user.Status != common.UserStatusEnabled || user.Quota > common.MaxWalletQuota-result.Quota || user.Quota < -common.MaxWalletQuota {
					return ErrGachaRunState
				}
				up := tx.Model(&User{}).Where("id = ? AND quota <= ?", userID, common.MaxWalletQuota-result.Quota).Update("quota", gorm.Expr("quota + ?", result.Quota))
				if up.Error != nil {
					return up.Error
				}
				if up.RowsAffected != 1 {
					return ErrGachaConcurrent
				}
				daily.Quota += result.Quota
				credited = result.Quota
				result.Drops = append(result.Drops, "站点额度 +"+formatGachaQuota(result.Quota))
			}
			cardDrop, err := gachaChance(rule.CardRate)
			if err != nil {
				return err
			}
			if cardDrop {
				total := 0
				for _, kind := range gachaCardKinds {
					total += kind.Weight
				}
				roll, err := gachaRandom(total)
				if err != nil {
					return err
				}
				kind := gachaCardKinds[0]
				for _, candidate := range gachaCardKinds {
					if roll < candidate.Weight {
						kind = candidate
						break
					}
					roll -= candidate.Weight
				}
				id, err := uuid.NewRandom()
				if err != nil {
					return err
				}
				card := GachaCard{ID: "card_" + strings.ReplaceAll(id.String(), "-", ""), UserID: userID, Kind: kind.Kind, Name: kind.Name, Discount: kind.Discount, DurationMin: kind.DurationMin, Status: "stored", Stage: run.Stage, ObtainedAt: now.UnixMilli()}
				if err := tx.Create(&card).Error; err != nil {
					return err
				}
				run.CardID = card.ID
				daily.Cards++
				result.Card = gachaCardView(&card, now)
				result.Drops = append(result.Drops, "倍率卡："+card.Name)
			}
			kind := "campaign"
			if stage.RelicTier > 0 {
				kind = "rift"
			}
			recordGachaProgressionActivity(state, kind, now)
		}
		if err := tx.Model(&GachaDaily{}).Where("day = ? AND user_id = ?", daily.Day, userID).Updates(map[string]any{"quota": daily.Quota, "cards": daily.Cards, "updated_at": now.UnixMilli()}).Error; err != nil {
			return err
		}
		if err := storeGachaPlayer(tx, player, state, now); err != nil {
			return err
		}
		result.Save = state
		result.Revision = player.Revision
		result.Today = gachaDailyView(daily)
		if err := tx.Model(&User{}).Where("id = ?", userID).Select("quota").Scan(&result.WalletQuota).Error; err != nil {
			return err
		}
		data, err := common.Marshal(result)
		if err != nil {
			return err
		}
		up := tx.Model(&GachaRun{}).Where("id = ? AND user_id = ? AND status = ? AND result = ?", run.ID, userID, "running", "").Updates(map[string]any{"status": "settled", "finished_at": now.UnixMilli(), "stars": result.Stars, "quota": result.Quota, "card_id": run.CardID, "result": GachaDocument(data)})
		if up.Error != nil {
			return up.Error
		}
		if up.RowsAffected != 1 {
			return ErrGachaConcurrent
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if credited > 0 {
		syncCreditUserQuotaCache(userID, credited, "gacha dungeon")
		RecordLog(userID, LogTypeSystem, "副本产出站点额度 +"+formatGachaQuota(credited))
	}
	return result, nil
}

// Completed battles are settled even if the browser is suspended or offline.
// Expired historical runs are excluded; the worker never invents compensation.
func HasCompletedGachaRuns(now time.Time) bool {
	var run GachaRun
	return DB.Select("id").Where("status = ? AND ready_at > 0 AND ready_at <= ? AND expire_at > ? AND result = ?", "running", now.UnixMilli(), now.UnixMilli(), "").Take(&run).Error == nil
}

func SettleCompletedGachaRuns(ctx context.Context, now time.Time) error {
	var runs []GachaRun
	if err := DB.WithContext(ctx).Select("id", "user_id").Where("status = ? AND ready_at > 0 AND ready_at <= ? AND expire_at > ? AND result = ?", "running", now.UnixMilli(), now.UnixMilli(), "").Order("ready_at ASC").Limit(50).Find(&runs).Error; err != nil {
		return err
	}
	var failure error
	for _, run := range runs {
		if err := ctx.Err(); err != nil {
			return err
		}
		_, err := FinishGachaRun(run.UserID, run.ID, false, now)
		failure = errors.Join(failure, err)
	}
	return failure
}

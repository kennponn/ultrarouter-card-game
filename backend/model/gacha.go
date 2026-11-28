package model

import (
	crand "crypto/rand"
	"encoding/hex"
	"errors"
	"maps"
	mrand "math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// 副本产出与站点仓库（倍率卡）。
//
// 服务端是唯一权威：副本局由 StartGachaRun 签发，结算只认签发过的局，
// 掉落、养成和战斗结果由数据库状态生成，客户端只能提交操作意图。

var (
	ErrGachaStage     = errors.New("unknown dungeon stage")
	ErrGachaRunState  = errors.New("dungeon run is no longer valid")
	ErrGachaCardState = errors.New("card is not available")
)

const (
	// 一局的有效时间：超过即作废，避免攒一堆 run 之后批量结算
	GachaRunTTL = 15 * time.Minute
	// 结算时允许的最大星数
	gachaMaxStars = 3
)

type gachaStageRule struct {
	QuotaMin int
	QuotaMax int
	CardRate float64
}

// 与前端 STAGES 一一对应；难度越高，额度与卡片掉率越高。
var gachaStageRules = map[string]gachaStageRule{
	"s1":  {QuotaMin: 75_000, QuotaMax: 110_000, CardRate: 0.030},
	"s5":  {QuotaMin: 100_000, QuotaMax: 150_000, CardRate: 0.040},
	"s2":  {QuotaMin: 200_000, QuotaMax: 275_000, CardRate: 0.060},
	"s4":  {QuotaMin: 225_000, QuotaMax: 325_000, CardRate: 0.075},
	"s3":  {QuotaMin: 375_000, QuotaMax: 500_000, CardRate: 0.090},
	"s6":  {QuotaMin: 475_000, QuotaMax: 625_000, CardRate: 0.110},
	"s7":  {QuotaMin: 425_000, QuotaMax: 575_000, CardRate: 0.115},
	"s8":  {QuotaMin: 500_000, QuotaMax: 675_000, CardRate: 0.120},
	"s9":  {QuotaMin: 625_000, QuotaMax: 800_000, CardRate: 0.125},
	"s10": {QuotaMin: 600_000, QuotaMax: 775_000, CardRate: 0.130},
	"s11": {QuotaMin: 700_000, QuotaMax: 900_000, CardRate: 0.135},
	"s12": {QuotaMin: 750_000, QuotaMax: 1_000_000, CardRate: 0.140},
	"r1":  {}, "r2": {}, "r3": {}, "r4": {}, "r5": {}, "r6": {},
	"n1": {}, "n2": {}, "n3": {}, "n4": {}, "n5": {}, "n6": {},
}

type gachaCardKind struct {
	Kind        string
	Name        string
	Discount    int
	DurationMin int
	Weight      int
}

// 折扣卡：激活后在有效期内把动态倍率按百分比下调，与动态倍率相乘生效。
var gachaCardKinds = []gachaCardKind{
	{Kind: "discount_5", Name: "微光折扣卡", Discount: 5, DurationMin: 60, Weight: 55},
	{Kind: "discount_10", Name: "元素折扣卡", Discount: 10, DurationMin: 45, Weight: 33},
	{Kind: "discount_20", Name: "星屑折扣卡", Discount: 20, DurationMin: 30, Weight: 12},
}

// GachaRun 是一局副本的凭证：签发时记下关卡，结算时校验状态与有效期。
type GachaRun struct {
	ID           string        `gorm:"type:varchar(40);primaryKey" json:"id"`
	UserID       int           `gorm:"index:idx_gacha_run_user;index" json:"user_id"`
	Stage        string        `gorm:"type:varchar(16);index" json:"stage"`
	Day          string        `gorm:"type:varchar(10);index" json:"day"`
	Status       string        `gorm:"type:varchar(16);index" json:"status"`
	StartedAt    int64         `json:"started_at"`
	ExpireAt     int64         `json:"expire_at"`
	FinishedAt   int64         `json:"finished_at"`
	Stars        int           `json:"stars"`
	Quota        int           `json:"quota"`
	CardID       string        `gorm:"type:varchar(40)" json:"card_id"`
	ReadyAt      int64         `json:"ready_at"`
	Snapshot     GachaDocument `json:"-"`
	BattleReport GachaDocument `json:"-"`
	Result       GachaDocument `json:"-"`
}

// GachaCard 是站点仓库里的一张倍率卡。
type GachaCard struct {
	ID          string `gorm:"type:varchar(40);primaryKey" json:"id"`
	UserID      int    `gorm:"index:idx_gacha_card_user;index" json:"user_id"`
	Kind        string `gorm:"type:varchar(24);index" json:"kind"`
	Name        string `gorm:"type:varchar(32)" json:"name"`
	Discount    int    `json:"discount"`
	DurationMin int    `json:"duration_min"`
	Status      string `gorm:"type:varchar(16);index" json:"status"` // stored / active / used
	Stage       string `gorm:"type:varchar(16)" json:"stage"`
	ObtainedAt  int64  `json:"obtained_at"`
	ActivatedAt int64  `json:"activated_at"`
	ExpireAt    int64  `gorm:"index:idx_gacha_card_expire" json:"expire_at"`
}

// GachaDaily 记录每日副本次数与实际额度产出，两者均不设每日上限。
type GachaDaily struct {
	Day       string `gorm:"type:varchar(10);primaryKey" json:"day"`
	UserID    int    `gorm:"primaryKey" json:"user_id"`
	Runs      int    `json:"runs"`
	Quota     int    `json:"quota"`
	Cards     int    `json:"cards"`
	UpdatedAt int64  `json:"updated_at"`
}

// ---------- 视图 ----------

type GachaCardView struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Discount    int    `json:"discount"`
	DurationMin int    `json:"duration_min"`
	Status      string `json:"status"`
	Stage       string `json:"stage"`
	ObtainedAt  int64  `json:"obtained_at"`
	ActivatedAt int64  `json:"activated_at"`
	ExpireAt    int64  `json:"expire_at"`
	RemainSec   int64  `json:"remain_sec"`
}

type GachaDailyView struct {
	Day        string `json:"day"`
	Runs       int    `json:"runs"`
	RunLimit   *int   `json:"run_limit"` // nil means unlimited.
	Quota      int    `json:"quota"`
	QuotaLimit *int   `json:"quota_limit"` // nil means unlimited.
	Cards      int    `json:"cards"`
}

type GachaProfile struct {
	Market         *GachaMarketRules               `json:"market"`
	CaptainRules   *GachaCaptainRules              `json:"captain_rules"`
	Voyage         *GachaVoyageCatalog             `json:"voyage"`
	Festival       *GachaFestivalRules             `json:"festival"`
	Dispatch       *GachaDispatchRules             `json:"dispatch"`
	Vigil          *GachaVigilRules                `json:"vigil"`
	Cases          *GachaCaseRules                 `json:"cases"`
	UserID         int                             `json:"user_id"`
	Name           string                          `json:"name"`
	Quota          int                             `json:"quota"`
	Today          GachaDailyView                  `json:"today"`
	Active         *GachaCardView                  `json:"active"`
	Stored         int                             `json:"stored"`
	Save           *GachaGameState                 `json:"save"`
	Revision       int64                           `json:"revision"`
	ActiveRun      *GachaRunResult                 `json:"active_run"`
	StageRewards   map[string]GachaStageRewardView `json:"stage_rewards"`
	Guild          *GachaGuildView                 `json:"guild,omitempty"`
	RunRules       GachaRunRules                   `json:"run_rules"`
	Progression    GachaProgressionView            `json:"progression"`
	Cultivation    *GachaCultivationRules          `json:"cultivation"`
	Training       *GachaTrainingRules             `json:"training"`
	Equipment      *GachaEquipmentRules            `json:"equipment"`
	ArtifactGrades *GachaArtifactGradeRules        `json:"artifact_grades"`
	Items          *GachaItemAtlas                 `json:"items"`
	Carry          *GachaCarryRules                `json:"carry"`
}

type GachaRunRules struct {
	BurstLimit      int `json:"burst_limit"`
	BurstWindowSec  int `json:"burst_window_sec"`
	RestSeconds     int `json:"rest_seconds"`
	RelicDailyLimit int `json:"relic_daily_limit"`
	SigilCapacity   int `json:"sigil_capacity"`
}

type GachaStageRewardView struct {
	QuotaMin int `json:"quota_min"`
	QuotaMax int `json:"quota_max"`
}

type GachaFinishResult struct {
	Win         bool            `json:"win"`
	Rounds      int             `json:"rounds"`
	AliveN      int             `json:"alive_n"`
	TotalN      int             `json:"total_n"`
	Fast        bool            `json:"fast"`
	IsBest      bool            `json:"is_best"`
	Drops       []string        `json:"drops"`
	Save        *GachaGameState `json:"save"`
	Revision    int64           `json:"revision"`
	Quota       int             `json:"quota"`
	WalletQuota int             `json:"wallet_quota"`
	Stars       int             `json:"stars"`
	Card        *GachaCardView  `json:"card"`
	Today       GachaDailyView  `json:"today"`
	QuotaLeft   *int            `json:"quota_left"` // nil means unlimited; old receipts retain their numeric value.
}

// ---------- 工具 ----------

func gachaDay(now time.Time) string {
	return now.In(gameZone).Format("2006-01-02")
}

func gachaRandID(prefix string) string {
	buf := make([]byte, 9)
	if _, err := crand.Read(buf); err != nil {
		return prefix + hex.EncodeToString([]byte(time.Now().Format("150405.000")))
	}
	return prefix + hex.EncodeToString(buf)
}

func gachaCardView(card *GachaCard, now time.Time) *GachaCardView {
	if card == nil {
		return nil
	}
	view := &GachaCardView{
		ID: card.ID, Name: card.Name, Discount: card.Discount,
		DurationMin: card.DurationMin, Status: card.Status, Stage: card.Stage,
		ObtainedAt: card.ObtainedAt, ActivatedAt: card.ActivatedAt, ExpireAt: card.ExpireAt,
	}
	if card.Status == "active" && card.ExpireAt > now.UnixMilli() {
		view.RemainSec = (card.ExpireAt - now.UnixMilli()) / 1000
	}
	return view
}

func gachaDailyView(daily *GachaDaily) GachaDailyView {
	view := GachaDailyView{}
	if daily != nil {
		view.Day = daily.Day
		view.Runs = daily.Runs
		view.Quota = daily.Quota
		view.Cards = daily.Cards
	}
	return view
}

func gachaPickCard() gachaCardKind {
	total := 0
	for _, k := range gachaCardKinds {
		total += k.Weight
	}
	roll := mrand.Intn(total)
	for _, k := range gachaCardKinds {
		if roll < k.Weight {
			return k
		}
		roll -= k.Weight
	}
	return gachaCardKinds[0]
}

// 星数越高产出越多：1 星原价，2 星 +10%，3 星 +20%
func gachaStarMultiplier(stars int) float64 {
	switch {
	case stars >= 3:
		return 1.2
	case stars == 2:
		return 1.1
	default:
		return 1.0
	}
}

// ---------- 业务 ----------

func GetGachaProfile(userID int, now time.Time) (*GachaProfile, error) {
	profile := &GachaProfile{}
	err := gachaTransaction(func(tx *gorm.DB) error {
		player, state, err := loadGachaPlayer(tx, userID, now)
		if err != nil {
			return err
		}
		var user User
		if err := tx.First(&user, userID).Error; err != nil {
			return err
		}
		day := gachaDay(now)
		var daily GachaDaily
		if err := tx.Where("day = ? AND user_id = ?", day, userID).First(&daily).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		profile.UserID = user.Id
		profile.Name = user.Username
		profile.Cultivation = &gachaCultivation
		profile.Training = &gachaTraining
		profile.CaptainRules = &gachaCaptainRules
		profile.Vigil = &gachaVigil
		profile.Dispatch = &gachaDispatch
		profile.Festival = festivalEdition(now)
		profile.Cases = &gachaCases
		profile.Equipment = &gachaEquipment
		profile.ArtifactGrades = &gachaArtifactGrades
		profile.Carry = &gachaCarry
		profile.Market = &gachaMarket
		profile.Items = &gachaItems
		profile.Voyage = &gachaVoyage
		profile.Progression = GachaProgressionView{Rules: state.ProgressionRules, Branches: map[string][]GachaAwakeningBranch{},
			CanConfigure: user.Role >= common.RoleAdminUser, DailyBankDays: 7, WeeklyBankWeeks: 2, PowerWeights: maps.Clone(gachaAwakeningPowerWeights)}
		for _, girl := range gachaCatalog.Girls {
			profile.Progression.Branches[girl.ID] = GachaAwakeningBranches(girl.ID)
		}
		profile.Quota = user.Quota
		profile.Today = gachaDailyView(dailyPointer(&daily))
		profile.RunRules = GachaRunRules{BurstLimit: gachaBurstRunLimit, BurstWindowSec: int(gachaBurstRunWindow / time.Second), RestSeconds: int(gachaBurstRestDuration / time.Second), RelicDailyLimit: gachaRelicDailyLimit, SigilCapacity: gachaSigilCapacity}
		profile.StageRewards = make(map[string]GachaStageRewardView, len(gachaStageRules))
		for stage, rule := range gachaStageRules {
			profile.StageRewards[stage] = GachaStageRewardView{QuotaMin: rule.QuotaMin, QuotaMax: rule.QuotaMax}
		}
		var active GachaCard
		if err := tx.Where("user_id = ? AND status = ? AND expire_at > ?", userID, "active", now.UnixMilli()).Order("discount DESC").First(&active).Error; err == nil {
			profile.Active = gachaCardView(&active, now)
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var stored int64
		if err := tx.Model(&GachaCard{}).Where("user_id = ? AND status = ?", userID, "stored").Count(&stored).Error; err != nil {
			return err
		}
		profile.Stored = int(stored)
		if err := storeGachaPlayer(tx, player, state, now); err != nil {
			return err
		}
		profile.Save = state
		profile.Guild, err = getGachaGuildView(tx, userID, state, now)
		if err != nil {
			return err
		}
		profile.Revision = player.Revision
		profile.ActiveRun = nil
		var run GachaRun
		if err := tx.Where("user_id = ? AND status = ? AND expire_at > ? AND ready_at > ?", userID, "running", now.UnixMilli(), 0).Order("started_at DESC").First(&run).Error; err == nil {
			view, err := gachaRunView(&run, state, player.Revision)
			if err != nil {
				return err
			}
			profile.ActiveRun = view
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return profile, nil
}

func dailyPointer(d *GachaDaily) *GachaDaily {
	if d.Day == "" {
		return nil
	}
	return d
}

func formatGachaQuota(quota int) string {
	// 1E = 500000 quota，展示时保留四位小数
	return strconv.FormatFloat(float64(quota)/500000, 'f', 4, 64) + "E"
}

func ListGachaCards(userID int, now time.Time) ([]GachaCardView, *GachaCardView) {
	var cards []GachaCard
	DB.Where("user_id = ?", userID).Order("obtained_at DESC").Limit(200).Find(&cards)
	views := make([]GachaCardView, 0, len(cards))
	var active *GachaCardView
	for i := range cards {
		card := &cards[i]
		// 过期的激活卡惰性收尾
		if card.Status == "active" && card.ExpireAt < now.UnixMilli() {
			card.Status = "used"
			DB.Model(&GachaCard{}).Where("id = ?", card.ID).Update("status", "used")
		}
		view := gachaCardView(card, now)
		if card.Status == "active" && active == nil {
			active = view
		}
		views = append(views, *view)
	}
	return views, active
}

func ActivateGachaCard(userID int, cardID string, now time.Time) (*GachaCardView, error) {
	var view *GachaCardView
	err := gachaTransaction(func(tx *gorm.DB) error {
		player, state, err := loadGachaPlayer(tx, userID, now)
		if err != nil {
			return err
		}
		var card GachaCard
		if err := lockForUpdate(tx).Where("id = ? AND user_id = ?", cardID, userID).First(&card).Error; err != nil {
			return ErrGachaCardState
		}
		if card.UserID != userID || card.Status != "stored" {
			return ErrGachaCardState
		}
		var activeCount int64
		if err := tx.Model(&GachaCard{}).
			Where("user_id = ? AND status = ? AND expire_at > ?", userID, "active", now.UnixMilli()).
			Count(&activeCount).Error; err != nil {
			return err
		}
		if activeCount > 0 {
			return ErrGachaCardState
		}
		expireAt := now.Add(time.Duration(card.DurationMin) * time.Minute).UnixMilli()
		update := tx.Model(&GachaCard{}).Where("id = ? AND user_id = ? AND status = ?", card.ID, userID, "stored").Updates(map[string]any{
			"status": "active", "activated_at": now.UnixMilli(), "expire_at": expireAt,
		})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrGachaCardState
		}
		if err := storeGachaPlayer(tx, player, state, now); err != nil {
			return err
		}
		card.Status = "active"
		card.ActivatedAt = now.UnixMilli()
		card.ExpireAt = expireAt
		view = gachaCardView(&card, now)
		return nil
	})
	if err != nil {
		return nil, err
	}
	gachaCardCache.Store(userID, gachaCardCacheEntry{Discount: float64(view.Discount) / 100, ExpireAt: view.ExpireAt, CheckedAt: now.UnixMilli()})
	return view, nil
}

// ---------- 计价用：当前生效折扣 ----------

type gachaCardCacheEntry struct {
	Discount  float64
	ExpireAt  int64
	CheckedAt int64
}

var gachaCardCache sync.Map // userID -> gachaCardCacheEntry

const gachaCardCacheTTL = 30 * time.Second

// ActiveCardDiscount 返回该用户当前生效的折扣比例（0~1）。
// 计价链路会在每个请求上调用，所以做一个 30 秒的进程内缓存。
func ActiveCardDiscount(userID int, now time.Time) float64 {
	if userID <= 0 {
		return 0
	}
	nowMs := now.UnixMilli()
	if v, ok := gachaCardCache.Load(userID); ok {
		entry := v.(gachaCardCacheEntry)
		if nowMs-entry.CheckedAt < int64(gachaCardCacheTTL/time.Millisecond) {
			if entry.ExpireAt > nowMs {
				return entry.Discount
			}
			return 0
		}
	}
	var card GachaCard
	discount := 0.0
	expireAt := int64(0)
	if err := DB.Where("user_id = ? AND status = ? AND expire_at > ?", userID, "active", nowMs).
		Order("discount DESC").First(&card).Error; err == nil {
		discount = float64(card.Discount) / 100
		expireAt = card.ExpireAt
	}
	gachaCardCache.Store(userID, gachaCardCacheEntry{Discount: discount, ExpireAt: expireAt, CheckedAt: nowMs})
	return discount
}

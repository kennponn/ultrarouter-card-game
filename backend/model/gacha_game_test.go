package model_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type gachaSchemaRecorder struct {
	mu  sync.Mutex
	ddl []string
}

func (r *gachaSchemaRecorder) LogMode(logger.LogLevel) logger.Interface { return r }
func (*gachaSchemaRecorder) Info(context.Context, string, ...any)       {}
func (*gachaSchemaRecorder) Warn(context.Context, string, ...any)       {}
func (*gachaSchemaRecorder) Error(context.Context, string, ...any)      {}
func (r *gachaSchemaRecorder) Trace(_ context.Context, _ time.Time, sql func() (string, int64), _ error) {
	statement, _ := sql()
	upper := strings.ToUpper(strings.TrimSpace(statement))
	if strings.HasPrefix(upper, "ALTER ") || strings.HasPrefix(upper, "CREATE ") || strings.HasPrefix(upper, "DROP ") || strings.HasPrefix(upper, "RENAME ") {
		r.mu.Lock()
		r.ddl = append(r.ddl, statement)
		r.mu.Unlock()
	}
}

// This is the released, pre-server-save run schema, not a production DB copy.
type releasedGachaRun struct {
	ID         string `gorm:"type:varchar(40);primaryKey"`
	UserID     int    `gorm:"index:idx_gacha_run_user;index"`
	Stage      string `gorm:"type:varchar(16);index"`
	Day        string `gorm:"type:varchar(10);index"`
	Status     string `gorm:"type:varchar(16);index"`
	StartedAt  int64
	ExpireAt   int64
	FinishedAt int64
	Stars      int
	Quota      int
	CardID     string `gorm:"type:varchar(40)"`
}

func (releasedGachaRun) TableName() string { return "gacha_runs" }

type releasedGachaPlayer struct {
	UserID    int    `gorm:"primaryKey;autoIncrement:false"`
	State     string `gorm:"type:text;not null"`
	Revision  int64  `gorm:"not null"`
	CreatedAt int64
	UpdatedAt int64
}

func (releasedGachaPlayer) TableName() string { return "gacha_players" }

type releasedArenaSeat struct {
	UserID    int    `gorm:"primaryKey;autoIncrement:false"`
	ID        string `gorm:"type:varchar(36);not null;uniqueIndex"`
	Status    string `gorm:"type:varchar(16);not null;index:idx_gacha_queue,priority:1"`
	StakeE    int    `gorm:"not null;index:idx_gacha_queue,priority:2"`
	Power     int    `gorm:"not null;index:idx_gacha_queue,priority:3"`
	CreatedAt int64
	ExpireAt  int64               `gorm:"index"`
	Snapshot  model.GachaDocument `gorm:"not null"`
	MatchID   string              `gorm:"type:varchar(36)"`
}

func (releasedArenaSeat) TableName() string { return "gacha_arena_seats" }

var gachaTestUserSequence atomic.Int64
var gachaTestRunID = uuid.NewString()[:8]

func gachaUser(t *testing.T) model.User {
	t.Helper()
	n := gachaTestUserSequence.Add(1)
	user := model.User{Username: fmt.Sprintf("gacha-%s-%d", gachaTestRunID, n), Password: "unused-test-hash", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1, Quota: 1000, AffCode: fmt.Sprintf("gacha-aff-%s-%d", gachaTestRunID, n)}
	require.NoError(t, model.DB.Create(&user).Error)
	return user
}

func gachaState(t *testing.T, id int, now time.Time) *model.GachaGameState {
	t.Helper()
	profile, err := model.GetGachaProfile(id, now)
	require.NoError(t, err)
	require.NotNil(t, profile.Save)
	return profile.Save
}

func seedGachaState(t *testing.T, id int, state *model.GachaGameState) {
	t.Helper()
	data, err := common.Marshal(state)
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.GachaPlayer{}).Where("user_id = ?", id).Update("state", string(data)).Error)
}

func gachaTestSigil(family string, quality int) *model.GachaSigil {
	values := [3][2]int{{2, 1}, {5, 3}, {8, 5}}
	return &model.GachaSigil{ID: "sigil_" + strings.ReplaceAll(uuid.NewString(), "-", ""), Family: family, Quality: quality,
		Affixes: map[string]int{"atk": values[quality-1][0], "cr": values[quality-1][1]}}
}

func strongGachaState(state *model.GachaGameState) {
	state.Squad = []string{"c04", "c07", "c09", "c03"}
	for _, id := range state.Squad {
		state.Girls[id] = &model.GachaOwnedGirl{Star: 5, Level: 60}
	}
}

func ptrTo[T any](v T) *T { return &v }

func gachaRequest(t *testing.T, userID int, session bool, handler gin.HandlerFunc, body any) *httptest.ResponseRecorder {
	t.Helper()
	data, err := common.Marshal(body)
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/user/gacha/test", bytes.NewReader(data))
	c.Request.Header.Set("Content-Type", "application/json")
	if userID > 0 {
		c.Set("id", userID)
		c.Set("auth_version", int64(1))
		if session {
			c.Set("session_id", "isolated-test-session")
			c.Set("session_version", int64(1))
		}
	}
	handler(c)
	return recorder
}

func gachaSchemaModels() []any {
	return []any{&model.GachaRun{}, &model.GachaCard{}, &model.GachaDaily{}, &model.GachaPlayer{}, &model.GachaAction{}, &model.GachaArenaGate{}, &model.GachaArenaSeat{}, &model.GachaArenaDuel{}, &model.GachaArenaEntry{}, &model.GachaArenaRoom{}}
}

func gachaRoomPlayer(t *testing.T, now time.Time) model.User {
	t.Helper()
	user := gachaUser(t)
	require.NoError(t, model.DB.Model(&user).Update("quota", 10*model.GachaArenaQuotaPerE).Error)
	state := gachaState(t, user.Id, now)
	state.Squad = []string{"c01"}
	state.Girls["c01"] = &model.GachaOwnedGirl{Star: 1, Level: 20}
	seedGachaState(t, user.Id, state)
	return user
}

func TestGachaAuthorityDatabaseMatrix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("GACHA_MATCHMAKING_ENABLED", "true")
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			previousDB, previousLogDB := model.DB, model.LOG_DB
			previousMain, previousLog := common.MainDatabaseType(), common.LogDatabaseType()
			previousSQLite, previousMaster, previousRedis := common.SQLitePath, common.IsMasterNode, common.RedisEnabled
			t.Cleanup(func() {
				model.DB, model.LOG_DB = previousDB, previousLogDB
				common.SetDatabaseTypes(previousMain, previousLog)
				common.SQLitePath, common.IsMasterNode, common.RedisEnabled = previousSQLite, previousMaster, previousRedis
			})
			var dsn string
			var driver gorm.Dialector
			switch dialect {
			case "sqlite":
				common.SQLitePath = filepath.Join(t.TempDir(), "gacha-authority.db") + "?_pragma=busy_timeout(30000)&_pragma=journal_mode(WAL)&_txlock=immediate"
				dsn, driver = "local", sqlite.Open(common.SQLitePath)
			case "mysql":
				dsn = os.Getenv("TEST_MYSQL_DSN")
				driver = mysql.Open(dsn)
			case "postgres":
				dsn = os.Getenv("TEST_POSTGRES_DSN")
				driver = postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true})
			}
			if dsn == "" {
				if os.Getenv("GACHA_REQUIRE_DB_MATRIX") == "1" {
					t.Fatal("required isolated database DSN is missing")
				}
				t.Skip("isolated database DSN is not configured")
			}
			if dialect != "sqlite" {
				require.Contains(t, dsn, "gacha_authority_test", "never run these write tests on a production database")
			}
			probe, err := gorm.Open(driver, &gorm.Config{})
			require.NoError(t, err)
			probeSQL, err := probe.DB()
			require.NoError(t, err)
			defer probeSQL.Close()
			var version, database string
			if dialect == "sqlite" {
				require.NoError(t, probe.Raw("SELECT sqlite_version()").Scan(&version).Error)
			} else {
				require.NoError(t, probe.Raw("SELECT version()").Scan(&version).Error)
				query := "SELECT DATABASE()"
				if dialect == "postgres" {
					query = "SELECT current_database()"
				}
				require.NoError(t, probe.Raw(query).Scan(&database).Error)
				require.Equal(t, "gacha_authority_test", database)
			}
			t.Logf("real %s version: %s", dialect, version)
			t.Setenv("SQL_DSN", dsn)
			t.Setenv("LOG_SQL_DSN", "")
			t.Setenv("SQL_MAX_OPEN_CONNS", "8")
			t.Setenv("SQL_MAX_IDLE_CONNS", "4")
			common.IsMasterNode, common.RedisEnabled = true, false
			initDB := func() {
				if model.DB != previousDB {
					oldSQL, err := model.DB.DB()
					require.NoError(t, err)
					require.NoError(t, oldSQL.Close())
				}
				require.NoError(t, model.InitDB(), "the actual startup migration must succeed")
				model.LOG_DB = model.DB
			}
			initDB()
			initDB()
			t.Cleanup(func() {
				if db, err := model.DB.DB(); err == nil {
					_ = db.Close()
				}
			})
			now := time.Now().UTC().Truncate(time.Second)

			t.Run("fresh_and_released_schema_migrate_twice", func(t *testing.T) {
				recorder := &gachaSchemaRecorder{}
				require.NoError(t, model.DB.Session(&gorm.Session{Logger: recorder}).AutoMigrate(gachaSchemaModels()...))
				assert.Empty(t, recorder.ddl, "repeating the fresh schema must not mutate it")
				user := gachaUser(t)
				legacyState := gachaState(t, user.Id, now)
				legacyBytes, err := common.Marshal(legacyState)
				require.NoError(t, err)
				var legacyFields map[string]any
				require.NoError(t, common.Unmarshal(legacyBytes, &legacyFields))
				delete(legacyFields, "power")
				delete(legacyFields, "arena")
				legacyBytes, err = common.Marshal(legacyFields)
				require.NoError(t, err)
				require.NoError(t, model.DB.Migrator().DropTable(&model.GachaPlayer{}, &model.GachaAction{}, &model.GachaRun{}, &model.GachaArenaGate{}, &model.GachaArenaSeat{}, &model.GachaArenaDuel{}, &model.GachaArenaEntry{}))
				require.NoError(t, model.DB.AutoMigrate(&releasedGachaRun{}, &releasedGachaPlayer{}))
				legacyPlayer := releasedGachaPlayer{UserID: user.Id, State: string(legacyBytes), Revision: 17, CreatedAt: now.UnixMilli() - 1000, UpdatedAt: now.UnixMilli() - 500}
				require.NoError(t, model.DB.Create(&legacyPlayer).Error)
				legacy := releasedGachaRun{ID: "released-run", UserID: user.Id, Stage: "s1", Day: now.In(time.FixedZone("CST", 8*3600)).Format("2006-01-02"), Status: "running", StartedAt: now.UnixMilli(), ExpireAt: now.Add(model.GachaRunTTL).UnixMilli(), Stars: 3, Quota: 330}
				require.NoError(t, model.DB.Create(&legacy).Error)
				card := model.GachaCard{ID: uuid.NewString(), UserID: user.Id, Name: "existing-card", Kind: "discount_5", Status: "stored", Discount: 5, DurationMin: 60}
				require.NoError(t, model.DB.Create(&card).Error)
				initDB()
				var migratedPlayer model.GachaPlayer
				require.NoError(t, model.DB.First(&migratedPlayer, "user_id = ?", user.Id).Error)
				assert.Equal(t, legacyPlayer.State, migratedPlayer.State)
				assert.Equal(t, legacyPlayer.Revision, migratedPlayer.Revision)
				assert.Equal(t, legacyPlayer.UpdatedAt, migratedPlayer.UpdatedAt)
				assert.Positive(t, migratedPlayer.Power)
				assert.Zero(t, migratedPlayer.ArenaPoints)
				largeJSON, err := common.Marshal(map[string]string{"payload": strings.Repeat("x", 100*1024)})
				require.NoError(t, err)
				document := model.GachaDocument(largeJSON)
				largeRun := model.GachaRun{ID: uuid.NewString(), UserID: user.Id, Status: "abandoned", Snapshot: document, BattleReport: document, Result: document}
				largeAction := model.GachaAction{UserID: user.Id, RequestID: uuid.NewString(), IntentHash: strings.Repeat("a", 64), Result: document}
				require.NoError(t, model.DB.Create(&largeRun).Error)
				require.NoError(t, model.DB.Create(&largeAction).Error)
				initDB()
				var preservedRun model.GachaRun
				var preservedAction model.GachaAction
				require.NoError(t, model.DB.First(&preservedRun, "id = ?", largeRun.ID).Error)
				require.NoError(t, model.DB.First(&preservedAction, "user_id = ? AND request_id = ?", user.Id, largeAction.RequestID).Error)
				assert.Equal(t, document, preservedRun.Snapshot)
				assert.Equal(t, document, preservedRun.BattleReport)
				assert.Equal(t, document, preservedRun.Result)
				assert.Equal(t, document, preservedAction.Result)
				if dialect == "mysql" {
					var columns []struct{ TableName, ColumnName, DataType string }
					require.NoError(t, model.DB.Raw("SELECT TABLE_NAME AS table_name, COLUMN_NAME AS column_name, DATA_TYPE AS data_type FROM information_schema.columns WHERE table_schema = DATABASE() AND ((table_name = 'gacha_runs' AND column_name IN ('snapshot', 'battle_report', 'result')) OR (table_name = 'gacha_actions' AND column_name = 'result'))").Scan(&columns).Error)
					require.Len(t, columns, 4)
					for _, column := range columns {
						assert.Equal(t, "longtext", strings.ToLower(column.DataType), "%s.%s must not truncate combat reports", column.TableName, column.ColumnName)
					}
				}
				var saved model.GachaRun
				require.NoError(t, model.DB.First(&saved, "id = ?", legacy.ID).Error)
				assert.Equal(t, legacy.UserID, saved.UserID)
				assert.Equal(t, legacy.Quota, saved.Quota)
				assert.Equal(t, legacy.Stars, saved.Stars)
				assert.Empty(t, saved.Snapshot)
				assert.True(t, model.DB.Migrator().HasIndex(&model.GachaRun{}, "idx_gacha_run_user"))
				var savedCard model.GachaCard
				require.NoError(t, model.DB.First(&savedCard, "id = ?", card.ID).Error)
				assert.Equal(t, card, savedCard)
				var savedUser model.User
				require.NoError(t, model.DB.First(&savedUser, user.Id).Error)
				assert.Equal(t, user.Username, savedUser.Username)
				assert.Equal(t, user.Quota, savedUser.Quota)
				recorder = &gachaSchemaRecorder{}
				require.NoError(t, model.DB.Session(&gorm.Session{Logger: recorder}).AutoMigrate(gachaSchemaModels()...))
				assert.Empty(t, recorder.ddl, "upgrading the released schema must be idempotent")
				_, err = model.FinishGachaRun(user.Id, legacy.ID, false, now.Add(time.Minute))
				assert.ErrorIs(t, err, model.ErrGachaRunState, "legacy client-star runs are never reward proofs")
				run, err := model.StartGachaRun(user.Id, "s1", uuid.NewString(), now)
				require.NoError(t, err, "an invisible legacy run must not block a new authoritative battle")
				require.NotNil(t, run.Result)
				_, err = model.FinishGachaRun(user.Id, run.ID, true, now)
				require.NoError(t, err)
			})

			t.Run("latest_social_release_gains_queue_tables_without_rewriting_accounts", func(t *testing.T) {
				user := gachaUser(t)
				gachaState(t, user.Id, now)
				var player model.GachaPlayer
				require.NoError(t, model.DB.First(&player, "user_id = ?", user.Id).Error)
				// The immediately preceding social release has the same player/run
				// schema and power indexes, but none of the four matchmaking tables.
				require.NoError(t, model.DB.Migrator().DropTable(&model.GachaArenaGate{}, &model.GachaArenaSeat{}, &model.GachaArenaDuel{}, &model.GachaArenaEntry{}))
				initDB()
				initDB()
				var after model.GachaPlayer
				require.NoError(t, model.DB.First(&after, "user_id = ?", user.Id).Error)
				assert.Equal(t, player, after)
				var wallet model.User
				require.NoError(t, model.DB.First(&wallet, user.Id).Error)
				assert.Equal(t, user.Quota, wallet.Quota)
				recorder := &gachaSchemaRecorder{}
				require.NoError(t, model.DB.Session(&gorm.Session{Logger: recorder}).AutoMigrate(gachaSchemaModels()...))
				assert.Empty(t, recorder.ddl)
			})

			t.Run("latest_matchmaking_schema_keeps_existing_escrow_through_room_upgrade", func(t *testing.T) {
				user := gachaRoomPlayer(t, now)
				state := gachaState(t, user.Id, now)
				snapshot, err := common.Marshal(state)
				require.NoError(t, err)
				require.NoError(t, model.DB.Migrator().DropTable(&model.GachaArenaSeat{}, &model.GachaArenaRoom{}))
				require.NoError(t, model.DB.AutoMigrate(&releasedArenaSeat{}))
				old := releasedArenaSeat{UserID: user.Id, ID: uuid.NewString(), Status: "waiting", StakeE: 1, Power: state.Power, CreatedAt: now.UnixMilli(), ExpireAt: now.Add(model.GachaArenaQueueTTL).UnixMilli(), Snapshot: model.GachaDocument(snapshot)}
				require.NoError(t, model.DB.Create(&old).Error)
				require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", user.Id).Update("quota", 9*model.GachaArenaQuotaPerE).Error)
				require.NoError(t, model.DB.Create(&model.GachaArenaEntry{QueueID: old.ID, Phase: "reserve", UserID: user.Id, DeltaQuota: -model.GachaArenaQuotaPerE, Reason: "waiting", CreatedAt: now.UnixMilli()}).Error)
				initDB()
				initDB()
				var upgraded model.GachaArenaSeat
				require.NoError(t, model.DB.First(&upgraded, "user_id = ?", user.Id).Error)
				assert.Empty(t, upgraded.RoomID)
				assert.Equal(t, old.Snapshot, upgraded.Snapshot)
				assert.Equal(t, old.ID, upgraded.ID)
				var same model.GachaArenaSeat
				same = upgraded
				same.UserID++
				assert.Error(t, model.DB.Create(&same).Error, "ticket uniqueness survives migration")
				out, err := model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "arena_cancel", QueueID: &old.ID}, now)
				require.NoError(t, err)
				assert.Equal(t, 10*model.GachaArenaQuotaPerE, out.Queue.Quota)
				recorder := &gachaSchemaRecorder{}
				require.NoError(t, model.DB.Session(&gorm.Session{Logger: recorder}).AutoMigrate(gachaSchemaModels()...))
				assert.Empty(t, recorder.ddl)
			})

			t.Run("anonymous_and_PAT_rejected_before_any_game_mutation", func(t *testing.T) {
				user := gachaUser(t)
				for _, handler := range []gin.HandlerFunc{controller.GetGachaProfile, controller.ApplyGachaAction, controller.StartGachaRun, controller.FinishGachaRun, controller.GetGachaWarehouse, controller.ActivateGachaCard, controller.GetGachaSocial, controller.GetGachaArenaReplay, controller.GetGachaArenaQueue, controller.GetGachaArenaRooms} {
					for _, id := range []int{0, user.Id} {
						response := gachaRequest(t, id, false, handler, map[string]any{})
						assert.Equal(t, http.StatusForbidden, response.Code)
						assert.Contains(t, response.Body.String(), "AUTH_SESSION_REQUIRED")
					}
				}
				var rows int64
				require.NoError(t, model.DB.Model(&model.GachaPlayer{}).Where("user_id = ?", user.Id).Count(&rows).Error)
				assert.Zero(t, rows)
			})

			t.Run("strict_HTTP_intents_reject_saves_star_claims_and_null", func(t *testing.T) {
				user := gachaUser(t)
				for _, extra := range []string{"save", "state", "gold", "mats", "stars", "user_id", "seed"} {
					body := map[string]any{"request_id": uuid.NewString(), "action": "gacha", "count": 1, extra: 999999999}
					response := gachaRequest(t, user.Id, true, controller.ApplyGachaAction, body)
					assert.Equal(t, http.StatusBadRequest, response.Code, extra)
				}
				for _, body := range []map[string]any{{"run_id": "forged-run", "stars": 3}, {"run_id": "forged-run", "abandon": nil}, {"run_id": "forged-run", "win": true}} {
					assert.Equal(t, http.StatusBadRequest, gachaRequest(t, user.Id, true, controller.FinishGachaRun, body).Code)
				}
				assert.Equal(t, http.StatusBadRequest, gachaRequest(t, user.Id, true, controller.StartGachaRun, map[string]any{"stage": "s6", "request_id": uuid.NewString(), "squad": []string{"c04"}}).Code)
				assert.Equal(t, http.StatusBadRequest, gachaRequest(t, user.Id, true, controller.ApplyGachaAction, map[string]any{"request_id": uuid.NewString(), "action": "gacha", "count": 1, "save": strings.Repeat("x", 17000)}).Code)
				state := gachaState(t, user.Id, now)
				assert.Equal(t, 600, state.Gold)
				assert.Zero(t, state.TotalPull)
			})

			t.Run("mutating_returned_state_cannot_mint_assets_or_unlock_stages", func(t *testing.T) {
				user := gachaUser(t)
				state := gachaState(t, user.Id, now)
				state.Progression.Permits = 5
				seedGachaState(t, user.Id, state)
				state.Gold, state.Mats["mat_r"], state.Stars["s6"], state.Cleared["s3"] = 999999999, 999999999, 3, true
				strongGachaState(state)
				count := 1
				result, err := model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "gacha", Count: &count}, now)
				require.NoError(t, err)
				assert.Equal(t, 450, result.Save.Gold)
				assert.Zero(t, result.Save.Mats["mat_r"])
				assert.Zero(t, result.Save.Stars["s6"])
				assert.False(t, result.Save.Cleared["s3"])
				assert.Equal(t, []string{"c08", "c06", "c01", "c12"}, result.Save.Squad)
				_, err = model.StartGachaRun(user.Id, "s6", uuid.NewString(), now)
				assert.ErrorIs(t, err, model.ErrGachaLocked)
			})

			t.Run("action_ids_are_atomic_idempotent_and_user_scoped", func(t *testing.T) {
				user, other := gachaUser(t), gachaUser(t)
				for _, id := range []int{user.Id, other.Id} {
					state := gachaState(t, id, now)
					state.Progression.Permits = 5
					seedGachaState(t, id, state)
				}
				count := 1
				intent := model.GachaActionIntent{RequestID: uuid.NewString(), Action: "gacha", Count: &count}
				start := make(chan struct{})
				results := make([]*model.GachaActionResult, 2)
				errors := make([]error, 2)
				var wg sync.WaitGroup
				for i := range 2 {
					wg.Go(func() { <-start; results[i], errors[i] = model.ApplyGachaAction(user.Id, intent, now) })
				}
				close(start)
				wg.Wait()
				for _, err := range errors {
					require.NoError(t, err)
				}
				assert.Equal(t, results[0], results[1])
				assert.Equal(t, 450, gachaState(t, user.Id, now).Gold)
				assert.Equal(t, 1, gachaState(t, user.Id, now).TotalPull)
				var rows int64
				require.NoError(t, model.DB.Model(&model.GachaAction{}).Where("user_id = ?", user.Id).Count(&rows).Error)
				assert.EqualValues(t, 1, rows)
				count10 := 10
				changed := intent
				changed.Count = &count10
				_, err := model.ApplyGachaAction(user.Id, changed, now)
				assert.ErrorIs(t, err, model.ErrGachaReplay)
				otherResult, err := model.ApplyGachaAction(other.Id, intent, now)
				require.NoError(t, err)
				assert.Equal(t, 450, otherResult.Save.Gold)
			})

			t.Run("concurrent_spends_cannot_overdraw_the_database_balance", func(t *testing.T) {
				user := gachaUser(t)
				state := gachaState(t, user.Id, now)
				state.Gold = 150
				state.Progression.Permits = 2
				seedGachaState(t, user.Id, state)
				count := 1
				start := make(chan struct{})
				errors := make([]error, 2)
				var wg sync.WaitGroup
				for i := range 2 {
					wg.Go(func() {
						<-start
						_, errors[i] = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "gacha", Count: &count}, now)
					})
				}
				close(start)
				wg.Wait()
				successes := 0
				for _, err := range errors {
					if err == nil {
						successes++
					} else {
						assert.ErrorIs(t, err, model.ErrGachaFunds)
					}
				}
				assert.Equal(t, 1, successes)
				state = gachaState(t, user.Id, now)
				assert.Zero(t, state.Gold)
				assert.Equal(t, 1, state.TotalPull)
				_, err := model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "gacha", Count: &count}, now)
				assert.ErrorIs(t, err, model.ErrGachaFunds)
			})

			t.Run("upgrade_and_team_intents_use_owned_database_resources", func(t *testing.T) {
				user := gachaUser(t)
				state := gachaState(t, user.Id, now)
				state.Gold, state.Mats["mat_r"], state.Girls["c08"].Shards = 35, 20, 20
				seedGachaState(t, user.Id, state)
				girl := "c08"
				_, err := model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "level", GirlID: &girl}, now)
				assert.ErrorIs(t, err, model.ErrGachaExperience)
				assert.Equal(t, 35, gachaState(t, user.Id, now).Gold)
				assert.Equal(t, 1, gachaState(t, user.Id, now).Girls[girl].Level)
				star, err := model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "star", GirlID: &girl}, now)
				require.NoError(t, err)
				assert.Equal(t, 2, star.Save.Girls[girl].Star)
				assert.Zero(t, star.Save.Girls[girl].Shards)
				assert.Zero(t, star.Save.Mats["mat_r"])
				_, err = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "star", GirlID: &girl}, now)
				assert.ErrorIs(t, err, model.ErrGachaFunds)
				for _, team := range [][]string{{"c04"}, {"c08", "c08"}, {"not-a-girl"}} {
					_, err = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "squad", Squad: &team}, now)
					assert.ErrorIs(t, err, model.ErrGachaTeam)
				}
			})

			t.Run("a_girl_fields_in_only_one_preset_and_legacy_duplicates_are_repaired", func(t *testing.T) {
				user := gachaUser(t)
				state := gachaState(t, user.Id, now)
				for _, id := range []string{"c03", "c04", "c06", "c07", "c09"} {
					state.Girls[id] = &model.GachaOwnedGirl{Star: 3, Level: 20}
				}
				state.Teams = [][]string{{"c04", "c07"}, {}, {}, {}}
				state.TeamOn = 1
				state.Squad = []string{"c04", "c07"}
				seedGachaState(t, user.Id, state)

				// Legacy saves could repeat a girl in two presets; loading repairs them.
				broken := gachaState(t, user.Id, now)
				broken.Teams = [][]string{{"c04", "c07"}, {"c04", "c06"}, {}, {}}
				broken.TeamOn = 1
				broken.Squad = []string{"c04", "c07"}
				seedGachaState(t, user.Id, broken)
				repaired := gachaState(t, user.Id, now)
				assert.Equal(t, []string{"c06"}, repaired.Teams[1])

				// The second preset refuses a girl still fielded by the first.
				slot := 2
				_, err := model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "team_select", Count: &slot}, now)
				require.NoError(t, err)
				shared := []string{"c04"}
				_, err = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "squad", Squad: &shared}, now)
				assert.ErrorIs(t, err, model.ErrGachaTeamShared)
				assert.Equal(t, []string{"c06"}, gachaState(t, user.Id, now).Squad)

				// A free girl still joins, and duplicates inside one preset stay rejected.
				free := []string{"c06", "c09"}
				out, err := model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "squad", Squad: &free}, now)
				require.NoError(t, err)
				assert.Equal(t, []string{"c06", "c09"}, out.Save.Squad)
				assert.Equal(t, []string{"c04", "c07"}, out.Save.Teams[0])
				assert.Equal(t, []string{"c06", "c09"}, out.Save.Teams[1])
				duplicate := []string{"c09", "c09"}
				_, err = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "squad", Squad: &duplicate}, now)
				assert.ErrorIs(t, err, model.ErrGachaTeam)
			})

			t.Run("legacy_save_with_stale_or_shared_squad_entries_is_repaired_not_rejected", func(t *testing.T) {
				user := gachaUser(t)
				state := gachaState(t, user.Id, now)
				state.Teams = nil
				state.TeamOn = 0
				// A save written before presets existed: one live squad that still
				// names a girl the account no longer owns, plus a duplicate id.
				state.Squad = []string{"c08", "c06", "ghost_girl", "c08"}
				seedGachaState(t, user.Id, state)
				out, err := model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "team_select", Count: ptrTo(1)}, now)
				require.NoError(t, err)
				require.Len(t, out.Save.Teams, 4)
				assert.Equal(t, []string{"c08", "c06"}, out.Save.Teams[0])
				assert.Equal(t, 1, out.Save.TeamOn)
				assert.Equal(t, []string{"c08", "c06"}, out.Save.Squad)
			})

			t.Run("growth_page_save_of_team_two_is_checked_at_save_time_not_at_load", func(t *testing.T) {
				user := gachaUser(t)
				state := gachaState(t, user.Id, now)
				for _, id := range []string{"c04", "c07", "c09", "c28", "c11", "c05", "c13", "c10"} {
					branch := model.GachaAwakeningBranches(id)[0].ID
					state.Girls[id] = &model.GachaOwnedGirl{Level: 60, Star: 5, Awakening: 3, Branch: branch, Gems: make([]string, 3)}
				}
				state.Cleared["s6"] = true
				state.Teams = [][]string{{"c04", "c07", "c11", "c05"}, {"c09", "c28", "c13", "c10"}, {}, {}}
				state.TeamOn = 2
				state.Squad = slices.Clone(state.Teams[1])
				seedGachaState(t, user.Id, state)

				// Trial girls may also belong to ordinary presets; saving never edits them.
				shared := []string{"c09", "c04", "c13", "c10", "c11", "c05", "c07", "c28"}
				out, err := model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "teams_save", Squad: &shared}, now)
				require.NoError(t, err)
				assert.Equal(t, shared[:4], out.Save.Progression.TrialSquad)
				assert.Equal(t, shared[4:], out.Save.Progression.ReserveSquad)
				after := gachaState(t, user.Id, now)
				assert.Equal(t, []string{"c09", "c28", "c13", "c10"}, after.Squad)
				assert.Equal(t, []string{"c04", "c07", "c11", "c05"}, after.Teams[0])
				_, err = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "team_select", Count: ptrTo(1)}, now)
				require.NoError(t, err)
				after = gachaState(t, user.Id, now)
				assert.Equal(t, shared[:4], after.Progression.TrialSquad)
				assert.Equal(t, shared[4:], after.Progression.ReserveSquad)
				_, err = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "team_select", Count: ptrTo(2)}, now)
				require.NoError(t, err)

				// The squad editor rejects the same girl, so both entries agree.
				editor := []string{"c09", "c04"}
				_, err = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "squad", Squad: &editor}, now)
				assert.ErrorIs(t, err, model.ErrGachaTeamShared)

				// A reload changes nothing: no ghost write reached the save.
				reloaded := gachaState(t, user.Id, now)
				assert.Equal(t, 2, reloaded.TeamOn)
				assert.Equal(t, []string{"c09", "c28", "c13", "c10"}, reloaded.Teams[1])
			})

			t.Run("legacy_duplicate_is_repaired_in_favour_of_the_team_that_is_fighting", func(t *testing.T) {
				user := gachaUser(t)
				state := gachaState(t, user.Id, now)
				for _, id := range []string{"c04", "c07", "c09", "c28"} {
					state.Girls[id] = &model.GachaOwnedGirl{Level: 20, Star: 3}
				}
				// A save written before the save-time check existed: preset two is
				// live and shares c04 with preset one. Loading must keep her where
				// the player is fighting, not strip the live team.
				state.Teams = [][]string{{"c04", "c07"}, {"c04", "c09"}, {}, {}}
				state.TeamOn = 2
				state.Squad = []string{"c04", "c09"}
				seedGachaState(t, user.Id, state)
				out, err := model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "team_select", Count: ptrTo(2)}, now)
				require.NoError(t, err)
				assert.Equal(t, []string{"c07"}, out.Save.Teams[0])
				assert.Equal(t, []string{"c04", "c09"}, out.Save.Teams[1])
				assert.Equal(t, []string{"c04", "c09"}, out.Save.Squad)
			})

			t.Run("parallel_run_start_produces_only_one_active_run_and_stamina_charge", func(t *testing.T) {
				user := gachaUser(t)
				daily := model.GachaDaily{UserID: user.Id, Day: now.In(time.FixedZone("CST", 8*3600)).Format("2006-01-02"), Runs: 20}
				require.NoError(t, model.DB.Create(&daily).Error)
				start := make(chan struct{})
				errors := make([]error, 2)
				var wg sync.WaitGroup
				for i := range 2 {
					wg.Go(func() { <-start; _, errors[i] = model.StartGachaRun(user.Id, "s1", uuid.NewString(), now) })
				}
				close(start)
				wg.Wait()
				successes := 0
				for _, err := range errors {
					if err == nil {
						successes++
					} else {
						assert.ErrorIs(t, err, model.ErrGachaRunState)
					}
				}
				assert.Equal(t, 1, successes)
				var pending int64
				require.NoError(t, model.DB.Model(&model.GachaRun{}).Where("user_id = ? AND status = ? AND ready_at > ?", user.Id, "running", 0).Count(&pending).Error)
				assert.EqualValues(t, 1, pending)
				assert.Equal(t, 118, gachaState(t, user.Id, now).Stamina)
				require.NoError(t, model.DB.Where("user_id = ? AND day = ?", user.Id, daily.Day).First(&daily).Error)
				assert.Equal(t, 21, daily.Runs, "concurrent starts past 20 count and charge exactly once")
			})

			t.Run("run_time_ownership_and_repeated_rewards_are_server_authoritative", func(t *testing.T) {
				user, other := gachaUser(t), gachaUser(t)
				state := gachaState(t, user.Id, now)
				strongGachaState(state)
				seedGachaState(t, user.Id, state)
				requestID := uuid.NewString()
				run, err := model.StartGachaRun(user.Id, "s1", requestID, now)
				require.NoError(t, err)
				require.True(t, run.Result.Win)
				again, err := model.StartGachaRun(user.Id, "s1", requestID, now)
				require.NoError(t, err)
				assert.Equal(t, run, again)
				count := 1
				_, err = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: requestID, Action: "gacha", Count: &count}, now)
				assert.ErrorIs(t, err, model.ErrGachaReplay, "start and growth actions share the same request-id namespace")
				timeline := 0
				for _, event := range run.Result.Events {
					timeline += event.DurationMS
				}
				assert.Equal(t, timeline, run.Result.DurationMS, "settlement must not add a wait after the final battle event")
				assert.Equal(t, now.UnixMilli()+int64(timeline), run.ReadyAt)
				assert.Less(t, timeline, 30_000, "a strong squad's short victory must not inherit the old minimum wait")
				_, err = model.FinishGachaRun(user.Id, run.ID, false, time.UnixMilli(run.ReadyAt-1))
				assert.ErrorIs(t, err, model.ErrGachaNotReady)
				_, err = model.FinishGachaRun(other.Id, run.ID, false, time.UnixMilli(run.ReadyAt))
				assert.ErrorIs(t, err, model.ErrGachaRunState)
				require.NoError(t, model.DB.Model(&model.GachaDaily{}).Where("user_id = ?", user.Id).Update("quota", 7_500_000).Error)
				start := make(chan struct{})
				results := make([]*model.GachaFinishResult, 2)
				errors := make([]error, 2)
				var wg sync.WaitGroup
				for i := range 2 {
					wg.Go(func() {
						<-start
						results[i], errors[i] = model.FinishGachaRun(user.Id, run.ID, false, time.UnixMilli(run.ReadyAt))
					})
				}
				close(start)
				wg.Wait()
				for _, err := range errors {
					require.NoError(t, err)
				}
				assert.Equal(t, results[0], results[1])
				require.True(t, results[0].Win)
				assert.Positive(t, results[0].Quota, "reaching the former 15 E cap must not suppress concurrent settlement")
				assert.Equal(t, 7_500_000+results[0].Quota, results[0].Today.Quota)
				assert.Equal(t, run.Result.Stars, results[0].Stars)
				assert.Equal(t, run.Result.Rounds, results[0].Rounds)
				var wallet model.User
				require.NoError(t, model.DB.First(&wallet, user.Id).Error)
				assert.Equal(t, user.Quota+results[0].Quota, wallet.Quota)
				assert.Equal(t, results[0].Save.Gold, gachaState(t, user.Id, time.UnixMilli(run.ReadyAt)).Gold)
				assert.True(t, results[0].Save.Cleared["s1"])
				assert.Equal(t, run.Result.Stars, results[0].Save.Stars["s1"])
				_, err = model.StartGachaRun(user.Id, "s5", uuid.NewString(), time.UnixMilli(run.ReadyAt))
				require.NoError(t, err, "the server's victory unlocks the next stage")
			})

			t.Run("new_chapters_unlock_and_settle_on_the_server", func(t *testing.T) {
				user := gachaUser(t)
				clock := now
				previous := "s6"
				quota := 0
				for _, stage := range []string{"s7", "s8", "s9", "s10", "s11", "s12"} {
					state := gachaState(t, user.Id, clock)
					strongGachaState(state)
					state.Stamina = state.StaminaMax
					delete(state.Cleared, previous)
					seedGachaState(t, user.Id, state)
					_, err := model.StartGachaRun(user.Id, stage, uuid.NewString(), clock)
					require.ErrorIs(t, err, model.ErrGachaLocked, stage)
					state.Cleared[previous] = true
					seedGachaState(t, user.Id, state)
					run, err := model.StartGachaRun(user.Id, stage, uuid.NewString(), clock)
					require.NoError(t, err, stage)
					require.True(t, run.Result.Win, "a fully developed squad can clear %s", stage)
					clock = time.UnixMilli(run.ReadyAt)
					result, err := model.FinishGachaRun(user.Id, run.ID, false, clock)
					require.NoError(t, err, stage)
					assert.True(t, result.Save.Cleared[stage])
					assert.Positive(t, result.Quota)
					assert.Equal(t, run.Result.Stars, result.Save.Stars[stage])
					assert.Less(t, result.Save.Stamina, state.Stamina)
					again, err := model.FinishGachaRun(user.Id, run.ID, false, clock)
					require.NoError(t, err)
					assert.Equal(t, result, again, "new stages must not pay twice")
					quota += result.Quota
					previous = stage
				}
				var wallet model.User
				require.NoError(t, model.DB.First(&wallet, user.Id).Error)
				assert.Equal(t, user.Quota+quota, wallet.Quota)
			})

			t.Run("dungeon_balance_rewards_match_server_preview_and_stamina_cost", func(t *testing.T) {
				for _, tc := range []struct {
					stage     string
					low, high int
					cost      int
				}{
					{"s1", 75_000, 110_000, 2}, {"s5", 100_000, 150_000, 3}, {"s2", 200_000, 275_000, 5},
					{"s4", 225_000, 325_000, 6}, {"s3", 375_000, 500_000, 9}, {"s6", 475_000, 625_000, 11},
					{"s7", 425_000, 575_000, 10}, {"s8", 500_000, 675_000, 12}, {"s9", 625_000, 800_000, 14},
					{"s10", 600_000, 775_000, 14}, {"s11", 700_000, 900_000, 16}, {"s12", 750_000, 1_000_000, 18},
				} {
					user := gachaUser(t)
					state := gachaState(t, user.Id, now)
					strongGachaState(state)
					for _, stage := range []string{"s1", "s5", "s2", "s4", "s3", "s6", "s7", "s8", "s9", "s10", "s11", "s12"} {
						state.Cleared[stage] = true
					}
					seedGachaState(t, user.Id, state)
					profile, err := model.GetGachaProfile(user.Id, now)
					require.NoError(t, err)
					assert.Len(t, profile.StageRewards, 24)
					assert.Equal(t, tc.low, profile.StageRewards[tc.stage].QuotaMin)
					assert.Equal(t, tc.high, profile.StageRewards[tc.stage].QuotaMax)
					run, err := model.StartGachaRun(user.Id, tc.stage, uuid.NewString(), now)
					require.NoError(t, err)
					assert.Equal(t, state.Stamina-tc.cost, run.Save.Stamina, "higher rewards require the announced stamina cost")
					result, err := model.FinishGachaRun(user.Id, run.ID, false, time.UnixMilli(run.ReadyAt))
					require.NoError(t, err)
					require.True(t, result.Win)
					percent := 100 + (result.Stars-1)*10
					assert.GreaterOrEqual(t, result.Quota, tc.low*percent/100)
					assert.LessOrEqual(t, result.Quota, tc.high*percent/100)
					again, err := model.FinishGachaRun(user.Id, run.ID, false, time.UnixMilli(run.ReadyAt))
					require.NoError(t, err)
					assert.Equal(t, result, again)
					var wallet model.User
					require.NoError(t, model.DB.First(&wallet, user.Id).Error)
					assert.Equal(t, user.Quota+result.Quota, wallet.Quota)
				}
			})

			t.Run("runs_past_twenty_require_stamina_and_abandon_never_pays", func(t *testing.T) {
				user := gachaUser(t)
				state := gachaState(t, user.Id, now)
				strongGachaState(state)
				seedGachaState(t, user.Id, state)
				run, err := model.StartGachaRun(user.Id, "s1", uuid.NewString(), now)
				require.NoError(t, err)
				result, err := model.FinishGachaRun(user.Id, run.ID, true, now)
				require.NoError(t, err)
				assert.False(t, result.Win)
				assert.Zero(t, result.Quota)
				assert.Zero(t, result.Stars)
				assert.Equal(t, 600, result.Save.Gold)
				require.NoError(t, model.DB.Model(&model.GachaDaily{}).Where("user_id = ?", user.Id).Update("runs", 20).Error)
				requestID := uuid.NewString()
				next, err := model.StartGachaRun(user.Id, "s1", requestID, now)
				require.NoError(t, err, "the twenty-first run must be allowed")
				assert.Equal(t, result.Save.Stamina-2, next.Save.Stamina)
				replayed, err := model.StartGachaRun(user.Id, "s1", requestID, now)
				require.NoError(t, err)
				assert.Equal(t, next, replayed)
				clock := time.UnixMilli(next.ReadyAt)
				win, err := model.FinishGachaRun(user.Id, next.ID, false, clock)
				require.NoError(t, err)
				require.True(t, win.Win)
				assert.Positive(t, win.Quota)
				assert.Equal(t, 21, win.Today.Runs)
				assert.Nil(t, win.Today.RunLimit)
				next, err = model.StartGachaRun(user.Id, "s1", uuid.NewString(), clock)
				require.NoError(t, err, "later runs remain available")
				abandoned, err := model.FinishGachaRun(user.Id, next.ID, true, clock)
				require.NoError(t, err)
				assert.Zero(t, abandoned.Quota)
				assert.Equal(t, 22, abandoned.Today.Runs)
				state = abandoned.Save
				state.Stamina = 0
				state.StaminaTs = clock.UnixMilli()
				seedGachaState(t, user.Id, state)
				_, err = model.StartGachaRun(user.Id, "s1", uuid.NewString(), clock)
				assert.ErrorIs(t, err, model.ErrGachaFunds, "unlimited runs must not bypass stamina costs")
				profile, err := model.GetGachaProfile(user.Id, clock)
				require.NoError(t, err)
				assert.Equal(t, 22, profile.Today.Runs)
				assert.Equal(t, user.Quota+win.Quota, profile.Quota)
				assert.Nil(t, profile.Today.RunLimit)
			})

			t.Run("expired_runs_and_token_limits_preserve_full_balance_rewards", func(t *testing.T) {
				user := gachaUser(t)
				state := gachaState(t, user.Id, now)
				strongGachaState(state)
				state.Daily.TokenGained = 99
				seedGachaState(t, user.Id, state)
				profile, err := model.GetGachaProfile(user.Id, now)
				require.NoError(t, err)
				assert.Nil(t, profile.Today.QuotaLimit, "daily balance earnings have no cap")
				run, err := model.StartGachaRun(user.Id, "s1", uuid.NewString(), now)
				require.NoError(t, err)
				_, err = model.FinishGachaRun(user.Id, run.ID, false, time.UnixMilli(run.ExpireAt))
				assert.ErrorIs(t, err, model.ErrGachaRunState)
				const previousEarnings = 7_500_000 - 10
				require.NoError(t, model.DB.Model(&model.GachaDaily{}).Where("user_id = ?", user.Id).Update("quota", previousEarnings).Error)
				result, err := model.FinishGachaRun(user.Id, run.ID, false, time.UnixMilli(run.ReadyAt))
				require.NoError(t, err)
				require.True(t, result.Win)
				percent := 100 + (result.Stars-1)*10
				assert.GreaterOrEqual(t, result.Quota, 75_000*percent/100, "crossing 15 E must award the full stage reward")
				assert.LessOrEqual(t, result.Quota, 110_000*percent/100)
				assert.Equal(t, previousEarnings+result.Quota, result.Today.Quota)
				assert.Nil(t, result.Today.QuotaLimit)
				assert.Nil(t, result.QuotaLeft)
				assert.Equal(t, 100, result.Save.Daily.TokenGained)
				assert.Equal(t, 1, result.Save.Token)
				clock := time.UnixMilli(run.ReadyAt)
				next, err := model.StartGachaRun(user.Id, "s1", uuid.NewString(), clock)
				require.NoError(t, err)
				uncapped, err := model.FinishGachaRun(user.Id, next.ID, false, time.UnixMilli(next.ReadyAt))
				require.NoError(t, err)
				require.True(t, uncapped.Win)
				percent = 100 + (uncapped.Stars-1)*10
				assert.GreaterOrEqual(t, uncapped.Quota, 75_000*percent/100, "further victories above 15 E still award the full stage reward")
				assert.LessOrEqual(t, uncapped.Quota, 110_000*percent/100)
				assert.Equal(t, previousEarnings+result.Quota+uncapped.Quota, uncapped.Today.Quota)
				assert.Nil(t, uncapped.QuotaLeft)
				assert.Equal(t, 100, uncapped.Save.Daily.TokenGained)
				assert.Equal(t, 1, uncapped.Save.Token)
				again, err := model.FinishGachaRun(user.Id, next.ID, false, time.UnixMilli(next.ReadyAt))
				require.NoError(t, err)
				assert.Equal(t, uncapped, again, "unlimited daily earnings must not duplicate a run's reward")
				var wallet model.User
				require.NoError(t, model.DB.First(&wallet, user.Id).Error)
				assert.Equal(t, user.Quota+result.Quota+uncapped.Quota, wallet.Quota)
			})

			t.Run("dungeon_reward_wallet_capacity_failure_is_atomic", func(t *testing.T) {
				user := gachaUser(t)
				state := gachaState(t, user.Id, now)
				strongGachaState(state)
				seedGachaState(t, user.Id, state)
				run, err := model.StartGachaRun(user.Id, "s1", uuid.NewString(), now)
				require.NoError(t, err)
				require.True(t, run.Result.Win)
				require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", user.Id).Update("quota", common.MaxWalletQuota).Error)
				clock := time.UnixMilli(run.ReadyAt)
				_, err = model.FinishGachaRun(user.Id, run.ID, false, clock)
				assert.ErrorIs(t, err, model.ErrGachaRunState)
				var player model.GachaPlayer
				require.NoError(t, model.DB.Where("user_id = ?", user.Id).First(&player).Error)
				assert.Equal(t, run.Revision, player.Revision)
				profile, err := model.GetGachaProfile(user.Id, clock)
				require.NoError(t, err)
				assert.EqualValues(t, common.MaxWalletQuota, profile.Quota)
				assert.Zero(t, profile.Today.Quota)
				assert.Equal(t, run.Save.Gold, profile.Save.Gold)
				assert.Equal(t, run.Save.Token, profile.Save.Token)
				require.NotNil(t, profile.ActiveRun)
				assert.Equal(t, run.ID, profile.ActiveRun.ID)
				require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", user.Id).Update("quota", 500_000).Error)
				result, err := model.FinishGachaRun(user.Id, run.ID, false, clock)
				require.NoError(t, err)
				assert.Positive(t, result.Quota)
				assert.Equal(t, 500_000+result.Quota, result.WalletQuota)
			})

			t.Run("card_activation_is_owner_bound_and_only_one_card_can_be_active", func(t *testing.T) {
				user, other := gachaUser(t), gachaUser(t)
				cards := []model.GachaCard{
					{ID: uuid.NewString(), UserID: user.Id, Name: "card-one", Kind: "discount_5", Status: "stored", Discount: 5, DurationMin: 60},
					{ID: uuid.NewString(), UserID: user.Id, Name: "card-two", Kind: "discount_10", Status: "stored", Discount: 10, DurationMin: 45},
				}
				require.NoError(t, model.DB.Create(&cards).Error)
				_, err := model.ActivateGachaCard(other.Id, cards[0].ID, now)
				assert.ErrorIs(t, err, model.ErrGachaCardState)
				start := make(chan struct{})
				errors := make([]error, 2)
				var wg sync.WaitGroup
				for i := range 2 {
					wg.Go(func() { <-start; _, errors[i] = model.ActivateGachaCard(user.Id, cards[i].ID, now) })
				}
				close(start)
				wg.Wait()
				successes := 0
				for _, err := range errors {
					if err == nil {
						successes++
					} else {
						assert.ErrorIs(t, err, model.ErrGachaCardState)
					}
				}
				assert.Equal(t, 1, successes)
				var active, stored int64
				require.NoError(t, model.DB.Model(&model.GachaCard{}).Where("user_id = ? AND status = ?", user.Id, "active").Count(&active).Error)
				require.NoError(t, model.DB.Model(&model.GachaCard{}).Where("user_id = ? AND status = ?", user.Id, "stored").Count(&stored).Error)
				assert.EqualValues(t, 1, active)
				assert.EqualValues(t, 1, stored)
			})

			t.Run("shop_spending_is_atomic_idempotent_and_daily_limited", func(t *testing.T) {
				user := gachaUser(t)
				state := gachaState(t, user.Id, now)
				state.Token, state.Stamina = 500, 60
				state.Daily.ShopBought = map[string]int{"mat_r": 3}
				seedGachaState(t, user.Id, state)
				item := "gold"
				intent := model.GachaActionIntent{RequestID: uuid.NewString(), Action: "shop", ItemID: &item}
				results := make([]*model.GachaActionResult, 2)
				errs := make([]error, 2)
				var wg sync.WaitGroup
				for i := range 2 {
					wg.Go(func() { results[i], errs[i] = model.ApplyGachaAction(user.Id, intent, now) })
				}
				wg.Wait()
				for _, err := range errs {
					require.NoError(t, err)
				}
				assert.Equal(t, results[0], results[1])
				assert.Equal(t, 470, results[0].Save.Token)
				assert.Equal(t, 750, results[0].Save.Gold)
				assert.Equal(t, 3, results[0].Save.Daily.ShopBought["mat_r"], "existing daily purchases survive the new shared limit")
				for range 2 {
					intent.RequestID = uuid.NewString()
					_, err := model.ApplyGachaAction(user.Id, intent, now)
					require.NoError(t, err)
				}
				intent.RequestID = uuid.NewString()
				_, err := model.ApplyGachaAction(user.Id, intent, now)
				require.NoError(t, err, "the former three-purchase item limit no longer applies")
				social, err := model.GetGachaSocial(user.Id)
				require.NoError(t, err)
				assert.Equal(t, 20, social.ShopDailyLimit)
				assert.Equal(t, 500000, social.TokenExchange.QuotaPerToken)
				for _, tc := range []struct {
					id           string
					cost, amount int
				}{{"stamina", 20, 30}, {"mat_r", 15, 20}, {"mat_sr", 25, 10}, {"mat_ssr", 40, 5}, {"multi", 80, 1}} {
					before := gachaState(t, user.Id, now)
					out, err := model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "shop", ItemID: &tc.id}, now)
					require.NoError(t, err, tc.id)
					assert.Equal(t, before.Token-tc.cost, out.Save.Token)
					switch tc.id {
					case "stamina":
						assert.Equal(t, before.Stamina+tc.amount, out.Save.Stamina)
					case "multi":
						assert.Equal(t, before.Multi+tc.amount, out.Save.Multi)
					default:
						assert.Equal(t, before.Mats[tc.id]+tc.amount, out.Save.Mats[tc.id])
					}
					assert.Equal(t, before.Daily.TokenGained, out.Save.Daily.TokenGained)
				}
				intent.RequestID = uuid.NewString()
				next, err := model.ApplyGachaAction(user.Id, intent, now.Add(24*time.Hour))
				require.NoError(t, err)
				assert.Equal(t, 1, next.Save.Daily.ShopBought["gold"])
				assert.Zero(t, next.Save.Daily.ShopBought["mat_r"])
				item = "stamina"
				before := gachaState(t, user.Id, now.Add(24*time.Hour))
				_, err = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "shop", ItemID: &item}, now.Add(24*time.Hour))
				require.ErrorIs(t, err, model.ErrGachaCapacity)
				assert.Equal(t, before.Token, gachaState(t, user.Id, now.Add(24*time.Hour)).Token)
				poor := gachaUser(t)
				item = "gold"
				_, err = model.ApplyGachaAction(poor.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "shop", ItemID: &item}, now)
				require.ErrorIs(t, err, model.ErrGachaFunds)
				for _, extra := range []string{"price", "amount", "power", "points", "win", "report", "save"} {
					body := map[string]any{"request_id": uuid.NewString(), "action": "shop", "item_id": "gold", extra: 1}
					assert.Equal(t, http.StatusBadRequest, gachaRequest(t, user.Id, true, controller.ApplyGachaAction, body).Code)
				}
				count, opponent := 1, user.Id
				for _, invalid := range []model.GachaActionIntent{
					{Action: "shop", ItemID: &item, Count: &count}, {Action: "shop", ItemID: &item, OpponentID: &opponent},
					{Action: "gacha", Count: &count, ItemID: &item}, {Action: "arena", OpponentID: &opponent, ItemID: &item},
				} {
					invalid.RequestID = uuid.NewString()
					assert.ErrorIs(t, model.ValidateGachaAction(invalid), model.ErrGachaIntent)
				}
			})

			t.Run("twentieth_shop_purchase_wins_once_across_items", func(t *testing.T) {
				user := gachaUser(t)
				state := gachaState(t, user.Id, now)
				state.Token = 1000
				state.Daily.ShopBought = map[string]int{"gold": 18, "mat_r": 1}
				seedGachaState(t, user.Id, state)
				items := []string{"gold", "mat_sr"}
				errs := make([]error, len(items))
				var wg sync.WaitGroup
				for i := range items {
					wg.Go(func() {
						_, errs[i] = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "shop", ItemID: &items[i]}, now)
					})
				}
				wg.Wait()
				success := 0
				for _, err := range errs {
					if err == nil {
						success++
					} else {
						assert.ErrorIs(t, err, model.ErrGachaShopLimit)
					}
				}
				assert.Equal(t, 1, success)
				after := gachaState(t, user.Id, now)
				total := 0
				for _, n := range after.Daily.ShopBought {
					total += n
				}
				assert.Equal(t, 20, total)
				assert.Contains(t, []int{970, 975}, after.Token)
			})

			t.Run("token_exchange_is_atomic_and_replay_safe", func(t *testing.T) {
				user := gachaUser(t)
				require.NoError(t, model.DB.Model(&user).Update("quota", 3*model.GachaArenaQuotaPerE+123).Error)
				state := gachaState(t, user.Id, now)
				state.Daily.TokenGained = 100
				state.Daily.ShopBought = map[string]int{"gold": 20}
				seedGachaState(t, user.Id, state)
				count := 3
				intent := model.GachaActionIntent{RequestID: uuid.NewString(), Action: "token_exchange", Count: &count}
				results, errs := make([]*model.GachaActionResult, 2), make([]error, 2)
				var wg sync.WaitGroup
				for i := range 2 {
					wg.Go(func() { results[i], errs[i] = model.ApplyGachaAction(user.Id, intent, now) })
				}
				wg.Wait()
				for _, err := range errs {
					require.NoError(t, err)
				}
				assert.Equal(t, results[0], results[1])
				assert.Equal(t, &model.GachaTokenExchangeReceipt{Count: 3, CostQuota: 1500000, WalletQuota: 123}, results[0].Exchange)
				assert.Equal(t, 3, results[0].Save.Token)
				assert.Equal(t, state.Daily, results[0].Save.Daily, "exchange uses neither shop allowance nor dungeon token budget")
				var wallet model.User
				require.NoError(t, model.DB.First(&wallet, user.Id).Error)
				assert.Equal(t, 123, wallet.Quota)
				count = 1
				_, err := model.ApplyGachaAction(user.Id, intent, now)
				assert.ErrorIs(t, err, model.ErrGachaReplay)
				intent.RequestID = uuid.NewString()
				_, err = model.ApplyGachaAction(user.Id, intent, now)
				assert.ErrorIs(t, err, model.ErrGachaWalletFunds)
				assert.Equal(t, 3, gachaState(t, user.Id, now).Token)
			})

			t.Run("token_exchange_rolls_back_if_receipt_cannot_commit", func(t *testing.T) {
				user := gachaUser(t)
				require.NoError(t, model.DB.Model(&user).Update("quota", 500000).Error)
				gachaState(t, user.Id, now)
				count := 1
				intent := model.GachaActionIntent{RequestID: uuid.NewString(), Action: "token_exchange", Count: &count}
				injected := errors.New("isolated receipt write failure")
				callback := "gacha_test_exchange_receipt"
				require.NoError(t, model.DB.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
					if tx.Statement.Table == "gacha_actions" {
						tx.AddError(injected)
					}
				}))
				_, err := model.ApplyGachaAction(user.Id, intent, now)
				require.NoError(t, model.DB.Callback().Create().Remove(callback))
				assert.ErrorIs(t, err, injected)
				var wallet model.User
				require.NoError(t, model.DB.First(&wallet, user.Id).Error)
				assert.Equal(t, 500000, wallet.Quota)
				assert.Zero(t, gachaState(t, user.Id, now).Token)
				out, err := model.ApplyGachaAction(user.Id, intent, now)
				require.NoError(t, err)
				assert.Equal(t, 1, out.Save.Token)
				assert.Zero(t, out.Exchange.WalletQuota)
			})

			t.Run("token_exchange_validates_amount_and_capacity_before_debit", func(t *testing.T) {
				user := gachaUser(t)
				for _, count := range []any{nil, 0, -1, 1.5, 1_000_000_001, 1e30, "1"} {
					body := map[string]any{"request_id": uuid.NewString(), "action": "token_exchange", "count": count}
					assert.Equal(t, http.StatusBadRequest, gachaRequest(t, user.Id, true, controller.ApplyGachaAction, body).Code)
				}
				for _, extra := range []string{"price", "rate", "cost_quota", "wallet_quota", "token", "save", "item_id", "stake_e"} {
					body := map[string]any{"request_id": uuid.NewString(), "action": "token_exchange", "count": 1, extra: 1}
					assert.Equal(t, http.StatusBadRequest, gachaRequest(t, user.Id, true, controller.ApplyGachaAction, body).Code)
				}
				body := map[string]any{"request_id": uuid.NewString(), "action": "token_exchange", "count": 1}
				assert.Equal(t, http.StatusForbidden, gachaRequest(t, user.Id, false, controller.ApplyGachaAction, body).Code)
				require.NoError(t, model.DB.Model(&user).Update("quota", 500_000_000_000_000).Error)
				count := 1_000_000_000
				out, err := model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "token_exchange", Count: &count}, now)
				require.NoError(t, err)
				assert.Equal(t, count, out.Save.Token)
				assert.Equal(t, 500_000_000_000_000, out.Exchange.CostQuota)
				assert.Zero(t, out.Exchange.WalletQuota)
				require.NoError(t, model.DB.Model(&user).Update("quota", 500000).Error)
				count = 1
				_, err = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "token_exchange", Count: &count}, now)
				assert.ErrorIs(t, err, model.ErrGachaCapacity)
				var wallet model.User
				require.NoError(t, model.DB.First(&wallet, user.Id).Error)
				assert.Equal(t, 500000, wallet.Quota)
				previousBatch := common.BatchUpdateEnabled
				common.BatchUpdateEnabled = true
				t.Cleanup(func() { common.BatchUpdateEnabled = previousBatch })
				_, err = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "token_exchange", Count: &count}, now)
				assert.ErrorIs(t, err, model.ErrGachaExchangeUnavailable)
			})

			t.Run("token_exchange_competes_safely_with_wallet_spending", func(t *testing.T) {
				for _, competitor := range []string{"exchange", "relay", "arena"} {
					t.Run(competitor, func(t *testing.T) {
						user := gachaUser(t)
						require.NoError(t, model.DB.Model(&user).Update("quota", 500000).Error)
						gachaState(t, user.Id, now)
						count := 1
						var firstErr, secondErr error
						var reserved bool
						var wg sync.WaitGroup
						wg.Go(func() {
							_, firstErr = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "token_exchange", Count: &count}, now)
						})
						wg.Go(func() {
							if competitor == "relay" {
								reserved, secondErr = model.TryReserveUserQuota(user.Id, 500000)
								return
							}
							intent := model.GachaActionIntent{RequestID: uuid.NewString(), Action: "token_exchange", Count: &count}
							if competitor == "arena" {
								intent.Action, intent.Count, intent.StakeE = "arena_join", nil, &count
							}
							_, secondErr = model.ApplyGachaAction(user.Id, intent, now)
							reserved = secondErr == nil
						})
						wg.Wait()
						if firstErr == nil {
							assert.False(t, reserved)
						} else {
							assert.ErrorIs(t, firstErr, model.ErrGachaWalletFunds)
							assert.True(t, reserved)
						}
						if competitor == "relay" {
							require.NoError(t, secondErr)
						} else if !reserved {
							expected := model.ErrGachaWalletFunds
							if competitor == "arena" {
								expected = model.ErrGachaArenaFunds
							}
							assert.ErrorIs(t, secondErr, expected)
						}
						var wallet model.User
						require.NoError(t, model.DB.First(&wallet, user.Id).Error)
						assert.Zero(t, wallet.Quota)
						expectedTokens := 0
						if firstErr == nil || competitor == "exchange" {
							expectedTokens = 1
						}
						assert.Equal(t, expectedTokens, gachaState(t, user.Id, now).Token)
						if competitor == "arena" && reserved {
							seat, err := model.GetGachaArenaQueue(user.Id, now)
							require.NoError(t, err)
							_, err = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "arena_cancel", QueueID: &seat.ID}, now)
							require.NoError(t, err)
						}
					})
				}
			})

			t.Run("parallel_shop_purchases_cannot_overdraw_tokens", func(t *testing.T) {
				user := gachaUser(t)
				state := gachaState(t, user.Id, now)
				state.Token = 30
				seedGachaState(t, user.Id, state)
				item := "gold"
				errs := make([]error, 2)
				var wg sync.WaitGroup
				for i := range 2 {
					wg.Go(func() {
						_, errs[i] = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "shop", ItemID: &item}, now)
					})
				}
				wg.Wait()
				success := 0
				for _, err := range errs {
					if err == nil {
						success++
					} else {
						assert.ErrorIs(t, err, model.ErrGachaFunds)
					}
				}
				assert.Equal(t, 1, success)
				after := gachaState(t, user.Id, now)
				assert.Zero(t, after.Token)
				assert.Equal(t, 750, after.Gold)
				assert.Equal(t, 1, after.Daily.ShopBought[item])
			})

			t.Run("public_rooms_require_two_ready_players_and_isolate_random_queue", func(t *testing.T) {
				host, guest, outsider := gachaRoomPlayer(t, now), gachaRoomPlayer(t, now), gachaRoomPlayer(t, now)
				stake := 5
				create := model.GachaActionIntent{RequestID: uuid.NewString(), Action: "room_create", StakeE: &stake}
				created, err := model.ApplyGachaAction(host.Id, create, now)
				require.NoError(t, err)
				id := created.Room.ID
				assert.Equal(t, 10*model.GachaArenaQuotaPerE, created.Queue.Quota, "creating a room does not reserve money")
				again, err := model.ApplyGachaAction(host.Id, create, now)
				require.NoError(t, err)
				assert.Equal(t, created, again)
				lobby, err := model.GetGachaArenaRooms(guest.Id, now)
				require.NoError(t, err)
				assert.Contains(t, lobby.Rooms, created.Room.GachaArenaRoom)
				_, err = model.ApplyGachaAction(outsider.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "room_ready", RoomID: &id}, now)
				assert.ErrorIs(t, err, model.ErrGachaIntent)
				joined, err := model.ApplyGachaAction(guest.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "room_join", RoomID: &id}, now)
				require.NoError(t, err)
				assert.Equal(t, 10*model.GachaArenaQuotaPerE, joined.Queue.Quota)
				_, err = model.ApplyGachaAction(outsider.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "room_join", RoomID: &id}, now)
				assert.ErrorIs(t, err, model.ErrGachaRoom)
				ready := model.GachaActionIntent{RequestID: uuid.NewString(), Action: "room_ready", RoomID: &id}
				first, err := model.ApplyGachaAction(host.Id, ready, now)
				require.NoError(t, err)
				assert.Nil(t, first.Arena)
				assert.Equal(t, 5*model.GachaArenaQuotaPerE, first.Queue.Quota)
				_, err = model.ApplyGachaAction(host.Id, ready, now)
				require.NoError(t, err)
				queued, err := model.ApplyGachaAction(outsider.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "arena_join", StakeE: &stake}, now)
				require.NoError(t, err)
				assert.Equal(t, "waiting", queued.Queue.Status, "random matching cannot select a ready room player")
				team := []string{"c08"}
				_, err = model.ApplyGachaAction(guest.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "squad", Squad: &team}, now)
				require.NoError(t, err)
				paired, err := model.ApplyGachaAction(guest.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "room_ready", RoomID: &id}, now)
				require.NoError(t, err)
				require.NotNil(t, paired.Arena)
				assert.Equal(t, "c01", paired.Arena.Report.Allies[0].GirlID, "ready uses the frozen entry lineup")
				assert.Equal(t, "matched", paired.Room.Status)
				var wallets []model.User
				require.NoError(t, model.DB.Where("id IN ?", []int{host.Id, guest.Id}).Find(&wallets).Error)
				assert.Equal(t, 20*model.GachaArenaQuotaPerE, wallets[0].Quota+wallets[1].Quota)
				for _, wallet := range wallets {
					assert.Contains(t, []int{5 * model.GachaArenaQuotaPerE, 15 * model.GachaArenaQuotaPerE}, wallet.Quota)
				}
				_, err = model.GetGachaArenaReplay(outsider.Id, paired.Arena.ID)
				assert.ErrorIs(t, err, model.ErrGachaIntent)
				_, err = model.ApplyGachaAction(outsider.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "arena_cancel", QueueID: &queued.Queue.ID}, now)
				require.NoError(t, err)
			})

			t.Run("room_exit_expiry_and_concurrent_join_keep_wallets_safe", func(t *testing.T) {
				for _, expire := range []bool{false, true} {
					host, guest, other := gachaRoomPlayer(t, now), gachaRoomPlayer(t, now), gachaRoomPlayer(t, now)
					stake := 1
					created, err := model.ApplyGachaAction(host.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "room_create", StakeE: &stake}, now)
					require.NoError(t, err)
					id := created.Room.ID
					users := []int{guest.Id, other.Id}
					results := make([]*model.GachaActionResult, 2)
					errs := make([]error, 2)
					var wg sync.WaitGroup
					for i := range 2 {
						wg.Go(func() {
							results[i], errs[i] = model.ApplyGachaAction(users[i], model.GachaActionIntent{RequestID: uuid.NewString(), Action: "room_join", RoomID: &id}, now)
						})
					}
					wg.Wait()
					winner := 0
					if errs[0] != nil {
						winner = 1
					}
					require.NoError(t, errs[winner])
					assert.ErrorIs(t, errs[1-winner], model.ErrGachaRoom)
					_, err = model.ApplyGachaAction(host.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "room_ready", RoomID: &id}, now)
					require.NoError(t, err)
					if expire {
						require.NoError(t, model.ExpireGachaArenaSeats(context.Background(), now.Add(model.GachaRoomTTL)))
						require.NoError(t, model.ExpireGachaArenaSeats(context.Background(), now.Add(model.GachaRoomTTL)))
					} else {
						_, err = model.ApplyGachaAction(users[winner], model.GachaActionIntent{RequestID: uuid.NewString(), Action: "room_leave", RoomID: &id}, now)
						require.NoError(t, err)
					}
					view, err := model.GetGachaArenaRooms(host.Id, now)
					require.NoError(t, err)
					assert.Equal(t, 10*model.GachaArenaQuotaPerE, view.Quota)
					assert.NotEqual(t, "open", view.Room.Status)
					assert.Nil(t, view.Room.Match)
				}
			})

			t.Run("rooms_reject_power_gap_and_failed_ready_does_not_charge", func(t *testing.T) {
				host, guest, strong := gachaRoomPlayer(t, now), gachaRoomPlayer(t, now), gachaRoomPlayer(t, now)
				state := gachaState(t, strong.Id, now)
				strongGachaState(state)
				seedGachaState(t, strong.Id, state)
				stake := 5
				created, err := model.ApplyGachaAction(host.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "room_create", StakeE: &stake}, now)
				require.NoError(t, err)
				id := created.Room.ID
				_, err = model.ApplyGachaAction(strong.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "room_join", RoomID: &id}, now)
				assert.ErrorIs(t, err, model.ErrGachaRoom)
				_, err = model.ApplyGachaAction(guest.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "room_join", RoomID: &id}, now)
				require.NoError(t, err)
				_, err = model.ApplyGachaAction(host.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "room_ready", RoomID: &id}, now)
				require.NoError(t, err)
				require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", guest.Id).Update("quota", 0).Error)
				_, err = model.ApplyGachaAction(guest.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "room_ready", RoomID: &id}, now)
				assert.ErrorIs(t, err, model.ErrGachaArenaFunds)
				view, err := model.GetGachaArenaRooms(guest.Id, now)
				require.NoError(t, err)
				assert.True(t, view.Room.HostReady)
				assert.False(t, view.Room.GuestReady)
				assert.Nil(t, view.Room.Match)
				assert.Zero(t, view.Quota)
				_, err = model.ApplyGachaAction(guest.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "room_leave", RoomID: &id}, now)
				require.NoError(t, err)
				view, err = model.GetGachaArenaRooms(host.Id, now)
				require.NoError(t, err)
				assert.Equal(t, 10*model.GachaArenaQuotaPerE, view.Quota)
			})

			t.Run("concurrent_room_ready_and_leave_settle_or_refund_once", func(t *testing.T) {
				host, guest := gachaRoomPlayer(t, now), gachaRoomPlayer(t, now)
				stake := 3
				created, err := model.ApplyGachaAction(host.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "room_create", StakeE: &stake}, now)
				require.NoError(t, err)
				id := created.Room.ID
				_, err = model.ApplyGachaAction(guest.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "room_join", RoomID: &id}, now)
				require.NoError(t, err)
				_, err = model.ApplyGachaAction(host.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "room_ready", RoomID: &id}, now)
				require.NoError(t, err)
				var readyErr, leaveErr error
				var wg sync.WaitGroup
				wg.Go(func() {
					_, readyErr = model.ApplyGachaAction(guest.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "room_ready", RoomID: &id}, now)
				})
				wg.Go(func() {
					_, leaveErr = model.ApplyGachaAction(host.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "room_leave", RoomID: &id}, now)
				})
				wg.Wait()
				require.NoError(t, leaveErr)
				view, err := model.GetGachaArenaRooms(host.Id, now)
				require.NoError(t, err)
				var wallets []model.User
				require.NoError(t, model.DB.Where("id IN ?", []int{host.Id, guest.Id}).Find(&wallets).Error)
				require.Len(t, wallets, 2)
				assert.Equal(t, 20*model.GachaArenaQuotaPerE, wallets[0].Quota+wallets[1].Quota)
				if view.Room.Status == "matched" {
					require.NoError(t, readyErr)
					require.NotNil(t, view.Room.Match)
					for _, wallet := range wallets {
						assert.Contains(t, []int{7 * model.GachaArenaQuotaPerE, 13 * model.GachaArenaQuotaPerE}, wallet.Quota)
					}
				} else {
					assert.Equal(t, "cancelled", view.Room.Status)
					assert.ErrorIs(t, readyErr, model.ErrGachaRoom)
					assert.Nil(t, view.Room.Match)
					for _, wallet := range wallets {
						assert.Equal(t, 10*model.GachaArenaQuotaPerE, wallet.Quota)
					}
				}
				var entries []model.GachaArenaEntry
				require.NoError(t, model.DB.Where("queue_id = ?", created.Queue.ID).Find(&entries).Error)
				assert.Len(t, entries, 2, "exactly one reserve and one release regardless of the race")
			})

			t.Run("offline_dungeon_completion_settles_once_and_returns_wallet_receipt", func(t *testing.T) {
				user := gachaUser(t)
				state := gachaState(t, user.Id, now)
				strongGachaState(state)
				seedGachaState(t, user.Id, state)
				run, err := model.StartGachaRun(user.Id, "s1", uuid.NewString(), now)
				require.NoError(t, err)
				require.True(t, run.Result.Win)
				require.NoError(t, model.DB.Model(&model.GachaDaily{}).Where("user_id = ?", user.Id).Update("quota", 8_000_000).Error)
				require.NoError(t, model.SettleCompletedGachaRuns(context.Background(), time.UnixMilli(run.ReadyAt)))
				require.NoError(t, model.SettleCompletedGachaRuns(context.Background(), time.UnixMilli(run.ReadyAt)))
				finish, err := model.FinishGachaRun(user.Id, run.ID, false, time.UnixMilli(run.ReadyAt))
				require.NoError(t, err)
				assert.Positive(t, finish.Quota)
				assert.Equal(t, 8_000_000+finish.Quota, finish.Today.Quota)
				assert.Equal(t, user.Quota+finish.Quota, finish.WalletQuota)
				var wallet model.User
				require.NoError(t, model.DB.First(&wallet, user.Id).Error)
				assert.Equal(t, finish.WalletQuota, wallet.Quota)
			})

			t.Run("all_eight_new_characters_require_experience_and_join_owned_teams", func(t *testing.T) {
				user := gachaUser(t)
				state := gachaState(t, user.Id, now)
				state.Gold = 10000
				for i := 21; i <= 28; i++ {
					state.Girls[fmt.Sprintf("c%d", i)] = &model.GachaOwnedGirl{Level: 1, Star: 1}
				}
				seedGachaState(t, user.Id, state)
				for i := 21; i <= 28; i++ {
					id := fmt.Sprintf("c%d", i)
					out, err := model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "level", GirlID: &id}, now)
					assert.ErrorIs(t, err, model.ErrGachaExperience)
					assert.Nil(t, out)
					team := []string{id}
					out, err = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "squad", Squad: &team}, now)
					require.NoError(t, err)
					assert.Positive(t, out.Save.Power)
				}
			})

			t.Run("matchmaking_requires_two_consents_same_stake_and_conserves_wallets", func(t *testing.T) {
				user, other, outsider := gachaUser(t), gachaUser(t), gachaUser(t)
				for _, id := range []int{user.Id, other.Id, outsider.Id} {
					require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", id).Update("quota", 10*model.GachaArenaQuotaPerE).Error)
				}
				state := gachaState(t, user.Id, now)
				state.Squad = []string{"c01"}
				state.Girls["c01"] = &model.GachaOwnedGirl{Star: 1, Level: 20}
				seedGachaState(t, user.Id, state)
				otherState := gachaState(t, other.Id, now)
				otherState.Squad = []string{"c01"}
				otherState.Girls["c01"] = &model.GachaOwnedGirl{Star: 1, Level: 20}
				seedGachaState(t, other.Id, otherState)
				_, err := model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "arena", OpponentID: &outsider.Id}, now)
				require.ErrorIs(t, err, model.ErrGachaIntent)
				stake := 5
				intent := model.GachaActionIntent{RequestID: uuid.NewString(), Action: "arena_join", StakeE: &stake}
				out, err := model.ApplyGachaAction(user.Id, intent, now)
				require.NoError(t, err)
				require.NotNil(t, out.Queue)
				assert.Equal(t, "waiting", out.Queue.Status)
				assert.Nil(t, out.Arena)
				assert.Equal(t, 5*model.GachaArenaQuotaPerE, out.Queue.Quota)
				repeat, err := model.ApplyGachaAction(user.Id, intent, now)
				require.NoError(t, err)
				assert.Equal(t, out, repeat)
				_, err = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "arena_join", StakeE: &stake}, now)
				assert.ErrorIs(t, err, model.ErrGachaArenaWaiting)
				one := 1
				wrongTier, err := model.ApplyGachaAction(other.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "arena_join", StakeE: &one}, now)
				require.NoError(t, err)
				assert.Equal(t, "waiting", wrongTier.Queue.Status)
				_, err = model.ApplyGachaAction(other.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "arena_cancel", QueueID: &wrongTier.Queue.ID}, now)
				require.NoError(t, err)
				// Later squad edits cannot replace the already-consented snapshot.
				team := []string{"c08"}
				_, err = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "squad", Squad: &team}, now)
				require.NoError(t, err)
				paired, err := model.ApplyGachaAction(other.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "arena_join", StakeE: &stake}, now)
				require.NoError(t, err)
				require.NotNil(t, paired.Arena)
				assert.Equal(t, "matched", paired.Queue.Status)
				require.False(t, paired.Arena.Draw)
				delta := -5 * model.GachaArenaQuotaPerE
				if paired.Arena.Win {
					delta = -delta
				}
				assert.Equal(t, delta, paired.Arena.DeltaQuota)
				view, err := model.GetGachaArenaQueue(user.Id, now)
				require.NoError(t, err)
				require.NotNil(t, view.Match)
				assert.NotEqual(t, paired.Arena.Win, view.Match.Win)
				assert.Equal(t, 10*model.GachaArenaQuotaPerE-delta, view.Quota)
				assert.Equal(t, 10*model.GachaArenaQuotaPerE+delta, paired.Queue.Quota)
				assert.Equal(t, paired.Arena.ID, view.Match.ID)
				assert.Equal(t, "c01", view.Match.Report.Allies[0].GirlID)
				assert.Equal(t, "c01", paired.Arena.Report.Waves[0][0].GirlID)
				assert.Equal(t, "ally", paired.Arena.Report.Allies[0].Side)
				assert.True(t, strings.HasPrefix(paired.Arena.Report.Allies[0].UID, "a"))
				assert.Equal(t, team, gachaState(t, user.Id, now).Squad)
				assert.Equal(t, 1, gachaState(t, other.Id, now).Daily.ArenaPlayed)
				assert.Equal(t, 1, gachaState(t, user.Id, now).Daily.ArenaPlayed)
				replay, err := model.GetGachaArenaReplay(user.Id, view.Match.ID)
				require.NoError(t, err)
				assert.Equal(t, view.Match, replay)
				_, err = model.GetGachaArenaReplay(outsider.Id, view.Match.ID)
				assert.ErrorIs(t, err, model.ErrGachaIntent)
				// A cancel arriving after pairing returns the settled result, never a refund.
				late, err := model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "arena_cancel", QueueID: &out.Queue.ID}, now)
				require.NoError(t, err)
				assert.Equal(t, "matched", late.Queue.Status)
				assert.Equal(t, view.Quota, late.Queue.Quota)
				var wallets []model.User
				require.NoError(t, model.DB.Where("id IN ?", []int{user.Id, other.Id, outsider.Id}).Order("id").Find(&wallets).Error)
				require.Len(t, wallets, 3)
				assert.Equal(t, 30*model.GachaArenaQuotaPerE, wallets[0].Quota+wallets[1].Quota+wallets[2].Quota)
				assert.Equal(t, 10*model.GachaArenaQuotaPerE, wallets[2].Quota)
				var journal []model.GachaArenaEntry
				require.NoError(t, model.DB.Where("queue_id = ?", out.Queue.ID).Order("phase").Find(&journal).Error)
				require.Len(t, journal, 2)
				assert.Equal(t, -delta, journal[0].DeltaQuota+journal[1].DeltaQuota)

			})

			t.Run("matchmaking_rejects_large_power_gaps_and_keeps_frozen_power", func(t *testing.T) {
				weak, strong := gachaUser(t), gachaUser(t)
				for _, id := range []int{weak.Id, strong.Id} {
					require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", id).Update("quota", 5*model.GachaArenaQuotaPerE).Error)
				}
				state := gachaState(t, strong.Id, now)
				strongGachaState(state)
				seedGachaState(t, strong.Id, state)
				stake := 3
				first, err := model.ApplyGachaAction(weak.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "arena_join", StakeE: &stake}, now)
				require.NoError(t, err)
				// Upgrade after entering; the queue must keep the entry power and squad.
				state = gachaState(t, weak.Id, now)
				strongGachaState(state)
				seedGachaState(t, weak.Id, state)
				second, err := model.ApplyGachaAction(strong.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "arena_join", StakeE: &stake}, now.Add(119*time.Second))
				require.NoError(t, err)
				assert.Equal(t, "waiting", second.Queue.Status, "waiting almost two minutes must never widen the range")
				assert.Nil(t, second.Arena)
				assert.Greater(t, second.Queue.Power, first.Queue.MaxPower)
				view, err := model.GetGachaArenaQueue(weak.Id, now.Add(119*time.Second))
				require.NoError(t, err)
				assert.Equal(t, first.Queue.Power, view.Power)
				assert.Equal(t, "waiting", view.Status)
				require.NoError(t, model.ExpireGachaArenaSeats(context.Background(), now.Add(4*time.Minute)))
				for _, id := range []int{weak.Id, strong.Id} {
					view, err := model.GetGachaArenaQueue(id, now.Add(4*time.Minute))
					require.NoError(t, err)
					assert.Equal(t, "expired", view.Status)
					assert.Equal(t, 5*model.GachaArenaQuotaPerE, view.Quota)
				}
			})

			t.Run("matchmaking_chooses_closest_compatible_power_before_queue_age", func(t *testing.T) {
				users := []model.User{gachaUser(t), gachaUser(t), gachaUser(t)}
				levels := []int{20, 30, 27}
				stake := 4
				var result *model.GachaActionResult
				for i, user := range users {
					require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", user.Id).Update("quota", 5*model.GachaArenaQuotaPerE).Error)
					state := gachaState(t, user.Id, now)
					state.Squad = []string{"c01"}
					state.Girls["c01"] = &model.GachaOwnedGirl{Star: 1, Level: levels[i]}
					seedGachaState(t, user.Id, state)
					var err error
					result, err = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "arena_join", StakeE: &stake}, now.Add(time.Duration(i)*time.Second))
					require.NoError(t, err)
					if i < 2 {
						assert.Equal(t, "waiting", result.Queue.Status)
					}
				}
				require.NotNil(t, result.Arena)
				assert.Equal(t, users[1].Id, result.Arena.OpponentID, "level 30 is closer to level 27 than the older level 20 ticket")
				view, err := model.GetGachaArenaQueue(users[0].Id, now.Add(3*time.Second))
				require.NoError(t, err)
				assert.Equal(t, "waiting", view.Status)
				require.NoError(t, model.ExpireGachaArenaSeats(context.Background(), now.Add(3*time.Minute)))
			})

			t.Run("queue_refunds_expiry_cancel_restart_and_rejects_forgery", func(t *testing.T) {
				user := gachaUser(t)
				stake := 1
				intent := model.GachaActionIntent{RequestID: uuid.NewString(), Action: "arena_join", StakeE: &stake}
				_, err := model.ApplyGachaAction(user.Id, intent, now)
				require.ErrorIs(t, err, model.ErrGachaArenaFunds)
				for _, n := range []int{-1, 0, 6, 2147483647} {
					bad := intent
					bad.StakeE = &n
					assert.ErrorIs(t, model.ValidateGachaAction(bad), model.ErrGachaIntent)
				}
				for _, extra := range []string{"opponent_id", "quota", "win", "report", "snapshot"} {
					body := map[string]any{"request_id": uuid.NewString(), "action": "arena_join", "stake_e": 1, extra: 1}
					assert.Equal(t, http.StatusBadRequest, gachaRequest(t, user.Id, true, controller.ApplyGachaAction, body).Code)
				}
				require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", user.Id).Update("quota", 2*model.GachaArenaQuotaPerE).Error)
				t.Setenv("GACHA_MATCHMAKING_ENABLED", "false")
				_, err = model.ApplyGachaAction(user.Id, intent, now)
				assert.ErrorIs(t, err, model.ErrGachaArenaPending)
				t.Setenv("GACHA_MATCHMAKING_ENABLED", "true")
				common.BatchUpdateEnabled = true
				_, err = model.ApplyGachaAction(user.Id, intent, now)
				common.BatchUpdateEnabled = false
				assert.ErrorIs(t, err, model.ErrGachaArenaPending)
				concurrent := make([]*model.GachaActionResult, 2)
				failures := make([]error, 2)
				var group sync.WaitGroup
				for i := range 2 {
					group.Go(func() { concurrent[i], failures[i] = model.ApplyGachaAction(user.Id, intent, now) })
				}
				group.Wait()
				for _, failure := range failures {
					require.NoError(t, failure)
				}
				require.Equal(t, concurrent[0], concurrent[1])
				queued := concurrent[0]
				// Real startup preserves active escrow and the idempotency journal.
				initDB()
				initDB()
				view, err := model.GetGachaArenaQueue(user.Id, now)
				require.NoError(t, err)
				assert.Equal(t, queued.Queue, view)
				cancel := model.GachaActionIntent{RequestID: uuid.NewString(), Action: "arena_cancel", QueueID: &view.ID}
				cancelled, err := model.ApplyGachaAction(user.Id, cancel, now)
				require.NoError(t, err)
				assert.Equal(t, 2*model.GachaArenaQuotaPerE, cancelled.Queue.Quota)
				replay, err := model.ApplyGachaAction(user.Id, cancel, now)
				require.NoError(t, err)
				assert.Equal(t, cancelled, replay)
				intent.RequestID = uuid.NewString()
				again, err := model.ApplyGachaAction(user.Id, intent, now)
				require.NoError(t, err)
				cancel.RequestID = uuid.NewString()
				_, err = model.ApplyGachaAction(user.Id, cancel, now)
				assert.ErrorIs(t, err, model.ErrGachaIntent, "old ticket cannot cancel a later queue")
				other := gachaUser(t)
				_, err = model.ApplyGachaAction(other.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "arena_cancel", QueueID: &again.Queue.ID}, now)
				assert.ErrorIs(t, err, model.ErrGachaIntent)
				require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", user.Id).Update("status", common.UserStatusDisabled).Error)
				require.NoError(t, model.ExpireGachaArenaSeats(context.Background(), now.Add(model.GachaArenaQueueTTL)))
				require.NoError(t, model.ExpireGachaArenaSeats(context.Background(), now.Add(model.GachaArenaQueueTTL)))
				var wallet model.User
				require.NoError(t, model.DB.First(&wallet, user.Id).Error)
				assert.Equal(t, 2*model.GachaArenaQuotaPerE, wallet.Quota, "offline/disabled owners still receive one refund")
			})

			t.Run("concurrent_queue_pair_cancel_and_relay_do_not_double_spend", func(t *testing.T) {
				users := []model.User{gachaUser(t), gachaUser(t), gachaUser(t)}
				for _, user := range users {
					require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", user.Id).Update("quota", 2*model.GachaArenaQuotaPerE).Error)
					gachaState(t, user.Id, now)
				}
				stake := 1
				queued, err := model.ApplyGachaAction(users[0].Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "arena_join", StakeE: &stake}, now)
				require.NoError(t, err)
				var wg sync.WaitGroup
				errs := make([]error, 3)
				for i := range 2 {
					wg.Go(func() {
						_, errs[i] = model.ApplyGachaAction(users[i+1].Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "arena_join", StakeE: &stake}, now)
					})
				}
				wg.Go(func() {
					_, errs[2] = model.ApplyGachaAction(users[0].Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "arena_cancel", QueueID: &queued.Queue.ID}, now)
				})
				wg.Wait()
				for _, err := range errs {
					require.NoError(t, err)
				}
				var matches int64
				require.NoError(t, model.DB.Model(&model.GachaArenaDuel{}).Where("left_id IN ?", []int{users[0].Id, users[1].Id, users[2].Id}).Count(&matches).Error)
				assert.EqualValues(t, 1, matches)
				require.NoError(t, model.ExpireGachaArenaSeats(context.Background(), now.Add(model.GachaArenaQueueTTL)))
				var total int64
				require.NoError(t, model.DB.Model(&model.User{}).Where("id IN ?", []int{users[0].Id, users[1].Id, users[2].Id}).Select("SUM(quota)").Scan(&total).Error)
				assert.EqualValues(t, 6*model.GachaArenaQuotaPerE, total)
				poor := gachaUser(t)
				require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", poor.Id).Update("quota", model.GachaArenaQuotaPerE).Error)
				gachaState(t, poor.Id, now)
				var queueErr, relayErr error
				var reserved bool
				wg.Go(func() {
					_, queueErr = model.ApplyGachaAction(poor.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "arena_join", StakeE: &stake}, now)
				})
				wg.Go(func() { reserved, relayErr = model.TryReserveUserQuota(poor.Id, model.GachaArenaQuotaPerE) })
				wg.Wait()
				require.NoError(t, relayErr)
				if queueErr == nil {
					assert.False(t, reserved)
				} else {
					assert.ErrorIs(t, queueErr, model.ErrGachaArenaFunds)
					assert.True(t, reserved)
				}
				var wallet model.User
				require.NoError(t, model.DB.First(&wallet, poor.Id).Error)
				assert.Zero(t, wallet.Quota)
				require.NoError(t, model.ExpireGachaArenaSeats(context.Background(), now.Add(model.GachaArenaQueueTTL)))
			})

			t.Run("matchmaking_draw_refunds_both_sides_and_daily_limit_resets", func(t *testing.T) {
				user, other := gachaUser(t), gachaUser(t)
				for _, id := range []int{user.Id, other.Id} {
					require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", id).Update("quota", model.GachaArenaQuotaPerE).Error)
					state := gachaState(t, id, now)
					state.Squad = []string{"c12"}
					state.Girls["c12"] = &model.GachaOwnedGirl{Star: 5, Level: 60}
					state.Daily.ArenaPlayed = 4
					seedGachaState(t, id, state)
				}
				stake := 1
				_, err := model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "arena_join", StakeE: &stake}, now)
				require.NoError(t, err)
				out, err := model.ApplyGachaAction(other.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "arena_join", StakeE: &stake}, now)
				require.NoError(t, err)
				assert.True(t, out.Arena.Draw)
				assert.Zero(t, out.Arena.DeltaQuota)
				assert.Equal(t, model.GachaArenaQuotaPerE, out.Queue.Quota)
				view, err := model.GetGachaArenaQueue(user.Id, now)
				require.NoError(t, err)
				assert.Equal(t, model.GachaArenaQuotaPerE, view.Quota)
				_, err = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "arena_join", StakeE: &stake}, now)
				assert.ErrorIs(t, err, model.ErrGachaArenaLimit)
				_, err = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "arena_join", StakeE: &stake}, now.Add(24*time.Hour))
				require.NoError(t, err)
				require.NoError(t, model.ExpireGachaArenaSeats(context.Background(), now.Add(24*time.Hour+model.GachaArenaQueueTTL)))
			})

			t.Run("rankings_are_current_stable_public_snapshots_and_exclude_disabled_players", func(t *testing.T) {
				user, other := gachaUser(t), gachaUser(t)
				for _, id := range []int{user.Id, other.Id} {
					state := gachaState(t, id, now)
					strongGachaState(state)
					for _, own := range state.Girls {
						own.Level = 60
					}
					seedGachaState(t, id, state)
					gachaState(t, id, now)
				}
				view, err := model.GetGachaSocial(user.Id)
				require.NoError(t, err)
				require.GreaterOrEqual(t, len(view.PowerBoard), 2)
				left, right := -1, -1
				for i, entry := range view.PowerBoard {
					if entry.UserID == user.Id {
						left = i
					}
					if entry.UserID == other.Id {
						right = i
					}
				}
				require.GreaterOrEqual(t, left, 0)
				assert.Equal(t, left+1, right, "equal power uses a stable ascending user-ID tie break")
				assert.EqualValues(t, left+1, view.PowerRank)
				assert.Len(t, view.PowerBoard[0].Squad, 4)
				assert.Empty(t, view.PowerBoard[0].State)
				public, err := common.Marshal(view)
				require.NoError(t, err)
				for _, secret := range []string{"password", "quota", "wallet_quota", "email", "staminaTs", "auth_version", "tokenGained"} {
					assert.NotContains(t, string(public), `"`+secret+`":`)
				}
				team := []string{}
				out, err := model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "squad", Squad: &team}, now)
				require.NoError(t, err)
				assert.Zero(t, out.Save.Power)
				view, err = model.GetGachaSocial(user.Id)
				require.NoError(t, err)
				assert.Zero(t, view.PowerRank)
				for _, p := range view.PowerBoard {
					assert.NotEqual(t, user.Id, p.UserID)
				}
				require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", other.Id).Update("status", common.UserStatusDisabled).Error)
				view, err = model.GetGachaSocial(user.Id)
				require.NoError(t, err)
				for _, p := range view.PowerBoard {
					assert.NotEqual(t, other.Id, p.UserID)
				}
				_, err = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "arena", OpponentID: &other.Id}, now)
				assert.ErrorIs(t, err, model.ErrGachaIntent)
			})

			t.Run("talents_are_owned_paid_once_and_persist_across_restart", func(t *testing.T) {
				user := gachaUser(t)
				state := gachaState(t, user.Id, now)
				girl, talent := "c01", "power"
				intent := model.GachaActionIntent{RequestID: uuid.NewString(), Action: "talent", GirlID: &girl, ItemID: &talent}
				_, err := model.ApplyGachaAction(user.Id, intent, now)
				assert.ErrorIs(t, err, model.ErrGachaTalentLocked)
				state.Girls[girl].Star = 3
				state.Mats["mat_r"] = 5
				seedGachaState(t, user.Id, state)
				out, err := model.ApplyGachaAction(user.Id, intent, now)
				require.NoError(t, err)
				assert.Equal(t, "power", out.Save.Girls[girl].Talent)
				assert.Equal(t, 100, out.Save.Gold)
				assert.Zero(t, out.Save.Mats["mat_r"])
				repeat, err := model.ApplyGachaAction(user.Id, intent, now)
				require.NoError(t, err)
				assert.Equal(t, out, repeat)
				talent = "guardian"
				intent.RequestID = uuid.NewString()
				_, err = model.ApplyGachaAction(user.Id, intent, now)
				assert.ErrorIs(t, err, model.ErrGachaFunds)
				assert.Equal(t, "power", gachaState(t, user.Id, now).Girls[girl].Talent)
				initDB()
				assert.Equal(t, "power", gachaState(t, user.Id, now).Girls[girl].Talent)
				for _, extra := range []string{"save", "talent_rank", "guild", "damage", "boss_hp"} {
					body := map[string]any{"request_id": uuid.NewString(), "action": "talent", "girl_id": girl, "item_id": talent, extra: 3}
					assert.Equal(t, http.StatusBadRequest, gachaRequest(t, user.Id, true, controller.ApplyGachaAction, body).Code)
				}
			})

			t.Run("expedition_uses_issued_route_and_credits_only_game_rewards_once", func(t *testing.T) {
				user := gachaUser(t)
				state := gachaState(t, user.Id, now)
				strongGachaState(state)
				state.Daily.TokenGained = 98
				seedGachaState(t, user.Id, state)
				_, err := model.StartGachaRun(user.Id, "expedition", uuid.NewString(), now)
				assert.ErrorIs(t, err, model.ErrGachaMode)
				route := "assault"
				_, err = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "expedition_route", ItemID: &route}, now)
				require.NoError(t, err)
				requestID := uuid.NewString()
				run, err := model.StartGachaRun(user.Id, "expedition", requestID, now)
				require.NoError(t, err)
				require.True(t, run.Result.Win)
				require.NotNil(t, run.Scene)
				assert.Contains(t, run.Scene.Name, "第 1 层")
				assert.Equal(t, 107, run.Save.Stamina)
				replay, err := model.StartGachaRun(user.Id, "expedition", requestID, now)
				require.NoError(t, err)
				assert.Equal(t, run, replay)
				route = "fortune"
				_, err = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "expedition_route", ItemID: &route}, now)
				assert.ErrorIs(t, err, model.ErrGachaRunState)
				out, err := model.FinishGachaRun(user.Id, run.ID, false, time.UnixMilli(run.ReadyAt))
				require.NoError(t, err)
				assert.Equal(t, 2, out.Save.Expedition.Floor)
				assert.Equal(t, 1, out.Save.Expedition.Best)
				assert.Equal(t, 100, out.Save.Daily.TokenGained)
				assert.Equal(t, 2, out.Save.Token)
				assert.Greater(t, out.Save.Gold, 600)
				assert.Greater(t, out.Save.Mats["mat_r"], 0)
				assert.Zero(t, out.Quota)
				assert.Equal(t, user.Quota, out.WalletQuota)
				assert.Nil(t, out.Card)
				again, err := model.FinishGachaRun(user.Id, run.ID, false, time.UnixMilli(run.ReadyAt))
				require.NoError(t, err)
				assert.Equal(t, out, again)
				state = gachaState(t, user.Id, now)
				state.Expedition.Floor, state.Expedition.Route, state.Stamina = 10, "fortune", 120
				seedGachaState(t, user.Id, state)
				finalRun, err := model.StartGachaRun(user.Id, "expedition", uuid.NewString(), now)
				require.NoError(t, err)
				require.True(t, finalRun.Result.Win)
				final, err := model.FinishGachaRun(user.Id, finalRun.ID, false, time.UnixMilli(finalRun.ReadyAt))
				require.NoError(t, err)
				assert.Equal(t, 11, final.Save.Expedition.Floor)
				gemTotal := 0
				for _, count := range final.Save.GemInventory {
					gemTotal += count
				}
				assert.Equal(t, 1, gemTotal)
				_, err = model.StartGachaRun(user.Id, "expedition", uuid.NewString(), now)
				assert.ErrorIs(t, err, model.ErrGachaMode)
			})

			t.Run("weekly_modes_keep_old_week_rewards_without_advancing_new_week", func(t *testing.T) {
				clock := time.Date(2026, 11, 22, 15, 59, 59, 0, time.UTC)
				for _, stage := range []string{"expedition", "weekly_boss"} {
					user := gachaUser(t)
					state := gachaState(t, user.Id, clock)
					strongGachaState(state)
					state.Expedition.Route = "bastion"
					seedGachaState(t, user.Id, state)
					run, err := model.StartGachaRun(user.Id, stage, uuid.NewString(), clock)
					require.NoError(t, err)
					require.True(t, run.Result.Win)
					out, err := model.FinishGachaRun(user.Id, run.ID, false, time.UnixMilli(run.ReadyAt))
					require.NoError(t, err)
					assert.True(t, out.Win)
					assert.Equal(t, "2026-W48", out.Save.Expedition.Week)
					assert.Equal(t, 1, out.Save.Expedition.Floor)
					assert.False(t, out.Save.WeeklyBoss.Cleared)
					assert.Zero(t, out.Save.WeeklyBoss.Attempts)
					assert.Zero(t, out.Quota)
					assert.Greater(t, out.Save.Gold, state.Gold)
				}
			})

			t.Run("weekly_boss_attempts_cannot_replay_or_consume_unused_double_cards", func(t *testing.T) {
				user := gachaUser(t)
				clock := now
				for attempt := 1; attempt <= 3; attempt++ {
					state := gachaState(t, user.Id, clock)
					state.Stamina, state.Multi, state.MultiOn = 120, 2, true
					seedGachaState(t, user.Id, state)
					run, err := model.StartGachaRun(user.Id, "weekly_boss", uuid.NewString(), clock)
					require.NoError(t, err)
					assert.Equal(t, attempt, run.Save.WeeklyBoss.Attempts)
					assert.Equal(t, 2, run.Save.Multi)
					out, err := model.FinishGachaRun(user.Id, run.ID, true, clock)
					require.NoError(t, err)
					assert.False(t, out.Win)
					assert.Zero(t, out.Save.WeeklyBoss.BestDamage)
					assert.Empty(t, out.Drops)
					clock = clock.Add(time.Minute)
				}
				_, err := model.StartGachaRun(user.Id, "weekly_boss", uuid.NewString(), clock)
				assert.ErrorIs(t, err, model.ErrGachaModeLimit)
				assert.Zero(t, gachaState(t, user.Id, now.Add(7*24*time.Hour)).WeeklyBoss.Attempts)
			})

			t.Run("guild_capacity_and_daily_rewards_are_serialized_and_survive_rejoining", func(t *testing.T) {
				leader := gachaUser(t)
				name := "星穹旅团"
				created, err := model.ApplyGachaAction(leader.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "guild_create", ItemID: &name}, now)
				require.NoError(t, err)
				require.NotNil(t, created.Guild)
				code := created.Guild.Code
				users := make([]model.User, 10)
				for i := range users {
					users[i] = gachaUser(t)
					gachaState(t, users[i].Id, now)
					if i < 8 {
						_, err := model.ApplyGachaAction(users[i].Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "guild_join", ItemID: &code}, now)
						require.NoError(t, err)
					}
				}
				var wg sync.WaitGroup
				failures := make([]error, 2)
				for i := range 2 {
					wg.Go(func() {
						_, failures[i] = model.ApplyGachaAction(users[i+8].Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "guild_join", ItemID: &code}, now)
					})
				}
				wg.Wait()
				joined := 0
				for _, failure := range failures {
					if failure == nil {
						joined++
					} else {
						assert.ErrorIs(t, failure, model.ErrGachaGuildLimit)
					}
				}
				assert.Equal(t, 1, joined)
				profile, err := model.GetGachaProfile(leader.Id, now)
				require.NoError(t, err)
				assert.Len(t, profile.Guild.Members, 10)
				member := users[0]
				intent := model.GachaActionIntent{RequestID: uuid.NewString(), Action: "guild_attack"}
				attack, err := model.ApplyGachaAction(member.Id, intent, now)
				require.NoError(t, err)
				assert.Equal(t, 680, attack.Save.Gold)
				assert.Equal(t, 110, attack.Save.Stamina)
				assert.Equal(t, 24000, attack.Guild.BossHP+attack.Guild.TotalDamage)
				replay, err := model.ApplyGachaAction(member.Id, intent, now)
				require.NoError(t, err)
				assert.Equal(t, attack, replay)
				_, err = model.ApplyGachaAction(member.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "guild_leave"}, now)
				require.NoError(t, err)
				_, err = model.ApplyGachaAction(member.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "guild_join", ItemID: &code}, now)
				require.NoError(t, err)
				_, err = model.ApplyGachaAction(member.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "guild_attack"}, now)
				assert.ErrorIs(t, err, model.ErrGachaGuildLimit)
				assert.Equal(t, 680, gachaState(t, member.Id, now).Gold)
				var wallet model.User
				require.NoError(t, model.DB.First(&wallet, member.Id).Error)
				assert.Equal(t, member.Quota, wallet.Quota)
				initDB()
				profile, err = model.GetGachaProfile(leader.Id, now)
				require.NoError(t, err)
				assert.Len(t, profile.Guild.Members, 10)
			})

			t.Run("guild_parallel_attacks_debit_one_daily_attempt", func(t *testing.T) {
				leader, member := gachaUser(t), gachaUser(t)
				name := "并发旅团"
				created, err := model.ApplyGachaAction(leader.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "guild_create", ItemID: &name}, now)
				require.NoError(t, err)
				code := created.Guild.Code
				_, err = model.ApplyGachaAction(member.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "guild_join", ItemID: &code}, now)
				require.NoError(t, err)
				var wg sync.WaitGroup
				failures := make([]error, 2)
				for i := range 2 {
					wg.Go(func() {
						_, failures[i] = model.ApplyGachaAction(member.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "guild_attack"}, now)
					})
				}
				wg.Wait()
				accepted := 0
				for _, err := range failures {
					if err == nil {
						accepted++
					} else {
						assert.ErrorIs(t, err, model.ErrGachaGuildLimit)
					}
				}
				assert.Equal(t, 1, accepted)
				state := gachaState(t, member.Id, now)
				assert.Equal(t, 110, state.Stamina)
				assert.Equal(t, 680, state.Gold)
				assert.True(t, state.Daily.GuildAttacked)
				view, err := model.GetGachaProfile(member.Id, now)
				require.NoError(t, err)
				assert.Equal(t, 24000, view.Guild.BossHP+view.Guild.MyDamage)
			})

			t.Run("bond_power_refresh_preserves_released_gameplay_state", func(t *testing.T) {
				user := gachaUser(t)
				gachaState(t, user.Id, now)
				var player model.GachaPlayer
				require.NoError(t, model.DB.First(&player, "user_id = ?", user.Id).Error)
				require.NoError(t, model.DB.Model(&player).UpdateColumns(map[string]any{"power": 1, "power_version": 1}).Error)
				require.NoError(t, model.InitializeGachaPower())
				require.NoError(t, model.InitializeGachaPower())
				var after model.GachaPlayer
				require.NoError(t, model.DB.First(&after, "user_id = ?", user.Id).Error)
				assert.Equal(t, player.State, after.State)
				assert.Equal(t, player.Revision, after.Revision)
				assert.Greater(t, after.Power, 1)
				assert.Equal(t, 4, after.PowerVersion)
			})

			t.Run("guild_kill_rewards_require_participation_and_cannot_be_claimed_twice", func(t *testing.T) {
				leader, member, spectator := gachaUser(t), gachaUser(t), gachaUser(t)
				name := "晨星公会"
				created, err := model.ApplyGachaAction(leader.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "guild_create", ItemID: &name}, now)
				require.NoError(t, err)
				code := created.Guild.Code
				for _, user := range []model.User{member, spectator} {
					_, err = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "guild_join", ItemID: &code}, now)
					require.NoError(t, err)
				}
				state := gachaState(t, leader.Id, now)
				state.Guild.BossHP, state.Guild.TotalDamage = 1, 23999
				seedGachaState(t, leader.Id, state)
				attack, err := model.ApplyGachaAction(member.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "guild_attack"}, now)
				require.NoError(t, err)
				assert.Zero(t, attack.Guild.BossHP)
				assert.Equal(t, 1, attack.Guild.MyDamage)
				assert.True(t, attack.Guild.CanClaim)
				_, err = model.ApplyGachaAction(spectator.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "guild_claim"}, now)
				assert.ErrorIs(t, err, model.ErrGachaGuildLimit)
				claim := model.GachaActionIntent{RequestID: uuid.NewString(), Action: "guild_claim"}
				out, err := model.ApplyGachaAction(member.Id, claim, now)
				require.NoError(t, err)
				assert.Equal(t, 930, out.Save.Gold)
				assert.Equal(t, 3, out.Save.Mats["mat_sr"])
				assert.True(t, out.Guild.RewardClaimed)
				again, err := model.ApplyGachaAction(member.Id, claim, now)
				require.NoError(t, err)
				assert.Equal(t, out, again)
				claim.RequestID = uuid.NewString()
				_, err = model.ApplyGachaAction(member.Id, claim, now)
				assert.ErrorIs(t, err, model.ErrGachaGuildLimit)
				fresh, err := model.GetGachaProfile(member.Id, now.Add(7*24*time.Hour))
				require.NoError(t, err)
				assert.Equal(t, 24000, fresh.Guild.BossHP)
				assert.Zero(t, fresh.Guild.MyDamage)
				assert.False(t, fresh.Guild.CanClaim)
			})

			t.Run("sigils_are_server_rolled_owned_and_replay_safe", func(t *testing.T) {
				user, other := gachaUser(t), gachaUser(t)
				state := gachaState(t, user.Id, now)
				state.Gold, state.Mats["essence_ember"], state.Mats["sigil_dust"] = 800, 40, 15
				seedGachaState(t, user.Id, state)
				for _, extra := range []string{"quality", "affixes", "sigil", "save", "relic_runs"} {
					body := map[string]any{"request_id": uuid.NewString(), "action": "sigil_craft", "item_id": "ember", extra: 3}
					assert.Equal(t, http.StatusBadRequest, gachaRequest(t, user.Id, true, controller.ApplyGachaAction, body).Code)
				}
				family := "ember"
				intent := model.GachaActionIntent{RequestID: uuid.NewString(), Action: "sigil_craft", ItemID: &family}
				made, err := model.ApplyGachaAction(user.Id, intent, now)
				require.NoError(t, err)
				require.NotNil(t, made.Sigil)
				assert.Equal(t, 3, made.Sigil.Quality)
				assert.Equal(t, "ember", made.Sigil.Family)
				assert.Len(t, made.Sigil.Affixes, 2)
				assert.Zero(t, made.Save.Gold)
				assert.Zero(t, made.Save.Mats["essence_ember"])
				assert.Zero(t, made.Save.Mats["sigil_dust"])
				again, err := model.ApplyGachaAction(user.Id, intent, now)
				require.NoError(t, err)
				assert.Equal(t, made, again)
				girl, sigilID := "c01", made.Sigil.ID
				equip := model.GachaActionIntent{RequestID: uuid.NewString(), Action: "sigil_equip", GirlID: &girl, ItemID: &sigilID}
				_, err = model.ApplyGachaAction(other.Id, equip, now)
				assert.ErrorIs(t, err, model.ErrGachaSigil)
				out, err := model.ApplyGachaAction(user.Id, equip, now)
				require.NoError(t, err)
				assert.Equal(t, sigilID, out.Save.Girls[girl].Sigil)
				_, err = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "sigil_salvage", ItemID: &sigilID}, now)
				assert.ErrorIs(t, err, model.ErrGachaSigil, "equipped items cannot be destroyed")
				initDB()
				assert.Equal(t, sigilID, gachaState(t, user.Id, now).Girls[girl].Sigil)
				_, err = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "sigil_remove", GirlID: &girl}, now)
				require.NoError(t, err)
				salvage := model.GachaActionIntent{RequestID: uuid.NewString(), Action: "sigil_salvage", ItemID: &sigilID}
				removed, err := model.ApplyGachaAction(user.Id, salvage, now)
				require.NoError(t, err)
				assert.Empty(t, removed.Save.Sigils)
				assert.Equal(t, 8, removed.Save.Mats["sigil_dust"])
				replayed, err := model.ApplyGachaAction(user.Id, salvage, now)
				require.NoError(t, err)
				assert.Equal(t, removed, replayed)
				var wallet model.User
				require.NoError(t, model.DB.First(&wallet, user.Id).Error)
				assert.Equal(t, user.Quota, wallet.Quota, "sigil actions never debit or credit E")
			})

			t.Run("parallel_sigil_crafts_and_equips_cannot_multiply_resources", func(t *testing.T) {
				user := gachaUser(t)
				state := gachaState(t, user.Id, now)
				state.Gold, state.Mats["essence_tide"], state.Mats["sigil_dust"] = 800, 40, 15
				seedGachaState(t, user.Id, state)
				family := "tide"
				var successes atomic.Int32
				var wg sync.WaitGroup
				for range 2 {
					wg.Go(func() {
						_, err := model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "sigil_craft", ItemID: &family}, now)
						if err == nil {
							successes.Add(1)
						} else {
							assert.ErrorIs(t, err, model.ErrGachaFunds)
						}
					})
				}
				wg.Wait()
				assert.Equal(t, int32(1), successes.Load())
				state = gachaState(t, user.Id, now)
				require.Len(t, state.Sigils, 1)
				assert.Zero(t, state.Gold)
				var sigilID string
				for id := range state.Sigils {
					sigilID = id
				}
				successes.Store(0)
				for _, girl := range []string{"c01", "c06"} {
					wg.Go(func() {
						_, err := model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "sigil_equip", GirlID: &girl, ItemID: &sigilID}, now)
						if err == nil {
							successes.Add(1)
						} else {
							assert.ErrorIs(t, err, model.ErrGachaSigil)
						}
					})
				}
				wg.Wait()
				assert.Equal(t, int32(1), successes.Load())
				state = gachaState(t, user.Id, now)
				count := 0
				for _, own := range state.Girls {
					if own.Sigil == sigilID {
						count++
					}
				}
				assert.Equal(t, 1, count)
			})

			t.Run("sigil_receipt_failure_rolls_back_craft_and_bulk_salvage_preserves_equipment", func(t *testing.T) {
				user := gachaUser(t)
				state := gachaState(t, user.Id, now)
				state.Gold, state.Mats["essence_grove"], state.Mats["sigil_dust"] = 800, 40, 15
				seedGachaState(t, user.Id, state)
				callback, db := "gacha-test-sigil-receipt", model.DB
				require.NoError(t, db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
					if tx.Statement.Schema != nil && tx.Statement.Schema.Table == "gacha_actions" {
						tx.AddError(errors.New("isolated receipt failure"))
					}
				}))
				t.Cleanup(func() { _ = db.Callback().Create().Remove(callback) })
				family := "grove"
				_, err := model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "sigil_craft", ItemID: &family}, now)
				require.Error(t, err)
				require.NoError(t, db.Callback().Create().Remove(callback))
				state = gachaState(t, user.Id, now)
				assert.Empty(t, state.Sigils)
				assert.Equal(t, 800, state.Gold)
				assert.Equal(t, 40, state.Mats["essence_grove"])
				assert.Equal(t, 15, state.Mats["sigil_dust"])
				free, equipped, rare := gachaTestSigil("grove", 1), gachaTestSigil("tide", 1), gachaTestSigil("astral", 2)
				state.Sigils = map[string]*model.GachaSigil{free.ID: free, equipped.ID: equipped, rare.ID: rare}
				state.Girls["c01"].Sigil = equipped.ID
				seedGachaState(t, user.Id, state)
				out, err := model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "sigil_salvage_low"}, now)
				require.NoError(t, err)
				assert.NotContains(t, out.Save.Sigils, free.ID)
				assert.Contains(t, out.Save.Sigils, equipped.ID)
				assert.Contains(t, out.Save.Sigils, rare.ID)
				assert.Equal(t, 17, out.Save.Mats["sigil_dust"])
			})

			t.Run("relic_loot_is_family_specific_daily_limited_and_never_pays_E", func(t *testing.T) {
				user := gachaUser(t)
				state := gachaState(t, user.Id, now)
				strongGachaState(state)
				state.Cleared["s6"] = true
				state.Daily.RelicRuns = 29
				seedGachaState(t, user.Id, state)
				_, err := model.StartGachaRun(user.Id, "n1", uuid.NewString(), now)
				assert.ErrorIs(t, err, model.ErrGachaLocked)
				run, err := model.StartGachaRun(user.Id, "r1", uuid.NewString(), now)
				require.NoError(t, err)
				require.True(t, run.Result.Win)
				assert.Equal(t, 30, run.Save.Daily.RelicRuns)
				assert.Equal(t, state.Stamina-12, run.Save.Stamina)
				clock := time.UnixMilli(run.ReadyAt)
				out, err := model.FinishGachaRun(user.Id, run.ID, false, clock)
				require.NoError(t, err)
				assert.Zero(t, out.Quota)
				assert.Nil(t, out.Card)
				assert.GreaterOrEqual(t, out.Save.Mats["essence_ember"], 2)
				assert.LessOrEqual(t, out.Save.Mats["essence_ember"], 4)
				assert.Zero(t, out.Save.Mats["essence_tide"])
				for _, sigil := range out.Save.Sigils {
					assert.Equal(t, "ember", sigil.Family)
				}
				again, err := model.FinishGachaRun(user.Id, run.ID, false, clock)
				require.NoError(t, err)
				assert.Equal(t, out, again)
				next, err := model.StartGachaRun(user.Id, "r1", uuid.NewString(), clock)
				require.NoError(t, err, "ordinary rewards remain available beyond relic supplies")
				more, err := model.FinishGachaRun(user.Id, next.ID, false, time.UnixMilli(next.ReadyAt))
				require.NoError(t, err)
				require.True(t, more.Win)
				assert.Equal(t, out.Save.Mats["essence_ember"], more.Save.Mats["essence_ember"])
				assert.Len(t, more.Save.Sigils, len(out.Save.Sigils))
				assert.Greater(t, more.Save.Gold, out.Save.Gold)
				var wallet model.User
				require.NoError(t, model.DB.First(&wallet, user.Id).Error)
				assert.Equal(t, user.Quota, wallet.Quota)
			})

			t.Run("rapid_run_rest_survives_restart_midnight_and_does_not_charge_denied_starts", func(t *testing.T) {
				clock := time.Date(2026, 11, 23, 15, 59, 30, 0, time.UTC)
				user := gachaUser(t)
				state := gachaState(t, user.Id, clock)
				strongGachaState(state)
				state.Activity = model.GachaRunActivity{WindowStartedAt: clock.Add(-time.Minute).UnixMilli(), Starts: 11}
				seedGachaState(t, user.Id, state)
				requestID := uuid.NewString()
				run, err := model.StartGachaRun(user.Id, "s1", requestID, clock)
				require.NoError(t, err)
				assert.Equal(t, 12, run.Save.Activity.Starts)
				assert.Equal(t, run.ReadyAt+120_000, run.Save.Activity.RestUntil)
				replay, err := model.StartGachaRun(user.Id, "s1", requestID, clock)
				require.NoError(t, err, "idempotent retries are allowed during rest")
				assert.Equal(t, run, replay)
				abandoned, err := model.FinishGachaRun(user.Id, run.ID, true, clock)
				require.NoError(t, err)
				assert.Zero(t, abandoned.Quota)
				initDB()
				afterMidnight := clock.Add(time.Minute)
				_, err = model.StartGachaRun(user.Id, "s1", uuid.NewString(), afterMidnight)
				assert.ErrorIs(t, err, model.ErrGachaRest)
				var saved model.GachaPlayer
				require.NoError(t, model.DB.First(&saved, "user_id = ?", user.Id).Error)
				var stored model.GachaGameState
				require.NoError(t, common.Unmarshal([]byte(saved.State), &stored))
				assert.Equal(t, abandoned.Save.Stamina, stored.Stamina)
				assert.Equal(t, abandoned.Save.Activity, stored.Activity)
				fresh, err := model.StartGachaRun(user.Id, "s1", uuid.NewString(), time.UnixMilli(stored.Activity.RestUntil+1))
				require.NoError(t, err)
				assert.Equal(t, 1, fresh.Save.Activity.Starts)
				assert.Zero(t, fresh.Save.Activity.RestUntil)
				profile, err := model.GetGachaProfile(user.Id, time.UnixMilli(stored.Activity.RestUntil+1))
				require.NoError(t, err)
				assert.Nil(t, profile.Today.RunLimit)
			})

			t.Run("parallel_twelfth_start_and_HTTP_rest_reject_forged_counters", func(t *testing.T) {
				user := gachaUser(t)
				state := gachaState(t, user.Id, now)
				strongGachaState(state)
				state.Activity = model.GachaRunActivity{WindowStartedAt: now.UnixMilli(), Starts: 11}
				seedGachaState(t, user.Id, state)
				var success atomic.Int32
				var wg sync.WaitGroup
				for range 4 {
					wg.Go(func() {
						_, err := model.StartGachaRun(user.Id, "s1", uuid.NewString(), now)
						if err == nil {
							success.Add(1)
						} else {
							assert.ErrorIs(t, err, model.ErrGachaRest)
						}
					})
				}
				wg.Wait()
				assert.Equal(t, int32(1), success.Load())
				state = gachaState(t, user.Id, now)
				assert.Equal(t, 118, state.Stamina)
				state.Activity.RestUntil = time.Now().Add(time.Minute).UnixMilli()
				seedGachaState(t, user.Id, state)
				response := gachaRequest(t, user.Id, true, controller.StartGachaRun, map[string]any{"stage": "s1", "request_id": uuid.NewString()})
				assert.Equal(t, http.StatusTooManyRequests, response.Code)
				assert.NotEmpty(t, response.Header().Get("Retry-After"))
				assert.Contains(t, response.Body.String(), "GACHA_REST")
				for _, field := range []string{"activity", "rest_until", "relic_eligible", "rules_version"} {
					body := map[string]any{"stage": "s1", "request_id": uuid.NewString(), field: 0}
					assert.Equal(t, http.StatusBadRequest, gachaRequest(t, user.Id, true, controller.StartGachaRun, body).Code)
				}
			})

			t.Run("sigil_power_backfill_preserves_game_state_and_revision", func(t *testing.T) {
				user := gachaUser(t)
				state := gachaState(t, user.Id, now)
				strongGachaState(state)
				sigil := gachaTestSigil("astral", 3)
				state.Sigils = map[string]*model.GachaSigil{sigil.ID: sigil}
				state.Girls[state.Squad[0]].Sigil = sigil.ID
				seedGachaState(t, user.Id, state)
				require.NoError(t, model.DB.Model(&model.GachaPlayer{}).Where("user_id = ?", user.Id).Updates(map[string]any{"power": 1, "power_version": 2}).Error)
				var before, after model.GachaPlayer
				require.NoError(t, model.DB.First(&before, "user_id = ?", user.Id).Error)
				expected, err := model.GachaSquadPower(state)
				require.NoError(t, err)
				require.NoError(t, model.InitializeGachaPower())
				require.NoError(t, model.DB.First(&after, "user_id = ?", user.Id).Error)
				assert.Equal(t, before.State, after.State)
				assert.Equal(t, before.Revision, after.Revision)
				assert.Equal(t, expected, after.Power)
				assert.Equal(t, 4, after.PowerVersion)
			})

			runGachaProgressionMatrix(t, now)

			t.Run("disabled_or_missing_users_cannot_act", func(t *testing.T) {
				user := gachaUser(t)
				require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", user.Id).Update("status", common.UserStatusDisabled).Error)
				count := 1
				for _, id := range []int{0, user.Id, 999999999} {
					_, err := model.ApplyGachaAction(id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "gacha", Count: &count}, now)
					assert.Error(t, err)
				}
			})
		})
	}
}

func runGachaProgressionMatrix(t *testing.T, now time.Time) {
	t.Helper()
	t.Run("progression_legacy_resources_are_preserved_and_no_past_budgets_are_minted", func(t *testing.T) {
		user := gachaUser(t)
		state := gachaState(t, user.Id, now)
		strongGachaState(state)
		state.Gold, state.Token, state.Mats["shard_dust"], state.Multi = 1_276_401, 1234, 4321, 98
		for _, own := range state.Girls {
			if own.Gems == nil {
				own.Gems = make([]string, 3)
			}
		}
		state.Cleared["s12"] = true
		state.Progression = nil
		seedGachaState(t, user.Id, state)
		out := gachaState(t, user.Id, now)
		assert.Equal(t, state.Girls, out.Girls)
		assert.Equal(t, state.Mats, out.Mats)
		assert.Equal(t, state.Gold, out.Gold)
		assert.Equal(t, state.Token, out.Token)
		assert.Equal(t, state.Multi, out.Multi)
		require.Len(t, out.Progression.Days, 1)
		require.Len(t, out.Progression.Weeks, 1)
		assert.True(t, out.Progression.Legacy)
		assert.Zero(t, out.Progression.Marks)
		assert.Zero(t, out.Progression.Permits)
		assert.Equal(t, out.Progression, gachaState(t, user.Id, now).Progression)
		var wallet model.User
		require.NoError(t, model.DB.First(&wallet, user.Id).Error)
		assert.Equal(t, user.Quota, wallet.Quota)
	})

	t.Run("progression_permits_are_claimed_once_concurrently_and_gold_cannot_skip_experience", func(t *testing.T) {
		user := gachaUser(t)
		state := gachaState(t, user.Id, now)
		state.Gold = 1_000_000
		seedGachaState(t, user.Id, state)
		count, girl, day := 1, "c08", state.Progression.Days[0].Date
		_, err := model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "gacha", Count: &count}, now)
		assert.ErrorIs(t, err, model.ErrGachaPermits)
		_, err = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "level", GirlID: &girl}, now)
		assert.ErrorIs(t, err, model.ErrGachaExperience)
		assert.Equal(t, state.Gold, gachaState(t, user.Id, now).Gold)
		intents := []model.GachaActionIntent{{RequestID: uuid.NewString(), Action: "daily_permits", ItemID: &day}, {RequestID: uuid.NewString(), Action: "daily_permits", ItemID: &day}}
		results, errs := make([]*model.GachaActionResult, 2), make([]error, 2)
		start := make(chan struct{})
		var wg sync.WaitGroup
		for i := range 2 {
			wg.Go(func() { <-start; results[i], errs[i] = model.ApplyGachaAction(user.Id, intents[i], now) })
		}
		close(start)
		wg.Wait()
		success := -1
		for i, err := range errs {
			if err == nil {
				require.Equal(t, -1, success)
				success = i
			} else {
				assert.ErrorIs(t, err, model.ErrGachaProgressionLimit)
			}
		}
		require.NotEqual(t, -1, success)
		retry, err := model.ApplyGachaAction(user.Id, intents[success], now)
		require.NoError(t, err)
		assert.Equal(t, results[success], retry)
		assert.Equal(t, 5, gachaState(t, user.Id, now).Progression.Permits)
		out, err := model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "gacha", Count: &count}, now)
		require.NoError(t, err)
		assert.Equal(t, 4, out.Save.Progression.Permits)
		assert.Equal(t, state.Gold-150, out.Save.Gold)
	})

	t.Run("progression_new_rarity_dust_cannot_feed_SSR_but_legacy_dust_still_works", func(t *testing.T) {
		user := gachaUser(t)
		state := gachaState(t, user.Id, now)
		state.Girls["c01"].Star, state.Girls["c01"].Shards = 5, 500
		state.Girls["c04"] = &model.GachaOwnedGirl{Level: 40, Star: 3, Gems: make([]string, 3)}
		state.Mats["mat_ssr"] = 100
		seedGachaState(t, user.Id, state)
		girl, count := "c01", 500
		out, err := model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "shard_decompose", GirlID: &girl, Count: &count}, now)
		require.NoError(t, err)
		assert.Equal(t, 500, out.Save.Mats["shard_dust_r"])
		assert.Zero(t, out.Save.Mats["shard_dust"])
		girl = "c04"
		_, err = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "star", GirlID: &girl}, now)
		assert.ErrorIs(t, err, model.ErrGachaFunds)
		state = gachaState(t, user.Id, now)
		state.Mats["shard_dust"] = 120
		seedGachaState(t, user.Id, state)
		out, err = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "star", GirlID: &girl}, now)
		require.NoError(t, err)
		assert.Equal(t, 4, out.Save.Girls[girl].Star)
		assert.Zero(t, out.Save.Mats["shard_dust"])
		assert.Equal(t, 500, out.Save.Mats["shard_dust_r"])
	})

	t.Run("progression_training_uses_real_experience_failure_retry_and_seven_day_bank", func(t *testing.T) {
		user := gachaUser(t)
		state := gachaState(t, user.Id, now)
		state.Squad = []string{"c04"}
		state.Girls["c04"] = &model.GachaOwnedGirl{Level: 20, Star: 3, Gems: make([]string, 3)}
		seedGachaState(t, user.Id, state)
		run, err := model.StartGachaRun(user.Id, "training", uuid.NewString(), now)
		require.NoError(t, err)
		assert.True(t, run.Result.Win)
		out, err := model.FinishGachaRun(user.Id, run.ID, true, now)
		require.NoError(t, err)
		assert.Equal(t, 3, out.Save.Progression.Days[0].TrainingLeft)
		run, err = model.StartGachaRun(user.Id, "training", uuid.NewString(), now.Add(time.Second))
		require.NoError(t, err)
		out, err = model.FinishGachaRun(user.Id, run.ID, false, time.UnixMilli(run.ReadyAt+1))
		require.NoError(t, err)
		assert.Equal(t, 21, out.Save.Girls["c04"].Level)
		assert.Zero(t, out.Save.Girls["c04"].Exp)
		assert.Equal(t, 2, out.Save.Progression.Days[0].TrainingLeft)
		assert.Equal(t, state.Gold, out.Save.Gold)
		assert.Equal(t, state.Stamina, out.Save.Stamina)
		assert.Zero(t, out.Quota)
		assert.Nil(t, out.Card)
		repeat, err := model.FinishGachaRun(user.Id, run.ID, false, time.UnixMilli(run.ReadyAt+1))
		require.NoError(t, err)
		assert.Equal(t, out, repeat)
		state = gachaState(t, user.Id, now.AddDate(0, 0, 20))
		require.Len(t, state.Progression.Days, 7)
		require.Len(t, state.Progression.Weeks, 2)
		remaining := 0
		for _, day := range state.Progression.Days {
			remaining += day.TrainingLeft
		}
		assert.Equal(t, 21, remaining)
		assert.Zero(t, state.Progression.Marks)
		assert.Equal(t, state.Progression, gachaState(t, user.Id, now.AddDate(0, 0, 20)).Progression)
	})

	t.Run("progression_Sunday_Monday_rewards_cannot_fully_awaken_even_one_character", func(t *testing.T) {
		when := time.Date(2026, 11, 22, 12, 0, 0, 0, time.FixedZone("CST", 8*3600))
		user := gachaUser(t)
		state := gachaState(t, user.Id, when)
		state.Squad = []string{"c04"}
		state.Girls["c04"] = &model.GachaOwnedGirl{Level: 60, Star: 5, Gems: make([]string, 3)}
		seedGachaState(t, user.Id, state)
		for offset := range 2 {
			clock := when.AddDate(0, 0, offset)
			for range 3 {
				run, err := model.StartGachaRun(user.Id, "training", uuid.NewString(), clock)
				require.NoError(t, err)
				_, err = model.FinishGachaRun(user.Id, run.ID, false, time.UnixMilli(run.ReadyAt+1))
				require.NoError(t, err)
				clock = time.UnixMilli(run.ReadyAt + 1).Add(time.Minute)
			}
			for _, girl := range []string{"c04", "c07"} {
				_, err := model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "wish", GirlID: &girl}, clock)
				require.NoError(t, err)
			}
			state = gachaState(t, user.Id, clock)
			day, week, girl := state.Progression.Days[len(state.Progression.Days)-1].Date, state.Progression.Weeks[len(state.Progression.Weeks)-1].Week, "c04"
			_, err := model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "contract_claim", ItemID: &day}, clock)
			require.NoError(t, err)
			_, err = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "weekly_claim", ItemID: &week, GirlID: &girl}, clock)
			require.NoError(t, err)
		}
		state = gachaState(t, user.Id, when.AddDate(0, 0, 1).Add(time.Hour))
		assert.Equal(t, 320, state.Progression.Marks)
		assert.Less(t, state.Progression.Marks, 400)
	})

	t.Run("progression_personal_trial_and_marks_unlock_one_mutually_exclusive_skill_branch", func(t *testing.T) {
		user := gachaUser(t)
		state := gachaState(t, user.Id, now)
		strongGachaState(state)
		state.Gold, state.Progression.Marks = 2000, 400
		state.Squad = []string{"c07", "c09", "c03"}
		state.Girls["c04"] = &model.GachaOwnedGirl{Level: 40, Star: 3, Gems: make([]string, 3)}
		seedGachaState(t, user.Id, state)
		girl, branch := "c04", "focus"
		_, err := model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "awakening", GirlID: &girl, Choice: &branch}, now)
		assert.ErrorIs(t, err, model.ErrGachaAwakening)
		run, err := model.StartGachaRun(user.Id, "trial_c04_1", uuid.NewString(), now)
		require.NoError(t, err)
		require.True(t, run.Result.Win)
		assert.Equal(t, "c04", run.Result.Allies[3].GirlID)
		finished, err := model.FinishGachaRun(user.Id, run.ID, false, time.UnixMilli(run.ReadyAt+1))
		require.NoError(t, err)
		assert.Equal(t, 1, finished.Save.Progression.TrialWins[girl])
		assert.Zero(t, finished.Quota)
		intent := model.GachaActionIntent{RequestID: uuid.NewString(), Action: "awakening", GirlID: &girl, Choice: &branch}
		out, err := model.ApplyGachaAction(user.Id, intent, time.UnixMilli(run.ReadyAt+1))
		require.NoError(t, err)
		assert.Equal(t, 1, out.Save.Girls[girl].Awakening)
		assert.Equal(t, "focus", out.Save.Girls[girl].Branch)
		assert.Equal(t, 320, out.Save.Progression.Marks)
		retry, err := model.ApplyGachaAction(user.Id, intent, time.UnixMilli(run.ReadyAt+1))
		require.NoError(t, err)
		assert.Equal(t, out, retry)
		branch = "sweep"
		out, err = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "awakening_branch", GirlID: &girl, Choice: &branch}, time.UnixMilli(run.ReadyAt+1))
		require.NoError(t, err)
		assert.Equal(t, 1750, out.Save.Gold)
		assert.Equal(t, 320, out.Save.Progression.Marks)
		assert.Equal(t, "sweep", out.Save.Girls[girl].Branch)
		assert.Positive(t, out.Save.Progression.Milestones["first_awakening"])
	})

	t.Run("progression_two_squads_are_distinct_and_run_uses_the_locked_eight_characters", func(t *testing.T) {
		user := gachaUser(t)
		state := gachaState(t, user.Id, now)
		team := []string{"c04", "c07", "c09", "c28", "c11", "c05", "c13", "c10"}
		for _, id := range team {
			branch := model.GachaAwakeningBranches(id)[0].ID
			state.Girls[id] = &model.GachaOwnedGirl{Level: 60, Star: 5, Awakening: 3, Branch: branch, Gems: make([]string, 3)}
		}
		state.Cleared["s6"] = true
		seedGachaState(t, user.Id, state)
		duplicate := slices.Clone(team)
		duplicate[7] = duplicate[0]
		_, err := model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "teams_save", Squad: &duplicate}, now)
		assert.ErrorIs(t, err, model.ErrGachaDualTeam)
		_, err = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "teams_save", Squad: &team}, now)
		require.NoError(t, err)
		run, err := model.StartGachaRun(user.Id, "dual_trial", uuid.NewString(), now)
		require.NoError(t, err)
		require.True(t, run.Result.Win)
		changed := []string{"c11", "c05", "c13", "c10", "c04", "c07", "c09", "c28"}
		_, err = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "teams_save", Squad: &changed}, now)
		require.NoError(t, err)
		out, err := model.FinishGachaRun(user.Id, run.ID, false, time.UnixMilli(run.ReadyAt+1))
		require.NoError(t, err)
		assert.Equal(t, 1, out.Save.Progression.Dual.Phase)
		run, err = model.StartGachaRun(user.Id, "dual_trial", uuid.NewString(), time.UnixMilli(run.ReadyAt+1))
		require.NoError(t, err)
		ids := []string{}
		for _, ally := range run.Result.Allies {
			ids = append(ids, ally.GirlID)
		}
		assert.Equal(t, team[4:], ids)
		require.True(t, run.Result.Win)
		out, err = model.FinishGachaRun(user.Id, run.ID, false, time.UnixMilli(run.ReadyAt+1))
		require.NoError(t, err)
		assert.True(t, out.Save.Progression.Dual.Cleared)
		assert.Equal(t, 2100, out.Save.Gold)
		assert.Equal(t, 20, out.Save.Mats["sigil_dust"])
		assert.Zero(t, out.Quota)
	})

	t.Run("progression_sigil_refinement_is_deterministic_preserves_other_affix_and_spends_once", func(t *testing.T) {
		user := gachaUser(t)
		state := gachaState(t, user.Id, now)
		sigil := gachaTestSigil("ember", 3)
		state.Sigils[sigil.ID] = sigil
		state.Gold, state.Mats["sigil_dust"] = 2000, 50
		seedGachaState(t, user.Id, state)
		choice, slot := "spd", 1
		intent := model.GachaActionIntent{RequestID: uuid.NewString(), Action: "sigil_refine", ItemID: &sigil.ID, Choice: &choice, Count: &slot}
		out, err := model.ApplyGachaAction(user.Id, intent, now)
		require.NoError(t, err)
		assert.Equal(t, map[string]int{"spd": 10, "cr": 5}, out.Save.Sigils[sigil.ID].Affixes)
		assert.Equal(t, "ember", out.Save.Sigils[sigil.ID].Family)
		assert.Equal(t, 1500, out.Save.Gold)
		assert.Equal(t, 40, out.Save.Mats["sigil_dust"])
		assert.Equal(t, 1, out.Save.Progression.Weeks[0].RefinementsLeft)
		retry, err := model.ApplyGachaAction(user.Id, intent, now)
		require.NoError(t, err)
		assert.Equal(t, out, retry)
	})

	t.Run("progression_weekly_variety_can_finish_after_five_repeated_activities", func(t *testing.T) {
		user := gachaUser(t)
		state := gachaState(t, user.Id, now)
		state.Progression.Weeks[0].Completed = 5
		state.Progression.Weeks[0].Activities = []string{"training"}
		seedGachaState(t, user.Id, state)
		id, week := "c04", state.Progression.Weeks[0].Week
		_, err := model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "weekly_claim", GirlID: &id, ItemID: &week}, now)
		assert.ErrorIs(t, err, model.ErrGachaProgressionLimit)
		_, err = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "wish", GirlID: &id}, now)
		require.NoError(t, err)
		out, err := model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "weekly_claim", GirlID: &id, ItemID: &week}, now)
		require.NoError(t, err)
		assert.Equal(t, 5, out.Save.Progression.Weeks[0].Completed)
		assert.Len(t, out.Save.Progression.Weeks[0].Activities, 2)
		assert.Equal(t, 120, out.Save.Progression.Marks)
	})

	t.Run("progression_receipt_failure_rolls_back_the_claim_and_keeps_its_opportunity", func(t *testing.T) {
		user := gachaUser(t)
		state := gachaState(t, user.Id, now)
		day := state.Progression.Days[0].Date
		intent := model.GachaActionIntent{RequestID: uuid.NewString(), Action: "daily_permits", ItemID: &day}
		callback := "gacha-progression-receipt-" + uuid.NewString()
		db := model.DB
		require.NoError(t, db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
			if action, ok := tx.Statement.Dest.(*model.GachaAction); ok && action.RequestID == intent.RequestID {
				tx.AddError(errors.New("isolated receipt failure"))
			}
		}))
		t.Cleanup(func() { _ = db.Callback().Create().Remove(callback) })
		_, err := model.ApplyGachaAction(user.Id, intent, now)
		require.Error(t, err)
		after := gachaState(t, user.Id, now)
		assert.Zero(t, after.Progression.Permits)
		assert.False(t, after.Progression.Days[0].PermitsClaimed)
		assert.Equal(t, state.Gold, after.Gold)
		require.NoError(t, db.Callback().Create().Remove(callback))
		out, err := model.ApplyGachaAction(user.Id, intent, now)
		require.NoError(t, err)
		assert.Equal(t, 5, out.Save.Progression.Permits)
		assert.True(t, out.Save.Progression.Days[0].PermitsClaimed)
	})

	t.Run("progression_fragment_unlock_uses_only_personal_fragments_and_replays_once", func(t *testing.T) {
		user := gachaUser(t)
		state := gachaState(t, user.Id, now)
		state.Girls["c13"] = &model.GachaOwnedGirl{Shards: 50, Gems: make([]string, 3)}
		state.Girls["c28"] = &model.GachaOwnedGirl{Gems: make([]string, 3)}
		state.Mats["shard_dust"], state.Mats["shard_dust_ssr"] = 5000, 5000
		seedGachaState(t, user.Id, state)
		id := "c28"
		_, err := model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "shard_unlock", GirlID: &id}, now)
		assert.ErrorIs(t, err, model.ErrGachaFunds)
		id = "c13"
		intent := model.GachaActionIntent{RequestID: uuid.NewString(), Action: "shard_unlock", GirlID: &id}
		out, err := model.ApplyGachaAction(user.Id, intent, now)
		require.NoError(t, err)
		assert.Equal(t, 2, out.Save.Girls[id].Star)
		assert.Equal(t, 1, out.Save.Girls[id].Level)
		assert.Zero(t, out.Save.Girls[id].Shards)
		assert.Equal(t, state.Mats, out.Save.Mats)
		retry, err := model.ApplyGachaAction(user.Id, intent, now)
		require.NoError(t, err)
		assert.Equal(t, out, retry)
	})

	t.Run("progression_last_bank_day_training_finishes_after_midnight_without_losing_or_repeating_XP", func(t *testing.T) {
		start := time.Date(2026, 11, 17, 12, 0, 0, 0, time.FixedZone("CST", 8*3600))
		when := time.Date(2026, 11, 23, 23, 59, 59, 800_000_000, start.Location())
		user := gachaUser(t)
		state := gachaState(t, user.Id, start)
		state.Squad = []string{"c04"}
		state.Girls["c04"] = &model.GachaOwnedGirl{Level: 20, Star: 3, Gems: make([]string, 3)}
		seedGachaState(t, user.Id, state)
		run, err := model.StartGachaRun(user.Id, "training", uuid.NewString(), when)
		require.NoError(t, err)
		finishedAt := time.UnixMilli(run.ReadyAt + 1)
		require.Equal(t, 24, finishedAt.In(start.Location()).Day())
		out, err := model.FinishGachaRun(user.Id, run.ID, false, finishedAt)
		require.NoError(t, err)
		assert.Equal(t, 21, out.Save.Girls["c04"].Level)
		require.Len(t, out.Save.Progression.Days, 7)
		assert.Equal(t, "2026-11-18", out.Save.Progression.Days[0].Date)
		assert.Equal(t, "2026-11-24", out.Save.Progression.Days[6].Date)
		retry, err := model.FinishGachaRun(user.Id, run.ID, false, finishedAt)
		require.NoError(t, err)
		assert.Equal(t, out, retry)
	})

	t.Run("progression_dual_final_reward_survives_week_rollover_without_clearing_new_week", func(t *testing.T) {
		zone := time.FixedZone("CST", 8*3600)
		when := time.Date(2026, 11, 22, 23, 50, 0, 0, zone)
		user := gachaUser(t)
		state := gachaState(t, user.Id, when)
		team := []string{"c04", "c07", "c09", "c28", "c11", "c05", "c13", "c10"}
		for _, id := range team {
			state.Girls[id] = &model.GachaOwnedGirl{Level: 60, Star: 5, Awakening: 3, Branch: model.GachaAwakeningBranches(id)[0].ID, Gems: make([]string, 3)}
		}
		state.Progression.TrialSquad, state.Progression.ReserveSquad = slices.Clone(team[:4]), slices.Clone(team[4:])
		state.Cleared["s6"] = true
		seedGachaState(t, user.Id, state)
		run, err := model.StartGachaRun(user.Id, "dual_trial", uuid.NewString(), when)
		require.NoError(t, err)
		require.True(t, run.Result.Win)
		_, err = model.FinishGachaRun(user.Id, run.ID, false, time.UnixMilli(run.ReadyAt+1))
		require.NoError(t, err)
		run, err = model.StartGachaRun(user.Id, "dual_trial", uuid.NewString(), time.Date(2026, 11, 22, 23, 59, 59, 800_000_000, zone))
		require.NoError(t, err)
		require.True(t, run.Result.Win)
		out, err := model.FinishGachaRun(user.Id, run.ID, false, time.UnixMilli(run.ReadyAt+1))
		require.NoError(t, err)
		assert.Equal(t, "2026-W48", out.Save.Progression.Dual.Week)
		assert.Zero(t, out.Save.Progression.Dual.Phase)
		assert.False(t, out.Save.Progression.Dual.Cleared)
		assert.Equal(t, 2100, out.Save.Gold)
		assert.Equal(t, 20, out.Save.Mats["sigil_dust"])
		assert.Zero(t, out.Quota)
		retry, err := model.FinishGachaRun(user.Id, run.ID, false, time.UnixMilli(run.ReadyAt+1))
		require.NoError(t, err)
		assert.Equal(t, out, retry)
	})

	t.Run("progression_configuration_changes_do_not_rewrite_an_already_issued_training_reward", func(t *testing.T) {
		t.Cleanup(func() {
			require.NoError(t, model.DB.Where(&model.Option{Key: "GachaProgressionConfig"}).Delete(&model.Option{}).Error)
		})
		user := gachaUser(t)
		state := gachaState(t, user.Id, now)
		state.Squad = []string{"c04"}
		state.Girls["c04"] = &model.GachaOwnedGirl{Level: 20, Star: 3, Gems: make([]string, 3)}
		seedGachaState(t, user.Id, state)
		run, err := model.StartGachaRun(user.Id, "training", uuid.NewString(), now)
		require.NoError(t, err)
		require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", user.Id).Update("role", common.RoleRootUser).Error)
		config := model.DefaultGachaProgressionConfig()
		config.Version, config.TrainingXP = 2, [3]int{3000, 5000, 7000}
		_, err = model.ApplyGachaAction(user.Id, model.GachaActionIntent{RequestID: uuid.NewString(), Action: "progression_config", Config: &config}, now)
		require.NoError(t, err)
		out, err := model.FinishGachaRun(user.Id, run.ID, false, time.UnixMilli(run.ReadyAt+1))
		require.NoError(t, err)
		assert.Equal(t, 21, out.Save.Girls["c04"].Level)
		assert.Zero(t, out.Save.Girls["c04"].Exp)
		assert.Equal(t, 2000, out.Save.Progression.Days[0].Rules.TrainingXP[0])
		assert.Equal(t, 3000, out.Save.ProgressionRules.TrainingXP[0])
	})

	t.Run("progression_config_requires_admin_and_only_future_entitlements_use_new_budgets", func(t *testing.T) {
		t.Cleanup(func() {
			require.NoError(t, model.DB.Where(&model.Option{Key: "GachaProgressionConfig"}).Delete(&model.Option{}).Error)
		})
		user := gachaUser(t)
		profile, err := model.GetGachaProfile(user.Id, now)
		require.NoError(t, err)
		assert.False(t, profile.Progression.CanConfigure)
		config := model.DefaultGachaProgressionConfig()
		config.Version, config.PermitsPerDay, config.TrainingXP = 2, 7, [3]int{3000, 5000, 7000}
		intent := model.GachaActionIntent{RequestID: uuid.NewString(), Action: "progression_config", Config: &config}
		_, err = model.ApplyGachaAction(user.Id, intent, now)
		assert.ErrorIs(t, err, model.ErrGachaProgressionAdmin)
		require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", user.Id).Update("role", common.RoleRootUser).Error)
		out, err := model.ApplyGachaAction(user.Id, intent, now)
		require.NoError(t, err)
		assert.Equal(t, 5, out.Save.Progression.Days[0].Rules.PermitsPerDay)
		assert.Equal(t, 7, out.ProgressionRules.PermitsPerDay)
		state := gachaState(t, user.Id, now.AddDate(0, 0, 1))
		assert.Equal(t, 7, state.Progression.Days[1].Rules.PermitsPerDay)
		intent.RequestID = uuid.NewString()
		_, err = model.ApplyGachaAction(user.Id, intent, now.AddDate(0, 0, 1))
		assert.ErrorIs(t, err, model.ErrGachaConfigVersion)
		require.NoError(t, model.DB.Where(&model.Option{Key: "GachaProgressionConfig"}).Delete(&model.Option{}).Error)
	})
}

package controller

import (
	"errors"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"slices"
	"strconv"
	"time"
)

// Game state and rewards require a dashboard session, never a manually bound PAT.
func gachaSessionUser(c *gin.Context) (int, bool) {
	identity, ok := middleware.GetSessionAuthIdentity(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "code": "AUTH_SESSION_REQUIRED", "message": "请先登录站点"})
		return 0, false
	}
	return identity.UserID, true
}

func decodeGachaIntent(c *gin.Context, body any, allowed []string) bool {
	raw, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, 16*1024))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "GACHA_INVALID_INTENT", "message": "操作参数不正确"})
		return false
	}
	var fields map[string]common.RawMessage
	if err := common.Unmarshal(raw, &fields); err != nil || fields == nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "GACHA_INVALID_INTENT", "message": "操作参数不正确"})
		return false
	}
	for field, value := range fields {
		if !slices.Contains(allowed, field) || string(value) == "null" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "GACHA_INVALID_INTENT", "message": "不接受存档、战斗结果或其他未知参数"})
			return false
		}
	}
	if err := common.Unmarshal(raw, body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "GACHA_INVALID_INTENT", "message": "操作参数不正确"})
		return false
	}
	return true
}

func writeGachaError(c *gin.Context, err error) {
	status, code, message := http.StatusInternalServerError, "GACHA_INTERNAL_ERROR", "游戏服务暂时不可用，请稍后重试"
	switch {
	case errors.Is(err, model.ErrGachaIntent):
		status, code, message = http.StatusBadRequest, "GACHA_INVALID_INTENT", "操作参数不正确"
	case errors.Is(err, model.ErrGachaStage):
		status, code, message = http.StatusBadRequest, "GACHA_STAGE_UNKNOWN", "关卡不存在"
	case errors.Is(err, model.ErrGachaTeamShared):
		status, code, message = http.StatusBadRequest, "GACHA_TEAM_SHARED", "同一少女只能在一支队伍中出战，请先在本队或其他队伍中调整"
	case errors.Is(err, model.ErrGachaTeam):
		status, code, message = http.StatusBadRequest, "GACHA_TEAM_INVALID", "请先上阵已拥有的少女，队伍最多四人且不能重复"
	case errors.Is(err, model.ErrGachaLocked):
		status, code, message = http.StatusConflict, "GACHA_STAGE_LOCKED", "请先通关前置关卡"
	case errors.Is(err, model.ErrGachaFunds):
		status, code, message = http.StatusConflict, "GACHA_RESOURCES_REQUIRED", "代币、体力、金币、碎片或材料不足"
	case errors.Is(err, model.ErrGachaExperience):
		status, code, message = http.StatusConflict, "GACHA_EXPERIENCE_REQUIRED", "等级需要真实经验，金币不能直升；请完成经验训练或挑战副本"
	case errors.Is(err, model.ErrGachaPermits):
		status, code, message = http.StatusConflict, "GACHA_PERMITS_REQUIRED", "招募券不足，请到成长页面领取日常、周常或主线奖励"
	case errors.Is(err, model.ErrGachaProgressionLimit):
		status, code, message = http.StatusConflict, "GACHA_PROGRESSION_LIMIT", "该机会已使用、已过期或尚未满足条件，请查看成长页面"
	case errors.Is(err, model.ErrGachaAwakening):
		status, code, message = http.StatusConflict, "GACHA_AWAKENING_REQUIRED", "觉醒需要对应等级、星级和专属试炼；一至三阶分别需要40/50/60级与3/4/5星"
	case errors.Is(err, model.ErrGachaDualTeam):
		status, code, message = http.StatusConflict, "GACHA_DUAL_TEAM_REQUIRED", "请保存八名不同角色的两支队伍；双队试炼要求每名角色至少40级3星"
	case errors.Is(err, model.ErrGachaProgressionAdmin):
		status, code, message = http.StatusForbidden, "GACHA_PROGRESSION_ADMIN", "只有管理员可以修改成长配置"
	case errors.Is(err, model.ErrGachaConfigVersion):
		status, code, message = http.StatusConflict, "GACHA_CONFIG_VERSION", "配置版本已更新，请同步成长页面后重新提交新版本"
	case errors.Is(err, model.ErrGachaShopLimit):
		status, code, message = http.StatusConflict, "GACHA_SHOP_LIMIT", "今日商店20次兑换已用完，明天北京时间零点刷新"
	case errors.Is(err, model.ErrGachaWalletFunds):
		status, code, message = http.StatusConflict, "GACHA_WALLET_FUNDS", "站点可用余额不足，请减少兑换数量"
	case errors.Is(err, model.ErrGachaExchangeUnavailable):
		status, code, message = http.StatusConflict, "GACHA_EXCHANGE_UNAVAILABLE", "余额兑换暂不可用，请稍后重试"
	case errors.Is(err, model.ErrGachaCapacity):
		status, code, message = http.StatusConflict, "GACHA_CAPACITY", "资源或灵印背包达到上限，请先使用或分解"
	case errors.Is(err, model.ErrGachaSigil):
		status, code, message = http.StatusConflict, "GACHA_SIGIL", "灵印不存在或已被其他少女装备，请先拆下；已装备的灵印不能分解"
	case errors.Is(err, model.ErrGachaRest):
		var rest *model.GachaRestError
		if errors.As(err, &rest) {
			retry := max(1, int((rest.Until-time.Now().UnixMilli()+999)/1000))
			c.Header("Retry-After", strconv.Itoa(retry))
			c.JSON(http.StatusTooManyRequests, gin.H{"success": false, "code": "GACHA_REST", "message": "连续挑战过快，休整后再出战；可继续编队、养成和整理背包", "retry_after": retry, "rest_until": rest.Until})
			return
		}
		status, code, message = http.StatusTooManyRequests, "GACHA_REST", "连续挑战过快，请稍作休整"
	case errors.Is(err, model.ErrGachaMode):
		status, code, message = http.StatusConflict, "GACHA_MODE", "该玩法当前不可挑战，请先选择路线或等待下周刷新"
	case errors.Is(err, model.ErrGachaModeLimit):
		status, code, message = http.StatusConflict, "GACHA_MODE_LIMIT", "本周首领已击败或三次挑战已用完，下周一北京时间零点刷新"
	case errors.Is(err, model.ErrGachaTalentLocked):
		status, code, message = http.StatusConflict, "GACHA_TALENT_LOCKED", "少女升至三星后才能选择天赋"
	case errors.Is(err, model.ErrGachaGuild):
		status, code, message = http.StatusConflict, "GACHA_GUILD", "公会不存在、邀请码无效，或你已加入公会"
	case errors.Is(err, model.ErrGachaGuildLimit):
		status, code, message = http.StatusConflict, "GACHA_GUILD_LIMIT", "公会人数已满、今日已挑战或奖励暂不可领取；会长需待其他成员退出后才能退出"
	case errors.Is(err, model.ErrGachaArenaLimit):
		status, code, message = http.StatusConflict, "GACHA_ARENA_LIMIT", "每日最多匹配五场，明天北京时间零点刷新"
	case errors.Is(err, model.ErrGachaOpponent):
		status, code, message = http.StatusConflict, "GACHA_OPPONENT", "对手阵容暂不可用，请刷新后选择其他玩家"
	case errors.Is(err, model.ErrGachaArenaPending):
		status, code, message = http.StatusConflict, "GACHA_ARENA_PENDING", "匹配暂不可用，请稍后重试；已预留金额仍可退回"
	case errors.Is(err, model.ErrGachaArenaWaiting):
		status, code, message = http.StatusConflict, "GACHA_ARENA_WAITING", "你已在队列或房间中，请先查看或退出当前对战"
	case errors.Is(err, model.ErrGachaRoom):
		status, code, message = http.StatusConflict, "GACHA_ROOM_UNAVAILABLE", "房间已满、已结束、尚未满员，或战力差超过20%，请刷新大厅"
	case errors.Is(err, model.ErrGachaArenaFunds):
		status, code, message = http.StatusConflict, "GACHA_ARENA_FUNDS", "站点可用余额不足，请选择较低的匹配金额"
	case errors.Is(err, model.ErrGachaNotReady):
		status, code, message = http.StatusConflict, "GACHA_RUN_NOT_READY", "战斗尚未结束，请等待服务端结算时间"
	case errors.Is(err, model.ErrGachaReplay):
		status, code, message = http.StatusConflict, "GACHA_REQUEST_REUSED", "该操作编号已用于其他操作"
	case errors.Is(err, model.ErrGachaConcurrent):
		status, code, message = http.StatusConflict, "GACHA_STATE_CONFLICT", "状态已更新，请使用同一操作编号重试"
	case errors.Is(err, model.ErrGachaRunState):
		status, code, message = http.StatusConflict, "GACHA_RUN_INVALID", "副本已失效或上一局尚未结束"
	case errors.Is(err, model.ErrGachaCardState):
		status, code, message = http.StatusConflict, "GACHA_CARD_INVALID", "卡片不可用，或已有生效中的折扣卡"
	default:
		common.SysError("gacha request failed: " + err.Error())
	}
	c.JSON(status, gin.H{"success": false, "code": code, "message": message})
}

func GetGachaProfile(c *gin.Context) {
	userID, ok := gachaSessionUser(c)
	if !ok {
		return
	}
	profile, err := model.GetGachaProfile(userID, time.Now())
	if err != nil {
		writeGachaError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": profile})
}

func ApplyGachaAction(c *gin.Context) {
	userID, ok := gachaSessionUser(c)
	if !ok {
		return
	}
	var body model.GachaActionIntent
	if !decodeGachaIntent(c, &body, []string{"request_id", "action", "count", "girl_id", "squad", "enabled", "item_id", "stake_e", "queue_id", "room_id", "choice", "config"}) {
		return
	}
	if err := model.ValidateGachaAction(body); err != nil {
		writeGachaError(c, err)
		return
	}
	result, err := model.ApplyGachaAction(userID, body, time.Now())
	if err != nil {
		writeGachaError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

func GetGachaSocial(c *gin.Context) {
	userID, ok := gachaSessionUser(c)
	if !ok {
		return
	}
	result, err := model.GetGachaSocial(userID)
	if err != nil {
		writeGachaError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

func GetGachaArenaReplay(c *gin.Context) {
	userID, ok := gachaSessionUser(c)
	if !ok {
		return
	}
	result, err := model.GetGachaArenaReplay(userID, c.Query("request_id"))
	if err != nil {
		writeGachaError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

func GetGachaArenaQueue(c *gin.Context) {
	userID, ok := gachaSessionUser(c)
	if !ok {
		return
	}
	result, err := model.GetGachaArenaQueue(userID, time.Now())
	if err != nil {
		writeGachaError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

func GetGachaArenaRooms(c *gin.Context) {
	userID, ok := gachaSessionUser(c)
	if !ok {
		return
	}
	result, err := model.GetGachaArenaRooms(userID, time.Now())
	if err != nil {
		writeGachaError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

func StartGachaRun(c *gin.Context) {
	userID, ok := gachaSessionUser(c)
	if !ok {
		return
	}
	var body struct {
		Stage     string `json:"stage"`
		RequestID string `json:"request_id"`
	}
	if !decodeGachaIntent(c, &body, []string{"stage", "request_id"}) {
		return
	}
	run, err := model.StartGachaRun(userID, body.Stage, body.RequestID, time.Now())
	if err != nil {
		writeGachaError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": run})
}

func FinishGachaRun(c *gin.Context) {
	userID, ok := gachaSessionUser(c)
	if !ok {
		return
	}
	var body struct {
		RunID   string `json:"run_id"`
		Abandon bool   `json:"abandon"`
	}
	if !decodeGachaIntent(c, &body, []string{"run_id", "abandon"}) {
		return
	}
	result, err := model.FinishGachaRun(userID, body.RunID, body.Abandon, time.Now())
	if err != nil {
		writeGachaError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

func GetGachaWarehouse(c *gin.Context) {
	userID, ok := gachaSessionUser(c)
	if !ok {
		return
	}
	cards, active := model.ListGachaCards(userID, time.Now())
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"cards": cards, "active": active}})
}

func ActivateGachaCard(c *gin.Context) {
	userID, ok := gachaSessionUser(c)
	if !ok {
		return
	}
	var body struct {
		CardID string `json:"card_id"`
	}
	if !decodeGachaIntent(c, &body, []string{"card_id"}) {
		return
	}
	if len(body.CardID) == 0 || len(body.CardID) > 40 {
		writeGachaError(c, model.ErrGachaIntent)
		return
	}
	card, err := model.ActivateGachaCard(userID, body.CardID, time.Now())
	if err != nil {
		writeGachaError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": card})
}

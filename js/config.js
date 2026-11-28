/* =====================================================
 * config.js —— 站点联动配置
 * 游戏与站点同域部署（/gacha/ + /api/），接口走相对路径
 * ===================================================== */
const SITE = {
  // 站点接口：与游戏同域，直接用相对路径，避免跨域与 http 混合内容
  apiBase: "",
  ep: {
    session:   "/api/user/auth/refresh",       // POST  复用站点 HttpOnly 登录 Cookie
    profile:   "/api/user/gacha/profile",      // GET  账号 + 今日产出
    action:    "/api/user/gacha/action",       // POST 操作意图，服务端计算并写入数据库
    start:     "/api/user/gacha/run/start",    // POST {stage, request_id} -> 服务器战报
    finish:    "/api/user/gacha/run/finish",   // POST {run_id, abandon?}，不上传星级或奖励
    warehouse: "/api/user/gacha/warehouse",    // GET  倍率卡仓库
    activate:  "/api/user/gacha/card/activate",// POST {card_id}
    social:    "/api/user/gacha/social",      // GET rankings and opponents
    fleet:     "/api/user/gacha/fleet",
    world:     "/api/user/gacha/world",
    arenaQueue: "/api/user/gacha/arena/queue",
    arenaRooms: "/api/user/gacha/arena/rooms",
    arenaReplay: "/api/user/gacha/arena/replay",
    ladder: "/api/user/gacha/ladder",
    ladderReplay: "/api/user/gacha/ladder/replay",
  },
  // 站点额度口径：1E = 500,000 quota
  quotaPerE: 500000,
  requestTimeoutMs: 8000,
  loginUrl: "/sign-in",
  warehouseUrl: "/warehouse",
};

/* 体力恢复：每 5 分钟 1 点 */
const STAMINA_REGEN_MIN = 5;

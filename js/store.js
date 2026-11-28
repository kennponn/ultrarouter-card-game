/* =====================================================
 * store.js —— 复用站点登录态 + 副本产出（服务器为准）
 * 浏览器只保留显示快照，进度与每次经济操作由数据库和服务器决定
 * ===================================================== */

let SITE_ACCOUNT = null;   // 仅内存保存当前登录账号，不保存访问令牌

const Store = {
  serverOK: false,
  lastError: "",
  profile: null,
  connecting: false,
  accessToken: "",
  accessExpiresAt: 0,
  sessionId: "",
  sessionEpoch: 0,
  sessionPromise: null,
  profilePromise: null,
  authUserId: 0,
  runSessions: new Map(),
  revision: -1,
  activeRun: null,
  mutationBusy: false,
  pendingAction: null,
  pendingStart: null,
  serverClock: null,
  runRestUntil: 0,

  serverNow() {
    if (!this.serverClock) return Date.now();
    return this.serverClock.time + performance.now() - this.serverClock.sampledAt;
  },

  /* ---------- 初始化 ---------- */
  init() {
    // 不读取、不导入未受信任的旧本地档案。
    for (const key of ["h5gacha_bind_v2", "h5gacha_save_v1"]) {
      try { localStorage.removeItem(key); } catch (e) {}
    }
    SAVE = null;
  },

  ready() {
    return Boolean(this.serverOK && this.sessionId && SAVE);
  },

  applyState(data) {
    const s = data && data.save;
    if (!s || !s.girls || !Array.isArray(s.squad) || !s.daily || !s.mats ||
        !s.cleared || !s.stars || !Number.isSafeInteger(data.revision)) {
      throw new Error("服务器游戏档案不完整");
    }
    if (data.revision < this.revision) return false;
    SAVE = s;
    this.revision = data.revision;
    if (data.progression_rules && this.profile?.progression) this.profile.progression.rules = data.progression_rules;
    if (Number.isSafeInteger(s.activity?.rest_until)) this.runRestUntil = s.activity.rest_until;
    return true;
  },

  /* ---------- 请求封装 ---------- */
  async request(path, method, body, headers) {
    const url = new URL(SITE.apiBase + path, location.href);
    if (url.origin !== location.origin) throw new Error("仅支持同站点登录");
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), SITE.requestTimeoutMs);
    try {
      const r = await fetch(url.href, {
        method: method || "GET",
        credentials: "same-origin",
        cache: "no-store",
        signal: controller.signal,
        headers: Object.assign({ "Content-Type": "application/json" }, headers),
        body: body ? JSON.stringify(body) : undefined,
      });
      // HTTP Date is server time; monotonic elapsed time ignores local clock changes.
      const serverDate = r.headers && r.headers.get("Date");
      const serverTime = serverDate && Date.parse(serverDate);
      if (Number.isFinite(serverTime)) {
        this.serverClock = { time: serverTime, sampledAt: performance.now() };
      }
      let d = null;
      try { d = await r.json(); } catch (e) {}
      return { status: r.status, ok: r.ok, data: d };
    } catch (e) {
      const error = new Error(e.name === "AbortError" ? "TIMEOUT" : e.message);
      if (e.name === "TypeError") error.code = "NETWORK";
      throw error;
    } finally {
      clearTimeout(timer);
    }
  },

  clearSession() {
    this.sessionEpoch++;
    this.accessToken = "";
    this.accessExpiresAt = 0;
    this.sessionId = "";
    this.authUserId = 0;
    this.runSessions.clear();
    this.revision = -1;
    this.activeRun = null;
    this.pendingAction = null;
    this.pendingStart = null;
    this.runRestUntil = 0;
    SAVE = null;
    this.serverOK = false;
    this.profile = null;
    SITE_ACCOUNT = null;
    if (typeof lockGame === "function") lockGame("请登录站点后进入游戏");
  },

  /* HttpOnly Cookie 只交给站点刷新接口；短期登录凭证只留在内存。 */
  async restoreSession() {
    if (this.sessionPromise) return this.sessionPromise;
    const epoch = this.sessionEpoch;
    const restore = async () => {
      const delays = [80, 200, 500];
      for (let attempt = 0; ; attempt++) {
        if (epoch !== this.sessionEpoch) throw new Error("AUTH_SESSION_CHANGED");
        const headers = this.sessionId ? { "X-Auth-Session": this.sessionId } : {};
        const r = await this.request(SITE.ep.session, "POST", null, headers);
        if (epoch !== this.sessionEpoch) throw new Error("AUTH_SESSION_CHANGED");
        const d = r.data;
        if (r.status === 409 && d && d.code === "AUTH_REFRESH_RACE" && attempt < delays.length) {
          await new Promise(resolve => setTimeout(resolve, delays[attempt]));
          continue;
        }
        if (r.status === 401) {
          this.clearSession();
          throw new Error("AUTH");
        }
        if (r.status === 409 && d && d.code === "AUTH_SESSION_MISMATCH") {
          this.clearSession();
          throw new Error("AUTH_SESSION_CHANGED");
        }
        const bundle = d && d.data;
        if (!r.ok || !d || !d.success || !bundle) {
          const error = new Error((d && d.message) || ("HTTP " + r.status));
          error.status = r.status;
          error.code = d && d.code;
          throw error;
        }
        if (typeof bundle.access_token !== "string" || !bundle.access_token ||
            bundle.token_type !== "Bearer" || !Number.isFinite(bundle.access_expires_at) ||
            bundle.access_expires_at <= this.serverNow() / 1000 ||
            !bundle.user || !Number.isInteger(bundle.user.id) || bundle.user.id <= 0 ||
            !bundle.session || !bundle.session.sid || bundle.session.current !== true) {
          throw new Error("站点登录信息不完整");
        }
        if (this.sessionId && (bundle.session.sid !== this.sessionId || bundle.user.id !== this.authUserId)) {
          this.clearSession();
          throw new Error("AUTH_SESSION_CHANGED");
        }
        this.accessToken = bundle.access_token;
        this.accessExpiresAt = bundle.access_expires_at;
        this.sessionId = bundle.session.sid;
        this.authUserId = bundle.user.id;
        return;
      }
    };
    // 与站点主页面共用锁，避免两个标签同时轮转刷新 Cookie。
    this.sessionPromise = (navigator.locks
      ? navigator.locks.request("new-api:auth-refresh", { mode: "exclusive" }, restore)
      : restore()).finally(() => { this.sessionPromise = null; });
    return this.sessionPromise;
  },

  async api(path, method, body) {
    const epoch = this.sessionEpoch;
    try {
      if (!this.accessToken || this.accessExpiresAt <= this.serverNow() / 1000 + 30) {
        await this.restoreSession();
      }
      if (epoch !== this.sessionEpoch) throw new Error("AUTH_SESSION_CHANGED");
      let r = await this.request(path, method, body, { Authorization: "Bearer " + this.accessToken });
      if (epoch !== this.sessionEpoch) throw new Error("AUTH_SESSION_CHANGED");
      // 仅重试被鉴权层拒绝的请求，不重放网络失败或已到达结算层的请求。
      if (r.status === 401) {
        await this.restoreSession();
        if (epoch !== this.sessionEpoch) throw new Error("AUTH_SESSION_CHANGED");
        r = await this.request(path, method, body, { Authorization: "Bearer " + this.accessToken });
      }
      if (epoch !== this.sessionEpoch) throw new Error("AUTH_SESSION_CHANGED");
      if (r.status === 401) {
        this.clearSession();
        throw new Error("AUTH");
      }
      const d = r.data;
      if (!r.ok || !d || d.success !== true) {
        const error = new Error((d && d.message) || ("HTTP " + r.status));
        error.status = r.status;
        error.code = d && d.code;
        if (error.code === "GACHA_REST" && Number.isSafeInteger(d.rest_until)) {
          this.runRestUntil = d.rest_until;
          if (typeof updateStageRunLimits === "function") updateStageRunLimits();
        }
        throw error;
      }
      this.lastError = "";
      return d.data;
    } catch (e) {
      if (epoch !== this.sessionEpoch && e.message !== "AUTH" && e.message !== "AUTH_SESSION_CHANGED") {
        throw new Error("AUTH_SESSION_CHANGED");
      }
      this.lastError = e.message;
      throw e;
    }
  },

  /* 刷新站点余额与今日产出 */
  refresh() {
    if (this.profilePromise) return this.profilePromise;
    this.connecting = true;
    this.profilePromise = this.refreshProfile().finally(() => {
      this.connecting = false;
      this.profilePromise = null;
    });
    return this.profilePromise;
  },

  async refreshProfile() {
    const epoch = this.sessionEpoch;
    try {
      let d;
      try {
        d = await this.api(SITE.ep.profile, "GET");
      } catch (e) {
        // 切换站点账号后只重新读取档案，写操作不会转交给新账号。
        if (e.message !== "AUTH_SESSION_CHANGED") throw e;
        d = await this.api(SITE.ep.profile, "GET");
      }
      if (!d || d.user_id !== this.authUserId) throw new Error("站点账号信息不匹配");
      const accepted = this.applyState(d);
      if (accepted) SITE_ACCOUNT = { uid: d.user_id, name: d.name, quota: d.quota };
      if (accepted) this.activeRun = d.active_run || null;
      else d = { ...this.profile, save: SAVE, revision: this.revision, active_run: this.activeRun };
      if (this.activeRun) this.runSessions.set(this.activeRun.id, this.sessionId);
      this.serverOK = true;
      if (this.activeRun) this.pendingStart = null;
      this.lastError = "";
      this.profile = d;
      return d;
    } catch (e) {
      const transient = e.status === 429 || e.status >= 500 || e.message === "TIMEOUT" || e.code === "NETWORK";
      if (transient && epoch === this.sessionEpoch && this.ready()) {
        this.lastError = e.message;
        if (typeof toast === "function") toast("同步暂未成功，当前页面已保留，请稍后重试");
        return null;
      }
      this.serverOK = false;
      this.profile = null;
      SITE_ACCOUNT = null;
      this.lastError = e.message;
      let message = "暂时无法进入游戏，请重新验证；若仍失败，请重新登录站点";
      if (e.message === "AUTH" || e.message === "AUTH_SESSION_CHANGED") {
        message = "站点登录已失效或账号已切换，请重新登录站点";
      } else if (e.status === 429) {
        message = "验证过于频繁，请稍后点击重新验证";
      } else if (e.status >= 500) {
        message = "站点服务暂时不可用，请稍后点击重新验证";
      } else if (e.message === "TIMEOUT") {
        message = "连接站点超时，请检查网络后重新验证";
      } else if (e.code === "NETWORK") {
        message = "网络连接失败，请检查网络后重新验证";
      } else if (e.message === "站点登录信息不完整") {
        message = "站点登录信息校验失败，请重新登录站点";
      }
      if (typeof lockGame === "function") lockGame(message);
      return null;
    }
  },

  /* ---------- 副本：开局 / 结算 ---------- */
  async startRun(stageId) {
    const epoch = this.sessionEpoch;
    if (this.profilePromise) await this.profilePromise;
    if (epoch !== this.sessionEpoch) { this.lastError = "AUTH_SESSION_CHANGED"; return null; }
    if (!this.ready()) return null;
    try {
      if (this.pendingStart && this.pendingStart.stage !== stageId) {
        throw new Error("上一局开局结果未确认，请先重新同步");
      }
      this.pendingStart ||= { stage: stageId, request_id: crypto.randomUUID() };
      const run = await this.api(SITE.ep.start, "POST", this.pendingStart);
      this.applyState(run);
      this.pendingStart = null;
      this.activeRun = run;
      if (run && run.id) this.runSessions.set(run.id, this.sessionId);
      return run;
    } catch (e) {
      if (e.status && e.status < 500 && e.code !== "GACHA_STATE_CONFLICT") this.pendingStart = null;
      this.lastError = e.message;
      return null;
    }
  },

  async finishRun(runId, abandon) {
    if (runId && this.runSessions.get(runId) !== this.sessionId) {
      this.lastError = "AUTH_SESSION_CHANGED";
      return null;
    }
    if (!this.sessionId || !runId) return null;
    try {
      const body = { run_id: runId };
      if (abandon === true) body.abandon = true;
      const d = await this.api(SITE.ep.finish, "POST", body);
      const accepted = this.applyState(d);
      if (accepted && SITE_ACCOUNT && Number.isSafeInteger(d.wallet_quota)) {
        SITE_ACCOUNT.quota = d.wallet_quota;
      }
      this.runSessions.delete(runId);
      this.activeRun = null;
      return d;
    } catch (e) {
      this.lastError = e.message;
      return null;
    }
  },

  /* ---------- 仓库 ---------- */
  async warehouse() {
    if (!this.ready()) return null;
    try { return await this.api(SITE.ep.warehouse, "GET"); } catch (e) { return null; }
  },

  async activate(cardId) {
    if (!this.ready()) return { ok: false, msg: "请先登录站点" };
    try {
      const d = await this.api(SITE.ep.activate, "POST", { card_id: cardId });
      return { ok: true, card: d };
    } catch (e) {
      return { ok: false, msg: e.message };
    }
  },

  /* 只提交操作意图；重试未确认的操作沿用同一幂等键。 */
  async mutate(action, fields) {
    if (!this.ready()) return null;
    if (this.mutationBusy) { toast("操作正在处理，请稍候"); return null; }
    const fingerprint = JSON.stringify({ action, ...fields });
    if (this.pendingAction && this.pendingAction.fingerprint !== fingerprint) {
      showPendingAction("上一操作结果未确认，请先重试原操作");
      return null;
    }
    this.pendingAction ||= {
      fingerprint,
      body: { action, ...fields, request_id: crypto.randomUUID() },
    };
    return this.retryPendingAction();
  },

  async retryPendingAction() {
    if (!this.ready() || this.mutationBusy || !this.pendingAction) return null;
    const pending = this.pendingAction;
    this.mutationBusy = true;
    try {
      const d = await this.api(SITE.ep.action, "POST", pending.body);
      const accepted = this.applyState(d);
      if (accepted && pending.body.action.startsWith("guild_") && this.profile) {
        this.profile.guild = d.guild || null;
      }
      this.pendingAction = null;
      return d;
    } catch (e) {
      if (e.status && e.status < 500 && e.code !== "GACHA_STATE_CONFLICT") this.pendingAction = null;
      if (this.pendingAction && this.ready()) showPendingAction(e.message);
      else toast(e.message === "AUTH" ? "登录已失效，请重新登录" : e.message);
      return null;
    } finally {
      this.mutationBusy = false;
    }
  },
};

/* 额度展示：quota -> E */
function quotaToE(quota) {
  return (quota / SITE.quotaPerE).toFixed(4);
}

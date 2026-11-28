/* =====================================================
 * main.js —— 启动入口与全局事件
 * ===================================================== */

function boot() {
  Store.init();
  lockGame("正在确认站点登录并读取数据库档案…");
  syncSiteAccount();
}

function lockGame(message) {
  if (typeof closeFleetOfficial === "function") closeFleetOfficial();
  document.getElementById("app").classList.add("hidden");
  document.getElementById("title-screen").classList.remove("hidden");
  document.getElementById("auth-status").textContent = message;
  document.getElementById("auth-actions").classList.toggle("hidden", Store.connecting);
  document.getElementById("modal-mask").classList.add("hidden");
  document.getElementById("summon-fx").classList.add("hidden");
  if (BAT) BAT.abort = true;
  BAT = null;
  battleRunning = false;
  document.body.classList.remove("in-battle");
}

function requireGame() {
  if (Store.ready()) return true;
  lockGame(Store.connecting ? "正在验证站点账号…" : "请登录站点后进入游戏");
  return false;
}

function unlockGame() {
  if (!Store.ready()) return;
  const alreadyOpen = !document.getElementById("app").classList.contains("hidden");
  document.getElementById("title-screen").classList.add("hidden");
  document.getElementById("app").classList.remove("hidden");
  // A same-account refresh must not rebuild tabs, dialogs, or an in-progress form.
  if (alreadyOpen) {
    renderTopbar();
    const page = document.body.dataset.page;
    if (page === "page-captain" && typeof renderCaptain === "function") renderCaptain();
    if (page === "page-festival" && typeof festivalView !== "undefined" && festivalView?.date !== SAVE.daily.date) renderFestival();
    if (page === "page-home" && typeof renderFestivalToday === "function") renderFestivalToday();
    return;
  }
  if (battleRunning && BAT) { renderTopbar(); return; }
  const hash = (location.hash || "").replace(/^#/, "");
  let page = document.body.dataset.page || "page-home";
  if (hash && hash !== "battle" && document.getElementById("page-" + hash)) page = "page-" + hash;
  if (page === "page-battle") page = "page-stage";
  nav(page);
  if (Store.activeRun && Store.activeRun.result) resumeBattle(Store.activeRun);
}

/* 兜底：任何未捕获错误都提示出来，避免白屏无反馈 */
window.addEventListener("error", e => {
  if (typeof toast === "function") toast("运行错误：" + e.message);
});

/* 底部导航 */
document.querySelectorAll(".nav-btn").forEach(b =>
  b.addEventListener("click", () => {
    if (battleRunning && b.dataset.nav !== "page-battle") {
      return toast("战斗进行中，等结算后再离开");
    }
    nav(b.dataset.nav);
  }));

/* 页内跳转按钮（主页 -> 副本/召唤） */
document.querySelectorAll("[data-nav]").forEach(el => {
  if (!el.classList.contains("nav-btn")) {
    el.addEventListener("click", () => {
      nav(el.dataset.nav);
      if (el.dataset.homePool && Store.ready()) selectDrawPool(el.dataset.homePool);
      if (el.dataset.arenaMode) pickArenaTab(el.dataset.arenaMode);
    });
  }
});

/* 召唤按钮 */
document.getElementById("btn-gacha-1").addEventListener("click", () => doGacha(1));
document.getElementById("btn-gacha-10").addEventListener("click", () => doGacha(10));

/* 点遮罩关弹窗 */
document.getElementById("modal-mask").addEventListener("click", e => {
  if (e.target.id === "modal-mask") closeModal();
});

/* 返回游戏或站点切换账号时，同步账号；不在后台定时轮转登录 Cookie。 */
window.addEventListener("focus", () => syncSiteAccount(true));
document.addEventListener("visibilitychange", () => {
  if (document.visibilityState === "visible") syncSiteAccount(true);
});

function onSiteSessionEvent(event) {
  if (!event || !["authenticated", "signed_out"].includes(event.kind) ||
      typeof event.sid !== "string" || !Number.isFinite(event.timestamp) ||
      Math.abs(Date.now() - event.timestamp) > 60000) return;
  if (event.kind === "signed_out" && event.sid !== Store.sessionId) return;
  if (event.kind === "authenticated" && event.sid === Store.sessionId) return;
  Store.clearSession();
  Store.lastError = "AUTH";
  if (event.kind === "authenticated") {
    const pending = Store.profilePromise || Promise.resolve();
    pending.then(() => {
      if (Store.sessionId !== event.sid) syncSiteAccount();
    });
  }
}

if (typeof BroadcastChannel !== "undefined") {
  const channel = new BroadcastChannel("new-api:auth-session");
  channel.addEventListener("message", event => onSiteSessionEvent(event.data));
} else {
  window.addEventListener("storage", event => {
    if (event.key !== "new-api:auth-session:event" || !event.newValue) return;
    try { onSiteSessionEvent(JSON.parse(event.newValue)); } catch (e) {}
  });
}

boot();

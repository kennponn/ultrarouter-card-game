/* =====================================================
 * ui.js —— 页面交互：导航 / 主页 / 召唤 / 编队 / 副本
 * ===================================================== */

function esc(value) {
  return String(value).replace(/[&<>"']/g, ch => ({
    "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;",
  })[ch]);
}

/* ---------- 页面路由 ---------- */
function nav(pageId) {
  if (!requireGame()) return;
  if (typeof discardArtifactReturn === "function") discardArtifactReturn();
  stopArenaPolling();
  if (typeof stopGrowthRestTimer === "function") stopGrowthRestTimer();
  if (pageId === "page-arena") arenaTab = "ladder";
  document.querySelectorAll(".page").forEach(p =>
    p.classList.toggle("active", p.id === pageId));
  document.querySelectorAll(".nav-btn").forEach(b =>
    b.classList.toggle("active", b.dataset.nav === pageId));
  document.querySelectorAll(".nav-btn").forEach(button => {
    if (button.dataset.nav === pageId) button.setAttribute("aria-current", "page");
    else button.removeAttribute("aria-current");
  });
  document.body.classList.toggle("in-battle", pageId === "page-battle");  // 横屏战斗全屏态
  document.body.dataset.page = pageId;                                    // 供样式按页面微调
  const R = { "page-home": renderHome, "page-gacha": renderGacha,
              "page-squad": renderSquad, "page-stage": renderStage,
              "page-modes": renderModes, "page-growth": typeof renderGrowth === "function" ? renderGrowth : null,
              "page-bag": renderBag, "page-shop": renderShop, "page-arena": renderArena,
              "page-captain": typeof renderCaptain === "function" ? renderCaptain : null,
              "page-festival": typeof renderFestival === "function" ? renderFestival : null,
              "page-fleet": typeof renderFleet === "function" ? renderFleet : null,
              "page-mail": typeof renderHarborMail === "function" ? renderHarborMail : null }[pageId];
  if (R) R();
  if (pageId === "page-modes") refreshModes();
  renderTopbar();
}

/* ---------- 顶部资源栏 ---------- */
function renderTopbar() {
  if (!SAVE) return;
  if (typeof renderHarborMailBadge === "function") renderHarborMailBadge();
  if (typeof renderCaptainSummary === "function") renderCaptainSummary();
  document.getElementById("res-stamina").textContent = SAVE.stamina + "/" + SAVE.staminaMax;
  document.getElementById("res-token").textContent = SAVE.token;
  document.getElementById("res-gold").textContent = SAVE.gold;
  document.getElementById("res-multi").textContent = "x" + SAVE.multi + (SAVE.multiOn ? "（使用中）" : "");
  document.getElementById("res-budget").textContent =
    Math.max(0, TOKEN_BUDGET - SAVE.daily.tokenGained);
  const site = document.getElementById("res-site");
  if (site) site.textContent = (Store.serverOK && SITE_ACCOUNT)
    ? quotaToE(SITE_ACCOUNT.quota || 0) + " E"
    : Store.connecting ? "同步中" : "未登录";
}

/* ---------- 通用弹窗 ---------- */
/* 标题与右上角关闭按钮都放进素材自带的藏青标题带（弹窗框的上边 62px），
   内容包在 .mb-scroll 里单独滚动，保证再长的掉落列表也不会把框撑破。 */
function showModal(html) {
  const box = document.getElementById("modal-box");
  const tpl = document.createElement("div");
  tpl.innerHTML = html.trim();
  const title = tpl.querySelector("h3");
  const rest = document.createElement("div");
  rest.innerHTML = tpl.innerHTML;
  const restTitle = rest.querySelector("h3");
  if (restTitle) restTitle.remove();

  box.innerHTML = "";
  // 标题直接挂在框上（绝对定位到素材的藏青标题带），不随内容滚动
  if (title) box.appendChild(title);
  const close = document.createElement("button");
  close.className = "mb-close";
  close.type = "button";
  close.setAttribute("aria-label", "关闭");
  close.title = "关闭";
  close.addEventListener("click", closeModal);
  box.appendChild(close);

  const scroll = document.createElement("div");
  scroll.className = "mb-scroll";
  while (rest.firstChild) scroll.appendChild(rest.firstChild);
  box.appendChild(scroll);
  document.getElementById("modal-mask").classList.remove("hidden");
}
function closeModal(force) {
  if (force !== true && BAT && !BAT.abort && !BAT.settled && BAT.playbackComplete) {
    toast("结算尚未确认，请重试结算或重新同步");
    return;
  }
  document.getElementById("modal-mask").classList.add("hidden");
}
function showPendingAction(message) {
  if (!requireGame()) return;
  showModal(`<h3>操作尚未确认</h3><p>${esc(message)}<br>将使用原操作编号查询或重试，不会重复扣款。</p>
    <div class="modal-btns"><button class="gbtn gbtn-primary" onclick="retryGameAction()"><span>重试原操作</span></button></div>`);
}
async function retryGameAction() {
  if (!requireGame()) return;
  const result = await Store.retryPendingAction();
  if (!result) return;
  closeModal();
  if (battleRunning) renderTopbar();
  else nav(document.body.dataset.page || "page-home");
  if (result.room) { pickArenaTab("rooms"); }
  else if (result.queue) { await refreshArenaQueue(); }
  if (result.queue) { await Store.refresh(); renderTopbar(); }
  if (result.arena) showArenaResult(result.arena);
  if (result.ladder) {
    lastArenaResult = result.ladder; lastArenaEpoch = Store.sessionEpoch;
    watchArenaResult();
  }
  if (result.guild_battle?.report && !battleRunning) {
    const receipt = result.guild_battle;
    resumeBattle({id:"guild-retry-"+Date.now(),stage:"guild_raid",guild_battle:receipt,result:receipt.report,ready_at:Store.serverNow()+receipt.report.duration_ms});
  }
  toast("原操作已由数据库确认");
}

/* ---------- 轻提示 ---------- */
function toast(msg) {
  const t = document.createElement("div");
  t.textContent = msg;
  Object.assign(t.style, {
    position:"fixed", top:"56px", left:"50%", transform:"translateX(-50%)",
    background:"var(--panel2)", border:"1px solid var(--line)", color:"var(--text)",
    padding:"8px 14px", borderRadius:"6px", zIndex:"200", fontSize:"13px",
  });
  document.body.appendChild(t);
  setTimeout(() => t.remove(), 2200);
}

/* ---------- 主页 ---------- */
function renderHome() {
  if (!requireGame()) return;
  const pityLeft = GACHA.pitySSR - (SAVE.pity % GACHA.pitySSR);
  const owned = Object.values(SAVE.girls).filter(g => g.star > 0).length;
  const budgetLeft = Math.max(0, TOKEN_BUDGET - SAVE.daily.tokenGained);
  document.getElementById("home-info").innerHTML = `
    <div class="info-card"><div class="k">今日代币池剩余</div><div class="v">${budgetLeft}/${TOKEN_BUDGET}</div></div>
    <div class="info-card"><div class="k">SSR 保底</div><div class="v">还差 ${pityLeft} 抽</div></div>
    <div class="info-card"><div class="k">已解锁少女</div><div class="v">${owned}/${GIRLS.length}</div></div>
    <div class="info-card"><div class="k">双倍卡</div>
      <div class="v">x2 时 x${SAVE.multi}</div>
      <button class="ui-switch ${SAVE.multiOn ? "on" : ""}" onclick="toggleMulti()"
        aria-pressed="${SAVE.multiOn}">
        <img src="assets/ui/${SAVE.multiOn ? "toggle-on" : "toggle-off"}.webp" alt="">
      </button>
    </div>`;
  renderSiteAccount();
  if (typeof renderFestivalToday === "function") renderFestivalToday();
  if (typeof renderGrowthSummary === "function") renderGrowthSummary();
}

/* ---------- 站点账号 ---------- */
function renderSiteAccount() {
  const box = document.getElementById("site-account");
  if (!box) return;
  if (!Store.serverOK || !SITE_ACCOUNT) {
    let heading = "请登录站点";
    let note = "游戏需要站点登录，账号档案保存在数据库，无需另外绑定。";
    if (Store.connecting) {
      heading = "正在同步站点账号";
      note = "正在验证站点登录状态并读取数据库档案。";
    } else if (Store.lastError && Store.lastError !== "AUTH") {
      heading = "站点账号暂时无法同步";
      note = "档案同步失败，恢复连接后请重新验证；不会进行离线结算。";
    }
    box.innerHTML = `
      <div class="bind-card">
        <div class="bind-h">${heading}</div>
        <div class="bind-note">${note}</div>
        <div class="bind-row">
          <button class="gbtn gbtn-sm gbtn-primary" onclick="openSiteLogin()"><span>登录站点</span></button>
          <button class="gbtn gbtn-sm" onclick="refreshSite()" ${Store.connecting ? "disabled" : ""}><span>重新同步</span></button>
        </div>
      </div>`;
    return;
  }
  const p = Store.profile;
  const today = p && p.today ? p.today : null;
  box.innerHTML = `
    <div class="bind-card ok">
      <div class="bind-h"><span id="site-account-name"></span> · ${quotaToE(SITE_ACCOUNT.quota || 0)} E</div>
      <div class="bind-note">${today
        ? `今日产出 ${quotaToE(today.quota)} E · ${today.quota_limit == null ? "不设每日上限" : `上限 ${quotaToE(today.quota_limit)} E`} · 今日副本 ${today.runs}${today.run_limit == null ? " 次（不限次数）" : `/${today.run_limit} 次`}`
        : "已自动使用当前站点登录账号"}</div>
      <div class="bind-row">
        <button class="gbtn gbtn-sm gbtn-primary" onclick="openWarehouse()"><span>站点仓库${p && p.stored ? " · " + p.stored + " 张" : ""}</span></button>
        <button class="gbtn gbtn-sm" onclick="refreshSite()"><span>刷新余额</span></button>
      </div>
    </div>`;
  document.getElementById("site-account-name").textContent = SITE_ACCOUNT.name;
}

/* ---------- 站点仓库：查看持有卡片并激活 ---------- */
async function openWarehouse() {
  if (!requireGame()) return;
  const epoch = Store.sessionEpoch;
  showModal(`<h3>站点仓库</h3>
    <p class="modal-note">正在读取仓库…</p>
    <div class="modal-btns">
      <button class="gbtn gbtn-sm" onclick="closeModal()"><span>关闭</span></button>
    </div>`);
  const w = await Store.warehouse();
  if (epoch !== Store.sessionEpoch || !Store.ready()) return;
  if (!w) {
    showModal(`<h3>站点仓库</h3>
      <p class="modal-note">读取失败${Store.lastError === "AUTH" ? "：站点登录已失效，请登录后重新同步" : "，请先同步站点账号后重试"}</p>
      <div class="modal-btns">
        <button class="gbtn gbtn-sm" onclick="openWarehouse()"><span>重试</span></button>
        <button class="gbtn gbtn-sm" onclick="closeModal()"><span>关闭</span></button>
      </div>`);
    return;
  }
  const active = w.active;
  const cards = (w.cards || []).filter(c => c.status === "stored");
  const used = (w.cards || []).filter(c => c.status !== "stored");
  const activeHTML = active
    ? `<div class="wh-active">
        <div><div class="wh-k">生效中</div>
          <div class="wh-name">${active.name}<i>-${active.discount}%</i></div>
          <div class="wh-sub">到期 ${fmtClock(active.expire_at)}</div></div>
        <div class="wh-left">${fmtRemain(active.expire_at)}</div>
      </div>`
    : `<div class="wh-none">当前没有生效中的折扣卡。激活后，站点调用按卡面折扣与动态倍率相乘计费。</div>`;
  const cardsHTML = cards.length
    ? `<div class="wh-list">${cards.map(c => `
        <div class="wh-card">
          <div class="wh-name">${c.name}<i>-${c.discount}%</i></div>
          <div class="wh-sub">有效 ${c.duration_min} 分钟 · ${fmtClock(c.obtained_at)} 获得</div>
          <button class="gbtn gbtn-sm ${active ? "" : "gbtn-primary"}" ${active ? "disabled" : ""}
            onclick="activateCard('${c.id}')"><span>${active ? "已有生效" : "激活"}</span></button>
        </div>`).join("")}</div>`
    : `<div class="wh-none">仓库里还没有卡。深渊回廊掉率约 11%，普通本约 3%。</div>`;
  const usedHTML = used.length
    ? `<div class="wh-history"><div class="wh-k">使用记录</div>
        ${used.slice(0, 5).map(c => `<div class="wh-hrow"><span>${c.name}</span>
          <span>-${c.discount}%</span><span>${fmtClock(c.activated_at)}</span>
          <span>${c.status === "active" ? "生效中" : "已结束"}</span></div>`).join("")}</div>`
    : "";
  showModal(`<h3>站点仓库</h3>
    ${activeHTML}
    ${cardsHTML}
    ${usedHTML}
    <div class="modal-btns">
      <button class="gbtn gbtn-sm" onclick="window.open('${SITE.warehouseUrl}','_blank','noopener')"><span>站点仓库页</span></button>
      <button class="gbtn gbtn-sm" onclick="closeModal()"><span>关闭</span></button>
    </div>`);
}

async function activateCard(cardId) {
  if (!requireGame()) return;
  const epoch = Store.sessionEpoch;
  const r = await Store.activate(cardId);
  if (epoch !== Store.sessionEpoch || !Store.ready()) return;
  if (!r.ok) return toast("激活失败：" + (r.msg || "稍后再试"));
  toast(r.card.name + " 已生效");
  await Store.refresh();
  if (epoch !== Store.sessionEpoch || !Store.ready()) return;
  renderTopbar(); renderSiteAccount();
  openWarehouse();
}

/* 到期时间：月日 时:分；到期剩余：时分秒 */
function fmtClock(ms) {
  if (!ms) return "—";
  const d = new Date(ms), p = n => String(n).padStart(2, "0");
  return `${d.getMonth() + 1}月${d.getDate()}日 ${p(d.getHours())}:${p(d.getMinutes())}`;
}
function fmtRemain(ms) {
  const sec = Math.floor((ms - Date.now()) / 1000);
  if (sec <= 0) return "已结束";
  const h = Math.floor(sec / 3600), m = Math.floor((sec % 3600) / 60), s = sec % 60;
  return h > 0 ? `${h} 小时 ${m} 分` : m > 0 ? `${m} 分 ${s} 秒` : `${s} 秒`;
}

async function refreshSite() {
  if (Store.connecting) return;
  Store.accessExpiresAt = 0;
  const pending = Store.refresh();
  renderTopbar(); renderSiteAccount();
  const d = await pending;
  document.getElementById("auth-actions").classList.remove("hidden");
  renderTopbar(); renderSiteAccount();
  if (!d) return toast(Store.lastError === "AUTH" ? "请先登录站点" : "账号同步失败，请稍后重试");
  unlockGame();
  toast("余额 " + quotaToE(SITE_ACCOUNT.quota) + " E");
}

let lastBackgroundSync = -Infinity;
async function syncSiteAccount(background = false) {
  if (Store.connecting) return;
  if (background && performance.now() - lastBackgroundSync < 3000) return;
  if (background) lastBackgroundSync = performance.now();
  Store.accessExpiresAt = 0;
  const pending = Store.refresh();
  document.getElementById("auth-actions").classList.add("hidden");
  renderTopbar(); renderSiteAccount();
  const d = await pending;
  document.getElementById("auth-actions").classList.remove("hidden");
  if (d) unlockGame();
  renderTopbar(); renderSiteAccount();
}

function openSiteLogin() {
  window.open(SITE.loginUrl, "_blank", "noopener");
}
async function toggleMulti() {
  if (!requireGame()) return;
  if (!await Store.mutate("multi", { enabled: !SAVE.multiOn })) return;
  renderHome(); renderTopbar();
}

/* ---------- 召唤页 ---------- */
function cardHTML(g, isNew, extra) {
  const r = RARITY[g.rarity];
  return `<div class="card r-${g.rarity} ${isNew ? "new" : ""} ${extra||""}">
    <img src="${g.img}" alt="${g.name}" loading="lazy">
    <div class="name">${g.name} · ${ELEM[g.elem].name}系</div>
  </div>`;
}

function renderGacha() {
  if (!requireGame()) return;
  if (typeof renderArtifactPool === "function" && renderArtifactPool()) return;
  const pityDone = SAVE.pity % GACHA.pitySSR;
  const pityPct = Math.max(pityDone / GACHA.pitySSR, 0.04) * 100;
  document.getElementById("gacha-pity").innerHTML = `
    <div class="pity-row">
      <span>累计 ${SAVE.totalPull} 抽 · 距离 SSR 硬保底还差
        <b>${GACHA.pitySSR - pityDone}</b> 抽 · 招募券 ${SAVE.progression?.permits || 0} · 金币 ${SAVE.gold}</span>
      <span class="pity-n">${pityDone}/${GACHA.pitySSR}</span>
    </div>
    <div class="pity-bar"><i style="width:${pityPct}%"></i></div>`;
  document.getElementById("btn-gacha-1").disabled = (SAVE.progression?.permits || 0) < 1 || SAVE.gold < GACHA.cost1;
  document.getElementById("btn-gacha-10").disabled = (SAVE.progression?.permits || 0) < 10 || SAVE.gold < GACHA.cost10;
  document.getElementById("gacha-result").innerHTML = "";

  const pct = n => (n * 100).toFixed(1) + "%";
  const hist = (SAVE.history || []).slice(0, 6);
  document.getElementById("gacha-extra").innerHTML = `
    <div class="gx-bar">
      <span class="gx-k">出率</span>
      <span class="rr r-SSR">SSR ${pct(GACHA.weightSSR)}</span>
      <span class="rr r-SR">SR ${pct(GACHA.weightSR)}</span>
      <span class="rr r-R">R ${pct(GACHA.weightR)}</span>
      <span class="gx-k gx-hist-k">最近获得</span>
      <span class="gx-hist">
        ${hist.length ? hist.map(h => `<span class="hchip r-${h.r}">${h.n}</span>`).join("")
                      : `<span class="gx-empty">暂无记录</span>`}
      </span>
    </div><div class="growth-gacha-link"><span>招募券来自日常、周常和主线里程碑，不能用金币无限抽取。</span><button type="button" class="gbtn gbtn-sm" onclick="nav('page-growth')"><span>领取招募券</span></button></div>`;
}

async function doGacha(times) {
  if (!requireGame()) return;
  const outcome = await Store.mutate("gacha", { count: times });
  if (!outcome) return;
  const epoch = Store.sessionEpoch;
  const results = (outcome.results || []).map(r => ({
    g: GIRLS.find(g => g.id === r.girl_id), isNew: r.is_new, dup: r.dup, shards: r.shards,
  })).filter(r => r.g);
  renderGacha(); renderTopbar();
  // 召唤法阵特效 -> 翻牌揭晓
  const fx = document.getElementById("summon-fx");
  const box = document.getElementById("gacha-result");
  box.innerHTML = "";
  fx.classList.remove("hidden");
  const best = results.some(r => r.g.rarity === "SSR");
  setTimeout(() => {
    fx.classList.add("hidden");
    if (epoch !== Store.sessionEpoch || !Store.ready()) return;
    box.innerHTML = results.map((r, i) => `
      <div class="card r-${r.g.rarity} flip ${r.isNew ? "new" : ""}" style="animation-delay:${i * 70}ms">
        <img src="${r.g.img}" loading="lazy">
        ${r.dup ? `<span class="shard-n">碎片+${r.shards}</span>` : ""}
        <div class="name">${r.g.name}</div>
      </div>`).join("");
    toast(best ? "出金了！" : "召唤完成");
  }, 950);
}

/* ---------- 编队页 ---------- */
function powerOf(id) {
  const s = finalStats(id, true);
  const base = s.atk * 2.2 * (1 + s.cr * s.cd / 10000) + s.hp * 0.35 + s.def * 1.6 + s.spd * 3 + s.ehr * 2 + s.res * 2;
  const own = SAVE.girls[id];
  const weight = Store.profile?.progression?.power_weights?.[own?.branch] || 0.06;
  return Math.round(base * (1 + (own?.awakening || 0) * weight));
}
function squadBondLabels(ids) {
  const selected = ids.map(id => GIRLS.find(g => g.id === id)).filter(Boolean);
  const elems = {}, roles = {};
  selected.forEach(g => { elems[g.elem] = (elems[g.elem] || 0) + 1; roles[g.role] = (roles[g.role] || 0) + 1; });
  const labels = [];
  Object.entries(elems).filter(([, n]) => n >= 2).forEach(([id, n]) => labels.push(`${ELEM[id].name}系${n}人 · 攻击 +6%`));
  Object.entries(roles).filter(([, n]) => n >= 3).forEach(([role, n]) => labels.push(`${{tank:"守护",sdps:"单攻",adps:"群攻",heal:"治疗",support:"辅助"}[role] || role}${n}人 · 生命 +7%`));
  if (new Set(selected.map(g => g.elem)).size >= 4) labels.push("四象协奏 · 防御 +5%");
  return labels.length ? labels : ["编队两名同元素少女触发元素共鸣；三名同定位触发职能协奏"];
}

function codexSummary() {
  const flat = codexBonuses();
  return `生命 +${flat.hp} · 攻击 +${flat.atk} · 防御 +${flat.def}`;
}

function collectionCrewMarkup(bond, cardId) {
  const { spec, owned, percent } = bond, rules = cultivationRules();
  const members = spec.members.map(member => GIRLS.find(girl => girl.id === member));
  const stat = CULTIVATION_STATS[spec.stat], advanced = percent > 0 && members.every(girl => SAVE.girls[girl.id]?.star >= rules.bond_star);
  return `<article class="at-bond ${percent ? "is-active" : ""}" ${cardId ? "data-card-bond" : "data-album-bond"}="${spec.members.join('-')}"><div class="at-bond-title"><h4>${esc(spec.name)}</h4><em>${advanced ? "进阶已生效" : percent ? "已激活" : `待集齐 ${owned}/${members.length}`}</em></div><div class="at-partners">${members.map(girl => {
    const star = SAVE.girls[girl.id]?.star || 0, contents = `<img src="${girl.img}" alt="" loading="lazy"><span><b>${esc(girl.name)}</b><small>${star ? `${star} 星` : "未获得"}</small></span>`;
    return girl.id === cardId ? `<div class="collection-partner ${star ? "" : "is-missing"}">${contents}</div>` : `<button type="button" class="collection-partner ${star ? "" : "is-missing"}" aria-label="查看${esc(girl.name)}" onclick="openGirl('${girl.id}')">${contents}</button>`;
  }).join("")}</div><p><b>当前${stat} +${percent}%</b> · 集齐 +${spec.base}% · 全员 ${rules.bond_star} 星增至 +${spec.advanced}%</p></article>`;
}

function collectionAlbumMarkup() {
  const rules = cultivationRules();
  if (!rules) return "";
  const crews = rules.crews || rules.fellowships || rules.bonds, active = crews.filter(spec => spec.members.every(id => SAVE.girls[id]?.star > 0)).length;
  return `<details class="collection-album"><summary>收藏图鉴 · 羁绊 ${active}/${crews.length} <small>全员 ${codexSummary()}</small></summary><p>获得同组全部整卡，即为组员激活羁绊；碎片不计，也不要求上阵。</p><div class="at-squad-bonds">${SAVE.squad.map(id => {
    const girl = GIRLS.find(entry => entry.id === id), bonds = collectionBonds(id);
    return `<button type="button" class="collection-partner" onclick="${typeof openWorkshop === "function" && workshopRules() ? `openWorkshop('${id}','bonds')` : `openGirl('${id}')`}"><img src="${girl.img}" alt="" loading="lazy"><span><b>${esc(girl.name)}</b><small>羁绊 ${bonds.filter(bond => bond.percent).length}/${bonds.length}</small></span></button>`;
  }).join("")}</div></details>`;
}
const SQUAD_LIMIT = 6;
let squadSelectedId = null;
let squadSelectionEpoch = null;
let squadMutation = null;
let squadDrag = null;
let squadClickBlockedUntil = 0;
function renderSquad() {
  if (!requireGame()) return;
  cancelSquadDrag();
  if (squadSelectionEpoch !== Store.sessionEpoch) { squadSelectedId = null; squadSelectionEpoch = Store.sessionEpoch; }
  if (!SAVE.squad.includes(squadSelectedId)) squadSelectedId = null;
  const slots = document.getElementById("squad-slots");
  let power = 0;
  const agg = { hp: 0, atk: 0, def: 0, spd: 0 };
  slots.innerHTML = Array.from({ length: SQUAD_LIMIT }, (_, i) => {
    const rank = i < 3 ? "前排" : "后排";
    const id = SAVE.squad[i];
    if (!id) return `<button type="button" class="team-slot empty" data-squad-slot="${i}" onclick="selectSquadSlot(${i})" aria-label="${rank}空位 ${i + 1}，将已选队员移至队尾">
        <img class="slot-add" src="assets/ui/slot-add.webp" alt="">
        <span class="slot-lb">${rank} ${i + 1}</span></button>`;
    const g = GIRLS.find(x => x.id === id);
    const own = SAVE.girls[id];
    power += powerOf(id);
    const s = finalStats(id, true);
    agg.hp += s.hp; agg.atk += s.atk; agg.def += s.def; agg.spd += s.spd;
    return `<div class="team-slot filled r-${g.rarity} ${squadSelectedId === id ? "slot-selected" : ""}" data-squad-slot="${i}" ${typeof trainingFrame === "function" ? trainingFrame(own) : ""}>
      <button type="button" class="squad-pick" draggable="true" data-squad-id="${id}" onclick="selectSquadSlot(${i})" aria-pressed="${squadSelectedId === id}" aria-label="${rank}${i + 1} ${esc(g.name)}，拖动或点击换位">
      <span class="ts-idx">${rank} ${i + 1}</span>
      <img src="${g.img}" loading="lazy" draggable="false" alt="">
      <div class="tn-name">${g.name}</div>
      <div class="tn-sub"><span>Lv${own.level} · ${own.star}星</span>${typeof trainingGradeTag === "function" ? trainingGradeTag(own) : ""}</div>
      </button><button type="button" class="ts-x" onclick="unslot(${i})" aria-label="移出${esc(g.name)}">×</button>
    </div>`;
  }).join("");
  bindSquadDrag(slots);
  setSquadBusy(squadMutation?.epoch === Store.sessionEpoch);
  document.getElementById("team-count").textContent = SAVE.squad.length + "/" + SQUAD_LIMIT;
  const selection = document.getElementById("squad-selection");
  if (selection) selection.textContent = squadSelectedId ? "已选中「" + GIRLS.find(g => g.id === squadSelectedId).name + "」，再点队友交换，点空位移至队尾；再点自身取消。" : "拖动队员交换站位，也可依次点击两位队员；拖到空位会移至队尾。";
  document.getElementById("team-power").textContent = "队伍战力 " + (Number.isSafeInteger(SAVE.power) ? SAVE.power : power).toLocaleString();
  const n = SAVE.squad.length || 1;
  document.getElementById("team-stats").innerHTML = `
    <span>生命 <b>${agg.hp.toLocaleString()}</b></span>
    <span>攻击 <b>${agg.atk.toLocaleString()}</b></span>
    <span>防御 <b>${agg.def.toLocaleString()}</b></span>
    <span>均速 <b>${Math.round(agg.spd / n)}</b></span>
    <div class="bond-line"><b>上阵羁绊</b>${squadBondLabels(SAVE.squad).map(x => `<span>${esc(x)}</span>`).join("")}</div>${collectionAlbumMarkup()}`;

  // 列表：图鉴全量展示（未获得置灰），在队优先，其次已拥有、稀有度、等级
  const order = { SSR: 0, SR: 1, R: 2 };
  const ids = GIRLS.map(g => g.id).sort((a, b) => {
    const ia = SAVE.squad.indexOf(a), ib = SAVE.squad.indexOf(b);
    const ina = ia >= 0, inb = ib >= 0;
    if (ina !== inb) return ina ? -1 : 1;
    if (ina && inb) return ia - ib;
    const oa = SAVE.girls[a]?.star > 0 ? 1 : 0, ob = SAVE.girls[b]?.star > 0 ? 1 : 0;
    if (oa !== ob) return ob - oa;
    const ga = GIRLS.find(g => g.id === a), gb = GIRLS.find(g => g.id === b);
    return order[ga.rarity] - order[gb.rarity]
      || (SAVE.girls[b] ? SAVE.girls[b].level : 0) - (SAVE.girls[a] ? SAVE.girls[a].level : 0);
  });
  document.getElementById("owned-title").textContent =
    "图鉴 " + Object.values(SAVE.girls).filter(g => g.star > 0).length + "/" + GIRLS.length +
    " · 在队 " + SAVE.squad.length + " 位";
  document.getElementById("owned-list").innerHTML = ids.map(id => {
    const g = GIRLS.find(x => x.id === id);
    const own = SAVE.girls[id];
    if (!own || own.star <= 0) return `<div class="card r-${g.rarity} card-locked" data-id="${id}" title="尚未获得">
      <img src="${g.img}" loading="lazy">
      <span class="shard-n">${own?.shards > 0 ? "碎片 " + own.shards : ""}</span>
      <div class="name">${g.name}<i>未获得</i></div>
    </div>`;
    const slot = SAVE.squad.indexOf(id);          // -1 = 未上阵
    const inSquad = slot >= 0;
    return `<div class="card r-${g.rarity} ${inSquad ? "in-team" : ""}" data-id="${id}" ${typeof trainingFrame === "function" ? trainingFrame(own) : ""} data-prestige-card="${own.star >= 12 ? "crown" : own.star >= 9 ? "crimson" : own.star >= 6 ? "gold" : ""}">
      <img src="${g.img}" loading="lazy">
      ${inSquad ? `<span class="team-flag">出战 ${slot + 1}</span>` : ""}
      <span class="shard-n">${own.shards > 0 ? "碎片 " + own.shards : ""}</span>
      ${typeof atelierCrest === "function" ? atelierCrest(own.star) : ""}
      <div class="name">${g.name} ${typeof trainingGradeTag === "function" ? trainingGradeTag(own) : ""}<i>Lv${own.level} · ${own.star}星${own.talent ? ` · ${TALENTS[own.talent]?.name || "天赋"}` : ""}</i></div>
      <span class="quick-toggle" title="${inSquad ? "移出队伍" : "加入队伍"}">${inSquad ? "−" : "+"}</span>
    </div>`;
  }).join("");

  // 事件委托：点角标快速上下阵，点卡片打开详情
  document.querySelectorAll("#owned-list .card").forEach(card => {
    card.addEventListener("click", e => {
      if (e.target.closest(".quick-toggle")) { toggleSquad(card.dataset.id); return; }
      openGirl(card.dataset.id);
    });
  });
}
function setSquadBusy(busy) {
  const slots = document.getElementById("squad-slots");
  if (!slots) return;
  slots.setAttribute("aria-busy", String(busy));
  slots.querySelectorAll("button").forEach(button => { button.disabled = busy; });
}
async function persistSquad(squad, epoch = Store.sessionEpoch) {
  if (!requireGame() || epoch !== Store.sessionEpoch || squadMutation?.epoch === epoch) return false;
  if (squad.length > SQUAD_LIMIT || new Set(squad).size !== squad.length || squad.some(id => !SAVE.girls[id]?.star)) return false;
  if (squad.join(",") === SAVE.squad.join(",")) return false;
  const mutation = squadMutation = {epoch};
  setSquadBusy(true);
  try {
    const result = await Store.mutate("squad", {squad});
    if (!result || epoch !== Store.sessionEpoch || !Store.ready()) return false;
    squadSelectedId = null;
    renderSquad(); renderTopbar();
    return true;
  } finally {
    if (squadMutation === mutation) { squadMutation = null; setSquadBusy(false); }
  }
}
async function moveSquadCard(id, target, epoch = Store.sessionEpoch, original = SAVE.squad.join(",")) {
  if (!requireGame() || epoch !== Store.sessionEpoch || original !== SAVE.squad.join(",") || !Number.isInteger(target) || target < 0 || target >= SQUAD_LIMIT) return;
  const squad = [...SAVE.squad], from = squad.indexOf(id);
  if (from < 0 || from === target) return;
  if (target < squad.length) [squad[from], squad[target]] = [squad[target], squad[from]];
  else { squad.splice(from, 1); squad.push(id); }
  await persistSquad(squad, epoch);
}
function cancelSquadDrag() {
  const drag = squadDrag;
  squadDrag = null;
  if (drag?.active) squadClickBlockedUntil = performance.now() + 400;
  drag?.ghost?.remove();
  document.querySelectorAll("#squad-slots .slot-dragging, #squad-slots .slot-drop-target").forEach(slot => slot.classList.remove("slot-dragging", "slot-drop-target"));
  if (drag?.pointerId != null) {
    try { if (drag.button.hasPointerCapture?.(drag.pointerId)) drag.button.releasePointerCapture(drag.pointerId); } catch (_) {}
  }
}
function validSquadDrag() {
  return squadDrag && Store.ready() && squadDrag.epoch === Store.sessionEpoch && squadDrag.original === SAVE.squad.join(",") && document.body.dataset.page === "page-squad" && squadMutation?.epoch !== Store.sessionEpoch;
}
function squadDropTarget(element) {
  const slot = element?.closest?.("#squad-slots [data-squad-slot]");
  return slot && Number.isInteger(Number(slot.dataset.squadSlot)) ? slot : null;
}
function markSquadDrop(slot) {
  document.querySelectorAll("#squad-slots .slot-drop-target").forEach(item => item.classList.remove("slot-drop-target"));
  if (slot && slot !== squadDrag?.button.parentElement) slot.classList.add("slot-drop-target");
}
function bindSquadDrag(slots) {
  if (slots.dataset.dragBound) return;
  slots.dataset.dragBound = "true";
  const start = (button, kind, event) => {
    if (!button || button.disabled || !Store.ready() || squadMutation?.epoch === Store.sessionEpoch || !SAVE.squad.includes(button.dataset.squadId)) return false;
    cancelSquadDrag();
    squadDrag = {kind, button, id:button.dataset.squadId, epoch:Store.sessionEpoch, original:SAVE.squad.join(","), active:kind === "native", x:event.clientX, y:event.clientY};
    return true;
  };
  slots.addEventListener("click", event => {
    if (performance.now() >= squadClickBlockedUntil) return;
    event.preventDefault(); event.stopImmediatePropagation();
  }, true);
  slots.addEventListener("dragstart", event => {
    if (!start(event.target.closest(".squad-pick"), "native", event)) { event.preventDefault(); return; }
    squadDrag.button.parentElement.classList.add("slot-dragging");
    if (event.dataTransfer) { event.dataTransfer.effectAllowed = "move"; event.dataTransfer.setData("text/plain", squadDrag.id); }
  });
  slots.addEventListener("dragover", event => {
    if (!validSquadDrag() || squadDrag.kind !== "native") return;
    const target = squadDropTarget(event.target);
    if (!target) return;
    event.preventDefault();
    if (event.dataTransfer) event.dataTransfer.dropEffect = "move";
    markSquadDrop(target);
  });
  slots.addEventListener("dragleave", event => { if (!slots.contains(event.relatedTarget)) markSquadDrop(null); });
  slots.addEventListener("drop", event => {
    if (!validSquadDrag() || squadDrag.kind !== "native") { cancelSquadDrag(); return; }
    event.preventDefault();
    const drag = squadDrag, target = squadDropTarget(event.target);
    cancelSquadDrag();
    if (target) void moveSquadCard(drag.id, Number(target.dataset.squadSlot), drag.epoch, drag.original);
  });
  slots.addEventListener("dragend", cancelSquadDrag);
  slots.addEventListener("pointerdown", event => {
    if (!["touch", "pen"].includes(event.pointerType) || event.isPrimary === false || event.button !== 0) return;
    if (!start(event.target.closest(".squad-pick"), "pointer", event)) return;
    squadDrag.pointerId = event.pointerId;
    try { squadDrag.button.setPointerCapture?.(event.pointerId); } catch (_) {}
  });
  document.addEventListener("pointermove", event => {
    if (squadDrag?.kind !== "pointer" || squadDrag.pointerId !== event.pointerId) return;
    if (!validSquadDrag()) { cancelSquadDrag(); return; }
    const drag = squadDrag;
    if (!drag.active && Math.hypot(event.clientX-drag.x, event.clientY-drag.y) < 8) return;
    if (!drag.active) {
      drag.active = true;
      const source = drag.button.parentElement, rect = source.getBoundingClientRect();
      drag.offsetX = drag.x-rect.left; drag.offsetY = drag.y-rect.top;
      drag.ghost = source.cloneNode(true);
      drag.ghost.classList.add("squad-drag-ghost");
      drag.ghost.setAttribute("aria-hidden", "true");
      drag.ghost.querySelectorAll("button").forEach(button => { button.disabled = true; button.removeAttribute("onclick"); button.draggable = false; });
      drag.ghost.style.width = rect.width + "px"; drag.ghost.style.height = rect.height + "px";
      document.body.append(drag.ghost);
      source.classList.add("slot-dragging");
    }
    event.preventDefault();
    drag.ghost.style.left = (event.clientX-drag.offsetX) + "px";
    drag.ghost.style.top = (event.clientY-drag.offsetY) + "px";
    markSquadDrop(squadDropTarget(document.elementFromPoint(event.clientX,event.clientY)));
  }, {passive:false});
  document.addEventListener("pointerup", event => {
    if (squadDrag?.kind !== "pointer" || squadDrag.pointerId !== event.pointerId) return;
    const drag = squadDrag, valid = validSquadDrag();
    const target = valid && drag.active ? squadDropTarget(document.elementFromPoint(event.clientX,event.clientY)) : null;
    if (drag.active) event.preventDefault();
    cancelSquadDrag();
    if (target) void moveSquadCard(drag.id, Number(target.dataset.squadSlot), drag.epoch, drag.original);
  });
  document.addEventListener("pointercancel", event => { if (squadDrag?.pointerId === event.pointerId) cancelSquadDrag(); });
  slots.addEventListener("lostpointercapture", event => { if (squadDrag?.pointerId === event.pointerId) cancelSquadDrag(); });
  document.addEventListener("keydown", event => { if (event.key === "Escape") cancelSquadDrag(); });
  window.addEventListener("blur", cancelSquadDrag);
}
async function toggleSquad(id) {
  if (!requireGame()) return;
  if (!SAVE.girls[id] || SAVE.girls[id].star <= 0) return toast("该少女尚未解锁");
  let squad;
  if (SAVE.squad.includes(id)) squad = SAVE.squad.filter(x => x !== id);
  else {
    if (SAVE.squad.length >= SQUAD_LIMIT) return toast("最多上阵 6 人，先移出一名");
    squad = [...SAVE.squad, id];
  }
  await persistSquad(squad);
}
async function selectSquadSlot(index) {
  if (!requireGame() || squadMutation?.epoch === Store.sessionEpoch || !Number.isInteger(index) || index < 0 || index >= SQUAD_LIMIT) return;
  if (squadSelectionEpoch !== Store.sessionEpoch) { squadSelectedId = null; squadSelectionEpoch = Store.sessionEpoch; }
  const id = SAVE.squad[index];
  if (!id) {
    if (squadSelectedId) return moveSquadCard(squadSelectedId, index);
    return toast("从下方图鉴点 + 上阵，或拖动队员到这里移至队尾");
  }
  if (!squadSelectedId || !SAVE.squad.includes(squadSelectedId)) squadSelectedId = id;
  else if (squadSelectedId === id) squadSelectedId = null;
  else {
    return moveSquadCard(squadSelectedId, index);
  }
  renderSquad(); renderTopbar();
}
async function unslot(i) {
  if (!requireGame()) return;
  const squad = SAVE.squad.filter((_, index) => index !== i);
  await persistSquad(squad);
}

function shardReserve(g, own) {
  if (!own || own.star < 1) return RARITY[g.rarity].shardToUnlock;
  return growthStarCost(g, own.star + 1);
}
function recyclableShards(girl, own) {
  const surplus = Math.max(0, (own?.shards || 0) - shardReserve(girl, own));
  const rate = cultivationRules()?.shards?.per_ore[girl.rarity] || 1;
  return surplus - surplus % rate;
}

async function exchangeLegacyDust(itemId) {
  if (!requireGame() || !["shard_dust", "shard_dust_r", "shard_dust_sr", "shard_dust_ssr"].includes(itemId)) return;
  const cost = cultivationRules()?.shards?.legacy_dust_per_ore;
  if (!cost || (SAVE.mats[itemId] || 0) < cost) return;
  const epoch = Store.sessionEpoch, result = await Store.mutate("salvage_dust", { item_id: itemId });
  if (!result || epoch !== Store.sessionEpoch || !Store.ready()) return;
  if (document.body.dataset.page === "page-bag") renderBag();
  toast((result.drops || []).join(" · ") || "旧尘回收已确认");
}

function confirmShardDecompose(id, count) {
  const g = GIRLS.find(x => x.id === id);
  if (!g || !Number.isSafeInteger(count) || count < 1) return;
  const rate = cultivationRules()?.shards?.per_ore[g.rarity];
  if (rate) count -= count % rate;
  if (count < 1 || count > recyclableShards(g, SAVE.girls[id])) return;
  showModal(`<h3>分解少女碎片</h3>
    <p>分解 ${esc(g.name)} 的 ${count} 枚多余专属碎片，${rate ? `获得 ${count / rate} 枚星锻矿（每 ${rate} 碎片换 1 矿）。星锻矿不能替代任何角色的升星碎片。` : `获得 ${count} 个${g.rarity}共鸣尘。`}</p>
    <p class="modal-note">系统会保留解锁或下一次升星所需的碎片。</p>
    <div class="modal-btns">
      <button class="gbtn gbtn-sm" onclick="openGirl('${id}')"><span>取消</span></button>
      <button class="gbtn gbtn-sm gbtn-primary" onclick="decomposeShards('${id}',${count})"><span>确认分解</span></button>
    </div>`);
}
async function decomposeShards(id, count) {
  if (!requireGame()) return;
  const girl = GIRLS.find(entry => entry.id === id), rate = cultivationRules()?.shards?.per_ore[girl?.rarity] || 1;
  if (!girl || !Number.isSafeInteger(count) || count < 1 || count % rate || count > recyclableShards(girl, SAVE.girls[id])) return;
  const epoch = Store.sessionEpoch;
  const result = await Store.mutate("shard_decompose", { girl_id: id, count });
  if (!result?.decomposition || epoch !== Store.sessionEpoch || !Store.ready()) return;
  const page = document.body.dataset.page || "page-bag";
  closeModal();
  nav(page);
  const receipt = result.decomposition;
  toast(`分解 ${receipt.shards} 个碎片，获得 ${receipt.item_id === "forge_ore" ? receipt.quantity + " 星锻矿" : receipt.dust + " " + (receipt.rarity || "") + "共鸣尘"}`);
}

function gemSocketMarkup(id, own, matKey) {
  const opened = own.sockets || 0;
  const equipped = own.gems || [];
  const inventory = SAVE.gem_inventory || {};
  const matName = MATS[matKey].name;
  const slots = [0, 1, 2].map(index => {
    const slot = index + 1;
    if (index < opened) {
      const gemId = equipped[index];
      const gem = gemId && GEMS[gemId];
      return `<div class="gem-slot ${gem ? "filled" : "empty"}">
        <span class="gem-orb" style="--gem-color:${gem ? gem.color : "#9aa5b5"}">${gem ? "✦" : "◇"}</span>
        <div class="gem-slot-info"><b>${gem ? gem.name : `第 ${slot} 孔 · 空`}</b><span>${gem ? gem.effect : "选择一颗宝石镶嵌"}</span></div>
        ${gem ? `<button class="gbtn gbtn-sm" onclick="removeGem('${id}',${slot})"><span>拆下</span></button>` : `<button class="gbtn gbtn-sm gbtn-primary" onclick="openGemPicker('${id}',${slot})" ${Object.values(inventory).some(n => n > 0) ? "" : "disabled"}><span>镶嵌</span></button>`}
      </div>`;
    }
    if (index === opened) {
      const cost = GEM_SOCKET_COST[index];
      const enough = SAVE.gold >= cost.gold && (SAVE.mats[matKey] || 0) >= cost.mat;
      return `<div class="gem-slot locked">
        <span class="gem-orb" style="--gem-color:#9aa5b5">◇</span>
        <div class="gem-slot-info"><b>第 ${slot} 孔 · 未开孔</b><span>${cost.gold} 金币 + ${matName} ${cost.mat}</span></div>
        <button class="gbtn gbtn-sm gbtn-gold" onclick="openGemSocket('${id}',${slot})" ${enough ? "" : "disabled"}><span>开孔</span></button>
      </div>`;
    }
    return `<div class="gem-slot locked"><span class="gem-orb" style="--gem-color:#9aa5b5">◇</span><div class="gem-slot-info"><b>第 ${slot} 孔 · 未开孔</b><span>请先开启前一孔</span></div></div>`;
  }).join("");
  return `<section class="gem-panel">
    <div class="gem-panel-head"><h4>共鸣宝石 · ${opened}/3 孔</h4><span>副本胜利有 18% 概率掉落 1 颗</span></div>
    <div class="gem-slots">${slots}</div>
  </section>`;
}

function openGemPicker(id, slot) {
  if (!requireGame()) return;
  const inventory = SAVE.gem_inventory || {};
  const options = Object.entries(GEMS).filter(([gemId]) => inventory[gemId] > 0);
  if (!options.length) return toast("还没有宝石，通关副本有概率获得");
  showModal(`<h3>选择第 ${slot} 孔的宝石</h3>
    <p class="modal-note">宝石可以免费拆下，拆下后会放回背包。</p>
    <div class="gem-picker">${options.map(([gemId, gem]) => `<button class="gem-choice" onclick="equipGem('${id}',${slot},'${gemId}')">
      <span class="gem-orb" style="--gem-color:${gem.color}">✦</span><b>${gem.name} ×${inventory[gemId]}</b><small>${gem.effect}</small>
    </button>`).join("")}</div>
    <div class="modal-btns"><button class="gbtn gbtn-sm" onclick="openGirl('${id}')"><span>返回角色</span></button></div>`);
}

async function openGemSocket(id, slot) {
  if (!requireGame()) return;
  if (!await Store.mutate("gem_socket", { girl_id: id, count: slot })) return;
  openGirl(id); renderTopbar();
  toast(`第 ${slot} 孔已开启`);
}

async function equipGem(id, slot, gemId) {
  if (!requireGame()) return;
  if (!await Store.mutate("gem_equip", { girl_id: id, count: slot, item_id: gemId })) return;
  openGirl(id); renderTopbar();
  toast(`已镶嵌${GEMS[gemId].name}`);
}

async function removeGem(id, slot) {
  if (!requireGame()) return;
  if (!await Store.mutate("gem_remove", { girl_id: id, count: slot })) return;
  openGirl(id); renderTopbar();
  toast("宝石已放回背包");
}

function talentMarkup(id, own, matKey) {
  const rank = Math.min(3, Math.max(0, own.star - 2));
  if (own.star < 3) return `<section class="talent-panel"><div class="mode-card-head"><b>角色天赋</b><span>升至 3 星解锁</span></div><p>选择一条专精路线，升星会自动强化天赋阶级。</p></section>`;
  const cost = own.talent ? { gold: 1000, mat: 10 } : { gold: 500, mat: 5 };
  const enough = SAVE.gold >= cost.gold && (SAVE.mats[matKey] || 0) >= cost.mat;
  return `<section class="talent-panel"><div class="mode-card-head"><b>角色天赋 · ${own.talent ? `${TALENTS[own.talent].name} ${rank}/3 阶` : "尚未选择"}</b><span>3～5星强化专精，最高三阶</span></div>
    <div class="talent-branches">${Object.entries(TALENTS).map(([key, talent]) => `<button type="button" class="talent-branch ${own.talent === key ? "selected" : ""}" style="--talent:${talent.color}" ${own.talent === key || !enough ? "disabled" : ""} onclick="chooseGirlTalent('${id}','${key}')"><b>${talent.name} · ${talent.style}</b><small>${talent.effect}</small><small class="build-tradeoff">${talent.tradeoff}</small><em>${own.talent === key ? `已激活 ${rank}/3` : `选择 ${cost.gold} 金币 + ${MATS[matKey].name} ${cost.mat}`}</em></button>`).join("")}</div></section>`;
}

function sigilHolder(id) {
  return Object.keys(SAVE.girls).find(girlId => SAVE.girls[girlId]?.sigil === id);
}
function ownedSigils() {
  return Object.values(SAVE.sigils || {}).filter(s => s && /^sigil_[0-9a-f]{32}$/.test(s.id) && SIGIL_FAMILIES[s.family] && SIGIL_QUALITY[s.quality]);
}
function sigilName(sigil) {
  return `${SIGIL_QUALITY[sigil.quality]} · ${SIGIL_FAMILIES[sigil.family].name}`;
}
function sigilAffixMarkup(sigil) {
  return Object.entries(sigil.affixes || {}).filter(([id, value]) => SIGIL_STATS[id] && Number.isSafeInteger(value)).map(([id, value]) => `<span>${SIGIL_STATS[id].name} <b>+${value}${SIGIL_STATS[id].unit}</b></span>`).join("");
}
function sigilCard(sigil, action) {
  const family = SIGIL_FAMILIES[sigil.family];
  const holder = sigilHolder(sigil.id);
  const holderName = holder && GIRLS.find(g => g.id === holder)?.name;
  const emblem = typeof atelierIcon === "function" && Store.profile?.items ? atelierIcon(`sigil_${sigil.family}`,({1:"white",2:"blue",3:"purple"})[sigil.quality],"seal") : '<span class="sigil-mark">◇</span>';
  return `<article class="sigil-card quality-${sigil.quality}" style="--sigil:${family.color}"><div class="sigil-heading">${emblem}<div><small>${SIGIL_QUALITY[sigil.quality]} · 随机双词条</small><h4>${family.name}</h4></div></div><div class="sigil-affixes">${sigilAffixMarkup(sigil)}</div><p>${family.effect}</p>${holder ? `<span class="sigil-holder">${esc(holderName || "少女")} 已装备</span>` : action}${SAVE.progression ? `<button type="button" class="gbtn gbtn-sm sigil-refine" onclick="openSigilRefine('${sigil.id}')"><span>定向一条属性</span></button>` : ""}</article>`;
}
function sigilEquipmentMarkup(id, own) {
  const sigil = own.sigil && SAVE.sigils?.[own.sigil];
  const valid = sigil && SIGIL_FAMILIES[sigil.family] && SIGIL_QUALITY[sigil.quality];
  return `<section class="sigil-panel"><div class="mode-card-head"><b>灵印 · 流派核心</b><span>每位少女1枚，双词条随机</span></div>${valid ? sigilCard(sigil, "") : '<p class="mode-note">未装备灵印。六系裂隙掉落，背包可打造史诗灵印。</p>'}<div class="modal-btns"><button class="gbtn gbtn-sm gbtn-primary" onclick="openSigilPicker('${id}')"><span>${valid ? "更换灵印" : "选择灵印"}</span></button>${valid ? `<button class="gbtn gbtn-sm" onclick="removeSigil('${id}')"><span>拆下</span></button>` : ""}<button class="gbtn gbtn-sm" onclick="closeModal();nav('page-bag')"><span>灵印背包</span></button></div></section>`;
}
function openSigilPicker(girlId) {
  if (!requireGame() || !SAVE.girls[girlId]?.star) return;
  const available = ownedSigils().sort((a, b) => b.quality - a.quality);
  showModal(`<h3>选择灵印</h3><p class="mode-note">可替换当前灵印，旧灵印保留。其他少女装备中的灵印需先拆下。</p><div class="sigil-grid">${available.length ? available.map(sigil => sigilCard(sigil, `<button class="gbtn gbtn-sm gbtn-primary" onclick="confirmSigilEquip('${girlId}','${sigil.id}')"><span>装备</span></button>`)).join("") : '<p class="social-empty">还没有灵印，去六系裂隙收集精华与灵印。</p>'}</div><div class="modal-btns"><button class="gbtn gbtn-sm" onclick="openGirl('${girlId}')"><span>返回角色</span></button></div>`);
}
async function confirmSigilEquip(girlId, sigilId) {
  if (!requireGame()) return;
  const epoch = Store.sessionEpoch;
  if (!await Store.mutate("sigil_equip", { girl_id: girlId, item_id: sigilId }) || epoch !== Store.sessionEpoch) return;
  openGirl(girlId); renderTopbar();
  if (document.body.dataset.page === "page-squad") renderSquad();
  toast("灵印已装备，属性与被动已进入实战");
}
async function removeSigil(girlId) {
  if (!requireGame()) return;
  const epoch = Store.sessionEpoch;
  if (!await Store.mutate("sigil_remove", { girl_id: girlId }) || epoch !== Store.sessionEpoch) return;
  openGirl(girlId); renderTopbar();
  if (document.body.dataset.page === "page-squad") renderSquad();
}
function confirmSigilSalvage(id) {
  if (!requireGame()) return;
  const sigil = ownedSigils().find(s => s.id === id);
  if (!sigil || sigilHolder(id)) return;
  showModal(`<h3>分解${esc(sigilName(sigil))}？</h3><p>获得 ${SIGIL_DUST[sigil.quality]} 灵印粉尘。分解后原词条不可找回，已装备的灵印不会被分解。</p><div class="modal-btns"><button class="gbtn gbtn-sm" onclick="closeModal()"><span>取消</span></button><button class="gbtn gbtn-sm gbtn-primary" onclick="salvageSigils('${id}')"><span>确认分解</span></button></div>`);
}
function confirmLowSigilSalvage() {
  if (!requireGame()) return;
  const amount = ownedSigils().filter(s => s.quality === 1 && !sigilHolder(s.id)).length;
  if (!amount) return toast("没有可分解的未装备普通灵印");
  showModal(`<h3>分解未装备普通灵印？</h3><p>当前 ${amount} 枚可获得 ${amount * 2} 粉尘。已装备、稀有与史诗灵印均保留。</p><div class="modal-btns"><button class="gbtn gbtn-sm" onclick="closeModal()"><span>取消</span></button><button class="gbtn gbtn-sm gbtn-primary" onclick="salvageSigils()"><span>确认分解</span></button></div>`);
}
async function salvageSigils(id) {
  const epoch = Store.sessionEpoch;
  const result = await Store.mutate(id ? "sigil_salvage" : "sigil_salvage_low", id ? { item_id: id } : {});
  if (!result || epoch !== Store.sessionEpoch) return;
  closeModal(); nav("page-bag");
  toast((result.drops || []).join(" · ") || "灵印已分解");
}
function confirmSigilCraft(familyId) {
  if (!requireGame() || !SIGIL_FAMILIES[familyId]) return;
  const family = SIGIL_FAMILIES[familyId];
  showModal(`<h3>打造史诗${family.name}？</h3><p>${family.effect}。固定为史诗品质，随机生成两种不同词条，数值由服务器生成。</p><p>消耗 ${family.mat} 40、灵印粉尘15、金币800，不扣站点余额。</p><div class="modal-btns"><button class="gbtn gbtn-sm" onclick="closeModal()"><span>取消</span></button><button class="gbtn gbtn-sm gbtn-primary" onclick="craftSigil('${familyId}')"><span>确认打造</span></button></div>`);
}
async function craftSigil(familyId) {
  const epoch = Store.sessionEpoch;
  const result = await Store.mutate("sigil_craft", { item_id: familyId });
  if (!result || epoch !== Store.sessionEpoch) return;
  closeModal(); nav("page-bag");
  toast((result.drops || []).join(" · ") || "史诗灵印已打造");
}
function sigilInventoryMarkup() {
  const sigils = ownedSigils().sort((a, b) => b.quality - a.quality || a.family.localeCompare(b.family));
  const capacity = Store.profile?.run_rules?.sigil_capacity || 160;
  const low = sigils.filter(s => s.quality === 1 && !sigilHolder(s.id)).length;
  const recipes = Object.entries(SIGIL_FAMILIES).map(([id, family]) => {
    const essence = SAVE.mats["essence_" + id] || 0;
    const canCraft = essence >= 40 && (SAVE.mats.sigil_dust || 0) >= 15 && SAVE.gold >= 800 && sigils.length < capacity;
    return `<div class="sigil-recipe" style="--sigil:${family.color}"><b>${family.name}</b><small>${family.effect}</small><span>${family.mat} ${essence}/40</span><button class="gbtn gbtn-sm" onclick="confirmSigilCraft('${id}')" ${canCraft ? "" : "disabled"}><span>打造史诗</span></button></div>`;
  }).join("");
  return `<section class="sigil-workshop"><div class="mode-card-head"><div><p class="social-kicker">六系核心 · 随机双词条</p><h3>灵印背包 ${sigils.length}/${capacity}</h3></div><button class="gbtn gbtn-sm" onclick="confirmLowSigilSalvage()" ${low ? "" : "disabled"}><span>分解未装备普通灵印</span></button></div><p class="mode-note">每枚有固定流派被动和两条随机属性。每位少女只装备一枚；拆下不消耗。打造额外需要粉尘15、金币800。</p><div class="sigil-recipes">${recipes}</div><div class="sigil-grid">${sigils.length ? sigils.map(sigil => sigilCard(sigil, `<button class="gbtn gbtn-sm" onclick="confirmSigilSalvage('${sigil.id}')"><span>分解 · ${SIGIL_DUST[sigil.quality]}粉尘</span></button>`)).join("") : '<p class="social-empty">去第五章六系裂隙寻找你的流派核心。</p>'}</div></section>`;
}

function chooseGirlTalent(id, talent) {
  if (!requireGame()) return;
  const own = SAVE.girls[id], spec = TALENTS[talent];
  if (!own || own.star < 3 || !spec || own.talent === talent) return;
  const cost = own.talent ? { gold: 1000, mat: 10 } : { gold: 500, mat: 5 };
  const rarity = GIRLS.find(g => g.id === id)?.rarity;
  const matKey = MAT_BY_RARITY[rarity];
  showModal(`<h3>确认选择「${spec.name}」</h3><div class="social-confirm"><p>${spec.effect}</p><p>${own.talent ? "重置专精" : "首次选择"}消耗 ${cost.gold} 金币与 ${MATS[matKey].name} ${cost.mat}。</p></div><div class="modal-btns"><button class="gbtn" onclick="openGirl('${id}')"><span>取消</span></button><button class="gbtn gbtn-primary" onclick="confirmGirlTalent('${id}','${talent}')"><span>确认</span></button></div>`);
}

async function confirmGirlTalent(id, talent) {
  if (!await Store.mutate("talent", { girl_id: id, item_id: talent })) return;
  openGirl(id); renderTopbar();
  toast(`已选择${TALENTS[talent].name}专精`);
}

function starTrainingMarkup(id) {
  const girl = GIRLS.find(entry => entry.id === id), own = SAVE.girls[id];
  const step = growthStarStep(girl, own), mat = MAT_BY_RARITY[girl.rarity];
  const dedicated = !!cultivationRules()?.shards, shards = (own.shards || 0) + (dedicated ? 0 : growthDust(girl));
  const limit = growthStarMax();
  const groups = [1,7].filter(first => first <= limit).map(first => `<span class="star-grade-group" data-star-tier="${first === 1 ? 1 : 2}">${Array.from({length:Math.min(6,limit - first + 1)}, (_, index) => `<i data-star-index="${first+index}" class="${first+index <= own.star ? "is-lit" : ""}">${first+index <= own.star ? "★" : "☆"}</i>`).join("")}</span>`).join("");
  return `<section class="cultivation-star at-star-training" data-current-star="${own.star}"><div><h4>星级突破 · ${own.star}/${limit} 星 ${typeof atelierCrest === "function" ? atelierCrest(own.star) : ""}</h4><div class="star-grades" aria-hidden="true">${groups}</div>${step ? `<p>${dedicated ? "本卡专属碎片" : "碎片与可用尘"} ${shards}/${step.shards} · ${MATS[mat].name} ${SAVE.mats[mat] || 0}/${step.material}${step.awakening ? ` · 角色觉醒 ${own.awakening || 0}/${step.awakening} 阶` : ""}</p>` : "<p>已达到最高星级</p>"}</div><button type="button" class="gbtn gbtn-sm gbtn-gold" data-cultivation-action="star" onclick="starUp('${id}')" ${step?.ready ? "" : "disabled"}><span>${step ? `升至 ${step.next} 星` : "已满星"}</span></button>${typeof atelierMilestones === "function" ? atelierMilestones(id) : ""}</section>`;
}

function cultivationOptions(id) {
  const rules = cultivationRules(), own = SAVE.girls[id], girl = GIRLS.find(entry => entry.id === id);
  if (!girl || !rules || !own?.star) return [];
  const level = own.weapon_level || 0, awake = own.weapon_awake || 0, potential = own.potential || 0;
  const ore = SAVE.mats.forge_ore || 0, cores = SAVE.mats.weapon_core || 0, manuals = SAVE.mats.training_manual || 0;
  const levelCosts = Store.profile?.equipment?.level_costs || rules.weapon_ore;
  const mat = MAT_BY_RARITY[girl.rarity], matCount = SAVE.mats[mat] || 0;
  const options = [
    { action: "weapon_upgrade", label: level < levelCosts.length ? `强化至 +${level + 1}` : "专武已满级", detail: level < levelCosts.length ? `星锻矿 ${ore}/${levelCosts[level]}` : `强化 +${level}`, ready: level < levelCosts.length && ore >= levelCosts[level] },
    { action: "weapon_awaken", label: awake < rules.weapon_cores.length ? `专武觉醒 ${awake + 1} 阶` : "专武觉醒已满", detail: awake < rules.weapon_cores.length ? `觉醒棱芯 ${cores}/${rules.weapon_cores[awake]} · 专武 +${rules.weapon_level_gates[awake]} · 角色觉醒 ${rules.weapon_awakening_gates[awake]} 阶` : `${awake} 阶`, ready: awake < rules.weapon_cores.length && cores >= rules.weapon_cores[awake] && level >= rules.weapon_level_gates[awake] && (own.awakening || 0) >= rules.weapon_awakening_gates[awake] },
    { action: "potential_unlock", label: potential < rules.potential_stars.length ? `解锁潜能 ${potential + 1}` : "潜能已全开", detail: potential < rules.potential_stars.length ? `${rules.potential_stars[potential]} 星 · ${MATS[mat].name} ${matCount}/${rules.potential_mats[potential]}` : `${potential} 个节点`, ready: potential < rules.potential_stars.length && own.star >= rules.potential_stars[potential] && matCount >= rules.potential_mats[potential] },
    { action: "use_manual", label: own.level < cardLevelLimit() ? "研读手记" : "已达卡级上限", detail: `持有 ${manuals} 份 · 每份 ${rules.manual_xp} 经验`, ready: own.level < cardLevelLimit() && manuals > 0 },
  ];
  for (const choice of ["edge", "ward", "tempo"]) {
    const rune = rules.engravings[choice];
    if (!rune) continue;
    options.push({ action: "weapon_engrave", choice, label: rune.name, detail: `${own.weapon_rune === choice ? "已铭刻 · " : ""}${CULTIVATION_STATS[rune.stat]} +${rune.value}${rune.stat === "spd" ? "" : "%"}`, ready: level >= rules.weapon_rune_level && ore >= rules.weapon_rune_ore && own.weapon_rune !== choice });
  }
  return options;
}

function cultivationButton(id, option) {
  const { action, choice } = option;
  return `<button type="button" class="gbtn gbtn-sm" data-cultivation-action="${action}" ${choice ? `data-choice="${choice}"` : ""} onclick="cultivateGirl('${action}','${id}'${choice ? `,'${choice}'` : ""})" ${option.ready ? "" : "disabled"}><span>${esc(option.label)}</span><span class="sub">${esc(option.detail)}</span></button>`;
}

function cultivationPanel(id) {
  const rules = cultivationRules(), own = SAVE.girls[id], weapon = rules?.weapons[id];
  if (!weapon || !own?.star) return "";
  const level = own.weapon_level || 0, awake = own.weapon_awake || 0, potential = own.potential || 0;
  const rune = rules.engravings[own.weapon_rune], options = cultivationOptions(id);
  const buttons = actions => options.filter(option => actions.includes(option.action)).map(option => cultivationButton(id, option)).join("");
  return `<section class="cultivation-tools"><div class="cultivation-heading"><div><small>自带专武 · 绑定本人</small><h4>${esc(weapon.name)} <span>+${level}</span></h4></div><strong>觉醒 ${awake}/${rules.weapon_cores.length}</strong></div>
    <p>${CULTIVATION_STATS[weapon.stat]} +${level * rules.weapon_per_level}%（每级 +${rules.weapon_per_level}%） · 觉醒每阶生命 +${rules.weapon_vitality_per_awake}%、初始怒气 +${rules.weapon_rage_per_awake}</p>
    <div class="cultivation-buttons">${buttons(["weapon_upgrade", "weapon_awaken"])}</div>
    <details class="cultivation-runes"><summary>专武铭刻 · ${rune ? esc(rune.name) : "未铭刻"}</summary><p>专武 +${rules.weapon_rune_level} 开启；每次消耗 ${rules.weapon_rune_ore} 星锻矿（持有 ${SAVE.mats.forge_ore || 0}）。重刻替换当前铭纹。</p><div class="cultivation-buttons">${buttons(["weapon_engrave"])}</div></details>
    <div class="cultivation-heading"><div><small>角色潜能 · ${potential}/${rules.potential_stars.length} 节点</small><h4>生命、攻击、防御 +${potential * rules.potential_per_node}%</h4></div><span class="potential-nodes" aria-hidden="true">${rules.potential_stars.map((_, index) => index < potential ? "◆" : "◇").join("")}</span></div>
    <div class="cultivation-buttons">${buttons(["potential_unlock", "use_manual"])}</div></section>`;
}

function collectionMarkup(id) {
  const rules = cultivationRules(), weapon = rules?.weapons[id];
  if (!weapon) return "";
  const bonds = collectionBonds(id), active = SAVE.girls[id]?.star > 0;
  return `<section class="cultivation-bond at-bonds"><div><small>伙伴图鉴</small><h4>本卡收藏羁绊 <span>${bonds.filter(bond => bond.percent).length}/${bonds.length} 已激活</span></h4><p>集齐整组即为组员提供加成，无需上阵。${rules.bond_cap ? `相同属性累计不超过 ${rules.bond_cap}%。` : ""}</p></div><div class="at-bond-list">${bonds.map(bond => collectionCrewMarkup(bond,id)).join("")}</div><p class="codex-line"><b>图鉴常驻</b> ${active ? "已贡献" : "获得后"}：全员${CULTIVATION_STATS[weapon.stat]} +${rules.codex_flat[weapon.stat]}，下阵仍保留。图鉴合计 ${codexSummary()}。</p></section>`;
}

function refreshCultivationViews(id) {
  const page = document.body.dataset.page;
  if (page === "page-growth") {
    const runesOpen = document.querySelector("#growth-cultivation .cultivation-runes")?.open;
    renderGrowth();
    const runes = document.querySelector("#growth-cultivation .cultivation-runes");
    if (runes) runes.open = Boolean(runesOpen);
  }
  if (page === "page-bag") {
    const selected = document.getElementById("manual-target")?.value;
    renderBag();
    const select = document.getElementById("manual-target");
    if (select && selected && SAVE.girls[selected]?.level < cardLevelLimit()) {
      select.value = selected;
      updateManualTarget();
    }
  }
  if (page === "page-squad") renderSquad();
  if (page === "page-home") renderGrowthSummary();
  renderTopbar();
  if (document.querySelector("#modal-mask:not(.hidden) [data-cultivation-card]")?.dataset.cultivationCard === id) {
    if (document.querySelector("#modal-box .hero-workshop") && typeof paintWorkshop === "function") { paintWorkshop(); return; }
    const scrollTop = document.querySelector("#modal-box .mb-scroll").scrollTop;
    const runesOpen = document.querySelector("#modal-box .cultivation-runes")?.open;
    openGirl(id);
    const runes = document.querySelector("#modal-box .cultivation-runes");
    if (runes) runes.open = Boolean(runesOpen);
    document.querySelector("#modal-box .mb-scroll").scrollTop = scrollTop;
  }
}

async function cultivateGirl(action, id, choice) {
  if (!requireGame()) return;
  const option = cultivationOptions(id).find(entry => entry.action === action && entry.choice === choice);
  if (!option?.ready) return;
  const epoch = Store.sessionEpoch;
  const fields = choice ? { girl_id: id, choice } : { girl_id: id };
  const result = await Store.mutate(action, fields);
  if (!result || epoch !== Store.sessionEpoch || !Store.ready()) return;
  refreshCultivationViews(id);
  toast((result.drops || []).join(" · ") || "培养已确认");
}

/* ---------- 少女详情 / 养成 ---------- */
function openGirl(id) {
  if (!requireGame()) return;
  const g = GIRLS.find(x => x.id === id);
  if (!g) return;
  const own = SAVE.girls[id];
  if (own?.star && typeof openWorkshop === "function" && workshopRules()) return openWorkshop(id);
  const s = finalStats(id);
  const el = ELEM[g.elem], rar = RARITY[g.rarity];
  const shards = own?.shards || 0;
  const rageSkill = typeof growthSkill === "function" ? growthSkill(id) : { name: g.skills.skill.name, desc: g.skills.skill.desc };
  const decomposableShards = recyclableShards(g, own);
  const salvageNote = cultivationRules()?.shards ? `回收 ${decomposableShards / cultivationRules().shards.per_ore[g.rarity]} 枚星锻矿` : "获得同等数量共鸣尘";

  /* 图鉴中尚未获得：只展示基础信息与获取途径 */
  if (!own || own.star <= 0) {
    showModal(`
      <div class="gc-head">
        <img src="${g.img}" alt="" class="gc-lock-img">
        <div class="gc-info">
          <div class="gc-name">${g.name}
            <i class="gc-tag" style="color:${rar.color}">${rar.name}</i>
            <i class="gc-tag" style="color:${el.color}">${el.name}系</i></div>
          <div class="gc-desc">${g.desc}</div>
          <div class="mat-line">尚未获得 · 碎片 ${own?.shards || 0}/${rar.shardToUnlock}<span>（可召唤获取，或攒副本碎片解锁）</span></div>
        </div>
      </div>
      <div class="gc-body">
        <div class="gc-stats">
          <span>生命 <b>${s.hp}</b></span><span>攻击 <b>${s.atk}</b></span>
          <span>防御 <b>${s.def}</b></span><span>速度 <b>${s.spd}</b></span>
        </div>
        <div class="gc-skills">
          <div>普攻 · ${g.skills.normal.name}：${g.skills.normal.desc}</div>
          <div>怒气 · ${g.skills.skill.name}：${g.skills.skill.desc}</div>
        </div>
      </div>
      ${collectionMarkup(id)}
      ${typeof atelierMilestones === "function" ? atelierMilestones(id) : ""}
      <div class="modal-btns">
        <button class="gbtn gbtn-sm" onclick="closeModal()"><span>关闭</span></button>
        <button class="gbtn gbtn-sm gbtn-primary" onclick="closeModal();nav('page-gacha')"><span>去召唤</span></button>
        ${SAVE.progression ? `<button type="button" class="gbtn gbtn-sm" onclick="unlockShardGirl('${id}')" ${shards >= RARITY[g.rarity].shardToUnlock ? "" : "disabled"}><span>碎片解锁 · ${RARITY[g.rarity].shardToUnlock}枚</span></button>` : ""}
      </div>`);
    if (decomposableShards > 0) {
      document.querySelector("#modal-box .mb-scroll")?.insertAdjacentHTML("beforeend", `
        <div class="modal-btns">
          <button class="gbtn gbtn-sm gbtn-gold" onclick="confirmShardDecompose('${id}',${decomposableShards})">
            <span>分解多余碎片 ×${decomposableShards}</span><span class="sub">${salvageNote}</span>
          </button>
        </div>`);
    }
    return;
  }

  const inSquad = SAVE.squad.includes(id);
  const matKey = MAT_BY_RARITY[g.rarity];
  showModal(`
    <div class="gc-head" data-cultivation-card="${id}">
      <img src="${g.img}" alt="">
      <div class="gc-info">
        <div class="gc-name">${g.name}
          <i class="gc-tag" style="color:${rar.color}">${rar.name}</i>
          <i class="gc-tag" style="color:${el.color}">${el.name}系</i></div>
        <div class="gc-desc">${g.desc}</div>
        <div class="gc-lv">Lv${own.level} · ${own.star}星 · 碎片 ${own.shards}</div>
        <div class="mat-line"><span>${g.rarity}共鸣尘 ${SAVE.mats["shard_dust_" + g.rarity.toLowerCase()] || 0} · 旧通用共鸣尘 ${SAVE.mats.shard_dust || 0}</span></div>
      </div>
    </div>
    <div class="gc-body">
      <div class="gc-stats">
        <span>生命 <b>${s.hp}</b></span><span>攻击 <b>${s.atk}</b></span>
        <span>防御 <b>${s.def}</b></span><span>速度 <b>${s.spd}</b></span>
        <span>暴击 <b>${s.cr}%</b></span><span>爆伤 <b>${s.cd}%</b></span>
        <span>命中 <b>${s.ehr}</b></span><span>抵抗 <b>${s.res}</b></span>
      </div>
      <div class="gc-skills">
        <div>普攻 · ${g.skills.normal.name}：${g.skills.normal.desc}</div>
        <div>怒气 · ${esc(rageSkill.name)}：${esc(rageSkill.desc)}</div>
      </div>
    </div>
    ${starTrainingMarkup(id)}
    ${cultivationPanel(id)}
    ${collectionMarkup(id)}
    ${gemSocketMarkup(id, own, matKey)}
    ${sigilEquipmentMarkup(id, own)}
    ${talentMarkup(id, own, matKey)}
    ${typeof awakeningDetailMarkup === "function" ? awakeningDetailMarkup(id, own) : ""}
    <div class="modal-btns">
      <button class="gbtn gbtn-sm" onclick="levelUp('${id}')" ${own.level >= cardLevelLimit() ? "disabled" : ""}>
        <span>经验培养</span><span class="sub">${Store.profile?.training?.experience ? "选择材料 · 一键升级" : "副本或固定经验训练"}</span></button>
    </div>
    ${decomposableShards > 0 ? `<div class="modal-btns">
      <button class="gbtn gbtn-sm gbtn-gold" onclick="confirmShardDecompose('${id}',${decomposableShards})">
        <span>分解多余碎片 ×${decomposableShards}</span><span class="sub">${salvageNote}</span>
      </button>
    </div>` : ""}
    <div class="modal-btns">
      <button class="gbtn gbtn-sm" onclick="closeModal()"><span>关闭</span></button>
      <button class="gbtn gbtn-sm gbtn-primary" onclick="toggleSquad('${id}');closeModal()"><span>${inSquad ? "移出队伍" : "加入队伍"}</span></button>
    </div>`);
}
async function levelUp(id) {
  if (!requireGame()) return;
  if (Store.profile?.training?.experience && typeof upgradeWorkshopSheet === "function" && SAVE.girls[id]?.star) openWorkshop(id,"upgrade");
  else if (typeof openGrowthGirl === "function") openGrowthGirl(id);
  else { closeModal(); nav("page-growth"); }
}
async function starUp(id) {
  if (!requireGame()) return;
  const epoch = Store.sessionEpoch, own = SAVE.girls[id];
  if (!own?.star || own.star >= growthStarMax()) return;
  if (cultivationRules()?.shards && !growthStarStep(GIRLS.find(girl => girl.id === id), own)?.ready) return;
  if (!await Store.mutate("star", { girl_id: id }) || epoch !== Store.sessionEpoch || !Store.ready()) return;
  refreshCultivationViews(id);
  toast(`已升至 ${SAVE.girls[id].star} 星`);
}

/* ---------- 通用小组件 ---------- */
/* 星级：三颗星按点亮数量渲染，尺寸由 --sz 控制 */
function starsHTML(n, size) {
  const one = on => `<svg viewBox="0 0 24 24" class="${on ? "on" : ""}">
    <path d="M12 2.4l2.94 6.02 6.56.92-4.75 4.6 1.13 6.5L12 17.36 6.12 20.4l1.13-6.5L2.5 9.34l6.56-.92z"/></svg>`;
  return `<span class="star-row" style="--sz:${size || 12}px">` +
    [0, 1, 2].map(i => one(i < n)).join("") + `</span>`;
}
const LOCK_SVG = `<svg viewBox="0 0 24 24"><path d="M7 10V7a5 5 0 0 1 10 0v3h1a2 2 0 0 1 2 2v7a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2v-7a2 2 0 0 1 2-2h1zm2 0h6V7a3 3 0 0 0-6 0v3z"/></svg>`;

/* ---------- 副本页：章节 / 关卡节点 / 详情 ---------- */
let stageView = { ch: 0, sel: null };

const stageStars = id => SAVE.cleared?.[id] ? Math.max(0, Math.min(3, Number(SAVE.stars?.[id]) || 0)) : 0;
const stageLocked = s => !!(s.unlockStage && !SAVE.cleared[s.unlockStage]);

function pickChapter(i) { if (!requireGame()) return; stageView.ch = i; stageView.sel = null; renderStage(); }
function pickStage(id) { if (!requireGame()) return; stageView.sel = id; renderStage(); }

let stageRestTimer = null;
function gameRestSeconds() {
  const until = Math.max(SAVE?.activity?.rest_until || 0, Store.runRestUntil || 0);
  return Math.max(0, Math.ceil((until - Store.serverNow()) / 1000));
}
function updateStageRunLimits() {
  if (typeof updateGrowthRest === "function") updateGrowthRest(true);
  clearTimeout(stageRestTimer);
  if (!Store.ready() || document.body.dataset.page !== "page-stage") return;
  const seconds = gameRestSeconds();
  const stage = STAGES.find(s => s.id === stageView.sel);
  const button = document.getElementById("stage-start");
  if (button && stage) button.disabled = stageLocked(stage) || seconds > 0;
  const note = document.getElementById("stage-rest-note");
  if (note) note.textContent = seconds ? `连续挑战休整中 · ${seconds} 秒后可出战，期间可养成和编队` : "出战可用 · 连续高速挑战会触发短暂休整，不影响养成与整理背包";
  if (typeof voyageUpdateRunButton === "function") voyageUpdateRunButton(seconds);
  if (seconds) stageRestTimer = setTimeout(updateStageRunLimits, 1000);
}

function stageItemReward(id, quantity, source, fallback) {
  const guide = Store.profile?.items, item = guide?.items?.[id], grade = guide?.qualities?.[item?.quality];
  const paint = /^#[0-9a-f]{6}$/i.test(grade?.color || "") ? grade.color : "#818c98";
  return `<span class="mat" ${source ? `data-${source}-drop="${esc(id)}"` : ""} ${grade ? `data-reward-quality="${esc(item.quality)}" title="${esc(grade.name)}"` : ""}><i aria-hidden="true" style="--c:${paint}"></i>${esc(item?.name || fallback || id)} ${esc(quantity)}</span>`;
}

function renderStage() {
  if (!requireGame()) return;
  if (typeof renderVoyage === "function" && renderVoyage()) return;
  if (!CHAPTERS[stageView.ch]) stageView.ch = 0;
  const ch = CHAPTERS[stageView.ch];
  document.getElementById("node-track").classList.toggle("many-nodes", ch.stages.length > 3);
  if (!stageView.sel || !ch.stages.includes(stageView.sel))
    stageView.sel = ch.stages.find(id => !SAVE.cleared[id]) || ch.stages[ch.stages.length - 1];

  document.getElementById("chapter-bar").innerHTML = CHAPTERS.map((c, i) => {
    const done = c.stages.filter(id => SAVE.cleared[id]).length;
    return `<button class="ch-tab ${i === stageView.ch ? "on" : ""}" onclick="pickChapter(${i})">
      <span class="cn">${c.sub}</span>
      <span class="ct">${c.name}</span>
      <span class="cp">${done}/${c.stages.length}</span>
    </button>`;
  }).join("");

  document.getElementById("node-track").innerHTML = ch.stages.map((id, i) => {
    const s = STAGES.find(x => x.id === id);
    const locked = stageLocked(s);
    return `<button class="node ${locked ? "locked" : ""} ${id === stageView.sel ? "sel" : ""}"
      onclick="pickStage('${id}')">
      <img src="${s.img}" alt="">
      <span class="no">${i + 1}</span>
      ${locked ? `<span class="lock">${LOCK_SVG}</span>` : ""}
      <div class="nm">${s.name}</div>
      <div class="st">${starsHTML(stageStars(id), 11)}</div>
    </button>`;
  }).join("");

  /* 章节概览卡：进度、星级、本章产出材料、关卡清单 */
  const doneN = ch.stages.filter(id => SAVE.cleared[id]).length;
  const starSum = ch.stages.reduce((a, id) => a + stageStars(id), 0);
  const matKeys = [...new Set(ch.stages.map(id => STAGES.find(x => x.id === id).matKey))];
  const stageRows = ch.stages.map((id, i) => {
    const st = STAGES.find(x => x.id === id);
    const lk = stageLocked(st);
    const sn = stageStars(id);
    return `<button class="cc-item ${id === stageView.sel ? "on" : ""} ${lk ? "locked" : ""}"
      onclick="pickStage('${id}')">
      <span class="ci-no">${i + 1}</span>
      <span class="ci-nm">${st.name}</span>
      <span class="ci-st">${lk ? "未解锁" : starsHTML(sn, 10)}</span>
    </button>`;
  }).join("");
  document.getElementById("chapter-card").innerHTML = `
    <img src="${STAGES.find(x => x.id === ch.stages[0]).img}" alt="">
    <div class="cc-in">
      <div class="cc-sub">${ch.sub}</div>
      <div class="cc-name">${ch.name}</div>
      <div class="cc-row">通关 <b>${doneN}/${ch.stages.length}</b>
        <span class="dot">·</span> 星级 <b>${starSum}/${ch.stages.length * 3}</b></div>
      <div class="cc-bar">${Array.from({ length: ch.stages.length * 3 }, (_, i) =>
        `<i class="${i < starSum ? "on" : ""}"></i>`).join("")}</div>
      <div class="cc-mat">${matKeys.map(k =>
        `<span><i style="--c:${MATS[k].color}"></i>${MATS[k].name}</span>`).join("")}</div>
      <div class="cc-list">${stageRows}</div>
      <div class="cc-note">${ch.note}</div>
    </div>`;

  document.getElementById("stage-detail").innerHTML = legacyStageDetailHTML(STAGES.find(x => x.id === stageView.sel));
  updateStageRunLimits();
}

function legacyStageDetailHTML(s, inVoyage = false) {
  const locked = stageLocked(s);
  const prev = s.unlockStage ? STAGES.find(x => x.id === s.unlockStage) : null;
  const best = stageStars(s.id);
  const reward = Store.profile?.stage_rewards?.[s.id];
  const quotaReward = reward && reward.quota_max > 0;
  const family = SIGIL_FAMILIES[s.lootFamily];
  const rules = Store.profile?.run_rules || { burst_limit: 12, rest_seconds: 120, relic_daily_limit: 30 };
  const gear = Store.profile?.equipment, lootKind = s.relicTier >= 2 ? "nightmare" : s.relicTier === 1 ? "rift" : s.hasBoss ? "campaign_boss" : "campaign";
  return `
    <div class="sd-info">
      <div class="sd-title">${s.name}<span class="type-chip">${s.type}</span>
        ${best ? `<span class="sd-best">最佳 ${starsHTML(best, 12)}</span>` : ""}</div>
      <div class="sd-desc">${locked ? "需先通关「" + prev.name + "」" : s.desc}</div>
      <div class="sd-meta">
        <span>敌人 Lv${s.enemyLv}</span>
        <span>${s.waves} 波</span>
        <span>代币 ${s.tokenRange[0]}-${s.tokenRange[1]}</span>
        <span>金币 ${s.gold[0]}-${s.gold[1]}</span>
        ${quotaReward ? `<span>通关余额 ${(reward.quota_min / SITE.quotaPerE).toFixed(2)}～${(reward.quota_max / SITE.quotaPerE).toFixed(2)} E</span>` : s.relicTier ? '<span>灵印材料本 · 不发放站点额度</span>' : ""}
        <span class="mat"><i style="--c:${MATS[s.matKey].color}"></i>${MATS[s.matKey].name} ${s.mat[0]}-${s.mat[1]}</span>
        ${cultivationRules() ? stageItemReward("forge_ore",`×${1 + Math.floor(s.cost / 6)}`,"","星锻矿") + (s.hasBoss ? stageItemReward("weapon_core","×1","","觉醒棱芯") + stageItemReward("training_manual","×1","","历练手记") : "") : ""}
        ${Object.entries(gear?.drops?.[lootKind] || {}).map(([key, count]) => stageItemReward(key,`×${count}`,"gear",gear.materials[key].name)).join("")}
        ${Object.entries(Store.profile?.training?.experience?.drops?.[lootKind] || {}).map(([key,count]) => stageItemReward(key,`×${count}`,"experience")).join("")}
        ${Object.entries({ ...(Store.profile?.training?.drops?.[lootKind] || {}), ...(Store.profile?.carry?.drops?.[lootKind] ? { route_badge:Store.profile.carry.drops[lootKind] } : {}) }).map(([key,count]) => stageItemReward(key,`×${count}`,"mastery")).join("")}
        ${cultivationRules()?.artifacts && (s.relicTier || s.hasBoss) ? stageItemReward(s.relicTier ? "chip_ticket" : "gem_ticket","×1","",ARTIFACT_TICKETS[s.relicTier ? "chip_ticket" : "gem_ticket"].name) : ""}
        <span>三星 ≤ ${s.starRound} 回合</span>
      </div>
      ${family ? `<p class="sd-loot-focus">${ELEM[family.element].name}系碎片优先 · 宝石偏向本系${s.relicTier ? ` · ${family.mat} ${s.essence[0]}-${s.essence[1]} · ${s.relicTier === 2 ? "稀有/史诗" : "普通/稀有"}灵印` : ""}</p>` : ""}
      ${s.mechanic ? `<p class="sd-mechanic">${DUNGEON_MECHANICS[s.mechanic]}</p>` : ""}
      ${quotaReward ? `<p class="sd-reward-note">以上为一星奖励，二星 +10%，三星 +20%；副本总次数与余额产出不设每日上限，高收益本需要更多体力。</p>` : ""}
      ${s.relicTier ? `<p class="sd-reward-note">今日灵印补给 ${SAVE.daily.relic_runs || 0}/${rules.relic_daily_limit}，开局即计次，失败/撤退不返还；超出后仍可获得普通材料、金币与经验。</p>` : ""}
      <p class="sd-reward-note">10分钟内连续 ${rules.burst_limit} 局，当前局结算后休整 ${Math.ceil(rules.rest_seconds / 60)} 分钟。</p>${inVoyage ? "" : '<p id="stage-rest-note" class="sd-rest-note" role="status"></p>'}
    </div>
    ${inVoyage ? "" : `<button id="stage-start" class="gbtn gbtn-primary" ${locked || gameRestSeconds() ? "disabled" : ""} onclick="startBattle('${s.id}')"><span>出战</span><span class="sub">体力 ${s.cost}</span></button>`}`;
}

function renderModes() {
  if (!requireGame()) return;
  const body = document.getElementById("modes-body");
  const expedition = SAVE.expedition || { floor: 1, best: 0, route: "" };
  const weekly = SAVE.weekly_boss || { attempts: 0, best_damage: 0, cleared: false };
  const elemId = weekly.element;
  const elem = ELEM[elemId];
  const routeNames = { assault: "破阵路线", bastion: "守御路线", fortune: "寻宝路线" };
  const routeEffects = { assault: "本次全队攻击 +12%", bastion: "本次全队生命 +12%", fortune: "本次金币与代币奖励 +25%" };
  const stamina = SAVE.stamina || 0;
  const guild = Store.profile?.guild;
  const expeditionDone = expedition.floor > 10;
  const guildMarkup = `<article class="mode-card guild-card"><div class="mode-card-head"><div><p class="social-kicker">母港共建 · 公会讨伐</p><h3>${guild ? esc(guild.name) : "公会招募"}</h3></div></div><p class="mode-note">${guild ? `本周公会航绩 ${fleetNumber(guild.harbor?.score)} · ${guild.attacked_today ? "今日已讨伐" : "今日可讨伐"}` : "查看正在招募的公会，或邀请同伴一起建会。"}</p><button type="button" class="gbtn gbtn-primary" onclick="nav('page-fleet')"><span>进入港务厅</span></button></article>`;
  const bondLabels = squadBondLabels(SAVE.squad);
  body.innerHTML = `<div class="mode-grid">
    <article class="mode-card expedition-card"><div class="mode-card-head"><div><p class="social-kicker">每周 10 层 · 路线增益</p><h3>星穹远征</h3></div><span>${expedition.week || weekly.week || "本周"}</span></div>
      <p class="mode-note">当前第 ${Math.min(expedition.floor,10)} / 10 层 · 本周最高 ${expedition.best || 0} 层 · 消耗 ${12 + Math.min(expedition.floor,10)} 体力。胜利奖励仅限游戏内资源。</p>
      <div class="route-choices" role="group" aria-label="远征路线">${Object.entries(routeNames).map(([key,name]) => `<button type="button" class="route-choice ${expedition.route === key ? "selected" : ""}" aria-pressed="${expedition.route === key}" onclick="chooseExpeditionRoute('${key}')"><b>${name}</b><small>${routeEffects[key]}</small></button>`).join("")}</div>
      <button type="button" class="gbtn gbtn-primary mode-start" onclick="startModeRun('expedition')" ${expeditionDone || !expedition.route || stamina < 12 + Math.min(expedition.floor,10) ? "disabled" : ""}><span>${expeditionDone ? "本周已通关" : !expedition.route ? "先选择本层路线" : stamina < 12 + Math.min(expedition.floor,10) ? "体力不足" : `挑战第 ${expedition.floor} 层`}</span><span class="sub">${expeditionDone ? "下周重置" : expedition.route ? routeEffects[expedition.route] : "胜利后推进一层，失败可重试"}</span></button></article>
    <article class="mode-card weekly-card"><div class="mode-card-head"><div><p class="social-kicker">每周 3 次 · 元素轮换</p><h3>星蚀巨像</h3></div><span class="boss-element" style="--element:${elem?.color || "#999"}">${elem?.name || "元素"}系</span></div>
      <p class="mode-note">本周首领 ${elem?.name || "元素"}系 · 已挑战 ${weekly.attempts || 0} / 3 次 · 最高伤害 ${(weekly.best_damage || 0).toLocaleString()}。击败可得金币与星屑，不发放站点额度。</p>
      <div class="weekly-reward"><span>击败奖励</span><b>金币 +500</b><b>星屑 ×8</b></div>
      <button type="button" class="gbtn gbtn-primary mode-start" onclick="startModeRun('weekly_boss')" ${weekly.cleared || (weekly.attempts || 0) >= 3 || stamina < 20 ? "disabled" : ""}><span>${weekly.cleared ? "本周首领已击败" : (weekly.attempts || 0) >= 3 ? "本周次数用完" : stamina < 20 ? "体力不足" : `挑战首领 · ${3 - (weekly.attempts || 0)} 次剩余`}</span><span class="sub">每次消耗 20 体力</span></button></article>
    ${guildMarkup}
    <article class="mode-card bond-card"><div class="mode-card-head"><div><p class="social-kicker">组合编队 · 实战生效</p><h3>队伍羁绊</h3></div><button class="gbtn gbtn-sm" onclick="nav('page-squad')"><span>调整阵容</span></button></div><div class="guild-members">${bondLabels.map(x => `<span>${esc(x)}</span>`).join("")}</div><p class="mode-note">同元素共鸣、相同定位协奏和四种元素齐备会分别强化属性；效果进入战斗与战力计算。</p></article>
  </div><p class="mode-help">每周一北京时间零点刷新远征与首领。远征代币与普通副本共享每日代币池；新玩法产出为游戏内资源。</p><button type="button" class="gbtn gbtn-sm" onclick="refreshModes()"><span>刷新玩法与公会</span></button>`;
}

async function startModeRun(id) {
  if (!requireGame() || battleRunning) return;
  const expedition = SAVE.expedition;
  const mode = GACHA_RUN_MODES[id];
  if (!mode) return;
  const cost = id === "expedition" ? 12 + expedition.floor : 20;
  const note = id === "expedition" ? `挑战第 ${expedition.floor} 层。失败保留当前层数，消耗本次体力。` : "首领 Lv42，每周最多挑战三次，撤退也计一次。请先调整元素和阵容。";
  showModal(`<h3>${mode.name}</h3><div class="social-confirm"><p>${esc(note)}</p><p>本次消耗 ${cost} 体力。</p></div><div class="modal-btns"><button class="gbtn" onclick="closeModal()"><span>返回</span></button><button class="gbtn gbtn-primary" onclick="closeModal();startBattle('${id}')"><span>确认挑战</span></button></div>`);
}
async function chooseExpeditionRoute(route) {
  if (!requireGame() || !["assault", "bastion", "fortune"].includes(route)) return;
  const result = await Store.mutate("expedition_route", { item_id: route });
  if (!result) return;
  renderModes();
}
async function refreshModes() {
  if (!Store.ready()) return;
  const epoch = Store.sessionEpoch;
  const data = await Store.refresh();
  if (!data || epoch !== Store.sessionEpoch || !Store.ready()) return;
  renderTopbar();
  if (document.body.dataset.page === "page-modes") renderModes();
}
async function copyGuildCode(code) {
  try { await navigator.clipboard.writeText(code); toast("邀请码已复制"); }
  catch { toast("邀请码：" + code); }
}

/* ---------- 背包页：材料 / 碎片 / 消耗品 ---------- */
function bagCultivationMarkup(includeStock = true) {
  const rules = cultivationRules();
  if (!rules) return "";
  const targets = GIRLS.filter(girl => SAVE.girls[girl.id]?.star > 0 && SAVE.girls[girl.id].level < cardLevelLimit());
  return `<section class="bag-cultivation"><h3>${includeStock ? "培养奇物" : "研读手记"}</h3>${includeStock ? `<div class="cultivation-stock">${Object.entries(CULTIVATION_ITEMS).map(([key, item]) => `<div><small>${item.use}</small><b>${item.name} <strong>×${SAVE.mats[key] || 0}</strong></b><span>${item.from}</span></div>`).join("")}</div>` : ""}
    <div class="manual-choice"><label class="growth-select">历练手记 · 研读角色<select id="manual-target" onchange="updateManualTarget()"><option value="">${targets.length ? "选择角色" : "没有未满级角色"}</option>${targets.map(girl => `<option value="${girl.id}">${esc(girl.name)} · Lv${SAVE.girls[girl.id].level}</option>`).join("")}</select></label><button type="button" class="gbtn gbtn-sm" data-cultivation-action="use_manual" id="bag-use-manual" onclick="cultivateGirl('use_manual',document.getElementById('manual-target').value)" disabled><span>消耗 1 份</span><span class="sub">经验 +${rules.manual_xp}</span></button></div></section>`;
}

function updateManualTarget() {
  const select = document.getElementById("manual-target"), button = document.getElementById("bag-use-manual");
  if (select && button) button.disabled = !cultivationOptions(select.value).find(option => option.action === "use_manual")?.ready;
}

function renderBag() {
  if (!requireGame()) return;
  if (typeof renderWorkshopBag === "function" && renderWorkshopBag()) return;
  const body = document.getElementById("bag-body");
  const mats = Object.keys(MATS).map(k => `
    <div class="mat-cell">
      <i class="ic" style="--c:${MATS[k].color}"></i>
      <div class="mn">${MATS[k].name}</div>
      <div class="mv">${SAVE.mats[k] || 0}</div>
      <div class="mf">${MATS[k].from}</div>
      <div class="mu">${MATS[k].use}${cultivationRules() && Object.values(MAT_BY_RARITY).includes(k) ? "、角色潜能" : ""}</div>
    </div>`).join("");
  const gemInventory = SAVE.gem_inventory || {};
  const gems = Object.entries(GEMS).map(([id, gem]) => `
    <div class="gem-inventory-cell" style="--gem-color:${gem.color}">
      <span class="gem-orb">✦</span><b>${gem.name}</b><strong>×${gemInventory[id] || 0}</strong><small>${gem.effect}</small>
    </div>`).join("");

  // 碎片：已拥有（含 0 碎片，方便看升星进度）或未解锁但在攒碎片
  const list = Object.keys(SAVE.girls).map(id => {
    const own = SAVE.girls[id];
    const g = GIRLS.find(x => x.id === id);
    return { g, own, n: own.shards || 0 };
  }).filter(x => x.n > 0 || x.own.level > 0 || x.own.star > 0)
    .sort((a, b) => b.n - a.n);

  body.innerHTML = `
    ${bagCultivationMarkup()}
    <div class="bag-top">
      <div class="bag-mats">${mats}</div>
      <div class="bag-side">
        <div class="side-row"><span>双倍卡</span><b>x${SAVE.multi}</b></div>
        <div class="side-row"><span>代币余额</span><b>${SAVE.token}</b></div>
        <button class="gbtn gbtn-sm gbtn-primary" onclick="nav('page-shop')"><span>代币商店</span><span class="sub">兑换金币、体力与材料</span></button>
        <button class="gbtn gbtn-sm gbtn-gold" onclick="openWarehouse()">
          <span>我的仓库</span><span class="sub">站点折扣卡在这里用</span>
        </button>
      </div>
    </div>
    <div class="sub-title">共鸣宝石 · ${Object.values(gemInventory).reduce((sum, count) => sum + count, 0)} 颗（副本胜利有 18% 概率掉落）</div>
    <div class="gem-inventory">${gems}</div>
    ${sigilInventoryMarkup()}
    <div class="sub-title">少女碎片 · ${list.length} 种</div>
    <div class="shard-grid">
      ${list.length ? list.map(({ g, own, n }) => {
        const need = RARITY[g.rarity].shardToUnlock;
        const unlocking = !own.star;
        return `<div class="shard-cell r-${g.rarity}" onclick="openGirl('${g.id}')">
          <img src="${g.img}" alt="" loading="lazy">
          <span class="sn">${n}</span>
          <div class="sname">${g.name}</div>
          <div class="sneed">${unlocking ? "解锁需 " + need : own.star + "星"}</div>
        </div>`;
      }).join("") : `<div class="bag-empty">还没有碎片，副本与召唤都可以获得</div>`}
    </div>`;
}

/* ---------- 代币商店与异步竞技场 ---------- */
let socialViewVersion = 0;
let socialData = null;
let arenaTab = "ladder";
let lastArenaResult = null;
let lastArenaEpoch = -1;

async function loadSocial(page, render) {
  const version = ++socialViewVersion;
  const epoch = Store.sessionEpoch;
  const body = document.getElementById(page === "page-shop" ? "shop-body" : "arena-body");
  body.innerHTML = '<p class="social-empty" role="status">正在读取商店与玩家档案…</p>';
  try {
    const profile = await Store.refresh();
    if (!profile || version !== socialViewVersion || epoch !== Store.sessionEpoch || document.body.dataset.page !== page) return;
    const data = await Store.api(SITE.ep.social, "GET");
    if (version !== socialViewVersion || epoch !== Store.sessionEpoch || document.body.dataset.page !== page) return;
    socialData = data;
    renderTopbar();
    render();
  } catch (e) {
    if (version !== socialViewVersion || epoch !== Store.sessionEpoch || document.body.dataset.page !== page) return;
    body.innerHTML = `<div class="social-empty"><p>${esc(e.message)}</p><button class="gbtn gbtn-sm" onclick="nav('${page}')"><span>重新读取</span></button></div>`;
  }
}

function renderShop() {
  if (!requireGame()) return;
  loadSocial("page-shop", paintShop);
}
function shopDailyRemaining() {
  const bought = Object.values(SAVE.daily.shopBought || {}).reduce((sum, n) => sum + n, 0);
  return Math.max(0, (socialData?.shop_daily_limit || 0) - bought);
}
function shopBlockReason(item) {
  if (shopDailyRemaining() === 0) return "今日次数用完";
  if (SAVE.token < item.price) return "代币不足";
  if (item.resource === "stamina" && SAVE.stamina + item.amount > SAVE.staminaMax) return "体力空间不足";
  return "";
}
function paintShop() {
  const items = socialData.shop || [];
  const exchange = socialData.token_exchange;
  const maximum = tokenExchangeMax();
  document.getElementById("shop-body").innerHTML = `
    <div class="social-wallet"><span>我的代币 <b>${SAVE.token.toLocaleString()}</b></span><span id="shop-daily-remaining">今日兑换剩余 <strong>${shopDailyRemaining()} / ${socialData.shop_daily_limit}</strong> 次</span></div>
    <section class="token-exchange" aria-labelledby="token-exchange-title">
      <div><h3 id="token-exchange-title">余额兑换代币</h3><p>1 E = 1 代币 · 站点可用余额 <strong>${quotaToE(SITE_ACCOUNT?.quota || 0)} E</strong></p><p>不占商店兑换次数及副本代币产出额度</p></div>
      <div class="token-exchange-controls"><label for="token-exchange-count">兑换数量（整数）</label><input id="token-exchange-count" type="number" inputmode="numeric" min="1" max="${maximum}" step="1" value="${Math.max(1, Math.min(10, maximum))}" oninput="updateTokenExchange()" ${exchange?.enabled && maximum > 0 ? "" : "disabled"} aria-describedby="token-exchange-cost"><p id="token-exchange-cost" aria-live="polite"></p><button id="token-exchange-submit" class="gbtn gbtn-sm gbtn-primary" onclick="confirmTokenExchange()"><span>兑换代币</span></button></div>
    </section>
    <div class="shop-grid">${items.map(item => {
      const reason = shopBlockReason(item);
      const bought = SAVE.daily.shopBought?.[item.id] || 0;
      const detail = { gold: "用于召唤与升级", stamina: "恢复体力，上限 120", mat_r: "用于 R 少女升星", mat_sr: "用于 SR 少女升星", mat_ssr: "用于 SSR 少女升星", multi: "主页开启后用于下一次副本" }[item.resource] || "游戏补给";
      const id = /^[a-z_]+$/.test(item.id) ? item.id : "";
      return `<article class="shop-item shop-${esc(item.resource)}"><span class="shop-mark" aria-hidden="true">${item.resource === "gold" ? "G" : item.resource === "stamina" ? "+" : item.resource === "multi" ? "×2" : "◆"}</span><div><h3>${esc(item.name)} <b>×${item.amount}</b></h3><p>${detail}</p></div><div class="shop-stock">今日已兑换 ${bought} 次 · 全店共用 ${socialData.shop_daily_limit} 次</div><button class="gbtn gbtn-sm gbtn-primary" onclick="confirmShopItem('${id}')" ${reason ? "disabled" : ""}><span>${reason || item.price + " 代币兑换"}</span>${reason ? `<span class="sub">${item.price} 代币</span>` : ""}</button></article>`;
    }).join("")}</div>
    <p class="social-note">全店每天共可兑换 ${socialData.shop_daily_limit} 次，北京时间零点刷新。副本每日最多产出 ${TOKEN_BUDGET} 代币，余额兑换的代币另计；副本双倍卡与站点倍率折扣卡是不同奇物。</p>`;
  updateTokenExchange();
}
function tokenExchangeMax() {
  const rules = socialData?.token_exchange;
  if (!rules?.enabled || !(rules.quota_per_token > 0)) return 0;
  return Math.max(0, Math.min(Math.floor((SITE_ACCOUNT?.quota || 0) / rules.quota_per_token), rules.max_count - SAVE.token));
}
function updateTokenExchange() {
  const input = document.getElementById("token-exchange-count");
  if (!input) return false;
  const count = Number(input.value);
  const maximum = tokenExchangeMax();
  let message = "";
  if (!socialData?.token_exchange?.enabled) message = "余额兑换暂不可用";
  else if (maximum < 1) message = "余额不足 1 E 或代币已达上限";
  else if (!Number.isSafeInteger(count) || count < 1 || count > maximum) message = `请输入 1–${maximum.toLocaleString()} 的整数`;
  document.getElementById("token-exchange-submit").disabled = Boolean(message);
  document.getElementById("token-exchange-cost").textContent = message || `消耗 ${count.toLocaleString()} E，获得 ${count.toLocaleString()} 代币`;
  return !message;
}
function confirmTokenExchange() {
  if (!requireGame() || !updateTokenExchange()) return;
  const count = Number(document.getElementById("token-exchange-count").value);
  showModal(`<h3>确认兑换代币</h3><div class="social-confirm"><h4>${count.toLocaleString()} E → ${count.toLocaleString()} 代币</h4><p>将从站点余额扣除 <b>${count.toLocaleString()} E</b>，获得 <b>${count.toLocaleString()} 代币</b>。</p><p>这是单向兑换，代币不能换回站点余额。</p></div><div class="modal-btns"><button class="gbtn gbtn-sm" onclick="closeModal()"><span>取消</span></button><button class="gbtn gbtn-sm gbtn-primary" onclick="exchangeTokens(${count})"><span>确认兑换</span></button></div>`);
}
async function exchangeTokens(count) {
  if (!requireGame()) return;
  const result = await Store.mutate("token_exchange", { count });
  if (!result) return;
  closeModal(); renderTopbar(); renderShop();
  toast(`兑换成功，获得 ${result.exchange.count} 代币`);
}
function confirmShopItem(id) {
  if (!requireGame()) return;
  const item = socialData?.shop?.find(x => x.id === id);
  if (!item) return;
  const reason = shopBlockReason(item);
  if (reason) return toast(reason);
  showModal(`<h3>兑换补给</h3><div class="social-confirm"><h4>${esc(item.name)} ×${item.amount}</h4><p>消耗 <b>${item.price} 代币</b> · 当前拥有 ${SAVE.token}</p><p>全店每日共 ${socialData.shop_daily_limit} 次，今日剩余 ${shopDailyRemaining()} 次；本次占用 1 次。</p></div><div class="modal-btns"><button class="gbtn gbtn-sm" onclick="closeModal()"><span>取消</span></button><button class="gbtn gbtn-sm gbtn-primary" onclick="buyShopItem('${id}')"><span>确认兑换</span></button></div>`);
}
async function buyShopItem(id) {
  if (!requireGame()) return;
  const result = await Store.mutate("shop", { item_id: id });
  if (!result) return;
  closeModal(); renderTopbar(); renderShop(); toast("兑换成功，补给已放入背包");
}

let arenaQueue = null;
let arenaStake = 1;
let arenaPollTimer = null;
let arenaQueueVersion = 0;
let arenaViewEpoch = -1;
let arenaRankPage = 0;
let arenaHistoryFilter = "all";
let arenaRoomFilter = "all";
const arenaRankPageSize = 10;

function stopArenaPolling() {
  clearTimeout(arenaPollTimer);
  arenaPollTimer = null;
  arenaQueueVersion++;
}
function renderArena() {
  if (!requireGame()) return;
  stopArenaPolling();
  if (arenaViewEpoch !== Store.sessionEpoch) {
    arenaViewEpoch = Store.sessionEpoch;
    socialData = null;
    arenaQueue = null;
    arenaLobby = null;
    arenaRankPage = 0;
    arenaHistoryFilter = arenaRoomFilter = "all";
    ladderData = null;
  }
  syncArenaTabs();
  paintArenaSummary();
  if (arenaTab.startsWith("ladder")) return renderLadder();
  loadSocial("page-arena", paintArena);
}
function arenaOwnSquad() {
  return SAVE.squad.flatMap(id => SAVE.girls[id] ? [{ id, ...SAVE.girls[id] }] : []);
}
function paintArenaSummary() {
  if (arenaTab.startsWith("ladder")) { paintLadderSummary(); return; }
  const arena = SAVE.arena || { wins: 0, played: 0 };
  const leader = GIRLS.find(g => g.id === SAVE.squad[0]);
  document.getElementById("arena-summary").innerHTML = `<div class="arena-captain">
    ${leader ? `<img src="${leader.img}" alt="${leader.name}">` : '<span class="arena-emblem" aria-hidden="true">✦</span>'}
    <div><span class="social-kicker">我的竞技档案</span><strong>${esc(SITE_ACCOUNT?.name || "星霜旅人")}</strong><span>当前编队战力</span><b>${(SAVE.power || 0).toLocaleString()}</b></div></div>
    <div class="arena-metrics"><div><span>战力排名</span><b>${socialData?.power_rank || "未上榜"}</b></div><div><span>今日剩余</span><b>${Math.max(0, 5 - (SAVE.daily.arenaPlayed || 0))}<small> / 5</small></b></div><div><span>累计胜场</span><b>${arena.wins}<small> / ${arena.played}</small></b></div><div><span>累计胜率</span><b>${arena.played ? Math.round(arena.wins / arena.played * 100) + "%" : "—"}</b></div></div>
    <div class="arena-sidebar-squad"><span>当前出战阵容</span><div class="arena-squad">${arenaSquadHTML(arenaOwnSquad()) || '<p>尚未编队</p>'}</div></div>`;
}
function pickArenaTab(tab) {
  if (!["ladder", "ladder-board", "ladder-rewards", "ladder-records", "queue", "rooms", "power", "history"].includes(tab)) return;
  arenaTab = tab;
  renderArena();
}
function arenaSquadHTML(squad) {
  return (squad || []).map(own => {
    const girl = GIRLS.find(g => g.id === own.id);
    return girl ? `<span title="${girl.name} · Lv${own.level} · ${own.star}星"><img src="${girl.img}" alt="${girl.name}" loading="lazy"><small>Lv${own.level}</small></span>` : "";
  }).join("");
}
function syncArenaTabs() {
  document.querySelectorAll("[data-arena-tab]").forEach(b => {
    b.classList.toggle("active", b.dataset.arenaTab === arenaTab);
    b.setAttribute("aria-pressed", String(b.dataset.arenaTab === arenaTab));
  });
}
function paintArena() {
  paintArenaSummary();
  syncArenaTabs();
  const body = document.getElementById("arena-body");
  if (arenaTab === "rooms") {
    body.innerHTML = '<p class="social-empty" role="status">正在读取自定义大厅…</p>';
    refreshArenaRooms();
    return;
  }
  if (arenaTab === "queue") {
    body.innerHTML = '<p class="social-empty" role="status">正在读取匹配状态…</p>';
    refreshArenaQueue();
    return;
  }
  if (arenaTab === "history") {
    paintArenaHistory();
    return;
  }
  paintArenaRanking();
}
function arenaBoard() {
  return (socialData?.power_board || []).filter(entry => Number.isSafeInteger(entry.user_id) && entry.user_id > 0);
}
function paintArenaRanking() {
  if (!Store.ready() || arenaTab !== "power") return;
  const board = arenaBoard(), pages = Math.max(1, Math.ceil(board.length / arenaRankPageSize));
  arenaRankPage = Math.max(0, Math.min(arenaRankPage, pages - 1));
  document.getElementById("arena-body").innerHTML = `<div class="arena-section-heading"><div><p class="social-kicker">星霜群英</p><h3>战力风云榜</h3><p>展示当前编队战力前 50 名</p></div><button type="button" class="social-refresh" onclick="renderArena()">刷新榜单</button></div>
    <div class="arena-podium">${board.slice(0, 3).map((entry, i) => {
      const leader = GIRLS.find(g => g.id === entry.squad?.[0]?.id);
      return `<button type="button" class="arena-podium-card podium-${i + 1}" onclick="${entry.user_id === Store.authUserId ? "nav('page-squad')" : `previewOpponent(${entry.user_id})`}" aria-label="第 ${i + 1} 名 ${esc(entry.name)}，查看阵容"><span class="arena-podium-place">0${i + 1}</span><span class="arena-podium-title">${["星霜之巅", "耀星之席", "逐光之席"][i]}</span>${leader ? `<img src="${leader.img}" alt="${leader.name}" loading="lazy">` : '<span class="arena-emblem" aria-hidden="true">✦</span>'}<strong>${esc(entry.name)}</strong><span>战力 <b>${entry.power.toLocaleString()}</b></span></button>`;
    }).join("")}</div><div class="arena-list-head"><p>我的战力排名 <b>${socialData?.power_rank || "未上榜"}</b></p><button type="button" class="social-refresh" onclick="locateArenaRank()">定位我的排名</button></div>
    <div class="arena-list">${board.slice(arenaRankPage * arenaRankPageSize, (arenaRankPage + 1) * arenaRankPageSize).map((entry, i) => {
    const id = Number(entry.user_id);
    const self = id === Store.authUserId;
    return `<article class="arena-row ${self ? "is-self" : ""}" ${self ? 'tabindex="-1" aria-label="我的排名"' : ""}><span class="arena-rank">${arenaRankPage * arenaRankPageSize + i + 1}</span><div class="arena-player"><strong>${esc(entry.name)}${self ? " · 我" : ""}</strong><p>战力 <b>${entry.power.toLocaleString()}</b></p><div class="arena-squad">${arenaSquadHTML(entry.squad)}</div></div><button class="gbtn gbtn-sm" onclick="${self ? "nav('page-squad')" : `previewOpponent(${id})`}"><span>${self ? "编队" : "查看阵容"}</span></button></article>`;
  }).join("") || '<p class="social-empty">暂时没有玩家的可用阵容，稍后刷新看看。</p>'}</div>
    <div class="arena-pagination"><button type="button" class="social-refresh" onclick="setArenaRankPage(${arenaRankPage - 1})" ${arenaRankPage === 0 ? "disabled" : ""}>上一页</button><span>第 ${arenaRankPage + 1} / ${pages} 页 · 当前展示 ${board.length} 人</span><button type="button" class="social-refresh" onclick="setArenaRankPage(${arenaRankPage + 1})" ${arenaRankPage + 1 >= pages ? "disabled" : ""}>下一页</button></div>
    <p class="social-note">战力综合当前编队的攻防、生命、速度、暴击与效果命中/抵抗；流派被动和技能搭配也会影响胜负。排行榜仅展示阵容，不能指定玩家发起对战。</p>`;
}
function setArenaRankPage(page) {
  if (!Number.isInteger(page)) return;
  arenaRankPage = page;
  paintArenaRanking();
}
function locateArenaRank() {
  const index = arenaBoard().findIndex(entry => entry.user_id === Store.authUserId);
  if (index < 0) return toast(socialData?.power_rank ? `你的战力排名为 ${socialData.power_rank}，当前榜单只展示前 50 名` : "保存出战阵容后即可参与战力排行");
  setArenaRankPage(Math.floor(index / arenaRankPageSize));
  const row = document.querySelector(".arena-row.is-self");
  row?.focus({ preventScroll: true });
  row?.scrollIntoView?.({ block: "nearest", behavior: window.matchMedia?.("(prefers-reduced-motion: reduce)").matches ? "instant" : "smooth" });
}
function setArenaHistoryFilter(filter) {
  if (!["all", "win", "loss", "draw"].includes(filter)) return;
  arenaHistoryFilter = filter;
  paintArenaHistory();
}
function paintArenaHistory() {
  if (!Store.ready() || arenaTab !== "history") return;
  const history = SAVE.arena?.history || [];
  const filtered = history.filter(m => arenaHistoryFilter === "all" || (arenaHistoryFilter === "win" ? m.win : arenaHistoryFilter === "draw" ? m.draw : !m.win && !m.draw));
  const total = history.reduce((sum, m) => sum + (Number(m.delta_quota) || 0), 0);
  document.getElementById("arena-body").innerHTML = `<div class="arena-section-heading"><div><p class="social-kicker">每场交锋 · 皆有回响</p><h3>我的战斗记录</h3><p>最近 ${history.length} 场 · ${history.filter(m => m.win).length} 胜 · 净变动 ${arenaDelta(total)}</p></div><button type="button" class="social-refresh" onclick="renderArena()">刷新战报</button></div>
    <div class="arena-filters" role="group" aria-label="战报结果筛选">${[["all", "全部"], ["win", "胜利"], ["loss", "惜败"], ["draw", "平局"]].map(([key, label]) => `<button type="button" class="social-refresh" aria-pressed="${arenaHistoryFilter === key}" onclick="setArenaHistoryFilter('${key}')">${label}</button>`).join("")}</div>
    <div class="arena-list">${filtered.map(m => {
      const id = /^[0-9a-f-]{36}$/i.test(m.id) ? m.id : "";
      return `<article class="arena-history"><strong class="${m.win ? "arena-win" : ""}">${m.win ? "胜利" : m.draw ? "平局" : "惜败"}</strong><div><span class="arena-record-label">我方编队 <span aria-hidden="true">⚔</span> 对手</span><b>${esc(m.name)}</b><p>${esc(new Date(m.played_at).toLocaleString("zh-CN", { timeZone: "Asia/Shanghai", hour12: false }))} · 北京时间</p><span class="arena-record-label">本场 ${m.stake_e} E · 已结算</span></div><b>${arenaDelta(m.delta_quota)}</b><button class="gbtn gbtn-sm" ${id ? "" : "disabled"} onclick="replayArena('${id}')"><span>看战报</span></button></article>`;
    }).join("") || `<div class="social-empty"><span class="arena-empty-emblem" aria-hidden="true">⚔</span><p>${history.length ? "最近的战报中没有此类结果。" : "你的第一场交锋，即将开启。"}</p>${history.length ? '<button class="social-refresh" onclick="setArenaHistoryFilter(\'all\')">查看全部战报</button>' : '<button class="gbtn gbtn-primary" onclick="pickArenaTab(\'queue\')"><span>前往匹配</span></button>'}</div>`}</div><p class="social-note">保留最近 10 场对战。回放使用当时的阵容和战斗记录，不消耗次数，也不会重复结算。</p>`;
}
function previewOpponent(id) {
  if (!requireGame()) return;
  const entry = socialData?.power_board?.find(x => x.user_id === id);
  if (!entry || id === Store.authUserId) return;
  const power = SAVE.power || 0;
  const compatible = power > 0 && Math.max(power, entry.power) * 5 <= Math.min(power, entry.power) * 6;
  showModal(`<h3>阵容对照</h3><div class="social-confirm"><div class="arena-comparison"><section><p class="social-kicker">对方阵容</p><h4>${esc(entry.name)}</h4><p>编队战力 <b>${entry.power.toLocaleString()}</b></p><div class="arena-squad arena-preview">${arenaSquadHTML(entry.squad)}</div></section><section><p class="social-kicker">我的阵容</p><h4>${esc(SITE_ACCOUNT?.name || "星霜旅人")}</h4><p>编队战力 <b>${power.toLocaleString()}</b></p><div class="arena-squad arena-preview">${arenaSquadHTML(arenaOwnSquad()) || "尚未编队"}</div></section></div><p class="arena-compare-note">${!power ? "先配置出战阵容，再来切磋吧。" : compatible ? "双方战力在可匹配范围内，技能与搭配也会影响胜负。" : "双方战力差超过较低方的 20%，暂不符合对战条件。"}</p><p>仅查看阵容。对战需双方主动匹配，或进入同一房间并分别准备。</p></div><div class="modal-btns"><button class="gbtn gbtn-sm" onclick="closeModal()"><span>返回排行榜</span></button><button class="gbtn gbtn-sm gbtn-primary" onclick="closeModal();nav('page-squad')"><span>调整我的编队</span></button></div>`);
}
function showArenaRules() {
  if (!requireGame()) return;
  if (arenaTab.startsWith("ladder")) { showLadderRules(); return; }
  showModal(`<h3>竞技场规则</h3><div class="arena-rules"><section><h4>01 · 选择对战方式</h4><p>随机匹配优先选择同金额、战力最接近的玩家；也可创建或加入公开房间，双方分别准备后开战。战力差不能超过较低方的 20%，等待不会放宽范围。</p></section><section><h4>02 · 出战阵容与次数</h4><p>随机匹配按入队时、房间按进入时保存阵容。两种模式合计每天最多 5 场，北京时间零点刷新。双方自动战斗，60 回合未分胜负判平局。</p></section><section><h4>03 · 余额与退出</h4><p>每场选择 1～5 E：胜利增加本场金额，失败扣除本场金额，平局退回预留余额。入队或确认房间准备时预留；创建和加入房间不扣款。</p><p>随机匹配等待最多 2 分钟，房间最多保留 5 分钟。开战前取消或超时会退款。离开页面不会立即取消排队或准备，仍可能开战。</p></section><section><h4>04 · 排行与战报</h4><p>战力榜展示当前编队前 50 名，可查看和对照阵容。排名不代表胜负，也不发放排名奖励。最近 10 场战报可重复观看，回放不再次扣款。</p></section></div><div class="modal-btns"><button class="gbtn gbtn-sm" onclick="closeModal()"><span>知道了</span></button></div>`);
}
function arenaDelta(quota) {
  const value = Number(quota) || 0;
  return (value > 0 ? "+" : "") + (value / SITE.quotaPerE).toFixed(0) + " E";
}
function setArenaStake(value) {
  if (!Number.isInteger(value) || value < 1 || value > 5 || arenaQueue?.status === "waiting") return;
  arenaStake = value;
  paintArenaQueue();
}
function paintArenaQueue() {
  if (!arenaQueue || !SAVE || arenaTab !== "queue" || document.body.dataset.page !== "page-arena") return;
  if (arenaQueue.status === "room") {
    document.getElementById("arena-body").innerHTML = `<div class="social-empty"><p>你已加入自定义房间，请先完成或退出房间。</p><button class="gbtn gbtn-primary" onclick="pickArenaTab('rooms')"><span>返回我的房间</span></button></div>`;
    return;
  }
  const waiting = arenaQueue.status === "waiting";
  const available = arenaQueue.quota / SITE.quotaPerE;
  const match = arenaQueue.match;
  const limit = (SAVE.daily.arenaPlayed || 0) >= 5;
  const blocked = !socialData?.arena_enabled || !SAVE.squad.length || limit || available < arenaStake;
  const seconds = Math.min(120, Math.max(0, Math.ceil((arenaQueue.expire_at - Store.serverNow()) / 1000)));
  const leader = !waiting && GIRLS.find(g => g.id === SAVE.squad[0]);
  document.getElementById("arena-body").innerHTML = `<div class="arena-duel-banner"><div class="arena-duel-team">${leader ? `<img src="${leader.img}" alt="${leader.name}">` : '<span class="arena-emblem" aria-hidden="true">✦</span>'}<div><p class="social-kicker">${waiting ? "本场阵容已锁定" : "我的出战编队"}</p><h3>${esc(SITE_ACCOUNT?.name || "星霜旅人")}</h3><p>战力 <b>${(waiting ? arenaQueue.power : SAVE.power || 0).toLocaleString()}</b></p><span>${waiting ? "以入队时保存的阵容出战" : `${SAVE.squad.length} 位少女 · 准备交锋`}</span></div></div><span class="arena-versus" aria-hidden="true">VS</span><div class="arena-duel-target"><span class="arena-target-emblem" aria-hidden="true">◇</span><b>${waiting ? "寻找势均力敌的对手" : "下一位对手，静待相逢"}</b><span>同金额 · 战力差 ≤ 20%</span></div></div><div class="arena-queue">
    <div class="arena-queue-head"><div><p class="social-kicker">双方自愿 · 同金额匹配</p><h3>${waiting ? "正在等待对手" : "选择本场匹配金额"}</h3></div><div class="arena-available"><span>可用站点余额</span><b>${available.toFixed(4)} E</b></div></div>
    ${waiting ? `<div class="arena-wait" role="status"><strong>已预留 ${arenaQueue.stake_e} E</strong><p>${seconds > 0 ? `最多再等待 ${seconds} 秒` : "正在确认超时退款…"} · 使用入队时保存的阵容</p><p>入队战力 <b>${arenaQueue.power.toLocaleString()}</b> · 对手范围 <b>${arenaQueue.min_power.toLocaleString()}～${arenaQueue.max_power.toLocaleString()}</b></p><p>匹配成功后立即结算。取消需在匹配成功前生效。</p><button class="gbtn gbtn-primary" onclick="cancelArenaQueue()"><span>取消匹配并退款</span></button></div>` : `
    <div class="arena-stakes" role="group" aria-label="本场匹配金额">${[1,2,3,4,5].map(n => `<button type="button" aria-pressed="${arenaStake === n}" class="${arenaStake === n ? "selected" : ""}" onclick="setArenaStake(${n})"><b>${n}</b><span>E</span></button>`).join("")}</div>
    <div class="arena-terms"><p>本场获胜 <b>+${arenaStake} E</b> · 失败 <b>−${arenaStake} E</b> · 平局 <b>0 E</b></p><p>入队先预留 ${arenaStake} E；取消或 2 分钟未匹配成功，全额退回。预留金额暂不能用于站点消费。</p><p>离开页面不会立即退队，剩余等待时间内仍可匹配；不想继续请先取消。</p></div>
    ${arenaQueue.status === "cancelled" || arenaQueue.status === "expired" ? `<p class="arena-refunded" role="status">${arenaQueue.status === "expired" ? "匹配已超时" : "已取消匹配"}，${arenaQueue.stake_e} E 已退回站点余额。</p>` : ""}
    ${match ? `<div class="arena-last"><span>上场${match.win ? "胜利" : match.draw ? "平局" : "惜败"} · ${arenaDelta(match.delta_quota)}</span><button type="button" class="social-refresh" onclick="replayArena('${esc(match.id)}')">查看结果与战报</button></div>` : ""}
    <button class="gbtn gbtn-primary arena-join" ${blocked ? "disabled" : ""} onclick="confirmArenaQueue()"><span>${!socialData?.arena_enabled ? "匹配暂未开放" : limit ? "今日匹配次数已用完" : !SAVE.squad.length ? "请先编队" : available < arenaStake ? "可用余额不足" : `加入 ${arenaStake} E 匹配`}</span></button>`}
    <p class="social-note">仅从同金额且尚未过期的队列中配对，不会攻击未排队的玩家。使用服务器保存的阵容自动对战，60 回合未分胜负为平局；站点余额结算不受关闭页面影响。</p>
  </div>`;
}
async function refreshArenaQueue() {
  const epoch = Store.sessionEpoch;
  const version = ++arenaQueueVersion;
  clearTimeout(arenaPollTimer);
  const current = () => epoch === Store.sessionEpoch && version === arenaQueueVersion && Store.ready();
  try {
    const data = await Store.api(SITE.ep.arenaQueue, "GET");
    if (!current()) return;
    const wasWaiting = arenaQueue?.status === "waiting";
    arenaQueue = data;
    if (SITE_ACCOUNT) SITE_ACCOUNT.quota = data.quota;
    renderTopbar();
    if (wasWaiting && data.status !== "waiting") {
      await Store.refresh();
      if (!current()) return;
      paintArenaSummary(); renderTopbar();
    }
    paintArenaQueue();
    if (data.status === "waiting" && document.body.dataset.page === "page-arena" && arenaTab === "queue" && !document.hidden) {
      arenaPollTimer = setTimeout(refreshArenaQueue, 3000);
    }
  } catch (e) {
    if (!current() || document.body.dataset.page !== "page-arena" || arenaTab !== "queue") return;
    document.getElementById("arena-body").innerHTML = `<div class="social-empty"><p>${esc(e.message)}</p><p>匹配状态保存在服务器，重新读取不会再次扣款。</p><button class="gbtn gbtn-sm" onclick="refreshArenaQueue()"><span>重新读取匹配</span></button></div>`;
  }
}
function confirmArenaQueue() {
  if (!requireGame() || arenaQueue?.status === "waiting") return;
  showModal(`<h3>确认加入匹配</h3><div class="social-confirm"><h4>本场 ${arenaStake} E</h4><p>胜利获得对方 ${arenaStake} E，失败损失自己 ${arenaStake} E。</p><p>确认后预留余额并锁定本场阵容，只匹配同金额、战力差不超过较低方 20% 的玩家。等待最多 2 分钟，取消或超时退回；离开页面仍可能匹配。</p></div><div class="modal-btns"><button class="gbtn gbtn-sm" onclick="closeModal()"><span>返回</span></button><button class="gbtn gbtn-sm gbtn-primary" onclick="joinArenaQueue(${arenaStake})"><span>确认入队</span></button></div>`);
}
async function joinArenaQueue(stake) {
  if (!requireGame() || !Number.isInteger(stake) || stake < 1 || stake > 5) return;
  stopArenaPolling();
  const result = await Store.mutate("arena_join", { stake_e: stake });
  if (!result) return;
  closeModal();
  await refreshArenaQueue();
  paintArenaSummary();
  if (arenaQueue?.match) showArenaResult(arenaQueue.match);
}
async function cancelArenaQueue() {
  if (!requireGame() || arenaQueue?.status !== "waiting") return;
  const id = arenaQueue.id;
  stopArenaPolling();
  const result = await Store.mutate("arena_cancel", { queue_id: id });
  if (!result) return;
  await refreshArenaQueue();
  if (arenaQueue?.match) showArenaResult(arenaQueue.match);
}
document.addEventListener("visibilitychange", () => {
  if (document.hidden) stopArenaPolling();
  else if (Store.ready() && document.body.dataset.page === "page-arena" && arenaTab.startsWith("ladder")) renderArena();
  else if (Store.ready() && document.body.dataset.page === "page-arena" && arenaTab === "queue") refreshArenaQueue();
  else if (Store.ready() && document.body.dataset.page === "page-arena" && arenaTab === "rooms") refreshArenaRooms();
});

let arenaLobby = null;
let roomStake = 1;
function setArenaRoomFilter(filter) {
  if (!["all", "compatible"].includes(filter)) return;
  arenaRoomFilter = filter;
  paintArenaRooms();
}
function arenaRoomCompatible(room) {
  const power = SAVE.power || 0;
  return power > 0 && Math.max(power, room.host_power) * 5 <= Math.min(power, room.host_power) * 6;
}
function arenaRoomCards(blocked) {
  const rooms = arenaLobby.rooms.filter(room => arenaRoomFilter !== "compatible" || arenaRoomCompatible(room));
  return rooms.map(room => {
    const compatible = arenaRoomCompatible(room);
    const entry = arenaBoard().find(player => player.user_id === room.host_id);
    const leader = GIRLS.find(girl => girl.id === entry?.squad?.[0]?.id);
    const roomID = /^[0-9a-f-]{36}$/i.test(room.id) ? room.id : "";
    return `<article class="room-row arena-opponent-card ${compatible ? "is-compatible" : ""}"><div class="arena-opponent-top"><span class="arena-room-badge">${compatible ? "战力相近" : "战力超出范围"}</span><span>1 / 2 人</span></div><div class="arena-opponent-portrait">${leader ? `<img src="${leader.img}" alt="${leader.name}" loading="lazy">` : '<span class="arena-emblem" aria-hidden="true">✦</span>'}</div><h4>${esc(room.host_name)} 的房间</h4><p>入场战力 <b>${room.host_power.toLocaleString()}</b></p><strong class="arena-room-stake">${room.stake_e} <small>E / 场</small></strong><p class="arena-room-note">双方准备后开战 · 加入不扣款</p><button class="gbtn gbtn-sm" ${blocked || !compatible || !roomID ? "disabled" : ""} onclick="confirmJoinRoom('${roomID}')"><span>${!compatible ? "战力差超过20%" : "进入房间"}</span></button></article>`;
  }).join("") || `<div class="social-empty"><span class="arena-empty-emblem" aria-hidden="true">◇</span><p>${arenaRoomFilter === "compatible" && arenaLobby.rooms.length ? "暂时没有战力相近的房间，可创建自己的房间等待对手。" : "暂时没有可加入的房间，创建一个等大家来吧。"}</p></div>`;
}
function setRoomStake(value) {
  if (!Number.isInteger(value) || value < 1 || value > 5) return;
  roomStake = value;
  paintArenaRooms();
}
function paintArenaRooms() {
  if (!arenaLobby || arenaTab !== "rooms" || document.body.dataset.page !== "page-arena") return;
  const own = arenaLobby.room;
  const active = own?.status === "open";
  const body = document.getElementById("arena-body");
  const remaining = Math.max(0, 5 - (SAVE.daily.arenaPlayed || 0));
  const signature = JSON.stringify([arenaLobby, roomStake, remaining, socialData?.arena_enabled, arenaRoomFilter, SAVE.power]);
  if (body.querySelector(".room-lobby") && body.dataset.roomSignature === signature) {
    const timer = body.querySelector("[data-room-time]");
    if (timer) timer.textContent = Math.max(0, Math.ceil((own.expire_at - Store.serverNow()) / 1000));
    return;
  }
  body.dataset.roomSignature = signature;
  const blocked = !socialData?.arena_enabled || remaining === 0 || arenaLobby.queued || !SAVE.squad.length;
  let content;
  if (active) {
    const host = own.host_id === Store.authUserId;
    const ready = host ? own.host_ready : own.guest_ready;
    const members = [[own.host_name, own.host_power, own.host_ready, "房主"], [own.guest_name || "等待玩家加入", own.guest_power, own.guest_ready, "挑战者"]];
    content = `<div class="room-current"><div class="arena-queue-head"><div><p class="social-kicker">我的房间 · ${own.stake_e} E 对战</p><h3>双方准备后自动开战</h3></div><span><b data-room-time>${Math.max(0, Math.ceil((own.expire_at - Store.serverNow()) / 1000))}</b> 秒后关闭</span></div><div class="room-members">${members.map(m => `<article><span>${m[3]}</span><h4>${esc(m[0])}</h4><p>战力 ${m[1] ? m[1].toLocaleString() : "—"}</p><strong>${m[2] ? `已准备 · 已预留 ${own.stake_e} E` : "未准备 · 未扣款"}</strong></article>`).join("")}</div><p>阵容按进入房间时锁定。胜利 +${own.stake_e} E，失败 −${own.stake_e} E。任一方在开战前退出，房间关闭，已预留金额全额退回。</p><div class="modal-btns"><button class="gbtn" onclick="roomAction('room_leave','${esc(own.id)}')"><span>退出房间</span><span class="sub">开战前退出退款</span></button><button class="gbtn gbtn-primary" ${ready || !own.guest_id ? "disabled" : ""} onclick="confirmRoomReady()"><span>${ready ? "已准备，等待对方" : !own.guest_id ? "等待第二位玩家" : "准备对战"}</span><span class="sub">${ready ? `已预留 ${own.stake_e} E` : `确认后预留 ${own.stake_e} E`}</span></button></div></div>`;
  } else {
    content = `${own?.status === "cancelled" || own?.status === "expired" ? '<p class="arena-refunded">房间已关闭，双方已预留的金额均已退回。</p>' : ""}${own?.match ? `<div class="arena-last"><span>上场对战 ${arenaDelta(own.match.delta_quota)}</span><button class="social-refresh" onclick="replayArena('${esc(own.match.id)}')">查看战报</button></div>` : ""}<div class="room-create"><h3>创建公开房间</h3><div class="arena-stakes" role="group" aria-label="房间对战金额">${[1,2,3,4,5].map(n => `<button type="button" aria-pressed="${roomStake === n}" class="${roomStake === n ? "selected" : ""}" onclick="setRoomStake(${n})"><b>${n}</b><span>E</span></button>`).join("")}</div><button class="gbtn gbtn-primary" ${blocked ? "disabled" : ""} onclick="createArenaRoom()"><span>${arenaLobby.queued ? "请先取消随机匹配" : !socialData?.arena_enabled ? "对战暂未开放" : !remaining ? "今日对战次数已用完" : !SAVE.squad.length ? "请先编队" : "创建 " + roomStake + " E 房间"}</span><span class="sub">创建和加入均不扣款</span></button></div><div class="arena-list-head"><h3>寻找对手 <small>· ${arenaLobby.rooms.length} 个房间</small></h3><button class="social-refresh" onclick="refreshArenaRooms()">刷新大厅</button></div><div class="arena-filters" role="group" aria-label="房间筛选">${[["all", "全部房间"], ["compatible", "战力相近"]].map(([key, label]) => `<button type="button" class="social-refresh" aria-pressed="${arenaRoomFilter === key}" onclick="setArenaRoomFilter('${key}')">${label}</button>`).join("")}</div><div class="room-list arena-opponents">${arenaRoomCards(blocked)}</div>`;
  }
  body.innerHTML = `<div class="room-lobby"><div class="social-wallet"><span>可用余额 <b>${quotaToE(arenaLobby.quota)} E</b></span><span>今日剩余 ${remaining} / 5 场</span></div>${content}<p class="social-note">直接选择房间加入，无需房间码。战力差不超过较低方的20%；仅双方准备后对战。房间5分钟后自动关闭并退款，离开页面不会取消已确认的准备。</p></div>`;
}
async function refreshArenaRooms() {
  clearTimeout(arenaPollTimer);
  const epoch = Store.sessionEpoch;
  const version = ++arenaQueueVersion;
  const current = () => epoch === Store.sessionEpoch && version === arenaQueueVersion && Store.ready() && document.body.dataset.page === "page-arena" && arenaTab === "rooms";
  try {
    const data = await Store.api(SITE.ep.arenaRooms, "GET");
    if (!current()) return;
    const ended = arenaLobby?.room?.status === "open" && data.room?.status !== "open";
    arenaLobby = data;
    if (SITE_ACCOUNT) SITE_ACCOUNT.quota = data.quota;
    if (ended) { await Store.refresh(); if (!current()) return; paintArenaSummary(); }
    paintArenaRooms(); renderTopbar();
    if (!document.hidden) arenaPollTimer = setTimeout(refreshArenaRooms, 5000);
  } catch (e) {
    if (!current()) return;
    toast("大厅同步失败，请稍后刷新");
    if (arenaLobby) paintArenaRooms();
    else document.getElementById("arena-body").innerHTML = '<div class="social-empty"><p>暂时无法读取大厅</p><button class="gbtn" onclick="refreshArenaRooms()"><span>重试</span></button></div>';
  }
}
async function createArenaRoom() { await roomAction("room_create", null, roomStake); }
function confirmJoinRoom(id) {
  const room = arenaLobby?.rooms.find(r => r.id === id);
  if (!room) return;
  showModal(`<h3>进入房间</h3><div class="social-confirm"><h4>${esc(room.host_name)} 的房间</h4><p>战力 ${room.host_power.toLocaleString()} · 本场 ${room.stake_e} E</p><p>加入不扣款；双方分别确认准备后才进行余额对战。</p></div><div class="modal-btns"><button class="gbtn" onclick="closeModal()"><span>返回</span></button><button class="gbtn gbtn-primary" onclick="roomAction('room_join','${esc(id)}')"><span>进入房间</span></button></div>`);
}
function confirmRoomReady() {
  const room = arenaLobby?.room;
  if (!room || room.status !== "open" || !room.guest_id) return;
  showModal(`<h3>确认准备对战</h3><div class="social-confirm"><h4>本场 ${room.stake_e} E</h4><p>胜利 +${room.stake_e} E，失败 −${room.stake_e} E，平局退款。</p><p>确认后预留 ${room.stake_e} E。双方准备后立即结算，关闭页面也会生效。</p></div><div class="modal-btns"><button class="gbtn" onclick="closeModal()"><span>返回</span></button><button class="gbtn gbtn-primary" onclick="roomAction('room_ready','${esc(room.id)}')"><span>确认准备</span></button></div>`);
}
async function roomAction(action, id, stake) {
  if (!requireGame()) return;
  stopArenaPolling();
  const result = await Store.mutate(action, action === "room_create" ? { stake_e: stake } : { room_id: id });
  if (!result) { if (Store.ready()) await refreshArenaRooms(); return; }
  closeModal();
  await refreshArenaRooms(); paintArenaSummary();
  if (arenaLobby?.room?.match) showArenaResult(arenaLobby.room.match);
}
function showArenaResult(result) {
  if (!requireGame()) return;
  lastArenaResult = result;
  lastArenaEpoch = Store.sessionEpoch;
  if (result.ladder) { showLadderResult(result); return; }
  showModal(`<h3>${result.win ? "对战胜利" : result.draw ? "对战平局" : "对战结束"}</h3><div class="social-confirm"><h4>${esc(result.name)}</h4><p>${result.win ? "你的队伍获胜，对方预留的余额已转入" : result.draw ? "60 回合未分胜负，双方预留金额已退回" : "对方队伍获胜，本场预留余额已转给对方"}</p><div class="arena-award">${arenaDelta(result.delta_quota)}<span>站点余额变动</span></div><p>本场金额 ${result.stake_e} E · ${result.report.rounds} 回合<br>余额已结算，重看战报不会再次扣款</p></div><div class="modal-btns"><button class="gbtn gbtn-sm" onclick="leaveArenaReplay()"><span>返回竞技场</span></button><button class="gbtn gbtn-sm gbtn-primary" onclick="watchArenaResult()"><span>观看战报</span></button></div>`);
}
async function replayArena(id) {
  if (!requireGame()) return;
  try {
    const result = await Store.api(SITE.ep.arenaReplay + "?request_id=" + encodeURIComponent(id), "GET");
    showArenaResult(result);
  } catch (e) { if (Store.ready()) toast(e.message); }
}
function watchArenaResult() {
  if (!requireGame() || lastArenaEpoch !== Store.sessionEpoch || !lastArenaResult) return;
  if (BAT?.arena) { BAT.abort = true; BAT = null; battleRunning = false; }
  if (battleRunning) return toast("请先结束当前副本");
  closeModal(true);
  const arena = lastArenaResult;
  resumeBattle({ id: arena.id, arena, result: arena.report, ready_at: Store.serverNow() + arena.report.duration_ms });
}
function leaveArenaReplay() {
  const ranked = lastArenaResult?.ladder;
  if (BAT?.arena) { BAT.abort = true; BAT = null; battleRunning = false; }
  closeModal(true); nav("page-arena");
  if (!ranked) pickArenaTab("queue");
}

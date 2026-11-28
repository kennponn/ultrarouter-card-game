const VOYAGE_MODES = { story: "剧情", hard: "困难", nightmare: "噩梦" };
const VOYAGE_PORTS = { story: "主线副本", resources: "资源副本", rifts: "元素裂隙" };
let voyageView = { epoch: -1, port: "story", chapter: 0, mode: "story", rift: 0 };
let voyageNoticeTimer;
let voyageDayTimer, voyageDayRequest;

function voyageServerDay(now = Store.serverNow()) {
  return new Date(now + 8 * 3600000).toISOString().slice(0,10);
}

function voyageDayStatus(retry = false) {
  const now = Store.serverNow(), today = voyageServerDay(now), epoch = Store.sessionEpoch;
  clearTimeout(voyageDayTimer);
  if (SAVE.daily.date === today) {
    const next = Date.parse(`${today}T00:00:00+08:00`) + 86400000;
    voyageDayTimer = setTimeout(() => {
      if (epoch === Store.sessionEpoch && Store.ready() && document.body.dataset.page === "page-stage") updateStageRunLimits();
    },next-now+50);
    return "";
  }
  if (voyageDayRequest?.epoch !== epoch || voyageDayRequest.day !== today || retry && !voyageDayRequest.pending) {
    const request = {epoch, day:today, pending:true};
    voyageDayRequest = request;
    Promise.resolve().then(() => epoch === Store.sessionEpoch && Store.ready() && document.body.dataset.page === "page-stage" ? Store.refresh() : null).catch(() => null).then(() => {
      request.pending = false;
      if (voyageDayRequest !== request || epoch !== Store.sessionEpoch || !Store.ready() || document.body.dataset.page !== "page-stage") return;
      if (SAVE.daily.date === voyageServerDay()) { renderStage(); renderTopbar(); }
      else updateStageRunLimits();
    });
  }
  return voyageDayRequest.pending ? "新航日正在同步，暂不可出战或扫荡" : "今日次数尚未同步，暂不可出战或扫荡";
}

function voyageDayNotice(node, message) {
  if (!node) return;
  if (message ? node.dataset.voyDayStatus === message : !node.dataset.voyDayStatus) return;
  node.textContent = message;
  if (!message) { delete node.dataset.voyDayStatus; return; }
  node.dataset.voyDayStatus = message;
  if (!voyageDayRequest.pending) {
    const retry = document.createElement("button");
    retry.type = "button"; retry.className = "harbor-link"; retry.dataset.voyRetry = ""; retry.textContent = "重新同步";
    retry.onclick = event => { event.stopPropagation(); voyageUpdateRunButton(gameRestSeconds(), true); };
    node.append(" ", retry);
  }
}

function voyageCurrentDetail(detail) {
  if (!Store.ready() || document.body.dataset.page !== "page-stage" || detail !== document.querySelector("#modal-mask:not(.hidden) [data-voy-detail]") || detail.dataset.voyEpoch !== String(Store.sessionEpoch)) return false;
  const today = voyageServerDay();
  if (detail.dataset.voyDay === today && SAVE.daily.date === today) return true;
  updateStageRunLimits();
  return false;
}

function voyageSession() {
  if (voyageView.epoch !== Store.sessionEpoch) voyageView = { epoch: Store.sessionEpoch, port: "story", chapter: 0, mode: "story", rift: 0 };
  return voyageView;
}

function openVoyage(port, stageId) {
  if (!requireGame() || !Object.hasOwn(VOYAGE_PORTS, port)) return;
  if (stageId) closeModal();
  const view = voyageSession();
  view.port = port;
  if (stageId) view.selected = stageId;
  if (!Store.profile?.voyage && port === "rifts") stageView.ch = 4;
  nav("page-stage");
  if (stageId && voyageContext()?.ids.includes(stageId)) voyageDetail(stageId);
  if (!Store.profile?.voyage && port === "resources") toast("当前服务器尚未开放定向资源航线");
}

function voyageLedger() {
  const book = Store.profile?.voyage;
  if (!book?.chapters?.length || !book.stages || !SAVE?.stars) return null;
  const all = new Set(), tiers = [];
  for (const [index,chapter] of book.chapters.entries()) {
    for (const mode of Object.keys(VOYAGE_MODES)) {
      const ids = chapter.routes?.[mode], chests = chapter.chests?.[mode];
      if (!Array.isArray(ids) || !ids.length || !Array.isArray(chests) || !chests.length) return null;
      ids.forEach(id => all.add(id));
      const total = voyageTotal(ids), open = !voyageGate(voyageStage(ids[0]));
      for (const chest of chests) tiers.push({index,mode,name:chapter.name,chest,open,claimed:!!SAVE.voyage_claims?.[chest.id],missing:Math.max(0,chest.stars-total)});
    }
  }
  const pending = tiers.filter(tier => !tier.claimed), ready = pending.filter(tier => !tier.missing);
  const next = ready[0] || pending.sort((left,right) => Number(right.open)-Number(left.open) || left.missing-right.missing)[0] || tiers.at(-1);
  return {stars:voyageTotal([...all]), maximum:all.size*3, chapterCount:book.chapters.length, ready:ready.length, next};
}

function renderVoyageSummary() {
  const chapterNode = document.getElementById("home-campaign-progress"), chestNode = document.getElementById("home-chest-progress");
  if (!chapterNode || !chestNode) return;
  const ledger = voyageLedger();
  chapterNode.textContent = ledger ? `${ledger.chapterCount} 章 · 剧情 / 困难 / 噩梦` : "章节与难度待同步";
  chestNode.textContent = ledger ? `${ledger.stars} / ${ledger.maximum} 星 · ${ledger.ready ? `${ledger.ready} 档可领取` : ledger.next.claimed ? "已全部领取" : `下档还差 ${ledger.next.missing} 星`}` : "星级记录待同步";
  chestNode.closest("button").classList.toggle("is-ready", !!ledger?.ready);
}

function openVoyageChests() {
  if (!requireGame()) return;
  const next = voyageLedger()?.next, view = voyageSession();
  view.port = "story";
  if (next) { view.chapter = next.index; view.mode = next.mode; }
  nav("page-stage");
  if (next && voyageContext()) voyageChest(next.chest.id);
  else document.querySelector("#voyage-unavailable:not([hidden]) button")?.focus();
}

async function syncVoyageBook(button) {
  if (!Store.ready() || button.disabled) return;
  const epoch = Store.sessionEpoch;
  button.disabled = true;
  try {
    const result = await Store.refresh();
    if (!result || epoch !== Store.sessionEpoch || !Store.ready() || document.body.dataset.page !== "page-stage") return;
    renderStage(); renderTopbar();
    if (!voyageContext()) toast("当前服务器尚未提供三难度副本与星级宝箱");
  } finally { if (button.isConnected && epoch === Store.sessionEpoch) button.disabled = false; }
}

function voyageStage(id) {
  const local = STAGES.find(stage => stage.id === id);
  const point = Store.profile?.voyage?.stages?.[id];
  if (!point) return local;
  return { ...local, ...point, type: VOYAGE_MODES[point.kind] || "资源航线", desc: point.description || local?.desc || "" };
}

function voyageGate(point) {
  if (!point) return "副本资料待同步";
  const reasons = [];
  if ((point.captain_min || 0) > (SAVE.captain?.level || 1)) reasons.push(`船长达到 Lv.${point.captain_min}`);
  for (const id of new Set([...(point.requires || []), ...(point.unlockStage ? [point.unlockStage] : [])])) {
    if (!SAVE.cleared[id]) reasons.push(`通关「${voyageStage(id)?.name || id}」`);
  }
  if (point.chapter_gate && !SAVE.voyage_passes?.[point.id] && !SAVE.cleared[point.id] && !SAVE.cleared[point.chapter_gate]) reasons.push(`完成前章「${voyageStage(point.chapter_gate)?.name || point.chapter_gate}」`);
  return reasons.join("、");
}

function voyageContext() {
  const view = voyageSession(), book = Store.profile?.voyage;
  if (!book?.chapters?.length || !book.stages || !book.resources?.length) return null;
  const chapter = book.chapters[view.chapter] || book.chapters[0];
  if (view.port === "resources") return { view, book, chapter, ids: book.resources, title: "群岛补给线", caption: "定向物资 · 每港每日三次", chests: [], region: 2 };
  if (view.port === "rifts") {
    const rift = CHAPTERS[4 + view.rift];
    return { view, book, chapter, ids: rift.stages, title: rift.name, caption: "灵印打捞 · 六系守门人", chests: [], region: 3 };
  }
  return { view, book, chapter, ids: chapter.routes[view.mode], title: chapter.name, caption: `第 ${view.chapter + 1} 海域 · ${chapter.landmark}`, chests: chapter.chests[view.mode], region: view.chapter };
}

function voyageChart(context) {
  const {view,ids,region,title,chapter} = context;
  const stages = ids.map((id,index) => {
    const point = voyageStage(id), count = Math.max(1,Math.min(3,point.waves*point.enemyCount));
    return {...point, number:index+1, elite:point.hasBoss && index < ids.length-1, portrait:ENEMY_TPL[point.hasBoss ? ENEMY_TPL.length-1 : index%count].img,
      material:view.port === "resources" && point.prize?.length ? voyageItem(point.prize[0].item) : ""};
  });
  const selectedId = ids.includes(view.selected) ? view.selected : ids.find(id => !SAVE.cleared[id] && !voyageGate(voyageStage(id))) || ids[0];
  return VoyageMap.render({chapter:{id:chapter.id,region,number:view.chapter+1,name:title,landmark:view.port === "story" ? chapter.landmark : title},mode:{id:view.mode},stages,selectedId,
    getProgress:id => ({stars:voyageTotal([id])}), getGate:id => { const reason = voyageGate(voyageStage(id)); return {open:!reason,reason}; }});
}

function voyageItem(id) {
  return id === "gold" ? "金币" : Store.profile?.items?.items?.[id]?.name || id;
}

function voyagePrizes(prizes) {
  return (prizes || []).map(prize => `<span class="voy-prize">${esc(voyageItem(prize.item))} <b>×${esc(prize.amount)}</b></span>`).join("");
}

function voyageTotal(ids, state = SAVE) {
  return ids.reduce((sum, id) => sum + (state.cleared?.[id] ? Math.max(0, Math.min(3, Number(state.stars?.[id]) || 0)) : 0), 0);
}

function voyageIcon(id) {
  const paths = {story:'<circle cx="12" cy="12" r="9"/><path d="m16 8-2.5 5.5L8 16l2.5-5.5Z"/>',hard:'<path d="m4 3 17 17m-7 0 6-6M3 4l1 5 5-5Zm17-1L3 20m0-6 6 6m11-17-1 6-5-5Z"/>',nightmare:'<path d="M12 2c1 6 7 7 7 13a7 7 0 0 1-14 0c0-4 3-7 4-8 0 4 2 4 2 4s3-4 1-9Z"/>',lock:'<rect x="5" y="10" width="14" height="11" rx="2"/><path d="M8 10V7a4 4 0 0 1 8 0v3m-4 4v3"/>',gold:'<circle cx="12" cy="12" r="9"/><path d="M10 5v14m-3-3h7a3 3 0 0 0 0-6H8m0-4h5a2 2 0 0 1 0 4"/>',token:'<path d="m3 8 4-5h10l4 5-9 13Zm0 0h18M7 3l5 18 5-18M7 8l5-5 5 5"/>',stamina:'<path d="M13 2 4 14h7l-1 8 10-13h-8Z"/>',star:'<path d="m12 2 3 6 7 1-5 5 1 7-6-3-6 3 1-7-5-5 7-1Z"/>',chest:'<path d="M3 11V7a3 3 0 0 1 3-3h12a3 3 0 0 1 3 3v4M3 11h18v9H3Zm4-7v7m10-7v7"/><rect x="10" y="10" width="4" height="5" rx="1"/>',check:'<path d="m5 12 4 4L19 6"/>',back:'<path d="M20 12H4m6-6-6 6 6 6"/>',next:'<path d="M4 12h16m-6-6 6 6-6 6"/>'};
  return `<svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${paths[id]}</svg>`;
}

function voyageChestPanel(chests,total,maximum) {
  return `<section id="starRewards" class="star-panel voy-chests" aria-label="副本星级宝箱"><div class="star-top"><h2>${voyageIcon("chest")}副本星级宝箱</h2><span aria-label="本章此难度 ${total} / ${maximum} 星">${voyageIcon("star")}<b>${total}</b><small> / ${maximum}</small></span></div><nav class="chest-grid" aria-label="三档星级奖励">${chests.map((chest,index) => {
    const claimed = !!SAVE.voyage_claims?.[chest.id], ready = !claimed && total >= chest.stars;
    const status = claimed ? "已领取" : ready ? "可领取" : `还差 ${chest.stars-total} 星`;
    return `<button type="button" data-voy-chest="${esc(chest.id)}" data-tier="${index+1}" class="chest-button ${claimed ? "claimed" : ready ? "ready" : ""}" aria-label="${chest.stars} 星宝箱，${status}"><span class="chest-art">${voyageIcon(claimed ? "check" : "chest")}</span><span><b>${chest.stars} <span aria-hidden="true">★</span></b><small>${status}</small></span></button>`;
  }).join("")}</nav><progress class="star-track" value="${total}" max="${maximum}" aria-label="章节星级进度"></progress></section>`;
}

function renderVoyage() {
  const mount = document.getElementById("voyage-map"), context = voyageContext();
  const fallback = document.getElementById("voyage-unavailable");
  if (fallback) fallback.hidden = !!context;
  if (!mount || !context) {
    clearTimeout(voyageDayTimer);
    delete document.body.dataset.voyage;
    if (mount) { mount.hidden = true; mount.replaceChildren(); }
    return false;
  }
  const { view, chapter, chests, ids, book, title, caption } = context;
  const total = voyageTotal(ids), story = view.port === "story", maxStars = 3 * ids.length;
  const tabs = story ? Object.entries(VOYAGE_MODES).map(([id, label]) => {
    const gate = voyageGate(voyageStage(chapter.routes[id][0]));
    return `<button type="button" class="difficulty-tab ${{story:"gold",hard:"purple",nightmare:"red"}[id]} ${id === view.mode ? "active" : ""}" data-voy-mode="${id}" aria-pressed="${id === view.mode}" aria-label="${label}${gate ? "，未解锁" : ""}" title="${esc(gate ? `解锁条件：${gate}` : label)}">${voyageIcon(id)}<span>${label}</span>${gate ? `<span class="difficulty-lock">${voyageIcon("lock")}</span>` : ""}</button>`;
  }).join("") : view.port === "rifts" ? ["普通裂隙","噩梦回响"].map((label, index) => `<button type="button" class="difficulty-tab ${view.rift === index ? "active" : ""}" data-voy-rift="${index}" aria-pressed="${view.rift === index}">${label}</button>`).join("") : `<span class="voy-port-label">定向补给 · 北京零点换日</span>${Store.profile?.market ? '<button type="button" class="difficulty-tab" onclick="openGameMarket(\'dungeon\')">副本商店 ›</button>' : ""}`;
  const chestPanel = story ? voyageChestPanel(chests,total,maxStars) : `<section class="voy-legend"><strong>${view.port === "resources" ? "补给清单" : "裂隙图谱"}</strong><span>${view.port === "resources" ? "每港独立计次 · 胜利取得指定物资" : `已突破 ${ids.filter(id=>SAVE.cleared[id]).length} / ${ids.length} 道裂隙`}</span></section>`;
  const arrows = story ? `<nav class="map-chapter-controls" aria-label="切换章节">${view.chapter + 1 < book.chapters.length ? `<button type="button" class="chapter-step chapter-step-next" data-voy-step="1" title="下一章 · ${esc(book.chapters[view.chapter+1].name)}" aria-label="下一章">${voyageIcon("next")}</button>` : ""}${view.chapter ? `<button type="button" class="chapter-step chapter-step-prev" data-voy-step="-1" title="上一章" aria-label="上一章">${voyageIcon("back")}</button>` : ""}</nav>` : "";
  document.body.dataset.voyage = "open";
  mount.hidden = false;
  mount.dataset.region = context.region;
  mount.innerHTML = `<section class="map-panel" aria-label="副本地图"><div class="map-scroll"><div id="seaMap" class="sea-map" tabindex="-1" role="group" aria-label="${esc(title)}关卡">${voyageChart(context)}</div></div>
    <button class="map-back" type="button" data-voy-home aria-label="返回星霜港" title="返回星霜港">${voyageIcon("back")}</button>
    <div class="map-status"><details class="voy-port-menu"><summary class="map-pill voy-heading" title="切换副本分类">${voyageIcon("story")}<span><b>${esc(title)} ▾</b><small>${VOYAGE_PORTS[view.port]} · ${story ? VOYAGE_MODES[view.mode] : esc(caption)}</small></span></summary><nav aria-label="副本分类">${Object.entries(VOYAGE_PORTS).map(([id,label]) => `<button type="button" data-voy-port="${id}" aria-pressed="${view.port === id}">${label}</button>`).join("")}</nav></details>${story ? `<span class="map-star-count">${voyageIcon("star")}${total}<small> / ${maxStars}</small></span>` : ""}</div>
    <div id="wallet" class="wallet-bar" aria-label="副本资源">${[["gold","金币",SAVE.gold],["token","代币",SAVE.token]].map(([id,label,value]) => `<span class="wallet-item" aria-label="${label} ${esc(value)}">${voyageIcon(id)}<span class="resource-name">${label}</span><b>${esc(value)}</b></span>`).join("")}<span class="wallet-item" aria-label="体力 ${esc(SAVE.stamina)} / ${esc(SAVE.staminaMax)}">${voyageIcon("stamina")}<span class="resource-name">体力</span><b>${esc(SAVE.stamina)}<small> / ${esc(SAVE.staminaMax)}</small></b><button type="button" class="wallet-plus" data-voy-stamina aria-label="补充体力" title="前往时段体力补给">+</button></span></div>
    ${arrows}<div class="map-bottom-layer"><nav id="difficultyNav" class="difficulty-nav" aria-label="关卡难度">${tabs}</nav>${chestPanel}</div><span class="voy-status" role="status"></span></section>`;
  mount.onclick = voyageClickMap;
  updateStageRunLimits();
  return true;
}

function voyageClickMap(event) {
  const button = event.target.closest("button");
  if (!button || !requireGame()) return;
  const data = button.dataset, context = voyageContext();
  if (!context) return renderStage();
  if ("voyHome" in data) return nav("page-home");
  if ("voyStamina" in data) return nav("page-captain");
  if (data.voyPort) return openVoyage(data.voyPort);
  if (data.voyPoint) { context.view.selected = data.voyPoint; renderVoyage(); return voyageDetail(data.voyPoint); }
  if (data.voyChest) return voyageChest(data.voyChest);
  if (data.voyMode && Object.hasOwn(VOYAGE_MODES, data.voyMode)) context.view.mode = data.voyMode;
  if (data.voyRift === "0" || data.voyRift === "1") context.view.rift = Number(data.voyRift);
  if (data.voyStep) {
    const to = context.view.chapter + Number(data.voyStep), next = context.book.chapters[to];
    if (!next) return;
    const gate = voyageGate(voyageStage(next.routes[context.view.mode][0]));
    if (gate && to > context.view.chapter) {
      const notice = document.querySelector("#voyage-map .voy-status");
      if (notice.dataset.voyDayStatus) return;
      notice.textContent = `下一章需${gate}`;
      clearTimeout(voyageNoticeTimer);
      voyageNoticeTimer = setTimeout(() => { notice.textContent = ""; }, 3500);
      return;
    }
    context.view.chapter = to;
  }
  renderVoyage();
  const group = ["voyMode", "voyRift", "voyStep"].find(key => key in data);
  if (group) Array.from(document.querySelectorAll("#voyage-map button")).find(candidate => candidate.dataset[group] === data[group])?.focus();
}

function voyageRemaining(point) {
  return point.daily_limit ? Math.max(0, point.daily_limit - (SAVE.daily.voyage_attempts?.[point.id] || 0)) : null;
}

function voyageDetail(id) {
  const point = voyageStage(id), context = voyageContext();
  if (!point || !context?.ids.includes(id)) return;
  const gate = voyageGate(point), left = voyageRemaining(point), best = voyageTotal([id]), date = SAVE.daily.date;
  const milestones = context.view.port === "story" ? `<div class="voy-milestones"><p>首胜${SAVE.cleared[id] ? " · 已达成" : ""} ${voyagePrizes(context.book.first_clear)}</p><p>首次三星${best === 3 ? " · 已达成" : ""} ${voyagePrizes(context.book.first_three)}</p></div>` : "";
  const waves = Array.from({length: Math.min(point.waves, 8)}, (_, wave) => {
    const boss = point.hasBoss && wave === point.waves-1;
    return boss ? [ENEMY_TPL.at(-1)] : Array.from({length:point.enemyCount}, (_, enemy) => ENEMY_TPL[(wave * point.enemyCount + enemy) % 3]);
  });
  const enemies = [...new Set(waves.flat())].map(enemy => `<figure><img src="${esc(enemy.img)}" alt=""><figcaption>${esc(enemy.name)}</figcaption></figure>`).join("");
  const mobs = waves.map((wave,index) => `<p>第 ${index+1} 波 · ${wave.map(enemy => esc(enemy.name)).join("、")}</p>`).join("");
  const rewards = point.prize ? `<div class="voy-drop-grid">${voyagePrizes(point.prize)}</div><p>出战伙伴经验 +${point.exp}</p>` : `<div class="voy-legacy-loot">${legacyStageDetailHTML(point, true)}</div>`;
  const sweep = point.sweep_prize?.length ? `<section class="voy-sweep"><h4>三星扫荡</h4><div class="voy-drop-grid">${voyagePrizes(point.sweep_prize)}</div><p>每次直接获得上述固定物资，消耗 ${point.cost} 体力${point.daily_limit ? "及一次挑战次数" : ""}，并计入每日“三程巡航”。不发站点额度、首胜、首次三星或额外战斗奖励。</p><div class="voy-sweep-buttons">${[1,3].map(count => `<button type="button" class="gbtn gbtn-sm" data-voy-sweep="${count}"><span>扫荡 ${count} 次 · ${point.cost*count} 体力</span></button>`).join("")}</div><small data-voy-sweep-status></small></section>` : "";
  const mechanic = DUNGEON_MECHANICS[point.mechanic];
  showModal(`<h3>${esc(point.name)}</h3><article class="voy-detail" data-voy-detail="${esc(id)}"><header><b>${esc(point.type)} · 敌人 Lv.${point.enemyLv}</b><span>历史最佳 ${starsHTML(best, 13)}</span></header><div class="voy-detail-grid"><figure class="voy-stage-art"><img src="${esc(point.img)}" alt=""><figcaption>${esc(point.desc)}</figcaption></figure><div><h4>三星挑战条件</h4><ul class="voy-star-rules"><li>战斗胜利</li><li>全员存活</li><li>${point.starRound} 回合内取胜</li></ul><h4>敌人阵容</h4><div class="voy-enemy-row">${enemies}</div></div></div><details class="voy-waves"><summary>波次情报 · ${point.waves} 波</summary>${mobs}${mechanic ? `<p>${esc(mechanic)}</p>` : ""}</details><h4>胜利奖励</h4>${rewards}${milestones}${sweep}<div class="voy-meta"><b>体力 ${point.cost}</b>${left !== null ? `<b data-voy-left>今日余 ${left}/${point.daily_limit} 次</b>` : ""}<span>当前 ${SAVE.stamina} 体力</span></div><p class="voy-gate" role="status">${gate ? `解锁条件：${esc(gate)}` : ""}</p><div class="modal-btns"><button class="gbtn gbtn-sm" type="button" data-voy-formation><span>布阵</span></button><button class="gbtn gbtn-primary" type="button" data-voy-start="${esc(id)}"><span>挑战 · ${point.cost} 体力</span></button></div></article>`);
  const detail = document.querySelector("[data-voy-detail]");
  detail.dataset.voyDay = date;
  detail.dataset.voyEpoch = String(Store.sessionEpoch);
  detail.querySelector("[data-voy-formation]").onclick = () => { closeModal(); nav("page-squad"); };
  const start = detail.querySelector("[data-voy-start]");
  start.onclick = () => {
    if (!voyageCurrentDetail(detail) || start.disabled) return;
    closeModal(); startBattle(id);
  };
  detail.querySelectorAll("[data-voy-sweep]").forEach(button => { button.onclick = () => voyageSweep(detail, id, Number(button.dataset.voySweep)); });
  updateStageRunLimits();
  document.querySelector("#modal-box .mb-close")?.focus();
}

function voyageUpdateRunButton(seconds, retry = false) {
  if (!Store.ready() || document.body.dataset.page !== "page-stage" || document.body.dataset.voyage !== "open" || !Store.profile?.voyage) return;
  const dayStatus = voyageDayStatus(retry), mapNotice = document.querySelector("#voyage-map .voy-status");
  if (dayStatus) clearTimeout(voyageNoticeTimer);
  voyageDayNotice(mapNotice, dayStatus);
  const detail = document.querySelector("#modal-mask:not(.hidden) [data-voy-detail]"), button = detail?.querySelector("[data-voy-start]");
  if (!button) return;
  const point = voyageStage(button.dataset.voyStart);
  if (!point || detail.dataset.voyEpoch !== String(Store.sessionEpoch)) { closeModal(); return; }
  if (!dayStatus && detail.dataset.voyDay !== SAVE.daily.date) { voyageDetail(point.id); return; }
  const gate = voyageGate(point);
  const left = voyageRemaining(point), leftLabel = detail.querySelector("[data-voy-left]"), gateNote = detail.querySelector(".voy-gate");
  if (leftLabel) leftLabel.textContent = dayStatus ? "今日次数待同步" : `今日余 ${left}/${point.daily_limit} 次`;
  const reason = dayStatus || (gate ? `解锁条件：${gate}` : left === 0 ? "今日次数已用完，北京时间零点刷新" : seconds ? `连续挑战休整中，${seconds} 秒后可出战` : SAVE.stamina < point.cost ? "体力不足，可回港领取时段补给" : "");
  button.disabled = !!reason;
  voyageDayNotice(gateNote, dayStatus);
  if (!dayStatus) gateNote.textContent = reason;
  const sweepStatus = detail.querySelector("[data-voy-sweep-status]");
  if (sweepStatus) {
    const deny = dayStatus || (voyageTotal([point.id]) < 3 ? "本航点取得三星后可扫荡" : Store.activeRun ? "请先结算进行中的战斗" : reason);
    detail.querySelectorAll("[data-voy-sweep]").forEach(control => {
      const count = Number(control.dataset.voySweep);
      control.disabled = !!deny || Store.mutationBusy || SAVE.stamina < point.cost * count || left !== null && left < count;
    });
    sweepStatus.textContent = deny || "";
  }
}

async function voyageSweep(detail, id, count) {
  const button = detail.querySelector(`[data-voy-sweep="${count}"]`);
  if (!button || !requireGame() || !voyageCurrentDetail(detail) || button.disabled) return;
  const epoch = Store.sessionEpoch;
  detail.querySelectorAll("[data-voy-sweep]").forEach(control => { control.disabled = true; });
  const receipt = await Store.mutate("voyage_sweep", {item_id:id, count, choice:detail.dataset.voyDay});
  if (epoch !== Store.sessionEpoch || !Store.ready()) return;
  if (!receipt) { if (detail.isConnected) updateStageRunLimits(); return; }
  renderTopbar();
  if (document.body.dataset.page === "page-stage") {
    renderStage();
    if (detail.isConnected && !document.getElementById("modal-mask").classList.contains("hidden")) voyageDetail(id);
  }
  toast((receipt.drops || []).join(" · ") || "扫荡已结算");
}

function voyageChest(id) {
  const context = voyageContext(), chest = context?.chests.find(candidate => candidate.id === id);
  if (!chest) return;
  const stars = voyageTotal(context.ids), claimed = !!SAVE.voyage_claims?.[id], epoch = Store.sessionEpoch;
  showModal(`<h3>${esc(context.title)} · ${chest.stars} 星宝箱</h3><article class="voy-detail" data-voy-chest-detail="${esc(id)}"><p>${esc(VOYAGE_MODES[context.view.mode])} · 本章已收集 ${stars} / ${context.ids.length*3} 星</p><div class="voy-drop-grid">${voyagePrizes(chest.prize)}</div><p>各关只计历史最高星。每章、每个难度的宝箱独立计星，每档限领一次，领取后物资立即到账。</p><div class="modal-btns"><button class="gbtn gbtn-primary" type="button" data-voy-claim ${claimed || stars < chest.stars ? "disabled" : ""}><span>${claimed ? "已领取" : stars < chest.stars ? `还差 ${chest.stars - stars} 星` : "领取奖励"}</span></button></div></article>`);
  const button = document.querySelector("[data-voy-claim]");
  button.onclick = async () => {
    if (button.disabled || !Store.ready() || epoch !== Store.sessionEpoch || document.body.dataset.page !== "page-stage" || button !== document.querySelector("#modal-mask:not(.hidden) [data-voy-claim]")) return;
    button.disabled = true;
    const result = await Store.mutate("voyage_chest", {item_id:id});
    if (epoch !== Store.sessionEpoch || !Store.ready()) return;
    if (result) {
      if (button.isConnected) closeModal();
      if (document.body.dataset.page === "page-stage") renderStage();
      renderTopbar(); toast((result.drops || []).join(" · ") || "副本星级奖励已到账");
    } else if (button.isConnected) button.disabled = false;
  };
  document.querySelector("#modal-box .mb-close")?.focus();
}

function voyageResultSummary(id, result) {
  const book = Store.profile?.voyage;
  if (!book || !result.win || !result.save?.stars) return "";
  for (const chapter of book.chapters) {
    for (const [mode, ids] of Object.entries(chapter.routes)) {
      if (!ids.includes(id)) continue;
      const total = voyageTotal(ids, result.save), ready = chapter.chests[mode].filter(chest => total >= chest.stars && !result.save.voyage_claims?.[chest.id]);
      return `<p class="voy-result">${esc(chapter.name)} · ${esc(VOYAGE_MODES[mode])} ${total}/${ids.length*3} ★${ready.length ? ` · ${ready.length} 档星箱可领` : ""}</p>`;
    }
  }
  return "";
}

window.addEventListener("resize", () => {
  const world = document.querySelector("#voyage-map .voyage-world");
  if (world && document.body.dataset.page === "page-stage" && Store.ready() && world.dataset.layout !== VoyageMap.detectLayout()) renderStage();
});

document.addEventListener("keydown", event => {
  if (event.key !== "Escape" || document.body.dataset.page !== "page-stage" || document.getElementById("modal-mask")?.classList.contains("hidden")) return;
  const point = document.querySelector("[data-voy-detail]")?.dataset.voyDetail, chest = document.querySelector("[data-voy-chest-detail]")?.dataset.voyChestDetail;
  if (!point && !chest) return;
  closeModal();
  if (point) Array.from(document.querySelectorAll("[data-voy-point]")).find(button => button.dataset.voyPoint === point)?.focus();
  if (chest) Array.from(document.querySelectorAll("[data-voy-chest]")).find(button => button.dataset.voyChest === chest)?.focus();
});

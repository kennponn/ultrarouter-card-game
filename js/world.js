let worldPoolState = null;

function currentWorldPool() {
  if (!worldPoolState || worldPoolState.epoch !== Store.sessionEpoch || worldPoolState.owner !== Store.authUserId) {
    worldPoolState = {epoch: Store.sessionEpoch, owner: Store.authUserId, view: null, pending: null, busy: false, error: "", drops: [], readAt: 0, tab: "rank", panel: ""};
  }
  return worldPoolState;
}

function worldPoolVisible() {
  return Store.ready() && document.body.dataset.page === "page-gacha" && drawPoolState().id === "world";
}

function paintWorldIfVisible(state) {
  if (state === currentWorldPool() && worldPoolVisible()) renderWorldPool(document.getElementById("artifact-draw"));
}

function refreshWorldPool() {
  const state = currentWorldPool();
  if (!worldPoolVisible() || state.pending) return state.pending;
  state.error = "";
  state.pending = Promise.resolve().then(() => Store.api(SITE.ep.world, "GET")).then(view => {
    if (state !== currentWorldPool() || !Store.ready()) return;
    if (!view?.me || !GIRLS.some(girl => girl.id === view.hero) || !Array.isArray(view.board) || !Array.isArray(view.chests) || !Array.isArray(view.tiers)) throw new Error("服务器活动信息不完整");
    state.view = view;
  }).catch(error => { state.error = error.message || "无法同步强者世界"; }).finally(() => {
    state.readAt = Store.serverNow();
    state.pending = null;
    paintWorldIfVisible(state);
  });
  return state.pending;
}

function worldActionButton(action, week, amount, label, disabled = false) {
  return `<button type="button" class="sw-small-button" data-world-action="${action}" data-week="${esc(week)}"${amount === null ? "" : ` data-count="${amount}"`}${disabled ? " disabled" : ""}>${esc(label)}</button>`;
}

function worldRankBadge(rank, end = rank) {
  return `<span class="sw-rank-badge${rank !== end ? " is-range" : ""}" data-rank="${rank <= 3 && rank === end ? rank : 4}">${rank === end ? rank : `${rank}–${end}`}</span>`;
}

function worldBoardMarkup(rows) {
  if (!rows?.length) return '<div class="sw-empty"><span aria-hidden="true">✦</span><p>虚位以待</p><small>参与召唤，登上强者榜！</small></div>';
  return `<ol class="sw-ranking">${rows.map(entry => `<li class="sw-rank-row${entry.self ? " is-self" : ""}">${worldRankBadge(entry.rank)}<span class="sw-player-name">${esc(entry.name)}${entry.self ? '<small>（我）</small>' : ""}</span><span class="sw-rank-score" data-rank="${Math.min(entry.rank,4)}"><b>${entry.points}</b> 积分</span></li>`).join("")}</ol>`;
}

function worldTierMarkup(view, hero) {
  return `<ol class="sw-ranking sw-reward-list">${view.tiers.map((tier,index) => {
    const from = index ? view.tiers[index-1].through + 1 : 1;
    return `<li class="sw-rank-row">${worldRankBadge(from,tier.through)}<span class="sw-tier-caption">${from === tier.through ? `第 ${from} 名` : `第 ${from}–${tier.through} 名`}</span><span class="sw-shard-prize"><img src="${esc(hero.img)}" alt=""><span>${esc(hero.name)}碎片<b>×${tier.shards}</b></span></span></li>`;
  }).join("")}</ol>`;
}

function worldChestMarkup(entry, chests, busy, past = false) {
  return chests.map((chest,index) => {
    const claimed = Boolean((entry.chest_mask || 0) & (1 << index)), reached = entry.points >= chest.points;
    if (past && (claimed || !reached)) return "";
    const kind = Math.min(3, 1 + Math.floor(index * 3 / chests.length)), status = claimed ? "已领取" : reached ? "领取" : "未达成";
    const prize = `碎片 ×${chest.shards}${chest.tickets ? ` · 限定券 ×${chest.tickets}` : ""}`;
    return `<button type="button" class="sw-chest${reached && !claimed ? " is-ready" : ""}${claimed ? " is-claimed" : ""}" data-world-action="world_chest" data-week="${esc(entry.week)}" data-count="${chest.points}" aria-label="${chest.points} 积分宝箱，${esc(prize)}，${status}" title="${esc(prize)}"${busy || claimed || !reached ? " disabled" : ""}><span class="sw-chest-points">${chest.points} 积分</span><img src="assets/world/box_${kind}${claimed ? "_open" : ""}.webp" alt=""><span class="sw-chest-prize">${prize}</span><strong>${status}</strong></button>`;
  }).join("");
}

function worldRankClaim(entry, busy) {
  if (!(entry.final_rank > 0)) return "";
  const hero = GIRLS.find(girl => girl.id === entry.hero);
  return worldActionButton("world_rank", entry.week, null, `第 ${entry.final_rank} 名 · ${hero?.name || "英雄"}碎片 ${entry.prize_shards} · ${entry.rank_claimed ? "已领取" : "领取排名奖励"}`, busy || entry.rank_claimed);
}

function worldPanelMarkup(state, view, hero, busy) {
  if (!state.panel) return "";
  let title, content;
  if (state.panel === "rules") {
    title = "活动规则";
    const names = GIRLS.filter(girl => girl.rarity === "R" && cardSources(girl).includes("world")).map(girl => esc(girl.name)).join("、");
    content = `<p>北京时间每周一 00:00 至周五 24:00 开放召唤，周末休息。每周轮换一位限定 SSR。</p><h4>召唤与积分</h4><p>每抽消耗 1 张限定券，获得 1 积分。每抽 ${view.shard_percent}% 获得 1 枚${esc(hero.name)}碎片，否则获得一张 R 整卡；不会抽到 SR 或 SSR 整卡，不计入常驻保底。</p><p>同卡碎片集齐 ${RARITY.SSR.shardToUnlock} 枚后，可在图鉴主动合成。重复整卡保留在角色工坊，可自行分解。</p><h4>排名奖励</h4><p>全服前 20 名获得本期英雄碎片。同分先达到者优先，再同分按玩家编号；最终奖励以结算名次为准。积分宝箱与往期未领取奖励可分别领取。</p><h4>限定券来源</h4><p>每日任务积分箱、竞技场排名和本活动积分箱。</p><details><summary>查看 R 整卡名单</summary><p>${names}</p></details>`;
  } else if (state.panel === "history") {
    title = "往期领奖";
    content = (view.unclaimed || []).map(entry => {
      const girl = GIRLS.find(candidate => candidate.id === entry.hero);
      if (!girl) return "";
      return `<section class="sw-past-season"><h4>${esc(entry.week)} · ${esc(girl.name)}</h4><p>本期积分：${entry.points}</p><div class="sw-past-chests">${worldChestMarkup(entry,entry.chests || view.chests,busy,true)}</div>${worldRankClaim(entry,busy)}</section>`;
    }).join("") || '<p>暂无待领取的往期奖励。</p>';
    if (view.previous) content += `<h4>${esc(view.previous.week)} 周 · 上期榜单</h4>${worldBoardMarkup(view.previous_board)}`;
  } else {
    title = "奖励一览";
    content = `<h4>${esc(hero.name)} · 本期排名奖励</h4>${worldTierMarkup(view,hero)}<h4>积分宝箱</h4><div class="sw-past-chests">${worldChestMarkup(view.me,view.chests,busy)}</div>${worldRankClaim(view.me,busy)}`;
  }
  return `<div class="sw-overlay"><section class="sw-dialog" role="dialog" aria-modal="true" aria-labelledby="sw-dialog-title" tabindex="-1"><header><h3 id="sw-dialog-title">${title}</h3><button type="button" id="sw-panel-close" class="sw-close" data-world-action="close-panel" aria-label="关闭${title}">×</button></header><div class="sw-dialog-scroll">${content}</div></section></div>`;
}

function worldRemaining(end, now) {
  const seconds = Math.max(0, Math.floor((end-now)/1000));
  return `${Math.floor(seconds/86400)}天 ${String(Math.floor(seconds/3600)%24).padStart(2,"0")}:${String(Math.floor(seconds/60)%60).padStart(2,"0")}:${String(seconds%60).padStart(2,"0")}`;
}

function renderWorldPool(body) {
  if (!body || !worldPoolVisible()) return;
  const state = currentWorldPool(), view = state.view, now = Store.serverNow();
  const focusId = body.contains(document.activeElement) ? document.activeElement.id : "";
  const listScroll = body.querySelector(".sw-board-scroll")?.scrollTop || 0;
  const panelScroll = body.querySelector(".sw-dialog-scroll")?.scrollTop || 0;
  body.onclick = worldPoolClick;
  body.onkeydown = worldPoolKeydown;
  if (!state.pending && !state.busy && !state.error && (!view || now - state.readAt >= 30000)) refreshWorldPool();
  const reload = `<button type="button" id="sw-refresh" class="sw-refresh" data-world-action="refresh"${state.pending || state.busy ? " disabled" : ""}><span aria-hidden="true">↻</span>${state.pending ? "同步中" : "刷新榜单"}</button>`;
  const status = state.error ? `<div class="sw-status" role="status">${esc(state.error)} · 请重新同步。${view ? "" : reload}</div>` : "";
  if (!view) { body.innerHTML = `<section class="sw-loading"><img src="assets/world/title_shenshou.webp" alt="强者世界">${status || '<p role="status">正在读取本周英雄与榜单…</p>'}${state.error ? "" : reload}</section>`; return; }
  const hero = GIRLS.find(girl => girl.id === view.hero), own = SAVE.girls[hero.id], tickets = SAVE.mats.limited_ticket || 0;
  const busy = state.busy || Boolean(state.pending) || Boolean(state.error), open = view.open && now < view.closes_at;
  const next = new Intl.DateTimeFormat("zh-CN", {timeZone:"Asia/Shanghai", month:"numeric", day:"numeric"}).format(view.next_open);
  const rank = view.settled ? view.me.final_rank : view.board.find(entry => entry.self)?.rank;
  const maxPoints = view.chests.at(-1)?.points || 1;
  // Chests are spaced evenly, so interpolate between the surrounding thresholds.
  const nextChest = view.chests.findIndex(chest => chest.points > view.me.points);
  const progress = nextChest < 0 ? 100 : nextChest === 0 ? 0 : 100 * (nextChest - 1 + (view.me.points - view.chests[nextChest-1].points) / (view.chests[nextChest].points - view.chests[nextChest-1].points)) / Math.max(1,view.chests.length-1);
  body.innerHTML = `<section class="sw-frame" aria-label="强者世界"${state.panel ? " inert" : ""}>
    <header class="sw-header"><img class="sw-title" src="assets/world/title_shenshou.webp" alt="强者世界"><span class="sw-season">${esc(view.week)} 周 · ${open ? "限时召唤" : "本期已结束"}</span><span class="sw-wallet"><span aria-hidden="true">券</span>限定券 <b>×${tickets}</b></span><button type="button" class="sw-close sw-exit" data-world-action="exit" aria-label="返回常驻召唤">×</button></header>
    ${status}
    <div class="sw-main"><section class="sw-showcase" aria-label="本周限定英雄">
      <button type="button" id="sw-rules" class="sw-icon-button sw-rules-button" data-world-action="rules"><img src="assets/world/btn_gz_icon.webp" alt=""><span>规则</span></button>
      <div class="sw-hero-caption"><span>本周限定 SSR</span><h2>${esc(hero.name)}</h2><span class="sw-hero-stars" aria-label="SSR">★★★★★</span></div>
      <img class="sw-hero-art" src="${esc(hero.battleImg || hero.img)}" alt="${esc(hero.name)}立绘" fetchpriority="high">
      <button type="button" class="sw-icon-button sw-handbook" data-world-action="card"><img src="assets/world/icon_tj@.webp" alt=""><span>图鉴</span></button>
      <div class="sw-hero-shards">同卡碎片 <b>${own?.shards || 0}/${RARITY.SSR.shardToUnlock}</b><span>${own?.star ? "已拥有" : "集齐可合成"}</span></div>
      <div class="sw-summon-controls"><div class="sw-summon-tip">每次召唤必得 R 整卡或本期英雄碎片</div><div class="sw-draw-buttons">${[1,10].map(amount => `<button type="button" id="sw-pull-${amount}" class="sw-draw-button sw-draw-${amount}" data-world-action="world_pull" data-week="${esc(view.week)}" data-count="${amount}"${busy || !open || tickets < amount ? " disabled" : ""}><b>抽取${amount}次</b><span>消耗 <strong>${amount}</strong> 张限定券</span></button>`).join("")}</div><div class="sw-pull-note">${open ? `每抽积 1 分 · ${view.shard_percent}% 获得限定碎片` : `周末休息 · 下期 ${next} 周一 00:00 开放`}</div></div>
    </section>
    <section class="sw-board" aria-label="强者榜"><div class="sw-deadline"><span>${open ? "召唤倒计时" : "下期开放倒计时"}</span><strong data-world-countdown="${open ? view.closes_at : view.next_open}">${worldRemaining(open ? view.closes_at : view.next_open,now)}</strong><button type="button" id="sw-prizes" class="sw-icon-button sw-gift" data-world-action="prizes" aria-label="奖励一览"><img src="assets/world/btn_gift.webp" alt=""></button></div>
      <div class="sw-tabs" role="tablist" aria-label="榜单与奖励"><button type="button" id="sw-tab-rank" role="tab" aria-controls="sw-rank-panel" aria-selected="${state.tab === "rank"}" tabindex="${state.tab === "rank" ? 0 : -1}" data-world-action="tab-rank">积分排行</button><button type="button" id="sw-tab-awards" role="tab" aria-controls="sw-awards-panel" aria-selected="${state.tab === "awards"}" tabindex="${state.tab === "awards" ? 0 : -1}" data-world-action="tab-awards">排名奖励</button></div>
      <div class="sw-board-caption"><span>全服前 20 · ${view.settled ? "已结算" : "实时积分榜"}</span>${reload}</div>
      <div class="sw-board-scroll"><div id="sw-rank-panel" role="tabpanel" aria-labelledby="sw-tab-rank"${state.tab !== "rank" ? " hidden" : ""}>${worldBoardMarkup(view.board)}</div><div id="sw-awards-panel" role="tabpanel" aria-labelledby="sw-tab-awards"${state.tab !== "awards" ? " hidden" : ""}>${worldTierMarkup(view,hero)}</div></div>
      <div class="sw-my-rank"><span>我的排名：<b>${rank > 0 ? `第 ${rank} 名` : "未上榜"}</b></span><span>我的本期积分：<b>${view.me.points}</b></span></div>${worldRankClaim(view.me,busy)}<div class="sw-board-note">同分按达到时间排序 · 排名奖以结算为准</div>
    </section></div>
    <footer class="sw-milestones"><div class="sw-milestone-heading"><strong>积分奖励</strong><span>累计召唤，领取限定碎片</span><button type="button" id="sw-history" class="sw-history" data-world-action="history">往期领奖${view.unclaimed?.length ? '<i aria-label="有待领取奖励"></i>' : ""} ›</button></div><div class="sw-chest-track"><div class="sw-progress" role="progressbar" aria-label="本期积分奖励进度" aria-valuemin="0" aria-valuemax="${maxPoints}" aria-valuenow="${Math.min(view.me.points,maxPoints)}"><i style="width:${progress}%"></i></div><div class="sw-chests">${worldChestMarkup(view.me,view.chests,busy)}</div></div></footer>
    <div class="sw-receipt" aria-live="polite" aria-atomic="true">${state.busy ? "正在确认，请稍候…" : (state.drops || []).map(drop => esc(drop)).join(" · ")}</div>
  </section>${worldPanelMarkup(state,view,hero,busy)}`;
  if (focusId) document.getElementById(focusId)?.focus({preventScroll:true});
  body.querySelector(".sw-board-scroll").scrollTop = listScroll;
  if (state.panel) {
    body.querySelector(".sw-dialog-scroll").scrollTop = panelScroll;
    if (!body.querySelector(".sw-dialog").contains(document.activeElement)) document.getElementById("sw-panel-close")?.focus({preventScroll:true});
  }
}

function worldPoolKeydown(event) {
  const state = currentWorldPool();
  if (!worldPoolVisible()) return;
  if (state.panel && event.key === "Escape") { event.preventDefault(); closeWorldPanel(); return; }
  if (state.panel && event.key === "Tab") {
    const nodes = [...event.currentTarget.querySelectorAll('.sw-dialog button:not(:disabled),.sw-dialog summary')];
    const first = nodes[0], last = nodes.at(-1);
    if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus(); }
    else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus(); }
  }
  if (event.target.matches('[role="tab"]') && ["ArrowLeft","ArrowRight","Home","End"].includes(event.key)) {
    event.preventDefault();
    state.tab = event.key === "Home" ? "rank" : event.key === "End" ? "awards" : state.tab === "rank" ? "awards" : "rank";
    paintWorldIfVisible(state);
    document.getElementById(`sw-tab-${state.tab}`)?.focus();
  }
}

function closeWorldPanel() {
  const state = currentWorldPool(), source = state.panel;
  state.panel = "";
  paintWorldIfVisible(state);
  document.getElementById(`sw-${source}`)?.focus({preventScroll:true});
}

// Only the displayed clock advances locally; balances, rewards and ranks come from the server.
setInterval(() => {
  if (!worldPoolVisible()) return;
  const state = currentWorldPool(), now = Store.serverNow();
  document.querySelectorAll("[data-world-countdown]").forEach(node => { node.textContent = worldRemaining(Number(node.dataset.worldCountdown),now); });
  if (state.view?.open && now >= state.view.closes_at) {
    document.querySelectorAll('[data-world-action="world_pull"]').forEach(button => { button.disabled = true; });
    if (!state.pending && !state.busy && !state.error) refreshWorldPool();
  }
}, 1000);

async function worldPoolClick(event) {
  const button = event.target.closest?.("button[data-world-action]");
  if (!button || button.disabled || !worldPoolVisible()) return;
  const action = button.dataset.worldAction, state = currentWorldPool(), view = state.view;
  if (action === "exit") return selectDrawPool("heroes");
  if (action === "close-panel") return closeWorldPanel();
  if (action === "refresh") return refreshWorldPool();
  if (!view) return;
  if (action === "tab-rank" || action === "tab-awards") {
    state.tab = action === "tab-rank" ? "rank" : "awards";
    paintWorldIfVisible(state);
    return;
  }
  if (["rules","prizes","history"].includes(action)) {
    state.panel = action;
    paintWorldIfVisible(state);
    document.getElementById("sw-panel-close")?.focus({preventScroll:true});
    return;
  }
  if (state.busy || state.pending || state.error) return;
  if (action === "card") return typeof openCodex === "function" ? openCodex(view.hero) : openGirl(view.hero);
  if (!["world_pull","world_chest","world_rank"].includes(action)) return;
  if (action === "world_pull" && (!view.open || Store.serverNow() >= view.closes_at)) return refreshWorldPool();
  state.busy = true;
  paintWorldIfVisible(state);
  try {
    const fields = {item_id: button.dataset.week};
    if (button.dataset.count !== undefined) fields.count = Number(button.dataset.count);
    const receipt = await Store.mutate(action, fields);
    if (state !== currentWorldPool() || !Store.ready()) return;
    if (receipt) { state.drops = receipt.drops || []; renderTopbar(); }
  } finally {
    state.busy = false;
    state.readAt = 0;
    if (state === currentWorldPool() && worldPoolVisible()) await refreshWorldPool();
  }
}

async function recycleWorldCopy(id) {
  if (!requireGame() || !(SAVE.girls[id]?.copies > 0)) return;
  const state = currentWorldPool(), receipt = await Store.mutate("card_recycle_copy", {girl_id:id});
  if (receipt && state === currentWorldPool() && Store.ready()) { openGirl(id); toast((receipt.drops || []).join(" · ")); }
}

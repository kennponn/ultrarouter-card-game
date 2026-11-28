let worldPoolState = null;

function currentWorldPool() {
  if (!worldPoolState || worldPoolState.epoch !== Store.sessionEpoch || worldPoolState.owner !== Store.authUserId) {
    worldPoolState = {epoch: Store.sessionEpoch, owner: Store.authUserId, view: null, pending: null, busy: false, error: "", drops: [], readAt: 0};
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
  return `<button type="button" class="ws-command" data-world-action="${action}" data-week="${esc(week)}"${amount === null ? "" : ` data-count="${amount}"`}${disabled ? " disabled" : ""}>${esc(label)}</button>`;
}

function worldBoardMarkup(rows) {
  if (!rows?.length) return "<p>还没有玩家上榜。</p>";
  return `<ol class="ws-grade-loot">${rows.map(entry => `<li><span>第 ${entry.rank} 名 · ${esc(entry.name)}${entry.self ? "（我）" : ""}</span><strong>${entry.points} 分 · ${entry.shards} 碎片</strong></li>`).join("")}</ol>`;
}

function worldPrizes(entry, chests, busy, past = false) {
  const girl = GIRLS.find(candidate => candidate.id === entry.hero);
  if (!girl) return "";
  const buttons = chests.map((chest, index) => {
    const claimed = Boolean((entry.chest_mask || 0) & (1 << index)), reached = entry.points >= chest.points;
    if (past && (claimed || !reached)) return "";
    return worldActionButton("world_chest", entry.week, chest.points, `${chest.points} 分 · 碎片 ${chest.shards}${chest.tickets ? ` + 限定券 ${chest.tickets}` : ""} · ${claimed ? "已领" : reached ? "领取" : "未达成"}`, busy || claimed || !reached);
  });
  if (entry.final_rank > 0) buttons.push(worldActionButton("world_rank", entry.week, null, `第 ${entry.final_rank} 名 · ${girl.name}碎片 ${entry.prize_shards} · ${entry.rank_claimed ? "已领" : "领取"}`, busy || entry.rank_claimed));
  return `<section><h4>${past ? `${esc(entry.week)} · ${esc(girl.name)} · 往期领奖` : "本期积分宝箱与结算奖励"}</h4><div class="ws-actions">${buttons.join("")}</div></section>`;
}

function renderWorldPool(body) {
  if (!body || !worldPoolVisible()) return;
  const state = currentWorldPool(), view = state.view, now = Store.serverNow();
  body.onclick = worldPoolClick;
  if (!state.pending && !state.busy && !state.error && (!view || now - state.readAt >= 30000)) refreshWorldPool();
  const reload = `<button type="button" class="ws-command" data-world-action="refresh"${state.pending || state.busy ? " disabled" : ""}>${state.pending ? "同步中…" : "刷新榜单"}</button>`;
  const status = state.error ? `<p role="status">${esc(state.error)} · 请重新同步。</p>` : "";
  if (!view) { body.innerHTML = `<h3>强者世界</h3>${status || '<p role="status">正在读取本周英雄与榜单…</p>'}${reload}`; return; }
  const hero = GIRLS.find(girl => girl.id === view.hero), own = SAVE.girls[hero.id], tickets = SAVE.mats.limited_ticket || 0;
  const busy = state.busy || Boolean(state.pending) || Boolean(state.error), open = view.open && now < view.closes_at;
  const rPool = GIRLS.filter(girl => girl.rarity === "R" && cardSources(girl).includes("world")).map(girl => esc(girl.name)).join("、");
  const next = new Intl.DateTimeFormat("zh-CN", {timeZone:"Asia/Shanghai", month:"numeric", day:"numeric"}).format(view.next_open);
  body.innerHTML = `<div class="ws-draw-head"><div><small class="ws-kicker">强者世界 · ${esc(view.week)} 周</small><h2>${esc(hero.name)} · 限定 SSR</h2><p>北京时间周一 00:00 至周五 24:00；周末休息。${open ? "本期召唤开放。" : `本期召唤已结束，下期 ${next} 周一 00:00 开放。`}</p></div><strong>限定券 ×${tickets}</strong></div>${status}
    <div class="ws-draw-layout"><section class="ws-draw-catalog"><img src="${esc(hero.img)}" alt="${esc(hero.name)}" width="100"><h4>本周限定</h4><p>每抽 ${view.shard_percent}% 获得 1 枚${esc(hero.name)}碎片，否则获得一张 R 整卡；不会抽到 SR 或 SSR 整卡，不计入常驻保底。</p><details><summary>R 整卡名单</summary><p>${rPool}。重复整卡保留在角色工坊，可自行分解。</p></details><p>同卡碎片 ${own?.shards || 0}/${RARITY.SSR.shardToUnlock} · ${own?.star ? "已拥有" : "可在图鉴主动合成"}</p><button type="button" class="ws-command" data-world-action="card">查看英雄与合成</button></section>
    <section class="ws-draw-controls"><h4>每抽消耗 1 张限定券，积 1 分</h4><p>我的本期积分：<b>${view.me.points}</b></p><div class="ws-actions">${[1,10].map(amount => worldActionButton("world_pull", view.week, amount, `${amount === 1 ? "单抽" : "十连"} · ${amount} 限定券`, busy || !open || tickets < amount)).join("")}</div><p>限定券来自每日任务积分箱、竞技场排名和本活动积分箱。</p><div aria-live="polite">${(state.drops || []).map(drop => `<p>${esc(drop)}</p>`).join("")}</div></section></div>
    ${worldPrizes(view.me, view.chests, busy)}<section><h4>全服前 20 · ${view.settled ? "已结算" : "实时积分榜"}</h4><p>同分先达到者优先，再同分按玩家编号。仅前 20 得本期碎片：${view.tiers.map((tier,index) => `${index && tier.through > view.tiers[index-1].through + 1 ? `${view.tiers[index-1].through + 1}–` : ""}${tier.through} 名 ${tier.shards} 枚`).join("，")}。</p>${reload}${worldBoardMarkup(view.board)}</section>
    ${(view.unclaimed || []).map(entry => worldPrizes(entry, entry.chests || view.chests, busy, true)).join("")}${view.previous ? `<details><summary>${esc(view.previous.week)} 周 · 上期榜单</summary>${worldBoardMarkup(view.previous_board)}</details>` : ""}`;
}

async function worldPoolClick(event) {
  const button = event.target.closest?.("button[data-world-action]");
  if (!button || button.disabled || !worldPoolVisible()) return;
  const action = button.dataset.worldAction, state = currentWorldPool(), view = state.view;
  if (action === "refresh") return refreshWorldPool();
  if (!view || state.busy || state.pending || state.error) return;
  if (action === "card") return openGirl(view.hero);
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

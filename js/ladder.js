/* Ranked arena is an independent server-owned ladder; no E stake or local save. */
let ladderData = null;
let ladderSampledAt = 0;
let ladderRankPage = 0;
let ladderRecordFilter = "all";
let ladderQuote = null;
let ladderActionPending = false;

const ladderNow = () => ladderData ? ladderData.server_now + performance.now() - ladderSampledAt : Store.serverNow();
const ladderTime = value => new Date(value).toLocaleString("zh-CN", {timeZone:"Asia/Shanghai", hour12:false});
const ladderMaterials = materials => `<div class="ladder-materials">${["mat_r", "mat_sr", "mat_ssr"].map(key => `<span><i aria-hidden="true">◆</i>${esc(MATS[key]?.name || key)} <b>×${Number(materials?.[key]) || 0}</b></span>`).join("")}</div>`;
const ladderToken = amount => `<span class="ladder-token"><i aria-hidden="true">✦</i>代币 <b>×${Number(amount) || 0}</b></span>`;
const ladderReward = reward => `<div class="ladder-reward-content">${ladderToken(reward?.token)}${ladderMaterials(reward?.materials)}</div>`;
const ladderGuild = entry => `<span class="ladder-guild ${entry.guild_name ? "has-guild" : ""}">${entry.id < 0 ? "竞技场守卫" : entry.guild_name ? esc(entry.guild_name) : "未加入公会"}</span>`;
const ladderBestRank = () => ladderData?.best_rank || ladderData?.me?.best_rank || 0;

async function renderLadder() {
  const epoch = Store.sessionEpoch, version = ++arenaQueueVersion;
  const tab = arenaTab;
  const current = () => Store.ready() && epoch === Store.sessionEpoch && version === arenaQueueVersion && document.body.dataset.page === "page-arena" && arenaTab === tab;
  document.getElementById("arena-body").innerHTML = '<p class="social-empty" role="status">正在读取竞技排名与结算奖励…</p>';
  try {
    const data = await Store.api(SITE.ep.ladder, "GET");
    if (!current()) return;
    // The ladder request may settle rewards and deliver mail. Read the resulting
    // profile afterwards so the shared inbox badge includes that same delivery.
    const profile = await Store.refresh();
    if (!current()) return;
    if (!profile) throw new Error("暂时无法同步游戏资料，请重试。");
    ladderData = data;
    ladderSampledAt = performance.now();
    paintLadderSummary(); renderTopbar(); paintLadder();
    tickLadderClock();
  } catch (e) {
    if (!current()) return;
    document.getElementById("arena-body").innerHTML = `<div class="social-empty"><p>${esc(e.message)}</p><button class="gbtn" onclick="renderArena()"><span>重新读取竞技场</span></button></div>`;
  }
}

function paintLadderSummary() {
  const me = ladderData?.me;
  const leader = GIRLS.find(g => g.id === (me?.squad?.[0]?.id || SAVE?.squad?.[0]));
  document.getElementById("arena-summary").innerHTML = `<div class="arena-captain">${leader ? `<img src="${leader.img}" alt="${leader.name}">` : '<span class="arena-emblem">✦</span>'}<div><span>星霜排名竞技场</span><strong>${esc(SITE_ACCOUNT?.name || "星霜旅人")}</strong>${me ? ladderGuild(me) : ""}<span>当前进攻战力</span><b>${(SAVE?.power || 0).toLocaleString()}</b></div></div><div class="arena-metrics"><div><span>当前排名</span><b>${me ? `#${me.rank}` : "未报名"}</b></div><div><span>历史最高</span><b>${ladderBestRank() ? `#${ladderBestRank()}` : "—"}</b></div><div><span>今日挑战</span><b>${ladderData?.remaining ?? "—"}<small> / ${ladderData?.limit || 5}</small></b></div><div><span>每日结算</span><b>21:00</b></div></div><div class="arena-sidebar-squad"><span>已保存的防守阵容</span><div class="arena-squad">${arenaSquadHTML(me?.squad) || '<p>报名时保存当前编队</p>'}</div></div>`;
}

function ladderHeading(title, note) {
  return `<div class="arena-section-heading"><div><p class="social-kicker">RANKED ARENA · 每日 21:00</p><h3>${title}</h3><p>${note}</p></div><div><button class="social-refresh" onclick="renderArena()">刷新</button>${ladderData.me && arenaTab === "ladder" ? '<button class="social-refresh" onclick="confirmLadderLeave()">退出排名竞技</button>' : ""}</div></div>`;
}
function paintLadder() {
  if (!ladderData || !Store.ready()) return;
  if (arenaTab === "ladder-board") return paintLadderBoard();
  if (arenaTab === "ladder-rewards") return paintLadderRewards();
  if (arenaTab === "ladder-records") return paintLadderRecords();
  const {me, remaining, opponents} = ladderData;
  const tier = ladderData.rewards.find(t => !t.max_rank || me && me.rank <= t.max_rank);
  document.getElementById("arena-body").innerHTML = ladderHeading("向更高的名次出发", "挑战守卫与其他玩家的防守阵容 · 获胜晋位 · 不消耗 E 余额") +
    `<div class="ladder-settlement"><div><span>距离下次排名奖励</span><b data-ladder-time>—</b><small>每日 21:00 · 自动寄至邮箱</small></div><div><span>${me ? `当前第 ${me.rank} 名 · 预计奖励` : "报名后参与每日排名奖励"}</span>${me ? ladderReward(tier) : '<p>代币 · 微光结晶 · 元素结晶 · 星屑</p>'}</div></div>` +
    `<div class="ladder-breakthrough-note"><span aria-hidden="true">✦</span><p><b>突破历史名次，赢取高额代币</b><span>首次达到前100、50、20、10、3名和第1名，即可领取各档代币，领取后直接到账。</span></p><button class="social-refresh" onclick="pickArenaTab('ladder-rewards')">查看奖励</button></div>` +
    (!me ? `<div class="ladder-welcome"><span class="arena-empty-emblem">♜</span><h3>加入星霜竞技场</h3><p>从榜尾开始，逐步挑战更高排名的对手。报名会保存当前编队作为防守阵容，其他玩家可以挑战你的防守。</p><p>每日 5 次免费挑战，05:00 重置；21:00 按名次将代币和材料寄至邮箱。</p><button class="gbtn gbtn-primary" ${SAVE.squad.length ? "" : "disabled"} onclick="ladderAction('ladder_join')"><span>${SAVE.squad.length ? "免费报名 · 开启挑战" : "请先配置出战编队"}</span></button></div>` :
    `<div class="arena-list-head"><p>今日剩余 <b>${remaining} / ${ladderData.limit}</b> 次 · <span data-ladder-cooldown>准备就绪</span></p><button class="social-refresh" onclick="renderArena()">换一批对手</button></div><div class="ladder-opponents">${opponents.map(entry => ladderOpponentCard(entry)).join("") || '<p class="social-empty">暂无可挑战对手，稍后刷新看看。</p>'}</div><div class="ladder-defense-tools"><div><b>我的防守阵容 · 战力 ${me.power.toLocaleString()}</b><div class="arena-squad">${arenaSquadHTML(me.squad)}</div><p>养成或调整编队后，点击保存才会更新防守。进攻使用当前编队。</p></div><button class="gbtn" onclick="confirmLadderDefense()"><span>保存当前编队为防守</span></button></div>`);
}

function ladderOpponentCard(entry) {
  const girl = GIRLS.find(g => g.id === entry.squad?.[0]?.id);
  return `<article class="arena-opponent-card ladder-opponent"><div class="arena-opponent-top"><span class="ladder-rank">#${entry.rank}</span><span class="arena-room-badge">${entry.id < 0 ? "星霜守卫" : "玩家防守"}</span></div><button class="ladder-portrait" onclick="previewLadderOpponent(${entry.id})" aria-label="查看${esc(entry.name)}的阵容">${girl ? `<img src="${girl.img}" alt="${girl.name}" loading="lazy">` : "✦"}</button><h4>${esc(entry.name)}</h4>${ladderGuild(entry)}<p>防守战力 <b>${entry.power.toLocaleString()}</b></p><div class="arena-squad">${arenaSquadHTML(entry.squad)}</div><button class="gbtn gbtn-primary ladder-challenge" data-ladder-challenge ${ladderActionPending || ladderData.remaining <= 0 || ladderNow() < ladderData.me.cooldown_until || !SAVE.squad.length ? "disabled" : ""} onclick="previewLadderOpponent(${entry.id},true)"><span>挑战对手 <i aria-hidden="true">›</i></span></button><small>${entry.rank < ladderData.me.rank ? `获胜可升至第 ${entry.rank} 名` : "获胜保持当前名次"}</small></article>`;
}
function previewLadderOpponent(id, challenge = false) {
  if (!requireGame() || !ladderData) return;
  const entry = [...ladderData.opponents, ...ladderData.board].find(x => x.id === id);
  if (!entry) return;
  ladderQuote = {id, rank:entry.rank, epoch:Store.sessionEpoch};
  showModal(`<h3>${challenge ? "确认排名挑战" : "竞技场防守阵容"}</h3><div class="social-confirm"><h4>#${entry.rank} · ${esc(entry.name)}</h4>${ladderGuild(entry)}<div class="arena-comparison"><section><p>对方防守 · ${entry.power.toLocaleString()}</p><div class="arena-squad arena-preview">${arenaSquadHTML(entry.squad)}</div></section><section><p>当前进攻 · ${(SAVE.power || 0).toLocaleString()}</p><div class="arena-squad arena-preview">${arenaSquadHTML(arenaOwnSquad())}</div></section></div><p>获胜且对手排名更高时交换名次。挑战消耗 1 次机会，不消耗 E 余额；60 回合未击败对手视为进攻失败。</p></div><div class="modal-btns"><button class="gbtn" onclick="closeModal()"><span>返回</span></button>${challenge ? '<button class="gbtn gbtn-primary" data-ladder-submit onclick="submitLadderChallenge()"><span>进入战斗 · 1 次</span></button>' : ""}</div>`);
}
async function submitLadderChallenge() {
  if (!ladderQuote || ladderQuote.epoch !== Store.sessionEpoch) return;
  await ladderAction("ladder_challenge", {opponent_id:ladderQuote.id, count:ladderQuote.rank});
}
function confirmLadderDefense() {
  showModal(`<h3>更新防守阵容</h3><div class="social-confirm"><p>将当前编队的等级、星级和装备保存为防守快照，其他玩家挑战时使用这套阵容。</p><div class="arena-squad arena-preview">${arenaSquadHTML(arenaOwnSquad())}</div></div><div class="modal-btns"><button class="gbtn" onclick="closeModal()"><span>返回</span></button><button class="gbtn gbtn-primary" onclick="ladderAction('ladder_defense')"><span>确认保存</span></button></div>`);
}
async function ladderAction(action, fields = {}) {
  if (!requireGame() || ladderActionPending) return;
  const epoch = Store.sessionEpoch;
  ladderActionPending = true;
  document.querySelectorAll("[data-ladder-submit], [data-ladder-challenge], [data-ladder-best]").forEach(button => button.disabled = true);
  try {
    const result = await Store.mutate(action,fields);
    if (!result || epoch !== Store.sessionEpoch || !Store.ready()) return;
    closeModal(true);
    renderTopbar();
    if (result.ladder) {
      // The response is already authoritative, but the player sees the battle
      // before its result dialog. Historical replay keeps its existing entry.
      lastArenaResult = result.ladder;
      lastArenaEpoch = epoch;
      watchArenaResult();
      return;
    }
    if (action === "ladder_best_claim") {
      const tier = ladderData?.best_rewards?.find(item => `best:${item.max_rank}` === fields.item_id);
      if (tier) { tier.claimed = true; tier.can_claim = false; }
    }
    if (document.body.dataset.page === "page-arena" && arenaTab.startsWith("ladder")) await renderArena();
    if (epoch !== Store.sessionEpoch || !Store.ready()) return;
    if (action === "ladder_best_claim") toast((result.drops || []).join(" · ") || "历史名次奖励已直接到账");
    else if (action === "ladder_claim") toast("每日排名奖励已寄至邮箱");
    else if (action === "ladder_defense") toast("防守阵容已保存");
    else if (action === "ladder_leave") toast("已退出排名竞技");
  } finally {
    ladderActionPending = false;
    if (epoch === Store.sessionEpoch && Store.ready()) {
      document.querySelectorAll("[data-ladder-submit]").forEach(button => button.disabled = false);
      document.querySelectorAll("[data-ladder-best]").forEach(button => {
        const tier = ladderData?.best_rewards?.find(item => item.max_rank === Number(button.dataset.ladderBest));
        button.disabled = tier?.can_claim !== true || tier.claimed || !!tier.delivered_at;
        if (tier?.claimed) button.textContent = "已领取";
      });
      tickLadderClock();
    }
  }
}

function confirmLadderLeave() {
  showModal('<h3>退出排名竞技？</h3><p>退榜后不再接受排名挑战，也不再获得新的每日排名奖励。已寄出的奖励可在邮箱领取，历史名次奖励和当天次数不会因重新报名而重置。</p><div class="modal-btns"><button class="gbtn" onclick="closeModal()"><span>返回</span></button><button class="gbtn gbtn-primary" onclick="ladderAction(\'ladder_leave\')"><span>确认退榜</span></button></div>');
}

function paintLadderBoard() {
  const board = ladderData.board, pages = Math.max(1,Math.ceil(board.length/10));
  ladderRankPage = Math.max(0,Math.min(pages-1,ladderRankPage));
  document.getElementById("arena-body").innerHTML = ladderHeading("星霜竞技排名", `共 ${ladderData.total} 位参赛者 · 显示前 50 名 · 按挑战胜负晋位`) +
    `<div class="arena-podium">${board.slice(0,3).map((entry,i) => { const girl=GIRLS.find(g=>g.id===entry.squad?.[0]?.id); return `<button class="arena-podium-card podium-${i+1}" onclick="previewLadderOpponent(${entry.id})"><span class="arena-podium-place">0${i+1}</span><span class="arena-podium-title">${["星霜之巅","耀星之席","逐光之席"][i]}</span>${girl ? `<img src="${girl.img}" alt="${girl.name}">` : ""}<strong>${esc(entry.name)}</strong>${ladderGuild(entry)}<span>防守战力 <b>${entry.power.toLocaleString()}</b></span></button>`; }).join("")}</div>
    <p class="social-note">我的排名 ${ladderData.me ? `#${ladderData.me.rank}` : "尚未报名"}${ladderBestRank() ? ` · 历史最高 #${ladderBestRank()}` : ""}</p><div class="arena-list">${board.slice(ladderRankPage*10,(ladderRankPage+1)*10).map(entry=>`<article class="arena-row ${entry.id===Store.authUserId ? "is-self" : ""}"><span class="arena-rank">${entry.rank}</span><div class="arena-player"><strong>${esc(entry.name)}${entry.id===Store.authUserId ? " · 我" : ""}</strong>${ladderGuild(entry)}<p>防守战力 ${entry.power.toLocaleString()}</p></div><button class="gbtn gbtn-sm" onclick="previewLadderOpponent(${entry.id})"><span>查看阵容</span></button></article>`).join("")}</div><div class="arena-pagination"><button class="social-refresh" ${ladderRankPage===0?"disabled":""} onclick="changeLadderPage(-1)">上一页</button><span>${ladderRankPage+1} / ${pages}</span><button class="social-refresh" ${ladderRankPage+1>=pages?"disabled":""} onclick="changeLadderPage(1)">下一页</button></div>`;
}
function changeLadderPage(delta) { ladderRankPage += delta; paintLadderBoard(); }
function paintLadderRewards() {
  const {me, rewards, mail} = ladderData;
  const best = ladderBestRank();
  document.getElementById("arena-body").innerHTML = ladderHeading("突破名次 · 赢取代币", "历史名次奖励领取后直接到账 · 每日 21:00 奖励自动寄至邮箱") +
    `<h3 class="ladder-subtitle">历史最高名次 · 每档一次</h3><p class="social-note">历史最高 ${best ? `#${best}` : "—"}。达到的每一档均可手动领取，代币直接到账；退榜或重新报名不会重置奖励。</p><div class="ladder-best-tiers">${(ladderData.best_rewards || []).map(tier => {
      const reached = best > 0 && best <= tier.max_rank;
      const claimable = tier.can_claim === true && !tier.claimed && !tier.delivered_at;
      const state = tier.claimed ? "已领取" : tier.delivered_at ? "已转至原邮箱" : claimable ? "领取代币" : "未达成";
      const action = !tier.claimed && tier.delivered_at ? '<button class="social-refresh" onclick="openHarborMail()">查看原奖励邮件</button>' : `<button class="social-refresh" data-ladder-best="${tier.max_rank}" ${!claimable || ladderActionPending ? "disabled" : ""} onclick="claimLadderBest(${tier.max_rank},${Store.sessionEpoch})">${state}</button>`;
      return `<article class="ladder-best-tier ${reached ? "is-reached" : ""}"><span>${tier.max_rank === 1 ? "登顶第 1 名" : `进入前 ${tier.max_rank} 名`}</span>${ladderToken(tier.token)}<small>${tier.delivered_at && !tier.claimed ? "旧版已寄出，请从原邮件领取" : tier.claimed ? "本档奖励已领取" : claimable ? "已达成 · 领取后直接到账" : "首次达到此名次后解锁"}</small>${action}</article>`;
    }).join("")}</div>` +
    `<div class="ladder-mail-entry"><div><b>每日结算奖励保留在邮箱</b><p>每日 21:00 按当时名次寄出代币与材料，离线也不会错过。</p></div><button class="gbtn gbtn-primary" onclick="openHarborMail()"><span>前往邮箱</span></button></div>` +
    `<div class="ladder-settlement"><div><span>距离下次每日结算</span><b data-ladder-time>—</b><small>${esc(ladderTime(ladderData.next_settlement))}（北京时间）</small></div><p>每日 21:00 按当时名次发奖，代币与材料一同寄达。</p></div><h3 class="ladder-subtitle">每日排名奖励</h3><div class="ladder-reward-tiers">${rewards.map((tier,i)=>{
    const min=i ? rewards[i-1].max_rank+1 : 1;
    const active=me && me.rank>=min && (!tier.max_rank || me.rank<=tier.max_rank);
    return `<article class="ladder-reward-tier ${active?"is-current":""}"><div><b>${tier.max_rank===min ? `第 ${min} 名` : !tier.max_rank ? `${min} 名及以后` : `${min}～${tier.max_rank} 名`}</b>${active?'<span>当前档位</span>':""}</div>${ladderReward(tier)}</article>`;
  }).join("")}</div><h3 class="ladder-subtitle">每日寄送记录</h3><div class="arena-list">${mail.map(m=>`<article class="ladder-mail"><div><b>${esc(m.day)} · 第 ${m.rank} 名</b><p>21:00 排名结算</p>${ladderReward(m)}</div><span class="ladder-delivery-state">${m.claimed ? "已领取" : m.delivered_at ? "已寄至邮箱" : "待寄送 · 邮箱腾出空间后寄达"}</span></article>`).join("") || '<div class="social-empty">暂无每日结算记录。报名后，在下一次 21:00 获得每日奖励。</div>'}</div><p class="social-note">展示最近 60 次每日结算。已经寄出的奖励请到公共邮箱查看。</p>`;
}
async function claimLadderBest(maxRank, epoch) {
  if (!requireGame() || epoch !== Store.sessionEpoch || ![100, 50, 20, 10, 3, 1].includes(maxRank)) return;
  const tier = ladderData?.best_rewards?.find(item => item.max_rank === maxRank);
  if (tier?.can_claim !== true || tier.claimed || tier.delivered_at) return;
  await ladderAction("ladder_best_claim", {item_id:`best:${maxRank}`});
}
function claimLadderMail() {
  // Compatibility for an old cached button: collection now lives in one inbox.
  openHarborMail();
}
function paintLadderRecords() {
  const history = ladderData.history.filter(m => ladderRecordFilter === "all" || (ladderRecordFilter === "attack" ? m.attacker_id===Store.authUserId : m.defender_id===Store.authUserId));
  document.getElementById("arena-body").innerHTML = ladderHeading("每次交锋 · 每次晋位", "最近 20 场攻防记录 · 回放不消耗次数、不重复发奖") + `<div class="arena-filters">${[["all","全部"],["attack","我的进攻"],["defense","我的防守"]].map(([key,label])=>`<button class="social-refresh" aria-pressed="${ladderRecordFilter===key}" onclick="filterLadderRecords('${key}')">${label}</button>`).join("")}</div><div class="arena-list">${history.map(m=>{
    const attack=m.attacker_id===Store.authUserId, win=attack ? m.attack_win : !m.attack_win;
    const before=attack ? m.attacker_before : m.defender_before, after=attack ? m.attacker_after : m.defender_after;
    const valid=/^[0-9a-f-]{36}$/i.test(m.id);
    return `<article class="ladder-record"><div class="arena-list-head"><b class="${win?"arena-win":""}">${attack?"进攻":"防守"}${win?"胜利":"失败"}</b><span>${esc(ladderTime(m.played_at))}</span></div><div class="ladder-record-versus"><span><small>进攻方</small><b>${esc(m.attacker_name)}</b></span><span aria-hidden="true">→</span><span><small>防守方</small><b>${esc(m.defender_name)}</b></span></div><div class="arena-list-head"><p>我的排名 #${before} → #${after}</p><button class="gbtn gbtn-sm" ${valid?"":"disabled"} onclick="replayLadder('${valid?m.id:""}')"><span>战斗回放</span></button></div></article>`;
  }).join("") || '<div class="social-empty">暂无此类战报，发起一次排名挑战吧。</div>'}</div>`;
}
function filterLadderRecords(filter) { if (!["all","attack","defense"].includes(filter)) return; ladderRecordFilter=filter; paintLadderRecords(); }
async function replayLadder(id) {
  if (!requireGame()) return;
  const epoch=Store.sessionEpoch;
  try {
    const result=await Store.api(SITE.ep.ladderReplay+"?request_id="+encodeURIComponent(id),"GET");
    if (epoch===Store.sessionEpoch && Store.ready()) showArenaResult(result);
  } catch(e) { if (epoch===Store.sessionEpoch && Store.ready()) toast(e.message); }
}
function showLadderResult(result) {
  showModal(`<h3>排名竞技 · ${result.win?"胜利":"失败"}</h3><div class="social-confirm"><h4>${esc(result.name)}</h4><div class="arena-award">#${result.own_before} → #${result.own_after}<span>我的竞技排名</span></div><p>${result.report.timed_out?"达到60回合上限，防守方获胜。":"本场排名已结算。"} 历史名次达成奖励已自动寄达，每日奖励在21:00按届时名次寄至邮箱。</p><p>回放左侧为进攻方「${esc(result.attacker_name)}」，右侧为防守方「${esc(result.defender_name)}」。</p></div><div class="modal-btns"><button class="gbtn" onclick="leaveArenaReplay()"><span>返回排名竞技</span></button><button class="gbtn gbtn-primary" onclick="watchArenaResult()"><span>重看战斗</span></button></div>`);
}
function tickLadderClock() {
  clearTimeout(arenaPollTimer);
  if (!ladderData || !Store.ready() || !arenaTab.startsWith("ladder") || document.body.dataset.page!=="page-arena" || document.hidden) return;
  const now=ladderNow();
  if (now>=ladderData.next_settlement || now>=ladderData.next_reset) { renderArena(); return; }
  const seconds=Math.max(0,Math.ceil((ladderData.next_settlement-now)/1000));
  document.querySelectorAll("[data-ladder-time]").forEach(el=>el.textContent=[Math.floor(seconds/3600),Math.floor(seconds/60)%60,seconds%60].map(v=>String(v).padStart(2,"0")).join(":"));
  const cooldown=Math.max(0,Math.ceil(((ladderData.me?.cooldown_until || 0)-now)/1000));
  document.querySelectorAll("[data-ladder-cooldown]").forEach(el=>el.textContent=cooldown?`冷却 ${cooldown} 秒`:"准备就绪");
  document.querySelectorAll("[data-ladder-challenge]").forEach(el=>el.disabled=ladderActionPending || ladderData.remaining<=0 || cooldown>0 || !SAVE.squad.length);
  arenaPollTimer=setTimeout(tickLadderClock,1000);
}
function showLadderRules() {
  showModal(`<h3>排名竞技场规则</h3><div class="arena-rules"><section><h4>01 · 排名挑战</h4><p>免费报名后进入榜尾，挑战推荐对手保存的防守阵容。进攻获胜且对手名次更高时交换排名；失败或60回合未击败对手，排名不变。没有战力差限制，不消耗E余额。</p></section><section><h4>02 · 次数与防守</h4><p>每天5次主动挑战，北京时间05:00重置；每次挑战后冷却10秒。被挑战不消耗次数。进攻使用当前编队，防守使用报名或最近一次保存的阵容。</p></section><section><h4>03 · 每日奖励与历史突破</h4><p>每天21:00按当时名次将代币、微光结晶、元素结晶和星屑自动寄至邮箱。首次达到前100、50、20、10、3名和第1名，还能获得各档一次性代币奖励；跨档晋位补齐所有达到档位。退榜重入、改名或删除已领邮件都不会重置奖励。</p></section><section><h4>04 · 战斗与回放</h4><p>确认挑战后直接进入战斗，结束后展示胜负与名次变化。最近20场攻防记录可回放，不重复消耗次数或发奖。余额匹配和自定义房间是独立玩法，不影响排名竞技。</p></section></div><div class="modal-btns"><button class="gbtn" onclick="closeModal()"><span>知道了</span></button></div>`);
}

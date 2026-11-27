const fleetTabs = { crew: "公会总览", growth: "公会成长", raid: "协力副本", shop: "公会商店", members: "成员", directory: "招募名录" };
const fleetScreen = { tab: "directory", epoch: -1, request: 0, pages: [0], next: 0 };

function fleetNumber(value) {
  return (Number.isSafeInteger(value) && value >= 0 ? value : 0).toLocaleString("zh-CN");
}

function fleetButton(label, command, value = "", disabled = false) {
  return `<button type="button" class="fleet-button" data-fleet-act="${command}" data-value="${esc(value)}" ${disabled ? "disabled" : ""}>${esc(label)}</button>`;
}

function fleetEnrollment() {
  return `<div class="fleet-enroll"><details><summary>持邀请码入会</summary><form data-fleet-form="join"><label>邀请码<input name="code" placeholder="8 位邀请码" minlength="8" maxlength="8" pattern="[a-fA-F0-9]{8}" autocapitalize="characters" required></label><button class="fleet-button" type="submit">加入公会</button></form></details>
    <details><summary>创建公会</summary><form data-fleet-form="create"><label>公会名称<input name="name" minlength="2" maxlength="12" required></label><label class="fleet-check"><input name="listed" type="checkbox" checked> 公开招募并参加公会榜</label><button class="fleet-button" type="submit">成立公会</button></form></details></div>`;
}

function fleetNoGuild() {
  return `<section class="guild-welcome"><span class="guild-kicker">一起建港 · 一起成长</span><h3>让每一次出航，都留下共同的积累</h3><p>加入公会可获得每日公会币、协力首领首通奖励，以及每周限量培养物资。你的原有副本与养成照常开放。</p><div class="guild-benefits"><article><b>五阶远征</b><small>五个阶段逐步解锁<br>需公会等级解锁并参与讨伐</small></article><article><b>每日 2 次</b><small>协力挑战积累个人公会币<br>伤害共同推进首领进度</small></article><article><b>培养补给</b><small>公会币兑换书卷、晋阶章<br>星屑、核心与训练材料</small></article></div>${fleetButton("查看招募名录", "tab", "directory")}</section>${fleetEnrollment()}`;
}
function fleetGuildHeader(guild) {
  const level = guild.level || 1, members = guild.members || [];
  return `<header class="guild-hero"><div><span class="guild-kicker">公会母港 · Lv.${fleetNumber(level)}</span><h3>${esc(guild.name)}</h3><p>${fleetNumber(members.length)} / ${fleetNumber(guild.member_limit || 10)} 位船员 · ${esc(fleetRoleName(guild.role || (Store.profile.user_id === guild.leader_id ? "leader" : "member")))}</p></div><div class="guild-wallet"><small>我的公会币</small><strong>${fleetNumber(guild.coins)}</strong><span>退会保留 · 周限购跨公会共用</span></div><div class="guild-level"><label>母港经验 <b>${fleetNumber(guild.xp)}${guild.next_xp ? ` / ${fleetNumber(guild.next_xp)}` : " · 满级"}</b></label><progress max="${guild.next_xp || 260000}" value="${guild.xp || 0}" aria-label="公会经验"></progress><small>${guild.next_xp ? "升级增加 5 个成员位置，逐步解锁副本与商店。" : "母港已达最高等级，船员上限 55 人。"}</small></div></header>`;
}
function fleetRoleName(role) { return ({ leader: "会长", deputy: "副会长", elder: "长老", member: "成员" })[role] || "成员"; }
function fleetResourceName(id) { return Store.profile?.items?.items?.[id]?.name || ({token:"代币",gold:"金币",mat_sr:"元素结晶",mat_ssr:"星屑",forge_ore:"星锻矿",training_manual:"训练手册",expedition_atlas:"远征图鉴",training_ink:"研训墨",break_seal:"晋阶章",weapon_core:"觉醒核心",gem_ticket:"宝石券",chip_ticket:"芯片券"})[id] || id; }
function fleetPrizes(resources, coins = 0) {
  return `<div class="guild-materials">${coins ? `<span class="guild-coin-prize">公会币 <b>×${fleetNumber(coins)}</b></span>` : ""}${Object.entries(resources || {}).filter(([,amount])=>amount>0).map(([id,amount])=>`<span>${esc(fleetResourceName(id))} <b>×${fleetNumber(amount)}</b></span>`).join("")}</div>`;
}
function fleetGrowth(guild) {
  if (!guild) return fleetNoGuild();
  const activity = guild.activity;
  if (!activity) return `${fleetGuildHeader(guild)}<p class="fleet-empty">成长任务正在同步，请刷新公会资料。</p>`;
  return `${fleetGuildHeader(guild)}<section class="fleet-section"><div class="fleet-section-title"><h3>每日协力</h3><small>${esc(activity.day)} · 北京时间零点更新</small></div><p class="fleet-note">巡航、培养和竞技都会留下贡献。达到目标后领取，公会经验与个人公会币同时到账；换会不重置个人次数。</p><div class="guild-daily-grid">${activity.tasks.map(task=>`<article class="guild-task"><div><small>${task.claimed ? "今日已领取" : `${task.progress} / ${task.target}`}</small><h4>${esc(task.name)}</h4><p>${esc(task.goal)}</p></div><progress value="${task.progress}" max="${task.target}" aria-label="${esc(task.name)}进度"></progress><b>公会经验 +${task.xp} · 公会币 +${task.coins}</b>${fleetButton(task.claimed ? "已领取" : task.can_claim ? task.id === "checkin" ? "签到领取" : "领取贡献" : "未达成", task.id === "checkin" ? "checkin" : "activityClaim",task.id,!task.can_claim)}</article>`).join("")}</div></section>
  <section class="fleet-section"><div class="fleet-section-title"><h3>全会周目标</h3><small>${esc(activity.week)} · 我的贡献 ${fleetNumber(activity.contribution)}</small></div><p class="fleet-note">全体成员一起推进，达标时公会自动获得经验；本周参与过的成员可直接领取对应培养材料，每账号每档每周一次。</p><div class="guild-mission-grid">${activity.missions.map(task=>`<article class="guild-task"><div><small>${task.progress} / ${task.target} · 公会经验 +${task.xp}</small><h4>${esc(task.name)}</h4><p>${esc(task.goal)}</p></div><progress value="${task.progress}" max="${task.target}" aria-label="${esc(task.name)}进度"></progress>${fleetPrizes(task.resources,task.coins)}${fleetButton(task.claimed ? "本周已领取" : task.can_claim ? "领取协力奖励" : activity.contribution ? "等待全会达标" : "参与公会后可领取","missionClaim",task.id,!task.can_claim)}</article>`).join("")}</div></section>
  <section class="fleet-section"><h3>经验从哪里来</h3><div class="guild-xp-sources">${activity.sources.map(source=>`<article><b>${esc(source.name)}</b><span>${esc(source.detail)}</span><small>经验 +${source.xp}${source.coins ? ` · 公会币 +${source.coins}` : ""}</small></article>`).join("")}</div><p class="fleet-note">首领阶段首通额外提供 200 / 400 / 700 / 1100 / 1600 公会经验。高阶等级需要持续协作，原有公会升级时保留等级与该级进度。</p></section>`;
}
function fleetCrew(guild) {
  if (!guild) return fleetNoGuild();
  const harbor = guild.harbor;
  return `${fleetGuildHeader(guild)}<div class="guild-overview-links">${fleetButton("每日任务与公会成长", "tab", "growth")}${fleetButton("进入公会副本", "tab", "raid")}${fleetButton("打开公会商店", "tab", "shop")}${fleetButton("查看成员与职位", "tab", "members")}</div>
    ${harbor ? `<section class="fleet-section"><div class="fleet-section-title"><h3>母港共建</h3><small>本周交付 ${fleetNumber(harbor.my_cargo)} 次 · ${harbor.supplied ? "今日已交付" : "今日任选一项"}</small></div><p class="fleet-note">每日交付获得公会经验 40、公会币 35，并推进全会物资集结目标。公会成长永久保留，个人币不随退会清零。</p><div class="fleet-lanes">${harbor.lines.map((line, index) => `<article><small>工位 0${index + 1}</small><h4>${esc(line.name)}</h4><progress aria-label="${esc(line.name)}进度" value="${line.done}" max="${harbor.line_limit}"></progress><small>${fleetNumber(line.done)} / ${fleetNumber(harbor.line_limit)} 份</small>${fleetButton(`${line.cost} · 交付`, "supply", line.id, !line.eligible)}</article>`).join("")}</div>
    <h4 class="fleet-minor-title">共建里程</h4><p class="fleet-note">三个工位同时达标后，本周在本会交付过的成员可领取。每档每账号每周一次，奖励直接入库。</p><div class="fleet-list">${harbor.beacons.map(beacon => `<div class="fleet-row"><span class="fleet-ordinal">${beacon.tier}</span><div><b>${esc(beacon.name)}</b><small>各 ${beacon.goal} 份 · 金币 ${beacon.gold}、星锻矿 ${beacon.ore}</small></div>${fleetButton(beacon.claimed ? "已领取" : beacon.eligible ? "领取物资" : "未达成", "claim", String(beacon.tier), !beacon.eligible)}</div>`).join("")}</div></section>` : ""}
    <section class="fleet-section"><h3>公会成长路线</h3><div class="guild-roadmap"><span><b>Lv.1</b>10 位成员 · 暗礁守卫</span><span><b>Lv.3</b>20 位成员 · 深潮统领</span><span><b>Lv.5</b>30 位成员 · 深渊主宰</span><span><b>Lv.7</b>40 位成员 · 高阶远征</span><span><b>Lv.9</b>50 位成员 · 终阶远征</span><span><b>Lv.10</b>55 位成员 · 母港满级</span></div><p class="fleet-note">公会副本与商店提供额外成长资源。等级、职位和成员容量由母港记录共同维护。</p>${guild.can_manage || Store.profile.user_id === guild.leader_id ? fleetButton(guild.listed ? "停止公开招募" : "开启公开招募", "listing", guild.listed ? "off" : "on") : ""}</section>`;
}
function fleetRaidRanking(guild) {
  const ranking = guild.raid_damage_ranking;
  if (!ranking) return "";
  const entries = ranking.entries || [], total = entries.reduce((sum, entry) => sum + entry.damage, 0);
  return `<details class="guild-damage-board" open><summary><span>当前首领 · 成员伤害榜</span><small>${esc(ranking.stage_name)}${ranking.all_cleared ? " · 本周已全部通关" : ""}</small></summary><p class="fleet-note">${esc(ranking.week)} · 统计本阶段实际扣除的生命，包含本周已离会成员。每周一 00:00（北京时间）随副本重置。</p>${entries.length ? `<ol class="guild-damage-list" aria-label="当前首领成员累计伤害">${entries.map(entry => `<li class="${entry.user_id === Store.profile.user_id ? "is-mine" : ""}"><span class="guild-damage-rank">${fleetNumber(entry.rank)}</span><div><b>${esc(entry.name)}${entry.user_id === Store.profile.user_id ? " · 我" : ""}</b><small>${fleetNumber(entry.attempts)} 次挑战${entry.member === false ? " · 已离会" : ""}</small></div><div class="guild-damage-score"><strong>${fleetNumber(entry.damage)}</strong><small>${total ? (100 * entry.damage / total).toFixed(1) : "0.0"}% 贡献</small></div></li>`).join("")}</ol>` : '<p class="fleet-empty">本阶段还没有讨伐记录，出战后在这里查看贡献。</p>'}</details>`;
}
function fleetRaid(guild) {
  if (!guild) return fleetNoGuild();
  const stages = guild.raid_stages || [], active = stages.find(stage => stage.active), attempts = guild.attempts_today || 0, limit = guild.attempt_limit || 2;
  const canAttack = active?.unlocked && !active.cleared && attempts < limit && SAVE.stamina >= 10 && SAVE.squad.length > 0;
  const label = !active ? "本周已全部通关" : !active.unlocked ? `公会 Lv.${active.required_level} 解锁` : attempts >= limit ? "今日次数已用完" : !SAVE.squad.length ? "请先编队" : SAVE.stamina < 10 ? "体力不足" : "进入战斗 · 10 体力";
  return `${fleetGuildHeader(guild)}<section class="fleet-section"><div class="fleet-section-title"><h3>协力远征</h3><small>${esc(guild.boss_week)} · 今日 ${attempts} / ${limit} 次</small></div><p class="fleet-note">每周一 00:00（北京时间）重置副本、阶段奖励与伤害榜。使用当前编队自动战斗，每次最多 20 回合，未击败的伤害保留至本周结束。每次获得公会币 25、公会经验 30、研训墨 3。</p><div class="guild-raid-action"><div><strong>${active ? esc(active.name) : "本周远征完成"}</strong><small>我的本周累计伤害 ${fleetNumber(guild.my_damage)} · 每个阶段每账号每周领取一次。</small></div>${fleetButton(label,"attack","",!canAttack)}</div>${fleetRaidRanking(guild)}<div class="guild-raid-grid">${stages.map(stage => `<article class="guild-raid-card ${stage.active ? "is-active" : ""} ${!stage.unlocked ? "is-locked" : ""}"><div class="guild-raid-number">0${stage.stage}</div><div><small>公会 Lv.${stage.required_level} · ${stage.cleared ? "已通关" : stage.active && stage.unlocked ? "正在讨伐" : !stage.unlocked ? "等级未解锁" : "等待前一阶段"}</small><h4>${esc(stage.name)}</h4><progress value="${stage.hp}" max="${stage.max_hp}" aria-label="${esc(stage.name)}剩余生命"></progress><p>${fleetNumber(stage.hp)} / ${fleetNumber(stage.max_hp)}</p><p class="fleet-note">${esc(stage.description || "全会协作，共同推进首领生命")}</p><h5>本周阶段首通奖励</h5>${fleetPrizes(stage.resources || (stage.material ? {[stage.material]:stage.material_count} : {}),stage.coins)}${fleetButton(stage.claimed ? "本周已领取" : stage.can_claim ? "领取阶段首通" : !stage.participated ? "参战后可领取首通" : "等待通关", "bossClaim", String(stage.stage), !stage.can_claim)}</div></article>`).join("")}</div></section>`;
}
function fleetShop(guild) {
  if (!guild) return fleetNoGuild();
  return `${fleetGuildHeader(guild)}<section class="fleet-section"><div class="fleet-section-title"><h3>公会补给商店</h3><small>每周更新限购次数 · ${esc(guild.boss_week)}</small></div><p class="fleet-note">签到、每日协力任务、共建、讨伐和周目标均可获得公会币。兑换后物资直接入库，跨公会共用每周购买次数。</p><div class="guild-shop-grid">${(guild.shop || []).map(item => `<article class="guild-shop-card"><span class="guild-shop-icon">${item.resource === "token" ? "✦" : "◇"}</span><small>公会 Lv.${item.level}</small><h4>${esc(item.name)}</h4><p>${esc(fleetResourceName(item.resource))} × ${item.amount}</p><strong>${item.cost} <small>公会币</small></strong><span>本周剩余 ${item.remaining} / ${item.limit}</span>${fleetButton(!item.unlocked ? `Lv.${item.level} 解锁` : item.remaining === 0 ? "本周已售罄" : guild.coins < item.cost ? "公会币不足" : "兑换", "shop", item.id, !item.can_buy)}</article>`).join("")}</div></section>`;
}
function fleetMembers(guild) {
  if (!guild) return fleetNoGuild();
  const members = guild.members || [], leader = Store.profile.user_id === guild.leader_id;
  return `${fleetGuildHeader(guild)}<section class="fleet-section"><div class="fleet-section-title"><h3>成员与职位</h3><small>副会长最多 2 位 · 长老最多 4 位</small></div><p class="fleet-note">会长任免职位；会长与副会长管理公开招募。副会长可移出普通成员和长老，会长可移出其他成员；不能移出自己或会长。职位不影响个人奖励额度。</p><div class="guild-member-list">${members.map(member => { const role = member.role || (member.user_id === guild.leader_id ? "leader" : "member"); return `<article><div><b>${esc(member.name)}${member.user_id === Store.profile.user_id ? " · 我" : ""}</b><span class="guild-role guild-role-${esc(role)}">${esc(fleetRoleName(role))}</span></div>${guild.can_assign && member.user_id !== guild.leader_id ? `<div class="guild-role-controls">${["deputy","elder","member"].map(next => fleetButton(next === "member" ? "设为成员" : `任命${fleetRoleName(next)}`, "role", `${member.user_id}:${next}`, role === next)).join("")}</div>` : ""}${member.can_kick ? fleetButton("移出公会", "kick", String(member.user_id)) : ""}</article>`; }).join("")}</div><div class="fleet-actions">${fleetButton(`邀请码 ${guild.code} · 复制`,"copy",guild.code)}${guild.can_manage ? fleetButton(guild.listed ? "停止公开招募" : "开启公开招募","listing",guild.listed ? "off" : "on") : ""}</div><details class="fleet-details"><summary>退出公会</summary><p class="fleet-note">${leader && members.length > 1 ? "会长仍有其他成员时不能退出。" : "退出后本公会贡献清零；公会币、日挑战次数、本周领取和购买记录均保留。"}</p>${fleetButton("确认退出", "leave", "", leader && members.length > 1)}</details></section>`;
}

function fleetBoardMarkup(view) {
  const member = Boolean(Store.profile?.guild);
  const list = view.entries.map(entry => {
    const limit = entry.member_limit || view.member_limit || 10;
    const canJoin = !member && entry.members < limit && /^[0-9A-F]{8}$/.test(entry.code || "");
    return `<li class="fleet-row${entry.mine ? " is-mine" : ""}"><span class="fleet-ordinal">帆</span><div><b>${esc(entry.name)}${entry.mine ? "<em>本会</em>" : ""}</b><small>${fleetNumber(entry.members)} / ${fleetNumber(limit)} 位成员</small></div><div class="fleet-value"><b>${fleetNumber(entry.value)}</b><small>航绩</small></div>${canJoin ? fleetButton("加入", "recruit", entry.code) : ""}</li>`;
  }).join("");
  return `<section class="fleet-section"><div class="fleet-section-title"><h3>招募名录</h3><small>第 ${fleetScreen.pages.length} 页</small></div><p class="fleet-note">公开招募 · 仅列未满员公会</p>
    ${list ? `<ol class="fleet-list">${list}</ol>` : '<p class="fleet-empty">本页暂无招募中的公会，可凭邀请码加入或自行建会。</p>'}
    <div class="fleet-pager">${fleetButton("‹ 上一页", "previous", "", fleetScreen.pages.length < 2)}${fleetButton("下一页 ›", "next", "", !view.next)}</div></section>${!member ? fleetNoGuild() : ""}`;
}

function fleetCurrent(request, epoch) {
  return request === fleetScreen.request && Store.sessionEpoch === epoch && Store.ready() && document.body.dataset.page === "page-fleet";
}

async function fleetLoad(syncProfile = false) {
  const pane = document.getElementById("fleet-pane"), tab = fleetScreen.tab;
  if (!pane) return;
  const epoch = Store.sessionEpoch, request = ++fleetScreen.request;
  document.querySelectorAll("#fleet-body [data-fleet-tab]").forEach(button => button.setAttribute("aria-pressed", String(button.dataset.fleetTab === tab)));
  pane.innerHTML = '<p class="fleet-empty" role="status">正在读取港务记录…</p>';
  try {
    if (syncProfile || tab !== "directory") {
      if (!await Store.refresh()) throw new Error("暂未取得最新公会记录");
      if (!fleetCurrent(request, epoch)) return;
      renderTopbar();
    }
    if (tab !== "directory") { pane.innerHTML = ({crew:fleetCrew,growth:fleetGrowth,raid:fleetRaid,shop:fleetShop,members:fleetMembers})[tab](Store.profile?.guild); return; }
    const after = tab === "directory" ? fleetScreen.pages.at(-1) : 0;
    const data = await Store.api(`${SITE.ep.fleet}?board=${tab}&after=${after}`, "GET");
    if (!fleetCurrent(request, epoch)) return;
    if (data?.board !== tab || !Array.isArray(data.entries)) throw new Error("榜单记录暂不可用");
    fleetScreen.next = data.next || 0;
    pane.innerHTML = fleetBoardMarkup(data);
  } catch (error) {
    if (fleetCurrent(request, epoch)) pane.innerHTML = `<div class="fleet-empty"><p>${esc(error.message)}</p>${fleetButton("重新读取", "refresh")}</div>`;
  }
}

async function fleetMutate(action, fields, message) {
  const epoch = Store.sessionEpoch, request = fleetScreen.request;
  if (fleetScreen.mutationEpoch === epoch) return;
  fleetScreen.mutationEpoch = epoch;
  try {
    const result = await Store.mutate(action, fields);
    if (!result || !fleetCurrent(request, epoch)) return;
    if (["guild_join", "guild_create", "guild_leave"].includes(action)) {
      fleetScreen.tab = action === "guild_leave" ? "directory" : "crew";
      fleetScreen.pages = [0];
    }
    renderTopbar();
    toast((result.drops || []).join(" · ") || message);
    if (action === "guild_attack" && result.guild_battle?.report) {
      const receipt = result.guild_battle;
      return await resumeBattle({ id: "guild-" + Date.now(), stage: "guild_raid", guild_battle: receipt, result: receipt.report, ready_at: Store.serverNow() + receipt.report.duration_ms }, false);
    }
    await fleetLoad();
  } finally { if (fleetScreen.mutationEpoch === epoch) fleetScreen.mutationEpoch = null; }
}
function finishGuildReplay(battle) {
  if (!battleCurrent(battle) || !battle.guildBattle) return;
  battle.settled = true; battleRunning = false;
  renderBattleControls(battle, "协力挑战已结算");
  const receipt = battle.guildBattle;
  showModal(`<h3>${receipt.cleared ? "阶段首领已击败" : "协力挑战完成"}</h3><p style="text-align:center">${esc(receipt.stage_name)} · 本次伤害 ${fleetNumber(receipt.damage)}</p><p style="text-align:center">首领生命 ${fleetNumber(receipt.boss_hp)} / ${fleetNumber(receipt.boss_max_hp)}</p><ul class="drop-list">${(receipt.drops || []).map(line => `<li>${esc(line)}</li>`).join("")}</ul><div class="modal-btns"><button class="gbtn gbtn-primary" onclick="leaveGuildReplay()"><span>返回公会副本</span></button></div>`);
}
function leaveGuildReplay() {
  if (!BAT?.guildBattle) return;
  const previous = BAT; previous.abort = true;
  if (typeof dismissBattleMenu === "function") dismissBattleMenu(previous, false);
  window.BattleMotion?.cancelBattleMotion(previous);
  BAT = null; battleRunning = false; closeModal(true);
  fleetScreen.tab = "raid"; nav("page-fleet");
}

function fleetClick(event) {
  const button = event.target.closest("button");
  if (!button || button.disabled) return;
  const command = button.dataset.fleetAct, value = button.dataset.value || "";
  const tab = button.dataset.fleetTab || (command === "tab" && value);
  if (tab && Object.hasOwn(fleetTabs, tab)) {
    fleetScreen.tab = tab; fleetScreen.pages = [0]; return fleetLoad();
  }
  switch (command) {
    case "hall": return openFleetRanking("power", button);
    case "refresh": return fleetLoad(true);
    case "previous": if (fleetScreen.pages.length > 1) { fleetScreen.pages.pop(); return fleetLoad(); } break;
    case "next": if (fleetScreen.next) { fleetScreen.pages.push(fleetScreen.next); return fleetLoad(); } break;
    case "recruit": if (/^[0-9A-F]{8}$/.test(value)) return fleetMutate("guild_join", { item_id: value }, "已加入公会"); break;
    case "supply": return fleetMutate("guild_supply", { item_id: value }, "交付完成");
    case "claim": return fleetMutate("guild_supply_claim", { count: Number(value) }, "里程奖励已领取");
    case "listing": return fleetMutate("guild_listing", { enabled: value === "on" }, "招募状态已更新");
    case "attack": return fleetMutate("guild_attack", {}, "讨伐完成");
    case "bossClaim": return fleetMutate("guild_claim", value ? {count:Number(value)} : {}, "讨伐奖励已领取");
    case "checkin": if (Store.profile?.guild?.activity?.tasks?.some(task=>task.id === "checkin" && task.can_claim)) return fleetMutate("guild_checkin", {}, "签到奖励已到账"); break;
    case "activityClaim": if (["sortie","training","arena"].includes(value) && Store.profile?.guild?.activity?.tasks?.some(task=>task.id === value && task.can_claim)) return fleetMutate("guild_activity_claim", {item_id:value}, "成长贡献已到账"); break;
    case "missionClaim": if (["supply","raid","active"].includes(value) && Store.profile?.guild?.activity?.missions?.some(task=>task.id === value && task.can_claim)) return fleetMutate("guild_mission_claim", {item_id:value}, "协力奖励已到账"); break;
    case "kick": return confirmFleetKick(Number(value));
    case "shop": return fleetMutate("guild_shop", {item_id:value}, "物资已入库");
    case "role": { const [id,role]=value.split(":"); if (/^[1-9][0-9]*$/.test(id) && ["deputy","elder","member"].includes(role)) return fleetMutate("guild_role", {opponent_id:Number(id),item_id:role}, "职位已更新"); break; }
    case "leave": return fleetMutate("guild_leave", {}, "已退出公会");
    case "copy": return copyGuildCode(value);
  }
}

function confirmFleetKick(id) {
  const member = Store.profile?.guild?.members?.find(entry=>entry.user_id === id);
  if (!member?.can_kick || !Store.ready() || document.body.dataset.page !== "page-fleet") return;
  const epoch=Store.sessionEpoch, code=Store.profile.guild.code;
  showModal(`<h3>移出公会成员</h3><p>确定将 <b>${esc(member.name)}</b>（${esc(fleetRoleName(member.role))} · ID ${id}）移出公会？</p><p class="fleet-note">对方的公会币和已获得物资保留，移出后不再属于本会。</p><div class="modal-btns"><button class="fleet-button" data-fleet-kick-cancel>取消</button><button class="fleet-button guild-danger" data-fleet-kick-confirm>确认移出</button></div>`);
  document.querySelector('[data-fleet-kick-cancel]').onclick=()=>closeModal();
  const confirm=document.querySelector('[data-fleet-kick-confirm]');
  confirm.onclick=async()=>{
    if(confirm.disabled || !confirm.isConnected || epoch!==Store.sessionEpoch || !Store.ready() || document.body.dataset.page!=="page-fleet" || code!==Store.profile?.guild?.code || !Store.profile.guild.members.some(entry=>entry.user_id===id&&entry.can_kick))return;
    confirm.disabled=true;
    closeModal();
    await fleetMutate("guild_kick",{opponent_id:id},"成员已移出公会");
  };
}

function fleetSubmit(event) {
  const form = event.target;
  if (!form.dataset.fleetForm) return;
  event.preventDefault();
  const data = new FormData(form);
  if (form.dataset.fleetForm === "join") {
    const code = String(data.get("code") || "").trim().toUpperCase();
    if (!/^[0-9A-F]{8}$/.test(code)) return toast("请输入 8 位邀请码");
    return fleetMutate("guild_join", { item_id: code }, "已加入公会");
  }
  const name = String(data.get("name") || "").trim();
  if (Array.from(name).length < 2 || Array.from(name).length > 12) return toast("公会名称需为 2 至 12 个字");
  return fleetMutate("guild_create", { item_id: name, enabled: data.has("listed") }, "公会已成立");
}

function renderFleet() {
  if (!requireGame()) return;
  if (fleetScreen.epoch !== Store.sessionEpoch) {
    fleetScreen.epoch = Store.sessionEpoch;
    fleetScreen.tab = Store.profile?.guild ? "crew" : "directory";
    fleetScreen.pages = [0];
  }
  const root = document.getElementById("fleet-body");
  root.innerHTML = `<div class="fleet-nav"><div role="group" aria-label="港务厅栏目">${Object.entries(fleetTabs).map(([tab, name]) => `<button type="button" data-fleet-tab="${tab}">${name}</button>`).join("")}${fleetButton("航海风云榜", "hall")}</div><button type="button" class="fleet-reload" data-fleet-act="refresh" aria-label="刷新港务厅" title="刷新港务厅">↻</button></div><div id="fleet-pane"></div>`;
  root.onclick = fleetClick;
  root.onsubmit = fleetSubmit;
  return fleetLoad();
}

const fleetRankKinds = {
  power: ["战斗力", "POWER", "战力", "当前上阵编队的综合战力。"], collection: ["收藏率", "COLLECTION", "收藏率", "已获得的不同整卡占图鉴总数的比例，碎片不计。"],
  guild: ["公会排行", "UNION", "本周航绩", "参与公开招募的公会：每 100 点讨伐伤害计 1 航绩，每次共建计 2 航绩。"], arena: ["赏金竞技", "ARENA", "竞技胜场", "只计已结算的竞技胜场，平局不计。"],
  stars: ["副本星级榜", "ADVENTURE", "副本星数", "四章剧情、困难、噩梦逐关取历史最高星。同一关不重复计数；资源副本与裂隙不计。"],
};
const fleetRank = { mode: "", board: "power", request: 0, view: null, opener: null, detailRow: null, fromLive: false };

function fleetRankExample(board) {
  const cast = [
    ["霁湾来信", "薄荷灯塔", 91, ["c20", "c14", "c03", "c27"], 81670, 25, 1084, 1756, 147],
    ["风笺十七", "纸鸢测候社", 84, ["c12", "c04", "c18", "c24"], 79240, 26, 864, 1682, 132],
    ["银杯听潮", "绒鲸邮局", 79, ["c16", "c21", "c05", "c09"], 76190, 23, 956, 1540, 138],
    ["半枚潮印", "雾屿船票行", 72, ["c08", "c27", "c02", "c25"], 64810, 24, 695, 1492, 121],
    ["栗壳信标", "双弦渡口", 68, ["c17", "c06", "c22", "c13"], 59760, 20, 738, 1324, 105],
    ["蓝檐回声", "砾石巡潮队", 63, ["c09", "c11", "c19", "c01"], 57480, 21, 633, 1266, 95],
    ["墨斗晚钟", "弯月帆铺", 59, ["c22", "c10", "c05", "c14"], 53620, 17, 528, 1130, 86],
    ["曙砂折页", "霜灯图书船", 56, ["c06", "c02", "c16", "c07"], 48950, 19, 436, 1064, 72],
  ];
  const metric = {power:4, collection:5, guild:6, arena:7, stars:8}[board];
  const entries = cast.map((row, index) => ({ name: row[board === "guild" ? 1 : 0], level: row[2], value: row[metric], mine: index === 5, members: 10 - index % 4,
    team: row[3].map((id, slot) => ({ id, level: row[2] - slot * 2, star: 4 + (index + slot) % 2 })) })).sort((left, right) => right.value - left.value);
  const mine = entries.findIndex(entry => entry.mine);
  return { board, entries, catalog_size: 28, star_limit: 156, member_limit: 10, my_name: entries[mine].name, my_level: entries[mine].level, my_value: entries[mine].value, my_rank: mine + 1 };
}

function fleetRankScore(board, value, view) {
  if (board === "collection") return view.catalog_size ? `${(100 * value / view.catalog_size).toFixed(1)}%` : "0.0%";
  return `${fleetNumber(value)}${board === "stars" ? " ★" : board === "arena" ? " 胜" : ""}`;
}

function fleetRankCards(entry) {
  if (!Array.isArray(entry.team) || typeof GIRLS === "undefined") return [];
  return entry.team.slice(0, 6).flatMap(card => {
    const girl = GIRLS.find(girl => girl.id === card?.id);
    return girl ? [{ girl, level: card.level, star: card.star }] : [];
  });
}

function fleetRankFaces(cards) {
  return cards.length ? `<span class="rank-faces">${cards.map(({girl, level}) => `<span><img src="${esc(girl.img)}" alt="${esc(girl.name)}" title="${esc(girl.name)}"><small>Lv.${fleetNumber(level)}</small></span>`).join("")}</span>` : '<span class="rank-unformed">未编队</span>';
}

function fleetRankRows(view) {
  const board = view.board, guild = board === "guild", kind = fleetRankKinds[board];
  const items = view.entries.map((entry, index) => {
    const cards = fleetRankCards(entry), captain = cards[0]?.girl;
    const caption = guild ? `${fleetNumber(entry.members)} / ${fleetNumber(view.member_limit)} 位成员` : entry.level ? `Lv.${fleetNumber(entry.level)}` : "船长";
    const extra = board === "collection" ? `${fleetNumber(entry.value)} / ${fleetNumber(view.catalog_size)} 张整卡` : board === "stars" ? `上限 ${fleetNumber(view.star_limit)} ★` : "";
    return `<li><button type="button" class="rank-row${entry.mine ? " rank-mine" : ""}" data-rank-row="${index}" aria-haspopup="dialog"><span class="rank-medal" data-medal="${index < 3 ? index + 1 : 0}">${index + 1}</span><span class="rank-player">${captain ? `<img class="rank-avatar" src="${esc(captain.img)}" alt="">` : '<span class="rank-avatar rank-emblem" aria-hidden="true">⚓</span>'}<span><b>${esc(entry.name)}</b><small>${esc(caption)}${entry.mine ? (guild ? " · 本会" : " · 我") : ""}</small></span></span><span class="rank-score">${fleetRankScore(board, entry.value, view)}${extra ? `<small>${extra}</small>` : ""}</span>${fleetRankFaces(cards)}</button></li>`;
  }).join("");
  const standing = view.my_rank ? `第 ${fleetNumber(view.my_rank)} 名` : "暂未上榜";
  return `<div class="rank-columns"><span>排名</span><span>${guild ? "公会" : "船长"}</span><span>${kind[2]}</span><span>${guild ? "会长编队" : "队伍"}</span></div><div class="rank-scroll" tabindex="0" aria-label="排行榜名单">${items ? `<ol class="fleet-list rank-list">${items}</ol>` : `<div class="rank-empty"><b>暂无上榜记录</b><p>${kind[3]}</p><button type="button" data-rank-action="demo">查看演示榜</button></div>`}</div><footer class="fleet-standing rank-standing"><span>${guild ? "本会名次" : "我的名次"}</span><b>${standing}</b><span>${esc(view.my_name || "")}${view.my_level ? ` · ${guild ? "会长 " : ""}Lv.${fleetNumber(view.my_level)}` : ""}</span><strong>${kind[2]}：${fleetRankScore(board, view.my_value || 0, view)}</strong></footer>`;
}

function fleetOpenDialog(dialog) {
  if (typeof dialog.showModal === "function") dialog.showModal(); else dialog.setAttribute("open", "");
}

function fleetRemoveDialog(dialog) {
  if (typeof dialog?.close === "function" && dialog.open) dialog.close();
  dialog?.remove();
}

function closeFleetRankDetail() {
  const detail = document.getElementById("fleet-rank-detail");
  if (!detail) return;
  fleetRemoveDialog(detail);
  fleetRank.detailRow?.focus(); fleetRank.detailRow = null;
}

function closeFleetRanking() {
  const hall = document.getElementById("fleet-rank-hall");
  if (!hall) return;
  closeFleetRankDetail(); fleetRank.request++; fleetRank.view = null; fleetRank.mode = "";
  fleetRemoveDialog(hall);
  if (/^#rank-demo(?:-|$)/.test(location.hash)) history.replaceState(history.state, "", location.href.split("#")[0]);
  const fallback = document.querySelector('[data-rank-demo-launch], #title-screen:not(.hidden) [onclick^="openFleetRankDemo"], #app:not(.hidden) .page.active button, #title-screen:not(.hidden) #auth-actions:not(.hidden) button');
  (fleetRank.opener?.isConnected ? fleetRank.opener : fallback)?.focus();
  fleetRank.opener = null;
}

function closeFleetOfficial() {
  if (fleetRank.mode === "live") closeFleetRanking();
  else if (fleetRank.fromLive) {
    fleetRank.fromLive = false;
    const source = document.querySelector('#fleet-rank-hall [data-rank-action="source"]');
    if (source) source.hidden = true;
  }
}

function showFleetRankDetail(row, button) {
  const view = fleetRank.view, entry = view?.entries[row];
  if (!entry || !button || document.getElementById("fleet-rank-detail")) return;
  const dialog = document.createElement("dialog"), kind = fleetRankKinds[view.board], cards = fleetRankCards(entry);
  dialog.id = "fleet-rank-detail"; dialog.className = "rank-detail"; dialog.setAttribute("aria-labelledby", "rank-detail-name");
  dialog.innerHTML = `<header><small>${fleetRank.mode === "demo" ? "演示榜 · 虚构档案" : "正式榜 · 公开档案"}</small><h3 id="rank-detail-name">${esc(entry.name)}</h3></header><p>第 ${fleetNumber(row + 1)} 名 · ${kind[2]} ${fleetRankScore(view.board, entry.value, view)}</p><h4>${view.board === "guild" ? "会长编队" : "上阵队伍"}</h4>${fleetRankFaces(cards)}<p class="rank-rule">${kind[3]}</p><button type="button" data-rank-action="close-detail">返回排行榜</button>`;
  fleetRank.detailRow = button;
  document.getElementById("fleet-rank-hall").append(dialog);
  dialog.addEventListener("cancel", event => { event.preventDefault(); closeFleetRankDetail(); });
  fleetOpenDialog(dialog); dialog.querySelector("button").focus();
}

async function fleetRankLoad(board) {
  if (!Object.hasOwn(fleetRankKinds, board)) return;
  const hall = document.getElementById("fleet-rank-hall");
  if (!hall) return;
  closeFleetRankDetail();
  const demo = fleetRank.mode === "demo", request = ++fleetRank.request, epoch = demo ? null : Store.sessionEpoch;
  const current = () => hall.isConnected && request === fleetRank.request && (demo || (Store.ready() && epoch === Store.sessionEpoch));
  fleetRank.board = board; fleetRank.view = null;
  hall.dataset.rankBoard = board; hall.dataset.rankSource = demo ? "demo" : "live";
  hall.querySelectorAll("[data-fleet-tab]").forEach(button => button.setAttribute("aria-pressed", String(button.dataset.fleetTab === board)));
  hall.querySelector("[data-rank-origin]").textContent = demo ? "演示榜 · 虚构船长与公会" : "正式榜 · 全服";
  const toggle = hall.querySelector('[data-rank-action="source"]'); toggle.hidden = demo && !fleetRank.fromLive; toggle.textContent = demo ? "返回正式榜" : "查看演示榜";
  const pane = hall.querySelector(".rank-content");
  pane.innerHTML = '<p class="rank-empty" role="status">正在读取榜单…</p>';
  try {
    const view = demo ? fleetRankExample(board) : await Store.api(`${SITE.ep.fleet}?board=${board}&after=0`, "GET");
    if (!current()) return;
    if (!view || view.board !== board || !Array.isArray(view.entries)) throw new Error("暂未取得排行榜");
    fleetRank.view = view; pane.innerHTML = fleetRankRows(view);
  } catch (error) {
    if (current()) pane.innerHTML = `<div class="rank-empty" role="status"><b>暂未取得正式榜</b><p>${esc(error.message)}</p><button type="button" data-rank-action="retry">重新读取</button><button type="button" data-rank-action="demo">查看演示榜</button></div>`;
  }
}

function fleetRankClick(event) {
  const button = event.target.closest("button");
  if (!button) return;
  if (button.dataset.fleetTab) return fleetRankLoad(button.dataset.fleetTab);
  if (button.hasAttribute("data-rank-row")) return showFleetRankDetail(Number(button.dataset.rankRow), button);
  switch (button.dataset.rankAction) {
    case "back": return closeFleetRanking();
    case "close-detail": return closeFleetRankDetail();
    case "retry": return fleetRankLoad(fleetRank.board);
    case "demo": case "source": {
      if (fleetRank.mode === "live") { fleetRank.fromLive = true; fleetRank.mode = "demo"; }
      else if (fleetRank.fromLive && Store.ready()) fleetRank.mode = "live";
      else return;
      document.querySelector(`#fleet-rank-hall [data-fleet-tab="${fleetRank.board}"]`).focus();
      return fleetRankLoad(fleetRank.board);
    }
  }
}

function openFleetHall(board, opener, demo) {
  if (!Object.hasOwn(fleetRankKinds, board)) return;
  let hall = document.getElementById("fleet-rank-hall");
  if (!hall) {
    hall = document.createElement("dialog"); hall.id = "fleet-rank-hall"; hall.setAttribute("aria-labelledby", "rank-hall-title");
    hall.innerHTML = `<header class="rank-heading"><button class="rank-back" type="button" data-rank-action="back" aria-label="返回上一页" title="返回上一页">↶</button><h2 id="rank-hall-title">航海风云榜<small>HALL OF FAME</small></h2><div class="rank-source"><b data-rank-origin></b><button type="button" data-rank-action="source"></button></div></header><div class="rank-layout"><nav class="rank-sidebar" aria-label="排行榜分类">${Object.entries(fleetRankKinds).map(([key, kind]) => `<button type="button" data-fleet-tab="${key}"><b>${kind[0]}</b><small>${kind[1]}</small></button>`).join("")}</nav><section class="rank-content" aria-label="当前排行榜"></section></div>`;
    hall.addEventListener("click", fleetRankClick);
    hall.addEventListener("cancel", event => { if (event.target === hall) { event.preventDefault(); closeFleetRanking(); } });
    hall.addEventListener("keydown", event => {
      const active = event.target.dataset.fleetTab;
      if (!active || !["ArrowUp", "ArrowDown", "ArrowLeft", "ArrowRight", "Home", "End"].includes(event.key)) return;
      event.preventDefault();
      const tabs = Object.keys(fleetRankKinds), index = tabs.indexOf(active), direction = ["ArrowUp", "ArrowLeft"].includes(event.key) ? -1 : 1;
      const next = tabs[event.key === "Home" ? 0 : event.key === "End" ? tabs.length - 1 : (index + direction + tabs.length) % tabs.length];
      hall.querySelector(`[data-fleet-tab="${next}"]`).focus(); fleetRankLoad(next);
    });
    fleetRank.opener = opener?.isConnected ? opener : document.activeElement !== document.body ? document.activeElement : null;
    document.body.append(hall); fleetOpenDialog(hall);
  }
  fleetRank.fromLive = false; fleetRank.mode = demo ? "demo" : "live";
  hall.querySelector(`[data-fleet-tab="${board}"]`).focus();
  return fleetRankLoad(board);
}

function openFleetRanking(board = "power", opener) { return openFleetHall(board, opener, !Store.ready()); }
function openFleetRankDemo(board = "stars", opener) { return openFleetHall(board, opener, true); }

function fleetRankHash() {
  const match = /^#rank-demo(?:-(power|collection|guild|arena|stars))?$/.exec(location.hash);
  if (match) openFleetRankDemo(match[1] || "stars");
}
window.addEventListener("hashchange", fleetRankHash);
if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", fleetRankHash, {once:true}); else fleetRankHash();

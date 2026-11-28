/* Progression renders server-issued opportunities; the browser never awards resources. */
let growthSelectedGirl = null;
let growthAccount = null;
let pendingGrowthConfig = null;
let growthRestTimer = null;

GACHA_RUN_MODES.training = { id: "training", name: "经验训练", type: "成长训练", waves: 1, img: "assets/d06-observatory.webp", cost: 0, progression: true };
GACHA_RUN_MODES.dual_trial = { id: "dual_trial", name: "双队契约试炼", type: "双队挑战", waves: 2, img: "assets/d06-observatory.webp", cost: 14, progression: true };
for (const girl of GIRLS) {
  for (let rank = 1; rank <= 3; rank++) {
    const id = `trial_${girl.id}_${rank}`;
    GACHA_RUN_MODES[id] = { id, name: `${girl.name}专属试炼 · ${rank}阶`, type: "角色试炼", waves: rank === 3 ? 2 : 1, img: "assets/d06-observatory.webp", cost: 0, progression: true };
  }
}
for (const rarity of ["R", "SR", "SSR"]) {
  MATS["shard_dust_" + rarity.toLowerCase()] = { name: `${rarity}共鸣尘`, color: RARITY[rarity].color, from: "旧版分解库存", use: "旧尘用途以当前服务器物品目录为准" };
}
MATS.shard_dust.use = "旧版保留材料，详见回收说明";

const GROWTH_ACTIVITY_NAMES = { training: "经验训练", wish: "定向许愿", campaign: "主线副本", rift: "裂隙挑战", trial: "专属试炼", weekly: "远征或首领", dual: "双队挑战" };

function growthBudget() {
  const p = SAVE?.progression;
  if (!p) return { training: 0, wishes: 0, permits: 0, readyMarks: 0, refinement: 0 };
  return {
    training: p.days.reduce((n, d) => n + d.training_left, 0),
    wishes: p.days.reduce((n, d) => n + Math.max(0, d.rules.wishes_per_day - d.wish_targets.length), 0),
    permits: p.days.reduce((n, d) => n + (d.permits_claimed ? 0 : d.rules.permits_per_day), 0),
    readyMarks: p.days.reduce((n, d) => n + (!d.marks_claimed && d.activities.length >= 2 ? d.rules.marks_per_day : 0), 0),
    refinement: p.weeks.reduce((n, w) => n + w.refinements_left, 0),
  };
}

function growthStarMax() {
  return Math.min(GROWTH.starMax, 5 + (cultivationRules()?.advanced_awakening.length || 0));
}

function growthStarCost(girl, nextStar) {
  if (nextStar < 2 || nextStar > growthStarMax()) return 0;
  if (nextStar > 5) return cultivationRules().advanced_shards[girl.rarity][nextStar - 6];
  if (girl.rarity === "R") return [20, 40, 60, 80][nextStar - 2];
  if (girl.rarity === "SR") return ({ 3: 40, 4: 80, 5: 120 })[nextStar] || 0;
  return GROWTH.starCost[nextStar] || 0;
}

function growthDust(girl) {
  if (cultivationRules()?.shards) return 0;
  return (SAVE.mats.shard_dust || 0) + (SAVE.mats["shard_dust_" + girl.rarity.toLowerCase()] || 0);
}

function growthStarStep(girl, own) {
  if (!own?.star || own.star >= growthStarMax()) return null;
  const next = own.star + 1, rules = cultivationRules(), shards = growthStarCost(girl, next);
  const material = MAT_STAR_COST[girl.rarity] * (next > 5 ? rules.advanced_mat_multiplier[next - 6] : 1);
  const awakening = next > 5 ? rules.advanced_awakening[next - 6] : 0;
  const ready = shards > 0 && (own.shards || 0) + growthDust(girl) >= shards && (SAVE.mats[MAT_BY_RARITY[girl.rarity]] || 0) >= material && (own.awakening || 0) >= awakening;
  return { next, shards, material, awakening, ready };
}

function growthBranches(id) { return Store.profile?.progression?.branches?.[id] || []; }

function growthSkill(id) {
  const girl = GIRLS.find(g => g.id === id);
  const own = SAVE?.girls[id];
  if (!own?.awakening) return girl.skills.skill;
  const branch = growthBranches(id).find(b => b.id === own.branch);
  return { name: branch?.name || "觉醒技能", desc: branch?.description || "服务器按已选择的专属路线结算技能" };
}

function growthGoal(id) {
  const girl = GIRLS.find(g => g.id === id);
  const own = SAVE?.girls[id];
  const p = SAVE?.progression;
  if (!girl || !own || own.star < 1 || !p) return { title: "先领取入门奖励", note: "通关主线、领取招募券并选择第一名核心角色。", action: "stage" };
  const rank = (own.awakening || 0) + 1;
  if (rank > 3 && cultivationRules() && own.star < growthStarMax()) {
    const step = growthStarStep(girl, own), mat = MAT_BY_RARITY[girl.rarity];
    return { title: `${girl.name} · 冲刺${step.next}星`, note: `${cultivationRules()?.shards ? "本卡专属碎片" : "碎片与可用尘"} ${(own.shards || 0) + growthDust(girl)}/${step.shards}，${MATS[mat].name} ${SAVE.mats[mat] || 0}/${step.material}。`, action: "cultivation" };
  }
  if (rank > 3) return { title: `${girl.name}构筑已成型`, note: "培养互补的备用角色，挑战双队试炼；极品词条是可选目标。", action: "dual" };
  const level = 30 + rank * 10, star = rank + 2;
  if (own.level < level) return { title: `${girl.name}觉醒${rank}：先到${level}级`, note: `当前${own.level}级。经验训练与副本都提供真实经验，金币不能直升。`, action: "training" };
  if (own.star < star) {
    const need = growthStarCost(girl, own.star + 1);
    return { title: `${girl.name}觉醒${rank}：先升至${star}星`, note: `下一星还缺${Math.max(0, need - own.shards - growthDust(girl))}枚碎片；获取途径：${cardSourceDescription(girl)}。`, action: cardSources(girl).includes("growth") ? "wish" : "card_source" };
  }
  if ((p.trial_wins[id] || 0) < rank) return { title: `${girl.name}觉醒${rank}：通过专属试炼`, note: "开局会确保该角色出战。失败可重试，练习不重复发奖。", action: "trial", rank };
  const cost = Store.profile?.progression?.rules?.awakening_costs?.[rank - 1] || [80, 120, 200][rank - 1];
  if (p.marks < cost) return { title: `${girl.name}觉醒${rank}：还缺${cost - p.marks}契约印`, note: "完成两种不同活动领取日契约，完成周常委托获得额外契约印。不能用金币或E购买。", action: "contracts" };
  return { title: `${girl.name}可以觉醒${rank}阶`, note: rank === 1 ? "选择专属技能路线；两条路线互斥，之后可花250金币改选。" : "通过试炼后消耗契约印，永久解锁下一阶技能效果。", action: "awakening", rank };
}

function growthHeroOptions(selected, ownedOnly = false, wish = false) {
  return GIRLS.filter(g => ownedOnly ? (SAVE.girls[g.id]?.star || 0) > 0 : cardSources(g).includes("growth") && (!wish || SAVE.progression?.days.some(day => day.wish_targets.length < day.rules.wishes_per_day && !day.wish_targets.includes(g.id)))).map(g => `<option value="${g.id}" ${g.id === selected ? "selected" : ""}>${esc(g.name)} · ${g.rarity}${ownedOnly ? ` · Lv${SAVE.girls[g.id].level}` : ""}</option>`).join("");
}

function selectGrowthGirl(id) {
  if (!requireGame() || !GIRLS.some(g => g.id === id)) return;
  growthAccount = Store.authUserId;
  growthSelectedGirl = id;
  renderGrowth();
}

function openGrowthGirl(id) {
  if (!requireGame() || !SAVE.girls[id]?.star) return;
  growthAccount = Store.authUserId;
  growthSelectedGirl = id;
  closeModal();
  nav("page-growth");
}

function renderGrowthSummary() {
  const box = document.getElementById("home-growth");
  if (!box || !SAVE?.progression) return;
  const budget = growthBudget();
  const id = SAVE.squad.find(id => (SAVE.girls[id]?.awakening || 0) < 3) || SAVE.squad[0] || GIRLS.find(girl => SAVE.girls[girl.id]?.star)?.id;
  const goal = growthGoal(id);
  box.innerHTML = `<article class="growth-next"><div><p class="social-kicker">下一步成长</p><h3>${esc(goal.title)}</h3><p>${esc(goal.note)}</p><div class="growth-chips"><span>训练 ${budget.training} 次</span><span>许愿 ${budget.wishes} 次</span><span>待领招募券 ${budget.permits}</span><span>契约印 ${SAVE.progression.marks}</span></div></div><button type="button" class="gbtn gbtn-primary" onclick="openGrowthGirl('${id}')"><span>前往成长中心</span></button></article>`;
}

function renderGrowth() {
  if (!requireGame()) return;
  const body = document.getElementById("growth-body");
  if (!body) return;
  const p = SAVE.progression, rules = Store.profile?.progression?.rules;
  if (!p || !rules) {
    body.innerHTML = `<div class="growth-empty"><p>正在同步成长规则，请刷新站点档案。</p><button class="gbtn" onclick="refreshGrowth()"><span>重新同步</span></button></div>`;
    return;
  }
  if (growthAccount !== Store.authUserId) {
    growthAccount = Store.authUserId;
    growthSelectedGirl = null;
    pendingGrowthConfig = null;
  }
  if (!growthSelectedGirl || !SAVE.girls[growthSelectedGirl]?.star) growthSelectedGirl = SAVE.squad[0] || GIRLS.find(g => SAVE.girls[g.id]?.star)?.id;
  const budget = growthBudget(), goal = growthGoal(growthSelectedGirl);
  const dossier = typeof atelierGrowthDossier === "function" && typeof workshopRules === "function" && workshopRules() && Store.profile.equipment && SAVE.girls[growthSelectedGirl]?.star ? atelierGrowthDossier(growthSelectedGirl,goal) : "";
  const fallback = dossier ? "" : `
    <article class="growth-next"><div><p class="social-kicker">培养目标 · 可随时换人</p><h3>${esc(goal.title)}</h3><p>${esc(goal.note)}</p><button type="button" class="gbtn gbtn-sm" onclick="followGrowthGoal()"><span>前往对应玩法</span></button></div><label class="growth-select">关注角色<select id="growth-hero" onchange="selectGrowthGirl(this.value)">${growthHeroOptions(growthSelectedGirl, true)}</select></label></article>
    <div class="growth-roadmap" aria-label="成长路线"><span><b>入门</b>核心自选与主线</span><span><b>首周</b>基础阵容与许愿</span><span><b>进阶</b>${cultivationRules() ? "角色觉醒与专武铭刻" : "专属觉醒与路线"}</span><span><b>长线</b>${cultivationRules() ? `${growthStarMax()}星突破与收藏羁绊` : "八人双队与周挑战"}</span></div>
    ${cultivationRules() && SAVE.girls[growthSelectedGirl]?.star ? typeof workshopEntry === "function" && workshopRules() ? workshopEntry(growthSelectedGirl) : `<section id="growth-cultivation" class="cultivation-workbench"><h3>${esc(GIRLS.find(girl => girl.id === growthSelectedGirl).name)} · 卡牌培养</h3>${starTrainingMarkup(growthSelectedGirl)}${cultivationPanel(growthSelectedGirl)}${collectionMarkup(growthSelectedGirl)}</section>` : ""}`;
  body.innerHTML = `${dossier}
    <div class="growth-resources" aria-label="成长资源"><div><small>招募券</small><b>${p.permits}</b></div><div><small>契约印</small><b>${p.marks}</b></div><div><small>积攒训练</small><b>${budget.training}<em>次</em></b></div><div><small>定向机会</small><b>${budget.refinement}<em>次</em></b></div></div>${fallback}
    <p id="growth-rest-note" class="sd-rest-note" role="status"></p><div class="growth-grid">
      <article class="mode-card growth-daily" id="growth-contracts"><div class="mode-card-head"><div><p class="social-kicker">任选两种活动 · 可积攒7天</p><h3>日常成长委托</h3></div><span class="growth-badge">可领 ${budget.readyMarks} 契约印</span></div>
        <p class="mode-note">训练、许愿、主线、裂隙、专属试炼、周常、双队任选两种。每次实际完成会推进一个积攒委托，不是一份进度同时领七天。</p>
        <div class="growth-actions"><button type="button" data-growth-run class="gbtn gbtn-primary" onclick="startGrowthRun('training')" ${budget.training < 1 || battleRunning ? "disabled" : ""}><span>经验训练 · ${budget.training}次</span><span class="sub">固定经验 · 不消耗体力 · 失败可重试</span></button><button type="button" class="gbtn" onclick="nav('page-stage')"><span>挑战主线或裂隙</span><span class="sub">不同活动推进契约委托</span></button></div>
        <label class="growth-select">许愿角色<select id="growth-wish">${growthHeroOptions(growthSelectedGirl, false, true)}</select></label><button type="button" class="gbtn gbtn-sm" onclick="wishGrowthGirl()" ${budget.wishes < 1 ? "disabled" : ""}><span>定向许愿 · ${budget.wishes}次剩余</span><span class="sub">只列本卡来源允许且仍有资格的角色 · 未解锁也可攒碎片</span></button>
        <details class="growth-bank" open><summary>查看积攒委托和招募券</summary><div>${p.days.map(d => `<div class="growth-bank-row"><div><b>${esc(d.date)}</b><small>${d.activities.map(k => GROWTH_ACTIVITY_NAMES[k]).join(" + ") || "尚未完成活动"} · ${d.activities.length}/2</small><small>训练 ${d.training_left} · 许愿 ${Math.max(0, d.rules.wishes_per_day - d.wish_targets.length)}</small></div><div class="growth-bank-buttons"><button type="button" class="gbtn gbtn-sm" onclick="claimGrowthDay('${d.date}','daily_permits')" ${d.permits_claimed ? "disabled" : ""}><span>${d.permits_claimed ? "招募已寄" : `领${d.rules.permits_per_day}招募券`}</span></button><button type="button" class="gbtn gbtn-sm gbtn-primary" onclick="claimGrowthDay('${d.date}','daily_contract_claim')" ${d.marks_claimed || d.activities.length < 2 ? "disabled" : ""}><span>${d.marks_claimed ? "契约已寄" : `领${d.rules.marks_per_day}契约印`}</span></button></div></div>`).join("")}</div></details>
      </article>
      ${growthAwakeningMarkup(growthSelectedGirl)}
      <article class="mode-card growth-weekly"><div class="mode-card-head"><div><p class="social-kicker">五次实际活动 · 至少两种</p><h3>周常委托与定向培养</h3></div></div><p class="mode-note">周资格保留两周。契约印不能购买，灵印定向保留家族和另一词条；已达到保底值的同类型词条不会白扣次数。</p><label class="growth-select">周常自选碎片<select id="growth-weekly-girl">${growthHeroOptions(growthSelectedGirl)}</select></label>${p.weeks.map(w => `<div class="growth-bank-row"><div><b>${esc(w.week)}</b><small>${w.completed}/5 次 · ${w.activities.length}种活动 · 定向${w.refinements_left}次</small><small>契约印${w.rules.marks_per_week} · 招募券${w.rules.permits_per_week} · 自选碎片${w.rules.shards_per_week}</small></div><button type="button" class="gbtn gbtn-sm" onclick="claimGrowthWeek('${w.week}')" ${w.claimed || w.completed < 5 || w.activities.length < 2 ? "disabled" : ""}><span>${w.claimed ? "本周已领" : "领取周常"}</span></button></div>`).join("")}<div class="growth-actions"><button type="button" class="gbtn gbtn-sm" onclick="nav('page-bag')"><span>背包定向灵印</span></button><button type="button" class="gbtn gbtn-sm" onclick="nav('page-modes')"><span>远征、首领、公会</span></button></div></article>
      ${growthMainlineMarkup()}
      ${growthDualMarkup()}
    </div>
    <p class="mode-help">每日机会只积攒已到来的7个自然日，周机会保留当前和上一周，不预支未来产出。已有角色、等级、星级、旧共鸣尘、宝石、灵印与E余额均保留；60级5星是基础培养完成，不是玩法结束。</p>
    ${Store.profile.progression.can_configure ? growthConfigMarkup(rules) : ""}`;
  updateGrowthTeams(false);
  body.querySelectorAll("[data-growth-run]").forEach(button => { button.dataset.baseBlocked = button.disabled ? "1" : "0"; });
  updateGrowthRest(true);
}

function followGrowthGoal() {
  if (!requireGame()) return;
  const goal = growthGoal(growthSelectedGirl);
  if (goal.action === "card_source") return openCardSource(growthSelectedGirl);
  if (goal.action === "training") return growthBudget().training > 0 ? startGrowthRun("training") : nav("page-stage");
  if (goal.action === "trial") return startGrowthRun(`trial_${growthSelectedGirl}_${goal.rank}`);
  if (goal.action === "stage") return nav("page-stage");
  const targetId = { wish:"growth-wish", contracts:"growth-contracts", awakening:"growth-awakening", dual:"growth-dual", cultivation:"growth-cultivation" }[goal.action];
  const target = document.getElementById(targetId);
  target?.scrollIntoView?.({ block:"start", behavior:"auto" });
  if (goal.action === "wish") target?.focus();
}

function stopGrowthRestTimer() {
  if (growthRestTimer !== null) clearInterval(growthRestTimer);
  growthRestTimer = null;
}

function updateGrowthRest(startTimer = false) {
  if (!Store.ready() || document.body.dataset.page !== "page-growth") { stopGrowthRestTimer(); return; }
  const remaining = gameRestSeconds();
  const note = document.getElementById("growth-rest-note");
  if (note) note.textContent = remaining > 0 ? `连续挑战休整中，${remaining}秒后可出战；领取、许愿、编队与养成仍可使用。` : "";
  document.querySelectorAll("#growth-body [data-growth-run]").forEach(button => { button.disabled = remaining > 0 || button.dataset.baseBlocked === "1"; });
  if (remaining <= 0) stopGrowthRestTimer();
  else if (startTimer && growthRestTimer === null) growthRestTimer = setInterval(() => updateGrowthRest(false), 1000);
}

function growthAwakeningMarkup(id) {
  const own = SAVE.girls[id], girl = GIRLS.find(g => g.id === id);
  if (!own || !girl) return "";
  const p = SAVE.progression, rank = Math.min(3, (own.awakening || 0) + 1);
  const full = own.awakening === 3, level = 30 + rank * 10, star = rank + 2;
  const ready = own.level >= level && own.star >= star;
  const passed = (p.trial_wins[id] || 0) >= rank;
  const cost = Store.profile.progression.rules.awakening_costs[rank - 1];
  const eligible = !full && ready && passed && p.marks >= cost;
  const branches = growthBranches(id);
  return `<article class="mode-card growth-awakening" id="growth-awakening"><div class="growth-hero-head"><img src="${girl.img}" alt="${esc(girl.name)}" loading="lazy"><div><p class="social-kicker">角色专属 · 技能有取舍</p><h3>${esc(girl.name)} · 觉醒${own.awakening || 0}/3</h3><p>Lv${own.level} · ${own.star}星 · ${full ? "三阶已完成" : `下一阶需${level}级${star}星`}</p></div></div>
    <div class="growth-exp"><span>等级经验 ${own.level >= cardLevelLimit() ? `已达当前卡级上限 Lv.${cardLevelLimit()}` : `${own.exp || 0}/${cardLevelCost(own.level)}`}</span><div class="mode-progress"><div style="width:${own.level >= cardLevelLimit() ? 100 : Math.min(100,(own.exp || 0)/cardLevelCost(own.level)*100)}%"></div></div></div>
    <p class="mode-note">${full ? "路线可以低成本改选；接着培养第二支队伍。" : `觉醒${rank}消耗${cost}契约印，当前${p.marks}。${passed ? "专属试炼已通过。" : "需要先通过对应试炼。"}`}</p>
    <div class="growth-branches">${branches.map(b => `<div class="growth-branch ${b.id === own.branch ? "selected" : ""}"><b>${esc(b.name)}</b><p>${esc(b.description)}</p><button type="button" class="gbtn gbtn-sm" onclick="confirmGrowthBranch('${id}','${b.id}')" ${own.awakening ? b.id === own.branch || SAVE.gold < 250 ? "disabled" : "" : !eligible ? "disabled" : ""}><span>${own.awakening ? b.id === own.branch ? "当前路线" : "改选 · 250金币" : "选择路线并觉醒"}</span></button></div>`).join("")}</div>
    <div class="growth-actions"><button type="button" data-growth-run class="gbtn gbtn-sm" onclick="startGrowthRun('trial_${id}_${rank}')" ${!ready || battleRunning ? "disabled" : ""}><span>${passed ? "练习专属试炼" : `挑战${rank}阶专属试炼`}</span><span class="sub">免费重试 · 练习不重复产出</span></button>${own.awakening && !full ? `<button type="button" class="gbtn gbtn-sm gbtn-primary" onclick="confirmGrowthAwakening('${id}')" ${eligible ? "" : "disabled"}><span>觉醒${rank}阶 · ${cost}契约印</span></button>` : ""}<button type="button" class="gbtn gbtn-sm" onclick="openGirl('${id}')"><span>查看装备与升星</span></button></div></article>`;
}

function growthMainlineMarkup() {
  const p = SAVE.progression;
  // The server serializes an unclaimed Go slice as null.
  const claims = p.mainline_claims ?? [];
  return `<article class="mode-card growth-mainline"><div class="mode-card-head"><div><p class="social-kicker">每个账号一次 · 老玩家也可领取</p><h3>主线成长补给</h3></div></div>${[{ id: "s1", n: 5, xp: 14000 }, { id: "s3", n: 10, xp: 8000 }, { id: "s6", n: 15, xp: 0 }].map(m => { const stage = STAGES.find(s => s.id === m.id); const claimed = claims.includes(m.id); return `<div class="growth-bank-row"><div><b>${esc(stage.name)}</b><small>${m.n}招募券${m.xp ? ` · 出战经验${m.xp}` : ""} · 金币补给</small></div><button type="button" class="gbtn gbtn-sm" onclick="claimGrowthMainline('${m.id}')" ${claimed || !SAVE.cleared[m.id] ? "disabled" : ""}><span>${claimed ? "已领取" : SAVE.cleared[m.id] ? "领取奖励" : "先通关"}</span></button></div>`; }).join("")}
    <h4 class="growth-subhead">首核心SSR自选</h4><p class="mode-note">通关第一章冰晶殿堂后选择输出、盾卫或治疗核心，附带入门经验。已拥有时转为该角色碎片，不覆盖原培养。</p><div class="growth-starters">${GIRLS.filter(girl => cardSources(girl).includes("starter")).map(girl => `<button type="button" class="growth-starter" onclick="confirmGrowthStarter('${girl.id}')" ${p.starter_claimed || !SAVE.cleared.s2 ? "disabled" : ""}><img src="${girl.img}" alt="" loading="lazy"><b>${esc(girl.name)}</b><small>${({ c04: "输出核心", c07: "护盾核心", c28: "治疗核心" })[girl.id] || "入门核心"}</small></button>`).join("")}</div><p class="mode-note">${p.starter_claimed ? "本账号首核心奖励已领取。" : "终身只可领取一次，请先确认阵容需要。"}</p></article>`;
}

function growthDualMarkup() {
  const p = SAVE.progression, dual = p.dual;
  const owned = GIRLS.filter(g => (SAVE.girls[g.id]?.star || 0) > 0).map(g => g.id);
  const selected = [[...SAVE.squad], [...(p.reserve_squad || [])]];
  for (const team of selected) for (const id of owned) if (team.length < 4 && !selected.flat().includes(id)) team.push(id);
  const locked = dual.squads?.length === 2;
  return `<article class="mode-card growth-dual" id="growth-dual"><div class="mode-card-head"><div><p class="social-kicker">每队 4–6 人 · 每周机制轮换</p><h3>双队契约试炼</h3></div><span class="growth-badge">${dual.cleared ? `本周完成 · ${dual.best_rounds}回合` : `已完成${dual.phase || 0}/2战`}</span></div><p class="mode-note">一队清场，一队应对首领机制。每名角色至少40级3星；两战各消耗14体力，失败可重试。保存时两队人数相同、角色不重复，未选位置按顺序补齐。开局锁定名单，之后改常用队伍不会替换已锁定名单。</p>
    <div class="growth-team-editor">${[0, 1].map(team => `<fieldset><legend>第${team + 1}队 · 最多6人</legend>${Array.from({ length: 6 }, (_, slot) => `<label>${slot < 3 ? "前排" : "后排"}${slot + 1}<select id="growth-team-${team * 6 + slot}" onchange="updateGrowthTeams()"><option value="">${slot < 4 ? "请选择角色" : "可选角色"}</option>${growthHeroOptions(selected[team][slot], true)}</select></label>`).join("")}</fieldset>`).join("")}</div>
    <p id="growth-team-warning" class="growth-team-warning" role="status"></p><div class="growth-actions"><button type="button" class="gbtn gbtn-sm" id="growth-team-save" onclick="saveGrowthTeams()"><span>保存两支常用队伍</span></button><button type="button" data-growth-run class="gbtn gbtn-sm gbtn-primary" id="growth-dual-start" onclick="startGrowthRun('dual_trial')" ${dual.cleared || !SAVE.cleared.s6 || SAVE.stamina < 14 ? "disabled" : ""}><span>${dual.cleared ? "本周试炼已完成" : `挑战第${(dual.phase || 0) + 1}战 · 14体力`}</span></button></div>
    ${locked ? `<div class="growth-locked"><b>本周锁定名单</b>${dual.squads.map((team, i) => `<p>第${i + 1}队：${team.map(id => esc(GIRLS.find(g => g.id === id)?.name || id)).join("、")}</p>`).join("")}</div>` : ""}</article>`;
}

function growthTeamSelection() {
  return [0, 1].map(team => Array.from({ length: 6 }, (_, slot) => document.getElementById("growth-team-" + (team * 6 + slot))?.value || "").filter(Boolean));
}
function updateGrowthTeams(updateRest = true) {
  const teams = growthTeamSelection(), values = teams.flat();
  const warning = document.getElementById("growth-team-warning"), save = document.getElementById("growth-team-save"), start = document.getElementById("growth-dual-start");
  if (!warning || !save || !start) return;
  const distinct = teams[0].length >= 4 && teams[0].length === teams[1].length && new Set(values).size === values.length;
  save.disabled = !distinct;
  const saved = [...SAVE.squad, ...(SAVE.progression.reserve_squad || [])];
  const qualified = SAVE.squad.length >= 4 && SAVE.squad.length <= 6 && (SAVE.progression.reserve_squad || []).length >= 4 && (SAVE.progression.reserve_squad || []).length <= 6 && new Set(saved).size === saved.length && saved.every(id => SAVE.girls[id]?.level >= 40 && SAVE.girls[id]?.star >= 3);
  const locked = SAVE.progression.dual.squads?.length === 2;
  const same = teams[0].length === SAVE.squad.length && values.length === saved.length && values.every((id, i) => id === saved[i]);
  start.disabled = Boolean(battleRunning || SAVE.progression.dual.cleared || !SAVE.cleared.s6 || SAVE.stamina < 14 || !locked && (!qualified || !same));
  start.dataset.baseBlocked = start.disabled ? "1" : "0";
  warning.textContent = locked ? "本周试炼继续使用已锁定名单。" : !same && !distinct ? "保存时每队选择相同人数（4–6人），不能重复角色。" : !same ? "先保存当前两队，再开启试炼。" : !qualified ? "已保存的两队都需要4–6人，每名角色至少40级3星。" : "两队已保存，可以出战。";
  if (updateRest) updateGrowthRest(false);
}

async function growthMutate(action, fields) {
  if (!requireGame()) return null;
  const epoch = Store.sessionEpoch;
  const result = await Store.mutate(action, fields);
  if (!result || epoch !== Store.sessionEpoch || !Store.ready()) return null;
  closeModal();
  renderTopbar();
  if (document.body.dataset.page === "page-growth") renderGrowth();
  if (document.body.dataset.page === "page-home") renderGrowthSummary();
  toast((result.drops || []).join(" · ") || "成长操作已确认");
  return result;
}

async function refreshGrowth() {
  if (!Store.ready()) return;
  const epoch = Store.sessionEpoch;
  const result = await Store.refresh();
  if (result && epoch === Store.sessionEpoch && Store.ready()) { renderGrowth(); renderTopbar(); }
}

async function claimGrowthDay(day, action) {
  if (!["daily_permits", "daily_contract_claim"].includes(action)) return;
  await growthMutate(action, { item_id: day });
}

async function wishGrowthGirl() {
  const id = document.getElementById("growth-wish")?.value;
  if (id) await growthMutate("wish", { girl_id: id });
}

async function claimGrowthWeek(week) {
  const id = document.getElementById("growth-weekly-girl")?.value;
  if (id) await growthMutate("weekly_claim", { item_id: week, girl_id: id });
}

async function claimGrowthMainline(stage) { await growthMutate("mainline_claim", { item_id: stage }); }

async function unlockShardGirl(id) {
  const result = await growthMutate("shard_unlock", { girl_id: id });
  if (!result) return;
  if (document.body.dataset.page === "page-squad") renderSquad();
  if (document.body.dataset.page === "page-bag") renderBag();
  if (document.body.dataset.page === "page-gacha") renderGacha();
  openGirl(id);
}

function confirmGrowthStarter(id) {
  if (!requireGame()) return;
  const girl = GIRLS.find(g => g.id === id);
  if (!girl || !cardSources(girl).includes("starter")) return;
  showModal(`<h3>选择${esc(girl.name)}？</h3><div class="social-confirm"><p>每个账号只可自选一次SSR核心，已有角色不会覆盖，会获得其专属碎片与入门经验。</p></div><div class="modal-btns"><button class="gbtn" onclick="closeModal()"><span>再想想</span></button><button class="gbtn gbtn-primary" onclick="growthMutate('starter_select',{girl_id:'${id}'})"><span>确认选择</span></button></div>`);
}

function confirmGrowthBranch(id, choice) {
  if (!requireGame()) return;
  const own = SAVE.girls[id], branch = growthBranches(id).find(b => b.id === choice);
  if (!own || !branch) return;
  const action = own.awakening ? "awakening_branch" : "awakening";
  const cost = own.awakening ? "250金币，觉醒阶级保留" : `${Store.profile.progression.rules.awakening_costs[0]}契约印`;
  showModal(`<h3>${esc(branch.name)}</h3><div class="social-confirm"><p>${esc(branch.description)}</p><p>本次消耗${esc(cost)}。两条路线互斥，之后可以改选。</p></div><div class="modal-btns"><button class="gbtn" onclick="closeModal()"><span>取消</span></button><button class="gbtn gbtn-primary" onclick="growthMutate('${action}',{girl_id:'${id}',choice:'${choice}'})"><span>确认路线</span></button></div>`);
}

function confirmGrowthAwakening(id) {
  if (!requireGame()) return;
  const own = SAVE.girls[id];
  if (!own || !own.awakening || own.awakening >= 3) return;
  const rank = own.awakening + 1, cost = Store.profile.progression.rules.awakening_costs[rank - 1];
  showModal(`<h3>觉醒${rank}阶？</h3><div class="social-confirm"><p>消耗${cost}契约印，永久强化当前专属路线。角色等级、星级与装备不回退。</p></div><div class="modal-btns"><button class="gbtn" onclick="closeModal()"><span>取消</span></button><button class="gbtn gbtn-primary" onclick="growthMutate('awakening',{girl_id:'${id}'})"><span>确认觉醒</span></button></div>`);
}

function startGrowthRun(id) {
  if (!requireGame() || battleRunning) return;
  if (gameRestSeconds() > 0) return toast(`休整中，${gameRestSeconds()}秒后可出战`);
  const mode = GACHA_RUN_MODES[id];
  if (!mode?.progression) return;
  const note = id === "training" ? "出战角色获得固定真实经验，使用最早的积攒训练机会；失败不消耗资格，不产出E或金币。" : id === "dual_trial" ? "按本周锁定队伍出战，每战14体力。两战完成获得金币、灵印粉尘和本周记录，不产出额外契约印。" : "角色达到等级和星级要求后可挑战。首次通过解锁对应觉醒资格；练习不重复计委托，也不发资源。";
  showModal(`<h3>${esc(mode.name)}</h3><div class="social-confirm"><p>${esc(note)}</p><p>胜负与奖励由服务端保存，即使关页面也会结算。</p></div><div class="modal-btns"><button class="gbtn" onclick="closeModal()"><span>返回</span></button><button class="gbtn gbtn-primary" onclick="closeModal();startBattle('${id}')"><span>确认挑战</span></button></div>`);
}

async function saveGrowthTeams() {
  const teams = growthTeamSelection(), squad = teams.flat();
  if (teams[0].length < 4 || teams[0].length !== teams[1].length || new Set(squad).size !== squad.length) return toast("每队选择相同人数（4–6人），两队不可重复角色");
  await growthMutate("teams_save", { squad });
}

function awakeningDetailMarkup(id, own) {
  const branch = growthBranches(id).find(b => b.id === own.branch);
  return `<div class="talent-panel"><div class="mode-card-head"><b>专属觉醒 ${own.awakening || 0}/3</b><button class="gbtn gbtn-sm" onclick="openGrowthGirl('${id}')"><span>查看成长目标</span></button></div><p>${branch ? `${esc(branch.name)}：${esc(branch.description)}` : "40级3星起可挑战专属试炼，选择技能路线；契约印来自日常与周常。"}</p></div>`;
}

function openSigilRefine(id) {
  if (!requireGame()) return;
  const sigil = SAVE.sigils[id];
  if (!sigil) return;
  const keys = Object.keys(sigil.affixes).sort();
  showModal(`<h3>灵印定向培养</h3><div class="social-confirm"><p>消耗一次周定向机会、500金币与10灵印粉尘。只替换选中的一条属性，保留家族和另一条属性，保证该类型的中档数值；不会降低已更好的同类型属性。</p><label class="growth-select">要替换的属性<select id="refine-slot">${keys.map((key, i) => `<option value="${i + 1}">${SIGIL_STATS[key].name} ${sigil.affixes[key]}${SIGIL_STATS[key].unit}</option>`).join("")}</select></label><label class="growth-select">目标类型<select id="refine-type">${Object.entries(SIGIL_STATS).map(([key, value]) => `<option value="${key}">${value.name}</option>`).join("")}</select></label></div><div class="modal-btns"><button class="gbtn" onclick="closeModal()"><span>取消</span></button><button class="gbtn gbtn-primary" onclick="confirmSigilRefine('${id}')" ${growthBudget().refinement < 1 || SAVE.gold < 500 || (SAVE.mats.sigil_dust || 0) < 10 ? "disabled" : ""}><span>确认定向 · ${growthBudget().refinement}次剩余</span></button></div>`);
}

async function confirmSigilRefine(id) {
  const count = Number(document.getElementById("refine-slot")?.value), choice = document.getElementById("refine-type")?.value;
  const sigil = SAVE?.sigils[id];
  const old = sigil && Object.keys(sigil.affixes).sort()[count - 1];
  if (!old || !SIGIL_STATS[choice]) return;
  if (choice !== old && sigil.affixes[choice]) return toast("目标类型已在另一条属性中，请选择不同类型；不会扣除资源");
  const result = await growthMutate("sigil_refine", { item_id: id, count, choice });
  if (result && document.body.dataset.page === "page-bag") renderBag();
}

function growthConfigMarkup(rules) {
  const fields = [
    ["training_per_day", "每日训练次数", 1, 5], ["permits_per_day", "每日招募券", 1, 10],
    ["permits_per_week", "周常招募券", 0, 30], ["marks_per_day", "日契约印", 10, 80],
    ["marks_per_week", "周契约印", 0, 240], ["wishes_per_day", "每日许愿目标数", 1, 4],
    ["shards_per_wish", "每次许愿碎片", 1, 20], ["shards_per_week", "周常自选碎片", 0, 80],
    ["refinements_per_week", "每周定向次数", 1, 4],
  ];
  return `<details class="growth-admin"><summary>管理员成长配置 · 当前版本${rules.version}</summary><p class="mode-note">无需重新构建镜像。只影响未来发放的新机会；已积攒资格、已发出的战斗和领取凭据不改。契约印不能购买的规则不开放修改。</p><form id="growth-config-form" onsubmit="prepareGrowthConfig(event)"><div class="growth-config-fields">${fields.map(([key, label, min, max]) => `<label>${label}<input type="number" name="${key}" min="${min}" max="${max}" step="1" required value="${rules[key]}"></label>`).join("")}${rules.training_xp.map((xp, i) => `<label>训练档${i + 1}经验<input type="number" name="training_xp_${i}" min="500" max="10000" step="1" required value="${xp}"></label>`).join("")}${rules.awakening_costs.map((cost, i) => `<label>觉醒${i + 1}契约印<input type="number" name="awakening_costs_${i}" min="20" max="1000" step="1" required value="${cost}"></label>`).join("")}</div><button type="submit" class="gbtn gbtn-sm"><span>检查并保存下一版本</span></button></form></details>`;
}

function prepareGrowthConfig(event) {
  event.preventDefault();
  if (!requireGame() || !Store.profile?.progression?.can_configure) return;
  const form = document.getElementById("growth-config-form");
  if (!form.reportValidity()) return;
  const rules = Store.profile.progression.rules;
  const config = { ...rules, version: rules.version + 1, training_xp: [...rules.training_xp], awakening_costs: [...rules.awakening_costs] };
  for (const [name, value] of new FormData(form)) {
    const number = Number(value);
    if (!Number.isSafeInteger(number)) return toast("配置必须是有效整数");
    if (name.startsWith("training_xp_")) config.training_xp[Number(name.slice(-1))] = number;
    else if (name.startsWith("awakening_costs_")) config.awakening_costs[Number(name.slice(-1))] = number;
    else config[name] = number;
  }
  if (config.training_xp.some((n, i) => i > 0 && n < config.training_xp[i - 1]) || config.awakening_costs.some((n, i) => i > 0 && n < config.awakening_costs[i - 1])) return toast("训练经验与觉醒消耗必须随档位递增");
  pendingGrowthConfig = { config, epoch: Store.sessionEpoch };
  showModal(`<h3>保存成长配置版本${config.version}？</h3><div class="social-confirm"><p>已发放的日常和周常资格仍按原数值领取，新数值从下一次发放开始生效。</p><p>每日${config.marks_per_day}契约印，周常${config.marks_per_week}；每人三阶合计${config.awakening_costs.reduce((n, v) => n + v, 0)}。</p></div><div class="modal-btns"><button class="gbtn" onclick="closeModal()"><span>取消</span></button><button class="gbtn gbtn-primary" onclick="saveGrowthConfig()"><span>确认保存</span></button></div>`);
}

async function saveGrowthConfig() {
  if (!pendingGrowthConfig || pendingGrowthConfig.epoch !== Store.sessionEpoch || !Store.profile?.progression?.can_configure) return;
  const result = await growthMutate("progression_config", { config: pendingGrowthConfig.config });
  if (result) pendingGrowthConfig = null;
}

function trainingRules() { return Store.profile?.training; }

function trainingCard(own) {
  const value = own?.training || {};
  return { grade:value.grade || 0, plus:value.plus || 0, stage:value.stage || 0, effort:value.effort || {}, skills:value.skills || {} };
}

function trainingFrame(own) {
  if (!trainingRules() || !own?.star) return "";
  const card = trainingCard(own), grade = trainingRules().grades[card.grade];
  return `data-break-quality="${esc(grade.id)}" data-break-plus="${card.plus}"`;
}

function trainingGradeTag(own) {
  if (!trainingRules() || !own?.star) return "";
  const card = trainingCard(own), grade = trainingRules().grades[card.grade];
  return `<span class="tr-grade-tag" ${trainingFrame(own)}>${esc(grade.name)} +${card.plus}</span>`;
}

function trainingBonuses(own) {
  const rules = trainingRules(), card = trainingCard(own), flat = { hp:0, atk:0, def:0, spd:0, res:0 };
  if (!rules || !own?.star) return { percent:0, flat };
  const steps = rules.grades.slice(0,card.grade).reduce((total,grade) => total + grade.max_plus + 1,card.plus);
  for (const stat of rules.effort_stats) flat[stat.id] = effortBonus(rules,stat,card.effort[stat.id] || 0);
  return { percent:steps * rules.break_percent, flat };
}

function effortBonus(rules, stat, value) {
  if (!rules.effort_knee) return Math.floor(value/stat.step)*stat.gain;
  return Math.floor(Math.min(value,rules.effort_knee)/stat.early_step)*stat.early_gain + Math.floor(Math.max(0,value-rules.effort_knee)/stat.step)*stat.gain;
}

function trainingQuote(id, choice = "hp") {
  const rules = trainingRules(), own = SAVE?.girls[id];
  if (!rules || !own?.star) return null;
  const card = trainingCard(own), grade = rules.grades[card.grade], nextGrade = rules.grades[card.grade+1];
  const stage = rules.effort_stages[card.stage], nextStage = rules.effort_stages[card.stage+1];
  const ink = SAVE.mats.training_ink || 0, seals = SAVE.mats.break_seal || 0;
  const stones = SAVE.mats.effort_stone || 0;
  const cross = card.plus === grade.max_plus, end = cross && !nextGrade;
  const completed = rules.grades.slice(0,card.grade).reduce((total,entry) => total + entry.max_plus + 1,card.plus);
  const cost = rules.break_cost, breakLevel = cross ? nextGrade?.level : grade.plus_levels?.[card.plus];
  const breakInk = cost && cost.ink_base + completed*cost.ink_step + completed*(completed+1)/2;
  const breakSeals = cost && Math.floor((completed+1)/cost.seal_interval);
  const breakGold = cost && cost.gold_base + completed * cost.gold_step;
  const effortGold = rules.effort_gold_base + card.stage * rules.effort_gold_step;
  const canBreak = !!cost && !end && own.level >= breakLevel && ink >= breakInk && seals >= breakSeals && SAVE.gold >= breakGold;
  const full = rules.effort_stats.every(stat => (card.effort[stat.id] || 0) === stage.cap);
  const dailyLeft = Math.max(0,rules.effort_daily - (SAVE.daily?.effort_spent?.[id] || 0));
  const batch = effortGold > 0 && rules.effort_stats.some(stat => stat.id === choice) ? Math.min(dailyLeft,Math.floor((ink+stones)/stage.ink),Math.floor(SAVE.gold/effortGold),Math.ceil((stage.cap-(card.effort[choice] || 0))/stage.gain)) : 0;
  return { card, grade, nextGrade, stage, nextStage, ink, stones, seals, cross, end, breakLevel, breakInk, breakSeals, breakGold, effortGold, canBreak, full, dailyLeft, batch,
    canTrain:batch > 0, canPromote:!!nextStage && full && seals >= nextStage.seals };
}

let pendingTrainingRequest = null;

function trainingBusy() { return pendingTrainingRequest?.epoch === Store.sessionEpoch; }

function selectEffortStat(stat) {
  const view = currentWorkshop();
  if (view?.tab !== "training" || !trainingRules()?.effort_stats.some(entry => entry.id === stat)) return;
  view.effortChoice = stat;
  paintWorkshop(`[data-effort-stat="${stat}"]`);
}

function selectTrainingPanel(panel) {
  const view = currentWorkshop();
  if (view?.tab !== "training" || !["breakthrough","effort","skills"].includes(panel)) return;
  view.trainingPanel = panel;
  delete view.promotedStage;
  paintWorkshop(`[data-training-panel="${panel}"]`);
}

async function actTraining(action, choice) {
  if (["skill_point","skill_auto","skill_buy"].includes(action)) return actTrainingSkill(action,choice);
  const view = currentWorkshop();
  if (view?.tab !== "training" || trainingBusy()) return;
  const train = action === "effort_train" || action === "effort_train_max";
  const selected = train ? view.effortChoice || "hp" : choice, quote = trainingQuote(view.id,selected);
  if (!quote) return;
  const ready = {card_breakthrough:quote.canBreak, effort_train:quote.canTrain, effort_train_max:quote.canTrain, effort_promote:quote.canPromote};
  if (!Object.hasOwn(ready,action) || !ready[action] || choice !== undefined) return;
  const fields = { girl_id:view.id };
  if (train) fields.choice = selected;
  return submitTrainingIntent(view,action,fields);
}

async function submitTrainingIntent(view, action, fields) {
  const token = {epoch:view.epoch};
  pendingTrainingRequest = token;
  let result;
  const previousStage = trainingCard(SAVE.girls[view.id]).stage;
  try {
    paintWorkshop();
    result = await Store.mutate(action,fields);
    if (result && action === "effort_promote" && currentWorkshop() === view && view.trainingPanel === "effort" && trainingCard(SAVE.girls[view.id]).stage === previousStage+1) view.promotedStage = previousStage+1;
  } finally {
    if (pendingTrainingRequest === token) pendingTrainingRequest = null;
    if (Store.sessionEpoch === token.epoch && Store.ready()) {
      if (result) refreshCultivationViews(view.id);
      const visible = document.querySelector("#modal-mask:not(.hidden) .hero-workshop");
      if (visible?.dataset.tab === "training" && (!result || visible.dataset.cultivationCard !== view.id)) paintWorkshop();
    }
  }
  if (result && Store.sessionEpoch === token.epoch && Store.ready()) toast((result.drops || []).join(" · ") || "研修已确认");
}

function trainingBreakPanel(id, quote) {
  const trainingPending = trainingBusy();
  const own = SAVE.girls[id], rules = trainingRules(), { cross, end, breakLevel, breakInk, breakSeals } = quote;
  const bonuses = trainingBonuses(own);
  const needs = [["卡牌等级", own.level, breakLevel], ["研训墨", quote.ink, breakInk]];
  needs.push(["金币", SAVE.gold, quote.breakGold]);
  if (breakSeals) needs.push(["晋阶章", quote.seals, breakSeals]);
  return `<section class="tr-panel"><h4>卡牌突破 ${trainingGradeTag(own)}</h4><p>生命、攻击、防御 +${bonuses.percent}%</p>${end ? '<b>已达最高突破</b>' : !rules.break_cost ? '<p>请更新服务端突破规则。</p>' : `<dl class="tr-cost" data-break-cost>${needs.map(([label,stock,needed]) => `<div><dt>${label}</dt><dd>${stock} / ${needed} <small>${stock < needed ? `还差 ${needed-stock}` : '已满足'}</small></dd></div>`).join('')}</dl><button type="button" class="ws-command" data-training-action="card_breakthrough" ${quote.canBreak && !trainingPending ? '' : 'disabled'} onclick="actTraining('card_breakthrough')">${trainingPending ? '结算中…' : cross ? '突破升色' : '突破 +1'}</button>`}</section>`;
}

function trainingEffortPanel(view, quote) {
  const trainingPending = trainingBusy();
  const rules = trainingRules(), {card, stage, nextStage} = quote, choice = view.effortChoice || "hp";
  const floor = rules.effort_stages[card.stage-1]?.cap || 0, selected = rules.effort_stats.find(stat => stat.id === choice);
  const sessions = rules.effort_stats.reduce((amount, stat) => amount + Math.ceil((stage.cap-(card.effort[stat.id] || 0))/stage.gain),0);
  const days = Math.ceil(Math.ceil((stage.cap-floor)/stage.gain)*rules.effort_stats.length/rules.effort_daily);
  if (view.promotedStage === card.stage && card.stage > 0) return `<section class="tr-chapter" data-effort-arrival aria-live="polite"><small>努力研修 · 晋段成功</small><h4>${esc(stage.name)}</h4><p>五项各 ${floor} 点已继承，本阶段单项上限 <b>${stage.cap}</b>。</p><button type="button" class="ws-command" autofocus onclick="selectTrainingPanel('effort')">开始本阶段训练</button></section>`;
  return `<section class="tr-panel"><h4>努力值 · ${esc(stage.name)}</h4><div class="tr-budget" data-effort-budget><b>今日可练 ${quote.dailyLeft}/${rules.effort_daily} 次</b><span>每卡独立 · 北京时间零点恢复</span><small>该阶段从起点练满五项至少 ${days} 个游戏日；当前还差 ${sessions} 次训练。</small></div><div class="tr-effort">${rules.effort_stats.map(stat => {
    const value = card.effort[stat.id] || 0;
    return `<button type="button" data-effort-stat="${stat.id}" aria-pressed="${stat.id === choice}" onclick="selectEffortStat('${stat.id}')"><b>${esc(stat.name)}</b><progress aria-label="${esc(stat.name)}阶段进度" max="${stage.cap-floor}" value="${value-floor}"></progress><span>${value}/${stage.cap}</span><small>实战 +${effortBonus(rules,stat,value)}</small></button>`;
  }).join("")}</div><p>每练${esc(selected.name)} +${stage.gain}（不超过 ${stage.cap}），耗 ${stage.ink} 份培养材料、${quote.effortGold} 金币。优先用努力培养石（${quote.stones}），不足以研训墨（${quote.ink}）补齐；持有 ${SAVE.gold} 金币。</p><div class="tr-effort-actions"><button type="button" class="ws-command" data-training-action="effort_train" ${quote.canTrain && !trainingPending ? "" : "disabled"} onclick="actTraining('effort_train')">培养 1 次</button><button type="button" class="ws-command" data-training-action="effort_train_max" ${quote.canTrain && !trainingPending ? "" : "disabled"} onclick="actTraining('effort_train_max')">一键培养 · ${quote.batch} 次</button></div><div class="tr-promote"><b>${nextStage ? `下一阶段 · ${esc(nextStage.name)}` : "最终阶段"}</b><p>${nextStage ? `五项全满 ${stage.cap} 点后可晋段。已有数值保留，上限升为 ${nextStage.cap}；晋阶章 ${quote.seals}/${nextStage.seals}。` : `每项努力值最高 ${stage.cap} 点。`}</p><button type="button" class="ws-command" data-training-action="effort_promote" ${quote.canPromote && !trainingPending ? "" : "disabled"} onclick="actTraining('effort_promote')">${nextStage ? "晋升努力阶段" : "无下一阶段"}</button></div></section>`;
}

function trainingSkillQuote(id, line) {
  const rules = trainingRules()?.skill_pool, pool = SAVE?.skill_pool, own = SAVE?.girls[id];
  if (!rules || !pool || !own?.star || !["normal","rage","passive"].includes(line)) return null;
  const learned = trainingCard(own).skills[line] || 0, effective = Math.min(learned,own.level);
  const cap = own.level, next = rules.upgrade_gold_base + rules.upgrade_gold_step*learned, gold = SAVE.gold || 0;
  let count = 0, cost = 0;
  while (count < pool.points && learned+count < cap) {
    const price = next + rules.upgrade_gold_step*count;
    if (cost+price > gold) break;
    cost += price; count++;
  }
  return {learned,effective,cap,next,count,cost};
}

function trainingCanBuy(count) {
  const rules = trainingRules()?.skill_pool, pool = SAVE?.skill_pool;
  return !!rules && !!pool && Number.isSafeInteger(count) && count >= 1 && count <= rules.capacity-pool.points && (SAVE.gold || 0) >= count*rules.buy_gold;
}

function actTrainingSkill(action, value) {
  const view = currentWorkshop();
  if (view?.tab !== "training" || view.trainingPanel !== "skills" || trainingBusy()) return;
  if (action === "skill_buy") {
    if (trainingCanBuy(value)) return submitTrainingIntent(view,action,{count:value});
    return;
  }
  if (!["skill_point","skill_auto"].includes(action) || !trainingSkillQuote(view.id,value)?.count) return;
  return submitTrainingIntent(view,action,{girl_id:view.id,choice:value});
}

let trainingClockTimer = null, trainingClockRequest = null, trainingClockRetry = 0;

function trainingClockLabel(pool, rules) {
  if (pool.points >= rules.capacity) return "已满 · 恢复暂停";
  const seconds = Math.max(0,Math.ceil((pool.tick_at+rules.interval_seconds*1000-Store.serverNow())/1000));
  return seconds ? `${seconds} 秒后 +1` : "正在同步恢复点数";
}

function scheduleTrainingClock() {
  if (trainingClockTimer === null) trainingClockTimer = setTimeout(updateTrainingClock,500);
}

async function updateTrainingClock() {
  trainingClockTimer = null;
  const clock = document.querySelector('#modal-mask:not(.hidden) [data-skill-clock]'), view = currentWorkshop();
  const pool = SAVE?.skill_pool, rules = trainingRules()?.skill_pool;
  if (!clock || view?.tab !== "training" || view.trainingPanel !== "skills" || !rules || !pool) return;
  clock.textContent = trainingClockLabel(pool,rules);
  if (pool.points >= rules.capacity) return;
  const due = pool.tick_at+rules.interval_seconds*1000 <= Store.serverNow();
  if (due && document.visibilityState !== "hidden" && !trainingBusy() && !Store.mutationBusy && !trainingClockRequest && performance.now() >= trainingClockRetry) {
    const token = {epoch:view.epoch};
    trainingClockRequest = token;
    try {
      const profile = await Store.refresh();
      if (profile && Store.sessionEpoch === token.epoch && document.querySelector('#modal-mask:not(.hidden) [data-skill-clock]')) paintWorkshop();
    } finally {
      if (trainingClockRequest === token) trainingClockRequest = null;
      trainingClockRetry = performance.now()+1500;
    }
  }
  scheduleTrainingClock();
}

function trainingSkillPanel(id) {
  const rules = trainingRules()?.skill_pool, pool = SAVE.skill_pool, girl = GIRLS.find(entry => entry.id === id);
  if (!rules || !pool) return `<section class="tr-panel"><h4>技能加点</h4><p>当前服务端未开放技能点池。</p></section>`;
  const busy = trainingBusy(), skill = growthSkill(id);
  if (pool.points < rules.capacity) scheduleTrainingClock();
  const names = {normal:girl.skills.normal.name,rage:skill.name,passive:"蓄势"};
  const percent = value => `${Number((value/100).toFixed(2))}%`;
  return `<section class="tr-panel tr-skills"><header class="tr-energy"><div class="tr-pool-head"><h4>共享技能点 <b data-skill-points>${pool.points}/${rules.capacity}</b></h4><span data-skill-clock>${trainingClockLabel(pool,rules)}</span></div><progress aria-label="共享技能点" value="${pool.points}" max="${rules.capacity}"></progress><small>每 ${rules.interval_seconds} 秒恢复 1 点 · 金币 ${(SAVE.gold || 0).toLocaleString()}</small><div class="tr-pool-buy">${[1,5].map(count => `<button type="button" class="ws-command" data-skill-buy="${count}" ${!busy && trainingCanBuy(count) ? "" : "disabled"} onclick="actTraining('skill_buy',${count})">买 ${count} 点 · ${count*rules.buy_gold} 金币</button>`).join("")}</div></header><div class="tr-skill-lines">${Object.keys(names).map(line => {
    const {learned,effective,cap,next,count,cost} = trainingSkillQuote(id,line);
    const boost = line === "normal" ? `普攻倍率 +${percent(effective*rules.normal_bps)}` : line === "rage" ? `伤害、治疗及护盾倍率 +${percent(effective*rules.rage_bps)}，施放回怒 ${Math.floor(effective/rules.cast_refund_every)}` : `普攻额外回复 ${Math.floor(effective/rules.normal_rage_every)} 怒气`;
    const step = line === "normal" ? `每级 +${percent(rules.normal_bps)}` : line === "rage" ? `每级 +${percent(rules.rage_bps)}，每 ${rules.cast_refund_every} 级回怒 +1` : `每 ${rules.normal_rage_every} 级回怒 +1`;
    const availability = learned >= cap ? '已达当前等级上限' : !pool.points ? '等待技能点恢复' : !count ? `还差 ${next-(SAVE.gold || 0)} 金币` : '';
    return `<section data-skill-line="${line}"><div><small>${({normal:"普攻",rage:"怒气技",passive:"被动"})[line]}</small><h4>${esc(names[line])}</h4><div class="tr-skill-ranks">生效 Lv.<b data-skill-effective>${effective}</b> / ${cap} · 已学 <b data-skill-learned>${learned}</b></div><progress aria-label="${esc(names[line])}生效等级" value="${effective}" max="${cap || 1}"></progress><p>${boost}</p><small>${step}</small></div><div class="tr-skill-actions">${availability ? `<strong>${availability}</strong>` : ''}<button type="button" class="ws-command" data-skill-up="${line}" ${count && !busy ? "" : "disabled"} onclick="actTraining('skill_point','${line}')">升 1 级 <small>${learned >= cap ? '等级受限' : `1 点 · ${next} 金币`}</small></button><button type="button" class="ws-command" data-skill-auto="${line}" ${count && !busy ? "" : "disabled"} onclick="actTraining('skill_auto','${line}')">${count ? `自动升至 Lv.${learned+count}` : '自动升级'} <small>${count} 点 · ${cost} 金币</small></button></div></section>`;
  }).join("")}</div><small>自动升只使用已有技能点，不代购。</small></section>`;
}

function trainingSheet(view, girl) {
  const quote = trainingQuote(girl.id,view.effortChoice || "hp"), tab = view.trainingPanel || "breakthrough";
  return `<div class="tr-sheet" data-training-sheet="${girl.id}"><div class="ws-paper"><nav class="ws-subtabs" aria-label="研修方式">${Object.entries({breakthrough:"卡牌突破",effort:"努力值",skills:"技能加点"}).map(([key,label]) => `<button type="button" data-training-panel="${key}" aria-pressed="${key === tab}" onclick="selectTrainingPanel('${key}')">${label}</button>`).join("")}</nav>${tab === "effort" ? trainingEffortPanel(view,quote) : tab === "skills" ? trainingSkillPanel(girl.id) : trainingBreakPanel(girl.id,quote)}</div></div>`;
}

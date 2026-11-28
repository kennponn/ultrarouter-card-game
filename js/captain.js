let captainBoundaryTimer = null;

const captainDestinations = {voyage:["page-stage","去出航"],drill:["page-growth","去训练","#growth-contracts [data-growth-run]"],wish:["page-growth","去许愿","#growth-wish"],outfit:["page-growth","去培养","#growth-cultivation"],ladder:["page-arena","去挑战"],equipment:["page-growth","去强化","[data-growth-gear]"],skill:["page-growth","去学技能"],festival:["page-festival","去集章"]};
const captainXPSource = "每实际消耗 1 点体力，获得 1 点船长经验。每日任务还可领取额外经验。";

function captainGrowth() {
  const captain = SAVE?.captain, rules = Store.profile?.captain_rules;
  if (!Number.isSafeInteger(captain?.level) || captain.level < 1) return null;
  const full = captain.level >= GROWTH.maxLevel;
  const xp = Number.isSafeInteger(captain.xp) && captain.xp >= 0 ? captain.xp : null;
  const cost = Number.isSafeInteger(rules?.xp_base) && Number.isSafeInteger(rules?.xp_step) ? rules.xp_base + rules.xp_step * captain.level : 0;
  return {level:captain.level, xp, full, cost:cost > 0 ? cost : null};
}

function captainMeter(growth) {
  if (!growth) return '<span>船长经验待同步</span>';
  const {full,xp,cost} = growth;
  const amount = full ? 'value="1" max="1"' : cost && xp !== null ? `value="${xp}" max="${cost}"` : "";
  return `<span>${full ? "船长经验 · 已满级" : `船长经验 ${xp ?? "待同步"} / ${cost ?? "门槛待同步"}`}</span><progress aria-label="船长升级进度" ${amount}></progress>`;
}

function captainIdentity() {
  const profile = Store.profile, collection = SAVE?.girls && typeof GIRLS !== "undefined" && GIRLS.length ? GIRLS.filter(girl => SAVE.girls[girl.id]?.star > 0) : null;
  const leader = SAVE?.squad?.length && collection?.find(girl => girl.id === SAVE.squad[0]);
  return {name:profile?.name || "昵称待同步", id:profile?.user_id || "待同步", guild:profile?.guild?.name || (SAVE?.guild || !profile ? "待同步" : "尚未入会"), collection, portrait:leader || collection?.[0]};
}

function captainPortrait(girl) {
  return girl ? `<img src="${esc(girl.img)}" alt="">` : '<span aria-hidden="true">⚓</span>';
}

function renderCaptainMast(growth) {
  const mast = document.getElementById("captain-home-profile");
  if (!mast) return;
  const identity = captainIdentity();
  document.getElementById("captain-home-name").textContent = identity.name;
  document.getElementById("captain-home-level").textContent = growth ? `Lv.${growth.level}` : "等级待同步";
  document.getElementById("captain-home-xp").innerHTML = captainMeter(growth);
  const face = document.getElementById("captain-home-face"), portraitKey = identity.portrait?.id || "anchor";
  if (face.dataset.portrait !== portraitKey) { face.innerHTML = captainPortrait(identity.portrait); face.dataset.portrait = portraitKey; }
  document.getElementById("captain-home-sync").hidden = !!growth && (growth.full || growth.cost !== null && growth.xp !== null);
  if (typeof renderVoyageSummary === "function") renderVoyageSummary();
}

function openCaptainProfile(opener = document.activeElement) {
  if (!requireGame()) return;
  const identity = captainIdentity(), growth = captainGrowth(), ledger = typeof voyageLedger === "function" ? voyageLedger() : null, epoch = Store.sessionEpoch;
  const count = identity.collection?.length, size = count == null ? null : GIRLS.length;
  const facts = [["玩家 ID",identity.id],["所属公会",identity.guild],["伙伴数量",count ?? "待同步"],["图鉴收藏率",size ? `${(count*100/size).toFixed(1)}%（${count}/${size}）` : "待同步"],["编队战力",Number.isSafeInteger(SAVE.power) ? SAVE.power.toLocaleString("zh-CN") : "待同步"],["当前体力",Number.isSafeInteger(SAVE.stamina) && Number.isSafeInteger(SAVE.staminaMax) ? `${SAVE.stamina} / ${SAVE.staminaMax}` : "待同步"],["主线副本星数",ledger ? `${ledger.stars} / ${ledger.maximum}` : "待同步"],["伙伴等级上限",growth ? `Lv.${growth.level}` : "待同步"]];
  const shortcuts = [["story","主线副本"],["chests","副本星级宝箱"],["stars","副本星级榜"],["power","战力排行"],["collection","收藏率排行"],["journal","每日任务与体力"],["inbox","邮箱"],["redeem","兑换码"]];
  showModal(`<h3>船长资料</h3><section class="captain-dossier" aria-label="船长个人资料"><header><span class="captain-portrait">${captainPortrait(identity.portrait)}</span><div><small>星霜港 · 船长档案</small><h4>${esc(identity.name)}</h4><b>${growth ? `Lv.${growth.level}` : "等级待同步"}</b></div></header><div class="captain-profile-meter">${captainMeter(growth)}</div><p>${captainXPSource}</p><dl>${facts.map(([label,value]) => `<div><dt>${label}</dt><dd>${esc(value)}</dd></div>`).join("")}</dl><nav aria-label="船长快捷入口">${shortcuts.map(([target,label]) => `<button type="button" class="harbor-link" data-captain-dest="${target}">${label}</button>`).join("")}</nav>${!growth || !growth.full && (!growth.cost || growth.xp === null) ? '<footer>船长资料尚未完整同步。<button type="button" class="harbor-link" data-captain-profile-sync>同步船长资料</button></footer>' : ""}</section>`);
  const panel = document.querySelector("#modal-box .captain-dossier"), close = document.querySelector("#modal-box .mb-close");
  panel.profileOpener = opener;
  close.addEventListener("click", () => { if (Store.sessionEpoch === epoch) opener?.focus?.({preventScroll:true}); },{once:true});
  close.focus();
  panel.onclick = event => {
    const button = event.target.closest("button");
    if (!button || !Store.ready() || epoch !== Store.sessionEpoch || panel !== document.querySelector("#modal-mask:not(.hidden) .captain-dossier")) return;
    if (button.hasAttribute("data-captain-profile-sync")) return syncCaptainProfile(button);
    const destination = button.dataset.captainDest;
    closeModal();
    if (destination === "story") return openVoyage("story");
    if (destination === "chests") return openVoyageChests(opener);
    if (["power","collection","stars"].includes(destination)) return openFleetRanking(destination,opener);
    if (["inbox","redeem"].includes(destination)) return openHarborMail(destination);
    if (destination === "journal") nav("page-captain");
  };
}

async function syncCaptainProfile(button) {
  if (button.disabled || !Store.ready()) return;
  const epoch = Store.sessionEpoch, panel = button.closest(".captain-dossier");
  button.disabled = true;
  try {
    const result = await Store.refresh();
    if (!result || Store.sessionEpoch !== epoch || !Store.ready()) return;
    renderTopbar();
    if (panel && panel === document.querySelector("#modal-mask:not(.hidden) .captain-dossier")) openCaptainProfile(panel.profileOpener);
    if (!captainGrowth()) toast("当前服务器尚未提供船长等级与经验资料");
  } finally { if (button.isConnected && epoch === Store.sessionEpoch) button.disabled = false; }
}

document.addEventListener("keydown", event => {
  const panel = document.querySelector("#modal-mask:not(.hidden) .captain-dossier");
  if (!panel || !Store.ready()) return;
  const close = document.querySelector("#modal-box .mb-close"), last = [...panel.querySelectorAll("button:not(:disabled)")].at(-1);
  if (event.key === "Escape") { event.preventDefault(); close.click(); }
  if (event.key === "Tab" && last && (event.shiftKey && document.activeElement === close || !event.shiftKey && document.activeElement === last)) { event.preventDefault(); (event.shiftKey ? last : close).focus(); }
});

function captainJournal() {
  const captain = SAVE?.captain, rules = Store.profile?.captain_rules, log = SAVE?.daily?.captain_log;
  if (!captain || !Array.isArray(rules?.tasks) || !Array.isArray(rules?.supplies) || !log?.supplies || !log.claimed || !log.progress) return null;
  const clock = new Date(Store.serverNow() + 8 * 3600000), day = SAVE.daily.date;
  return { captain, rules, log, day, clock, hour:clock.getUTCHours(), current:day === clock.toISOString().slice(0,10) };
}

function captainSupplyState(journal, supply) {
  if (!journal.current) return "新日待同步";
  if (journal.log.supplies[supply.id]) return "已领取";
  if (journal.hour < supply.start) return "未开餐";
  return journal.hour < supply.end ? "领取体力" : "已收餐";
}

function captainTasksReady(journal) {
  return journal.current ? journal.rules.tasks.filter(task => !journal.log.claimed[task.id] && (journal.log.progress[task.id] || 0) >= task.target).length : 0;
}

function captainPoints(journal) {
  return journal.rules.tasks.reduce((total,task) => total + ((journal.log.progress[task.id] || 0) >= task.target ? task.points || 0 : 0),0);
}

function captainChestReady(journal, chest) {
  return journal.current && !journal.log.chests?.[chest.id] && captainPoints(journal) >= chest.points;
}

function captainTaskDestination(id, log) {
  if (id === "equipment" && log.gear_review || id === "skill" && log.skill_review || id === "outfit" && (log.gear_review || log.skill_review)) return ["page-stage","去实战复盘"];
  return captainDestinations[id];
}

function captainChests(journal) {
  if (!journal.rules.chests?.length) return "";
  const labels = {token:"代币",stamina_pack_30:"30体力礼包",limited_ticket:"限定券",gem_ticket:"宝石券",chip_ticket:"芯片券"};
  const points = captainPoints(journal), maximum = journal.rules.chests.at(-1).points;
  return `<section class="captain-duties" aria-labelledby="captain-chests"><header class="mode-card-head"><div><p class="social-kicker">完成任务即获得积分，不必先领单项奖励。每日北京时间 00:00 重置。</p><h3 id="captain-chests">任务积分宝箱 · ${points}/${maximum}</h3></div></header>${journal.rules.chests.map(chest => {
    const taken = journal.log.chests?.[chest.id], ready = captainChestReady(journal,chest);
    const rewards = Object.entries(chest.resources).map(([id,count]) => `${esc(Store.profile?.items?.items?.[id]?.name || labels[id] || id)} ×${esc(count)}`).join(" · ");
    return `<article class="captain-duty"><div><h4>${esc(chest.points)} 积分宝箱</h4><p>${rewards}</p></div><button type="button" class="gbtn gbtn-sm${ready ? " gbtn-primary" : ""}" data-captain-chest="${esc(chest.id)}" ${ready ? "" : "disabled"}><span>${taken ? "已领取" : ready ? "领取宝箱" : journal.current ? `还差 ${Math.max(0,chest.points-points)} 积分` : "待同步"}</span></button></article>`;
  }).join("")}</section>`;
}

function captainWatchBoundary(journal) {
  clearTimeout(captainBoundaryTimer);
  if (!journal?.current || !["page-captain","page-home"].includes(document.body.dataset.page)) return;
  const next = Math.min(24,...journal.rules.supplies.flatMap(slot => [slot.start,slot.end]).filter(hour => hour > journal.hour));
  const clock = journal.clock, delay = ((next-journal.hour)*60-clock.getUTCMinutes())*60000-clock.getUTCSeconds()*1000-clock.getUTCMilliseconds()+50;
  const epoch = Store.sessionEpoch;
  captainBoundaryTimer = setTimeout(() => {
    if (epoch !== Store.sessionEpoch || !Store.ready()) return;
    if (document.body.dataset.page === "page-captain") renderCaptain();
    if (["page-home","page-captain"].includes(document.body.dataset.page)) renderCaptainSummary();
  },delay);
}

function renderCaptainSummary() {
  const display = document.getElementById("res-captain"), summary = document.getElementById("captain-summary");
  const journal = captainJournal(), growth = captainGrowth();
  if (display) display.textContent = growth ? `Lv.${growth.level}` : "资料";
  renderCaptainMast(growth);
  if (summary) {
    if (!journal) summary.textContent = "今日航务待同步";
    else {
      const supply = journal.rules.supplies.find(slot => captainSupplyState(journal,slot) === "领取体力");
      const ready = captainTasksReady(journal), sent = journal.rules.tasks.filter(task => journal.log.claimed[task.id]).length;
      const chests = journal.rules.chests?.filter(chest => captainChestReady(journal,chest)).length || 0;
      summary.textContent = !journal.current ? "新航日已到 · 同步日志" : `${ready ? `${ready} 项任务可领取` : sent ? `${sent} 项任务奖励已领取` : "今日任务已登记"}${chests ? ` · ${chests} 个宝箱可领` : ""} · ${supply ? `${supply.name}可领` : "查看三餐补给"}`;
    }
  }
  captainWatchBoundary(journal);
}

function renderCaptain() {
  const body = document.getElementById("captain-body");
  if (!body || !Store.ready()) return;
  const journal = captainJournal();
  const syncButton = '<button type="button" class="gbtn gbtn-sm" data-captain-sync><span>同步日志</span></button>';
  body.onclick = captainJournalClick;
  body.dataset.captainDay = journal?.day || "";
  body.dataset.captainEpoch = String(Store.sessionEpoch);
  if (!journal) { body.innerHTML = `<p>当前服务器尚未提供今日任务和时段体力记录。</p>${syncButton}`; return; }
  const {captain,rules,log,day,current} = journal;
  const claimed = rules.tasks.filter(task => log.claimed[task.id]).length;
  body.innerHTML = `<section class="captain-passport"><div><p class="social-kicker">航籍 · ${esc(day)}</p><h3>船长 Lv.${esc(captain.level)}</h3><p>伙伴等级上限 <b>Lv.${esc(captain.level)}</b></p></div><div class="captain-xp">${captainMeter(captainGrowth())}<small>${captainXPSource}</small></div></section>
    <section class="captain-supplies" aria-labelledby="captain-galley"><header class="mode-card-head"><div><p class="social-kicker">北京时间 · 时段内直接领取 · 满体力亦可收取</p><h3 id="captain-galley">航海厨房</h3></div>${syncButton}</header>
    ${!current ? '<p role="status">已进入新的航海日，同步后可查看今日航务。</p>' : ""}<div class="captain-meals">${rules.supplies.map(slot => {
      const status = captainSupplyState(journal,slot), open = status === "领取体力";
      return `<article class="captain-meal" data-supply-state="${open ? "ready" : "closed"}"><small>${String(slot.start).padStart(2,"0")}:00–${String(slot.end).padStart(2,"0")}:00</small><h4>${esc(slot.name)}</h4><p>体力 +${slot.stamina}</p><button type="button" class="gbtn gbtn-sm${open ? " gbtn-primary" : ""}" data-captain-supply="${esc(slot.id)}" ${open ? "" : "disabled"}><span>${status}</span></button></article>`;
    }).join("")}</div></section>${captainChests(journal)}
    <section class="captain-duties" aria-labelledby="captain-orders"><header class="mode-card-head"><div><p class="social-kicker">每日 00:00 换日 · 奖励领取进度 ${claimed}/${rules.tasks.length}</p><h3 id="captain-orders">每日任务</h3></div></header>${rules.tasks.map(task => {
      const progress = log.progress[task.id] || 0, taken = log.claimed[task.id], ready = current && !taken && progress >= task.target;
      const destination = !ready && !taken && current && captainTaskDestination(task.id, log);
      return `<article class="captain-duty"><div><h4>${esc(task.name)} <small>${progress}/${task.target}</small></h4><p>${esc(task.goal)}</p><small>船长经验 +${task.xp} · 金币 +${task.gold}${task.points ? ` · 积分 +${task.points}` : ""}</small></div><button type="button" class="gbtn gbtn-sm${ready ? " gbtn-primary" : ""}" ${destination ? `data-captain-visit="${esc(task.id)}"` : `data-captain-task="${esc(task.id)}"`} ${ready || destination ? "" : "disabled"}><span>${taken ? "已领取" : ready ? "领取奖励" : destination ? destination[1] : current ? "进行中" : "待同步"}</span></button></article>`;
    }).join("")}</section>`;
  captainWatchBoundary(journal);
}

async function captainJournalClick(event) {
  const button = event.target.closest("button");
  if (!button || button.disabled || !Store.ready()) return;
  const epoch = Store.sessionEpoch, journal = captainJournal();
  const body = button.closest("#captain-body");
  if (!body || body.dataset.captainEpoch !== String(epoch)) return;
  const visit = journal && captainTaskDestination(button.dataset.captainVisit,journal.log);
  if (visit) {
    nav(visit[0]);
    if (visit[0] === "page-growth" && button.dataset.captainVisit === "skill" && typeof openWorkshop === "function") {
      const girl = SAVE.squad.find(id => SAVE.girls[id]?.star > 0) || Object.keys(SAVE.girls).find(id => SAVE.girls[id].star > 0);
      if (girl) {
        openWorkshop(girl,"training");
        if (typeof selectTrainingPanel === "function") selectTrainingPanel("skills");
      }
    }
    if (visit[2]) {
      const destination = document.querySelector(visit[2]);
      destination?.scrollIntoView?.({block:"center"});
      if (destination?.matches("button, select")) destination.focus({preventScroll:true});
    }
    return;
  }
  let receipt = null;
  button.disabled = true;
  try {
    if (button.hasAttribute("data-captain-sync")) await Store.refresh();
    else if (journal?.current && body.dataset.captainDay === journal.day) {
      const taskID = button.dataset.captainTask, supplyID = button.dataset.captainSupply;
      const task = journal.rules.tasks.find(rule => rule.id === taskID), supply = journal.rules.supplies.find(rule => rule.id === supplyID);
      const chest = journal.rules.chests?.find(rule => rule.id === button.dataset.captainChest);
      if (chest && captainChestReady(journal,chest)) {
        receipt = await Store.mutate("captain_chest",{item_id:`${journal.day}:${chest.id}`});
      } else if (task && !journal.log.claimed[taskID] && journal.log.progress[taskID] >= task.target) {
        receipt = await Store.mutate("captain_task",{item_id:`${journal.day}:${taskID}`});
      } else if (supply && captainSupplyState(journal,supply) === "领取体力") {
        receipt = await Store.mutate("captain_supply",{item_id:`${journal.day}:${supplyID}`});
      }
    }
  } finally {
    if (epoch === Store.sessionEpoch && Store.ready()) {
      if (document.body.dataset.page === "page-captain") renderCaptain();
      renderTopbar();
      if (receipt) toast((receipt.drops || []).join(" · ") || "奖励已到账");
    }
  }
}

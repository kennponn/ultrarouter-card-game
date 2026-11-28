const DISPATCH_ROOMS = {
  relic:["封存柜","◇","RELIQUARY"], clock:["雾钟校时","◷","CHRONOMETER"], contract:["限期委托","✎","COMMISSIONS"],
  route:["雾路运灯","⌁","WAYFINDER"], mosaic:["星图拼版","✣","STAR ATLAS"], mirror:["镜厅试阵","◈","MIRROR HALL"], cipher:["暗语译台","⌘","CIPHER DESK"]
};
const DISPATCH_GLYPHS = {ash:"✧",moon:"☾",tide:"≋",spring:"♧",fog:"≋",cache:"▣",beacon:"✦",vault:"◇"};
let dispatchPending = null;

function dispatchRoom(view) { return Object.hasOwn(DISPATCH_ROOMS,view.dispatchRoom) ? view.dispatchRoom : "relic"; }
function setDispatchRoom(id) {
  const view = currentWorkshop();
  if (view?.tab !== "dispatch" || !Object.hasOwn(DISPATCH_ROOMS,id)) return;
  view.dispatchRoom = id; paintWorkshop(`[data-dispatch-tab="${id}"]`);
}
function dispatchToday() { return SAVE.daily?.dispatch; }
function dispatchPay(reward) { return Object.entries(reward || {}).map(([id,count]) => `${esc(Store.profile.items?.items?.[id]?.name || id)} ×${count}`).join(" · ") || "无"; }
function dispatchEvidenceBag() {
  return `<div class="ws-bag-head"><h4>调查证物匣</h4><p>结案记录与可消耗数量分别保存；用于仪式、镇定和封存。</p></div><div class="at-shelf">${Object.keys(Store.profile.cases.evidence).map(id => `<article data-bag-evidence="${esc(id)}"><div class="at-object-title">${atelierIcon(id)}<div><small>调查证物</small><h4>${esc(Store.profile.cases.evidence[id])}</h4><strong>持有 ${SAVE.casebook?.evidence?.[id] || 0}</strong></div></div>${atelierItemDetails(id)}</article>`).join("")}</div>`;
}
function dispatchNote(line) { return `<p class="dp-note">${line}</p>`; }
function dispatchButton(action,label,ready,key="") {
  return `<button type="button" class="ws-command" data-desk-action="${action}" data-desk-key="${esc(key)}" onclick="dispatchAction(this.dataset.deskAction,this.dataset.deskKey)" ${ready && dispatchPending?.epoch !== Store.sessionEpoch ? "" : "disabled"}>${label}</button>`;
}
function dispatchPicks(view,field,values,selected,name) {
  return `<label>${name}<select data-desk-input="${field}" onchange="dispatchPick(this.dataset.deskInput,this.value)">${values.map(([id,label]) => `<option value="${esc(id)}" ${id === selected ? "selected" : ""}>${esc(label)}</option>`).join("")}</select></label>`;
}
function dispatchPick(field,value) {
  const view = currentWorkshop(), rules = Store.profile?.dispatch, day = dispatchToday();
  if (view?.tab !== "dispatch" || !day) return;
  if (field === "relic" && rules.relics.some(item => item.id === value)) view.deskRelic = value;
  else if (field === "tactic" && ["ward","spark","rush"].includes(value)) view.deskTactic = value;
  else if (["first","second"].includes(field) && day.mirror.roster.includes(value)) {
    const old = dispatchPair(view,day.mirror), side = field === "first" ? 0 : 1;
    if (old[1-side] === value) old[1-side] = old[side]; old[side] = value; view.deskPair = old;
  } else return;
  paintWorkshop(`[data-desk-input="${field}"]`);
}
function dispatchRelicPage(view,rules) {
  const item = rules.relics.find(entry => entry.id === view.deskRelic) || rules.relics[0], book = SAVE.dispatch, cases = Store.profile.cases;
  const owned = !!book.relics?.[item.id], active = book.active === item.id, unlocked = (SAVE.casebook?.endings?.[item.case]?.[item.ending] || 0) > 0;
  const stock = SAVE.casebook?.evidence || {}, grade = Store.profile.items?.qualities?.[item.quality];
  return `<div class="dp-shelf">${rules.relics.map((entry,index) => `<button type="button" aria-pressed="${entry === item}" data-desk-relic="${entry.id}" onclick="dispatchPick('relic',this.dataset.deskRelic)">${atelierIcon(entry.id,entry.quality,["dial","armor","lens"][index])}<b>${esc(entry.name)}</b><small>${book.active === entry.id ? "领队携行" : book.relics?.[entry.id] ? "已封存" : "未封存"}</small></button>`).join("")}</div>
  <div class="dp-stamp">${esc(grade?.name || item.quality)} · ${esc(item.name)}</div><div class="dp-trade"><p><b>收益</b>${esc(item.gain)}</p><p><b>代价</b>${esc(item.loss)}</p></div>
  ${dispatchNote(`制作资格：${esc(cases?.files?.[item.case]?.name || item.case)} / ${esc(cases?.endings?.[item.ending] || item.ending)} · ${unlocked ? "已有该结局" : "尚未记录"}。`)}
  <div class="dp-counters">${Object.entries(item.cost).map(([key,amount]) => `<span class="dp-evidence">${atelierIcon(key)} ${esc(cases?.evidence?.[key] || key)} <b>${stock[key] || 0} / ${amount}</b></span>`).join("")}</div>
  <div class="ws-actions">${dispatchButton("relic_bind",owned ? "已永久收录" : "交付证物 · 封存",!owned && unlocked && Object.entries(item.cost).every(([key,qty]) => (stock[key] || 0) >= qty),item.id)}${dispatchButton("relic_wear",active ? "携行中" : "交给编队第一位",owned && !active,item.id)}${dispatchButton("relic_store","归柜",!!book.active)}</div>${dispatchNote("账号只带一件。换件免费；开场、普攻与怒技效果只作用于本场领队，不改已保存的对战阵容。")}`;
}
function dispatchClockPage(view,rules,clock) {
  const labels = rules.symbols, draft = view.clockDraft || [], tries = clock.guesses || [], open = !clock.solved && tries.length < rules.clock_tries;
  const hint = ({a,op,b}) => ({before:`${labels[a]}在${labels[b]}之前。`,next:`${labels[a]}紧贴在${labels[b]}之后。`,apart:`${labels[a]}与${labels[b]}之间隔一枚。`,notfirst:`${labels[a]}不在首位。`,notlast:`${labels[a]}不在末位。`})[op];
  return `<div class="dp-puzzle"><ol class="dp-clues">${clock.hints.map(line => `<li>${esc(hint(line))}</li>`).join("")}</ol><div class="dp-dial" aria-label="从首至末的三个位置">${[0,1,2].map(index => `<span><b>${DISPATCH_GLYPHS[draft[index]] || "·"}</b><small>${draft[index] ? esc(labels[draft[index]]) : `第 ${index+1} 位`}</small></span>`).join("")}</div></div>
  <div class="dp-choices">${Object.entries(labels).map(([id,name]) => `<button type="button" data-clock-mark="${id}" onclick="dispatchClockPick(this.dataset.clockMark)" ${open && !draft.includes(id) ? "" : "disabled"}>${DISPATCH_GLYPHS[id]} ${esc(name)}</button>`).join("")}<button type="button" onclick="dispatchClockPick('undo')" ${open && draft.length ? "" : "disabled"}>撤一枚</button></div>
  ${dispatchNote(`每日只有一个正确的排列。已试 ${tries.length}/${rules.clock_tries} 次；相同排列不能重投。`)}${tries.length ? dispatchNote(`已试：${tries.map(order => order.split("-").map(key => esc(labels[key])).join("→")).join("；")}`) : ""}${dispatchButton("clock_align",clock.solved ? "已校时 · 奖励入库" : !open ? "今日齿轮已锁" : "锁定这个次序",open && draft.length === 3 && !tries.includes(draft.join("-")))}${dispatchNote(`成功：${dispatchPay(rules.clock_reward)}`)}`;
}
function dispatchClockPick(id) {
  const view = currentWorkshop(), rules = Store.profile?.dispatch, clock = dispatchToday()?.clock;
  if (view?.tab !== "dispatch" || dispatchRoom(view) !== "clock" || !clock || clock.solved || clock.guesses.length >= rules.clock_tries) return;
  const draft = view.clockDraft ||= []; if (id === "undo") draft.pop(); else if (Object.hasOwn(rules.symbols,id) && !draft.includes(id)) draft.push(id); else return;
  paintWorkshop();
}
function dispatchLineupStatus(clauseID,leaderID,squad = SAVE.squad) {
  if (!squad?.length || squad[0] !== leaderID) return {code:"leader",text:"请将签约角色放在编队第一位。"};
  if (clauseID === "courier" && squad.length > 2) return {code:"size",text:"静默护送最多两人，请移出多余的队员。"};
  if (clauseID === "choir" && new Set(squad.map(id => GIRLS.find(girl => girl.id === id)?.elem).filter(Boolean)).size < 3) return {code:"elements",text:"三色巡灯需要至少三种不同元素。"};
  return {code:"ready",text:"当前编队符合出战条件；是否计数以本次服务器战报为准。"};
}

function dispatchContractPage(view,rules,contract) {
  const heroName = id => GIRLS.find(girl => girl.id === id)?.name || id;
  const sweeps = rules.sweep_reports === true ? "可扫荡的主线由服务器逐场核验，不凭历史三星加次数。" : "当前服务器未提供扫荡委托战报，请手动出战。";
  return `${dispatchNote(`北京零点换约。签约不会更改编队；只有签约后新开、当日结算的合格战报才计数。${sweeps}`)}<div class="dp-clauses">${rules.contracts.map((clause,index) => {
    const mine = clause.id === contract?.id, ready = mine && contract.progress >= clause.target && !contract.claimed;
    let guidance = "";
    if (mine && !ready && !contract.claimed) {
      const status = dispatchLineupStatus(clause.id,contract.leader), rift = clause.id === "breach";
      guidance = `<p data-dispatch-lineup="${status.code}">实际出战：${esc((SAVE.squad || []).map(heroName).join("、") || "未编队")}。${esc(status.text)}</p><div class="dp-choices"><button type="button" onclick="closeModal();nav('page-squad')">调整编队</button><button type="button" onclick="closeModal();openVoyage('${rift ? 'rifts' : 'story'}')">${rift ? "前往元素裂隙" : "前往主线"}</button></div>`;
    }
    return `<article class="${mine ? "is-current" : ""}"><small>契约 0${index+1}</small><h4>${esc(clause.name)}</h4><p>${esc(clause.rule)}</p><strong>${dispatchPay(clause.reward)}</strong>${mine ? `<p>签约领队 ${esc(heroName(contract.leader))} · ${contract.progress}/${clause.target}</p>${guidance}${dispatchButton("dispatch_contract_claim",contract.claimed ? "已领取" : "交付战报领奖",ready)}` : dispatchButton("contract_sign",contract ? "与已签条款互斥" : `以${esc(heroName(view.id))}签下`,!contract,clause.id)}</article>`;
  }).join("")}</div>`;
}
function dispatchRoutePage(rules,route) {
  const lanes = ["left","middle","right"], names = ["左线","中线","右线"], depth = route.track.length;
  return `<div class="dp-counters"><span>灯油 <b>${route.light} / 7</b></span><span>随行货物 <b>${route.cargo}</b></span><span>驿程 <b>${depth} / 4</b></span></div><div class="dp-map" aria-label="四层三线雾路">${route.tiles.map((id,index) => {
    const tile = rules.tiles[id], row = Math.floor(index/3), lane = index%3, past = row < depth && route.track[row] === lane, ready = route.status === "open" && row === depth && (!depth || Math.abs(route.track.at(-1)-lane) <= 1) && route.light >= tile.fuel;
    return `<button type="button" data-route-cell="${index}" data-desk-key="${lanes[lane]}" class="${past ? "is-walked" : ""}" onclick="dispatchAction('route_step',this.dataset.deskKey)" ${ready ? "" : "disabled"}><small>${row+1} · ${names[lane]}</small><b>${DISPATCH_GLYPHS[id]} ${esc(tile.name)}</b><span>耗${tile.fuel} ${tile.light ? `补${tile.light}` : `货+${tile.cargo}`}</span></button>`;
  }).join("")}</div>${dispatchNote("每层只能前进至原线或相邻线。入站先耗油，再补给；前三层油归零会失货，抵达终点即使油尽也安全。可中途返程，每份货兑一份星锻矿，全程另奖三枚行旅徽记。")}${dispatchButton("route_bank",route.status === "open" ? `现在返程 · 保住 ${route.cargo} 货` : ({arrived:"全程送达",lost:"雾中失联",banked:"已带货返程"})[route.status],route.status === "open" && depth>0)}`;
}
const DESK_INKS = {a:["金线","Ⅰ"],b:["青线","Ⅱ"],c:["绯线","Ⅲ"]};
function dispatchMosaicPage(view,rules,board) {
  if (board.cleared >= 3) return `<div class="dp-stamp">三幅星图已收入本周图册</div>${dispatchNote("北京周一再来，新图的星核位置会改变。")}`;
  const draft = view.deskMosaic || Array(9).fill(""), ink = view.deskInk || "a";
  return `<div class="dp-counters"><span>${esc(board.week)}</span><span>本周第 <b>${board.cleared+1}/3</b> 幅</span></div>${dispatchNote("把九格分成三个相连的三格星座（只算上下左右）。每座恰含一个 ✦ 星核；三个星座中，一个必须是直线，另两个必须拐弯。")}
  <div class="dp-choices">${Object.entries(DESK_INKS).map(([key,[name,mark]]) => `<button type="button" data-desk-ink="${key}" aria-pressed="${ink === key}" onclick="dispatchInk(this.dataset.deskInk)">${mark} ${name} ${draft.filter(entry => entry === key).length}/3</button>`).join("")}</div>
  <div class="dp-mosaic">${draft.map((key,index) => `<button type="button" data-mosaic-cell="${index}" data-ink="${key}" aria-label="第${index+1}格 ${board.marks.includes(index) ? "星核" : "普通"} ${DESK_INKS[key]?.[0] || "空白"}" onclick="dispatchInk(${index})"><b>${board.marks.includes(index) ? "✦" : "·"}</b><small>${DESK_INKS[key]?.[1] || "—"}</small></button>`).join("")}</div>${dispatchButton("mosaic_place","递交整幅拼版",Object.keys(DESK_INKS).every(key => draft.filter(entry => entry === key).length === 3))}${dispatchNote(`可反复试摆；同色再点可擦除。每幅首次完成：${dispatchPay(rules.mosaic_reward)}。`)}`;
}
function dispatchInk(entry) {
  const view = currentWorkshop(); if (view?.tab !== "dispatch" || dispatchRoom(view) !== "mosaic" || SAVE.dispatch.mosaic.cleared >= 3) return;
  if (Object.hasOwn(DESK_INKS,entry)) view.deskInk = entry; else if (Number.isInteger(entry) && entry >= 0 && entry < 9) {const cells = view.deskMosaic ||= Array(9).fill(""), ink = view.deskInk || "a"; cells[entry] = cells[entry] === ink ? "" : ink;} else return;
  paintWorkshop();
}
function dispatchPair(view,desk) { return (view.deskPair?.every(id => desk.roster.includes(id)) ? view.deskPair : desk.roster.slice(0,2)).slice(); }
function dispatchMirrorPage(view,rules,desk) {
  const scene = rules.mirrors[desk.scene], pair = dispatchPair(view,desk), options = desk.roster.map(id => {const girl = GIRLS.find(member => member.id === id); return [id,`${girl?.name || id} · ${girl?.skills?.skill?.name || "借卡"}`];}), last = desk.last;
  return `<div class="dp-stamp">${esc(scene.name)} · ${desk.attempts}/3 次</div>${dispatchNote(`${esc(scene.rule)} 每日相同局势、同样的选择会得到相同战记。六张借卡统一三星 Lv.32，不占用你的角色、阵容或体力。`)}
  <div class="dp-selects">${dispatchPicks(view,"first",options,pair[0],"先锋借卡")}${dispatchPicks(view,"second",options,pair[1],"后援借卡")}${dispatchPicks(view,"tactic",[["ward","壁垒 · 先锋获得 25% 生命护盾"],["spark","引燃 · 先锋开场 +65 怒"],["rush","抢攻 · 两位借卡攻击 +15%，开场各耗 12.5% 生命"]],view.deskTactic || "ward","战前配令")}</div>
  ${dispatchButton("mirror_trial",desk.won ? "已达成今日目标" : "执行配令 · 进行试阵",!desk.won && desk.attempts < 3)}${dispatchNote(`首次达标：${dispatchPay(rules.mirror_reward)}。`)}${last ? `<div class="dp-log" data-mirror-log><strong>${last.win ? "战斗胜利" : "战斗失利"} · ${desk.won ? "达标" : "未达标"}</strong><p>${last.rounds} 回合 / ${last.alive} 位生还 / ${last.casts} 次我方施技</p>${(last.notes || []).map(line => `<small>${esc(line)}</small>`).join("")}</div>` : ""}`;
}
function dispatchWorkshopPanel(view) {
  const room = dispatchRoom(view), rules = Store.profile.dispatch, day = dispatchToday(), book = SAVE.dispatch;
  if (!day || !book) return `<section id="dispatch-panel" class="ws-paper">密务厅正在等待服务端同步。</section>`;
  const body = () => ({relic:() => dispatchRelicPage(view,rules),clock:() => dispatchClockPage(view,rules,day.clock),contract:() => dispatchContractPage(view,rules,day.contract),route:() => dispatchRoutePage(rules,day.route),mosaic:() => dispatchMosaicPage(view,rules,book.mosaic),mirror:() => dispatchMirrorPage(view,rules,day.mirror),cipher:() => typeof cipherWorkshopPanel === "function" ? cipherWorkshopPanel(view) : dispatchNote("译台暂未开放。")})[room]();
  return `<section id="dispatch-panel" class="ws-paper dp-desk" data-desk-room="${room}"><header><small>夜班档案室 / ${DISPATCH_ROOMS[room][2]}</small><h4><i>${DISPATCH_ROOMS[room][1]}</i> ${DISPATCH_ROOMS[room][0]}</h4><span>${esc(SAVE.daily.date)} · 北京时</span></header><nav class="dp-nav" aria-label="密务桌位">${Object.entries(DISPATCH_ROOMS).map(([id,[name,mark]]) => `<button type="button" data-dispatch-tab="${id}" aria-pressed="${id === room}" onclick="setDispatchRoom(this.dataset.dispatchTab)"><b aria-hidden="true">${mark}</b>${name}</button>`).join("")}</nav><div class="dp-sheet">${body()}</div></section>`;
}
async function dispatchAction(action,key="") {
  const view = currentWorkshop(), day = dispatchToday(), room = view && dispatchRoom(view), date = SAVE.daily?.date;
  if (view?.tab !== "dispatch" || !day || dispatchPending?.epoch === Store.sessionEpoch) return;
  const button = action === "route_step" ? [...document.querySelectorAll("[data-route-cell]")].find(node => node.dataset.deskKey === key && !node.disabled) : [...document.querySelectorAll("#dispatch-panel [data-desk-action]")].find(node => node.dataset.deskAction === action && node.dataset.deskKey === key && !node.disabled);
  if (!button) return;
  let fields;
  switch (action) {
    case "relic_bind": case "relic_wear": fields = {item_id:key}; break;
    case "relic_store": fields = {}; break;
    case "clock_align": fields = {item_id:date,count:day.clock.guesses.length+1,choice:view.clockDraft.join("-")}; break;
    case "contract_sign": fields = {item_id:date,girl_id:view.id,choice:key}; break;
    case "dispatch_contract_claim": fields = {item_id:date}; break;
    case "route_step": fields = {item_id:date,count:day.route.track.length+1,choice:key}; break;
    case "route_bank": fields = {item_id:date,count:day.route.track.length}; break;
    case "mosaic_place": {const cells = view.deskMosaic.join(""); fields = {item_id:SAVE.dispatch.mosaic.week,count:SAVE.dispatch.mosaic.cleared+1,choice:cells.match(/.{3}/g).join("-")}; break;}
    case "mirror_trial": fields = {item_id:date,count:day.mirror.attempts+1,squad:dispatchPair(view,day.mirror),choice:view.deskTactic || "ward"}; break;
    default: return;
  }
  const token = {epoch:Store.sessionEpoch}; dispatchPending = token;
  try {
    paintWorkshop(); const result = await Store.mutate(action,fields);
    if (result && Store.ready() && token.epoch === Store.sessionEpoch) {if (action === "mosaic_place") delete view.deskMosaic; if (action === "clock_align") delete view.clockDraft; toast((result.drops || []).join(" · ") || "密务记录已更新");}
  } finally {if (dispatchPending === token) dispatchPending = null; if (currentWorkshop() === view && document.querySelector("#modal-mask:not(.hidden) #dispatch-panel")) paintWorkshop(`[data-dispatch-tab="${room}"]`);}
}

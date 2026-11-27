const WORKSHOP_TABS = { overview: "总览", upgrade:"升级", training:"研修", vigil:"夜巡仪轨", cases:"异闻调查", dispatch:"密务厅", weapon: "专武", carry:"携带", potential: "潜能", gems: "宝石", chips: "芯片", bonds: "羁绊", talent: "专精" };
const WORKSHOP_STATS = { ...CULTIVATION_STATS, cr: "暴击", cd: "暴伤" };
const WORKSHOP_COLORS = { edge: "#ab5452", aegis: "#38796a", tempo: "#496d9c" };
const WORKSHOP_SIGNS = { ruby: "◆", jade: "❖", sapphire: "◈", wind: "✥", amethyst: "✦", edge: "⟁", aegis: "⬡", tempo: "ϟ" };
let workshopView = null;
let workshopSquadPending = null;
let artifactPool = { epoch:-1, id:"heroes", drops:[], loot:[], quality:"", returnTo:null };

function workshopRules() { return cultivationRules()?.artifacts; }

function workshopTabName(key) { return key === "weapon" && Store.profile?.equipment ? "装备" : WORKSHOP_TABS[key]; }

function workshopTabKeys() {
  const supported = {upgrade:Store.profile?.training?.experience && typeof upgradeWorkshopSheet === "function",training:Store.profile?.training && typeof trainingSheet === "function",carry:Store.profile?.carry && typeof carryWorkshopPanel === "function",vigil:Store.profile?.vigil && typeof vigilWorkshopPanel === "function",cases:Store.profile?.cases && typeof casesWorkshopPanel === "function"};
  supported.dispatch = Store.profile?.dispatch && typeof dispatchWorkshopPanel === "function";
  return Object.keys(WORKSHOP_TABS).filter(key => !Object.hasOwn(supported,key) || supported[key]);
}

function workshopEffect(effect) {
  return `${WORKSHOP_STATS[effect.stat]} +${effect.value}${["hp", "atk", "def", "cr", "cd"].includes(effect.stat) ? "%" : ""}`;
}

function workshopItems(tab, slot, variants = true, equipped = [], previewQuality = "") {
  const rules = workshopRules();
  const inventory = (tab === "gems" ? SAVE.gem_inventory : SAVE.chip_inventory) || {};
  const templates = tab === "gems" ? Object.entries(rules.gem_sets).map(([id, set]) => ({ id, family:id, set, name:GEMS[id].name, effect:GEMS[id].effect, color:GEMS[id].color })) : Object.entries(rules.chip_sets).flatMap(([family, set]) => (slot ? [slot] : rules.chip_stars.map((_, index) => index + 1)).map(position => {
    const id = `${family}_${position}`;
    return { id, family, set, position, name:`${set.name} · ${position}号`, effect:workshopEffect(set.base), color:WORKSHOP_COLORS[family] };
  }));
  const keys = new Set(variants ? [...Object.keys(inventory).filter(key => inventory[key] > 0), ...equipped] : []);
  const compare = (left,right) => {
    const first = artifactIdentity(left), second = artifactIdentity(right);
    return (first.grade || 0)-(second.grade || 0) || first.rank-second.rank || first.level-second.level;
  };
  return templates.flatMap(template => {
    const standard = artifactIdentity(template.id);
    const ids = !variants && previewQuality && standard?.quality ? [artifactProgressKey({...standard,quality:previewQuality})] : [template.id,...Array.from(keys).filter(key=>key && key!==template.id && artifactIdentity(key)?.base===template.id).sort(compare)];
    return ids.map(id => {
    const refine = refinementItem(tab,id), level = refine?.level || 0;
    if (refine?.quality) {
      const grade = Store.profile.artifact_grades.qualities[refine.grade], unit = refine.spec.stat === "cd" ? "%" : "点";
      const flat = `${WORKSHOP_STATS[refine.spec.stat]} +${refine.value}${unit}${refine.extraHp ? ` · 生命 +${refine.extraHp}点` : ""}`;
      const effect = template.id === "wind" ? flat : `${template.effect} · ${flat}`;
      return {...template,id,base:template.id,quality:grade.id,gradeName:grade.name,level,rank:refine.rank,refine,name:`${grade.name}·${template.name} Lv.${level} / ${refine.rank}阶`,effect,count:inventory[id] || 0};
    }
    const effect = refine ? `${template.effect} · ${WORKSHOP_STATS[refine.spec.stat]} +${refine.value}${refine.spec.stat === "cd" ? "%" : " 点"}` : template.effect;
    return { ...template, id, base:template.id, level, refine, name:`${template.name}${level ? ` +${level}` : ""}`, effect, count:inventory[id] || 0 };
    });
  });
}

function workshopItemIcon(item) {
  const art = typeof atelierIcon === "function" && atelierItem(item.base || item.id) ? atelierIcon(item.base || item.id,item.quality) : `<i class="ws-glyph ${item.position ? "ws-hex" : "ws-gem"}" style="--item:${item.color}" aria-hidden="true">${WORKSHOP_SIGNS[item.family]}${item.position ? `<sub>${item.position}</sub>` : ""}</i>`;
  return item.quality ? `<span class="ws-grade-art" data-artifact-quality="${item.quality}" title="${esc(item.name)}">${art}<b class="ws-grade-seal" aria-hidden="true">${esc(item.gradeName.at(-1))}</b></span>` : art;
}

function openWorkshop(id, tab = "overview", focusSelector) {
  if (!requireGame() || !workshopRules() || !GIRLS.some(girl => girl.id === id) || !SAVE.girls[id]?.star || !workshopTabKeys().includes(tab)) return;
  workshopView = { epoch: Store.sessionEpoch, id, tab, gear:"weapon", potentialMode: "nodes", slot: tab === "potential" ? Math.min((SAVE.girls[id].potential || 0) + 1, cultivationRules().potential_stars.length) : 1, item: "" };
  paintWorkshop(focusSelector);
}

function currentWorkshop() {
  return Store.ready() && workshopRules() && workshopView?.epoch === Store.sessionEpoch && SAVE.girls[workshopView.id]?.star > 0 ? workshopView : null;
}

function selectWorkshopTab(tab) {
  const view = currentWorkshop();
  if (view && Object.hasOwn(WORKSHOP_TABS, tab)) openWorkshop(view.id, tab, `[data-workshop-tab="${tab}"]`);
}

function selectWorkshopSlot(slot) {
  const view = currentWorkshop();
  if (!view) return;
  const limit = { gems: workshopRules().gem_sockets.length, chips: workshopRules().chip_stars.length, potential: cultivationRules().potential_stars.length }[view.tab];
  if (!Number.isInteger(slot) || !limit || slot < 1 || slot > limit) return;
  view.slot = slot; view.item = "";
  paintWorkshop(`[data-workshop-slot="${slot}"]`);
}

function selectWorkshopItem(id) {
  const view = currentWorkshop();
  if (!view || !["gems", "chips"].includes(view.tab)) return;
  const item = workshopItems(view.tab, view.slot, true, SAVE.girls[view.id][view.tab] || []).find(option => option.id === id);
  if (!item) return;
  view.item = id;
  paintWorkshop(`[data-workshop-item="${item.id}"]`);
}

function workshopSlotInfo(view) {
  const own = SAVE.girls[view.id], rules = workshopRules(), girl = GIRLS.find(entry => entry.id === view.id);
  const gems = view.tab === "gems", slots = gems ? own.gems || [] : own.chips || [];
  const open = gems ? view.slot <= (own.sockets || 0) : own.star >= rules.chip_stars[view.slot - 1];
  const locked = !gems && !!((own.chip_locks || 0) & (1 << (view.slot - 1)));
  const items = workshopItems(view.tab, view.slot, true, slots), equipped = items.find(item => item.id === slots[view.slot - 1]);
  const selected = items.find(item => item.id === view.item) || equipped || items.find(item => item.count > 0) || items[0];
  const cost = gems ? rules.gem_sockets[view.slot - 1] : null, mat = MAT_BY_RARITY[girl.rarity];
  const canSocket = gems && view.slot === (own.sockets || 0) + 1 && own.star >= cost.star && SAVE.gold >= cost.gold && (SAVE.mats[mat] || 0) >= cost.mat;
  return { own, slots, open, locked, items, equipped, selected, cost, mat, canSocket };
}

function workshopSlotControls(view, info) {
  const gems = view.tab === "gems", kind = gems ? "gem" : "chip";
  const canEquip = info.open && !info.locked && info.selected.count > 0 && info.equipped?.id !== info.selected.id;
  const options = [{ action: `${kind}_equip`, name: info.equipped ? "替换" : "装入", ready: canEquip }, { action: `${kind}_remove`, name: "卸下", ready: info.open && !info.locked && !!info.equipped }];
  if (gems && !info.open) return [{ action: "gem_socket", name: "开启此孔", ready: info.canSocket }];
  if (!gems) options.push({ action: "chip_lock", name: info.locked ? "解除保护" : "锁定芯片", ready: info.open && !!info.equipped });
  if (info.equipped?.quality) {
    const item = info.equipped.refine, prices = artifactPrices(view.tab,item), ready = costs => info.open && !info.locked && !!costs && Object.entries(costs).every(([material,amount])=>(SAVE.mats[material] || 0)>=amount);
    options.push({action:`${kind}_level`,name:prices.level ? `升级至 Lv.${item.level+1}` : "等级已满",ready:ready(prices.level)}, {action:`${kind}_rank`,name:prices.rank ? `升至 ${item.rank+1} 阶` : "阶位已满",ready:ready(prices.rank) && item.level>=prices.rankLevel});
  } else if (info.equipped?.refine) {
    const rules = Store.profile.equipment.refinement, level = info.equipped.level, material = gems ? rules.gem_mat : rules.chip_mat;
    const available = level < rules.costs.length;
    options.push({ action:`${kind}_refine`, name:available ? `精炼当前装配至 +${level + 1}` : "精炼已满", ready:info.open && !info.locked && available && (SAVE.mats[material] || 0) >= rules.costs[level] && (SAVE.mats.forge_ore || 0) >= rules.ore_costs[level] });
  }
  return options;
}

async function workshopSlotAction(action) {
  const view = currentWorkshop();
  if (!view || !["chips", "gems"].includes(view.tab)) return;
  const info = workshopSlotInfo(view);
  if (!workshopSlotControls(view, info).some(option => option.action === action && option.ready)) return;
  const fields = { girl_id: view.id, count: view.slot };
  if (action.endsWith("_equip")) fields.item_id = info.selected.id;
  if (/_refine$|_level$|_rank$/.test(action)) fields.item_id = info.equipped.id;
  if (action === "chip_lock") fields.enabled = !info.locked;
  const result = await Store.mutate(action, fields);
  if (!result || view.epoch !== Store.sessionEpoch || !Store.ready()) return;
  refreshCultivationViews(view.id);
  toast((result.drops || []).join(" · ") || "装配已确认");
}

function workshopDial(girl, tab, slots, selected) {
  const total = slots.length;
  const points = slots.map((_, index) => {
    const angle = (2 * Math.PI * index / total) - Math.PI / 2;
    return [50 + 36 * Math.cos(angle), 50 + 36 * Math.sin(angle)];
  });
  const line = order => order.map(index => points[index].map(value => value.toFixed(2)).join(",")).join(" ");
  const drawing = tab === "gems" ? '<circle cx="50" cy="50" r="36"/><circle cx="50" cy="50" r="27"/>' : tab === "potential" ? `<polygon points="${line([0,2,4])}"/><polygon points="${line([1,3,5])}"/>` : `<polygon points="${line([0,1,2,3,4,5])}"/><polygon points="24,35 50,20 76,35 76,65 50,80 24,65"/>`;
  return `<div class="ws-dial dial-${tab}"><svg viewBox="0 0 100 100" aria-hidden="true" focusable="false">${drawing}</svg><div class="ws-center"><img src="${girl.img}" alt="${esc(girl.name)}"><b>${esc(girl.name)}</b></div>${slots.map((slot, index) => `<button type="button" class="ws-node ${slot.state} ${selected === index + 1 ? "is-selected" : ""}" style="left:${points[index][0]}%;top:${points[index][1]}%;--item:${slot.color || "#647687"}" data-workshop-slot="${index + 1}" aria-pressed="${selected === index + 1}" aria-label="${esc(slot.label)}" title="${esc(slot.label)}" onclick="selectWorkshopSlot(${index + 1})"><span class="at-node-art">${slot.item ? workshopItemIcon(slot.item) : typeof atelierEmptySocket === "function" ? atelierEmptySocket(tab,slot.state) : slot.symbol}</span><small>${index + 1}</small></button>`).join("")}</div>`;
}

function workshopSetLedger(tab, slots, selected) {
  const rules = workshopRules(), gems = tab === "gems";
  return `<section class="ws-ledger"><h4>套装共鸣 <small>同卡累计</small></h4>${Object.entries(gems ? rules.gem_sets : rules.chip_sets).map(([family, set]) => {
    const count = slots.filter(id => { const base = artifactIdentity(id)?.base; return gems ? base === family : base?.split("_")[0] === family; }).length;
    const next = set.tiers.find(tier => count < tier.count);
    return `<details ${family === selected.family ? "open" : ""}><summary><b>${esc(set.name)}</b> ${count} 件 <small>${next ? `下档 ${next.count} 件` : "全档激活"}</small></summary><div>${set.tiers.map(tier => `<span class="${count >= tier.count ? "is-active" : ""}">${count >= tier.count ? "✓ " : ""}${tier.count} 件 · ${workshopEffect(tier)}</span>`).join("")}</div></details>`;
  }).join("")}</section>`;
}

function workshopRefinementInfo(view, info) {
  const item = info.equipped, rules = Store.profile?.equipment?.refinement;
  if (!item?.refine || !rules) return "";
  if (item.quality) {
    const current=item.refine, grades=Store.profile.artifact_grades, prices=artifactPrices(view.tab,current), next=prices.level && artifactGradeStats({...current,level:current.level+1},current.spec);
    const costs = amounts => Object.entries(amounts).map(([key,cost])=>`${esc(Store.profile.items?.items?.[key]?.name || key)} ${SAVE.mats[key] || 0}/${cost}`).join(" · ");
    const measure = stats => `${WORKSHOP_STATS[current.spec.stat]} +${stats.value}${current.spec.stat === "cd" ? "%" : "点"}${stats.extraHp ? ` · 生命 +${stats.extraHp}点` : ""}`;
    return `<section class="refine-readout ws-grade-readout" data-refinement="${item.id}"><h4>单件培养 <small>${esc(item.gradeName)} · Lv.${current.level}/${grades.level_max} · ${current.rank}/${grades.rank_max}阶</small></h4><dl><dt>等级</dt><dd>${measure(current)}${next ? ` → ${measure(next)}<br>${costs(prices.level)}` : " · 已满级"}</dd><dt>阶位</dt><dd>${prices.rank ? `升至 ${current.rank+1} 阶需 Lv.${prices.rankLevel}${current.level < prices.rankLevel ? "（未达到）" : ""}<br>${costs(prices.rank)}` : "已达最高阶"}</dd></dl>${workshopGradeEffects(current)}<small>同族同名效果只取最强；卸下保留品质、等级和阶位。</small></section>`;
  }
  const {spec, value, level} = item.refine, material = view.tab === "gems" ? rules.gem_mat : rules.chip_mat;
  const label = Store.profile.equipment.materials[material]?.name || material, next = spec.base + Math.floor((level + 1) / spec.every) * spec.step;
  const terms = {skill:"怒气技伤害", shield:"开场生命护盾", rage:"开场怒气", rage_gain:"普攻额外怒气", heal:"治疗与技能护盾", counter:"受击反震（防御比例）", leech:"命中汲取生命"};
  const unit = spec.stat === "cd" ? "%" : " 点";
  return `<section class="refine-readout" data-refinement="${item.id}"><h4>单件精炼 <small>+${level} / ${rules.costs.length}</small></h4><p>基础${WORKSHOP_STATS[spec.stat]} +${spec.base}${unit} · 当前 +${value}${unit}${level < rules.costs.length ? ` → +${next}${unit}` : ""}</p>${level < rules.costs.length ? `<p>${esc(label)} ${SAVE.mats[material] || 0}/${rules.costs[level]} · 星锻矿 ${SAVE.mats.forge_ore || 0}/${rules.ore_costs[level]}</p>` : ""}<ul>${spec.effects.map(effect => `<li class="${level >= effect.level ? 'is-active' : ''}"><b>+${effect.level} ${esc(effect.name)}</b> · ${terms[effect.effect]} +${effect.value}${["rage","rage_gain"].includes(effect.effect) ? "" : "%"}</li>`).join("")}</ul><small>同一角色的同族效果只触发一份；卸下保留精炼等级。</small></section>`;
}

function workshopGradeEffects(item) {
  const terms = {skill:"怒气技伤害",shield:"开场生命护盾",rage:"开场怒气",rage_gain:"普攻额外怒气",heal:"治疗与技能护盾",counter:"受击反震（防御比例）",leech:"命中汲取生命"};
  return `<ul class="ws-grade-effects">${artifactGradeEffects(item).map(effect=>`<li class="${effect.active ? "is-active" : ""}"><b>${esc(effect.gate)} · ${esc(effect.name)}</b> ${terms[effect.effect]} +${effect.value}${["rage","rage_gain"].includes(effect.effect) ? "" : "%"}${effect.gate === "每阶" ? "（每升一阶 +1）" : ""}</li>`).join("")}</ul>`;
}

function workshopEquipment(view, girl) {
  const info = workshopSlotInfo(view), rules = workshopRules(), gems = view.tab === "gems";
  const selected = info.selected, symbol = gems ? "◇" : "⬡", positions = gems ? rules.gem_sockets : rules.chip_stars;
  const nodes = positions.map((spec, index) => {
    const item = workshopItems(view.tab, index + 1, true, info.slots).find(item => item.id === info.slots[index]);
    const open = gems ? (info.own.sockets || 0) > index : info.own.star >= spec;
    const locked = !gems && !!((info.own.chip_locks || 0) & (1 << index));
    return { state: !open ? "is-closed" : item ? "is-filled" : "", item, symbol: item ? WORKSHOP_SIGNS[item.family] : open ? symbol : "·", color: item?.color, label: `${index + 1}号 · ${!open ? "未解锁" : item?.name || "空位"}${locked ? " · 已保护" : ""}` };
  });
  const controls = workshopSlotControls(view, info);
  const bonuses = Object.entries(artifactBonuses(info.own)).filter(([,value]) => value > 0).map(([stat,value]) => workshopEffect({stat,value}));
  return `<div class="ws-layout"><section class="ws-list"><small class="ws-kicker">INVENTORY</small><h4>${gems ? "宝石背包" : "芯片背包"}</h4><p>${gems ? "可选宝石" : `适配 ${view.slot} 号槽`} · 未装配数量</p><div class="ws-inventory">${info.items.map(item => `<button type="button" data-workshop-item="${item.id}" class="${item.id === selected.id ? "is-selected" : ""}" aria-pressed="${item.id === selected.id}" onclick="selectWorkshopItem('${item.id}')">${workshopItemIcon(item)}<span><b>${esc(item.name)}</b><small>${esc(item.effect)}</small></span><em>×${item.count}</em></button>`).join("")}</div><button type="button" class="ws-link" onclick="openArtifactDraw('${view.tab}')">前往${gems ? "宝石" : "芯片"}召唤 ›</button></section>
    <section class="ws-instrument"><div class="ws-instrument-title"><small>${gems ? "ASTRAL SOCKETS" : "CIRCUIT ARRAY"}</small><b>${gems ? "九曜星环" : "六方阵列"}</b></div>${workshopDial(girl, view.tab, nodes, view.slot)}<p>${gems ? `已开孔 ${info.own.sockets || 0}/${positions.length}` : `已装配 ${info.slots.filter(Boolean).length}/${positions.length}`} · ${girl.rarity} · ${info.own.star} 星</p></section>
    <section class="ws-detail"><small class="ws-kicker">SLOT ${String(view.slot).padStart(2,"0")}</small><h4>${gems ? "宝石孔" : "芯片位"} ${view.slot} <small>${info.open ? info.locked ? "已保护" : "已开启" : "未开启"}</small></h4><p>当前：${info.equipped ? `${esc(info.equipped.name)} · ${esc(info.equipped.effect)}` : "未装配"}</p><div class="ws-preview">${workshopItemIcon(selected)}<div><b>${esc(selected.name)}</b><small>单件 ${esc(selected.effect)} · 库存 ${selected.count}</small></div></div>
    ${!info.open ? `<p class="ws-condition">${gems ? `需 ${info.cost.star} 星、金币 ${SAVE.gold}/${info.cost.gold}、${MATS[info.mat].name} ${SAVE.mats[info.mat] || 0}/${info.cost.mat}。${view.slot !== (info.own.sockets || 0) + 1 ? "请先开启前一孔。" : ""}` : `角色 ${rules.chip_stars[view.slot - 1]} 星开启此槽。`}</p>` : info.locked ? '<p class="ws-condition">解除保护后可替换或卸下。</p>' : ""}
    ${workshopRefinementInfo(view,info)}<div class="ws-actions">${controls.map(option => `<button type="button" data-workshop-action="${option.action}" class="ws-command" ${option.ready ? "" : "disabled"} onclick="workshopSlotAction('${option.action}')">${option.name}</button>`).join("")}</div>${typeof atelierItem === "function" && atelierItem(selected.base) ? `<details class="at-specimen-notes"><summary>物品详情</summary>${atelierItemDetails(selected.id)}</details>` : ""}${workshopSetLedger(view.tab, info.slots, selected)}<p>芯片与套装总加成：${bonuses.length ? bonuses.join(" · ") : "暂无"}</p></section></div>`;
}

function workshopPotential(view, girl, own) {
  const rules = cultivationRules(), rank = own.potential || 0, mat = MAT_BY_RARITY[girl.rarity];
  const next = cultivationOptions(girl.id).find(option => option.action === "potential_unlock");
  const nodes = rules.potential_stars.map((star, index) => ({ state: index < rank ? "is-filled" : index === rank && next.ready ? "is-ready" : "is-closed", symbol: index < rank ? "✦" : index === rank ? "◇" : "·", color: "#358478", label: `节点 ${index + 1} · ${star} 星 · ${index < rank ? "已开启" : index === rank && next.ready ? "可开启" : "未开启"}` }));
  const index = view.slot - 1;
  return `<div class="ws-layout"><section class="ws-list"><small class="ws-kicker">潜能脉络</small><h4>${rank}/${nodes.length} 节点</h4><div class="ws-ranks">${rules.potential_stars.map((star, index) => `<button type="button" class="${view.slot === index + 1 ? "is-selected" : ""}" onclick="selectWorkshopSlot(${index + 1})"><b>${String(index + 1).padStart(2, "0")}</b><span>${star} 星</span><small>${index < rank ? "已开启" : index === rank && next.ready ? "可开启" : "未开启"}</small></button>`).join("")}</div></section><section class="ws-instrument"><div class="ws-instrument-title"><small>STELLAR MAP</small><b>六芒潜星</b></div>${workshopDial(girl, "potential", nodes, view.slot)}<p>生命、攻击、防御 +${rank * rules.potential_per_node}%</p></section><section class="ws-detail"><small class="ws-kicker">潜能 ${view.slot}</small><h4>${index < rank ? "节点已点亮" : index === rank ? "下一节点" : "未解锁节点"}</h4><p>生命、攻击、防御各 +${rules.potential_per_node}%</p><dl class="ws-values"><dt>星级需求</dt><dd>${own.star}/${rules.potential_stars[index]} 星</dd><dt>${MATS[mat].name}</dt><dd>${SAVE.mats[mat] || 0}/${rules.potential_mats[index]}</dd></dl><button type="button" class="ws-command" data-cultivation-action="potential_unlock" onclick="cultivateGirl('potential_unlock','${girl.id}')" ${index === rank && next.ready ? "" : "disabled"}>${index < rank ? "已开启" : index === rank ? "开启节点" : "前置节点未开启"}</button></section></div>`;
}

function selectPotentialMode(mode) {
  const view = currentWorkshop();
  if (!view || view.tab !== "potential" || !["nodes", "individual"].includes(mode) || mode === "individual" && !cultivationRules()?.individual) return;
  view.potentialMode = mode;
  paintWorkshop(`[data-potential-mode="${mode}"]`);
}

function workshopIVState(own) {
  const rules = cultivationRules().individual;
  const values = own.iv_values?.length === rules.stats.length ? own.iv_values : null;
  const locked = rules.stats.map((_, index) => !!(values && ((own.iv_locks || 0) & (1 << index))));
  const lockCount = locked.filter(Boolean).length;
  const ore = SAVE.mats.forge_ore || 0, cost = rules.ore_base + lockCount * rules.ore_per_lock;
  return { rules, values, locked, lockCount, ore, cost, canRoll: lockCount < rules.stats.length && ore >= cost };
}

function workshopIVRadar(state) {
  const { rules, values } = state;
  const point = (index, radius) => {
    const angle = index * 2 * Math.PI / rules.stats.length - Math.PI / 2;
    return [100 + Math.cos(angle) * radius, 100 + Math.sin(angle) * radius].map(axis => axis.toFixed(2));
  };
  const polygon = fraction => rules.stats.map((_, index) => point(index, 72 * fraction(index)).join(",")).join(" ");
  return `<svg class="ws-iv-radar" viewBox="0 0 200 200" role="img" aria-label="六维个体值分布">${[.25,.5,.75,1].map(ratio => `<polygon class="iv-guide" points="${polygon(() => ratio)}"/>`).join("")}${rules.stats.map(({stat},index) => {
    const [outerX,outerY] = point(index,72), [labelX,labelY] = point(index,92);
    return `<line x1="100" y1="100" x2="${outerX}" y2="${outerY}"/><text x="${labelX}" y="${labelY}">${WORKSHOP_STATS[stat]}</text>`;
  }).join("")}<polygon data-iv-polygon points="${polygon(index => (values?.[index] || 0) / rules.max_value)}"/>${values ? values.map((value,index) => {
    const [axisX,axisY] = point(index,72 * value / rules.max_value);
    return `<circle cx="${axisX}" cy="${axisY}" r="2.2"/>`;
  }).join("") : ""}</svg>`;
}

function workshopIndividual(own) {
  const state = workshopIVState(own), { rules, values, locked, lockCount } = state;
  const bonuses = individualBonuses(own);
  const freeMax = values?.some((value,index) => value === rules.max_value && !locked[index]);
  return `<div class="ws-iv-layout"><section class="ws-instrument"><div class="ws-instrument-title"><small>个体值</small><b>六维星谱</b></div>${workshopIVRadar(state)}<p>${values ? `个体值合计 ${values.reduce((sum,value) => sum + value,0)}/${rules.stats.length * rules.max_value}` : "尚未洗练"}</p></section><section class="ws-detail"><h4>个体洗练 <small>已锁 ${lockCount}/${rules.stats.length}</small></h4><div class="ws-iv-rows">${rules.stats.map(({stat,step},index) => {
    const name = WORKSHOP_STATS[stat], isMax = values?.[index] === rules.max_value;
    return `<div><b>${name}</b><span>${values ? `${values[index]}/${rules.max_value}` : "暂无"}${isMax ? '<i>满值</i>' : ""}</span><small title="每 ${step} 点个体值提升一次属性">${workshopEffect({stat,value:bonuses[stat]})}</small><button type="button" class="ws-iv-lock" data-iv-lock="${stat}" aria-label="${locked[index] ? "解锁" : "锁定"}${name}" aria-pressed="${locked[index]}" ${values ? "" : "disabled"} onclick="workshopIVAction('iv_lock','${stat}')">${locked[index] ? "已锁定" : "未锁定"}</button></div>`;
  }).join("")}</div><p>仅未锁定属性重掷，数值可能升高或降低。锁定免费，每锁一项再洗需多 ${rules.ore_per_lock} 星锻矿。</p>${freeMax ? '<p class="ws-condition">有满值属性尚未锁定，继续洗练可能降低。</p>' : ""}<div class="ws-iv-roll"><b>星锻矿 ${state.ore}/${state.cost}</b><button type="button" class="ws-command" data-workshop-action="iv_roll" ${state.canRoll ? "" : "disabled"} onclick="workshopIVAction('iv_roll')">${lockCount === rules.stats.length ? "六项已锁定" : values ? "重新洗练" : "首次洗练"}</button></div></section></div>`;
}

async function workshopIVAction(action, stat) {
  const view = currentWorkshop();
  if (!view || view.tab !== "potential" || view.potentialMode !== "individual" || !cultivationRules().individual) return;
  const state = workshopIVState(SAVE.girls[view.id]), fields = { girl_id:view.id };
  if (action === "iv_roll") { if (!state.canRoll || stat !== undefined) return; }
  else if (action === "iv_lock") {
    const index = state.rules.stats.findIndex(entry => entry.stat === stat);
    if (!state.values || index < 0) return;
    fields.choice = stat; fields.enabled = !state.locked[index];
  } else return;
  const result = await Store.mutate(action, fields);
  if (!result || view.epoch !== Store.sessionEpoch || !Store.ready()) return;
  refreshCultivationViews(view.id);
  toast((result.drops || []).join(" · ") || "个体培养已确认");
}

function workshopPortrait(girl, own) {
  const element = ELEM[girl.elem], role = {sdps:"突击",adps:"群攻",support:"辅助",tank:"守护",heal:"治疗"}[girl.role] || girl.role;
  return `<div class="ws-portrait ws-hero-print" ${typeof trainingFrame === "function" ? trainingFrame(own) : ""} style="--hero-accent:${esc(element.color)}"><span class="ws-stamp-art" aria-hidden="true"><svg viewBox="0 0 130 134" focusable="false"><g fill="none" stroke="currentColor"><path d="M65 4 119 37v62l-54 31-54-31V37zM7 65h116M65 9v114"/><circle cx="65" cy="65" r="44"/><circle cx="65" cy="65" r="37" stroke-dasharray="2 6"/></g><path fill="currentColor" d="m37 102 7-17 13-7 2-8-6-7-2-11-6 8 2-17 8-11 12-5 14 6 6 12-1 14-5-7-2 13-6 7 2 7 13 7 6 18-28 12z"/><path fill="none" stroke="#29494d" stroke-width="2" d="m52 49 11-5 15-6m-25 14 15-5m-13 35 10 10 10-12m-11 15 4 14"/></svg></span><span class="ws-hero-art" aria-hidden="true" style="background-image:url('${esc(girl.img)}')"></span><div class="ws-card-caption"><small>${esc(girl.rarity)} · ${esc(element.name)}系 · ${esc(role)}</small><b>${esc(girl.name)}</b><span><span data-workshop-level>Lv.${own.level}</span> · ${own.star} 星</span>${typeof trainingGradeTag === "function" ? trainingGradeTag(own) : ""}</div></div>`;
}

function workshopCardPanel(girl,own) {
  const stats = finalStats(girl.id), maximum = own.level < cardLevelLimit() ? cardLevelCost(own.level) : 0;
  return `<aside class="ws-card-panel" data-workshop-portrait="${girl.id}">${workshopPortrait(girl,own)}<div class="ws-meter"><span>战力</span><b>${powerOf(girl.id).toLocaleString()}</b></div><div class="ws-card-xp"><small data-workshop-exp>${maximum ? `经验 ${(own.exp || 0).toLocaleString()} / ${maximum.toLocaleString()}` : `已达卡级上限 Lv.${cardLevelLimit()}`}</small><progress aria-label="本级经验" max="${maximum || 1}" value="${maximum ? own.exp || 0 : 1}"></progress></div><dl class="ws-card-values">${Object.entries({hp:"生命",atk:"攻击",def:"防御",spd:"速度",cr:"暴击",cd:"暴伤"}).map(([key,label]) => `<div><dt>${label}</dt><dd>${(stats[key] || 0).toLocaleString()}${key === "cr" || key === "cd" ? "%" : ""}</dd></div>`).join("")}</dl></aside>`;
}

function workshopSquadButton(id) {
  const active = SAVE.squad.includes(id), full = !active && SAVE.squad.length >= 4;
  const pending = workshopSquadPending?.epoch === Store.sessionEpoch;
  const label = pending ? "正在确认编队…" : full ? "队伍已满（4/4）" : active ? "移出队伍" : "加入队伍";
  return `<button type="button" class="ws-command" data-workshop-action="squad" onclick="workshopSquadAction()" ${pending || full ? "disabled" : ""}>${label}</button>`;
}

async function workshopSquadAction() {
  const view = currentWorkshop();
  if (!view || view.tab !== "overview" || workshopSquadPending?.epoch === Store.sessionEpoch) return;
  const active = SAVE.squad.includes(view.id);
  if (!active && SAVE.squad.length >= 4) return;
  const squad = active ? SAVE.squad.filter(id => id !== view.id) : [...SAVE.squad, view.id];
  const token = { epoch:Store.sessionEpoch };
  workshopSquadPending = token;
  paintWorkshop();
  let result;
  try { result = await Store.mutate("squad", { squad }); }
  finally {
    if (workshopSquadPending === token) workshopSquadPending = null;
    if (token.epoch === Store.sessionEpoch && Store.ready()) {
      const current = document.querySelector("#modal-mask:not(.hidden) .hero-workshop")?.dataset.cultivationCard;
      if (result) refreshCultivationViews(view.id);
      if (current && (!result || current !== view.id)) paintWorkshop();
    }
  }
}

function workshopOverview(girl, own) {
  const stats = finalStats(girl.id), skill = growthSkill(girl.id), extra = recyclableShards(girl, own);
  const manual = cultivationOptions(girl.id).find(option => option.action === "use_manual");
  return `<div class="ws-folio"><div class="ws-paper"><small class="ws-kicker">角色档案</small><h4>${esc(girl.name)} · ${esc(girl.skills.skill.name)}</h4><p>${esc(girl.desc)}</p><div class="ws-stats"><span>命中<b>${stats.ehr}</b></span><span>抵抗<b>${stats.res}</b></span></div>${starTrainingMarkup(girl.id)}<div class="ws-skills"><p><b>普攻 · ${esc(girl.skills.normal.name)}</b> ${esc(girl.skills.normal.desc)}</p><p><b>怒气 · ${esc(skill.name)}</b> ${esc(skill.desc)}</p></div><div class="cultivation-buttons">${cultivationButton(girl.id, manual)}<button type="button" class="ws-command" onclick="openGrowthGirl('${girl.id}')">经验训练与角色觉醒</button>${workshopSquadButton(girl.id)}${extra ? `<button type="button" class="ws-command" onclick="confirmShardDecompose('${girl.id}',${extra})">分解多余碎片 ×${extra}</button>` : ""}</div></div></div>`;
}

function workshopWeapon(girl, own) {
  const rules = cultivationRules(), weapon = rules.weapons[girl.id];
  const level = own.weapon_level || 0, awake = own.weapon_awake || 0, rune = rules.engravings[own.weapon_rune];
  const options = cultivationOptions(girl.id);
  return `<div class="ws-folio"><section class="ws-paper"><small class="ws-kicker">绑定专武</small><h4>${esc(weapon.name)} +${level}</h4><p>${CULTIVATION_STATS[weapon.stat]} +${level * rules.weapon_per_level}% · 每级 +${rules.weapon_per_level}%</p><div class="ws-gauge"><i style="width:${100 * level / rules.weapon_ore.length}%"></i></div><h4>专武觉醒 ${awake}/${rules.weapon_cores.length}</h4><p>每阶生命 +${rules.weapon_vitality_per_awake}%、初始怒气 +${rules.weapon_rage_per_awake}</p><div class="cultivation-buttons">${options.filter(option => ["weapon_upgrade","weapon_awaken"].includes(option.action)).map(option => cultivationButton(girl.id, option)).join("")}</div><h4>铭刻 · ${rune ? esc(rune.name) : "未铭刻"}</h4><p>专武 +${rules.weapon_rune_level} · 每次 ${rules.weapon_rune_ore} 星锻矿（持有 ${SAVE.mats.forge_ore || 0}） · 重刻替换</p><div class="cultivation-buttons">${options.filter(option => option.action === "weapon_engrave").map(option => cultivationButton(girl.id, option)).join("")}</div></section></div>`;
}

function paintWorkshop(focusSelector) {
  const view = currentWorkshop();
  if (!view) return;
  const girl = GIRLS.find(entry => entry.id === view.id), own = SAVE.girls[view.id];
  let content;
  switch (view.tab) {
    case "upgrade": content = upgradeWorkshopSheet(view,girl,own); break;
    case "vigil": content = vigilWorkshopPanel(view.id,view.vigilChoice); break;
    case "cases": content = casesWorkshopPanel(girl,own); break;
    case "dispatch": content = dispatchWorkshopPanel(view); break;
    case "training": content = trainingSheet(view,girl,own); break;
    case "carry": content = carryWorkshopPanel(view.id,view.carryChoice); break;
    case "chips": case "gems": content = workshopEquipment(view, girl); break;
    case "potential": {
      const iv = cultivationRules().individual, individual = iv && view.potentialMode === "individual";
      content = `${iv ? `<nav class="ws-subtabs" aria-label="潜能培养方式">${Object.entries({nodes:"潜能节点",individual:"个体洗练"}).map(([mode,name]) => `<button type="button" data-potential-mode="${mode}" aria-pressed="${view.potentialMode === mode}" onclick="selectPotentialMode('${mode}')">${name}</button>`).join("")}</nav>` : ""}${individual ? workshopIndividual(own) : workshopPotential(view, girl, own)}`;
      break;
    }
    case "weapon": content = Store.profile.equipment && typeof atelierGearPanel === "function" ? atelierGearPanel(view,girl,own) : workshopWeapon(girl, own); break;
    case "bonds": content = `<div class="ws-folio"><div class="ws-paper">${collectionMarkup(view.id)}</div></div>`; break;
    case "talent": content = `<div class="ws-paper ws-specialty">${awakeningDetailMarkup(view.id, own)}${talentMarkup(view.id, own, MAT_BY_RARITY[girl.rarity])}${sigilEquipmentMarkup(view.id, own)}</div>`; break;
    default: content = workshopOverview(girl, own);
  }
  const previous = document.querySelector("#modal-mask:not(.hidden) .hero-workshop"), scroll = document.querySelector("#modal-box .mb-scroll");
  const offset = previous?.dataset.cultivationCard === view.id && previous.dataset.tab === view.tab ? scroll.scrollTop : 0;
  const active = document.activeElement;
  if (!focusSelector && previous?.contains(active)) {
    if (active.matches(".ws-head select")) focusSelector = ".ws-head select";
    else for (const attribute of ["data-choice","data-workshop-card","data-workshop-filter","data-skill-auto","data-skill-buy","data-skill-up","data-effort-stat","data-training-action","data-training-panel","data-carry-action","data-gear-action","data-atelier-gear","data-workshop-action","data-iv-lock","data-workshop-item","data-workshop-slot","data-potential-mode","data-workshop-tab","data-cultivation-action"]) {
      const value = active.getAttribute(attribute);
      if (value && /^[a-z0-9_@~.!-]+$/i.test(value)) { focusSelector = `[${attribute}="${value}"]`; break; }
    }
  }
  showModal(`<h3>角色工坊</h3><div class="hero-workshop ws-frame" data-workshop-frame data-cultivation-card="${girl.id}" data-tab="${view.tab}"><div class="ws-head ws-switch"><label>切换角色<select onchange="openWorkshop(this.value,'${view.tab}','.ws-head select')">${growthHeroOptions(view.id,true)}</select></label></div>${workshopCardPanel(girl,own)}<section class="ws-work-pane" data-workshop-pane><nav class="ws-tabs" aria-label="培养分类">${workshopTabKeys().map(key => `<button type="button" data-workshop-tab="${key}" aria-pressed="${view.tab === key}" onclick="selectWorkshopTab('${key}')">${workshopTabName(key)}</button>`).join("")}</nav>${content}</section></div>`);
  document.querySelector("#modal-box .mb-scroll").scrollTop = offset;
  const workshop = document.querySelector("#modal-box .hero-workshop");
  const focus = focusSelector ? workshop.querySelector(focusSelector) : null;
  (focus && !focus.disabled ? focus : workshop.querySelector('[data-workshop-tab][aria-pressed="true"]'))?.focus({ preventScroll:true });
}

function workshopEntry(id) {
  const girl = GIRLS.find(entry => entry.id === id), own = SAVE.girls[id];
  return `<section class="ws-entry" id="growth-cultivation"><div><small class="ws-kicker">星霜工坊</small><h3>${esc(girl.name)} · 角色培养</h3><p>${own.star}/${growthStarMax()} 星 · 专武 +${own.weapon_level || 0} · 潜能 ${own.potential || 0}/${cultivationRules().potential_stars.length}</p></div><div class="ws-entry-actions">${workshopTabKeys().map(key => `<button type="button" class="ws-command" onclick="openWorkshop('${id}','${key}')">${workshopTabName(key)}</button>`).join("")}</div></section>`;
}

function drawPoolState() {
  if (artifactPool.epoch !== Store.sessionEpoch || artifactPool.owner !== Store.authUserId) artifactPool = { epoch:Store.sessionEpoch, owner:Store.authUserId, id:"heroes", drops:[], loot:[], quality:"", returnTo:null };
  return artifactPool;
}

function discardArtifactReturn() { drawPoolState().returnTo = null; }

function artifactReturnOrigin() {
  const state = drawPoolState(), origin = state.returnTo;
  if (origin && (origin.tab !== state.id || origin.id && !SAVE?.girls[origin.id]?.star)) state.returnTo = null;
  return state.returnTo;
}

function returnFromArtifactDraw() {
  if (!requireGame() || !workshopRules() || document.body.dataset.page !== "page-gacha") return;
  const origin = artifactReturnOrigin();
  if (!origin) { renderGacha(); return; }
  nav(origin.page);
  if (!origin.id) { selectWorkshopBag(origin.tab); return; }
  workshopView = { epoch:Store.sessionEpoch, id:origin.id, tab:origin.tab, potentialMode:"nodes", slot:origin.slot, item:origin.item };
  paintWorkshop(`[data-workshop-slot="${origin.slot}"]`);
}

function selectDrawPool(pool) {
  if (!requireGame() || !workshopRules() || !["heroes","gems","chips"].includes(pool)) return;
  const state = drawPoolState();
  state.returnTo = null;
  if (state.id !== pool) { state.id = pool; state.drops = []; state.loot = []; state.quality = ""; }
  renderGacha();
}

function openArtifactDraw(pool) {
  if (!requireGame() || !workshopRules() || !["gems","chips"].includes(pool)) return;
  const page = document.body.dataset.page, view = currentWorkshop();
  const visible = document.querySelector("#modal-mask:not(.hidden) .hero-workshop");
  let origin = null;
  if (visible && visible.dataset.cultivationCard === view?.id && visible.dataset.tab === pool && view.tab === pool) {
    origin = { page:page || "page-squad", tab:pool, id:view.id, slot:view.slot, item:workshopSlotInfo(view).selected.id };
  } else if (!visible && page === "page-bag" && workshopBag.tab === pool) origin = { page, tab:pool };
  closeModal(); nav("page-gacha");
  const state = drawPoolState();
  if (state.id !== pool) { state.drops = []; state.loot = []; state.quality = ""; }
  state.id = pool; state.returnTo = origin;
  renderGacha();
}

function selectArtifactPoolQuality(quality) {
  if (!requireGame() || !["gems","chips"].includes(drawPoolState().id) || !Store.profile?.artifact_grades?.qualities.some(option=>option.id===quality)) return;
  drawPoolState().quality = quality;
  renderArtifactPool();
  document.querySelector(`[data-pool-quality="${quality}"]`)?.focus({preventScroll:true});
}

function workshopDrawReceipt(pool, items) {
  const {loot,drops} = drawPoolState();
  if (loot.length && Store.profile?.artifact_grades) {
    const variants = workshopItems(pool,undefined,true,loot.map(entry=>entry.item_id));
    const lines = loot.map(entry=>{
      const item = variants.find(option=>option.id===entry.item_id);
      return item && Number.isSafeInteger(entry.quantity) && entry.quantity>0 ? `<li data-receipt-item="${item.id}">${workshopItemIcon(item)}<span>${esc(item.name)}</span><strong>×${entry.quantity}</strong></li>` : "";
    }).join("");
    if (lines) return `<ul class="ws-grade-loot">${lines}</ul>`;
  }
  if (!drops.length) return "<p>暂无抽取记录</p>";
  const portraits = typeof atelierItem === "function" ? drops.map(drop=>items.find(item=>[item.name,`${item.name}位`].some(name=>String(drop).startsWith(name+" ")))).filter(Boolean).map(workshopItemIcon).join("") : "";
  return `${portraits ? `<div class="at-receipt-art" aria-hidden="true">${portraits}</div>` : ""}<ol>${drops.map(drop=>`<li>${esc(drop)}</li>`).join("")}</ol>`;
}

function renderArtifactPool() {
  const enabled = !!workshopRules(), pool = drawPoolState().id;
  const tabs = document.getElementById("gacha-pools"), body = document.getElementById("artifact-draw");
  if (!tabs || !body) return false;
  const active = enabled && pool !== "heroes";
  tabs.hidden = !enabled; body.hidden = !active;
  for (const control of document.querySelectorAll("#gacha-banner,#gacha-pity,#page-gacha>.gacha-btns,#gacha-result,#gacha-extra")) control.classList.toggle("pool-inactive", active);
  tabs.innerHTML = enabled ? Object.entries({heroes:"角色",gems:"宝石",chips:"芯片"}).map(([key,label]) => `<button type="button" data-draw-pool="${key}" aria-pressed="${pool === key}" onclick="selectDrawPool('${key}')">${label}</button>`).join("") : "";
  if (!active) { body.innerHTML = ""; return false; }
  const gems = pool === "gems", key = gems ? "gem_ticket" : "chip_ticket", ticket = ARTIFACT_TICKETS[key];
  const grades = Store.profile?.artifact_grades, quality = drawPoolState().quality || (gems ? grades?.gem_default : grades?.chip_default);
  const items = workshopItems(pool, undefined, false, [],quality), count = SAVE.mats[key] || 0, ore = SAVE.mats.forge_ore || 0, price = workshopRules().ticket_ore;
  const filters = grades ? `<fieldset class="ws-pool-qualities"><legend>品质概率</legend>${grades.qualities.map(grade=>`<button type="button" data-pool-quality="${grade.id}" aria-pressed="${grade.id===quality}" onclick="selectArtifactPoolQuality('${grade.id}')"><i aria-hidden="true"></i>${esc(grade.name)} <b>${grade.weight/100}%</b></button>`).join("")}</fieldset>` : "";
  const origin = artifactReturnOrigin();
  const ticketArt = typeof atelierItem === "function" && atelierItem(key) ? atelierIcon(key) : "";
  body.innerHTML = `<div class="ws-draw-head"><div><small class="ws-kicker">${gems ? "宝石召唤" : "芯片召唤"}</small><h2>${gems ? "宝石星阵" : "芯片工坊"}</h2><p>${esc(ticket.from)} · 每抽消耗一券</p></div><strong class="at-ticket-count">${ticketArt}<span>${ticket.name} <b>×${count}</b></span></strong></div><div class="ws-draw-layout"><section class="ws-draw-catalog"><h4>召唤池 <small>每${gems ? "种宝石" : "种编号芯片"} 1/${items.length}${grades ? " · 品质独立抽取" : ""}</small></h4>${filters}<div class="ws-artifact-grid">${items.map(item => `<div>${workshopItemIcon(item)}<b>${esc(item.name)}</b><small>${esc(item.effect)}</small>${item.quality && item.refine.grade>=4 ? `<small>${artifactGradeEffects(item.refine).filter(effect=>effect.active).map(effect=>esc(effect.name)).join(" · ")}</small>` : ""}</div>`).join("")}</div></section><section class="ws-draw-controls"><h4>召唤</h4><div class="ws-actions">${[1,10].map(amount => `<button type="button" class="ws-command" data-artifact-draw="${amount}" ${count < amount ? "disabled" : ""} onclick="drawArtifacts(${amount})">${amount === 1 ? "单次" : "十连"}召唤 · ${amount} 券</button>`).join("")}</div><h4>兑换召唤券</h4><p>星锻矿 ${ore}/${price}</p><button type="button" class="ws-command" data-ticket="${key}" ${ore < price ? "disabled" : ""} onclick="convertArtifactTicket()">兑换 1 张${ticket.name}</button><div class="ws-draw-receipt" aria-live="polite"><h4>本次结果</h4>${workshopDrawReceipt(pool,items)}</div></section></div>`;
  if (origin) {
    const label = origin.id ? `${GIRLS.find(girl => girl.id === origin.id).name}装配` : `${WORKSHOP_TABS[origin.tab]}背包`;
    body.querySelector(".ws-draw-head").insertAdjacentHTML("beforeend", `<button type="button" class="ws-command" data-draw-return onclick="returnFromArtifactDraw()">‹ 返回${esc(label)}</button>`);
  }
  return true;
}

async function changeArtifactPool(amount, conversion) {
  if (!requireGame() || !workshopRules()) return;
  const pool = drawPoolState().id, epoch = Store.sessionEpoch;
  if (!["gems","chips"].includes(pool)) return;
  const key = pool === "gems" ? "gem_ticket" : "chip_ticket";
  if (conversion ? (SAVE.mats.forge_ore || 0) < workshopRules().ticket_ore : ![1,10].includes(amount) || (SAVE.mats[key] || 0) < amount) return;
  const result = await Store.mutate(conversion ? "convert_tickets" : pool === "gems" ? "draw_gems" : "draw_chips", conversion ? { item_id:key } : { count:amount });
  if (!result || epoch !== Store.sessionEpoch || !Store.ready()) return;
  if (drawPoolState().id === pool && !conversion) {
    drawPoolState().drops = Array.isArray(result.drops) ? result.drops : [];
    drawPoolState().loot = Array.isArray(result.artifact_loot) ? result.artifact_loot.filter(entry=>entry && typeof entry.item_id === "string") : [];
  }
  if (document.body.dataset.page === "page-gacha") renderGacha();
  renderTopbar();
  if (conversion) toast((result.drops || []).join(" · ") || "兑换已确认");
}

function drawArtifacts(count) { return changeArtifactPool(count, false); }
function convertArtifactTicket() { return changeArtifactPool(1, true); }

const WORKSHOP_BAG_TABS = { materials:"培养材料", gems:"宝石背包", chips:"芯片背包", sigils:"灵印", shards:"角色碎片" };
function workshopBagTabs() { return { ...WORKSHOP_BAG_TABS, ...(Store.profile?.carry ? {carry:"随身奇物"} : {}), ...(Store.profile?.cases && typeof dispatchEvidenceBag === "function" ? {evidence:"调查证物"} : {}) }; }
const WORKSHOP_MAT_GROUPS = { workshop:"工料与手记", stars:"升星与回收", tickets:"召唤券", sigils:"灵印打造" };
let workshopBag = { epoch:-1, tab:"materials", group:"workshop", hero:"" };

function selectWorkshopBag(tab, group) {
  if (!requireGame() || !workshopRules() || !Object.hasOwn(workshopBagTabs(), tab)) return;
  workshopBag.tab = tab;
  if (group && Object.hasOwn(WORKSHOP_MAT_GROUPS, group)) workshopBag.group = group;
  renderBag();
}

function openBagItem(tab, itemId) {
  if (!requireGame() || !workshopRules() || !["gems","chips"].includes(tab)) return;
  const item = workshopItems(tab).find(entry => entry.id === itemId);
  if (!item || !SAVE.girls[workshopBag.hero]?.star) return;
  openWorkshop(workshopBag.hero, tab);
  if (item.position) workshopView.slot = item.position;
  workshopView.item = itemId;
  paintWorkshop();
}

function workshopBagBody() {
  const {tab,group} = workshopBag;
  if (tab === "evidence" && Store.profile?.cases && typeof dispatchEvidenceBag === "function") return dispatchEvidenceBag();
  if (tab === "carry" && Store.profile?.carry) return carryBag();
  if (tab === "gems" || tab === "chips") {
    const items = workshopItems(tab);
    return `<div class="ws-bag-head"><h4>${WORKSHOP_BAG_TABS[tab]} · ${items.reduce((sum,item) => sum + item.count,0)} 件</h4><label>装配角色<select onchange="workshopBag.hero=this.value">${growthHeroOptions(workshopBag.hero,true)}</select></label><button type="button" class="ws-link" onclick="openArtifactDraw('${tab}')">前往召唤 ›</button></div><div class="ws-artifact-grid ws-bag-grid">${items.map(item => `<button type="button" data-bag-item="${item.id}" onclick="openBagItem('${tab}','${item.id}')">${workshopItemIcon(item)}<b>${esc(item.name)}</b><strong>×${item.count}</strong><small>${esc(item.effect)}</small></button>`).join("")}</div>`;
  }
  if (tab === "sigils") return sigilInventoryMarkup();
  if (tab === "shards") return `<h4>专属碎片</h4><div class="shard-grid">${GIRLS.filter(girl => (SAVE.girls[girl.id]?.shards || 0) > 0 || SAVE.girls[girl.id]?.star).map(girl => {
    const own = SAVE.girls[girl.id];
    return `<button type="button" class="shard-cell r-${girl.rarity}" onclick="openGirl('${girl.id}')"><img src="${girl.img}" alt="" loading="lazy"><span class="sn">${own.shards || 0}</span><span class="sname">${esc(girl.name)}</span><span class="sneed">${own.star ? `${own.star} 星` : `解锁需 ${RARITY[girl.rarity].shardToUnlock}`}</span></button>`;
  }).join("")}</div>`;
  const groups = Object.entries(WORKSHOP_MAT_GROUPS).map(([key,label]) => `<button type="button" aria-pressed="${key === group}" onclick="selectWorkshopBag('materials','${key}')">${label}</button>`).join("");
  let rows = "";
  if (typeof atelierMaterialShelf === "function" && Store.profile?.items?.items) rows = atelierMaterialShelf(group) + (group === "workshop" ? bagCultivationMarkup(false) : "");
  else if (group === "workshop") rows = bagCultivationMarkup();
  else {
    const entries = group === "tickets" ? Object.entries(ARTIFACT_TICKETS) : Object.entries(MATS).filter(([key]) => group === "sigils" ? key === "sigil_dust" || key.startsWith("essence_") : key.startsWith("shard_dust") || Object.values(MAT_BY_RARITY).includes(key));
    rows = `<div class="ws-material-list">${entries.map(([key,item]) => `<div><i style="--item:${item.color}" aria-hidden="true">◆</i><b>${esc(item.name)}</b><strong>×${SAVE.mats[key] || 0}</strong><span>${esc(item.use)}${Object.values(MAT_BY_RARITY).includes(key) ? "、角色潜能" : ""}</span><small>来源：${esc(item.from)}</small></div>`).join("")}</div>`;
  }
  return `<div class="ws-subtabs">${groups}</div>${rows}<nav class="ws-bag-links" aria-label="资源入口"><button type="button" class="ws-link" onclick="nav('page-stage')">副本掉落 ›</button><button type="button" class="ws-link" onclick="nav('page-shop')">代币商店 ›</button><button type="button" class="ws-link" onclick="openWarehouse()">站点仓库 ›</button></nav>`;
}

function renderWorkshopBag() {
  if (!workshopRules()) return false;
  if (workshopBag.epoch !== Store.sessionEpoch) workshopBag = { epoch:Store.sessionEpoch, tab:"materials", group:"workshop", hero:"" };
  if (!SAVE.girls[workshopBag.hero]?.star) workshopBag.hero = SAVE.squad[0] || GIRLS.find(girl => SAVE.girls[girl.id]?.star)?.id;
  document.getElementById("bag-body").innerHTML = `<section class="ws-bag"><nav class="ws-tabs" aria-label="背包分类">${Object.entries(workshopBagTabs()).map(([key,label]) => `<button type="button" data-bag-tab="${key}" aria-pressed="${workshopBag.tab === key}" onclick="selectWorkshopBag('${key}')">${label}</button>`).join("")}</nav>${workshopBagBody()}</section>`;
  return true;
}

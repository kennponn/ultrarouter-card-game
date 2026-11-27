const CARRY_EFFECTS = {
  rage: value => `开场怒气 +${value}`, rage_gain: value => `每次普攻额外回怒 ${value}`,
  skill: value => `怒气技伤害倍率 +${value}%`, heal: value => `主动治疗与技能护盾 +${value}%`,
  shield: value => `开场护盾：最大生命的 ${value}%`, counter: value => `受伤存活时反震自身防御的 ${value}%`,
  leech: value => `吸取实际生命伤害的 ${value}%`,
};

let carryPending = null;

function carryName(id) { return atelierItem(id)?.name || id; }
function carryEffect(effect) { return CARRY_EFFECTS[effect.stat](effect.value); }

function carryQuote(girlId, choice) {
  const rules = Store.profile?.carry, own = SAVE.girls[girlId];
  if (!rules || !own?.star) return null;
  const id = Object.hasOwn(rules.items, choice) ? choice : own.carry || Object.keys(rules.items)[0];
  const item = rules.items[id], count = SAVE.carry_inventory?.[id] || 0;
  const recipe = Object.entries(item.recipe).map(([key, cost]) => ({ key, cost, have: SAVE.mats[key] || 0 }));
  const contributors = SAVE.squad.filter(member => rules.items[SAVE.girls[member]?.carry]?.set === item.set);
  const worn = Object.values(SAVE.girls).filter(entry => entry.carry === id).length;
  return { id, item, own, count, recipe, set:rules.sets[item.set], worn, contributors,
    distinct:new Set(contributors.map(member => SAVE.girls[member].carry)).size,
    wearerActive:contributors.includes(girlId), craft:recipe.every(line => line.have >= line.cost) && count + worn < 1e9,
    equip:count > 0 && own.carry !== id, remove:Boolean(own.carry) };
}

function selectCarryObject(id) {
  const view = currentWorkshop();
  if (!view || view.tab !== "carry" || !Object.hasOwn(Store.profile?.carry?.items || {}, id)) return;
  view.carryChoice = id;
  paintWorkshop(`[data-carry-object="${id}"]`);
}

async function carryWorkshopAction(action) {
  const view = currentWorkshop();
  if (view?.tab !== "carry" || carryPending?.epoch === view.epoch) return;
  const quote = carryQuote(view.id, view.carryChoice);
  if (!quote || !["carry_craft", "carry_equip", "carry_remove"].includes(action) || !quote[action.slice(6)]) return;
  const fields = action === "carry_craft" ? { item_id:quote.id } : action === "carry_equip" ? { girl_id:view.id, item_id:quote.id } : { girl_id:view.id };
  const token = { epoch:view.epoch };
  carryPending = token;
  let result;
  try {
    paintWorkshop();
    result = await Store.mutate(action, fields);
  } finally {
    if (carryPending === token) carryPending = null;
    if (Store.ready() && Store.sessionEpoch === token.epoch) {
      if (result) refreshCultivationViews(view.id);
      if (currentWorkshop()?.tab === "carry" && document.querySelector('#modal-mask:not(.hidden) .hero-workshop') && (!result || currentWorkshop().id !== view.id)) paintWorkshop();
    }
  }
  if (result && Store.ready() && Store.sessionEpoch === token.epoch) toast((result.drops || []).join(" · ") || "随身奇物已更新");
}

function carryWorkshopPanel(girlId, choice) {
  const rules = Store.profile.carry, quote = carryQuote(girlId, choice);
  if (!quote) return "";
  const { id, item, own, count, recipe, set, distinct, wearerActive } = quote;
  const grade = Store.profile.items.qualities[atelierItem(id).quality];
  const button = (action, label) => `<button type="button" class="ws-command" data-carry-action="carry_${action}" onclick="carryWorkshopAction('carry_${action}')" ${quote[action] && carryPending?.epoch !== Store.sessionEpoch ? "" : "disabled"}>${label}</button>`;
  return `<div class="carry-folio"><section class="carry-case"><small class="ws-kicker">随行匣 / 06</small><h4>行旅藏具</h4>${Object.entries(rules.sets).map(([family, group]) => `<fieldset><legend>${esc(group.name)}</legend><div>${Object.entries(rules.items).filter(([, piece]) => piece.set === family).map(([key]) => `<button type="button" data-carry-object="${key}" aria-pressed="${key === id}" onclick="selectCarryObject('${key}')">${atelierIcon(key)}<b>${esc(carryName(key))}</b><small>${own.carry === key ? "本卡携带" : `库存 ${SAVE.carry_inventory?.[key] || 0}`}</small></button>`).join("")}</div></fieldset>`).join("")}</section>
    <section class="carry-exhibit" style="--carry-accent:${grade.color}"><small>FIELD KIT · 随身一席</small><div class="carry-astrolabe">${atelierIcon(id)}</div><p class="carry-grade">${esc(grade.name)}</p><h4>${esc(carryName(id))}</h4><p>${esc(set.name)} / ${esc(workshopEffect(item))}</p><strong>${esc(carryEffect(item.effect))}</strong><footer>当前佩戴：${own.carry ? esc(carryName(own.carry)) : "空位"} · 未装备 ${count} 件</footer></section>
    <section class="carry-log"><small class="ws-kicker">配方 / 制作一件</small><div class="carry-recipe">${recipe.map(({key, cost, have}) => `<div class="${have >= cost ? "has-stock" : ""}">${atelierIcon(key)}<b>${esc(carryName(key))}</b><span>${have} / ${cost}</span></div>`).join("")}</div><div class="ws-actions">${button("craft","制作一件")}${button("equip", own.carry ? "替换佩戴" : "佩戴")}${button("remove","卸下当前奇物")}</div><p class="carry-hint">每人一席。替换时旧件退回背包。行旅徽记在主线及裂隙胜利后获得。</p>
    <div class="carry-resonance"><h4>${esc(set.name)} <small>编队 ${distinct} / 3 种</small></h4><p>仅同系奇物的上阵携带者受益；相同奇物不重复计数。</p>${set.tiers.map(tier => `<div data-carry-tier="${tier.count}" class="${wearerActive && distinct >= tier.count ? "is-active" : ""}"><b>${tier.count} 种</b><span>${esc(carryEffect(tier))}</span><small>${wearerActive && distinct >= tier.count ? "本卡生效" : "未生效"}</small></div>`).join("")}</div></section></div>`;
}

function openCarryFromBag(id) {
  if (!requireGame() || !SAVE.girls[workshopBag.hero]?.star || !Object.hasOwn(Store.profile?.carry?.items || {}, id)) return;
  openWorkshop(workshopBag.hero, "carry");
  selectCarryObject(id);
}

function carryBag() {
  return `<div class="ws-bag-head"><h4>行旅藏具 · 未装备库存</h4><label>携带角色<select onchange="workshopBag.hero=this.value">${growthHeroOptions(workshopBag.hero,true)}</select></label></div><div class="ws-artifact-grid ws-bag-grid">${Object.entries(Store.profile.carry.items).map(([id, item]) => `<button type="button" data-bag-item="${id}" onclick="openCarryFromBag('${id}')">${atelierIcon(id)}<b>${esc(carryName(id))}</b><strong>×${SAVE.carry_inventory?.[id] || 0}</strong><small>${esc(carryEffect(item.effect))}</small></button>`).join("")}</div>`;
}

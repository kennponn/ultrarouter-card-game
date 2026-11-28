const GAME_MARKETS = {
  token: ["代币商店", "代币", "通关、任务与排名奖励。"],
  arena: ["竞技场商店", "竞技场币", "参加免费排名挑战；每日 21:00 的排名邮件也含竞技场币、限定券。"],
  dungeon: ["副本商店", "副本币", "完成每日资源航线；三星扫荡与手动通关获得相同副本币。"],
  expedition: ["远征商店", "远征币", "星穹远征每通一层发放，越深层数奖励越多。"],
};
let marketTab = "token", marketTimer = null, marketLoadVersion = 0;

function openGameMarket(shop) {
  if (!Object.hasOwn(GAME_MARKETS, shop) || !requireGame()) return;
  marketTab = shop;
  nav("page-shop");
}
function shopBalance(shop) { return shop === "token" ? SAVE.token : SAVE.mats[shop + "_coin"] || 0; }
function shopItemName(item) {
  return Store.profile?.items?.items?.[item.resource]?.name || { gold:"金币补给", stamina:"即用体力补给", multi:"副本双倍卡" }[item.resource] || item.resource;
}
function shopRemaining(item) { return Math.max(0, item.limit - (SAVE.market?.bought?.[item.id] || 0)); }
function shopBlockReason(item) {
  if (!shopRemaining(item)) return "本期已售罄";
  if (item.resource === "stamina" && SAVE.stamina + item.amount > SAVE.staminaMax) return "体力空间不足";
  return shopBalance(item.shop) < item.price ? GAME_MARKETS[item.shop][1] + "不足" : "";
}
function marketTime(value) {
  return new Date(value).toLocaleString("zh-CN", {timeZone:"Asia/Shanghai", month:"numeric", day:"numeric", hour:"2-digit", minute:"2-digit", hour12:false});
}
function scheduleMarketRestock(page, callback) {
  clearTimeout(marketTimer);
  if (!SAVE.market?.next) return;
  const epoch = Store.sessionEpoch;
  marketTimer = setTimeout(() => {
    if (epoch === Store.sessionEpoch && Store.ready() && document.body.dataset.page === page) callback();
  }, Math.max(1000, Math.min(43201000, SAVE.market.next - Store.serverNow() + 1000)));
}
async function loadGameMarket() {
  const epoch = Store.sessionEpoch, request = ++marketLoadVersion;
  const body = document.getElementById("shop-body");
  body.innerHTML = '<p class="social-empty" role="status">正在读取个人库存…</p>';
  const profile = await Store.refresh();
  if (epoch !== Store.sessionEpoch || request !== marketLoadVersion || document.body.dataset.page !== "page-shop") return;
  if (!profile?.market || !SAVE?.market) {
    body.innerHTML = '<p class="social-empty">未取得商店档案。<button class="social-refresh" onclick="renderShop()">重新同步</button></p>';
    return;
  }
  renderTopbar();
  paintShop();
  scheduleMarketRestock("page-shop", renderShop);
}
function staminaPackShelf() {
  const packs = Store.profile?.market?.packs;
  if (!packs) return "";
  return `<div class="social-wallet"><b>我的体力礼包</b>${Object.entries(packs).filter(([id]) => /^stamina_pack_\d+$/.test(id)).sort((first,second) => first[1]-second[1]).map(([id,amount]) => `<button type="button" class="social-refresh" onclick="openStaminaPack('${id}',${Store.sessionEpoch})" ${(SAVE.mats[id] || 0) > 0 ? "" : "disabled"}>使用 ${amount} 体力礼包（${SAVE.mats[id] || 0}）</button>`).join("")}</div>`;
}
function paintShop() {
  const rules = Store.profile.market, currency = GAME_MARKETS[marketTab][1];
  const items = rules.offers.filter(item => item.shop === marketTab && /^[a-z0-9_]+$/.test(item.id));
  const restock = items.some(item => shopRemaining(item) < item.limit);
  document.getElementById("shop-body").innerHTML = `<nav class="arena-filters" aria-label="补给商店">${Object.entries(GAME_MARKETS).map(([key,[name]]) => `<button type="button" class="social-refresh" aria-pressed="${key === marketTab}" onclick="openGameMarket('${key}')">${name}</button>`).join("")}</nav>
    <div class="social-wallet"><span>我的${currency} <b>${shopBalance(marketTab).toLocaleString()}</b></span><span>下一次免费补货：${esc(marketTime(SAVE.market.next))}</span><button type="button" class="social-refresh" onclick="confirmMarketRefresh('${marketTab}')" ${restock && SAVE.token >= rules.refresh_price ? "" : "disabled"}>${rules.refresh_price} 代币刷新本店</button></div>
    <p class="social-note">${GAME_MARKETS[marketTab][2]} 库存独立，北京时间 09:00、14:00、21:00 免费补满。</p><div class="shop-grid">${items.map(item => {
      const reason = shopBlockReason(item), design = Store.profile.items?.items?.[item.resource];
      const detail = design?.use || { gold:"用于角色培养与招募", stamina:"直接恢复，不能超过自然恢复上限", multi:"主页开启后用于下一次普通副本" }[item.resource] || "培养补给";
      const icon = design && typeof atelierIcon === "function" ? atelierIcon(item.resource) : "◆";
      return `<article class="shop-item shop-${esc(item.resource)}"><span class="shop-mark" aria-hidden="true">${icon}</span><div><h3>${esc(shopItemName(item))} <b>×${item.amount}</b></h3><p>${esc(detail)}</p></div><div class="shop-stock">个人剩余 ${shopRemaining(item)} / ${item.limit}</div><button class="gbtn gbtn-sm gbtn-primary" onclick="confirmShopItem('${item.id}')" ${reason ? "disabled" : ""}><span>${reason || '兑换'}</span><span class="sub">${item.price} ${currency}</span></button></article>`;
    }).join("")}</div>${staminaPackShelf()}`;
}
function confirmShopItem(id) {
  if (!requireGame() || !/^[a-z0-9_]+$/.test(id)) return;
  const item = Store.profile.market?.offers.find(offer => offer.id === id && offer.shop === marketTab);
  if (!item || shopBlockReason(item)) return;
  showModal(`<h3>兑换补给</h3><p>${esc(shopItemName(item))} ×${item.amount} · 个人剩余 ${shopRemaining(item)}</p><p>消耗 <b>${item.price} ${GAME_MARKETS[item.shop][1]}</b>，持有 ${shopBalance(item.shop)}。</p><div class="modal-btns"><button class="gbtn" onclick="closeModal()"><span>取消</span></button><button class="gbtn gbtn-primary" onclick="buyShopItem('${id}',${Store.sessionEpoch})"><span>确认兑换</span></button></div>`);
}
async function buyShopItem(id, epoch = Store.sessionEpoch) {
  if (!requireGame() || epoch !== Store.sessionEpoch) return;
  const result = await Store.mutate("shop", {item_id:id});
  if (!result || epoch !== Store.sessionEpoch) return;
  closeModal(); renderTopbar();
  if (document.body.dataset.page === "page-shop") paintShop();
  toast("兑换成功，补给已到账");
}
function confirmMarketRefresh(shop) {
  if (!requireGame() || !Object.hasOwn(GAME_MARKETS, shop) && shop !== "guild" || !SAVE.market?.slot) return;
  const price = Store.profile.market.refresh_price;
  showModal(`<h3>刷新${GAME_MARKETS[shop]?.[0] || '公会商店'}？</h3><p>消耗 ${price} 代币，只补满你在这家商店的库存；下次免费刷新时间不变。</p><div class="modal-btns"><button class="gbtn" onclick="closeModal()"><span>取消</span></button><button class="gbtn gbtn-primary" onclick="refreshMarketStock('${shop}',${SAVE.market.slot},${Store.sessionEpoch})"><span>支付 ${price} 代币</span></button></div>`);
}
async function refreshMarketStock(shop, slot, epoch) {
  if (!requireGame() || epoch !== Store.sessionEpoch) return;
  const result = await Store.mutate("shop_refresh", {item_id:shop, choice:String(slot)});
  if (!result || epoch !== Store.sessionEpoch) return;
  closeModal(); renderTopbar();
  if (document.body.dataset.page === "page-shop") paintShop();
  if (document.body.dataset.page === "page-fleet") await fleetLoad(true);
  toast((result.drops || []).join(" · ") || "本店个人库存已补满");
}
async function openStaminaPack(id, epoch) {
  if (!requireGame() || epoch !== Store.sessionEpoch || !Store.profile.market?.packs[id] || !(SAVE.mats[id] > 0)) return;
  const result = await Store.mutate("use_stamina_pack", {item_id:id});
  if (!result || epoch !== Store.sessionEpoch) return;
  renderTopbar();
  if (document.body.dataset.page === "page-shop") paintShop();
  if (document.body.dataset.page === "page-bag") renderBag();
  toast((result.drops || []).join(" · "));
}

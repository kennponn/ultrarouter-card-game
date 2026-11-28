let harborMailView = {epoch:-1, tab:"inbox", selected:"", busy:false, note:""};

function harborMailMessages() { return Array.isArray(SAVE?.mailbox?.messages) ? SAVE.mailbox.messages : []; }
function harborMailPending(message) { return !message.claimed && (message.captain_xp > 0 || Object.keys(message.resources || {}).length > 0); }

function renderHarborMailBadge() {
  const count = harborMailMessages().filter(harborMailPending).length;
  document.querySelectorAll("[data-harbor-mail-badge]").forEach(badge => { badge.textContent = count ? String(count) : ""; badge.hidden = !count; });
}

function openHarborMail(tab = "inbox") {
  if (!requireGame()) return;
  closeModal();
  harborMailView = {epoch:Store.sessionEpoch, tab:tab === "redeem" ? "redeem" : "inbox", selected:"", busy:false, note:""};
  nav("page-mail");
}

function harborMailName(id) {
  const known = {gold:"金币",stamina:"体力",token:"代币",multi:"双倍卡",permits:"招募券",marks:"契约印"};
  if (id.startsWith("shard:")) return (GIRLS.find(girl => girl.id === id.slice(6))?.name || "伙伴")+"碎片";
  return known[id] || Store.profile?.items?.items?.[id]?.name || MATS[id]?.name || id;
}

function harborMailAttachments(message) {
  const rewards = Object.entries(message.resources || {}).map(([id, amount]) => {
    const icon = typeof atelierIcon === "function" && typeof atelierItem === "function" && atelierItem(id) ? atelierIcon(id) : '<span class="mail-gift-symbol" aria-hidden="true">✦</span>';
    return `<li>${icon}<span>${esc(harborMailName(id))}<b>×${amount.toLocaleString("zh-CN")}</b></span></li>`;
  });
  if (message.captain_xp) rewards.push(`<li><span class="mail-gift-symbol" aria-hidden="true">★</span><span>船长经验<b>+${message.captain_xp}</b></span></li>`);
  return rewards.length ? `<ul class="mail-attachments" aria-label="邮件附件">${rewards.join("")}</ul>` : '<p class="mail-empty">此邮件没有附件</p>';
}

function harborMailDate(timestamp) {
  return new Intl.DateTimeFormat("zh-CN",{timeZone:"Asia/Shanghai",month:"2-digit",day:"2-digit",hour:"2-digit",minute:"2-digit",hour12:false}).format(new Date(timestamp));
}

async function harborMailAction(action, fields = {}) {
  const view = harborMailView;
  if (!Store.ready() || view.busy || view.epoch !== Store.sessionEpoch) return;
  view.busy = true; view.note = ""; renderHarborMail();
  try {
    const result = action === "sync" ? await Store.refresh() : await Store.mutate(action,fields);
    if (result && Store.ready() && Store.sessionEpoch === view.epoch && harborMailView === view) {
      view.note = action === "sync" ? "邮箱已同步" : action === "mail_read" ? "" : (result.drops || []).join(" · ") || "邮件已整理";
      if (action === "redeem_code") { view.selected = ""; }
    }
  } finally {
    view.busy = false;
    if (Store.ready() && Store.sessionEpoch === view.epoch && harborMailView === view) {
      renderHarborMailBadge(); renderTopbar();
      if (document.body.dataset.page === "page-mail") renderHarborMail();
    }
  }
}

function renderHarborMail() {
  const body = document.getElementById("harbor-mail-body");
  if (!body || !Store.ready()) return;
  if (harborMailView.epoch !== Store.sessionEpoch) harborMailView = {epoch:Store.sessionEpoch,tab:"inbox",selected:"",busy:false,note:""};
  const view = harborMailView, messages = [...harborMailMessages()].reverse(), pending = messages.filter(harborMailPending);
  renderHarborMailBadge();
  const tabs = `<nav class="mail-tabs" aria-label="邮箱分类">${[["inbox","奖励邮件",pending.length],["all","全部邮件",messages.length],["redeem","兑换码",null]].map(([key,name,count]) => `<button type="button" data-mail-tab="${key}" aria-pressed="${view.tab === key}">${name}${count === null ? "" : `<small>${count}</small>`}</button>`).join("")}</nav>`;
  if (view.tab === "redeem") {
    body.innerHTML = `${tabs}<section class="mail-redeem"><div class="mail-seal" aria-hidden="true">✦</div><h3>兑换礼包</h3><p>输入兑换码，礼包奖励立即到账。</p><form data-mail-code><label for="harbor-code">兑换码</label><input id="harbor-code" name="code" maxlength="40" autocomplete="off" autocapitalize="characters" spellcheck="false" placeholder="请输入兑换码" required><button type="submit" class="gbtn gbtn-gold" ${view.busy ? "disabled" : ""}><span>确认兑换</span></button></form><p class="mail-rule">每个账号每种礼包限兑一次；有效期以活动公告为准。兑换不会消耗金币或体力。</p></section>`;
  } else {
    const shown = view.tab === "all" ? messages : pending;
    const selected = shown.find(message => message.id === view.selected) || shown[0];
    view.selected = selected?.id || "";
    const list = shown.map(message => `<button type="button" class="mail-row${message.id === view.selected ? " is-selected" : ""}" data-mail-open="${esc(message.id)}" aria-pressed="${message.id === view.selected}"><span class="mail-envelope" aria-hidden="true">✉${!message.read ? '<i></i>' : ""}</span><span><b>${esc(message.title)}</b><small>${esc(message.sender)} · ${harborMailDate(message.created_at)}</small></span><em>${message.claimed ? "已领取" : harborMailPending(message) ? "有附件" : "公告"}</em></button>`).join("");
    const detail = selected ? `<article class="mail-letter"><header><small>${esc(selected.sender)}</small><h3>${esc(selected.title)}</h3><time>${harborMailDate(selected.created_at)} · 北京时间</time></header><p>${esc(selected.body)}</p>${harborMailAttachments(selected)}<button type="button" class="gbtn gbtn-gold" data-mail-claim="${esc(selected.id)}" ${!harborMailPending(selected) || view.busy ? "disabled" : ""}><span>${selected.claimed ? "附件已领取" : "领取附件"}</span></button></article>` : `<article class="mail-letter mail-empty"><span class="mail-seal" aria-hidden="true">✉</span><h3>${view.tab === "inbox" ? "暂无待领附件" : "暂无邮件"}</h3><p>竞技场每日排名与系统赠礼会自动寄达。主动领取的奖励直接到账。</p><button type="button" class="harbor-link" data-mail-tab="redeem">前往兑换礼包</button></article>`;
    body.innerHTML = `${tabs}${!SAVE.mailbox ? '<p class="mail-rule" role="status">邮箱尚未同步，请同步资料后查看。</p>' : ""}<div class="mail-layout"><div class="mail-list" aria-label="邮件列表">${list || '<p class="mail-empty">没有符合当前分类的邮件</p>'}</div>${detail}</div><footer class="mail-footer"><span>附件保留至领取 · ${pending.length} 封待领</span><button type="button" class="harbor-link" data-mail-all ${!pending.length || view.busy ? "disabled" : ""}>一键领取</button><button type="button" class="harbor-link" data-mail-clear ${view.busy || !messages.some(message => message.read && !harborMailPending(message)) ? "disabled" : ""}>删除已读且已领</button><button type="button" class="harbor-link" data-mail-sync ${view.busy ? "disabled" : ""}>同步邮箱</button></footer>`;
  }
  body.insertAdjacentHTML("beforeend",`<p class="mail-status" role="status">${esc(view.note)}</p>`);
  body.querySelectorAll("[data-mail-tab]").forEach(button => button.onclick = () => { if (!view.busy) {view.tab = button.dataset.mailTab; view.selected = ""; view.note = ""; renderHarborMail();} });
  body.querySelectorAll("[data-mail-open]").forEach(button => button.onclick = () => { if (view.busy) return; view.selected = button.dataset.mailOpen; const message = messages.find(entry => entry.id === view.selected); if (!message.read) harborMailAction("mail_read",{item_id:message.id}); else renderHarborMail(); });
  body.querySelector("[data-mail-claim]")?.addEventListener("click", event => harborMailAction("mail_claim",{item_id:event.currentTarget.dataset.mailClaim}));
  body.querySelector("[data-mail-all]")?.addEventListener("click", () => harborMailAction("mail_all"));
  body.querySelector("[data-mail-clear]")?.addEventListener("click", () => harborMailAction("mail_delete_read"));
  body.querySelector("[data-mail-sync]")?.addEventListener("click", () => harborMailAction("sync"));
  body.querySelector("[data-mail-code]")?.addEventListener("submit", event => {
    event.preventDefault();
    const code = new FormData(event.currentTarget).get("code").trim();
    if (code) harborMailAction("redeem_code",{item_id:code});
  });
}

const festivalDays = ["周一", "周二", "周三", "周四", "周五", "周六", "周日"];
const festivalKinds = { cargo: "装船配载", sound: "海雾定位", relay: "灯阵调配", tide: "潮汐择时" };
let festivalView = null;

function festivalDateCurrent() {
  return SAVE?.daily?.date === new Date(Store.serverNow() + 8 * 3600000).toISOString().slice(0, 10);
}

function festivalPrizes(reward) {
  return Object.entries(reward).map(([id, count]) => `${esc(Store.profile.items?.items?.[id]?.name || id)} ×${count}`).join(" · ");
}

function renderFestivalToday() {
  const target = document.getElementById("home-festival-today"), log = SAVE?.festival, rules = Store.profile?.festival;
  if (target) target.textContent = rules && log?.week === rules.week && festivalDateCurrent() ? rules.events.slice(log.day * 2, log.day * 2 + 2).map(event => event.name).join(" · ") : "本周日历 · 集章奖励";
}

function renderPermanentActivities() {
  const host = document.getElementById("festival-permanent");
  if (!host) return;
  const options = typeof workshopRules === "function" && workshopRules() ? workshopTabKeys() : [];
  const places = [["cases", "异闻调查", "调查局"], ["dispatch", "密务厅", "七张委托桌"], ["vigil", "夜巡仪轨", "守夜人的誓约"]].filter(([key]) => options.includes(key));
  host.innerHTML = places.length ? `<section class="fest-permanent"><h3>常驻去处</h3><div>${places.map(([key, name, note]) => `<button type="button" class="harbor-route" data-desk="${key}"><b>${name}</b><small>${note}</small></button>`).join("")}</div></section>` : "";
  host.onclick = event => {
    const target = event.target.closest("[data-desk]");
    if (!target || !Store.ready() || !places.some(([key]) => key === target.dataset.desk)) return;
    const leader = SAVE.squad.find(id => SAVE.girls[id]?.star) || Object.keys(SAVE.girls).find(id => SAVE.girls[id]?.star);
    if (leader) openWorkshop(leader, target.dataset.desk);
    else nav("page-squad");
  };
}

function festivalCell(index) { return "ABC"[Math.floor(index / 3)] + (index % 3 + 1); }

function festivalFields(event) {
  const number = (name, value, label, type = "radio") => `<label class="fest-pick"><input type="${type}" name="${name}" value="${value}"><span>${label}</span></label>`;
  if (event.kind === "cargo") return `<p>装两箱 · 总重 ≤ ${event.limit} · 总价值 ≥ ${event.goal}</p><div class="fest-picks">${event.labels.map((name, index) => number("cargo", index, `${esc(name)}<small>重 ${event.values[index][0]} · 值 ${event.values[index][1]}</small>`, "checkbox")).join("")}</div>`;
  if (event.kind === "sound") return `<p>回声每次穿过一条横边或竖边算一格。</p><p class="fest-hint">${event.values.map(([cell, distance]) => `${festivalCell(cell)} 测站：${distance} 格`).join("　")}</p><div class="fest-grid">${Array.from({length:9}, (_, index) => number("sound", index, festivalCell(index))).join("")}</div>`;
  if (event.kind === "relay") return `<p>可用电荷 ${event.limit} · 每路一格电荷，同时点亮相邻两站各一格。</p><p class="fest-hint">${event.labels.map((label, index) => `${esc(label)}目标 ${event.values[0][index]}`).join("　")}</p><div class="fest-relays">${event.labels.map((label, index) => `<label>${esc(label)}＋${esc(event.labels[(index + 1) % 3])}<select name="relay">${Array.from({length:event.limit + 1}, (_, value) => `<option>${value}</option>`).join("")}</select></label>`).join("")}</div>`;
  return `<p>选择启航时刻。到站时刻除以周期的余数，须等于该站信号。</p><p class="fest-hint">${event.values.map(([period, signal, offset], index) => `${esc(event.labels[index])}：航时 +${offset}，周期 ${period}，信号 ${signal}`).join("<br>")}</p><div class="fest-picks">${Array.from({length:6}, (_, index) => number("tide", index, `刻 ${index}`)).join("")}</div>`;
}

function festivalDraft(form, event) {
  const selected = [...form.querySelectorAll("input:checked, select")].map(input => Number(input.value));
  if (event.kind === "cargo") {
    const [weight, value] = selected.reduce(([weight, value], index) => [weight + event.values[index][0], value + event.values[index][1]], [0, 0]);
    return {valid:selected.length === 2, text:`已选 ${selected.length}/2 箱 · 重 ${weight} · 值 ${value}`, choice:selected.join("")};
  }
  if (event.kind === "relay") {
    const used = selected.reduce((total, value) => total + value, 0);
    return {valid:used === event.limit, text:`电荷 ${used}/${event.limit} · 当前灯量 ${selected.map((value, index) => value + selected[(index + 2) % 3]).join(" / ")}`, choice:selected.join("")};
  }
  return {valid:selected.length === 1, text:selected.length ? (event.kind === "sound" ? `待派艇：${festivalCell(selected[0])}` : `待启航：刻 ${selected[0]}`) : "尚未选定", choice:selected.join("")};
}

function festivalEventCard(event, log, live) {
  const stamped = log.stamps[event.id], used = log.tries[event.id] || 0;
  const state = stamped ? "已集章" : !live ? "轮值预告" : used >= 3 ? "今日已收场" : `剩余 ${3 - used} 次`;
  return `<article class="fest-event"><header><div><small>${esc(festivalKinds[event.kind])}</small><h3>${esc(event.name)}</h3></div><strong>${state}</strong></header><p>${esc(event.story)}</p><div class="fest-pay">${festivalPrizes(event.reward)}</div>${live && !stamped && used < 3 ? `<form data-fest-id="${esc(event.id)}"><fieldset><legend>${festivalKinds[event.kind]}</legend>${festivalFields(event)}<div class="fest-launch"><output aria-live="polite"></output><button class="harbor-link" type="submit" disabled>提交方案</button></div></fieldset></form>` : ""}</article>`;
}

async function refreshFestival() {
  const epoch = Store.sessionEpoch;
  await Store.refresh();
  if (epoch === Store.sessionEpoch && Store.ready() && document.body.dataset.page === "page-festival") { renderFestival(); renderTopbar(); }
}

function openFestivalToday() {
  if (!requireGame()) return;
  if (festivalView?.epoch === Store.sessionEpoch && festivalView.date === SAVE?.daily?.date && SAVE?.festival) {
    festivalView.day = SAVE.festival.day;
    festivalView.note = "";
  }
  nav("page-festival");
  const control = document.querySelector("#festival-body form[data-fest-id] input:not(:disabled), #festival-body form[data-fest-id] select:not(:disabled)");
  control?.closest("form")?.scrollIntoView?.({block:"center", behavior:"auto"});
  control?.focus({preventScroll:true});
}

async function submitFestival(action, fields, id) {
  const view = festivalView;
  if (!view || view.busy || !Store.ready()) return;
  if (!festivalDateCurrent() || view.date !== SAVE.daily.date) { renderFestival(); return; }
  view.busy = true;
  document.querySelectorAll("#festival-body fieldset, #festival-body [data-album]").forEach(control => {control.disabled = true;});
  try {
    const result = await Store.mutate(action, fields);
    if (result && Store.sessionEpoch === view.epoch && document.body.dataset.page === "page-festival") {
      const stamped = !id || result.save?.festival?.stamps?.[id];
      view.note = stamped ? `已入库：${(result.drops || []).join(" · ")}` : "方案未达标，本次已记录。可以调整方案再试。";
    }
  } finally {
    view.busy = false;
    if (Store.ready() && Store.sessionEpoch === view.epoch && document.body.dataset.page === "page-festival") { renderFestival(); renderTopbar(); }
  }
}

function renderFestival() {
  const body = document.getElementById("festival-body"), log = SAVE?.festival, rules = Store.profile?.festival;
  if (!body) return;
  renderPermanentActivities();
  if (!rules || !log || rules.week !== log.week || !festivalDateCurrent()) {
    body.innerHTML = '<p role="status">活动日历待同步，请查看当前航海日的轮值。</p><button type="button" class="harbor-link" onclick="refreshFestival()">同步日历</button>';
    return;
  }
  if (festivalView?.epoch !== Store.sessionEpoch || festivalView.date !== SAVE.daily.date) festivalView = {epoch:Store.sessionEpoch, date:SAVE.daily.date, day:log.day, busy:false, note:""};
  const view = festivalView, live = view.day === log.day, events = rules.events.slice(view.day * 2, view.day * 2 + 2), stamps = Object.keys(log.stamps).length;
  const album = Object.entries(rules.album).map(([threshold, reward], index) => rewardChestCard({
    title:`${threshold} 枚航章宝箱`, threshold:`已收集 ${stamps} / ${threshold} 枚`, tier:index + 1,
    state:log.claimed[threshold] ? "claimed" : stamps >= Number(threshold) ? "ready" : "locked",
    status:log.claimed[threshold] ? "已领取" : stamps >= Number(threshold) ? "领取宝箱" : `还差 ${Number(threshold) - stamps} 枚`,
    rewards:Object.entries(reward).map(([id,count]) => ({name:Store.profile.items?.items?.[id]?.name || id, count})),
    attributes:{"data-album":threshold}
  })).join("");
  body.innerHTML = `<div class="fest-date"><strong>${esc(log.week)} 航务日历</strong><span>北京时间 · 每日两场</span><button class="harbor-link" type="button" onclick="refreshFestival()">同步</button></div><nav class="fest-week" aria-label="活动星期">${festivalDays.map((label, index) => `<button type="button" data-day="${index}" aria-pressed="${view.day === index}">${label}<small>${index === log.day ? "今日" : index < log.day ? "已结束" : "预告"}</small></button>`).join("")}</nav><div class="fest-events">${events.map(event => festivalEventCard(event, log, live)).join("")}</div><p role="status" class="fest-notice">${esc(view.note)}</p><section class="fest-album"><h3>航务集章册 <small>${stamps} / 14</small></h3><div class="reward-chest-grid" tabindex="0" role="group" aria-label="航章宝箱奖励">${album}</div></section>`;
  body.querySelectorAll("[data-day]").forEach(button => button.onclick = () => { view.day = Number(button.dataset.day); view.note = ""; renderFestival(); body.querySelector(`[data-day="${view.day}"]`).focus(); });
  body.querySelectorAll("[data-album]").forEach(button => button.onclick = () => submitFestival("festival_album", {item_id:log.week, count:Number(button.dataset.album)}));
  body.querySelectorAll("form[data-fest-id]").forEach(form => {
    const event = events.find(entry => entry.id === form.dataset.festId);
    const paint = () => { const draft = festivalDraft(form,event); form.querySelector("output").textContent = draft.text; form.querySelector("button").disabled = !draft.valid; };
    form.onchange = paint;
    form.onsubmit = submit => { submit.preventDefault(); const draft = festivalDraft(form,event); if (draft.valid) submitFestival("festival_play", {item_id:`${view.date}:${event.id}`, count:(log.tries[event.id] || 0) + 1, choice:draft.choice}, event.id); };
    paint();
  });
  if (view.busy) body.querySelectorAll("fieldset, [data-album]").forEach(control => { control.disabled = true; });
}

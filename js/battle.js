/* =====================================================
 * battle.js —— 播放服务器战报；胜负、消耗与掉落不在浏览器计算
 * ===================================================== */

let BAT = null;   // 当前战斗状态
let battleRunning = false;

function sleep(ms) { return new Promise(r => setTimeout(r, ms)); }

/* ---------- 开战入口 ---------- */
async function startBattle(stageId) {
  if (!requireGame() || battleRunning) return;
  const epoch = Store.sessionEpoch;
  battleRunning = true;
  const run = await Store.startRun(stageId);
  if (epoch !== Store.sessionEpoch) return;
  if (!run) {
    battleRunning = false;
    if (document.body.dataset.page === "page-stage") updateStageRunLimits();
    if (Store.ready()) toast(Store.lastError || "服务器未能开启副本，请重新同步");
    return;
  }
  await resumeBattle(run, true);
}

async function resumeBattle(run, starting) {
  if (!requireGame() || (battleRunning && !starting)) return;
  let st = run.arena ? { id: "arena", name: "竞技场 · " + run.arena.name, type: run.arena.ladder ? "排名竞技" : "玩家对战", waves: 1, img: "assets/d06-observatory.webp" } : (typeof voyageStage === "function" && voyageStage(run.stage)) || STAGES.find(x => x.id === run.stage) || GACHA_RUN_MODES[run.stage];
  if (run.guild_battle) st = { id: "guild_raid", name: run.guild_battle.stage_name, type: "公会副本", waves: 1, img: "assets/d06-observatory.webp" };
  if (st && run.scene) {
    st = { ...st, name: run.scene.name, img: run.scene.img, starRound: run.scene.star_round };
  }
  const report = run.result;
  if (!st || !report || !Array.isArray(report.allies) || !Array.isArray(report.waves) ||
      !Array.isArray(report.events) || !Number.isFinite(run.ready_at) ||
      !Number.isFinite(report.duration_ms) || report.duration_ms <= 0 ||
      !validBattleFormation(report.allies, report.version) || !report.waves.length || !report.waves.every(wave => validBattleFormation(wave, report.version))) {
    battleRunning = false;
    return toast("服务器战报不完整，请重新同步");
  }
  st = { ...st, waves: report.waves.length };
  const prepare = (u, index, side) => ({ ...u, side, slot: report.version >= 2 ? u.slot : index, girl: GIRLS.find(g => g.id === u.girlId) });
  const allies = report.allies.map((u, index) => prepare(u, index, "ally"));
  const waves = report.waves.map(wave => wave.map((u, index) => prepare(u, index, "enemy")));
  // HTTP Date has whole-second precision; do not add that rounding to a fresh report.
  const remaining = Math.min(report.duration_ms, Math.max(0, run.ready_at - Store.serverNow()));
  const battle = {
    stage: st, allies, waves, waveIdx: 0, log: [], rounds: 0, runId: run.id,
    report, arena: run.arena || null, guildBattle: run.guild_battle || null, readyAt: run.ready_at, readyDeadline: performance.now() + remaining,
    elapsed: Math.max(0, report.duration_ms - remaining), epoch: Store.sessionEpoch, abort: false,
    paused: false, menuPaused: false, steps: 0, speed: 1, skip: false, eventIndex: -1, turnOrder: [],
  };
  if (BAT) dismissBattleMenu(BAT, false);
  window.BattleMotion?.cancelBattleMotion();
  BAT = battle;
  battleRunning = true;
  try {
    nav("page-battle");
    document.getElementById("scene-bg").style.backgroundImage = "url(" + battleSceneImage(st) + ")";
    renderBattle();
    batLog("进入「" + esc(st.name) + "」，正在播放战报");
    await playServerBattle(battle);
  } catch (e) {
    if (battleCurrent(battle)) {
      if (battle.arena) { finishArenaReplay(battle); return; }
      if (battle.guildBattle) { finishGuildReplay(battle); return; }
      battle.playbackFailed = true;
      renderBattleControls(battle, "战报播放中断，可重新同步或重试结算");
      showBattleRecovery(battle, "战报播放中断，奖励仍由服务器结算，不会丢失或重复发放。");
    }
  } finally {
    window.BattleMotion?.cancelBattleMotion(battle);
    if (!battleCurrent(battle)) dismissBattleMenu(battle, false);
    if (BAT === battle) battleRunning = !battle.settled && !battle.abort;
  }
}

/* ---------- 渲染 ---------- */
const cutOf = src => unitAsset(src).replace(/\.webp$/, "-cut.webp");
function validBattleFormation(units, version = 0) {
  if (!Array.isArray(units) || !units.length || units.length > 6) return false;
  const slots = units.map((unit, index) => version >= 2 ? unit.slot : index);
  return slots.every(slot => Number.isInteger(slot) && slot >= 0 && slot < 6) && new Set(slots).size === slots.length;
}
function unitHTML(u, idx) {
  const hpPct = Math.max(0, u.hp / u.maxHp * 100);
  const face = unitAsset(u.girl?.battleImg || u.battleImg || cutOf(u.girl ? u.girl.img : u.img));
  const element = ELEM[u.elem];
  const slot = u.slot ?? idx;
  return `<div class="unit ${u.side === "enemy" ? "enemy" : ""} ${u.boss ? "boss" : ""} ${u.alive ? "" : "dead"}" id="u-${esc(u.uid)}" data-slot="${slot}" style="--bd:${-(idx * 0.9).toFixed(2)}s;--ed:${(idx * 0.09).toFixed(2)}s" title="${esc(u.name)} · 生命 ${u.hp}/${u.maxHp} · 怒气 ${u.rage}/100">
    <span class="u-slot">${u.boss ? "BOSS · " : ""}${slot < 3 ? "前" : "后"}${slot + 1}</span>
    <div class="u-body"><img class="u-face" src="${face}" alt=""></div>
    <div class="u-name">${esc(u.name)}${element ? `<span class="u-elem" style="--elem:${element.color}" title="${element.name}系">${element.name}</span>` : ""}</div>
    <div class="hp-bar" role="meter" aria-label="${esc(u.name)}生命" aria-valuemin="0" aria-valuemax="${u.maxHp}" aria-valuenow="${u.hp}"><div class="hp-fill" style="width:${hpPct}%"></div></div>
    <div class="mp-bar"><div class="mp-fill" style="width:${u.rage}%"></div></div>
    <div class="u-states" aria-label="状态">${unitStatusMarkup(u)}</div>
  </div>`;
}
function unitStatusMarkup(u) {
  const j = u.joinery;
  const joinery = j ? `${u.girlId === "xs039" ? `<span>亮榫 ${j.peg || 0}/3</span>` : ""}${j.cooldown > 0 ? `<span>冷却 ${j.cooldown}</span>` : ""}${j.pulses?.length ? `<span class="poison-state" title="行动开始时推进，净化可移除">留震 ${j.pulses.length}</span>` : ""}${j.mark ? '<span class="shield-state">榫印</span>' : ""}${j.guard > 0 ? '<span class="shield-state">救助减伤</span>' : ""}${j.field_until > 0 ? '<span>弦场</span>' : ""}` : "";
  return `${u.dotT > 0 ? `<span class="poison-state" title="每次行动受到侵蚀，治疗与吸血减半">侵蚀 ${u.dotT}</span>` : ""}${u.stunT > 0 ? '<span>眩晕</span>' : ""}${u.shield > 0 ? '<span class="shield-state">护盾</span>' : ""}${u.reflectT > 0 ? `<span class="shield-state">盾反${u.reflectPct}%</span>` : ""}${u.atkUp > 0 ? `<span>攻+${u.atkUp}%</span>` : ""}${u.defUp > 0 ? `<span>防+${u.defUp}%</span>` : ""}${u.taunt > 0 ? '<span>嘲讽</span>' : ""}${u.defBreakT > 0 ? '<span>破甲</span>' : ""}${u.slowT > 0 ? '<span>减速</span>' : ""}${u.extra ? '<span>再动</span>' : ""}${joinery}`;
}
function assignUids(list, side) { list.forEach((u, i) => u.uid = side + i); }
function formationHTML(list, side) {
  return Array.from({ length: 6 }, (_, slot) => {
    const index = list.findIndex((u, i) => (u.slot ?? i) === slot);
    const front = slot < 3;
    const column = side === "ally" ? (front ? 2 : 1) : (front ? 1 : 2);
    return `<div class="battle-slot ${index < 0 ? "vacant" : "occupied"}" data-slot="${slot}" style="--slot-row:${slot % 3 + 1};--slot-column:${column}">${index < 0 ? `<span>${front ? "前" : "后"} ${slot + 1}</span>` : unitHTML(list[index], index)}</div>`;
  }).join("");
}
function renderBattleTimeline(battle) {
  const round = document.getElementById("battle-round");
  if (round) round.textContent = battle.rounds || 0;
  const order = document.getElementById("battle-turn-order");
  const completed = battle.actedUids || [];
  const sequence = battle.turnOrder || [];
  const upcoming = sequence.filter(uid => uid !== battle.actor && !completed.includes(uid));
  const displayOrder = [...(battle.actor ? [battle.actor] : []), ...upcoming, ...sequence.filter(uid => uid !== battle.actor && completed.includes(uid))];
  if (order) order.innerHTML = displayOrder.map(uid => {
    const u = battleUnit(battle, uid);
    if (!u) return "";
    const active = battle.actor === uid;
    const face = unitAsset(u.girl?.img || u.img);
    return `<span class="turn-unit ${u.side === "enemy" ? "enemy" : "ally"} ${active ? "current" : ""} ${u.alive ? "" : "fallen"} ${!active && completed.includes(uid) ? "acted" : ""}" title="${esc(u.name)}${active ? " · 行动中" : ""}" aria-label="${esc(u.name)}${active ? " · 行动中" : ""}" ${active ? 'aria-current="step"' : ""}><img src="${esc(face)}" alt=""></span>`;
  }).join("") || '<span class="turn-empty">等待行动</span>';
  const progress = document.getElementById("battle-turn-progress");
  if (progress) progress.textContent = `${Math.max(0, (battle.eventIndex ?? -1) + 1)}/${battle.report.events.length}`;
}

function battleSceneImage(stage) {
  // Stage cards contain decorative frames; battlefields need a wide ground plane.
  return /(?:^|\/)d05-tide\.webp(?:[?#]|$)/.test(stage?.img || "") ? "assets/d05-tide.webp" : "assets/d06-observatory.webp";
}
function renderBattle() {
  window.BattleMotion?.cancelBattleMotion(BAT);
  const scene = document.getElementById("battle-scene");
  document.getElementById("scene-bg").style.backgroundImage = "url(" + battleSceneImage(BAT.stage) + ")";
  const log = document.getElementById("battle-log");
  // Keep cached HTML shells compatible with the independent report panel.
  if (log.parentElement === scene) scene.after(log);
  const cur = BAT.waves[BAT.waveIdx];
  assignUids(BAT.allies, "a");
  assignUids(cur, "e");
  renderBattleHud();
  const boxes = [["side-ally", BAT.allies], ["side-enemy", cur]];
  boxes.forEach(([id, list]) => {
    const box = document.getElementById(id);
    box.classList.toggle("solo-boss", list.length === 1 && Boolean(list[0].boss));
    box.innerHTML = formationHTML(list, id === "side-ally" ? "ally" : "enemy");
    box.querySelectorAll(".unit").forEach(el => el.classList.add("enter"));
  });
  setTimeout(() => {
    boxes.forEach(([id]) =>
      document.getElementById(id).querySelectorAll(".unit").forEach(el => el.classList.remove("enter")));
  }, 850);
  renderBattleTimeline(BAT);
}
function toggleBattleLog() {
  // Retained for older callers; reports no longer occupy the battlefield.
  document.getElementById("battle-log-toggle")?.setAttribute("aria-expanded", "false");
}
function updateUnit(u) {
  const el = document.getElementById("u-" + u.uid);
  if (!el) return;
  el.querySelector(".hp-fill").style.width = Math.max(0, u.hp / u.maxHp * 100) + "%";
  el.querySelector(".hp-bar").setAttribute("aria-valuenow", u.hp);
  el.title = `${u.name} · 生命 ${u.hp}/${u.maxHp} · 怒气 ${u.rage}/100`;
  el.querySelector(".mp-fill").style.width = u.rage + "%";
  el.classList.toggle("dead", !u.alive);
  el.classList.toggle("has-shield", (u.shield || 0) > 0);
  const states = el.querySelector(".u-states");
  if (states) states.innerHTML = unitStatusMarkup(u);
}
function batLog(msg) {
  BAT.log.push(msg);
  const el = document.querySelector("#battle-log .bl-lines");
  el.innerHTML = BAT.log.slice(-40).map(m => "<p>" + m + "</p>").join("");
  el.scrollTop = el.scrollHeight;
}
/* 顶部战斗 HUD：关卡名、类型、波次点 */
function renderBattleHud() {
  const st = BAT.stage;
  document.getElementById("bh-name").textContent = st.name;
  document.getElementById("bh-type").textContent = st.type;
  const box = document.getElementById("bh-waves");
  box.innerHTML = Array.from({ length: st.waves }, (_, i) =>
    `<i class="${i <= BAT.waveIdx ? "on" : ""}"></i>`).join("");
  renderBattleControls(BAT, "正在播放服务器战报");
}
function renderBattleControls(battle, message) {
  renderPlaybackControls(battle);
  const button = document.getElementById("btn-flee");
  let status = document.getElementById("bh-status");
  // An older cached HTML shell can load this script during a rolling release.
  if (!status && button) {
    status = document.createElement("span");
    status.id = "bh-status";
    status.setAttribute("role", "status");
    status.setAttribute("aria-live", "polite");
    button.before(status);
  }
  if (status) status.textContent = message;
  if (!button) return;
  button.onclick = battleControl;
  button.disabled = Boolean(battle.settling);
  if (battle.arena) {
    button.textContent = battle.settled ? "返回竞技场" : "跳过战报";
    return;
  }
  if (battle.guildBattle) {
    button.textContent = battle.settled ? "返回公会" : "跳过战斗";
    return;
  }
  if (battle.settled) button.textContent = battle.stage.progression ? "返回成长" : "返回副本";
  else if (battle.playbackFailed) button.textContent = "同步战报";
  else if (battle.playbackComplete) button.textContent = "重新同步";
  else button.textContent = "撤退";
}
function floatText(u, text, cls) {
  const el = document.getElementById("u-" + u.uid);
  if (!el) return;
  const t = document.createElement("div");
  t.className = "float-text " + cls;
  t.textContent = text;
  el.appendChild(t);
  setTimeout(() => t.remove(), 900);
}

/* ---------- 战斗特效 ---------- */
function unitPoint(u) {
  const el = document.getElementById("u-" + u.uid);
  if (!el) return null;
  const s = document.getElementById("battle-scene").getBoundingClientRect();
  const r = el.getBoundingClientRect();
  return { x: r.left - s.left + r.width / 2, y: r.top - s.top + r.height * 0.35 };
}
function spawnFx(x, y, cls, color) {
  const d = document.createElement("div");
  d.className = "fx " + cls;
  if (color) d.style.setProperty("--fx-c", color);
  d.style.left = x + "px"; d.style.top = y + "px";
  document.getElementById("fx-layer").appendChild(d);
  setTimeout(() => d.remove(), 750);
}
function hitAnim(u) {
  const el = document.getElementById("u-" + u.uid);
  if (!el) return;
  const dir = u.side === "ally" ? "hit-r" : "hit-l";   // 沿受击方向击退
  el.classList.remove("hit-r", "hit-l"); void el.offsetWidth;
  el.classList.add(dir);
  setTimeout(() => el.classList.remove(dir), 380);
}
function lungeAnim(u) {
  const el = document.getElementById("u-" + u.uid);
  if (!el) return;
  el.classList.remove("attacking"); void el.offsetWidth; el.classList.add("attacking");
  ghostTrail(u);   // 冲刺残影
  const p = unitPoint(u);
  if (p) spawnFx(p.x + (u.side === "ally" ? -22 : 22), p.y, "fx-dash");
  setTimeout(() => el.classList.remove("attacking"), 470);
}
function leanAnim(u) {   // 远程：拉弓后仰
  const el = document.getElementById("u-" + u.uid);
  if (!el) return;
  el.classList.remove("lean"); void el.offsetWidth; el.classList.add("lean");
  setTimeout(() => el.classList.remove("lean"), 400);
}
function castAnim(u) {   // 法术：浮空施法
  const el = document.getElementById("u-" + u.uid);
  if (!el) return;
  el.classList.remove("casting"); void el.offsetWidth; el.classList.add("casting");
  setTimeout(() => el.classList.remove("casting"), 640);
}
function jumpAnim(u) {   // 重击：跳起下砸
  const el = document.getElementById("u-" + u.uid);
  if (!el) return;
  el.classList.remove("attacking-jump"); void el.offsetWidth; el.classList.add("attacking-jump");
  setTimeout(() => el.classList.remove("attacking-jump"), 620);
}
function ghostTrail(u) { // 冲刺残影：复制立绘渐隐
  const el = document.getElementById("u-" + u.uid);
  const face = el && el.querySelector(".u-face");
  if (!el || !face) return;
  const s = document.getElementById("battle-scene").getBoundingClientRect();
  const r = face.getBoundingClientRect();
  const x = r.left - s.left + r.width / 2, y = r.top - s.top + r.height / 2;
  const dir = u.side === "ally" ? 1 : -1;   // 攻击方向
  for (let i = 1; i <= 2; i++) {
    const g = document.createElement("div");
    g.className = "fx fx-ghost";
    g.style.setProperty("--g-img", 'url("' + face.src + '")');
    g.style.setProperty("--g-dx", (-dir * 18 * i) + "px");
    g.style.left = x + "px"; g.style.top = y + "px";
    g.style.width = r.width + "px"; g.style.height = r.height + "px";
    g.style.animationDelay = (i * 45) + "ms";
    document.getElementById("fx-layer").appendChild(g);
    setTimeout(() => g.remove(), 650);
  }
}
function projectileAnim(src, dst, color) {  // 远程弹道
  return new Promise(res => {
    const sp = unitPoint(src), dp = unitPoint(dst);
    if (!sp || !dp) return res();
    const orb = document.createElement("div");
    orb.className = "fx fx-orb";
    if (color) orb.style.setProperty("--fx-c", color);
    orb.style.left = sp.x + "px"; orb.style.top = sp.y + "px";
    document.getElementById("fx-layer").appendChild(orb);
    requestAnimationFrame(() => requestAnimationFrame(() => {
      orb.style.transform = "translate(" + (dp.x - sp.x) + "px," + (dp.y - sp.y) + "px) translate(-50%,-50%)";
    }));
    setTimeout(() => {
      orb.remove();
      spawnFx(dp.x, dp.y, "fx-burst", color);
      res();
    }, 210);
  });
}
function shieldAnim(u) {
  const el = document.getElementById("u-" + u.uid);
  if (!el) return;
  el.classList.remove("shielded"); void el.offsetWidth; el.classList.add("shielded");
  setTimeout(() => el.classList.remove("shielded"), 550);
}
function skillBanner(name) {
  const b = document.getElementById("skill-banner");
  b.innerHTML = '<div class="sk-name">' + esc(name) + "</div>";
  b.classList.remove("hidden"); void b.offsetWidth;
  clearTimeout(b._t);
  b._t = setTimeout(() => b.classList.add("hidden"), 750);
}
function sceneShake() {
  const s = document.getElementById("battle-scene");
  s.classList.remove("shake"); void s.offsetWidth; s.classList.add("shake");
  setTimeout(() => s.classList.remove("shake"), 420);
}
function flashScene() {
  const f = document.getElementById("scene-flash");
  f.classList.remove("hidden", "go"); void f.offsetWidth;
  f.classList.add("go");
  setTimeout(() => f.classList.add("hidden"), 520);
}
function waveTitle(text) {
  const w = document.getElementById("wave-title");
  w.textContent = text;
  w.classList.remove("hidden", "go"); void w.offsetWidth;
  w.classList.add("go");
  setTimeout(() => w.classList.add("hidden"), 1150);
}
/* 一次攻击的完整演出：按攻击类型播放动作 -> 命中特效（受击反馈来自服务器战报） */
function atkTypeOf(u) {
  if (u.girl) return u.girl.atkType || "melee";
  return u.atkType || "melee";
}
async function playStrike(src, dst, color, battle = BAT) {
  const t = atkTypeOf(src);
  if (t === "magic") {
    castAnim(src);
    await battleDelay(battle, 260);
    const p = unitPoint(dst);
    if (p) spawnFx(p.x, p.y, "fx-burst", color);
  } else if (t === "ranged") {
    leanAnim(src);
    await battleDelay(battle, 140);
    await projectileAnim(src, dst, color);
  } else if (t === "heavy") {
    jumpAnim(src);
    await battleDelay(battle, 340);
    const p = unitPoint(dst);
    if (p) spawnFx(p.x, p.y + 14, "fx-burst", color);
  } else {                                // 近战：蓄力突进斩
    lungeAnim(src);
    await battleDelay(battle, 190);
    const p = unitPoint(dst);
    if (p) spawnFx(p.x, p.y, "fx-slash", color);
  }
  await battleDelay(battle, 70);
}

/* ---------- 服务器战报播放 ---------- */
function renderPlaybackControls(battle) {
  const ended = Boolean(battle.playbackComplete || battle.settled || battle.playbackFailed || battle.skip);
  const pause = document.getElementById("battle-pause");
  if (pause) { pause.textContent = battle.paused ? "继续" : "暂停"; pause.disabled = ended; pause.setAttribute("aria-pressed", String(Boolean(battle.paused))); }
  for (const id of ["battle-step", "battle-skip"]) {
    const button = document.getElementById(id);
    if (button) button.disabled = ended;
  }
  document.querySelectorAll("[data-battle-speed]").forEach(button => {
    button.setAttribute("aria-pressed", String(Number(button.dataset.battleSpeed) === (battle.speed || 1)));
    button.disabled = ended;
  });
  const cycle = document.getElementById("battle-speed-cycle");
  if (cycle) {
    const speed = battle.speed || 1, next = speed === 1 ? 2 : speed === 2 ? 4 : 1;
    cycle.textContent = speed + "×";
    cycle.setAttribute("aria-label", `战斗速度 ${speed} 倍，点击切换为 ${next} 倍`);
    cycle.disabled = ended || Boolean(battle.menuPaused);
  }
  const menu = document.getElementById("battle-menu-open");
  if (menu) menu.disabled = Boolean(battle.settled || battle.settling || battle.menuPaused);
  renderBattleMenu(battle);
}
// This clock counts presentation time only. Pausing never changes ready_at.
function battlePlaybackTime(battle) {
  const now = performance.now();
  if (battle.playbackClockAt === undefined) {
    battle.playbackClockAt = now;
    battle.playbackClockElapsed = 0;
  }
  if (!battle.menuPaused) battle.playbackClockElapsed += Math.max(0, now - battle.playbackClockAt) * (battle.speed || 1);
  battle.playbackClockAt = now;
  return battle.playbackClockElapsed;
}
function renderBattleMenu(battle) {
  const menu = document.getElementById("battle-menu");
  if (!menu || menu._battle !== battle || !battle.menuPaused) return;
  const status = document.getElementById("battle-menu-status");
  status.textContent = battle.settling ? "正在退出，请稍候…" : battle.menuError || (battle.arena ? "可以继续观看，也可以返回竞技场。" : battle.guildBattle ? "可以继续观看，也可以返回公会。" : "休息一下，准备好再继续。");
  status.classList.toggle("is-error", Boolean(battle.menuError));
  document.getElementById("battle-menu-resume").disabled = Boolean(battle.settling);
  const exit = document.getElementById("battle-menu-exit");
  exit.disabled = Boolean(battle.settling);
  exit.textContent = battle.menuError ? "重试退出" : battle.arena ? "返回竞技场" : battle.guildBattle ? "返回公会" : "退出战斗";
}
function openBattleMenu() {
  const battle = BAT, menu = document.getElementById("battle-menu");
  if (!battleCurrent(battle) || battle.settled || battle.settling || battle.menuPaused || !menu) return;
  battlePlaybackTime(battle);
  battle.menuPaused = true;
  battle.menuError = "";
  battle.menuFocusBefore = document.activeElement;
  menu._battle = battle;
  menu.hidden = false;
  document.getElementById("battle-scene").classList.add("battle-menu-paused");
  const focusable = () => [...menu.querySelectorAll("button:not(:disabled)")];
  battle.menuKeydown = event => {
    if (menu.hidden || menu._battle !== battle) return;
    if (event.key === "Escape") {
      event.preventDefault(); event.stopPropagation();
      closeBattleMenu();
    } else if (event.key === "Tab") {
      const buttons = focusable(), first = buttons[0], last = buttons.at(-1);
      if (!first) { event.preventDefault(); menu.focus(); }
      else if (event.shiftKey && (document.activeElement === first || !menu.contains(document.activeElement))) {
        event.preventDefault(); last.focus();
      } else if (!event.shiftKey && (document.activeElement === last || !menu.contains(document.activeElement))) {
        event.preventDefault(); first.focus();
      }
    }
  };
  battle.menuFocusin = event => {
    if (!menu.hidden && menu._battle === battle && !menu.contains(event.target)) (focusable()[0] || menu).focus();
  };
  document.addEventListener("keydown", battle.menuKeydown, true);
  document.addEventListener("focusin", battle.menuFocusin, true);
  renderPlaybackControls(battle);
  document.getElementById("battle-menu-resume").focus();
}
function dismissBattleMenu(battle, restoreFocus = true) {
  if (!battle) return;
  battlePlaybackTime(battle);
  battle.menuPaused = false;
  document.removeEventListener("keydown", battle.menuKeydown, true);
  document.removeEventListener("focusin", battle.menuFocusin, true);
  const menu = document.getElementById("battle-menu");
  if (menu?._battle !== battle) return;
  menu.hidden = true;
  menu._battle = null;
  document.getElementById("battle-scene")?.classList.remove("battle-menu-paused");
  if (battleCurrent(battle)) renderPlaybackControls(battle);
  if (restoreFocus) {
    const previous = battle.menuFocusBefore;
    if (previous?.isConnected && !previous.disabled) previous.focus();
    else document.getElementById("battle-menu-open")?.focus();
  }
  battle.menuFocusBefore = null;
}
function closeBattleMenu() {
  if (!battleCurrent(BAT) || BAT.settling) return;
  dismissBattleMenu(BAT);
}
async function exitBattleFromMenu() {
  if (!battleCurrent(BAT) || !BAT.menuPaused || BAT.settling) return;
  BAT.menuError = "";
  return exitBattle();
}
function cycleBattleSpeed() {
  setBattleSpeed(BAT?.speed === 1 ? 2 : BAT?.speed === 2 ? 4 : 1);
}
function toggleBattlePause() {
  if (!battleCurrent(BAT) || BAT.playbackComplete) return;
  BAT.paused = !BAT.paused; BAT.steps = 0;
  renderBattleControls(BAT, BAT.paused ? "本次行动结束后暂停" : "正在播放战报");
}
function stepBattle() {
  if (!battleCurrent(BAT) || BAT.playbackComplete) return;
  BAT.paused = true; BAT.steps = (BAT.steps || 0) + 1;
  renderBattleControls(BAT, "单步播放 · 完成后暂停");
}
function setBattleSpeed(speed) {
  if (!battleCurrent(BAT) || ![1, 2, 4].includes(speed)) return;
  battlePlaybackTime(BAT);
  BAT.speed = speed;
  renderPlaybackControls(BAT);
}
function skipBattlePlayback() {
  if (!battleCurrent(BAT) || BAT.playbackComplete) return;
  BAT.skip = true; BAT.paused = false; BAT.steps = 0;
  window.BattleMotion?.cancelBattleMotion(BAT);
  renderBattleControls(BAT, "正在跳过演出");
}
// Playback controls change pacing only; server snapshots, ready_at and settlement stay authoritative.
async function battleDelay(battle, milliseconds) {
  const started = battlePlaybackTime(battle);
  while (battleCurrent(battle) && !battle.skip) {
    if (!await waitBattleMenu(battle)) return;
    const remaining = milliseconds - (battlePlaybackTime(battle) - started);
    if (remaining <= 0) return;
    await sleep(Math.min(50, remaining / (battle.speed || 1)));
  }
}
async function waitBattleMenu(battle) {
  while (battleCurrent(battle) && battle.menuPaused) await sleep(50);
  return battleCurrent(battle);
}
async function waitBattleTurn(battle) {
  while (battleCurrent(battle) && (battle.menuPaused || (battle.paused && !battle.steps && !battle.skip))) await sleep(50);
  if (battle.paused && battle.steps > 0) battle.steps--;
  return battleCurrent(battle);
}
function battleCurrent(battle) {
  return Boolean(battle && BAT === battle && !battle.abort && battle.epoch === Store.sessionEpoch && Store.ready());
}
function battleUnit(battle, uid) {
  return [...battle.allies, ...battle.waves[battle.waveIdx]].find(u => u.uid === uid);
}
function mergeBattleEvent(battle, event) {
  // Validate the complete server position update before moving any DOM nodes.
  const next = new Map((event.units || []).map(snapshot => [snapshot.uid,snapshot]));
  for (const team of [battle.allies,battle.waves[battle.waveIdx]]) {
    const positions = team.map(unit => next.get(unit.uid)?.position ?? unit.slot);
    if (positions.some(slot => !Number.isInteger(slot) || slot < 0 || slot >= 6) || new Set(positions).size !== positions.length) throw new Error("Invalid server formation snapshot");
  }
  let moved = false;
  for (const snapshot of event.units || []) {
    const unit = battleUnit(battle, snapshot.uid);
    if (unit) { const slot = snapshot.position ?? unit.slot; moved ||= slot !== unit.slot; Object.assign(unit, snapshot, { slot }); updateUnit(unit); }
  }
  if (moved) { renderBattle(); batLog("对榫换位 · 已按战报调整阵位"); }
  battle.rounds = event.round;
  renderBattleTimeline(battle);
}
function battleImpactFeedback(src, dst, target, event, color) {
  const pos = unitPoint(dst);
  if (target.blocked) floatText(dst, "抵挡", "heal");
  if (target.rescued) {floatText(dst, "衣角接坠", "heal");batLog(`${esc(dst.name)} 获得致命救援，生命 ${target.hp_set}`);}
  if (target.elemental) {
    const label = target.elemental === "advantage" ? "属性克制 +20%" : "属性受克 −20%";
    batLog(`${esc(src.name)} 对 ${esc(dst.name)} ${label}`);
  }
  if (target.damage > 0 || target.absorbed > 0) {
    hitAnim(dst);
    if (target.damage > 0) floatText(dst, "-" + target.damage, target.crit ? "crit dmg" : "dmg");
    if (target.absorbed > 0) floatText(dst, "盾" + target.absorbed, "heal");
    if (pos && event.skill) spawnFx(pos.x, pos.y, "fx-burst", color);
  }
  if (target.heal > 0) {
    floatText(dst, "+" + target.heal, "heal");
    if (pos) spawnFx(pos.x, pos.y, "fx-heal");
  }
  if (target.shield > 0) {
    shieldAnim(dst); floatText(dst, "盾+" + target.shield, "heal");
    if (pos) spawnFx(pos.x, pos.y, "fx-ring", "#6ea0ff");
  }
  if (target.self_heal > 0) {
    floatText(src, "+" + target.self_heal, "heal");
    batLog(`${esc(src.name)} 吸血恢复 ${target.self_heal}`);
  }
  if (target.reflected > 0) {
    floatText(src, "反伤-" + target.reflected, "dmg");
    batLog(`${esc(src.name)} 受到反击/映伤 ${target.reflected}`);
  }
  if (target.rage_delta < 0) {
    floatText(dst, "怒气" + target.rage_delta, "dmg");
    batLog(`${esc(dst.name)} 被封怒，怒气 ${target.rage_delta}`);
  }
  if (target.rage_delta > 0) {
    floatText(dst, "怒气+" + target.rage_delta, "heal");
    batLog(`${esc(dst.name)} 获得怒气 +${target.rage_delta}`);
  }
}
async function playServerBattle(battle) {
  // 刷新页面后跳过已播放的时间段；这里只重放服务器快照，不计算战斗。
  const elapsed = battle.elapsed;
  let timeline = 0;
  for (const [index, event] of battle.report.events.entries()) {
    if (!battleCurrent(battle)) return;
    if (battle.menuPaused && !await waitBattleMenu(battle)) return;
    const duration = Math.max(0, event.duration_ms || 0);
    let skipped = battle.skip || timeline + duration <= elapsed;
    timeline += duration;
    if (!skipped && event.kind !== "round") {
      if (!await waitBattleTurn(battle)) return;
      skipped = Boolean(battle.skip);
    }
    battle.eventIndex = index;
    if (event.kind === "round") {
      battle.turnOrder = event.order || [];
      battle.actedUids = [];
      battle.actor = null;
      mergeBattleEvent(battle, event);
      continue;
    }
    if (event.kind === "wave") {
      battle.waveIdx = event.wave;
      battle.turnOrder = []; battle.actedUids = []; battle.actor = null;
      renderBattle();
      mergeBattleEvent(battle, event);
      if (!skipped) {
        flashScene();
        const boss = battle.waves[event.wave].some(u => u.boss);
        waveTitle(boss ? "BOSS 来袭" : "第 " + (event.wave + 1) + " 波");
        if (boss) sceneShake();
        batLog("—— 第 " + (event.wave + 1) + " 波 ——");
        await battleDelay(battle, duration);
      }
      continue;
    }
    if (skipped) { mergeBattleEvent(battle, event); continue; }
    const started = battlePlaybackTime(battle);
    const src = battleUnit(battle, event.src);
    battle.actor = event.src;
    if (!battle.turnOrder.length || battle.report.version !== 2) {
      battle.turnOrder = [...new Set(battle.report.events.slice(index).filter(e => e.wave === event.wave && e.round === event.round && e.kind === "action").map(e => e.src))];
    }
    renderBattleTimeline(battle);
    let motion = null;
    try {
      if (src) {
        document.getElementById("u-" + src.uid)?.classList.add("acting");
        const color = src.girl?.fxColor || (src.side === "ally" ? "#ffd76e" : "#ff8a8a");
        if (event.passive) batLog(esc(src.name) + " · 【" + esc(event.skill_name) + "】");
        else if (event.kind === "ailment") batLog(esc(src.name) + " 受到侵蚀伤害");
        else if (event.skipped) batLog(esc(src.name) + " 眩晕中，无法行动");
        else if (event.skill) {
          batLog(esc(src.name) + " 发动【" + esc(event.skill_name) + "】");
          if (!window.BattleMotion || event.kind !== "action") { castAnim(src); skillBanner(event.skill_name); await battleDelay(battle, 300); }
        } else {
          const target = (event.targets || []).find(t => t.damage > 0 || t.absorbed > 0);
          const dst = target && battleUnit(battle, target.uid);
          if (dst) { batLog(esc(src.name) + " 攻击 " + esc(dst.name)); if (!window.BattleMotion) await playStrike(src, dst, color, battle); }
        }
        if (window.BattleMotion && !event.skipped && event.kind === "action") motion = await window.BattleMotion.beginBattleAction(battle, src, event);
        if (!battleCurrent(battle)) return;
        for (const [targetIndex, target] of (event.targets || []).entries()) {
          if (!await waitBattleMenu(battle)) return;
          const dst = battleUnit(battle, target.uid);
          if (!dst) continue;
          const feedback = () => battleImpactFeedback(src, dst, target, event, color);
          if (motion) await motion.impact(target, targetIndex, feedback);
          else feedback();
          if (!battleCurrent(battle)) return;
        }
      }
      if (!await waitBattleMenu(battle)) return;
      mergeBattleEvent(battle, event);
      if (motion) await motion.finish();
    } finally {
      motion?.cancel();
      if (src) document.getElementById("u-" + src.uid)?.classList.remove("acting");
    }
    if (!battleCurrent(battle)) return;
    if (event.kind === "action") battle.actedUids = [...new Set([...(battle.actedUids || []), event.src])];
    battle.actor = null;
    renderBattleTimeline(battle);
    await battleDelay(battle, Math.max(0, duration - (battlePlaybackTime(battle) - started)));
  }
  if (!battleCurrent(battle)) return;
  battle.playbackComplete = true;
  battle.actor = null;
  renderBattleTimeline(battle);
  renderPlaybackControls(battle);
  // Arena reports are already settled; only live dungeon runs have a server deadline.
  while (!battle.arena && !battle.guildBattle && battleCurrent(battle) && performance.now() < battle.readyDeadline) {
    const remaining = battle.readyDeadline - performance.now();
    renderBattleControls(battle, "战报已结束，" + Math.ceil(remaining / 1000) + "秒后自动结算");
    await sleep(Math.min(1000, remaining));
  }
  if (await waitBattleMenu(battle)) await battleEnd(battle);
}

/* ---------- 数据库结算 ---------- */
function showBattleRecovery(battle, message) {
  if (!battleCurrent(battle)) return;
  showModal(`<h3>结算尚未确认</h3>
    <p style="color:var(--sub);text-align:center">${esc(message)}<br>产出以数据库为准，重试不会重复发奖。</p>
    <div class="modal-btns">
      <button class="gbtn gbtn-sm gbtn-primary" onclick="closeModal(true);battleEnd(BAT)"><span>重试结算</span></button>
      <button class="gbtn gbtn-sm" onclick="closeModal(true);recoverBattle()"><span>重新同步</span></button>
      <button class="gbtn gbtn-sm" onclick="closeModal(true);exitBattle()"><span>放弃本局</span></button>
    </div>`);
}
function battleDropLedger(drops) {
  const items = Object.entries(Store.profile?.items?.items || {});
  return drops.map(line => {
    const match = typeof line === "string" && /^(.+?)\s+[x×]\s*(\d+)$/.exec(line);
    const entry = match && items.find(([, item]) => item.name === match[1]);
    if (!entry || typeof atelierIcon !== "function") return `<li>${esc(line)}</li>`;
    return `<li class="battle-item-drop">${atelierIcon(entry[0])}<b>${esc(entry[1].name)}</b><strong>×${match[2]}</strong></li>`;
  }).join("") || "<li>无掉落</li>";
}

async function battleEnd(battle) {
  if (!battleCurrent(battle) || battle.settling || battle.settled) return;
  if (battle.arena) { finishArenaReplay(battle); return; }
  if (battle.guildBattle) { finishGuildReplay(battle); return; }
  battle.settling = true;
  renderBattleControls(battle, "正在结算…");
  const result = await Store.finishRun(battle.runId);
  battle.settling = false;
  if (!battleCurrent(battle)) return;
  if (!result) {
    renderBattleControls(battle, "结算未确认，请重试或重新同步");
    showBattleRecovery(battle, Store.lastError || "网络异常");
    return;
  }
  battle.settled = true;
  battleRunning = false;
  renderBattleControls(battle, "已结算");
  const win = result.win;
  const st = battle.stage;
  const drops = [...(result.drops || [])];
  batLog(win ? "战斗胜利！数据库结算完成" : "战斗失败，无掉落");
  waveTitle(win ? "胜 利" : "战 斗 失 败");
  if (win) {
    flashScene();
    battle.allies.forEach(u => {
      if (!u.alive) return;
      document.getElementById("u-" + u.uid)?.classList.add("victory");
      const p = unitPoint(u);
      if (p) spawnFx(p.x, p.y, "fx-ring", "#ffcf6e");
    });
  }
  // 余额同样重新读取，绝不在客户端把结算额度累加。
  Store.refresh().then(d => { if (d && battleCurrent(battle)) renderTopbar(); });
  renderTopbar();
  if (!battleCurrent(battle)) return;
  showModal(`
    <h3>${win ? "战斗胜利" : "战斗失败"}</h3>
    ${win ? `<div class="res-stars">${starsHTML(result.stars, 26)}</div>
    <div class="res-cond">
      <span class="${result.alive_n === result.total_n ? "ok" : ""}">全员存活 ${result.alive_n}/${result.total_n}</span>
      <span class="${result.fast ? "ok" : ""}">${result.rounds} 回合 · 目标 ${st.starRound}</span>
      ${result.is_best ? `<span class="new">刷新最佳</span>` : ""}
    </div>` : ""}
    <p style="color:var(--sub);font-size:12px;text-align:center">${esc(st.name)} · ${esc(st.type)}</p>
    ${typeof voyageResultSummary === "function" ? voyageResultSummary(st.id, result) : ""}
    <ul class="drop-list">${battleDropLedger(drops)}</ul>
    <div class="modal-btns">
      <button class="gbtn gbtn-sm" onclick="closeModal();nav('${battleDestination(st)}')"><span>${st.progression ? "返回成长" : GACHA_RUN_MODES[st.id] ? "返回玩法" : "返回副本"}</span></button>
      ${GACHA_RUN_MODES[st.id] && !st.progression ? "" : `<button class="gbtn gbtn-sm gbtn-primary" onclick="closeModal();retryBattle('${st.id}')"><span>再来一次</span></button>`}
    </div>`);
}
function retryBattle(id) {
  if (!requireGame()) return;
  startBattle(id);
}
function battleDestination(stage) {
  if (stage.progression) return "page-growth";
  return GACHA_RUN_MODES[stage.id] ? "page-modes" : "page-stage";
}
async function recoverBattle() {
  const battle = BAT;
  if (!battleCurrent(battle) || battle.settling) return;
  if (battle.arena) { finishArenaReplay(battle); return; }
  if (battle.guildBattle) { finishGuildReplay(battle); return; }
  const profile = await Store.refresh();
  if (!profile || !battleCurrent(battle) || battle.settling || battle.settled) return;
  battle.abort = true;
  BAT = null;
  battleRunning = false;
  nav(battleDestination(battle.stage));
  if (Store.activeRun) resumeBattle(Store.activeRun);
  else toast("已同步，本局已结算或过期");
}
function battleControl() {
  if (BAT?.guildBattle) {
    if (BAT.settled) leaveGuildReplay();
    else finishGuildReplay(BAT);
    return;
  }
  if (BAT?.arena) {
    if (BAT.settled) leaveArenaReplay();
    else { BAT.abort = true; battleRunning = false; showArenaResult(BAT.arena); }
    return;
  }
  if (BAT && (BAT.playbackComplete || BAT.playbackFailed) && !BAT.settled) return recoverBattle();
  return exitBattle();
}
/* 撤退也要由服务器确认，网络失败可用同一run_id安全重试。 */
async function exitBattle() {
  const battle = BAT;
  if (!battleCurrent(battle) || battle.settling) return;
  if (battle.arena) { dismissBattleMenu(battle, false); window.BattleMotion?.cancelBattleMotion(battle); leaveArenaReplay(); return true; }
  if (battle.guildBattle) { dismissBattleMenu(battle, false); window.BattleMotion?.cancelBattleMotion(battle); leaveGuildReplay(); return true; }
  if (battle.settled) {
    dismissBattleMenu(battle, false);
    battle.abort = true;
    BAT = null;
    battleRunning = false;
    closeModal(true);
    nav(battleDestination(battle.stage));
    return true;
  }
  battle.settling = true;
  renderPlaybackControls(battle);
  let result;
  try { result = await Store.finishRun(battle.runId, true); }
  catch (_) { result = null; }
  finally { battle.settling = false; }
  if (!battleCurrent(battle)) return;
  if (!result) {
    if (battle.menuPaused) battle.menuError = "暂时无法退出，请检查网络后重试。";
    else toast("撤退未确认，请重试：" + (Store.lastError || "网络异常"));
    renderPlaybackControls(battle);
    if (battle.playbackComplete && !battle.menuPaused) await battleEnd(battle);
    return false;
  }
  dismissBattleMenu(battle, false);
  window.BattleMotion?.cancelBattleMotion(battle);
  battle.abort = true;
  battleRunning = false;
  BAT = null;
  nav(battleDestination(battle.stage));
  toast(battle.stage.progression && !battle.stage.cost ? "已撤退，未发奖励，也未消耗训练资格" : "已撤退，体力不返还");
  return true;
}

function finishArenaReplay(battle) {
  if (!battleCurrent(battle)) return;
  battle.settled = true;
  battleRunning = false;
  renderBattleControls(battle, "挑战已结算");
  showArenaResult(battle.arena);
}

const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const { JSDOM } = require("jsdom");

const read = name => fs.readFileSync(path.join(__dirname, "..", name), "utf8");
const flush = async () => { for (let i = 0; i < 30; i++) await Promise.resolve(); };
const unit = (uid, side) => ({ uid, side, slot: 0, name: uid, img: "assets/gen/enemy-wolf.webp", atkType: "melee", elem: "plant", hp: 100, maxHp: 100, rage: 0, alive: true });

async function harness(context, arena = false) {
  const dom = new JSDOM(read("index.html"), { runScripts: "outside-only", url: "https://example.invalid/gacha/" });
  const w = dom.window, calls = [], dialogs = [], notices = [], hits = [], frames = new Map(), waits = [];
  context.after(() => w.close());
  let time = 0, serial = 0;
  Object.defineProperty(w.performance, "now", { value: () => time });
  w.requestAnimationFrame = callback => { frames.set(++serial, callback); return serial; };
  w.cancelAnimationFrame = id => frames.delete(id);
  w.matchMedia = () => ({ matches: false });
  w.Store = { profile: {}, sessionEpoch: 1, ready: () => true, serverNow: () => time,
    finishRun: async (runId, flee) => { calls.push({ runId, flee }); return { win: false }; }, refresh: async () => null };
  w.requireGame = () => true;
  w.__dialogs = dialogs; w.__notices = notices; w.__hits = hits;
  w.__sleep = milliseconds => new Promise(resolve => waits.push({ due: time + milliseconds, resolve }));
  w.eval(["js/data.js", "js/ui.js", "js/battle-motion.js", "js/battle.js"].map(read).join("\n;\n") + `
    sleep = __sleep;
    toast = message => __notices.push(message);
    showModal = html => __dialogs.push(html);
    renderTopbar = () => {};
    nav = page => { document.body.dataset.page = page; };
    battleImpactFeedback = (source, target, hit) => __hits.push(hit.damage);
    window.api = { openBattleMenu, closeBattleMenu, exitBattleFromMenu, cycleBattleSpeed,
      setBattleSpeed, toggleBattlePause, stepBattle, battleDelay, playServerBattle,
      get battle() { return BAT; }, get running() { return battleRunning; },
      async capture(run) { const original = playServerBattle; playServerBattle = async () => {}; try { await resumeBattle(run); if (run.arena) lastArenaResult = run.arena; } finally { playServerBattle = original; } }
    };
  `);
  const report = { version: 2, duration_ms: 1000, allies: [unit("a0", "ally")], waves: [[unit("e0", "enemy")]], events: [
    { kind: "round", wave: 0, round: 1, duration_ms: 0, order: ["a0", "e0"], units: [] },
    { kind: "action", wave: 0, round: 1, src: "a0", duration_ms: 600, targets: [{ uid: "e0", damage: 20 }, { uid: "e0", damage: 30 }], units: [{ uid: "e0", hp: 50, alive: true }] },
  ] };
  const run = { id: "menu-test", stage: "s1", ready_at: 1000, result: report };
  if (arena) run.arena = { id: run.id, name: "对手", ladder: true, report };
  await w.api.capture(run);
  const document = w.document;
  const rect = (left, top, width, height) => ({ left, top, width, height, right: left + width, bottom: top + height });
  document.getElementById("battle-scene").getBoundingClientRect = () => rect(0, 0, 800, 500);
  document.querySelector("#u-a0 .u-body").getBoundingClientRect = () => rect(80, 200, 100, 160);
  document.querySelector("#u-e0 .u-body").getBoundingClientRect = () => rect(560, 100, 100, 160);
  async function tick(milliseconds = 16) {
    time += milliseconds;
    for (const item of [...waits]) if (item.due <= time) { waits.splice(waits.indexOf(item), 1); item.resolve(); }
    const callbacks = [...frames.values()]; frames.clear(); callbacks.forEach(callback => callback(time));
    await flush();
  }
  async function until(condition, maximum = 200) {
    for (let i = 0; !condition() && i < maximum; i++) await tick();
    assert.ok(condition(), "playback reaches the expected checkpoint");
  }
  return { w, api: w.api, document, calls, dialogs, notices, hits, tick, until, get time() { return time; } };
}

test("pause opens an accessible dialog immediately, traps focus and preserves the step API", async context => {
  const h = await harness(context), { api, document } = h;
  const opener = document.getElementById("battle-menu-open"), menu = document.getElementById("battle-menu");
  api.toggleBattlePause(); api.stepBattle();
  opener.focus(); api.openBattleMenu();
  assert.equal(api.battle.menuPaused, true);
  assert.equal(api.battle.paused, true);
  assert.equal(api.battle.steps, 1, "opening a menu does not consume a pending step");
  assert.equal(menu.hidden, false);
  assert.equal(menu.getAttribute("role"), "dialog");
  assert.equal(document.activeElement.id, "battle-menu-resume");
  document.dispatchEvent(new h.w.KeyboardEvent("keydown", { key: "Tab", shiftKey: true, bubbles: true, cancelable: true }));
  assert.equal(document.activeElement.id, "battle-menu-exit");
  document.dispatchEvent(new h.w.KeyboardEvent("keydown", { key: "Tab", bubbles: true, cancelable: true }));
  assert.equal(document.activeElement.id, "battle-menu-resume");
  document.getElementById("bh-name").tabIndex = 0; document.getElementById("bh-name").focus();
  assert.equal(document.activeElement.id, "battle-menu-resume", "programmatic focus cannot escape the dialog");
  document.dispatchEvent(new h.w.KeyboardEvent("keydown", { key: "Escape", bubbles: true, cancelable: true }));
  assert.equal(menu.hidden, true);
  assert.equal(document.activeElement, opener);
  assert.equal(api.battle.menuPaused, false);
  assert.equal(api.battle.paused, true);
  assert.equal(api.battle.steps, 1);
  for (const speed of [2, 4, 1]) { api.cycleBattleSpeed(); assert.equal(api.battle.speed, speed); }
});

test("menu freezes a running movement and resumes from the same frame", async context => {
  const h = await harness(context), battle = h.api.battle;
  const playing = h.api.playServerBattle(battle);
  await flush(); await h.tick(80);
  const actor = h.document.querySelector(".bm-actor");
  assert.ok(actor);
  const position = actor.style.cssText, clock = h.w.battlePlaybackTime(battle);
  h.api.openBattleMenu();
  for (let i = 0; i < 6; i++) await h.tick(200);
  assert.equal(actor.style.cssText, position);
  assert.equal(h.w.battlePlaybackTime(battle), clock);
  assert.deepEqual(h.hits, []);
  assert.equal(h.calls.length, 0, "a server deadline cannot auto-settle through the pause menu");
  h.api.closeBattleMenu();
  await h.tick(16);
  assert.notEqual(actor.style.cssText, position);
  assert.equal(h.hits.length, 0, "resume does not fast-forward the interrupted movement");
  await h.until(() => battle.settled);
  await playing;
  assert.deepEqual(h.hits, [20, 30]);
  assert.equal(battle.waves[0][0].hp, 50);
  assert.deepEqual(h.calls, [{ runId: "menu-test", flee: undefined }]);
});

test("pause between hits freezes later targets and the event snapshot", async context => {
  const h = await harness(context), battle = h.api.battle;
  const playing = h.api.playServerBattle(battle);
  await h.until(() => h.hits.length === 1);
  h.api.openBattleMenu();
  const before = battle.waves[0][0].hp;
  await h.tick(500); await h.tick(500);
  assert.deepEqual(h.hits, [20]);
  assert.equal(battle.waves[0][0].hp, before);
  h.api.closeBattleMenu();
  await h.until(() => battle.settled);
  await playing;
  assert.deepEqual(h.hits, [20, 30]);
  assert.equal(battle.waves[0][0].hp, 50);
});

test("battleDelay excludes pause time and does not complete at the server deadline", async context => {
  const h = await harness(context);
  let done = false;
  const delay = h.api.battleDelay(h.api.battle, 100).then(() => { done = true; });
  await flush(); await h.tick(40);
  h.api.openBattleMenu();
  await h.tick(2000);
  assert.equal(done, false);
  h.api.closeBattleMenu();
  await h.tick(50);
  assert.equal(done, false, "60 ms remain at resume");
  await h.tick(10); await delay;
  assert.equal(done, true);
});

test("failed or thrown exits remain paused and retry the same server run without rewarding it", async context => {
  const h = await harness(context), battle = h.api.battle;
  battle.playbackComplete = true;
  let attempts = 0, resolveExit;
  h.w.Store.finishRun = (runId, flee) => {
    h.calls.push({ runId, flee }); attempts++;
    if (attempts === 1) return new Promise(resolve => { resolveExit = resolve; });
    if (attempts === 2) throw new Error("network unavailable");
    return Promise.resolve({ abandoned: true });
  };
  h.api.openBattleMenu();
  const pending = h.api.exitBattleFromMenu();
  assert.equal(battle.settling, true);
  h.api.closeBattleMenu();
  await h.api.exitBattleFromMenu();
  assert.equal(h.calls.length, 1, "double clicks cannot submit a second exit");
  assert.equal(h.document.getElementById("battle-menu").hidden, false);
  resolveExit(null); assert.equal(await pending, false);
  assert.equal(battle.menuPaused, true); assert.equal(battle.abort, false);
  assert.equal(battle.settling, false);
  assert.match(h.document.getElementById("battle-menu-status").textContent, /重试/);
  assert.equal(h.document.getElementById("battle-menu-exit").textContent, "重试退出");
  assert.equal(await h.api.exitBattleFromMenu(), false);
  assert.equal(await h.api.exitBattleFromMenu(), true);
  assert.deepEqual(h.calls, Array.from({ length: 3 }, () => ({ runId: "menu-test", flee: true })));
  assert.equal(h.api.battle, null);
  assert.equal(h.api.running, false);
  assert.equal(h.document.getElementById("battle-menu").hidden, true);
  assert.equal(h.document.body.dataset.page, "page-stage");
  assert.equal(h.dialogs.length, 0, "failed exits cannot fall through to normal reward settlement");
});

test("arena exit returns immediately without dungeon requests or reward actions", async context => {
  const h = await harness(context, true), battle = h.api.battle;
  h.api.openBattleMenu();
  assert.equal(await h.api.exitBattleFromMenu(), true);
  assert.equal(h.api.battle, null);
  assert.equal(battle.abort, true);
  assert.equal(h.document.body.dataset.page, "page-arena");
  assert.equal(h.calls.length, 0);
  assert.equal(h.dialogs.length, 0);
  assert.equal(h.document.getElementById("battle-menu").hidden, true);
});

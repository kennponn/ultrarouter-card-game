const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const { JSDOM } = require("jsdom");

function setup(context, reduced = false) {
  const dom = new JSDOM('<div id="battle-scene"><div id="u-a0"><div class="u-body"><img src="ally.webp"></div></div><div id="u-e0"><div class="u-body"><img src="enemy.webp"></div></div></div>', { runScripts: "outside-only", url: "https://example.invalid/gacha/" });
  context.after(() => dom.window.close());
  const w = dom.window, document = w.document;
  let time = 0, serial = 0;
  const callbacks = new Map();
  Object.defineProperty(w.performance, "now", { value: () => time });
  w.requestAnimationFrame = callback => { callbacks.set(++serial, callback); return serial; };
  w.cancelAnimationFrame = id => callbacks.delete(id);
  w.matchMedia = () => ({ matches: reduced });
  w.Store = { sessionEpoch: 9, ready: () => true };
  const rect = (left, top, width, height) => ({ left, top, width, height, right: left + width, bottom: top + height });
  document.getElementById("battle-scene").getBoundingClientRect = () => rect(100, 50, 800, 500);
  document.querySelector("#u-a0 .u-body").getBoundingClientRect = () => rect(130, 210, 100, 160);
  document.querySelector("#u-e0 .u-body").getBoundingClientRect = () => rect(580, 130, 120, 180);
  w.eval(fs.readFileSync(path.join(__dirname, "../js/battle-motion.js"), "utf8"));
  const battle = { epoch: 9, speed: 1, abort: false, skip: false };
  const src = { uid: "a0", side: "ally", atkType: "melee", name: "伙伴", hp: 900 };
  const event = { kind: "action", src: "a0", skill: false, duration_ms: 500,
    targets: [{ uid: "e0", damage: 80 }, { uid: "e0", damage: 65 }], units: [{ uid: "a0", hp: 900 }, { uid: "e0", hp: 100 }] };
  async function tick(ms = 16) {
    time += ms;
    const batch = [...callbacks.values()]; callbacks.clear();
    batch.forEach(callback => callback(time));
    for (let i = 0; i < 5; i++) await Promise.resolve();
  }
  async function settle(promise) {
    let done = false, value, error;
    promise.then(result => { value = result; done = true; }, reason => { error = reason; done = true; });
    let ticks = 0;
    while (!done && ticks++ < 200) await tick();
    assert.ok(done, "animation promise must settle");
    if (error) throw error;
    return { value, ticks };
  }
  return { w, document, battle, src, event, tick, settle, callbacks };
}

test("melee reaches the opposite body geometry, presents repeated hits and returns without changing the report", async context => {
  const { w, document, battle, src, event, settle, callbacks } = setup(context);
  const before = JSON.stringify({ src, event });
  const action = (await settle(w.BattleMotion.beginBattleAction(battle, src, event))).value;
  const actor = document.querySelector(".bm-actor");
  const x = parseFloat(actor.style.left), y = parseFloat(actor.style.top);
  assert.ok(x > 300 && x < 440, "contact comes from both body widths, not a fixed dash");
  assert.ok(y < 160 && y > 80, "approach follows the diagonal between the two figures");
  assert.ok(Math.abs((x - 30) / 460 - (y - 160) / -70) < .001, "figure centres share the approach vector");
  assert.equal(document.querySelector("#u-a0 .u-body").style.visibility, "hidden");
  const hits = [];
  for (const [index, target] of event.targets.entries()) await settle(action.impact(target, index, () => hits.push(target.damage)));
  assert.deepEqual(hits, [80, 65], "same-target hits remain separate and ordered");
  await settle(action.finish());
  assert.equal(document.querySelector(".battle-motion-layer"), null);
  assert.equal(document.querySelector("#u-a0 .u-body").style.visibility, "");
  assert.equal(document.querySelector(".bm-target"), null);
  assert.equal(callbacks.size, 0);
  assert.equal(JSON.stringify({ src, event }), before);
});

test("ranged skill stays in its slot and safely renders a portrait before the projectile", async context => {
  const { w, document, battle, src, event, settle, tick } = setup(context);
  src.atkType = "ranged"; event.skill = true; event.skill_name = "<img src=x onerror=alert(1)>";
  const starting = w.BattleMotion.beginBattleAction(battle, src, event);
  assert.equal(document.querySelector(".bm-cutin strong").textContent, event.skill_name);
  assert.equal(document.querySelector(".bm-cutin strong img"), null);
  const action = (await settle(starting)).value;
  assert.equal(document.querySelector(".bm-actor").style.left, "30px");
  const impact = action.impact(event.targets[0], 0);
  await tick();
  assert.ok(document.querySelector(".bm-projectile"));
  await settle(impact); await settle(action.finish());
  assert.equal(document.querySelector(".bm-shade"), null);
});

test("speed changes affect running motion and skip resolves promises with complete cleanup", async context => {
  const { w, document, battle, src, event, tick, settle, callbacks } = setup(context);
  const starting = w.BattleMotion.beginBattleAction(battle, src, event);
  await tick(32);
  const before = parseFloat(document.querySelector(".bm-actor").style.left);
  battle.speed = 4;
  await tick(32);
  assert.ok(parseFloat(document.querySelector(".bm-actor").style.left) > before);
  battle.skip = true;
  const action = (await settle(starting)).value;
  assert.equal(action.active, false);
  assert.equal(document.querySelector(".battle-motion-layer"), null);
  assert.equal(document.querySelector("#u-a0 .u-body").style.visibility, "");
  assert.equal(callbacks.size, 0);
});

test("session changes cancel between impact calls and preserve prior inline visibility", async context => {
  const { w, document, battle, src, event, tick, settle, callbacks } = setup(context);
  const body = document.querySelector("#u-a0 .u-body");
  body.style.setProperty("visibility", "visible", "important");
  const action = (await settle(w.BattleMotion.beginBattleAction(battle, src, event))).value;
  w.Store.sessionEpoch++;
  await tick();
  assert.equal(action.active, false);
  assert.equal(body.style.visibility, "visible");
  assert.equal(body.style.getPropertyPriority("visibility"), "important");
  assert.equal(callbacks.size, 0);
  assert.equal(await action.impact(event.targets[0]), false);
});

test("reduced motion leaves portraits in place and still runs each impact callback", async context => {
  const { w, document, battle, src, event, settle, callbacks } = setup(context, true);
  event.skill = true;
  const action = (await settle(w.BattleMotion.beginBattleAction(battle, src, event))).value;
  let hits = 0;
  for (const [index, target] of event.targets.entries()) await action.impact(target, index, () => hits++);
  assert.equal(hits, 2);
  assert.equal(document.querySelector(".battle-motion-layer"), null);
  assert.equal(document.querySelector("#u-a0 .u-body").style.visibility, "");
  await action.finish();
  assert.equal(callbacks.size, 0);
});

test("a failed impact callback restores the hidden actor and releases the action", async context => {
  const { w, document, battle, src, event, settle, callbacks } = setup(context);
  const action = (await settle(w.BattleMotion.beginBattleAction(battle, src, event))).value;
  const failed = action.impact(event.targets[0], 0, () => { throw new Error("interrupted effect"); });
  await assert.rejects(settle(failed), /interrupted effect/);
  assert.equal(document.querySelector(".battle-motion-layer"), null);
  assert.equal(document.querySelector("#u-a0 .u-body").style.visibility, "");
  assert.equal(callbacks.size, 0);
});

test("portrait contact approaches vertically and stays inside the battlefield", async context => {
  const { w, document, battle, src, event, settle } = setup(context);
  const box = (left, top, width, height) => ({ left, top, width, height });
  document.getElementById("battle-scene").getBoundingClientRect = () => box(0, 0, 320, 540);
  document.querySelector("#u-a0 .u-body").getBoundingClientRect = () => box(100, 360, 100, 150);
  document.querySelector("#u-e0 .u-body").getBoundingClientRect = () => box(100, 60, 100, 150);
  let action = (await settle(w.BattleMotion.beginBattleAction(battle, src, event))).value;
  let actor = document.querySelector(".bm-actor");
  assert.equal(parseFloat(actor.style.left), 100);
  assert.ok(parseFloat(actor.style.top) > 60 && parseFloat(actor.style.top) < 220, "actor approaches from below its target");
  await settle(action.finish());
  // Extreme diagonal geometry must never carry the clone outside the scene.
  document.querySelector("#u-e0 .u-body").getBoundingClientRect = () => box(-20, -45, 240, 220);
  action = (await settle(w.BattleMotion.beginBattleAction(battle, src, event))).value;
  actor = document.querySelector(".bm-actor");
  assert.ok(parseFloat(actor.style.left) >= 0 && parseFloat(actor.style.left) <= 220);
  assert.ok(parseFloat(actor.style.top) >= 0 && parseFloat(actor.style.top) <= 390);
  await settle(action.finish());
});

test("short skill reports keep a readable portrait without altering their duration", async context => {
  const { w, document, battle, src, event, tick, settle } = setup(context);
  event.skill = true; event.duration_ms = 420;
  const beginning = w.BattleMotion.beginBattleAction(battle, src, event);
  await tick(200);
  assert.ok(document.querySelector(".bm-cutin"), "normal-speed cut-in remains visible after 200 ms");
  const action = (await settle(beginning)).value;
  for (const [index, target] of event.targets.entries()) await settle(action.impact(target, index));
  await settle(action.finish());
  assert.equal(event.duration_ms, 420, "readability never mutates the server deadline or report");
  assert.ok(w.performance.now() >= 900 && w.performance.now() < 1100);
});

test("menu pause also holds reduced-motion callbacks and cancellation releases the waiter", async context => {
  const { w, battle, src, event, settle, tick } = setup(context, true);
  const action = (await settle(w.BattleMotion.beginBattleAction(battle, src, event))).value;
  battle.menuPaused = true;
  const hits = [];
  let done = false;
  const impact = action.impact(event.targets[0], 0, () => hits.push(80)).then(value => { done = true; return value; });
  await tick(2000);
  assert.equal(done, false);
  assert.deepEqual(hits, []);
  battle.menuPaused = false;
  assert.equal((await settle(impact)).value, true);
  assert.deepEqual(hits, [80]);
  battle.menuPaused = true;
  const pending = action.impact(event.targets[1], 1, () => hits.push(65));
  action.cancel();
  assert.equal((await settle(pending)).value, false);
  assert.deepEqual(hits, [80]);
});

test("moving figures own their ground shadow and skill portraits cover elevated slots", async context => {
  const { w, document, battle, src, event, settle, tick } = setup(context);
  const style = document.createElement("style");
  style.textContent = fs.readFileSync(path.join(__dirname, "../battle-motion.css"), "utf8");
  document.head.appendChild(style);
  event.skill = true;
  const beginning = w.BattleMotion.beginBattleAction(battle, src, event);
  await tick(80);
  const source = document.getElementById("u-a0"), layer = document.querySelector(".battle-motion-layer");
  assert.ok(source.classList.contains("bm-source"), "home-slot shadows are suppressed while the figure is cloned");
  assert.ok(document.querySelector(".bm-actor > .bm-avatar"), "the traveling actor supplies a foot-anchored CSS ellipse");
  assert.equal(w.getComputedStyle(layer).zIndex, "50", "cut-in clears the acting and target slots");
  const action = (await settle(beginning)).value;
  assert.equal(w.getComputedStyle(layer).zIndex, "35", "normal impacts return below the target labels");
  await settle(action.finish());
  assert.equal(source.classList.contains("bm-source"), false, "return restores the original ground shadows");
  assert.equal(document.querySelector(".bm-actor"), null);
});

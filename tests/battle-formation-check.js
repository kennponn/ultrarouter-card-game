const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const { JSDOM } = require("jsdom");

const root = path.resolve(__dirname, "..");
const read = name => fs.readFileSync(path.join(root, name), "utf8");
const clone = value => JSON.parse(JSON.stringify(value));
const flush = async () => { for (let i = 0; i < 20; i++) await Promise.resolve(); };
const ids = ["c08", "c06", "c01", "c12", "c02", "c20", "c21"];

function harness(t) {
  const dom = new JSDOM(read("index.html"), { runScripts: "outside-only", url: "https://example.invalid/gacha/", pretendToBeVisual: true });
  const w = dom.window, calls = [], notices = [], dialogs = [], waits = [], clock = { now: 0 };
  t.after(() => w.close());
  Object.defineProperty(w.performance, "now", { value: () => clock.now });
  const store = w.Store = {
    profile: {}, sessionEpoch: 1, ready: () => true, serverNow: () => clock.now,
    mutate(action, body) {
      let resolve;
      const promise = new Promise(done => { resolve = done; });
      calls.push({ kind: "action", action, body: clone(body), resolve });
      return promise;
    },
    async finishRun(runId) { calls.push({ kind: "finish", runId, at: clock.now }); return { win: false, drops: ["服务器确认的掉落"], stars: 0 }; },
    refresh: async () => null,
  };
  w.requireGame = () => store.ready();
  w.__notices = notices; w.__dialogs = dialogs; w.__waits = waits;
  w.eval(["js/data.js", "js/ui.js", "js/progression.js", "js/battle.js"].map(read).join("\n;\n") + `
    toast = message => __notices.push(message);
    showModal = html => __dialogs.push(html);
    renderTopbar = () => {};
    nav = page => { document.body.dataset.page = page; };
    sleep = milliseconds => new Promise(resolve => __waits.push({ milliseconds, resolve }));
    window.api = { toggleSquad, selectSquadSlot, renderSquad, validBattleFormation, formationHTML,
      mergeBattleEvent, renderBattle, resumeBattle, play: playServerBattle,
      toggleBattlePause, stepBattle, setBattleSpeed, skipBattlePlayback,
      growthTeamSelection, saveGrowthTeams, startGrowthRun,
      mountDual() { document.getElementById("growth-body").innerHTML = growthDualMarkup(); updateGrowthTeams(false); },
      get save() { return SAVE; }, setSave(value) { SAVE = value; }, get battle() { return BAT; },
      reset() { if (BAT) BAT.abort = true; BAT = null; battleRunning = false; },
      async capture(run) { const original = playServerBattle; playServerBattle = async () => {}; try { await resumeBattle(run); } finally { playServerBattle = original; } }
    };
  `);
  const api = w.api;
  api.setSave({ squad: ids.slice(0, 5), girls: Object.fromEntries(ids.map(id => [id, { star: 1, level: 1, exp: 0, shards: 0 }])), gold: 100, mats: {} });
  async function advance(milliseconds) {
    clock.now += milliseconds;
    waits.splice(0).forEach(wait => wait.resolve());
    await flush();
  }
  function accept(call, squad = call.body.squad) {
    api.setSave({ ...api.save, squad: clone(squad) });
    call.resolve({ save: clone(api.save) });
  }
  return { api, w, document: w.document, calls, notices, dialogs, waits, clock, advance, accept };
}

function unit(index, side = "ally", slot = index) {
  return { uid: (side === "ally" ? "a" : "e") + index, side, slot, name: side + index,
    girlId: side === "ally" ? ids[index] : "", img: "assets/gen/enemy-wolf.webp", elem: "plant", atkType: "melee",
    hp: 100, maxHp: 100, atk: 30, def: 20, spd: 100 - index, rage: 0, alive: true };
}

function run(version = 2) {
  return { id: "formation-run", stage: "s1", ready_at: 1000, result: {
    version, allies: Array.from({ length: 6 }, (_, index) => unit(index)), waves: [[unit(0, "enemy")]],
    events: [], duration_ms: 1000, win: true, stars: 3, rounds: 1, alive_n: 6, total_n: 6,
  } };
}

test("six-member roster is server-confirmed and a seventh member sends no intent", async t => {
  const h = harness(t), before = clone(h.api.save.squad);
  const addition = h.api.toggleSquad(ids[5]);
  assert.deepEqual(clone(h.api.save.squad), before);
  assert.deepEqual(h.calls[0].body, { squad: ids.slice(0, 6) });
  assert.equal(h.calls[0].action, "squad");
  h.calls[0].resolve(null);
  await addition;
  assert.deepEqual(clone(h.api.save.squad), before, "failed mutation cannot alter local squad");
  const accepted = h.api.toggleSquad(ids[5]);
  h.accept(h.calls[1]);
  await accepted;
  assert.deepEqual(clone(h.api.save.squad), ids.slice(0, 6));
  assert.equal(h.document.querySelectorAll("#squad-slots .team-slot").length, 6);
  assert.equal(h.document.getElementById("team-count").textContent, "6/6");
  await h.api.toggleSquad(ids[6]);
  assert.equal(h.calls.length, 2);
  assert.match(h.notices.at(-1), /6/);
});

test("selecting two cards sends an ordered squad intent and waits for the server", async t => {
  const h = harness(t);
  h.api.setSave({ ...h.api.save, squad: ids.slice(0, 6) });
  await h.api.selectSquadSlot(0);
  assert.equal(h.calls.length, 0);
  const pending = h.api.selectSquadSlot(5);
  const expected = [ids[5], ...ids.slice(1, 5), ids[0]];
  assert.deepEqual(h.calls[0].body, { squad: expected });
  assert.deepEqual(clone(h.api.save.squad), ids.slice(0, 6));
  h.accept(h.calls[0]);
  await pending;
  assert.deepEqual(clone(h.api.save.squad), expected);
  assert.match(h.document.querySelector("#squad-slots .squad-pick").getAttribute("aria-label"), /暖弦/);
});

test("switching accounts clears a selected swap card before render or the next click", async t => {
  const h = harness(t);
  await h.api.selectSquadSlot(0);
  assert.equal(h.document.querySelectorAll(".squad-pick[aria-pressed=true]").length, 1);
  h.w.Store.sessionEpoch++;
  h.api.setSave(clone(h.api.save));
  h.api.renderSquad();
  assert.equal(h.document.querySelectorAll(".squad-pick[aria-pressed=true]").length, 0);
  await h.api.selectSquadSlot(1);
  assert.equal(h.calls.length, 0, "new account's first click only selects a card");
  h.w.Store.sessionEpoch++;
  h.api.setSave(clone(h.api.save));
  await h.api.selectSquadSlot(2);
  assert.equal(h.calls.length, 0, "click handler also clears stale selection without a preceding render");
  assert.match(h.document.querySelector(".squad-pick[aria-pressed=true]").getAttribute("aria-label"), /灯里/);
});

test("six fixed battle positions preserve death slots and mirror front rows", async t => {
  const h = harness(t), report = run();
  report.result.allies = [unit(0, "ally", 5), unit(1, "ally", 0), unit(2, "ally", 2)];
  await h.api.capture(report);
  assert.equal(h.document.querySelectorAll("#side-ally > .battle-slot").length, 6);
  assert.equal(h.document.querySelectorAll("#side-enemy > .battle-slot").length, 6);
  assert.equal(h.document.querySelector("#u-a0").parentElement.dataset.slot, "5");
  assert.equal(h.document.querySelector('#side-ally > [data-slot="0"]').style.getPropertyValue("--slot-column"), "2");
  assert.equal(h.document.querySelector('#side-enemy > [data-slot="0"]').style.getPropertyValue("--slot-column"), "1");
  h.api.mergeBattleEvent(h.api.battle, { round: 1, units: [{ uid: "a1", alive: false, hp: 0, slot: 4 }] });
  h.api.renderBattle();
  assert.equal(h.document.querySelector("#u-a1").parentElement.dataset.slot, "0");
  assert(h.document.querySelector("#u-a1").classList.contains("dead"));
  assert.equal(h.document.querySelector("#u-a0").parentElement.dataset.slot, "5");
  assert.equal(h.document.querySelectorAll("#side-ally > .vacant").length, 3);
});

test("legacy reports infer slots while v2 rejects duplicate or out-of-range positions", async t => {
  const h = harness(t);
  for (const version of [undefined, 0, 1]) {
    for (const oldSlot of [undefined, 0]) {
      h.api.reset();
      const value = run(version);
      value.result.version = version;
      value.result.allies.forEach(entry => { entry.slot = oldSlot; });
      value.result.waves[0][0].slot = oldSlot;
      await h.api.capture(value);
      assert(h.api.battle, `version ${version}, slot ${oldSlot} must load`);
      assert.deepEqual(clone(h.api.battle.allies.map(entry => entry.slot)), [0, 1, 2, 3, 4, 5]);
      assert.equal(h.document.querySelectorAll("#side-ally .unit").length, 6);
    }
  }
  for (const slot of [0, -1, 6, 1.5, undefined]) {
    h.api.reset();
    const value = run(); value.result.allies[5].slot = slot;
    await h.api.capture(value);
    assert.equal(h.api.battle, null, `v2 must reject invalid sixth slot ${slot}`);
    assert.match(h.notices.at(-1), /战报不完整/);
  }
  assert.equal(h.calls.length, 0, "invalid reports cannot send settlement requests");
});

test("pause, step, speed and skip replay snapshots without bypassing ready_at or server outcome", async t => {
  const h = harness(t), value = run();
  value.result.events = [
    { kind: "round", round: 1, wave: 0, duration_ms: 0, order: ["a0", "e0"], units: [] },
    { kind: "action", round: 1, wave: 0, duration_ms: 200, src: "a0", targets: [{ uid: "e0", damage: 9999 }], units: [{ uid: "e0", hp: 37, alive: true }] },
    { kind: "action", round: 1, wave: 0, duration_ms: 200, src: "e0", targets: [], units: [{ uid: "a0", hp: 11, alive: true }] },
  ];
  await h.api.capture(value);
  h.api.toggleBattlePause();
  h.api.setBattleSpeed(4); h.api.setBattleSpeed(99);
  assert.equal(h.api.battle.speed, 4, "unsupported rates are ignored");
  const playing = h.api.play(h.api.battle);
  await flush();
  assert.equal(h.api.battle.waves[0][0].hp, 100, "pause prevents the next action");
  assert.equal(h.api.battle.turnOrder.join(","), "a0,e0");
  h.api.stepBattle();
  for (let i = 0; i < 6; i++) await h.advance(50);
  assert.equal(h.api.battle.waves[0][0].hp, 37, "HP comes from the snapshot, not the damage label");
  assert.equal(h.api.battle.allies[0].hp, 100, "one step does not run the second action");
  assert.equal(h.api.battle.paused, true);
  assert.equal(h.calls.length, 0);
  h.api.skipBattlePlayback();
  await h.advance(50);
  assert.equal(h.api.battle.allies[0].hp, 11, "skip still applies every server snapshot");
  assert.equal(h.api.battle.playbackComplete, true);
  assert.equal(h.calls.length, 0, "skip must wait for ready_at");
  await h.advance(649);
  assert.equal(h.calls.length, 0, "cannot finish one millisecond early");
  await h.advance(1);
  await playing;
  assert.deepEqual(h.calls.map(call => ({ kind: call.kind, at: call.at })), [{ kind: "finish", at: 1000 }]);
  assert.equal(h.api.battle.settled, true);
  assert.match(h.dialogs.at(-1), /战斗失败/, "receipt outcome overrides the initial win report");
  assert.match(h.dialogs.at(-1), /服务器确认的掉落/);
  assert.equal(h.api.save.gold, 100, "replay never awards local currency");
});

test("skipping a settled arena replay applies snapshots immediately without a dungeon settlement", async t => {
  const h = harness(t), value = run();
  h.w.eval('showArenaResult = result => __dialogs.push(result);');
  value.ready_at = 60000;
  value.result.duration_ms = 60000;
  value.result.events = [{ kind: "action", wave: 0, round: 1, src: "a0", duration_ms: 60000, units: [{ uid: "e0", hp: 0, alive: false }] }];
  value.arena = { id: value.id, ladder: true, win: true, report: value.result };
  await h.api.capture(value);
  h.api.skipBattlePlayback();
  await h.api.play(h.api.battle);
  assert.equal(h.api.battle.waves[0][0].hp, 0);
  assert.equal(h.api.battle.settled, true);
  assert.equal(h.clock.now, 0, "an already settled replay does not wait for a server grant");
  assert.equal(h.calls.length, 0, "replay cannot finish a dungeon or issue rewards");
  assert.equal(h.dialogs[0].ladder, true);
});

test("an abandoned final event cannot overwrite a new account's battle controls", async t => {
  const h = harness(t), previous = run();
  previous.result.events = [{ kind: "action", wave: 0, round: 9, src: "e0", duration_ms: 200, targets: [], units: [] }];
  await h.api.capture(previous);
  const oldBattle = h.api.battle, playing = h.api.play(oldBattle);
  await flush();
  assert.equal(oldBattle.rounds, 9);
  assert(h.waits.length > 0, "old replay is waiting inside its final event");
  h.api.reset();
  h.w.Store.sessionEpoch++;
  const current = run(); current.id = "new-account-run";
  await h.api.capture(current);
  const newBattle = h.api.battle;
  assert.equal(h.document.getElementById("battle-round").textContent, "0");
  assert.equal(h.document.getElementById("battle-pause").disabled, false);
  await h.advance(50);
  await playing;
  assert.equal(h.api.battle, newBattle);
  assert.equal(h.document.getElementById("battle-round").textContent, "0", "old round must not replace the new timeline");
  for (const id of ["battle-pause", "battle-step", "battle-skip"]) assert.equal(h.document.getElementById(id).disabled, false, id);
  assert.equal(h.calls.length, 0, "old replay cannot submit settlement");
  assert.equal(h.dialogs.length, 0);
});

test("dual-team editor preserves 4/5/6 and mixed legacy squads and submits twelve IDs", async t => {
  const h = harness(t);
  const twelve = JSON.parse(read("backend/model/gacha_catalog.json")).GIRLS.slice(0, 12).map(girl => girl.id);
  const state = (first, second) => ({
    ...h.api.save, squad: twelve.slice(0, first),
    girls: Object.fromEntries(twelve.map(id => [id, { star: 3, level: 40, shards: 0, exp: 0 }])),
    stamina: 100, cleared: { s6: true },
    progression: { reserve_squad: twelve.slice(first, first + second), dual: { phase: 0, cleared: false } },
  });
  for (const [first, second] of [[4, 4], [5, 5], [6, 4], [6, 6]]) {
    h.api.setSave(state(first, second));
    h.api.mountDual();
    assert.equal(h.document.querySelectorAll(".growth-team-editor select").length, 12);
    assert.deepEqual(clone(h.api.growthTeamSelection()), [twelve.slice(0, first), twelve.slice(first, first + second)]);
    assert.equal(h.document.getElementById("growth-dual-start").disabled, false, `${first}+${second} existing squad may start`);
    assert.equal(h.document.getElementById("growth-team-save").disabled, first !== second, "new saves require equal team sizes");
    assert.match(h.document.getElementById("growth-team-warning").textContent, /已保存/);
  }
  const before = clone(h.api.save);
  const saving = h.api.saveGrowthTeams();
  assert.equal(h.calls.length, 1);
  assert.equal(h.calls[0].action, "teams_save");
  assert.deepEqual(h.calls[0].body, { squad: twelve });
  assert.deepEqual(clone(h.api.save), before, "submitting the editor never mutates the snapshot");
  h.calls[0].resolve({ save: before });
  await saving;
  assert.deepEqual(clone(h.api.save.squad), twelve.slice(0, 6));
  assert.deepEqual(clone(h.api.save.progression.reserve_squad), twelve.slice(6));
});

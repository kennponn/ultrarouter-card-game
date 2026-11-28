const assert = require("node:assert/strict");
const test = require("node:test");
const fs = require("node:fs");
const path = require("node:path");
const { JSDOM } = require("jsdom");

function setup(context, guild = null, hash = "") {
  const dom = new JSDOM('<body data-page="page-fleet"><button id="opener" data-rank-demo-launch>打开</button><div id="fleet-body"></div></body>', { runScripts: "outside-only", url: `https://example.invalid/gacha/${hash}` });
  context.after(() => dom.window.close());
  const calls = [], reads = [], notices = [];
  const store = {
    ready: () => true, sessionEpoch: 1, profile: { user_id: 14, guild },
    refresh: async () => store.profile,
    mutate: async (action, fields) => { calls.push([action, JSON.parse(JSON.stringify(fields))]); return null; },
    api: async (url, method) => {
      assert.equal(method, "GET");
      const query = new URL(url, "https://example.invalid").searchParams;
      const board = query.get("board"), after = Number(query.get("after"));
      reads.push([board, after]);
      const entries = board === "directory" ? [{ name: after ? "后续" : "<img src=x>", members: 3, code: "00000002", value: 4 }] : [{ name: "藏家", mine: true, value: 7 }];
      return { board, entries, catalog_size: 28, star_limit: 156, member_limit: 10, my_name: "藏家", my_rank: 78, my_value: 7, next: board === "directory" && !after ? 12 : 0 };
    },
  };
  const window = dom.window;
  Object.assign(window, { Store: store, SITE: { ep: { fleet: "/api/user/gacha/fleet" } }, ELEM: {}, SAVE: { stamina: 90, squad: ["c01"] }, requireGame: () => true, renderTopbar() {}, toast: notice => notices.push(notice) });
  const ui = fs.readFileSync(path.join(__dirname, "../js/ui.js"), "utf8");
  window.eval(ui.slice(ui.indexOf("function esc("), ui.indexOf("/* ---------- 页面路由")));
  window.eval(`${fs.readFileSync(path.join(__dirname, "../js/data.js"), "utf8")}\nwindow.GIRLS = GIRLS;`);
  window.eval(fs.readFileSync(path.join(__dirname, "../js/fleet.js"), "utf8"));
  const document = window.document;
  const click = selector => {
    const target = document.querySelector(selector);
    assert.ok(target, selector);
    return target.closest("#fleet-rank-hall") ? window.fleetRankClick({ target }) : window.fleetClick({ target });
  };
  return { window, document, store, reads, calls, click, notices };
}

test("directory shows safe public entries, pages by cursor, and posts only the join intent", async context => {
  const { window, document, reads, calls, click } = setup(context);
  await window.renderFleet();
  assert.deepEqual(reads, [["directory", 0]]);
  assert.equal(document.querySelector("#fleet-body img"), null);
  assert.ok(document.getElementById("fleet-pane").textContent.includes("<img src=x>"));
  await click('[data-fleet-act="next"]');
  assert.deepEqual(reads.at(-1), ["directory", 12]);
  assert.equal(document.querySelector('[data-fleet-act="next"]').disabled, true);
  await click('[data-fleet-act="previous"]');
  await click('[data-fleet-act="recruit"]');
  assert.deepEqual(calls, [["guild_join", { item_id: "00000002" }]]);
  await click('[data-fleet-act="hall"]');
  await click('[data-fleet-tab="collection"]');
  assert.match(document.querySelector(".fleet-standing").textContent, /第 78 名/);
  assert.match(document.querySelector(".rank-list").textContent, /25\.0%/);
  assert.match(document.querySelector(".rank-list").textContent, /7 \/ 28 张整卡/);
  assert.equal(document.querySelector('[data-fleet-tab="collection"]').getAttribute("aria-pressed"), "true");
});

test("cooperative commands use server eligibility and wait for confirmed grants", async context => {
  const guild = { name: "波港", leader_id: 14, code: "0000000E", listed: false, members: [{ user_id: 14, name: "甲" }], boss_week: "2026-W48", boss_hp: 500, boss_max_hp: 24000,
    harbor: { score: 4, line_limit: 10, my_cargo: 1, lines: [{ id: "charts", name: "绘图", cost: "体力 5", done: 1, eligible: true }, { id: "lamps", name: "灯塔", cost: "结晶 2", done: 0, eligible: false }], beacons: [{ tier: 1, name: "浅湾", goal: 1, gold: 100, ore: 2, eligible: true }] } };
  const { window, document, click, calls } = setup(context, guild);
  await window.renderFleet();
  assert.equal(document.querySelector('[data-fleet-act="supply"][data-value="lamps"]').disabled, true);
  await click('[data-fleet-act="supply"][data-value="lamps"]');
  await click('[data-fleet-act="supply"][data-value="charts"]');
  await click('[data-fleet-act="claim"]');
  await click('[data-fleet-act="listing"]');
  assert.deepEqual(calls, [["guild_supply", { item_id: "charts" }], ["guild_supply_claim", { count: 1 }], ["guild_listing", { enabled: true }]]);
  assert.equal(guild.harbor.lines[0].done, 1);
  assert.equal(guild.harbor.beacons[0].claimed, undefined);
});

test("older rankings and late session responses cannot replace the current tab", async context => {
  const { window, document, click, store } = setup(context);
  await window.renderFleet();
  await click('[data-fleet-act="hall"]');
  const immediate = store.api;
  let release;
  store.api = () => new Promise(done => { release = done; });
  const old = click('[data-fleet-tab="power"]');
  store.api = immediate;
  await click('[data-fleet-tab="collection"]');
  release({ board: "power", entries: [{ name: "过时榜单" }] });
  await old;
  assert.doesNotMatch(document.getElementById("fleet-rank-hall").textContent, /过时榜单/);
  store.api = () => new Promise(done => { release = done; });
  const outgoing = click('[data-fleet-tab="power"]');
  store.sessionEpoch++;
  release({ board: "power", entries: [{ name: "其他账号" }] });
  await outgoing;
  assert.doesNotMatch(document.getElementById("fleet-rank-hall").textContent, /其他账号/);
  window.closeFleetOfficial();
  assert.equal(document.getElementById("fleet-rank-hall"), null);
});

test("standalone demo uses only original local portraits, five boards, focusable details and a return path without a session", async context => {
  const { window, document, store, click, reads, calls } = setup(context, null, "#rank-demo-stars");
  store.ready = () => false;
  const opener = document.getElementById("opener"); opener.focus();
  window.fleetRankHash();
  const hall = document.getElementById("fleet-rank-hall");
  assert.equal(hall.dataset.rankBoard, "stars");
  assert.equal(hall.dataset.rankSource, "demo");
  assert.match(hall.querySelector("[data-rank-origin]").textContent, /演示榜/);
  assert.equal(hall.querySelectorAll(".rank-columns span").length, 4);
  assert.equal(hall.querySelectorAll(".rank-list [data-rank-row]").length, 8);
  for (const medal of [1,2,3]) assert.equal(hall.querySelector(`[data-medal="${medal}"]`).textContent, String(medal));
  assert.equal(hall.querySelectorAll('[data-rank-row="0"] .rank-faces img').length, 4);
  for (const image of hall.querySelectorAll("img")) {
    assert.match(image.getAttribute("src"), /^content\/cards\/c\d\d\/assets\/portrait\.webp$/i);
    assert.ok(fs.existsSync(path.join(__dirname, "..", image.getAttribute("src"))));
  }
  assert.match(hall.querySelector(".rank-standing").textContent, /蓝檐回声/);
  await click('[data-rank-row="1"]');
  const detail = document.getElementById("fleet-rank-detail");
  assert.match(detail.textContent, /演示榜.*第 2 名/s);
  assert.equal(detail.querySelectorAll(".rank-faces img").length, 4);
  detail.dispatchEvent(new window.Event("cancel", { cancelable:true }));
  assert.equal(document.activeElement.dataset.rankRow, "1");
  for (const board of ["power", "collection", "guild", "arena", "stars"]) {
    await click(`[data-fleet-tab="${board}"]`);
    assert.equal(hall.dataset.rankBoard, board);
    assert.equal(hall.querySelector(`[data-fleet-tab="${board}"]`).getAttribute("aria-pressed"), "true");
  }
  hall.querySelector('[data-fleet-tab="stars"]').dispatchEvent(new window.KeyboardEvent("keydown", { key:"Home", bubbles:true }));
  assert.equal(document.activeElement.dataset.fleetTab, "power");
  assert.equal(hall.dataset.rankBoard, "power");
  await click('[data-rank-action="back"]');
  assert.equal(document.activeElement, opener);
  assert.equal(window.location.hash, "");
  assert.deepEqual(reads, []); assert.deepEqual(calls, []);
});

test("empty and unavailable live rankings offer an explicit demo, never filling or mutating the real board", async context => {
  const { window, document, store, calls, click } = setup(context);
  let reads = 0;
  store.api = async () => { reads++; return { board:"stars", entries:[], star_limit:156, my_rank:0, my_value:0 }; };
  await window.openFleetRanking("stars", document.getElementById("opener"));
  const hall = document.getElementById("fleet-rank-hall");
  assert.equal(hall.dataset.rankSource, "live");
  assert.match(hall.textContent, /暂无上榜记录/);
  assert.equal(hall.querySelectorAll(".rank-row").length, 0);
  await click('[data-rank-action="demo"]');
  assert.equal(hall.dataset.rankSource, "demo"); assert.equal(reads, 1);
  store.api = async () => { reads++; throw new Error("暂离线"); };
  await click('[data-rank-action="source"]');
  assert.match(hall.textContent, /暂未取得正式榜/);
  assert.equal(hall.querySelectorAll(".rank-row").length, 0);
  await click('[data-rank-action="demo"]');
  window.closeFleetOfficial();
  assert.equal(hall.dataset.rankSource, "demo");
  assert.equal(hall.querySelector('[data-rank-action="source"]').hidden, true);
  assert.equal(reads, 2); assert.deepEqual(calls, []);
});

function upgradedGuild() {
  return {
    name: "测试母港", leader_id: 14, code: "0000000E", level: 3, xp: 250, next_xp: 450,
    member_limit: 20, coins: 160, role: "leader", can_assign: true, can_manage: true,
    boss_week: "2026-W48", attempts_today: 0, attempt_limit: 2,
    members: [{ user_id: 14, name: "会长", role: "leader" }, { user_id: 22, name: "<img src=x>", role: "member" }],
    raid_stages: [
      { stage: 1, name: "守卫", required_level: 1, hp: 900, max_hp: 24000, active: true, unlocked: true, participated: true, token: 40, material: "mat_sr", material_count: 3, coins: 60, gold: 250 },
      { stage: 2, name: "统领", required_level: 3, hp: 60000, max_hp: 60000, unlocked: true, token: 70, material: "mat_ssr", material_count: 2, coins: 90 },
      { stage: 3, name: "主宰", required_level: 5, hp: 120000, max_hp: 120000, unlocked: false, token: 110, material: "mat_ssr", material_count: 4, coins: 140 },
    ],
    shop: [{ id: "tokens", name: "代币补给", level: 1, cost: 80, resource: "token", amount: 40, limit: 3, remaining: 3, unlocked: true, can_buy: true }],
  };
}

test("guild tabs expose capacity, server shop eligibility and exact appointment intents", async context => {
  const guild = upgradedGuild();
  const { window, document, click, calls } = setup(context, guild);
  await window.renderFleet();
  assert.match(document.querySelector(".guild-hero").textContent, /2 \/ 20/);
  await click('[data-fleet-tab="shop"]');
  await click('[data-fleet-act="shop"]');
  assert.deepEqual(calls.at(-1), ["guild_shop", { item_id: "tokens" }]);
  assert.equal(guild.coins, 160, "no optimistic debit");
  await click('[data-fleet-tab="members"]');
  assert.equal(document.querySelector(".guild-member-list img"), null);
  await click('[data-fleet-act="role"][data-value="22:deputy"]');
  assert.deepEqual(calls.at(-1), ["guild_role", { opponent_id: 22, item_id: "deputy" }]);
  guild.can_assign = false; guild.can_manage = false; guild.role = "member";
  await window.fleetLoad();
  assert.equal(document.querySelector('[data-fleet-act="role"]'), null);
  assert.equal(document.querySelector('[data-fleet-act="listing"]'), null);
});

test("guild challenge starts server replay after confirmation and deduplicates rapid clicks", async context => {
  const { window, document, store, click } = setup(context, upgradedGuild());
  const started = [];
  store.serverNow = () => 5000;
  window.resumeBattle = async (...args) => started.push(args);
  await window.renderFleet();
  await click('[data-fleet-tab="raid"]');
  let release, posts = 0;
  store.mutate = () => { posts++; return new Promise(resolve => { release = resolve; }); };
  const waiting = click('[data-fleet-act="attack"]');
  await click('[data-fleet-act="attack"]');
  assert.equal(posts, 1);
  assert.equal(started.length, 0);
  const receipt = { stage: 1, stage_name: "守卫", damage: 50, report: { duration_ms: 1000, events: [] } };
  release({ guild_battle: receipt, drops: [] });
  await waiting;
  assert.equal(started.length, 1);
  assert.equal(started[0][0].guild_battle, receipt);
  assert.equal(started[0][0].result, receipt.report);
  assert.equal(started[0][0].ready_at, 6000);
  assert.equal(started[0][1], false);
  assert.equal(document.querySelectorAll(".guild-raid-card").length, 3);
});

test("a guild battle response from an old account cannot start playback", async context => {
  const { window, store, click } = setup(context, upgradedGuild());
  let release, started = 0;
  store.serverNow = () => 1;
  window.resumeBattle = async () => { started++; };
  await window.renderFleet(); await click('[data-fleet-tab="raid"]');
  store.mutate = () => new Promise(resolve => { release = resolve; });
  const waiting = click('[data-fleet-act="attack"]');
  store.sessionEpoch++;
  release({ guild_battle: { report: { duration_ms: 1000 } } });
  await waiting;
  assert.equal(started, 0);
});

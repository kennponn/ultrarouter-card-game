const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const { JSDOM } = require("jsdom");

const clone = value => JSON.parse(JSON.stringify(value));
function fixture() {
  const squad = [{ id: "c01", level: 40, star: 3 }];
  const player = { id: 2, rank: 50, best_rank: 40, name: "<对手>", guild_name: "<海风公会>", power: 12345, squad, defense: { guild: { name: "旧快照公会" } } };
  return {
    me: { id: 1, rank: 101, best_rank: 20, guild_name: "晨星", power: 5000, squad, cooldown_until: 0 },
    best_rank: 20, remaining: 5, limit: 5, total: 101, server_now: 1000, next_settlement: 10000000, next_reset: 20000000,
    opponents: [player], board: [player], history: [],
    rewards: [{ max_rank: 1, token: 300, materials: { mat_r: 120, mat_sr: 60, mat_ssr: 20 } }, { max_rank: 0, token: 50, materials: { mat_r: 30, mat_sr: 15, mat_ssr: 3 } }],
    best_rewards: [{ max_rank: 100, token: 200 }, { max_rank: 50, token: 400 }, { max_rank: 20, token: 600 }, { max_rank: 10, token: 1000 }, { max_rank: 3, token: 1500 }, { max_rank: 1, token: 2500 }].map(tier => ({ ...tier, claimed: false, can_claim: tier.max_rank >= 20, delivered_at: 0 })),
    mail: [{ day: "2026-11-27", kind: "daily", rank: 101, token: 50, materials: { mat_r: 30, mat_sr: 15, mat_ssr: 3 }, settled_at: 1000, delivered_at: 1010, claimed: false }],
  };
}
function harness(context) {
  const dom = new JSDOM('<body data-page="page-arena"><div id="arena-summary"></div><div id="arena-body"></div><div id="modal-box"></div></body>', { runScripts: "outside-only" });
  const w = dom.window, calls = [], shown = [], watched = [], notices = [], reads = [];
  context.after(() => w.close());
  w.arenaTab = "ladder"; w.arenaQueueVersion = 0; w.arenaPollTimer = 0;
  w.lastArenaResult = null; w.lastArenaEpoch = 0;
  w.SAVE = { squad: ["c01"], power: 9999, token: 12, gold: 55, mailbox: { messages: [] } };
  w.SITE_ACCOUNT = { name: "旅人" };
  w.SITE = { ep: { ladder: "/ladder", ladderReplay: "/ladder/replay" } };
  w.GIRLS = [{ id: "c01", name: "伙伴", img: "portrait.webp" }];
  w.MATS = { mat_r: { name: "微光结晶" }, mat_sr: { name: "元素结晶" }, mat_ssr: { name: "星屑" } };
  w.esc = value => String(value).replaceAll("&", "&amp;").replaceAll("<", "&lt;").replaceAll('"', "&quot;").replaceAll("'", "&#39;");
  w.requireGame = () => w.Store.ready();
  w.arenaSquadHTML = () => '<span class="fixture-squad">当前阵容</span>';
  w.arenaOwnSquad = () => [];
  w.showModal = html => { shown.push(html); w.document.getElementById("modal-box").innerHTML = html; };
  w.closeModal = () => { w.document.getElementById("modal-box").innerHTML = ""; };
  w.showArenaResult = result => shown.push(result);
  w.watchArenaResult = () => { watched.push(w.lastArenaResult); w.document.body.dataset.page = "page-battle"; };
  w.renderTopbar = () => {};
  w.renderArena = async () => { reads.push("render-arena"); };
  w.toast = message => notices.push(message);
  w.openHarborMail = () => { w.document.body.dataset.page = "page-mail"; };
  w.setTimeout = () => 0;
  w.Store = { sessionEpoch: 4, authUserId: 1, ready: () => true, serverNow: () => 1000,
    refresh: async () => { reads.push("profile"); return {}; },
    api: async () => { reads.push("ladder"); return fixture(); },
    mutate: async (action, fields) => { calls.push({ action, ...clone(fields) }); return {}; } };
  w.eval(fs.readFileSync(path.join(__dirname, "../js/ladder.js"), "utf8") + `
    window.__ladder = { paintLadder, paintLadderBoard, paintLadderRewards, paintLadderSummary,
      previewLadderOpponent, submitLadderChallenge, ladderAction, claimLadderBest, replayLadder, renderLadder,
      setData(value) { ladderData = value; ladderSampledAt = performance.now(); },
      get data() { return ladderData; }
    };
  `);
  const api = w.__ladder; api.setData(fixture());
  return { w, api, document: w.document, calls, shown, watched, notices, reads };
}

test("historical tiers offer server-approved direct collection while daily rewards remain in the inbox", context => {
  const h = harness(context);
  h.api.paintLadderRewards();
  assert.equal(h.document.querySelectorAll(".ladder-best-tier").length, 6);
  assert.equal(h.document.querySelectorAll(".ladder-best-tier.is-reached").length, 3);
  assert.match(h.document.getElementById("arena-body").textContent, /2500/);
  assert.match(h.document.querySelector(".ladder-delivery-state").textContent, /已寄至邮箱/);
  assert.ok(h.document.querySelector('[onclick="openHarborMail()"]'));
  assert.equal(h.document.querySelectorAll('[data-ladder-best]:not(:disabled)').length, 3);
  assert.match(h.document.querySelector('[data-ladder-best="100"]').textContent, /领取代币/);
  assert.equal(h.document.querySelector('[onclick*="claimLadderMail"], [onclick*="ladder_claim"]'), null);
  assert.equal(h.w.SAVE.token, 12, "displaying entitlements cannot award local tokens");
  h.api.data.me = null;
  h.api.paintLadderRewards(); h.api.paintLadderSummary();
  assert.match(h.document.getElementById("arena-summary").textContent, /#20/, "retired players retain the server's historical best");
  assert.equal(h.document.querySelectorAll(".ladder-best-tier.is-reached").length, 3);
  assert.equal(h.document.querySelectorAll('[data-ladder-best]:not(:disabled)').length, 3);
});

test("direct tier collection sends only its exact key and waits for the server before marking claimed", async context => {
  const h = harness(context);
  let resolve;
  h.w.Store.mutate = (action, fields) => { h.calls.push({ action, ...clone(fields) }); return new Promise(done => { resolve = done; }); };
  h.api.paintLadderRewards();
  const before = clone(h.w.SAVE);
  const pending = h.api.claimLadderBest(100, 4);
  await h.api.claimLadderBest(100, 4);
  assert.deepEqual(h.calls, [{ action: "ladder_best_claim", item_id: "best:100" }]);
  assert.equal(h.document.querySelector('[data-ladder-best="100"]').disabled, true);
  assert.equal(h.api.data.best_rewards[0].claimed, false);
  assert.deepEqual(clone(h.w.SAVE), before);
  resolve({ drops: ["历史前100名 · 代币 ×200"] }); await pending;
  assert.equal(h.api.data.best_rewards[0].claimed, true);
  assert.equal(h.api.data.best_rewards[0].can_claim, false);
  assert.match(h.document.querySelector('[data-ladder-best="100"]').textContent, /已领取/);
  assert.equal(h.document.querySelector('[data-ladder-best="100"]').disabled, true);
  assert.match(h.notices.at(-1), /代币 ×200/);
  await h.api.claimLadderBest(100, 4);
  assert.equal(h.calls.length, 1);
});

test("denied, legacy, forged and stale historical rewards cannot send a claim", async context => {
  const h = harness(context);
  h.api.data.best_rewards[0].delivered_at = 999;
  h.api.data.best_rewards[1].claimed = true;
  h.api.data.best_rewards[2].can_claim = false;
  h.api.paintLadderRewards();
  assert.match(h.document.querySelector('.ladder-best-tier').textContent, /查看原奖励邮件/);
  assert.equal(h.document.querySelectorAll('[data-ladder-best]:not(:disabled)').length, 0);
  for (const rank of [100, 50, 20, 10, 2, "100"]) await h.api.claimLadderBest(rank, 4);
  h.api.data.best_rewards[2].can_claim = true;
  await h.api.claimLadderBest(20, 3);
  assert.deepEqual(h.calls, []);
  assert.equal(h.w.SAVE.token, 12);
});

test("failed collection remains retryable and stale responses cannot mark a new account's tiers", async context => {
  const h = harness(context);
  h.api.paintLadderRewards();
  h.w.Store.mutate = async () => null;
  await h.api.claimLadderBest(100, 4);
  assert.equal(h.api.data.best_rewards[0].claimed, false);
  assert.equal(h.document.querySelector('[data-ladder-best="100"]').disabled, false);
  let resolve;
  h.w.Store.mutate = () => new Promise(done => { resolve = done; });
  const pending = h.api.claimLadderBest(100, 4);
  h.w.Store.sessionEpoch++;
  h.api.setData(fixture());
  resolve({}); await pending;
  assert.equal(h.api.data.best_rewards[0].claimed, false);
  assert.deepEqual(h.notices, []);
});

test("opponents, board and previews show escaped live guild names", context => {
  const h = harness(context);
  h.api.paintLadder();
  assert.equal(h.document.querySelector(".ladder-opponent .ladder-guild").textContent, "<海风公会>");
  assert.equal(h.document.querySelector(".ladder-opponent .ladder-guild").children.length, 0);
  assert.doesNotMatch(h.document.getElementById("arena-body").textContent, /旧快照公会/);
  h.api.paintLadderBoard();
  assert.equal(h.document.querySelector(".arena-player .ladder-guild").textContent, "<海风公会>");
  h.api.previewLadderOpponent(2, true);
  assert.equal(h.document.querySelector("#modal-box .ladder-guild").textContent, "<海风公会>");
  h.api.data.opponents[0].guild_name = "";
  h.api.paintLadder();
  assert.equal(h.document.querySelector(".ladder-opponent .ladder-guild").textContent, "未加入公会");
});

test("confirming a challenge enters battle once before any result dialog or rank refresh", async context => {
  const h = harness(context);
  let resolve;
  h.w.Store.mutate = (action, fields) => { h.calls.push({ action, ...clone(fields) }); return new Promise(done => { resolve = done; }); };
  h.api.paintLadder(); h.api.previewLadderOpponent(2, true);
  const shownBefore = h.shown.length, before = clone(h.w.SAVE);
  const pending = h.api.submitLadderChallenge();
  await h.api.submitLadderChallenge();
  assert.deepEqual(h.calls, [{ action: "ladder_challenge", opponent_id: 2, count: 50 }]);
  assert.equal(h.document.querySelector("[data-ladder-submit]").disabled, true);
  assert.deepEqual(clone(h.w.SAVE), before);
  const result = { ladder: true, win: true, own_before: 101, own_after: 50, report: { duration_ms: 8000 } };
  resolve({ ladder: result }); await pending;
  assert.equal(h.document.body.dataset.page, "page-battle");
  assert.equal(h.shown.length, shownBefore, "no victory dialog is shown before playback");
  assert.deepEqual(h.watched, [result]);
  assert.equal(h.w.lastArenaEpoch, 4);
  assert.deepEqual(h.reads, [], "the rank page is not refreshed to reveal the result first");
});

test("failed and stale challenge responses do not open a battle; history still opens the replay result", async context => {
  const h = harness(context);
  h.api.previewLadderOpponent(2, true);
  h.w.Store.mutate = async () => null;
  await h.api.submitLadderChallenge();
  assert.equal(h.watched.length, 0);
  assert.equal(h.document.querySelector("[data-ladder-submit]").disabled, false);
  let resolve;
  h.w.Store.mutate = () => new Promise(done => { resolve = done; });
  const pending = h.api.submitLadderChallenge();
  h.w.Store.sessionEpoch++;
  resolve({ ladder: { win: true } }); await pending;
  assert.equal(h.watched.length, 0);
  const historical = { id: "past", ladder: true, win: false };
  h.w.Store.api = async () => historical;
  await h.api.replayLadder("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa");
  assert.equal(h.shown.at(-1), historical);
});

test("ranked refresh reads the newly delivered public mailbox after settlement", async context => {
  const h = harness(context);
  await h.api.renderLadder();
  assert.deepEqual(h.reads, ["ladder", "profile"]);
  assert.equal(h.api.data.rewards[0].token, 300);
});

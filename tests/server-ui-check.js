const fs = require("node:fs");
const path = require("node:path");
const assert = require("node:assert/strict");
const { JSDOM } = require("jsdom");

const root = path.resolve(__dirname, "..");
const catalog = JSON.parse(fs.readFileSync(path.join(root, "backend/model/gacha_catalog.json")));
const market = JSON.parse(fs.readFileSync(path.join(root, "backend/model/gacha_market.json")));
const clone = value => JSON.parse(JSON.stringify(value));
const server = {
  revision: 2, active: null, rejectFinish: false, calls: [], quota: 2500000,
  save: {
    stamina: 120, staminaMax: 120, staminaTs: Date.now(), token: 0, gold: 600,
    multi: 1, multiOn: false, daily: { date: "2026-11-17", tokenGained: 0 },
    market: {slot:Date.now()-1000, next:Date.now()+3600000, bought:{}},
    pity: 0, totalPull: 0, squad: ["c08", "c06", "c01", "c12"],
    girls: Object.fromEntries(["c08", "c06", "c01", "c12"].map(id => [id, { star: 1, level: 1, exp: 0, shards: 0 }])),
    cleared: { s1: true }, stars: { s1: 3 }, history: [], mats: { mat_r: 0, mat_sr: 0, mat_ssr: 0 },
  },
};
server.save.girls.c14 = { star: 0, level: 0, exp: 0, shards: 2 };
const rival = { id: -80, rank: 80, name: '<img src=x onerror="bad()">', power: 4000, squad: [{ id: "c06", level: 10, star: 2 }] };
server.ladder = { me: null, remaining: 5, limit: 5, total: 100, opponents: [rival], board: [rival], history: [], mail: [], rewards: [{ max_rank: 100, materials: { mat_r: 40, mat_sr: 20, mat_ssr: 5 } }] };
const errors = [];
const dom = new JSDOM(fs.readFileSync(path.join(root, "index.html"), "utf8"), {
  runScripts: "outside-only", url: "https://game.test/gacha/", pretendToBeVisual: true,
});
const w = dom.window;
w.addEventListener("error", event => errors.push(event.message));
const snapshot = () => ({ save: clone(server.save), revision: server.revision });
w.fetch = async (url, options) => {
  const ep = new URL(url).pathname;
  const body = options.body ? JSON.parse(options.body) : null;
  server.calls.push({ ep, body });
  let data;
  if (ep.endsWith("/auth/refresh")) data = {
    access_token: "test-only", token_type: "Bearer", access_expires_at: Date.now() / 1000 + 3600,
    user: { id: 7 }, session: { sid: "test-session", current: true },
  };
  else if (ep.endsWith("/profile")) data = {
    ...snapshot(), user_id: 7, name: "UI test", quota: server.quota, stored: 0, active: null,
    today: { runs: 21, run_limit: null, quota: 8000000, quota_limit: null }, active_run: server.active,
    stage_rewards: { s1: { quota_min: 75000, quota_max: 110000 } },
    market,
  };
  else if (ep.endsWith("/action")) {
    if (body.action === "ladder_join") server.ladder.me = { ...rival, id: 7, rank: 101, best_rank: 101, cooldown_until: 0 };
    if (body.action === "ladder_leave") server.ladder.me = null;
    if (body.action === "ladder_challenge") {
      server.ladder.me.rank = server.ladder.me.best_rank = 80;
      server.ladder.remaining--;
      server.ladderResult = { id: "00000000-0000-4000-8000-000000000080", ladder: true, win: true, name: rival.name, own_before: 101, own_after: 80, attacker_name: "UI test", defender_name: rival.name, report: report(true) };
    }
    if (body.action === "level") { server.save.gold -= 35; server.save.girls[body.girl_id].level++; }
    if (body.action === "multi") server.save.multiOn = body.enabled;
    if (body.action === "squad") server.save.squad = body.squad;
    if (body.action === "shop") {
      const offer = market.offers.find(item => item.id === body.item_id);
      if (offer.shop === "token") server.save.token -= offer.price;
      else server.save.mats[offer.shop+"_coin"] -= offer.price;
      if (offer.resource === "gold") server.save.gold += offer.amount;
      else server.save.mats[offer.resource] = (server.save.mats[offer.resource] || 0) + offer.amount;
      server.save.market.bought[offer.id] = (server.save.market.bought[offer.id] || 0) + 1;
    }
    if (body.action === "shop_refresh") {
      server.save.token -= market.refresh_price;
      market.offers.filter(item => item.shop === body.item_id).forEach(item => delete server.save.market.bought[item.id]);
    }
    if (body.action === "use_stamina_pack") {
      server.save.mats[body.item_id]--;
      server.save.stamina += market.packs[body.item_id];
    }
    if (body.action === "arena_join") {
      server.queue = { id: "00000000-0000-4000-8000-000000000001", status: "waiting", stake_e: body.stake_e, power: 10000, min_power: 8334, max_power: 12000, expire_at: Date.now()+120000, quota: 5000000-body.stake_e*500000 };
    }
    if (body.action === "arena_cancel") {
      server.queue = { ...server.queue, status: "cancelled", quota: 5000000 };
    }
    server.revision++;
    data = snapshot();
    if (body.action === "ladder_challenge") data.ladder = clone(server.ladderResult);
    if (body.action.startsWith("arena_")) data.queue = clone(server.queue);
    if (body.action === "room_join") {
      server.room = { ...server.rooms.find(r => r.id === body.room_id), guest_id: 7, guest_name: "UI test", guest_power: 4567, host_ready: false, guest_ready: false };
    }
    if (body.action === "room_ready") { server.room.guest_ready = true; server.roomQuota -= server.room.stake_e * 500000; }
    if (body.action === "room_leave") { server.room.status = "cancelled"; if (server.room.guest_ready) server.roomQuota += server.room.stake_e * 500000; }
    if (body.action.startsWith("room_")) data.room = clone(server.room);
  } else if (ep.endsWith("/ladder")) {
    data = { ...clone(server.ladder), server_now: Date.now(), next_settlement: Date.now() + 3600000, next_reset: Date.now() + 7200000 };
  } else if (ep.endsWith("/social")) {
    const opponent = { user_id: 9, name: '<img src=x onerror="bad()">', power: 4000, arena_points: 40, squad: [{ id: "c06", level: 10, star: 2 }] };
    data = { arena_enabled: server.arenaEnabled !== false, power_board: [opponent], arena_board: [opponent], opponents: [opponent], power_rank: 2, arena_rank: 0, arena_limit: 5 };
  } else if (ep.endsWith("/arena/queue")) {
    data = clone(server.queue || { status: "idle", quota: 5000000 });
  } else if (ep.endsWith("/arena/rooms")) {
    data = { rooms: clone(server.rooms), room: clone(server.room || null), quota: server.roomQuota, queued: false };
  } else if (ep.endsWith("/arena/replay")) {
    data = clone(server.arenaResult);
  } else if (ep.endsWith("/start")) {
    server.save.stamina -= catalog.STAGES.find(stage => stage.id === body.stage).cost; server.revision++;
    data = { ...snapshot(), id: "run_ui", stage: "s1", ready_at: Date.now() - 1,
      expire_at: Date.now() + 600000, result: report(false) };
    server.active = clone(data);
  } else if (ep.endsWith("/finish")) {
    if (server.rejectFinish) throw new Error("mock network interruption");
    if (!body.abandon) {
      server.save.gold += 123; server.save.token += 7; server.save.stars.s1 = 2;
    }
    server.revision++; server.active = null;
    data = { ...snapshot(), win: !body.abandon, stars: body.abandon ? 0 : 2,
      alive_n: 3, total_n: 4, fast: false, rounds: 22, is_best: false,
      drops: body.abandon ? [] : ["gold x123", "<untrusted>"], quota: 0, card: null };
  } else throw new Error("unexpected endpoint " + ep);
  return { ok: true, status: 200, json: async () => ({ success: true, data }) };
};

function report(live) {
  const allies = server.save.squad.map((id, i) => {
    const girl = catalog.GIRLS.find(g => g.id === id);
    return { uid: "a" + i, side: "ally", girlId: id, name: girl.name, img: girl.img,
      atkType: girl.atkType, maxHp: 100, hp: 100, rage: 0, alive: true, atk: 10, def: 5 };
  });
  const enemy = { uid: "e0", side: "enemy", name: "test enemy", img: "assets/gen/enemy-wolf.webp",
    maxHp: 200, hp: 200, rage: 0, alive: true, atkType: "melee" };
  return { allies, waves: [[enemy]], win: false, stars: 0, rounds: 999, duration_ms: live ? 500 : 30000,
    events: [
      { kind: "wave", wave: 0, round: 0, duration_ms: 0, units: [] },
      { kind: "action", wave: 0, round: 1, src: "a0", duration_ms: live ? 450 : 0,
        targets: [{ uid: "e0", damage: 10, absorbed: 3, crit: true }],
        units: [{ uid: "e0", hp: 190, rage: 0, shield: 0, alive: true }] },
    ] };
}

w.eval([...w.document.querySelectorAll('script[src^="js/"]')].map(script =>
  fs.readFileSync(path.join(root, script.getAttribute("src").split("?")[0]), "utf8")).join("\n;\n") + `
;window.G = {Store, nav, startBattle, resumeBattle, recoverBattle, exitBattle, battleEnd,
  openGirl, levelUp, toggleSquad, closeModal, renderSquad, playServerBattle,
  get save(){return SAVE}, get battle(){return BAT}, get running(){return battleRunning},
  setBattle(value){BAT=value; battleRunning=true;},
  speedUp(){sleep=async()=>{};}, lockGame};`);
const G = w.G;
const tick = () => new Promise(resolve => setTimeout(resolve, 0));
let checks = 0;
function check(condition, label) { assert.ok(condition, label); checks++; console.log("PASS " + label); }

(async () => {
  await G.Store.profilePromise;
  check(G.Store.ready(), "valid site session unlocks server save");
  check(w.document.getElementById("title-screen").classList.contains("hidden"), "authenticated gate is hidden");
  for (const page of ["home", "gacha", "squad", "codex", "stage", "growth", "bag"]) {
    G.nav("page-" + page);
    check(w.document.body.dataset.page === "page-" + page, "renders " + page);
  }
  G.nav("page-home");
  check(w.document.getElementById("page-home").textContent.includes("今日产出 16.0000 E · 不设每日上限 · 今日副本 21 次（不限次数）"), "renders actual earnings and runs beyond both former daily caps");
  G.nav("page-stage");
  w.pickStage("s1");
  check(w.document.getElementById("stage-detail").textContent.includes("0.15～0.22 E"), "stage reward preview comes from the server profile");
  check(catalog.STAGES.filter(stage => /^s\d+$/.test(stage.id)).length === 12, "all 12 main-story stages remain available alongside additional modes");
  check(w.document.getElementById("stage-detail").textContent.includes("体力 " + catalog.STAGES.find(stage => stage.id === "s1").cost), "selected stage shows its current server catalog stamina cost");
  const rewardNote = w.document.querySelector(".sd-reward-note").textContent;
  check(rewardNote.includes("三星 +20%") && rewardNote.includes("余额产出不设每日上限") && rewardNote.includes("体力"), "stage preview explains star bonuses, unlimited earnings, and stamina cost");
  const chapters = [...w.document.querySelectorAll("#chapter-bar .ch-tab")];
  check(chapters.length === 6, "four main-story and two rift chapters are accessible from the chapter bar");
  w.pickChapter(2);
  check(w.document.querySelectorAll("#node-track .node.locked").length === 3, "new stages remain locked without server clearance");
  check(w.document.querySelector("#stage-detail button").disabled, "locked new stages cannot start from the UI");
  G.save.cleared.s6 = true;
  w.pickChapter(2);
  check(!w.document.querySelector("#node-track .node").classList.contains("locked"), "clearing the prior chapter unlocks the next chapter's first stage");
  for (const [chapter, names] of [[2, ["夜潮栈道", "潮汐神殿", "月影王庭"]], [3, ["逐星长阶", "观星禁域", "星穹圣座"]]]) {
    w.pickChapter(chapter);
    check([...w.document.querySelectorAll("#node-track .nm")].map(el => el.textContent).join() === names.join(), "chapter " + (chapter + 1) + " shows its three new stages");
  }
  w.pickChapter(4);
  check(w.document.querySelectorAll("#node-track .node").length === 6, "elemental rift chapter shows its six separate stages");
  w.pickChapter(5);
  check(w.document.querySelectorAll("#node-track .node.locked").length === 6, "nightmare rifts remain locked before their corresponding rifts are cleared");
  G.nav("page-squad");
  check(!w.document.querySelector('#owned-list [data-id="c14"]'), "shard-only heroes do not appear in the formation roster");
  check(w.document.querySelectorAll("#owned-list .card").length === 4, "formation roster only contains acquired heroes");
  G.openGirl("c14");
  check(w.document.body.dataset.page === "page-codex", "unacquired hero details open in the codex");
  check(w.document.querySelector('#codex-grid [data-codex-id="c14"]').dataset.status === "shards", "codex shows the shard-only hero's actual acquisition state");
  check(!w.document.querySelector('#codex-detail [data-codex-action="workshop"],#codex-detail [data-codex-action="squad"]'), "unacquired codex hero cannot train or join a squad");
  G.closeModal();
  const beforeGrowth = server.calls.length;
  await G.levelUp("c08");
  check(w.document.body.dataset.page === "page-growth" && G.save.girls.c08.level === 1 && G.save.gold === 600, "upgrade opens progression without buying levels or changing server resources");
  check(server.calls.slice(beforeGrowth).every(call => call.body?.action !== "level"), "opening progression never sends a gold-for-level intent");
  G.closeModal();
  G.speedUp();
  server.rejectFinish = true;
  await G.startBattle("s1");
  check(G.battle && G.running, "unconfirmed settlement retains active battle");
  check(w.document.getElementById("modal-box").textContent.includes("重试结算"), "failed settlement has retry control");
  check(G.save.gold === 600 && G.save.token === 0, "no client rewards before server settlement");
  check(G.battle.waves[0][0].hp === 190, "playback uses server HP snapshot");
  server.rejectFinish = false;
  await G.battleEnd(G.battle);
  check(G.save.gold === 723 && G.save.token === 7, "settlement applies server rewards exactly once");
  check(G.save.stars.s1 === 2, "server result overrides client report result");
  check(w.document.querySelectorAll("#modal-box .res-stars .on").length === 2, "result displays server stars");
  check(!w.document.querySelector("#modal-box untrusted"), "reward text is escaped");
  const finishes = server.calls.filter(c => c.ep.endsWith("/finish"));
  check(finishes.every(c => Object.keys(c.body).join() === "run_id"), "settlement never sends stars or save");
  const gold = G.save.gold;
  await G.battleEnd(G.battle);
  check(G.save.gold === gold, "result UI cannot repeat settlement");
  await tick();
  G.closeModal();

  // Direct playback verifies the non-skipped animation path without local stats.
  const run = { id: "run_animation", stage: "s1", result: report(true), ready_at: Date.now() + 500, expire_at: Date.now() + 60000 };
  server.active = run;
  G.Store.runSessions.set(run.id, G.Store.sessionId);
  server.rejectFinish = true;
  await G.resumeBattle(run);
  check(w.document.querySelector("#u-e0 .hp-fill").style.width === "95%", "animated action merges authoritative HP");
  check(w.document.querySelectorAll(".float-text").length > 0, "animated report emits hit feedback");
  server.rejectFinish = false;
  await G.exitBattle();
  check(G.battle === null && !G.running, "server-confirmed retreat clears battle");
  const abandon = server.calls.filter(c => c.ep.endsWith("/finish")).at(-1).body;
  check(abandon.abandon === true && Object.keys(abandon).length === 2, "retreat sends only run ID and abandon intent");

  server.save.token = 100;
  server.save.stamina = 120;
  server.save.power = 4567;
  server.revision++;
  await G.Store.refresh();
  G.nav("page-shop"); await tick();
  check(w.document.querySelectorAll(".shop-item").length === market.offers.filter(item => item.shop === "token").length, "shop renders products and prices from server");
  check(w.document.querySelector(".shop-stamina button").disabled, "shop blocks a stamina purchase that would overflow");
  w.confirmShopItem("gold");
  check(w.document.getElementById("modal-box").textContent.includes("30 代币"), "shop confirmation shows the server price");
  await w.buyShopItem("gold"); await tick();
  const purchase = server.calls.filter(c => c.body?.action === "shop").at(-1).body;
  check(Object.keys(purchase).sort().join() === "action,item_id,request_id", "shop sends item intent without price, reward or save claims");
  check(G.save.token === 70 && G.save.market.bought.gold === 1, "shop uses returned personal shelf stock");
  server.save.market.bought = {};
  server.revision++;
  G.nav("page-shop"); await tick();
  check(w.document.querySelector(".shop-gold .shop-stock").textContent.includes("2 / 2"), "reopening shop synchronizes replenished item stock");
  check(!w.document.getElementById("token-exchange-submit") && !w.exchangeTokens, "the shop has no site balance exchange");
  server.save.market.bought = {gold: 2}; server.save.mats.arena_coin = 64; server.revision++;
  G.nav("page-shop"); await tick();
  check(w.document.querySelector(".shop-gold button").disabled && !w.document.querySelector(".shop-mat_r button").disabled, "sold-out items do not block other personal stock");
  w.openGameMarket("arena"); await tick();
  w.confirmShopItem("arena_pack_30");
  check(w.document.getElementById("modal-box").textContent.includes("32 竞技场币"), "arena shop quotes its own currency");
  await w.buyShopItem("arena_pack_30"); await tick();
  check(G.save.mats.arena_coin === 32 && G.save.token === 70 && G.save.mats.stamina_pack_30 === 1, "arena purchase grants a stored stamina pack, not immediate energy");
  await w.openStaminaPack("stamina_pack_30", G.Store.sessionEpoch);
  check(G.save.stamina === 150 && G.save.mats.stamina_pack_30 === 0, "opening the pack credits the exact stamina amount");
  w.confirmMarketRefresh("arena");
  check(w.document.getElementById("modal-box").textContent.includes("45 代币"), "paid shelf refresh shows the cost first");
  await w.refreshMarketStock("arena", G.save.market.slot, G.Store.sessionEpoch);
  check(G.save.token === 25 && !G.save.market.bought.arena_pack_30 && G.save.market.bought.gold === 2, "refresh consumes tokens only for the selected market");
  G.nav("page-arena"); await tick();
  check(w.document.querySelector('[data-arena-tab="ladder"]').getAttribute("aria-pressed") === "true", "arena navigation opens the ladder");
  check(w.document.querySelector('#page-home [data-arena-mode="queue"]').textContent.includes("匹配对战"), "home has an explicit matchmaking entrance");
  const refresh = G.Store.refresh;
  G.Store.refresh = async () => null;
  await w.renderArena();
  check(w.document.querySelector('#arena-body button').textContent.includes("重新读取"), "a current transient profile failure offers retry");
  G.Store.refresh = refresh;
  await w.renderArena();
  check(!!w.document.querySelector('.ladder-welcome'), "retry restores the ladder");
  await w.ladderAction('ladder_join');
  check(w.document.querySelectorAll('.ladder-opponent').length === 1 && !w.document.querySelector('.ladder-opponent h4 img'), "server opponent is visible and its name is escaped");
  w.previewLadderOpponent(-80, true);
  await w.submitLadderChallenge();
  const rankedIntent = server.calls.filter(call => call.body?.action === 'ladder_challenge').at(-1).body;
  check(Object.keys(rankedIntent).sort().join() === 'action,count,opponent_id,request_id', "challenge does not send a result or a balance stake");
  check(w.document.body.dataset.page === 'page-battle' && G.battle.arena?.ladder, "confirmed ranked challenge enters battle directly");
  check(!w.document.querySelector('.arena-award'), "rank change stays hidden until the battle presentation ends");
  w.skipBattlePlayback(); await tick();
  check(w.document.querySelector('.arena-award').textContent.includes('#101 → #80'), "completed ranked playback displays the server-confirmed rank change");
  w.leaveArenaReplay(); await tick();
  const mutateBeforeStale = G.Store.mutate;
  G.Store.mutate = async (...args) => { const result = await mutateBeforeStale.apply(G.Store, args); G.Store.sessionEpoch++; return result; };
  await w.ladderAction('ladder_challenge', { opponent_id: -80, count: 80 });
  G.Store.mutate = mutateBeforeStale;
  check(w.document.getElementById('modal-mask').classList.contains('hidden') && w.document.body.dataset.page === 'page-arena', "a session change during a ranked request discards the earlier account's result");
  await w.ladderAction('ladder_leave');
  check(!!w.document.querySelector('.ladder-welcome'), "withdrawal reopens the opt-in screen");
  w.pickArenaTab("power"); await tick(); await tick();
  check(w.document.getElementById("arena-summary").textContent.includes("4,567"), "arena displays server calculated squad power");
  check(!w.document.querySelector(".arena-player strong img"), "rankings escape player names");
  w.previewOpponent(9);
  check(w.document.querySelector(".arena-preview img").alt === "铃羽", "challenge preview shows the opponent's saved characters");
  check(!w.document.querySelector('[onclick^="challengeArena"]'), "rankings cannot attack a player directly");
  G.closeModal();
  w.pickArenaTab("queue"); await tick(); await tick();
  check(w.document.querySelectorAll(".arena-stakes button").length === 5, "queue offers the five authorized stake tiers");
  w.setArenaStake(5); w.confirmArenaQueue();
  check(w.document.getElementById("modal-box").textContent.includes("失败损失自己 5 E"), "joining requires an explicit loss disclosure");
  await w.joinArenaQueue(5); await tick();
  const join = server.calls.filter(c => c.body?.action === "arena_join").at(-1).body;
  check(Object.keys(join).sort().join() === "action,request_id,stake_e", "queue accepts stake intent only, no target or winner");
  check(w.document.querySelector(".arena-wait").textContent.includes("已预留 5 E"), "waiting view exposes the actual server escrow");
  check(w.document.querySelector(".arena-wait").textContent.includes("8,334～12,000"), "waiting view displays the frozen server power range");
  await w.cancelArenaQueue(); await tick();
  const cancel = server.calls.filter(c => c.body?.action === "arena_cancel").at(-1).body;
  check(cancel.queue_id === server.queue.id && !cancel.opponent_id, "cancel is bound to the current server queue ticket");
  check(w.document.querySelector(".arena-refunded").textContent.includes("5 E 已退回"), "cancel renders confirmed server refund");
  server.arenaResult = { id: "00000000-0000-4000-8000-000000000002", opponent_id: 9, name: '<img src=x onerror="bad()">', win: true, draw: false, stake_e: 5, delta_quota: 2500000, played_at: Date.now(), report: report(true) };
  server.arenaResult.report.duration_ms=1; server.arenaResult.report.events=[];
  server.save.arena={wins:1,played:1,history:[{...server.arenaResult,report:undefined}]};
  server.save.daily.arenaPlayed=1;
  server.queue={...server.queue,status:"matched",quota:7500000,match:server.arenaResult};
  await w.refreshArenaQueue();
  await w.replayArena(server.arenaResult.id);
  check(w.document.getElementById("modal-box").textContent.includes("+5 E"), "results show server-settled E rather than points");
  check(!w.document.querySelector(".social-confirm h4 img"), "arena results escape opponent names");
  const finishCount = server.calls.filter(c => c.ep.endsWith("/finish")).length;
  w.watchArenaResult(); await tick(); await tick();
  check(G.battle.arena && G.battle.settled, "arena replay completes without a dungeon settlement");
  check(server.calls.filter(c => c.ep.endsWith("/finish")).length === finishCount, "replay cannot debit another stake");
  w.leaveArenaReplay(); await tick();
  w.pickArenaTab("history"); await tick();
  check(w.document.querySelectorAll(".arena-history").length === 1, "arena history survives returned server state");
  await w.replayArena(server.arenaResult.id);
  check(server.calls.at(-1).ep.endsWith("/arena/replay"), "saved battle history reads the private replay endpoint");

  G.closeModal();
  server.roomQuota = 5000000;
  server.rooms = [
    {id: "00000000-0000-4000-8000-000000000003", status:"open", stake_e:3, host_id:9, host_name:'<img src=x onerror="bad()">', host_power:4500, guest_id:0, expire_at:Date.now()+300000},
    {id: "00000000-0000-4000-8000-000000000004", status:"open", stake_e:5, host_id:10, host_name:"Too strong", host_power:30000, guest_id:0, expire_at:Date.now()+300000},
  ];
  w.pickArenaTab("rooms"); await tick(); await tick();
  check(w.document.querySelectorAll(".room-row").length === 2, "public lobby lists rooms without asking for a room code");
  check(!w.document.querySelector(".room-row h4 img"), "room host names are escaped");
  check(w.document.querySelectorAll(".room-row button")[1].disabled, "lobby disables a room outside the 20 percent power range");
  const roomNode = w.document.querySelector(".room-lobby");
  await w.refreshArenaRooms();
  check(w.document.querySelector(".room-lobby") === roomNode, "unchanged lobby polling preserves the rendered page");
  w.confirmJoinRoom(server.rooms[0].id);
  check(w.document.getElementById("modal-box").textContent.includes("加入不扣款"), "room join clearly separates entry from readiness consent");
  await w.roomAction("room_join", server.rooms[0].id);
  check(w.document.querySelector(".room-current").textContent.includes("未准备 · 未扣款"), "joining a room does not imply readiness or debit");
  w.confirmRoomReady();
  check(w.document.getElementById("modal-box").textContent.includes("失败 −3 E"), "ready confirmation discloses the selected room's exact possible loss");
  await w.roomAction("room_ready", server.room.id);
  check(w.document.querySelector(".room-lobby .social-wallet").textContent.includes("7.0000 E"), "ready view uses the server's available wallet balance");
  check(w.document.querySelector(".room-current").textContent.includes("已准备 · 已预留 3 E"), "only server-confirmed readiness is shown");
  const roomIntent = server.calls.filter(c => c.body?.action === "room_ready").at(-1).body;
  check(Object.keys(roomIntent).sort().join() === "action,request_id,room_id", "room ready sends only room ID and idempotency key, no wallet or outcome");
  await w.roomAction("room_leave", server.room.id);
  check(w.document.querySelector(".room-lobby .social-wallet").textContent.includes("10.0000 E"), "room leave displays the confirmed refund");

  server.arenaEnabled = false;
  G.nav("page-arena"); await tick();
  w.pickArenaTab("queue"); await tick(); await tick();
  check(w.document.querySelector(".arena-join").disabled, "closed queue cannot join but remains visible for refunds");
  w.pickArenaTab("power"); await tick();
  w.previewOpponent(9);
  check(w.document.getElementById("modal-box").textContent.includes("双方主动匹配") && !w.document.querySelector('[onclick="challengeArena(9)"]'), "rankings always remain read-only");
  G.Store.clearSession();
  G.nav("page-gacha");
  check(!G.Store.ready() && G.save === null, "logout clears in-memory server snapshot");
  check(w.document.getElementById("app").classList.contains("hidden"), "logout blocks every game page");
  check(errors.length === 0, "no page or battle runtime errors: " + errors.join("; "));
  console.log(`${checks} server UI checks passed`);
  dom.window.close();
})().catch(error => { console.error(error); dom.window.close(); process.exitCode = 1; });

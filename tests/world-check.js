const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const {JSDOM} = require("jsdom");

const read = name => fs.readFileSync(path.resolve(__dirname, "..", name), "utf8");
const tick = () => new Promise(resolve => setImmediate(resolve));

function setup(context) {
  const dom = new JSDOM('<body data-page="page-gacha"><main id="page-gacha"><nav id="gacha-pools"></nav><div id="gacha-banner"></div><div id="artifact-draw"></div></main></body>', {runScripts:"dangerously"});
  context.after(() => dom.window.close());
  const day = "2026-11-23", now = Date.parse("2026-11-23T00:00:00+08:00");
  const view = {week:day, hero:"c03", open:true, closes_at:now+5*86400000, next_open:now+7*86400000, settled:false, shard_percent:55,
    me:{week:day, hero:"c03", points:3, chest_mask:0}, chests:[{points:3,shards:2,tickets:0},{points:8,shards:3,tickets:1}], tiers:[{through:1,shards:40},{through:20,shards:3}],
    board:[{rank:1,name:"<b>玩家昵称</b>",self:false,points:20,shards:40}], previous:null, unclaimed:[]};
  const save = {girls:{c03:{star:0,level:0,shards:79}}, mats:{limited_ticket:1}, gold:10000, token:10000};
  const calls = [], pending = [];
  const store = {profile:{},sessionEpoch:1,authUserId:1,ready:() => true,serverNow:() => now,api:async () => structuredClone(view),
    mutate:async (action, fields) => { calls.push([action,JSON.parse(JSON.stringify(fields))]); return await new Promise(resolve => pending.push(resolve)); }};
  dom.window.Store = store;
  dom.window.setupSave = save;
  dom.window.eval([read("js/config.js"),read("js/data.js"),read("js/ui.js"),read("js/progression.js"),read("js/workshop.js"),read("js/world.js"),`
    SAVE = window.setupSave; function requireGame(){return true} function renderTopbar(){} function renderGacha(){renderArtifactPool()}
    window.worldTest = {start(){renderGacha();selectDrawPool('world')}, refreshWorldPool, selectDrawPool, currentWorldPool, renderGacha, growthHeroOptions, cardSourceDescription, girls:GIRLS};
  `].join("\n;\n"));
  return {api:dom.window.worldTest, document:dom.window.document, store, day, now, view, save, calls, pending};
}

test("player sees the featured pool, can only spend tickets, and receives confirmed draws and chests", async context => {
  const {api,document,day,view,save,calls,pending} = setup(context);
  api.start(); await tick();
  const body = document.querySelector("#artifact-draw");
  const find = (action, count) => body.querySelector(`[data-world-action="${action}"]${count ? `[data-count="${count}"]` : ""}`);
  assert.equal(document.querySelector('[data-draw-pool="world"]').getAttribute("aria-pressed"),"true");
  assert.match(body.textContent,/限定 SSR/);
  assert.match(body.textContent,/55%/);
  assert.match(body.textContent,/<b>玩家昵称<\/b>/);
  assert(![...body.querySelectorAll("b")].some(node => node.textContent === "玩家昵称"));
  assert.equal(find("world_pull",10).disabled,true);
  find("world_pull",1).click();
  assert.deepEqual(calls,[["world_pull",{item_id:day,count:1}]]);
  assert.equal(save.mats.limited_ticket,1);
  assert.equal(save.girls.c03.shards,79);
  find("world_pull",1).click(); assert.equal(calls.length,1);
  save.mats.limited_ticket=0; save.girls.c03.shards=80; view.me.points=4;
  pending.shift()({drops:["本周英雄碎片 ×1"]}); await tick();
  assert.equal(save.girls.c03.shards,80); assert.match(body.textContent,/我的本期积分：4/);
  assert.equal(find("world_pull",1).disabled,true);
  assert.equal(find("world_chest",8).disabled,true);
  find("world_chest",3).click();
  assert.deepEqual(calls[1],["world_chest",{item_id:day,count:3}]);
  view.me.chest_mask=1; pending.shift()({drops:["本周英雄碎片 +2"]}); await tick();
  assert.equal(find("world_chest",3).disabled,true);
  assert.match(find("world_chest",3).textContent,/已领/);
  assert.match(body.textContent,/本周英雄碎片 \+2/);
  const all = api.girls, exclusive = all.find(card => card.id === "c03");
  assert.match(api.cardSourceDescription(exclusive),/强者世界/);
  assert(!api.growthHeroOptions("c03").includes('value="c03"'));
  assert(api.growthHeroOptions("c01").includes('value="c01"'));
  assert.equal(api.cardSourceDescription(all.find(card => card.id === "c16")),"常驻召唤");
});

test("weekend disables pulling while past-week claims remain reachable; previous account responses are discarded", async context => {
  const {api,document,day,view,store,save,calls,pending} = setup(context);
  view.open=false; view.settled=true; view.me.final_rank=2; view.me.prize_shards=24; save.mats.limited_ticket=10;
  view.unclaimed=[{week:"2026-11-16",hero:"c15",points:8,chest_mask:0,chests:[{points:4,shards:6,tickets:0}],final_rank:1,prize_shards:40,rank_claimed:false}];
  api.start(); await tick();
  const body = document.querySelector("#artifact-draw");
  assert.match(body.textContent,/周末休息/); assert.match(body.textContent,/往期领奖/);
  const each = selector => [...body.querySelectorAll(selector)];
  assert(each('[data-world-action="world_pull"]').every(button => button.disabled));
  assert(!body.querySelector('[data-world-action="world_chest"][data-week="2026-11-16"]'));
  body.querySelector('[data-world-action="history"]').click();
  assert.match(body.querySelector('[role="dialog"]').textContent,/往期领奖/);
  assert(body.querySelector('[data-world-action="world_chest"][data-week="2026-11-16"][data-count="4"]'));
  assert(!body.querySelector('[data-world-action="world_chest"][data-week="2026-11-16"][data-count="3"]'));
  assert(!body.querySelector(`[data-world-action="world_rank"][data-week="${day}"]`).disabled);
  body.querySelector('[data-world-action="world_rank"][data-week="2026-11-16"]').click();
  assert.deepEqual(calls[0],["world_rank",{item_id:"2026-11-16"}]);
  store.authUserId++; store.sessionEpoch++; api.start(); await tick();
  pending.shift()({drops:["上个账号的回执"]}); await tick();
  assert.equal(api.currentWorldPool().owner,2);
  assert(!body.textContent.includes("上个账号的回执"));
  assert(!body.querySelector('[role="dialog"]'));
});

test("ranking rewards use server tiers and tabs support mouse and keyboard navigation", async context => {
  const {api,document,view,calls} = setup(context);
  view.tiers = [{through:1,shards:40},{through:3,shards:12},{through:20,shards:3}];
  view.board[0].self = true;
  api.start(); await tick();
  const body = document.querySelector("#artifact-draw");
  const tab = name => body.querySelector(`#sw-tab-${name}`);
  const panel = name => body.querySelector(`#sw-${name}-panel`);
  const key = (name, value) => tab(name).dispatchEvent(new document.defaultView.KeyboardEvent("keydown",{key:value,bubbles:true,cancelable:true}));
  assert.equal(tab("rank").getAttribute("aria-selected"),"true");
  assert.equal(panel("rank").hidden,false);
  assert.equal(panel("awards").hidden,true);
  assert.match(body.querySelector(".sw-my-rank").textContent,/我的排名：第 1 名/);
  tab("awards").click();
  assert.equal(tab("awards").getAttribute("aria-selected"),"true");
  assert.equal(tab("awards").tabIndex,0);
  assert.equal(tab("rank").tabIndex,-1);
  assert.equal(panel("rank").hidden,true);
  assert.equal(panel("awards").hidden,false);
  assert.match(panel("awards").textContent,/第 2–3 名/);
  assert.match(panel("awards").textContent,/×12/);
  assert.match(panel("awards").textContent,/第 4–20 名/);
  key("awards","ArrowRight");
  assert.equal(document.activeElement,tab("rank"));
  assert.equal(panel("rank").hidden,false);
  key("rank","ArrowLeft");
  assert.equal(document.activeElement,tab("awards"));
  key("awards","Home");
  assert.equal(document.activeElement,tab("rank"));
  key("rank","End");
  assert.equal(document.activeElement,tab("awards"));
  assert.equal(panel("awards").hidden,false);
  assert.deepEqual(calls,[]);
});

test("rules dialog contains focus and Escape returns to its entry without changing gameplay", async context => {
  const {api,document,calls} = setup(context);
  api.start(); await tick();
  const body = document.querySelector("#artifact-draw");
  body.querySelector('[data-world-action="rules"]').focus();
  body.querySelector('[data-world-action="rules"]').click();
  const dialog = body.querySelector('[role="dialog"]');
  assert.equal(dialog.getAttribute("aria-modal"),"true");
  assert.equal(body.querySelector(`#${dialog.getAttribute("aria-labelledby")}`).textContent,"活动规则");
  assert(body.querySelector(".sw-frame").hasAttribute("inert"));
  assert.match(dialog.textContent,/55%/);
  assert.match(dialog.textContent,/每抽消耗 1 张限定券/);
  const close = dialog.querySelector('[data-world-action="close-panel"]');
  const summary = dialog.querySelector("summary");
  assert.equal(document.activeElement,close);
  close.dispatchEvent(new document.defaultView.KeyboardEvent("keydown",{key:"Tab",shiftKey:true,bubbles:true,cancelable:true}));
  assert.equal(document.activeElement,summary);
  summary.dispatchEvent(new document.defaultView.KeyboardEvent("keydown",{key:"Tab",bubbles:true,cancelable:true}));
  assert.equal(document.activeElement,close);
  close.dispatchEvent(new document.defaultView.KeyboardEvent("keydown",{key:"Escape",bubbles:true,cancelable:true}));
  assert(!body.querySelector('[role="dialog"]'));
  assert(!body.querySelector(".sw-frame").hasAttribute("inert"));
  assert.equal(document.activeElement,body.querySelector('[data-world-action="rules"]'));
  assert.deepEqual(calls,[]);
});

test("failed ranking refresh disables spending and retry reads the server without writing", async context => {
  const {api,document,view,store,save,calls} = setup(context);
  api.start(); await tick();
  const body = document.querySelector("#artifact-draw");
  const reads = [];
  let retry;
  store.api = async (endpoint,method) => {
    reads.push([endpoint,method]);
    if (reads.length === 1) throw new Error("榜单暂不可用");
    return await new Promise(resolve => { retry = resolve; });
  };
  body.querySelector('[data-world-action="refresh"]').click(); await tick();
  assert.match(body.querySelector('[role="status"]').textContent,/榜单暂不可用/);
  assert(body.querySelector('[data-world-action="world_pull"][data-count="1"]').disabled);
  assert(body.querySelector('[data-world-action="world_chest"][data-count="3"]').disabled);
  body.querySelector('[data-world-action="world_pull"][data-count="1"]').click();
  assert.equal(body.querySelectorAll('#sw-refresh').length,1);
  const refresh = body.querySelector('[data-world-action="refresh"]');
  refresh.click(); refresh.click(); await tick();
  assert.deepEqual(reads,[["/api/user/gacha/world","GET"],["/api/user/gacha/world","GET"]]);
  assert.deepEqual(calls,[]);
  assert.equal(save.mats.limited_ticket,1);
  retry(structuredClone(view)); await tick();
  assert(!body.querySelector('[role="status"]'));
  assert.equal(body.querySelector('[data-world-action="world_pull"][data-count="1"]').disabled,false);
  assert.equal(body.querySelector('[data-world-action="world_chest"][data-count="3"]').disabled,false);
  assert.deepEqual(calls,[]);
});

test("an expired activity refreshes instead of sending a draw from stale open controls", async context => {
  const {api,document,view,store,calls} = setup(context);
  api.start(); await tick();
  const body = document.querySelector("#artifact-draw");
  const pull = body.querySelector('[data-world-action="world_pull"][data-count="1"]');
  assert.equal(pull.disabled,false);
  store.serverNow = () => view.closes_at;
  view.open = false;
  pull.click(); await tick();
  assert.deepEqual(calls,[]);
  assert([...body.querySelectorAll('[data-world-action="world_pull"]')].every(button => button.disabled));
  assert.match(body.textContent,/周末休息/);
});

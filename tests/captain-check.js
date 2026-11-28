const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const {JSDOM} = require("jsdom");
const read = file => fs.readFileSync(path.join(__dirname,"..",file),"utf8");

function setup(context) {
  const browser = new JSDOM(read("index.html"),{runScripts:"outside-only"});
  context.after(() => browser.window.close());
  browser.window.document.body.dataset.page="page-captain";
  const log = {claimed:{},progress:{rollcall:1,voyage:2},supplies:{}};
  const state = {captain:{level:3,xp:8},daily:{date:"2026-11-27",captain_log:log},squad:["c02"],girls:{c01:{star:1},c02:{star:2},c03:{star:0,shards:9}},power:8903,stamina:131,staminaMax:120};
  const rules = {xp_base:20,xp_step:10,tasks:[{id:"rollcall",name:"点名",goal:"抵港",target:1,xp:40,gold:50},{id:"voyage",name:"巡航",goal:"赢三次",target:3,xp:100,gold:160}],supplies:[{id:"morning",name:"便当",start:9,end:12,stamina:30},{id:"noon",name:"炊烟",start:12,end:21,stamina:30},{id:"night",name:"热汤",start:21,end:24,stamina:30}]};
  let now = Date.parse("2026-11-27T01:00:00Z");
  const calls = [], notices = [], pages = [];
  const store = {sessionEpoch:1,profile:{captain_rules:rules,name:'<img src=x onerror="unsafe()">',user_id:42,guild:{name:"纸鹤"}},ready:() => true,serverNow:() => now,mutate:async(action,fields) => { calls.push([action,JSON.parse(JSON.stringify(fields))]); return null; }};
  const girls = ["c01","c02","c03"].map(id => ({id,img:`assets/${id}.webp`}));
  Object.assign(browser.window,{SAVE:state,Store:store,BAT:null,GIRLS:girls,GROWTH:{maxLevel:150},toast:text => notices.push(text),nav:page => pages.push(page),requireGame:() => store.ready(),openHarborMail:tab => pages.push(`mail:${tab}`),openFleetRanking:board => pages.push(`rank:${board}`),openVoyage:port => pages.push(`voyage:${port}`),openVoyageChests:() => pages.push("chests"),voyageLedger:() => ({stars:17,maximum:156})});
  const ui = read("js/ui.js");
  browser.window.eval(ui.slice(ui.indexOf("function esc("),ui.indexOf("/* ---------- 页面路由"))+ui.slice(ui.indexOf("function showModal("),ui.indexOf("function showPendingAction("))+read("js/captain.js")+`\nfunction renderTopbar(){renderCaptainSummary()}\nwindow.journalTest={renderCaptain,openCaptainProfile,syncCaptainProfile,renderCaptainSummary,captainJournalClick,captainPoints,captainChestReady};renderCaptain();renderCaptainSummary();`);
  const api = browser.window.journalTest, document = browser.window.document;
  return {state,store,api,log,calls,notices,pages,document,clock:time => { now=Date.parse(time); api.renderCaptain(); api.renderCaptainSummary(); },click:selector => api.captainJournalClick({target:document.querySelector(selector)})};
}

test("captain journal reads confirmed progress and submits only dated task and meal IDs", async context => {
  const {document,log,calls,click,api,pages} = setup(context);
  assert.equal(document.querySelector("#res-captain").textContent,"Lv.3");
  assert.match(document.querySelector("#captain-summary").textContent,/1 项任务可领取.*便当可领/);
  assert.equal(document.querySelector(".captain-xp progress").value,8);
  await click('[data-captain-visit="voyage"]');
  assert.deepEqual(pages,["page-stage"]);
  await click('[data-captain-task="rollcall"]');
  await click('[data-captain-supply="morning"]');
  await click('[data-captain-supply="night"]');
  assert.deepEqual(calls,[["captain_task",{item_id:"2026-11-27:rollcall"}],["captain_supply",{item_id:"2026-11-27:morning"}]]);
  assert.deepEqual(log.claimed,{});
  assert.deepEqual(log.supplies,{});
  log.claimed.rollcall = true; log.supplies.morning = true; api.renderCaptain();
  assert.equal(document.querySelector('[data-captain-task="rollcall"]').disabled,true);
  assert.equal(document.querySelector('[data-captain-supply="morning"]').disabled,true);
  assert.match(document.querySelector('[data-captain-supply="morning"]').textContent,/已领取/);
  await click('[data-captain-task="rollcall"]'); await click('[data-captain-supply="morning"]');
  assert.deepEqual(pages,["page-stage"]); assert.equal(calls.length,2);
});

test("home portrait opens a personal dossier with confirmed XP, full-card collection and working return and shortcuts", context => {
  const {state,store,api,document,pages,calls} = setup(context), find = selector => document.querySelector(selector), opener = find("#captain-home-profile");
  assert.equal(opener.getAttribute("aria-haspopup"),"dialog"); assert.equal(find("#captain-home-name").textContent,store.profile.name); assert.equal(find("#captain-home-name img"),null);
  assert.equal(find("#captain-home-face img").getAttribute("src"),"assets/c02.webp"); assert.equal(find("#captain-home-xp progress").value,8); assert.equal(find("#captain-home-xp progress").max,50);
  api.openCaptainProfile(opener); assert.equal(document.activeElement,find(".mb-close"));
  assert.equal(find(".captain-dossier h4").textContent,store.profile.name); assert.equal(find(".captain-dossier h4 img"),null);
  const record = Object.fromEntries([...document.querySelectorAll(".captain-dossier dl div")].map(row => [row.firstChild.textContent,row.lastChild.textContent]));
  assert.equal(record["玩家 ID"],"42"); assert.equal(record["所属公会"],"纸鹤"); assert.equal(record["图鉴收藏率"],"66.7%（2/3）"); assert.equal(record["伙伴数量"],"2");
  assert.equal(record["主线副本星数"],"17 / 156"); assert.equal(record["当前体力"],"131 / 120"); assert.match(find(".captain-dossier p").textContent,/实际消耗 1 点体力.*1 点船长经验/);
  document.dispatchEvent(new document.defaultView.KeyboardEvent("keydown",{key:"Escape"})); assert.equal(document.activeElement,opener); assert.equal(find("#modal-mask").classList.contains("hidden"),true);
  for (const [target,result] of [["story","voyage:story"],["chests","chests"],["stars","rank:stars"],["power","rank:power"],["collection","rank:collection"],["inbox","mail:inbox"],["redeem","mail:redeem"],["journal","page-captain"]]) {
    api.openCaptainProfile(opener); find(`[data-captain-dest="${target}"]`).click(); assert.equal(pages.at(-1),result);
  }
  state.squad=[]; api.renderCaptainSummary(); assert.equal(find("#captain-home-face img").getAttribute("src"),"assets/c01.webp"); assert.deepEqual(calls,[]);
});

test("captain XP survives an unsynchronized daily log and accurately distinguishes missing and maximum levels", async context => {
  const {state,store,api,document} = setup(context), find = selector => document.querySelector(selector), sync=find("#captain-home-sync");
  delete state.daily.captain_log; api.renderCaptainSummary(); assert.equal(find("#res-captain").textContent,"Lv.3"); assert.equal(find("#captain-home-xp progress").value,8);
  const rules = store.profile.captain_rules;
  delete store.profile.captain_rules; api.renderCaptainSummary(); assert.match(find("#captain-home-xp").textContent,/8 \/ 门槛待同步/); assert.equal(find("#captain-home-xp progress").hasAttribute("value"),false); assert.equal(sync.hidden,false);
  delete state.captain; api.renderCaptainSummary(); assert.equal(find("#captain-home-xp progress"),null); assert.match(find("#captain-home-level").textContent,/待同步/);
  store.refresh=async () => {state.captain={level:150,xp:0};store.profile.captain_rules=rules;return {save:state};};
  await api.syncCaptainProfile(sync); assert.equal(sync.hidden,true); assert.match(find("#captain-home-xp").textContent,/已满级/); assert.equal(find("#captain-home-xp progress").position,1);
});

test("meal windows and stale journal actions follow Beijing server time, including midnight", async context => {
  const {clock,document,state,store,notices,calls,click} = setup(context);
  for (const [time,open] of [["00:59:59",null],["01:00:00","morning"],["04:00:00","noon"],["13:00:00","night"],["16:00:00",null]]) {
    clock(`2026-11-27T${time}Z`);
    assert.deepEqual([...document.querySelectorAll('[data-captain-supply]:not(:disabled)')].map(button => button.dataset.captainSupply),open ? [open] : []);
  }
  assert.match(document.querySelector("#captain-summary").textContent,/新航日/);
  clock("2026-11-27T13:00:00Z");
  state.daily.date = "2026-11-28";
  await click('[data-captain-task="rollcall"]');
  assert.deepEqual(calls,[]);
  state.daily.date = "2026-11-27"; clock("2026-11-27T13:00:00Z");
  let resolve;
  store.mutate = () => new Promise(done => {resolve = done;});
  const pending = click('[data-captain-supply="night"]');
  store.sessionEpoch++; resolve({drops:["旧账号提示"]}); await pending;
  assert.deepEqual(notices,[]);
});

test("unfinished duties focus the wish or training control without performing the activity", async context => {
  const {api,store,document,pages,calls,click} = setup(context);
  store.profile.captain_rules.tasks.push({id:"wish",name:"许愿",goal:"选人",target:1,xp:50,gold:60},{id:"drill",name:"晨练",goal:"经验训练",target:1,xp:60,gold:80});
  document.body.insertAdjacentHTML("beforeend",'<select id="growth-wish"></select><div id="growth-contracts"><button data-growth-run>训练</button></div>');
  api.renderCaptain();
  await click('[data-captain-visit="wish"]');
  assert.equal(document.activeElement.id,"growth-wish");
  await click('[data-captain-visit="drill"]');
  assert.equal(document.activeElement.hasAttribute("data-growth-run"),true);
  assert.deepEqual(pages,["page-growth","page-growth"]); assert.equal(calls.length,0);
});

test("completed duties unlock dated chests before task claims and never grant assets in the browser", async context => {
  const {api,store,state,log,document,click,calls,clock,pages} = setup(context);
  store.profile.captain_rules = JSON.parse(read("backend/model/gacha_captain.json"));
  const rules = store.profile.captain_rules;
  api.renderCaptain();
  assert.equal(document.querySelector('[data-captain-chest="25"]').disabled,true);
  log.gear_review = true; api.renderCaptain();
  assert.equal(document.querySelector('[data-captain-visit="equipment"]').textContent,"去实战复盘");
  await click('[data-captain-visit="equipment"]'); assert.equal(pages.at(-1),"page-stage");
  for (const task of rules.tasks) log.progress[task.id] = task.target;
  api.renderCaptain(); api.renderCaptainSummary();
  assert.match(document.querySelector("#captain-chests").textContent,/100\/100/);
  assert.match(document.querySelector("#captain-summary").textContent,/4 个宝箱可领/);
  assert.match(document.querySelector('[data-captain-chest="100"]').parentElement.textContent,/限定券/);
  await click('[data-captain-chest="100"]');
  assert.deepEqual(calls,[["captain_chest",{item_id:`${state.daily.date}:100`}]]);
  assert.equal(log.chests,undefined); assert.deepEqual(log.claimed,{});
  log.chests={"100":true}; api.renderCaptain();
  assert.equal(document.querySelector('[data-captain-chest="100"]').disabled,true);
  assert.equal(document.querySelector('[data-captain-chest="75"]').disabled,false);
  clock("2026-11-27T16:00:00Z");
  assert.equal(document.querySelector('[data-captain-chest="75"]').disabled,true);
});

test("every possible task completion set agrees with the published chest thresholds", context => {
  const {api,log} = setup(context), rules = JSON.parse(read("backend/model/gacha_captain.json"));
  const journal = {current:true,rules,log};
  for (let mask = 0; mask < 2**rules.tasks.length; mask++) {
    let total = 0;
    for (const [index,task] of rules.tasks.entries()) {
      const complete = Boolean(mask & (1 << index));
      log.progress[task.id] = task.target - (complete ? 0 : 1);
      if (complete) total += task.points;
    }
    assert.equal(api.captainPoints(journal),total);
    for (const chest of rules.chests) {
      log.chests = {};
      assert.equal(api.captainChestReady(journal,chest),total >= chest.points);
      log.chests[chest.id] = true;
      assert.equal(api.captainChestReady(journal,chest),false);
    }
  }
});

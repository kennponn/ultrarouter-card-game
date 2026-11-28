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
  const calls = [], notices = [], pages = [], destinations = [];
  const store = {sessionEpoch:1,authUserId:42,profile:{captain_rules:rules,name:'<img src=x onerror="unsafe()">',user_id:42,guild:{name:"纸鹤"}},ready:() => true,serverNow:() => now,mutate:async(action,fields) => { calls.push([action,JSON.parse(JSON.stringify(fields))]); return null; }};
  const girls = ["c01","c02","c03"].map(id => ({id,img:`assets/${id}.webp`}));
  Object.assign(browser.window,{SAVE:state,Store:store,BAT:null,GIRLS:girls,GROWTH:{maxLevel:150},toast:text => notices.push(text),nav:page => pages.push(page),requireGame:() => store.ready(),openHarborMail:tab => pages.push(`mail:${tab}`),openFleetRanking:board => pages.push(`rank:${board}`),openVoyage:(port,stage) => { pages.push(`voyage:${port}`); destinations.push(["voyage",port,stage]); },openVoyageChests:() => pages.push("chests"),voyageLedger:() => ({stars:17,maximum:156})});
  const ui = read("js/ui.js");
  browser.window.eval(ui.slice(ui.indexOf("function esc("),ui.indexOf("/* ---------- 页面路由"))+ui.slice(ui.indexOf("function showModal("),ui.indexOf("function showPendingAction("))+read("js/reward-chests.js")+read("js/captain.js")+`\nfunction renderTopbar(){renderCaptainSummary()}\nwindow.journalTest={renderCaptain,openCaptainProfile,syncCaptainProfile,renderCaptainSummary,captainJournalClick,captainPoints,captainChestReady,captainJournal,captainNavigateTask};renderCaptain();renderCaptainSummary();`);
  const api = browser.window.journalTest, document = browser.window.document;
  return {state,store,api,log,calls,notices,pages,destinations,document,window:browser.window,clock:time => { now=Date.parse(time); api.renderCaptain(); api.renderCaptainSummary(); },click:selector => api.captainJournalClick({target:document.querySelector(selector)})};
}

function dailyNavigation(context) {
  const harness = setup(context), {api,store,state,window,document,destinations} = harness;
  store.profile.captain_rules = JSON.parse(read("backend/model/gacha_captain.json"));
  store.profile.equipment = {slots:[{id:"weapon"},{id:"armor"}]};
  store.profile.training = {effort_stats:[{id:"hp"},{id:"atk"}]};
  state.mats = {};
  state.girls.c01.level = 5;
  state.girls.c02.level = 10;
  Object.assign(window,{
    openWorkshop:(id,tab) => destinations.push(["workshop",id,tab]),
    selectAtelierGear:slot => destinations.push(["gear",slot]),
    selectTrainingPanel:panel => destinations.push(["training",panel]),
    selectEffortStat:stat => destinations.push(["effort",stat]),
    atelierGearQuote:() => ({levelCost:0,canUpgrade:false}),
    trainingSkillQuote:() => ({learned:5,cap:5,count:0}),
    trainingQuote:() => ({}),
    upgradeRules:() => ({materials:[{id:"exp_book"}]}),
    cardLevelLimit:() => 10
  });
  document.body.insertAdjacentHTML("beforeend",'<button data-gear-action="gear_upgrade">强化</button><button data-upgrade-open>升级材料</button><button data-skill-up="normal">普攻</button><button data-skill-up="rage">怒气技</button><button data-skill-up="passive">被动</button><button data-voy-start>出航</button>');
  api.renderCaptain();
  return harness;
}

test("captain journal reads confirmed progress and submits only dated task and meal IDs", async context => {
  const {document,log,calls,click,api,pages} = setup(context);
  assert.equal(document.querySelector("#res-captain").textContent,"Lv.3");
  assert.match(document.querySelector("#captain-summary").textContent,/1 项任务可领取.*便当可领/);
  assert.equal(document.querySelector(".captain-xp progress").value,8);
  await click('[data-captain-visit="voyage"]');
  assert.deepEqual(pages,["voyage:story"]);
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
  assert.deepEqual(pages,["voyage:story"]); assert.equal(calls.length,2);
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
  document.body.insertAdjacentHTML("beforeend",'<button data-growth-run id="unrelated-trial">专属试炼</button><select id="growth-wish"></select><div id="growth-contracts"><button data-growth-run id="daily-training">训练</button></div>');
  api.renderCaptain();
  await click('[data-captain-visit="wish"]');
  assert.equal(document.activeElement.id,"growth-wish");
  await click('[data-captain-visit="drill"]');
  assert.equal(document.activeElement.id,"daily-training");
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
  await click('[data-captain-visit="equipment"]'); assert.equal(pages.at(-1),"voyage:story");
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

test("equipment duties open an affordable owned hero and slot without upgrading automatically", async context => {
  const {window,state,document,destinations,pages,calls,click} = dailyNavigation(context), quoted = [];
  state.squad = ["c02","c02","c03","unknown"];
  window.atelierGearQuote = (id,slot) => {
    quoted.push([id,slot]);
    if (id === "c01" && slot === "armor") return {levelCost:50,canUpgrade:true};
    if (id === "c02" && slot === "armor") return {levelCost:100,canUpgrade:false};
    return {levelCost:0,canUpgrade:false};
  };
  const before = JSON.parse(JSON.stringify(state));
  await click('[data-captain-visit="equipment"]');
  assert.deepEqual(destinations,[["workshop","c01","weapon"],["gear","armor"]]);
  assert.deepEqual(quoted,[["c02","weapon"],["c02","armor"],["c01","weapon"],["c01","armor"]],"only implemented full cards are eligible, and squad duplicates are ignored");
  assert.equal(document.activeElement.dataset.gearAction,"gear_upgrade");
  assert.deepEqual(pages,[],"the workshop keeps the task page underneath for returning");
  assert.deepEqual(calls,[]);
  assert.deepEqual(state,before);
});

test("skill duties prefer an immediately learnable line and focus its upgrade control", async context => {
  const {window,state,document,destinations,calls,click} = dailyNavigation(context);
  window.trainingSkillQuote = (id,line) => {
    if (id === "c02" && line === "normal") return {learned:1,cap:5,count:0};
    if (id === "c01" && line === "rage") return {learned:0,cap:5,count:2};
    return {learned:5,cap:5,count:0};
  };
  const before = JSON.parse(JSON.stringify(state));
  await click('[data-captain-visit="skill"]');
  assert.deepEqual(destinations,[["workshop","c01","training"],["training","skills"]]);
  assert.equal(document.activeElement.dataset.skillUp,"rage");
  assert.deepEqual(calls,[]);
  assert.deepEqual(state,before);
});

test("cultivation duties choose a usable upgrade destination from confirmed inventory and limits", async context => {
  const scenarios = [
    {
      name:"experience books select an owned hero below the captain limit",
      prepare:({state,window}) => { state.mats.exp_book=1; window.atelierGearQuote=() => ({levelCost:50,canUpgrade:true}); },
      expected:[["workshop","c01","upgrade"]], focus:"[data-upgrade-open]"
    },
    {
      name:"no books falls back to affordable equipment",
      prepare:({window}) => { window.atelierGearQuote=(id,slot) => ({levelCost:slot === "armor" ? 50 : 0,canUpgrade:id === "c02" && slot === "armor"}); },
      expected:[["workshop","c02","weapon"],["gear","armor"]], focus:'[data-gear-action="gear_upgrade"]'
    },
    {
      name:"heroes at the level cap can use a learnable skill instead",
      prepare:({state,window}) => { state.mats.exp_book=1; state.girls.c01.level=10; window.trainingSkillQuote=(id,line) => ({learned:id === "c01" && line === "passive" ? 1 : 5,cap:5,count:1}); },
      expected:[["workshop","c01","training"],["training","skills"]], focus:'[data-skill-up="passive"]'
    },
    {
      name:"breakthrough materials open the qualified hero's breakthrough panel",
      prepare:({window}) => { window.trainingQuote=id => ({canBreak:id === "c01"}); },
      expected:[["workshop","c01","training"],["training","breakthrough"]]
    },
    {
      name:"effort materials select the qualified stat",
      prepare:({window}) => { window.trainingQuote=(id,stat) => ({canTrain:id === "c01" && stat === "atk"}); },
      expected:[["workshop","c01","training"],["training","effort"],["effort","atk"]]
    },
    {
      name:"unaffordable progress still opens a nonmax equipment slot for inspection",
      prepare:({window}) => { window.atelierGearQuote=(id,slot) => ({levelCost:id === "c01" && slot === "armor" ? 50 : 0,canUpgrade:false}); },
      expected:[["workshop","c01","weapon"],["gear","armor"]], focus:'[data-gear-action="gear_upgrade"]'
    }
  ];
  for (const scenario of scenarios) await context.test(scenario.name,async context => {
    const harness = dailyNavigation(context), {state,document,destinations,pages,calls,click} = harness;
    scenario.prepare(harness);
    const before = JSON.parse(JSON.stringify(state));
    await click('[data-captain-visit="outfit"]');
    assert.deepEqual(destinations,scenario.expected);
    if (scenario.focus) assert.equal(document.activeElement,document.querySelector(scenario.focus));
    assert.deepEqual(pages,[]);
    assert.deepEqual(calls,[]);
    assert.deepEqual(state,before);
  });
});

test("workshop duties with only fragments direct players to the codex", async context => {
  const {state,destinations,pages,calls,click} = dailyNavigation(context);
  for (const own of Object.values(state.girls)) { own.star=0; own.shards=100; }
  const before = JSON.parse(JSON.stringify(state));
  for (const duty of ["equipment","skill","outfit"]) await click(`[data-captain-visit="${duty}"]`);
  assert.deepEqual(pages,["page-codex","page-codex","page-codex"]);
  assert.deepEqual(destinations,[]);
  assert.deepEqual(calls,[]);
  assert.deepEqual(state,before,"fragment counts alone never grant ownership or synthesize a hero");
});

test("workshop duties fall back to growth with a synchronization notice when workshop data is unavailable", async context => {
  for (const boundary of ["missing rules","missing tab"]) await context.test(boundary,async context => {
    const {window,state,destinations,pages,notices,calls,click} = dailyNavigation(context);
    window.workshopRules = () => boundary === "missing rules" ? null : {};
    window.workshopTabKeys = () => boundary === "missing tab" ? ["overview"] : ["weapon","training"];
    const before = JSON.parse(JSON.stringify(state));
    for (const duty of ["equipment","skill"]) await click(`[data-captain-visit="${duty}"]`);
    assert.deepEqual(pages,["page-growth","page-growth"]);
    assert.equal(notices.length,2);
    for (const message of notices) assert.match(message,/工坊.*同步/);
    assert.deepEqual(destinations,[],"missing workshop data never opens an unusable workshop");
    assert.deepEqual(calls,[]);
    assert.deepEqual(state,before);
  });
});

test("voyage and review duties select an available story battle and leave combat to confirmation", async context => {
  const {api,window,state,store,log,document,destinations,calls,click} = dailyNavigation(context);
  const session = {chapter:99,mode:"heroic"};
  const stages = {
    free:{id:"free",cost:0,left:1},locked:{id:"locked",cost:5,left:1},
    spent:{id:"spent",cost:5,left:0},ready:{id:"ready",cost:5,left:1}
  };
  store.profile.voyage = {chapters:[{routes:{story:["missing","free","locked","spent"]}},{routes:{story:["ready"]}}]};
  Object.assign(window,{voyageStage:id => stages[id],voyageGate:point => point.id === "locked" ? "locked" : "",voyageRemaining:point => point.left,voyageSession:() => session});
  log.gear_review=true; log.skill_review=true; api.renderCaptain();
  const before = JSON.parse(JSON.stringify(state));
  for (const duty of ["voyage","equipment","skill","outfit"]) {
    session.chapter=99; session.mode="heroic";
    await click(`[data-captain-visit="${duty}"]`);
    assert.deepEqual(session,{chapter:1,mode:"story"});
    assert.deepEqual(destinations.at(-1),["voyage","story","ready"]);
    assert.equal(document.activeElement,document.querySelector("[data-voy-start]"));
  }
  assert.equal(destinations.length,4);
  assert.deepEqual(calls,[],"opening a stage never starts, sweeps or claims automatically");
  assert.deepEqual(state,before);
});

test("ladder and festival duties use their dedicated entry points without performing an action", async context => {
  const {window,state,destinations,pages,calls,click} = dailyNavigation(context);
  state.festival = {day:4};
  window.openFestivalToday = () => destinations.push(["festival",state.festival.day]);
  const before = JSON.parse(JSON.stringify(state));
  await click('[data-captain-visit="ladder"]');
  await click('[data-captain-visit="festival"]');
  assert.deepEqual(pages,["page-arena"]);
  assert.deepEqual(destinations,[["festival",4]]);
  assert.deepEqual(calls,[]);
  assert.deepEqual(state,before);
});

test("stale daily task destinations cannot navigate or consume after midnight or an account change", async context => {
  for (const boundary of ["date","epoch","owner"]) await context.test(boundary,async context => {
    const {api,document,state,store,pages,calls} = setup(context);
    const stale = document.querySelector('[data-captain-visit="voyage"]');
    assert.ok(stale && !stale.disabled);
    if (boundary === "date") state.daily.date = "2026-11-28";
    else if (boundary === "epoch") store.sessionEpoch++;
    else { store.authUserId++; store.profile.user_id++; }
    await api.captainJournalClick({target:stale});
    assert.deepEqual(pages,[]);
    assert.deepEqual(calls,[]);
    assert.equal(state.daily.captain_log.progress.voyage,2);
  });
});

test("returning from a task keeps unconfirmed progress and enables rewards only after a server snapshot", async context => {
  const {api,store,state,log,document,click,calls} = setup(context);
  store.profile.captain_rules = JSON.parse(read("backend/model/gacha_captain.json"));
  log.progress = {rollcall:1,voyage:3,outfit:0};
  api.renderCaptain();
  assert.equal(api.captainPoints(api.captainJournal()),20);
  assert.equal(document.querySelector('[data-captain-chest="25"]').disabled,true);
  const before = JSON.parse(JSON.stringify(state));
  await click('[data-captain-visit="outfit"]');
  api.renderCaptain();
  assert.deepEqual(state,before,"navigation alone cannot complete a duty or grant resources");
  let finish, reads = 0;
  store.refresh = () => { reads++; return new Promise(resolve => {finish = resolve;}); };
  const pending = click('[data-captain-sync]');
  assert.equal(reads,1);
  assert.equal(document.querySelector('[data-captain-chest="25"]').disabled,true);
  assert.equal(document.querySelector('[data-captain-task="outfit"]'),null);
  assert.deepEqual(state,before,"a pending profile response must not predict task progress");
  log.progress.outfit = 1;
  finish({save:state});
  await pending;
  assert.equal(api.captainPoints(api.captainJournal()),25);
  assert.equal(document.querySelector('[data-captain-task="outfit"]').disabled,false);
  assert.equal(document.querySelector('[data-captain-chest="25"]').disabled,false);
  assert.deepEqual(log.claimed,{},"completed duties unlock points before claiming their individual rewards");
  assert.deepEqual(calls,[],"navigation and read synchronization never consume or claim automatically");
  await click('[data-captain-chest="25"]');
  assert.deepEqual(calls,[["captain_chest",{item_id:`${state.daily.date}:25`}]]);
  assert.equal(log.chests,undefined,"even an explicit claim waits for the authoritative receipt");
});

test("confirmed workshop receipts refresh tasks and chest eligibility underneath the open workshop", async context => {
  for (const duty of ["equipment","skill"]) await context.test(duty,context => {
    const {api,window,state,log,document,calls} = dailyNavigation(context);
    const ui = read("js/ui.js");
    window.eval(ui.slice(ui.indexOf("function refreshCultivationViews("),ui.indexOf("async function cultivateGirl("))+"\nwindow.refreshDailyCultivation=refreshCultivationViews;");
    log.progress = {rollcall:1,voyage:3,[duty]:4};
    api.renderCaptain();
    document.getElementById("modal-mask").classList.remove("hidden");
    document.getElementById("modal-box").innerHTML='<section class="hero-workshop" data-cultivation-card="c02"></section>';
    let paints = 0;
    window.paintWorkshop = () => { paints++; };
    window.refreshDailyCultivation("c02");
    assert.ok(document.querySelector(`[data-captain-visit="${duty}"]`));
    assert.equal(document.querySelector('[data-captain-chest="25"]').disabled,true);
    assert.equal(api.captainPoints(api.captainJournal()),20);
    log.progress[duty] = 5;
    const confirmed = JSON.parse(JSON.stringify(state));
    window.refreshDailyCultivation("c02");
    assert.equal(document.querySelector(`[data-captain-task="${duty}"]`).disabled,false);
    assert.equal(document.querySelector(`[data-captain-visit="${duty}"]`),null);
    assert.equal(document.querySelector('[data-captain-chest="25"]').disabled,false);
    assert.equal(api.captainPoints(api.captainJournal()),35);
    assert.equal(document.body.dataset.page,"page-captain");
    assert.equal(document.getElementById("modal-mask").classList.contains("hidden"),false);
    assert.equal(paints,2,"both the workshop and the task page consume the same confirmed state");
    assert.deepEqual(calls,[]);
    assert.deepEqual(state,confirmed);
  });
});

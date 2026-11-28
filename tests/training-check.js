const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const { JSDOM } = require("jsdom");

const read = name => fs.readFileSync(path.join(__dirname,"..",name),"utf8");
const config = name => JSON.parse(read(`backend/model/gacha_${name}.json`));

function setup(context) {
  const dom = new JSDOM(read("index.html"),{runScripts:"outside-only"});
  context.after(() => dom.window.close());
  const source = ["config","store","data","ui","progression","atelier","training","experience","workshop","battle"].map(name => read(`js/${name}.js`));
  dom.window.eval(source.join("\n;\n") + `\nfunction requireGame(){return Store.ready()}\nwindow.trainingTest={Store,openWorkshop,selectTrainingPanel,selectEffortStat,trainingQuote,trainingSkillQuote,updateTrainingClock,actTraining,renderSquad,atelierGrowthDossier,finalStats,paintWorkshop,cardLevelCost,openUpgradePicker,confirmUpgradeFeed,setup(state){SAVE=state}};`);
  const view = dom.window.trainingTest, state = {girls:{c01:{star:2,level:30,training:{plus:1}},c08:{star:1,level:30}},squad:["c01"],gold:1000,mats:{training_ink:20,break_seal:3},daily:{}};
  view.Store.profile = {training:config("training"),cultivation:config("cultivation"),equipment:config("equipment"),items:config("items")};
  view.Store.ready = () => true;
  state.skill_pool = {points:4,tick_at:100000};
  view.Store.serverNow = () => 100000;
  view.setup(state);
  return {view,state,document:dom.window.document};
}

test("all grades finish their own + tiers before crossing; card frames distinguish grade, rarity and stars", context => {
  const {view,state,document} = setup(context), own = state.girls.c01;
  view.openWorkshop("c01","training");
  assert.equal(document.querySelector('[data-break-preview],[data-break-gate],.tr-grade-path,[data-workshop-roster]'),null);
  assert.doesNotMatch(document.querySelector('[data-training-sheet]').textContent,/青绿|历史最高|星级/);
  assert.equal(view.trainingQuote("c01").breakInk,13);
  own.training.plus = 2;
  assert.equal(view.trainingQuote("c01").canBreak,true);
  own.level = 9;
  assert.equal(view.trainingQuote("c01").canBreak,false);
  own.training = {grade:1,plus:2}; own.star=1; own.level=20; state.mats.training_ink=42;
  view.paintWorkshop(); assert.match(document.querySelector('[data-break-cost]').textContent,/42 \/ 43 还差 1/);
  assert.equal(view.trainingQuote('c01').canBreak,false);
  state.mats.training_ink++; assert.equal(view.trainingQuote('c01').canBreak,false);
  state.gold = view.trainingQuote('c01').breakGold; assert.equal(view.trainingQuote('c01').canBreak,true);
  own.level = 60; own.star = 12;
  let previous = 0;
  for (const [index,grade] of view.Store.profile.training.grades.entries()) {
    own.training.grade = index;
    for (let plus = 0; plus <= grade.max_plus; plus++) {
      own.training.plus = plus;
      const quote = view.trainingQuote("c01");
      assert.equal(quote.cross,plus === grade.max_plus);
      assert.equal(quote.end,index === 6 && plus === grade.max_plus);
      if (!quote.end) { assert.ok(quote.breakInk > previous); previous = quote.breakInk; }
    }
  }
  own.training = {grade:1,plus:0};
  view.renderSquad();
  assert.match(document.querySelector('#owned-list [data-id="c01"][data-break-quality="green"]').textContent,/青绿 \+0/);
  assert.match(document.querySelector('.team-slot[data-break-quality="green"]').textContent,/青绿 \+0/);
  const fragment = document.createElement("div");
  fragment.innerHTML = view.atelierGrowthDossier("c01",{title:"历练",note:"",action:"stage"});
  assert.equal(fragment.querySelector('[data-roster-girl]'),null);
  assert.ok(fragment.querySelector('.at-profile-head[data-break-quality="green"] .tr-grade-tag'));
  view.paintWorkshop();
  assert.match(document.querySelector('.ws-portrait[data-break-quality="green"]').textContent,/青绿 \+0/);
  delete view.Store.profile.training;
  view.openWorkshop("c01","overview");
  assert.equal(document.querySelector('[data-workshop-tab="training"]'),null);
  assert.equal(document.querySelector(".ws-portrait[data-break-quality]"),null);
});

test("effort carries floors, exposes its selected stat, and refuses an incomplete stage", async context => {
  const {view,state,document} = setup(context), card = state.girls.c01.training, calls = [];
  card.effort = {hp:160,atk:160,def:160,spd:160,res:150};
  view.Store.mutate = async (action,fields) => { calls.push([action,JSON.parse(JSON.stringify(fields))]); return null; };
  view.openWorkshop("c01","training"); view.selectTrainingPanel("effort");
  assert.equal(document.querySelectorAll("[data-effort-stat]").length,5);
  assert.equal(document.querySelector('[data-effort-route]'),null);
  assert.match(document.querySelector("[data-effort-budget]").textContent,/至少 8 个游戏日/);
  await view.actTraining("effort_promote"); await view.actTraining("effort_train");
  assert.deepEqual(calls,[]);
  view.selectEffortStat("res"); await view.actTraining("effort_train");
  assert.deepEqual(calls,[["effort_train",{girl_id:"c01",choice:"res"}]]);
  assert.equal(card.effort.res,150); assert.equal(state.mats.training_ink,20);
  assert.equal(view.trainingQuote("c01","res").batch,1);
  await view.actTraining("effort_train_max"); assert.equal(calls.at(-1)[0],"effort_train_max");
  state.daily.effort_spent={c01:10}; assert.equal(view.trainingQuote("c01","res").canTrain,false);
  card.effort.res = 160; view.paintWorkshop();
  assert.equal(document.querySelector('[data-training-action="effort_train"]').disabled,true);
  assert.equal(document.querySelector('[data-training-action="effort_promote"]').disabled,false);
  await view.actTraining("effort_promote"); assert.equal(calls.at(-1)[0],"effort_promote");
  assert.equal(document.querySelector('[data-effort-arrival]'),null);
  view.Store.mutate = async () => { card.stage = 1; return {drops:[]}; };
  await view.actTraining('effort_promote');
  assert.match(document.querySelector('[data-effort-arrival]').textContent,/扎营.*400/);
  assert.equal(document.querySelector('[data-effort-stat]'),null);
  view.selectTrainingPanel('effort');
  assert.equal(document.querySelector('[data-effort-stat="res"] progress').value,0);
  assert.match(document.querySelector('[data-effort-stat="res"]').textContent,/160\/400/);
});

test("effort stones can fund training but not a breakthrough", context => {
  const {view,state,document} = setup(context);
  state.mats.training_ink = 0;
  state.mats.effort_stone = 3;
  const quote = view.trainingQuote("c01","atk");
  assert.equal(quote.canTrain, true);
  assert.equal(quote.canBreak, false);
  assert.equal(quote.batch, 3);
  view.openWorkshop("c01","training"); view.selectTrainingPanel("effort");
  assert.match(document.querySelector("[data-effort-budget]").parentElement.textContent,/努力培养石（3）/);
  state.gold = 0;
  assert.equal(view.trainingQuote("c01","atk").canTrain, false);
});

test("selected books quote a real multi-level gain, clamp at 150 and submit only material counts", async context => {
  const {view,state,document} = setup(context), own = state.girls.c01, calls = [];
  Object.assign(state.mats,{training_manual:1,field_notes:2,astral_treatise:10});
  own.level = 149; own.exp = view.cardLevelCost(149)-601;
  assert.equal(view.cardLevelCost(0),100); assert.equal(view.cardLevelCost(120),49235); assert.equal(view.cardLevelCost(149),118400);
  view.Store.mutate = async (action,fields) => { calls.push([action,JSON.parse(JSON.stringify(fields))]); return null; };
  view.openWorkshop("c01","upgrade"); view.openUpgradePicker();
  assert.equal(document.querySelectorAll("[data-feed-material]").length,4);
  assert.equal(document.querySelector("[data-upgrade-submit]").disabled,false);
  assert.equal(document.querySelectorAll('[data-feed-check]:checked').length,3);
  document.querySelector("[data-feed-all]").click();
  assert.equal(document.querySelector('[data-upgrade-submit]').disabled,true);
  document.querySelector('[data-feed-all]').click();
  assert.match(document.querySelector("[data-feed-preview]").textContent,/149 → Lv\.150.*601.*历练手记 ×1、斥候札记 ×1/);
  assert.doesNotMatch(document.querySelector("[data-feed-preview]").textContent,/星轨秘典/);
  await view.confirmUpgradeFeed();
  assert.deepEqual(calls,[["card_feed",{girl_id:"c01",materials:{training_manual:1,field_notes:2,astral_treatise:10}}]]);
  assert.equal(own.level,149); assert.equal(state.mats.field_notes,2);
  const input=document.querySelector('[data-feed-material="field_notes"]'); input.value="3"; input.oninput();
  assert.equal(document.querySelector("[data-upgrade-submit]").disabled,true); await view.confirmUpgradeFeed(); assert.equal(calls.length,1);
  own.level=0; own.exp=0; own.training={grade:1}; own.peak_level=149;
  view.openWorkshop("c01","training"); assert.equal(document.querySelector('[data-break-reset]'),null);
  view.openWorkshop("c01","upgrade"); view.openUpgradePicker(); document.querySelector('[data-feed-all]').click(); document.querySelector('[data-feed-check="training_manual"]').click();
  assert.match(document.querySelector("[data-feed-preview]").textContent,/Lv\.0 → Lv\.3/);
});

test("book preview and submit stop at the current captain level without spending excess books", async context => {
  const {view,state,document} = setup(context), own = state.girls.c01;
  state.captain = {level:31,xp:0};
  own.exp = view.cardLevelCost(30)-1;
  state.mats.training_manual = 9;
  view.openWorkshop("c01","upgrade"); view.openUpgradePicker();
  assert.match(document.querySelector("[data-feed-preview]").textContent,/Lv\.30 → Lv\.31.*实得 1 经验.*历练手记 ×1$/);
  own.level = 31; own.exp = 0; view.openWorkshop("c01","upgrade"); view.openUpgradePicker();
  assert.equal(document.querySelector('[data-upgrade-open]').disabled,true);
  assert.equal(document.querySelector('[data-upgrade-picker]'),null);
  assert.match(document.querySelector(".xp-ledger").textContent,/提升船长等级/);
});

test("skills use only confirmed shared points and gold; single, automatic and purchase intents are distinct", async context => {
  const {view,state,document} = setup(context), calls = [];
  const {upgrade_gold_base:base,upgrade_gold_step:step} = view.Store.profile.training.skill_pool;
  view.Store.mutate = async (action,fields) => { calls.push([action,JSON.parse(JSON.stringify(fields))]); return null; };
  view.openWorkshop("c01","training"); view.selectTrainingPanel("skills");
  assert.equal(document.querySelectorAll("[data-skill-line]").length,3);
  assert.equal(document.querySelector("[data-skill-points]").textContent,"4/20");
  assert.equal(view.trainingSkillQuote("c01","normal").count,4);
  assert.equal(view.trainingSkillQuote("c01","normal").cost,4*base+6*step);
  await view.actTraining("skill_point","invented"); await view.actTraining("skill_buy",-1); await view.actTraining("skill_buy",17);
  state.gold = 29; await view.actTraining("skill_point","normal"); await view.actTraining("skill_buy",1); assert.deepEqual(calls,[]);
  state.gold = 2*base+step; assert.equal(view.trainingSkillQuote("c01","normal").count,2);
  await view.actTraining("skill_point","normal"); await view.actTraining("skill_auto","normal");
  state.gold = 600; await view.actTraining("skill_buy",5);
  assert.deepEqual(calls,[["skill_point",{girl_id:"c01",choice:"normal"}],["skill_auto",{girl_id:"c01",choice:"normal"}],["skill_buy",{count:5}]]);
  assert.equal(state.gold,600); assert.equal(state.skill_pool.points,4); assert.equal(state.girls.c01.training.skills,undefined);
  state.girls.c01.training.skills = {normal:12}; state.girls.c01.level = 0; view.paintWorkshop();
  const normal = document.querySelector('[data-skill-line="normal"]');
  assert.equal(normal.querySelector('[data-skill-learned]').textContent,"12"); assert.equal(normal.querySelector('[data-skill-effective]').textContent,"0");
  assert.equal(normal.querySelector('[data-skill-up]').disabled,true);
  state.girls.c01.level = 7; assert.equal(view.trainingSkillQuote("c01","normal").effective,7);
  state.girls.c01.level = 13; assert.equal(view.trainingSkillQuote("c01","normal").count,1);
  state.skill_pool.points = 0; view.openWorkshop("c08","training"); view.selectTrainingPanel("skills");
  assert.equal(document.querySelectorAll("[data-skill-auto]:disabled").length,3);
  await view.actTraining("skill_auto","rage"); assert.equal(calls.length,3);
  let resolve;
  state.skill_pool.points = 1; view.Store.mutate = () => new Promise(done => { resolve = done; });
  const pending = view.actTraining("skill_point","passive");
  assert.ok(resolve); view.Store.sessionEpoch++; view.openWorkshop("c01","training");
  resolve({drops:["old training receipt"]}); await pending;
  assert.doesNotMatch(document.body.textContent,/old training receipt/);
  assert.equal(document.querySelector('[data-training-action="card_breakthrough"]').disabled,false);
});

test("the display requests server regeneration at 20 seconds and never awards a point from a browser timer", async context => {
  const {view,state,document} = setup(context);
  Object.defineProperty(document,"visibilityState",{configurable:true,value:"visible"});
  state.skill_pool.points = 0;
  let time = state.skill_pool.tick_at+19999, reads = 0, finish;
  view.Store.serverNow = () => time;
  view.Store.refresh = () => { reads++; return new Promise(done => { finish = done; }); };
  view.openWorkshop("c01","training"); view.selectTrainingPanel("skills");
  await view.updateTrainingClock(); assert.equal(reads,0); assert.match(document.querySelector('[data-skill-clock]').textContent,/1 秒/);
  time++;
  const pending = view.updateTrainingClock(); assert.equal(reads,1); assert.equal(document.querySelector('[data-skill-points]').textContent,"0/20");
  state.skill_pool = {points:1,tick_at:time}; finish({save:state}); await pending;
  assert.equal(document.querySelector('[data-skill-points]').textContent,"1/20");
  assert.equal(document.querySelector('[data-skill-up="normal"]').disabled,false);
});

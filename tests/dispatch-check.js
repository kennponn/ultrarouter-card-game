const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const {JSDOM} = require("jsdom");
const read = name => fs.readFileSync(path.join(__dirname,"..",name),"utf8");
const catalog = name => JSON.parse(read(`backend/model/gacha_${name}.json`));

function desk(context) {
  const dom = new JSDOM(read("index.html"),{runScripts:"outside-only"}); context.after(() => dom.window.close());
  dom.window.eval(["config","store","data","ui","progression","atelier","dispatch","workshop"].map(name => read(`js/${name}.js`)).join("\n;\n") + `\nfunction requireGame(){return Store.ready()}\nwindow.deskTest={Store,openWorkshop,paintWorkshop,setDispatchRoom,dispatchAction,dispatchClockPick,dispatchInk,dispatchPick,dispatchEvidenceBag,setup(value){SAVE=value}};`);
  const view = dom.window.deskTest, document = dom.window.document, rules = catalog("dispatch"), day = "2026-11-26";
  const bureau = {clock:{hints:[{a:"moon",op:"next",b:"ash"},{a:"tide",op:"notfirst"}],guesses:[],solved:false},route:{tiles:"cache fog spring vault spring beacon fog cache spring beacon vault spring".split(" "),track:[],status:"open",light:5,cargo:0},mirror:{roster:rules.mirrors[0].pool,scene:0,attempts:0,won:false}};
  const state = {girls:{c01:{level:12,star:2}},squad:["c01"],mats:{},gold:50,dispatch:{relics:{},mosaic:{week:"2026-W48",cleared:0,marks:[0,4,8]}},daily:{date:day,dispatch:bureau},casebook:{evidence:{ember:1,echo:3},endings:{silent_bell:{revealed:1}}}};
  view.Store.profile = {dispatch:rules,cases:catalog("cases"),items:catalog("items"),cultivation:catalog("cultivation"),equipment:catalog("equipment")}; view.Store.ready = () => true; view.setup(state);
  const calls = []; view.Store.mutate = async (action,fields) => {calls.push([action,JSON.parse(JSON.stringify(fields))]);return null;};
  view.openWorkshop("c01","dispatch"); const find = selector => document.querySelector(selector), room = name => view.setDispatchRoom(name);
  return {view,state,rules,bureau,calls,find,room,day,document};
}

test("seven actual desks share an authoritative profile; old hosts do not advertise them", context => {
  const {view,room,find,document} = desk(context);
  assert.equal(document.querySelectorAll("[data-dispatch-tab]").length,7);
  for (const name of ["clock","route","mosaic","mirror","contract","relic"]) {room(name);assert.equal(find("#dispatch-panel").dataset.deskRoom,name);assert.ok(find("#dispatch-panel .dp-sheet button"));}
  assert.match(view.dispatchEvidenceBag(),/苍蓝|藍|蓝/);delete view.Store.profile.dispatch;view.openWorkshop("c01","overview");assert.equal(find('[data-workshop-tab="dispatch"]'),null);
});
test("contract notices only promise sweep reports advertised by the host", context => {
  const {view,rules,room,find} = desk(context);
  room("contract");
  assert.match(find(".dp-sheet .dp-note").textContent,/主线由服务器逐场核验/);
  delete rules.sweep_reports;
  view.paintWorkshop();
  assert.match(find(".dp-sheet .dp-note").textContent,/请手动出战/);
});
test("the cabinet needs the right ending AND its evidence; ownership is not activation", async context => {
  const {view,state,calls,find} = desk(context), button = type => find(`[data-desk-action="relic_${type}"]`);
  assert.equal(button("bind").disabled,false);assert.equal(button("wear").disabled,true);assert.match(find(".dp-trade").textContent,/28.*10%/);
  await view.dispatchAction("relic_bind","bell");assert.deepEqual(calls,[["relic_bind",{item_id:"bell"}]]);assert.equal(state.casebook.evidence.echo,3);
  state.casebook.endings.silent_bell={filed:2};view.paintWorkshop();assert.equal(button("bind").disabled,true);
  state.dispatch.relics.bell=true;view.paintWorkshop();await view.dispatchAction("relic_wear","bell");assert.deepEqual(calls.at(-1),["relic_wear",{item_id:"bell"}]);assert.equal(button("store").disabled,true);
  state.dispatch.active="bell";view.paintWorkshop();await view.dispatchAction("relic_store");assert.deepEqual(calls.at(-1),["relic_store",{}]);
});
test("clock accepts editable ordered symbols, posts only a dated attempt, and locks repeated answers", async context => {
  const {view,bureau,calls,find,room,day} = desk(context);room("clock");assert.match(find(".dp-clues").textContent,/弦月紧贴在灰烬之后/);
  await view.dispatchAction("clock_align");assert.equal(calls.length,0);view.dispatchClockPick("moon");view.dispatchClockPick("undo");for (const key of ["ash","moon","tide"]) view.dispatchClockPick(key);
  await view.dispatchAction("clock_align");assert.deepEqual(calls,[["clock_align",{item_id:day,count:1,choice:"ash-moon-tide"}]]);assert.equal(bureau.clock.guesses.length,0);
  bureau.clock.guesses.push("ash-moon-tide");view.paintWorkshop();assert.equal(find('[data-desk-action="clock_align"]').disabled,true);bureau.clock.guesses.push("tide-moon-ash");view.paintWorkshop();assert.match(find(".dp-sheet").textContent,/今日齿轮已锁/);
});
test("a signed commission disables alternatives, binds the selected leader, and exposes progress before claim", async context => {
  const {view,bureau,calls,find,room,day} = desk(context);room("contract");await view.dispatchAction("contract_sign","courier");assert.deepEqual(calls.at(-1),["contract_sign",{item_id:day,girl_id:"c01",choice:"courier"}]);
  bureau.contract={id:"courier",leader:"c01",progress:1,claimed:false};view.paintWorkshop();assert.equal(find('[data-desk-key="choir"]').disabled,true);assert.equal(find('[data-desk-action="dispatch_contract_claim"]').disabled,true);bureau.contract.progress=2;view.paintWorkshop();await view.dispatchAction("dispatch_contract_claim");assert.deepEqual(calls.at(-1),["dispatch_contract_claim",{item_id:day}]);assert.equal(bureau.contract.claimed,false);
});
test("fog trail shows but disables unreachable lanes, does not locally award cargo, and can bank instead", async context => {
  const {view,bureau,calls,find,room,day} = desk(context);room("route");assert.equal(find('[data-route-cell="0"]').disabled,false);await view.dispatchAction("route_step","left");assert.deepEqual(calls.at(-1),["route_step",{item_id:day,count:1,choice:"left"}]);assert.equal(bureau.route.track.length,0);
  Object.assign(bureau.route,{track:[0],light:3,cargo:3});view.paintWorkshop();assert.equal(find('[data-route-cell="5"]').disabled,true);assert.equal(find('[data-route-cell="4"]').disabled,false);await view.dispatchAction("route_bank");assert.deepEqual(calls.at(-1),["route_bank",{item_id:day,count:1}]);
  bureau.route.status="banked";view.paintWorkshop();assert.equal(find('[data-desk-action="route_bank"]').disabled,true);
});
test("star-map inks are reversibly painted and submit only the three-by-three layout and expected page", async context => {
  const {view,state,calls,find,room} = desk(context);room("mosaic");view.dispatchInk(0);view.dispatchInk(0);assert.equal(find('[data-mosaic-cell="0"]').dataset.ink,"");
  for (const [ink,cells] of [["a",[0,1,3]],["b",[2,5,8]],["c",[4,6,7]]]) {view.dispatchInk(ink);cells.forEach(pos => view.dispatchInk(pos));}
  await view.dispatchAction("mosaic_place");assert.deepEqual(calls.at(-1),["mosaic_place",{item_id:"2026-W48",count:1,choice:"aab-acb-ccb"}]);assert.equal(state.dispatch.mosaic.cleared,0);
  state.dispatch.mosaic.cleared=3;view.paintWorkshop();assert.match(find(".dp-sheet").textContent,/三幅星图已收入/);assert.equal(find("[data-mosaic-cell]"),null);
});
test("loaners swap seats, use independent tactics, reveal the authoritative log, and never change the player squad", async context => {
  const {view,state,bureau,rules,calls,find,room,day} = desk(context);room("mirror");const pair = rules.mirrors[0].pool.slice(0,2).reverse();
  view.dispatchPick("first",pair[0]);view.dispatchPick("tactic","spark");await view.dispatchAction("mirror_trial");assert.deepEqual(calls.at(-1),["mirror_trial",{item_id:day,count:1,squad:pair,choice:"spark"}]);assert.deepEqual(state.squad,["c01"]);
  Object.assign(bureau.mirror,{attempts:1,won:true,last:{squad:pair,choice:"spark",win:true,alive:2,rounds:5,casts:3,notes:["第 2 波 · 第 4 回合 · 怒气技"]}});view.paintWorkshop();assert.match(find("[data-mirror-log]").textContent,/胜利.*5 回合.*3 次我方施技/);assert.equal(find('[data-desk-action="mirror_trial"]').disabled,true);
});

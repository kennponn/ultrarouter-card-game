const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const {JSDOM} = require("jsdom");
const read = name => fs.readFileSync(path.join(__dirname,"..",name),"utf8");
const legacy = JSON.parse(read("backend/model/gacha_catalog.json")).STAGES;

function sampleVoyage() {
  const book = {stages:{}, chapters:[], resources:["res_manifest"], first_clear:[{item:"field_notes",amount:1}], first_three:[{item:"expedition_atlas",amount:1}]};
  for (let chapter = 1; chapter <= 2; chapter++) {
    const region = {id:`c${chapter}`,name:`海域${chapter}`,landmark:`灯塔${chapter}`,routes:{},chests:{}};
    for (const [mode,count] of [["story",8],["hard",3],["nightmare",2]]) {
      const ids = [];
      for (let index=0; index<count; index++) {
        const id = mode === "story" && index === 0 ? chapter === 1 ? "s1" : "s4" : `v${chapter}${mode[0]}${index+1}`;
        ids.push(id);
        book.stages[id] = {...legacy[0], id, name:`${chapter}区${mode}${index+1}`, kind:mode, captain_min:mode === "hard" ? 8 : mode === "nightmare" ? 15 : 0, requires:index ? [ids[index-1]] : [], prize:[{item:"gear_rivet",amount:2}], sweep_prize:mode === "nightmare" ? undefined : [{item:"forge_ore",amount:2}]};
      }
      region.routes[mode] = ids;
      region.chests[mode] = [1,2,3].map(tier => ({id:`c${chapter}-${mode}-${tier}`,stars:count*tier,prize:[{item:"gold",amount:150*tier}]}));
    }
    book.chapters.push(region);
  }
  book.stages.s4.chapter_gate = "v1s8";
  book.stages.res_manifest = {...legacy[0],id:"res_manifest",name:"领航抄录所",kind:"resources",cost:6,requires:["s1"],daily_limit:3,prize:[{item:"field_notes",amount:2}], sweep_prize:[{item:"field_notes",amount:2}]};
  return book;
}

function voyageHarness(context) {
  const dom = new JSDOM(read("index.html"), {runScripts:"outside-only"});
  context.after(() => dom.window.close());
  const doc = dom.window.document, book = sampleVoyage();
  const save = {gold:700, token:20, stamina:120, staminaMax:120, squad:["c08"], girls:{}, mats:{}, daily:{date:"2026-11-27", voyage_attempts:{}}, captain:{level:7}, cleared:{}, stars:{}, voyage_claims:{}};
  dom.window.eval(["config","data","store","ui","reward-chests","voyage-map","voyage"].map(name => read(`js/${name}.js`)).join("\n;\n") + `\nconst BAT=null;function requireGame(){return Store.ready()} function renderTopbar(){} function toast(){} function closeOnly(){closeModal()} function nav(page){document.body.dataset.page=page;if(page==='page-stage')renderStage()} function startBattle(id){window.started=id} window.mapAPI={Store,openVoyage,openVoyageChests,renderStage,renderVoyage,renderVoyageSummary,voyageLedger,updateStageRunLimits,voyageResultSummary,voyageStage,closeOnly,setup(state){SAVE=state}};`);
  const api = dom.window.mapAPI, find = selector => doc.querySelector(selector), all = selector => [...doc.querySelectorAll(selector)];
  api.Store.ready = () => true; api.Store.profile = {voyage:book,items:JSON.parse(read("backend/model/gacha_items.json"))}; api.setup(save);
  api.Store.serverNow = () => Date.parse("2026-11-27T12:00:00+08:00");
  const calls = []; api.Store.mutate = async (action, fields) => {calls.push([action,JSON.parse(JSON.stringify(fields))]);return null;};
  doc.body.dataset.page = "page-stage"; api.renderStage();
  return {dom,doc,api,save,book,calls,find,all};
}

test("voyage draws navigable desktop and portrait routes, independent modes and chapter gates", context => {
  const {dom,save,api,find,all} = voyageHarness(context);
  assert.equal(all("[data-voy-point]").length,8); assert.equal(all("[data-voy-mode]").length,3); assert.equal(all("[data-voy-chest]").length,3);
  assert.ok(find(".map-panel > .map-scroll > #seaMap .voyage-route-plane")); assert.ok(find("#seaMap .voyage-ocean .voyage-depth")); assert.ok(find("#seaMap .voyage-landmark text"));
  assert.equal(all(".voyage-island").length,8); assert.equal(all(".voyage-route").length,7); assert.equal(all(".voyage-node-portrait img").length,8);
  assert.ok(find(".map-panel > .map-back")); assert.ok(find(".map-panel > #wallet .wallet-plus")); assert.ok(find(".map-bottom-layer > #difficultyNav")); assert.ok(find(".map-bottom-layer > #starRewards"));
  find('[data-voy-step="1"]').click(); assert.match(find(".voy-status").textContent,/完成前章/); assert.match(find(".voy-heading").textContent,/海域1/);
  save.voyage_passes = {s4:true}; find('[data-voy-step="1"]').click(); assert.match(find(".voy-heading").textContent,/海域2/);
  find('[data-voy-step="-1"]').click(); find('[data-voy-mode="hard"]').click(); assert.equal(all("[data-voy-point]").length,3);
  find("[data-voy-point]").click(); assert.equal(find("[data-voy-start]").disabled,true); assert.match(find(".voy-gate").textContent,/Lv\.8/);
  api.closeOnly(); find('[data-voy-mode="nightmare"]').click(); assert.equal(all("[data-voy-point]").length,2);
  find('[data-voy-mode="story"]').click();
  for (const [width,height,layout,box,left,top] of [[390,844,"portrait","0 0 400 640",95/400*100,530/640*100],[844,390,"compact","0 0 820 260",95/820*100,181/260*100],[1366,900,"landscape","0 0 820 510",87/820*100,((345-124)*1.28+94)/510*100]]) {
    dom.window.innerWidth=width; dom.window.innerHeight=height; dom.window.dispatchEvent(new dom.window.Event("resize"));
    assert.equal(find(".voyage-world").dataset.layout,layout); assert.equal(find(".voyage-chart").getAttribute("viewBox"),box);
    assert.ok(Math.abs(parseFloat(find("[data-voy-point]").style.left)-left)<.0001); assert.ok(Math.abs(parseFloat(find("[data-voy-point]").style.top)-top)<.0001);
  }
});

test("home star rewards count unique mainline records and deep-link to a claimable tier without claiming it", context => {
  const {save,book,api,calls,find,all} = voyageHarness(context);
  Object.assign(save.stars,{s1:3,v1h1:3,v2n1:1,res_manifest:3,r1:3,unknown:3});
  Object.assign(save.cleared,{s1:true,v1h1:true,v2n1:true,res_manifest:true,r1:true});
  book.chapters[1].routes.story[0]="s1";
  api.renderVoyageSummary();
  assert.match(find("#home-chest-progress").textContent,/7 \/ 75 星 · 1 档可领取/);
  assert.match(find("#home-campaign-progress").textContent,/2 章/);
  api.openVoyageChests();
  assert.equal(find('[data-voy-mode="hard"]').getAttribute("aria-pressed"),"true");
  assert.equal(find("[data-voy-chest-detail]").dataset.voyChestDetail,"c1-hard-1"); assert.equal(find("[data-voy-claim]").disabled,false);
  assert.match(find('[data-voy-chest="c1-hard-1"]').textContent,/可领取/); assert.match(find('[data-voy-chest="c1-hard-2"]').textContent,/还差 3 星/);
  assert.equal(all('#starRewards .chest-art img[src^="assets/world/box_"]').length,3); assert.equal(find('#starRewards progress').max,9); assert.deepEqual(calls,[]);
  api.closeOnly(); save.voyage_claims['c1-hard-1']=true; api.openVoyageChests(); assert.equal(find("[data-voy-chest-detail]").dataset.voyChestDetail,"c1-story-1");
  assert.match(find("[data-voy-claim]").textContent,/还差 5 星/); assert.equal(find("[data-voy-claim]").disabled,true);
  api.closeOnly(); delete api.Store.profile.voyage; api.renderVoyageSummary(); api.openVoyageChests();
  assert.match(find("#home-chest-progress").textContent,/待同步/); assert.equal(find("#voyage-unavailable").hidden,false);
  assert.equal(find("#voyage-unavailable a").hash,"#rank-demo-stars"); assert.equal(find("#voyage-map").hidden,true); assert.deepEqual(calls,[]);
});

test("locked resource previews name the fixed loot and direct links issue only the selected stage", context => {
  const {api,find,save,dom} = voyageHarness(context);
  api.openVoyage("resources","res_manifest");
  assert.equal(find("[data-voy-detail]").dataset.voyDetail,"res_manifest"); assert.equal(find("[data-voy-start]").disabled,true); assert.match(find(".voy-detail").textContent,/斥候札记.*×2/s);
  save.cleared.s1=true; save.daily.voyage_attempts.res_manifest=3; api.openVoyage("resources","res_manifest"); assert.match(find(".voy-gate").textContent,/零点刷新/);
  save.daily.voyage_attempts.res_manifest=0; api.openVoyage("resources","res_manifest"); find("[data-voy-start]").click(); assert.equal(dom.window.started,"res_manifest");
  assert.equal(save.stamina,120); assert.deepEqual(save.mats,{}); assert.deepEqual(save.stars,{});
  assert.equal(api.voyageStage("res_manifest").name,"领航抄录所");
});

test("chest requires stars in the selected chapter and difficulty; posts only its stable claim key", async context => {
  const {save,api,find,calls} = voyageHarness(context);
  Object.assign(save.stars,{s1:3,v1s2:3,v1s3:2,v1h1:3});
  Object.assign(save.cleared,{s1:true,v1s2:true,v1s3:true,v1h1:true});
  api.renderStage(); find('[data-voy-chest="c1-story-1"]').click(); await find("[data-voy-claim]").onclick();
  assert.deepEqual(calls,[["voyage_chest",{item_id:"c1-story-1"}]]); assert.equal(save.gold,700); assert.deepEqual(save.voyage_claims,{});
  save.voyage_claims["c1-story-1"]=true; api.closeOnly(); api.renderStage(); find('[data-voy-chest="c1-story-1"]').click(); assert.equal(find("[data-voy-claim]").disabled,true);
  api.closeOnly(); find('[data-voy-mode="hard"]').click(); find('[data-voy-chest="c1-hard-2"]').click(); assert.equal(find("[data-voy-claim]").disabled,true); assert.match(find("[data-voy-claim]").textContent,/还差 3 星/);
  assert.match(api.voyageResultSummary("s1",{win:true,save}),/8\/24/);
});

test("uncleared stars cannot appear in totals or enable sweeps, and all prerequisites apply", context => {
  const {save,book,api,find} = voyageHarness(context);
  Object.assign(save.stars,{s1:3,v1h1:3,res_manifest:3});
  api.renderVoyageSummary();
  assert.equal(api.voyageLedger().stars,0);
  save.cleared.s1=true;
  book.stages.res_manifest.unlockStage='s5';
  api.openVoyage('resources','res_manifest');
  assert.equal(find('[data-voy-start]').disabled,true);
  assert.match(find('.voy-gate').textContent,/通关/);
  save.cleared.s5=true;
  api.openVoyage('resources','res_manifest');
  assert.equal(find('[data-voy-start]').disabled,false);
  assert.equal(find('[data-voy-sweep="1"]').disabled,true);
  save.cleared.res_manifest=true;
  api.openVoyage('resources','res_manifest');
  assert.equal(find('[data-voy-sweep="1"]').disabled,false);
  assert.equal(api.voyageLedger().stars,3);
});

test("sweeps require three stars, available attempts and stamina and submit only the dated server intent", async context => {
  const {save,api,find,calls} = voyageHarness(context);
  save.cleared.s1=true; save.cleared.res_manifest=true; save.stars.res_manifest=2; api.openVoyage("resources","res_manifest");
  assert.equal(find('[data-voy-sweep="1"]').disabled,true); assert.match(find(".voy-sweep").textContent,/不发站点额度/);
  save.stars.res_manifest=3; save.stamina=12; api.openVoyage("resources","res_manifest"); assert.equal(find('[data-voy-sweep="1"]').disabled,false); assert.equal(find('[data-voy-sweep="3"]').disabled,true);
  save.stamina=120; save.daily.voyage_attempts.res_manifest=1; api.openVoyage("resources","res_manifest"); assert.equal(find('[data-voy-sweep="3"]').disabled,true);
  save.daily.voyage_attempts.res_manifest=0; api.Store.activeRun={id:"in_progress"}; api.openVoyage("resources","res_manifest"); assert.equal(find('[data-voy-sweep="1"]').disabled,true);
  api.Store.activeRun=null; api.openVoyage("resources","res_manifest"); await find('[data-voy-sweep="3"]').onclick();
  assert.deepEqual(calls,[["voyage_sweep",{item_id:"res_manifest",count:3,choice:"2026-11-27"}]]); assert.equal(save.stamina,120); assert.equal(save.daily.voyage_attempts.res_manifest,0);
});

test("an idle voyage detail crosses Beijing midnight; only the confirmed new attempts unlock actions", async context => {
  const {dom,save,api,find,calls} = voyageHarness(context);
  let now = Date.parse("2026-11-27T15:59:59.900Z"), boundary, finishRefresh, reads = 0;
  api.Store.serverNow = () => now;
  dom.window.setTimeout = (callback, delay) => { if (delay === 150) boundary = callback; return 0; };
  api.Store.refresh = () => { reads++; return new Promise(resolve => { finishRefresh = resolve; }); };
  save.cleared.s1 = true; save.cleared.res_manifest = true; save.stars.res_manifest = 3; save.daily.voyage_attempts.res_manifest = 3;
  api.openVoyage("resources","res_manifest");
  assert.match(find('[data-voy-left]').textContent,/0\/3/);
  const oldStart = find('[data-voy-start]'), oldSweep = find('[data-voy-sweep="1"]');
  now += 150; boundary(); api.updateStageRunLimits();
  assert.equal(find('[data-voy-start]').disabled,true); assert.equal(find('[data-voy-sweep="1"]').disabled,true);
  assert.equal(find('[data-voy-left]').textContent,"今日次数待同步"); assert.match(find('.voy-gate').textContent,/正在同步/);
  await new Promise(done => setImmediate(done)); assert.equal(reads,1);
  save.daily = {date:"2026-11-28",voyage_attempts:{res_manifest:1}}; finishRefresh({save});
  await new Promise(done => setImmediate(done));
  assert.match(find('[data-voy-left]').textContent,/2\/3/); assert.equal(find('[data-voy-start]').disabled,false);
  assert.equal(find('[data-voy-sweep="1"]').disabled,false); assert.equal(find('[data-voy-sweep="3"]').disabled,true);
  oldStart.onclick(); await oldSweep.onclick(); assert.equal(dom.window.started,undefined); assert.deepEqual(calls,[]);
  await find('[data-voy-sweep="1"]').onclick();
  assert.deepEqual(calls,[["voyage_sweep",{item_id:"res_manifest",count:1,choice:"2026-11-28"}]]);
});

test("a click racing the midnight timer cannot start or sweep; failed refresh waits for explicit retry", async context => {
  const {dom,save,api,find,calls} = voyageHarness(context);
  save.cleared.s1 = true; save.cleared.res_manifest = true; save.stars.res_manifest = 3; api.openVoyage("resources","res_manifest");
  let reads = 0;
  api.Store.serverNow = () => Date.parse("2026-11-27T16:00:00Z");
  api.Store.refresh = async () => { reads++; throw Error("offline"); };
  const start = find('[data-voy-start]'), sweep = find('[data-voy-sweep="3"]');
  assert.equal(start.disabled,false); assert.equal(sweep.disabled,false);
  await sweep.onclick(); start.onclick(); await new Promise(done => setImmediate(done));
  api.updateStageRunLimits(); api.renderStage();
  assert.equal(reads,1); assert.equal(start.disabled,true); assert.equal(sweep.disabled,true);
  assert.equal(dom.window.started,undefined); assert.deepEqual(calls,[]);
  assert.match(find('.voy-gate').textContent,/尚未同步/);
  assert.ok(find('.voy-status [data-voy-retry]')); assert.ok(find('.voy-gate [data-voy-retry]'));
  api.Store.refresh = async () => { reads++; save.daily = {date:"2026-11-28",voyage_attempts:{}}; return {save}; };
  find('.voy-gate [data-voy-retry]').click(); await new Promise(done => setImmediate(done));
  assert.equal(reads,2); assert.match(find('[data-voy-left]').textContent,/3\/3/);
  assert.equal(find('.voy-status [data-voy-retry]'),null); assert.equal(find('[data-voy-sweep="3"]').disabled,false);
  find('[data-voy-start]').click(); assert.equal(dom.window.started,"res_manifest");
});

test("a stale challenge click never opens a run and a late refresh cannot reopen a dismissed detail", async context => {
  const {dom,save,api,find} = voyageHarness(context);
  save.cleared.s1 = true; api.openVoyage("resources","res_manifest");
  api.Store.serverNow = () => Date.parse("2026-11-27T16:00:00Z");
  let finishRefresh;
  api.Store.refresh = () => new Promise(resolve => { finishRefresh = resolve; });
  find('[data-voy-start]').click(); await new Promise(done => setImmediate(done));
  assert.equal(dom.window.started,undefined); assert.equal(find('[data-voy-start]').disabled,true);
  assert.match(find('.voy-gate').textContent,/同步/); api.closeOnly();
  save.daily = {date:"2026-11-28",voyage_attempts:{}}; finishRefresh({save}); await new Promise(done => setImmediate(done));
  assert.equal(find('#modal-mask').classList.contains('hidden'),true); assert.equal(dom.window.started,undefined);
});

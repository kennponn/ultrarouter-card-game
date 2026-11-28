const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");
const read = name=>fs.readFileSync(path.join(__dirname,"..",name),"utf8");
const profile = Object.fromEntries(["equipment","items","cultivation","artifact_grades"].map(key=>[key,JSON.parse(read(`backend/model/gacha_${key}.json`))]));
const plain = value=>JSON.parse(JSON.stringify(value));

function fixture() {
  const own={star:5,level:40,sockets:3,gems:["ruby~red!2.1","ruby@6","ruby~white"],chips:["edge_1~gold!2.1","edge_2~blue"]};
  const save={girls:{c01:own},squad:["c01"],mats:{gear_thread:6,gear_plate:6,forge_ore:10,gear_rivet:2,gem_ticket:1,chip_ticket:1},gem_inventory:{"ruby~red!2.1":1,"ruby~red":3,ruby:2},chip_inventory:{"edge_1~orange":2}};
  const panel={innerHTML:"",dataset:{}}, store={profile,sessionEpoch:1,ready:()=>true}, ctx=vm.createContext({Store:store,save,esc:String,document:{querySelectorAll:()=>[],getElementById:()=>panel}});
  const api=vm.runInContext(["data","atelier","workshop"].map(name=>read(`js/${name}.js`)).join("\n;\n")+`\n;SAVE=save;({artifactIdentity,artifactProgressKey,artifactGradeEffects,artifactPrices,artifactBonuses,equippedRefinement,refinementItem,workshopItems,workshopItemIcon,workshopSlotInfo,workshopSlotControls,workshopRefinementInfo,workshopSlotAction,workshopBagBody,atelierItemDetails,workshopDrawReceipt,renderArtifactPool,drawPoolState,setView(view){workshopView=view},setBag(tab){workshopBag.tab=tab}})`,ctx);
  ctx.growthHeroOptions=()=>"";
  return {api,store,own,save,panel};
}

test("all seven grades roll every template; old diagonal keys retain their physical level and rank",()=>{
  const {api}=fixture(), grades=profile.artifact_grades;
  assert.deepEqual(grades.qualities.map(option=>option.weight),[4200,2800,1650,860,350,120,20]);
  for(const [tab,count] of [["gems",5],["chips",18]]) for(const grade of grades.qualities) {
    const items=api.workshopItems(tab,undefined,false,[],grade.id);assert.equal(items.length,count);
    for(const item of items) { assert.equal(item.quality,grade.id);assert.equal(api.artifactProgressKey(api.artifactIdentity(item.id)),item.id);assert.ok(item.name.startsWith(grade.name));assert.match(api.workshopItemIcon(item),new RegExp(`data-artifact-quality="${grade.id}"`)); }
  }
  for (let legacy=1;legacy<=12;legacy++) for(const name of ["ruby","edge_1"]) {const id=`${name}@${legacy}`, item=api.artifactIdentity(id);assert.equal(item.level,legacy+1);assert.equal(item.rank,legacy);assert.equal(api.artifactProgressKey(item),id);}
  for(const id of ["ruby!14.12","ruby!2.2","ruby~red!110.12","edge_1~blue!120.12"]) assert.ok(api.artifactIdentity(id));
  for(const id of ["ruby~blue","edge_2~purple","edge_7","ruby~red@2","ruby!7.6","ruby~red!13.12","ruby!1.1","ruby~red!01.1","ruby~red!121.1","ruby~red!119.13","ruby~red!+1.1","ruby~red\n","gem_1"]) assert.equal(api.artifactIdentity(id),null,id);
});

test("server profile controls two independent prices and authoritative immutable training requests",async()=>{
  const {api,store,own,save}=fixture(), requests=[];
  store.mutate=async(action,data)=>{requests.push([action,plain(data)]);return null;};
  for (const tab of ["gems","chips"]) {
    const kind=tab.slice(0,-1),view={tab,id:"c01",slot:1,item:tab==="gems" ? "ruby" : "edge_1~orange",epoch:1};api.setView(view);
    const info=api.workshopSlotInfo(view),key=own[tab][0],prices=api.artifactPrices(tab,info.equipped.refine);
    assert.deepEqual(plain(prices.level),{[tab==="gems" ? "gear_thread" : "gear_plate"]:5,forge_ore:1});assert.deepEqual(plain(prices.rank),{gear_rivet:2,[`${kind}_ticket`]:1});
    const controls=()=>api.workshopSlotControls(view,api.workshopSlotInfo(view));assert.ok(controls().find(opt=>opt.action===`${kind}_rank`).ready);
    const before=plain(save);await api.workshopSlotAction(`${kind}_level`);await api.workshopSlotAction(`${kind}_rank`);assert.deepEqual(plain(save),before);
    assert.deepEqual(requests.slice(-2),["level","rank"].map(action=>[`${kind}_${action}`,{girl_id:"c01",count:1,item_id:key}]));
    if(tab==="chips") {own.chip_locks=1;assert.equal(controls().find(opt=>opt.action==="chip_rank").ready,false);own.chip_locks=0;}
    const html=api.workshopRefinementInfo(view,info);assert.ok(html.includes(`data-refinement="${key}"`));assert.ok(html.includes("120"));assert.ok(html.includes("12阶"));assert.ok(api.atelierItemDetails(key).includes(info.equipped.gradeName));
  }
  const cap=api.refinementItem("gems","ruby~red!120.12");assert.equal(api.artifactPrices("gems",cap).level,null);assert.equal(api.artifactPrices("gems",cap).rank,null);
  const late=api.refinementItem("gems","wind~white!119.1"),next=api.refinementItem("gems","wind~white!120.1");assert.equal(next.extraHp-late.extraHp,2);
  assert.equal(api.equippedRefinement({star:1,gems:["wind~white"]}).spd+3,2);assert.equal(api.equippedRefinement({star:1,gems:["wind~red"]}).spd+3,9);
  assert.equal(api.artifactBonuses(own).cr,6);assert.equal(api.artifactGradeEffects(cap).filter(effect=>effect.active).length,6);
});

test("public pool, bag and final receipt render the actual received tier independently of preview",()=>{
  const {api,panel,store}=fixture();api.setBag("gems");assert.match(api.workshopBagBody(),/data-bag-item="ruby~red!2.1"/);
  const draw=api.drawPoolState();draw.id="gems";draw.quality="white";draw.loot=[{item_id:"ruby~red",quantity:2}];draw.drops=["不可用于推断品质"];
  assert.equal(api.renderArtifactPool(),true);assert.equal((panel.innerHTML.match(/data-pool-quality=/g)||[]).length,7);assert.match(panel.innerHTML,/0\.2%/);assert.match(panel.innerHTML,/data-receipt-item="ruby~red"/);assert.match(panel.innerHTML,/绯红·赤曜石/);assert.match(panel.innerHTML,/×2/);
  store.sessionEpoch++;assert.deepEqual(plain(api.drawPoolState().loot),[]);
});

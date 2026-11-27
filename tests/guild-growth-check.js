const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const { JSDOM } = require("jsdom");

const clone = value => JSON.parse(JSON.stringify(value));
const root = path.resolve(__dirname,"..");
const read = file => fs.readFileSync(path.join(root,file),"utf8");

function guildFixture() {
  return {
    name:"协力母港",code:"0000000E",leader_id:14,role:"leader",level:9,xp:184000,next_xp:260000,
    member_limit:50,coins:160,can_assign:true,can_manage:true,boss_week:"2026-W48",attempt_limit:2,attempts_today:0,
    members:[{user_id:14,name:"会长",role:"leader",can_kick:false},{user_id:22,name:"<img src=x>",role:"elder",can_kick:true},{user_id:23,name:"另一副会长",role:"deputy",can_kick:false}],
    activity:{day:"2026-11-27",week:"2026-W48",contribution:5,
      tasks:[
        {id:"checkin",name:"母港签到",goal:"每日报到",target:1,progress:0,xp:20,coins:20,can_claim:true,claimed:false},
        {id:"sortie",name:"协力巡航",goal:"赢得三场副本",target:3,progress:3,xp:60,coins:45,can_claim:true,claimed:false},
        {id:"training",name:"整备伙伴",goal:"完成三次培养",target:3,progress:1,xp:40,coins:30,can_claim:false,claimed:false},
        {id:"arena",name:"演武交流",goal:"完成两场竞技",target:2,progress:2,xp:50,coins:35,can_claim:false,claimed:true}],
      missions:[
        {id:"supply",name:"物资集结",goal:"交付20份物资",progress:20,target:20,xp:800,coins:150,resources:{training_ink:20,training_manual:6},can_claim:true,claimed:false},
        {id:"raid",name:"并肩讨伐",goal:"讨伐30次",progress:4,target:30,xp:1200,coins:250,resources:{forge_ore:20,break_seal:4},can_claim:false,claimed:false},
        {id:"active",name:"百舸争流",goal:"领取50项任务",progress:50,target:50,xp:1800,coins:400,resources:{expedition_atlas:8,weapon_core:3},can_claim:false,claimed:true}],
      sources:[{name:"签到",detail:"每日返回母港",xp:20,coins:20},{name:"协力讨伐",detail:"每次战斗",xp:30,coins:25}]},
    raid_stages:[
      {name:"暗礁守卫",hp:0,max_hp:300000,resources:{training_manual:12,training_ink:30,break_seal:4,mat_sr:12}},
      {name:"深潮统领",hp:900000,max_hp:900000,resources:{training_manual:20,training_ink:50,break_seal:8,forge_ore:35,mat_ssr:8}},
      {name:"深渊主宰",hp:2400000,max_hp:2400000,resources:{expedition_atlas:15,training_ink:80,break_seal:15,weapon_core:6}},
      {name:"永夜司钟",hp:5000000,max_hp:5000000,resources:{expedition_atlas:25,training_ink:120,weapon_core:12,gem_ticket:8}},
      {name:"终焉潮皇",hp:9000000,max_hp:9000000,resources:{expedition_atlas:40,training_ink:180,break_seal:40,weapon_core:20,chip_ticket:12}}
    ].map((stage,index)=>({...stage,stage:index+1,required_level:1+index*2,coins:300+index*200,unlocked:true,active:index===1,cleared:index===0,participated:index<2,can_claim:index===0,claimed:false,description:"全体成员共同推进阶段首领"})),
    shop:[]
  };
}

function harness(context) {
  const dom = new JSDOM('<body data-page="page-fleet"><div id="fleet-body"></div><div id="modal-box"></div></body>',{runScripts:"outside-only",url:"https://example.invalid/gacha/"});
  context.after(()=>dom.window.close());
  const w=dom.window,calls=[],reads=[],notices=[];
  const Store=w.Store={sessionEpoch:1,ready:()=>true,profile:{user_id:14,guild:guildFixture(),items:{items:{training_manual:{name:"<专用训练手册>"}}}},
    refresh:async()=>{reads.push("profile");return Store.profile;},
    mutate(action,fields){return new Promise(resolve=>calls.push({action,fields:clone(fields),resolve}));},
    api:async()=>{throw new Error("unexpected directory request");}};
  Object.assign(w,{SITE:{ep:{fleet:"/fleet"}},SAVE:{stamina:90,squad:["c01"],token:5,mats:{training_ink:3}},requireGame:()=>Store.ready(),renderTopbar(){},toast:text=>notices.push(text)});
  w.showModal=html=>{w.document.getElementById("modal-box").innerHTML=html;};
  w.closeModal=()=>{w.document.getElementById("modal-box").innerHTML="";};
  const ui=read("js/ui.js");
  w.eval(ui.slice(ui.indexOf("function esc("),ui.indexOf("/* ---------- 页面路由")));
  w.eval(read("js/fleet.js"));
  function click(selector){const target=w.document.querySelector(selector);assert.ok(target,selector);return w.fleetClick({target});}
  function forged(command,value){const target=w.document.createElement("button");target.dataset.fleetAct=command;target.dataset.value=value;return w.fleetClick({target});}
  return {w,Store,document:w.document,calls,reads,notices,click,forged};
}

test("current raid damage board renders server standings, departed members and weekly reset rules", async context => {
  const h = harness(context);
  h.Store.profile.guild.raid_damage_ranking = {week:"2026-W48",stage:2,stage_name:"深潮统领",all_cleared:false,entries:[
    {rank:1,user_id:22,name:"<img src=x>",damage:60000,attempts:3,member:false},
    {rank:2,user_id:14,name:"会长",damage:40000,attempts:2,member:true}
  ]};
  await h.w.renderFleet(); await h.click('[data-fleet-tab="raid"]');
  const board = h.document.querySelector('.guild-damage-board');
  assert.match(board.textContent,/深潮统领/);
  assert.match(board.textContent,/每周一 00:00（北京时间）/);
  assert.equal(board.querySelector('img'),null);
  assert.match(board.querySelector('li').textContent,/<img src=x>/);
  assert.match(board.querySelector('li').textContent,/已离会/);
  assert.match(board.querySelector('li').textContent,/60.0%/);
  assert.match(board.querySelector('.is-mine').textContent,/40,000/);
  assert.equal(h.calls.length,0);
  h.Store.profile.guild.raid_damage_ranking = {week:"2026-W49",stage:1,stage_name:"暗礁守卫",all_cleared:false,entries:[]};
  await h.w.renderFleet();
  assert.equal(h.document.querySelectorAll('.guild-damage-list li').length,0);
  assert.match(h.document.querySelector('.guild-damage-board').textContent,/本阶段还没有讨伐记录/);
  assert.doesNotMatch(h.document.querySelector('.guild-damage-board').textContent,/深潮统领|60,000/);
});

test("kick controls follow per-member permission and the confirmation identifies the exact escaped target",async context=>{
  const h=harness(context);
  h.Store.profile.guild.role="deputy";
  await h.w.renderFleet();await h.click('[data-fleet-tab="members"]');
  assert.equal(h.document.querySelectorAll('[data-fleet-act="kick"]').length,1);
  assert.equal(h.document.querySelector('[data-fleet-act="kick"]').dataset.value,"22");
  await h.forged("kick","14");await h.forged("kick","23");
  assert.equal(h.document.querySelector('[data-fleet-kick-confirm]'),null);
  await h.click('[data-fleet-act="kick"]');
  const modal=h.document.getElementById("modal-box");
  assert.match(modal.textContent,/<img src=x>/);
  assert.match(modal.textContent,/长老/);
  assert.match(modal.textContent,/22/);
  assert.equal(modal.querySelector("img"),null);
  assert.equal(h.calls.length,0);
  h.document.querySelector('[data-fleet-kick-cancel]').click();
  assert.equal(h.document.querySelector('[data-fleet-kick-confirm]'),null);
  assert.equal(h.calls.length,0);
});

test("kick confirmation posts one exact ID and membership changes only after a successful refreshed profile",async context=>{
  const h=harness(context);
  await h.w.renderFleet();await h.click('[data-fleet-tab="members"]');await h.click('[data-fleet-act="kick"]');
  const confirm=h.document.querySelector('[data-fleet-kick-confirm]');
  const pending=confirm.onclick();await confirm.onclick();
  assert.deepEqual(h.calls.map(({action,fields})=>({action,...fields})),[{action:"guild_kick",opponent_id:22}]);
  assert.equal(h.Store.profile.guild.members.length,3);
  assert.equal(h.document.querySelectorAll('.guild-member-list article').length,3);
  h.Store.refresh=async()=>{h.reads.push("profile");h.Store.profile.guild={...h.Store.profile.guild,members:h.Store.profile.guild.members.filter(member=>member.user_id!==22)};return h.Store.profile;};
  h.calls[0].resolve({drops:[]});await pending;
  assert.equal(h.Store.profile.guild.members.length,2);
  assert.equal(h.document.querySelectorAll('.guild-member-list article').length,2);
  assert.equal(h.document.querySelector('[data-fleet-act="kick"]'),null);
});

test("changed account, guild or permissions and a cancelled dialog cannot submit an earlier kick",async context=>{
  for(const change of [h=>h.Store.sessionEpoch++,h=>h.Store.profile.guild.code="000000FF",h=>h.Store.profile.guild.members[1].can_kick=false,h=>h.Store.profile.guild=null,h=>h.w.closeModal()]){
    const h=harness(context);
    await h.w.renderFleet();await h.click('[data-fleet-tab="members"]');await h.click('[data-fleet-act="kick"]');
    const confirm=h.document.querySelector('[data-fleet-kick-confirm]');
    change(h);await confirm.onclick();
    assert.equal(h.calls.length,0);
  }
});

test("a rejected kick keeps the member visible and can be deliberately retried",async context=>{
  const h=harness(context);
  await h.w.renderFleet();await h.click('[data-fleet-tab="members"]');await h.click('[data-fleet-act="kick"]');
  const pending=h.document.querySelector('[data-fleet-kick-confirm]').onclick();
  h.calls[0].resolve(null);await pending;
  assert.equal(h.Store.profile.guild.members.length,3);
  assert.equal(h.document.querySelectorAll('.guild-member-list article').length,3);
  await h.click('[data-fleet-act="kick"]');
  const retry=h.document.querySelector('[data-fleet-kick-confirm]').onclick();
  assert.equal(h.calls.length,2);
  h.calls[1].resolve(null);await retry;
});

test("daily and weekly claims send only eligible rule IDs and apply resources after the server responds",async context=>{
  const h=harness(context);
  await h.w.renderFleet();await h.click('[data-fleet-tab="growth"]');
  assert.equal(h.document.querySelector('[data-value="training"]').disabled,true);
  assert.equal(h.document.querySelector('[data-value="arena"]').disabled,true);
  const before=clone(h.w.SAVE),beforeCoins=h.Store.profile.guild.coins;
  const daily=h.click('[data-fleet-act="activityClaim"][data-value="sortie"]');
  await h.click('[data-fleet-act="activityClaim"][data-value="sortie"]');
  assert.deepEqual(h.calls.map(({action,fields})=>({action,...fields})),[{action:"guild_activity_claim",item_id:"sortie"}]);
  assert.equal(h.Store.profile.guild.coins,beforeCoins);
  assert.deepEqual(clone(h.w.SAVE),before);
  h.Store.refresh=async()=>{h.reads.push("profile");const task=h.Store.profile.guild.activity.tasks.find(task=>task.id==="sortie");task.claimed=true;task.can_claim=false;h.Store.profile.guild.coins=beforeCoins+45;h.Store.profile.guild.xp+=60;return h.Store.profile;};
  h.calls[0].resolve({drops:["公会币 ×45"]});await daily;
  assert.equal(h.Store.profile.guild.coins,beforeCoins+45);
  assert.equal(h.document.querySelector('[data-value="sortie"]').disabled,true);
  assert.match(h.document.querySelector('.guild-wallet').textContent,/205/);
  const weekly=h.click('[data-fleet-act="missionClaim"][data-value="supply"]');
  assert.deepEqual(h.calls[1].fields,{item_id:"supply"});
  assert.equal(h.calls[1].action,"guild_mission_claim");
  assert.equal(h.w.SAVE.mats.training_ink,3);
  h.Store.refresh=async()=>{h.reads.push("profile");const mission=h.Store.profile.guild.activity.missions[0];mission.can_claim=false;mission.claimed=true;h.Store.profile.guild.coins+=150;return h.Store.profile;};
  h.w.SAVE={...h.w.SAVE,mats:{training_ink:23,training_manual:6}};
  h.calls[1].resolve({drops:["研训墨 ×20","训练手册 ×6"]});await weekly;
  assert.equal(h.w.SAVE.mats.training_ink,23);
  assert.equal(h.document.querySelector('[data-fleet-act="missionClaim"][data-value="supply"]').disabled,true);
  assert.match(h.notices.at(-1),/研训墨 ×20/);
});

test("unknown, disabled and now-ineligible task IDs cannot be submitted by forged or stale buttons",async context=>{
  const h=harness(context);
  await h.w.renderFleet();await h.click('[data-fleet-tab="growth"]');
  const stale=h.document.querySelector('[data-fleet-act="activityClaim"][data-value="sortie"]');
  h.Store.profile.guild.activity.tasks[1].can_claim=false;
  await h.w.fleetClick({target:stale});
  for(const id of ["unknown","checkin","training","arena","supply","sortie "])await h.forged("activityClaim",id);
  for(const id of ["unknown","raid","active","sortie","supply "])await h.forged("missionClaim",id);
  assert.equal(h.calls.length,0);
  assert.equal(h.w.SAVE.token,5);
});

test("daily check-in sends no client reward values and becomes unavailable when server eligibility expires",async context=>{
  const h=harness(context);
  await h.w.renderFleet();await h.click('[data-fleet-tab="growth"]');
  const pending=h.click('[data-fleet-act="checkin"]');
  assert.deepEqual(h.calls[0].fields,{});
  assert.equal(h.calls[0].action,"guild_checkin");
  assert.equal(h.Store.profile.guild.coins,160);
  h.calls[0].resolve(null);await pending;
  h.Store.profile.guild.activity.tasks[0].can_claim=false;
  await h.forged("checkin","checkin");
  assert.equal(h.calls.length,1);
});

test("failed and stale claim responses cannot fabricate grants or paint another account's guild",async context=>{
  const h=harness(context);
  await h.w.renderFleet();await h.click('[data-fleet-tab="growth"]');
  const failed=h.click('[data-fleet-act="missionClaim"][data-value="supply"]');
  h.calls[0].resolve(null);await failed;
  assert.equal(h.Store.profile.guild.activity.missions[0].claimed,false);
  assert.equal(h.w.SAVE.mats.training_ink,3);
  const stale=h.click('[data-fleet-act="missionClaim"][data-value="supply"]');
  const reads=h.reads.length;
  h.Store.sessionEpoch++;
  h.calls[1].resolve({drops:["不应显示的旧账号奖励"]});await stale;
  assert.equal(h.reads.length,reads);
  assert.deepEqual(h.notices,[]);
  assert.equal(h.w.SAVE.mats.training_ink,3);
});

test("all five raid stages display named cultivation resources and preserve server claim eligibility",async context=>{
  const h=harness(context);
  await h.w.renderFleet();await h.click('[data-fleet-tab="raid"]');
  assert.equal(h.document.querySelectorAll('.guild-raid-card').length,5);
  const pane=h.document.getElementById("fleet-pane");
  for(const name of ["<专用训练手册>","研训墨","晋阶章","元素结晶","星锻矿","星屑","远征图鉴","觉醒核心","宝石券","芯片券"])assert.ok(pane.textContent.includes(name),name);
  assert.doesNotMatch(pane.textContent,/training_manual|training_ink|break_seal|expedition_atlas|weapon_core|gem_ticket|chip_ticket|mat_ssr|mat_sr|forge_ore/);
  assert.equal(pane.querySelectorAll('.guild-materials img').length,0);
  assert.equal(h.document.querySelectorAll('[data-fleet-act="bossClaim"]:not(:disabled)').length,1);
  const pending=h.click('[data-fleet-act="bossClaim"][data-value="1"]');
  assert.deepEqual(h.calls[0].fields,{count:1});
  h.calls[0].resolve(null);await pending;
  assert.equal(h.w.SAVE.mats.training_ink,3);
});

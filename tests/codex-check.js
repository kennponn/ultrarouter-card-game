const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const { JSDOM } = require("jsdom");

const root = path.resolve(__dirname, "..");
const read = file => fs.readFileSync(path.join(root, file), "utf8");
const copy = value => JSON.parse(JSON.stringify(value));
const tick = () => new Promise(resolve => setImmediate(resolve));
const prose = value => typeof value === "string" ? value : value?.text || value?.description || "";
const normalized = value => String(value).replace(/\s+/g,"");
function includesProse(node, value, message) {
  assert.ok(normalized(node.textContent).includes(normalized(prose(value))),message);
}

function harness(context) {
  const dom = new JSDOM(read("index.html"), { runScripts:"outside-only", url:"https://game.test/gacha/", pretendToBeVisual:true });
  context.after(() => dom.window.close());
  const w = dom.window, calls = [], pending = [], storage = [], errors = [], effects = [];
  let paused = false;
  w.CodexEffects = {
    play(canvas,options) { paused=false; effects.push({method:"play",canvas,options}); },
    stop() { effects.push({method:"stop"}); },
    togglePause() { paused=!paused; effects.push({method:"pause",paused}); return paused; },
    setSpeed(speed) { effects.push({method:"speed",speed}); }
  };
  w.HTMLDialogElement.prototype.showModal = function() { this.setAttribute("open",""); };
  w.HTMLDialogElement.prototype.close = function() {
    if (!this.hasAttribute("open")) return;
    this.removeAttribute("open");
    this.dispatchEvent(new w.Event("close"));
  };
  w.addEventListener("error", event => errors.push(event.message));
  w.Storage.prototype.getItem = function(key) { storage.push(["read",key]); return JSON.stringify({ acquired:["c03","c11"], stars:99 }); };
  w.Storage.prototype.setItem = function(key) { storage.push(["write",key]); };
  w.eval(["config", "store", "data", "ui", "progression", "battle", "codex-data", "codex"].map(name => read(`js/${name}.js`)).join("\n;\n") + `
    function requireGame() { return Store.ready(); }
    renderTopbar = () => {};
    toast = () => {};
    window.codexTest = { Store, GIRLS, RARITY, CODEX_LORE, currentCodex, renderCodex, openCodex, selectCodexHero,
      openCodexWorld, openCodexPreview, closeCodexDialogs,
      unlockCodexHero, codexOwnership, renderSquad, openGirl, nav, toggleSquad,
      get save() { return SAVE; }, setSave(value) { SAVE = value; } };
  `);
  const api = w.codexTest;
  let signedIn = true;
  Object.assign(api.Store, { sessionEpoch:1, authUserId:7, profile:{}, ready:() => signedIn,
    mutate(action, fields) {
      calls.push([action,copy(fields)]);
      return new Promise(resolve => pending.push(resolve));
    }
  });
  api.setSave({
    squad:["c01"], girls:{c01:{star:1,level:1,exp:0,shards:0}, c03:{star:0,level:0,shards:79}, c11:{star:0,level:0,shards:api.RARITY.SR.shardToUnlock}},
    mats:{}, gold:600, token:0, daily:{date:"2026-11-28",tokenGained:0}, cleared:{}, stars:{}, sigils:{},
    progression:{days:[],weeks:[],marks:0,permits:0,reserve_squad:[],dual:{},trial_wins:{}}
  });
  const d = w.document;
  const card = id => d.querySelector(`#codex-grid button[data-codex-id="${id}"]`);
  const action = name => d.querySelector(`#codex-detail [data-codex-action="${name}"]`);
  const visibleIds = () => [...d.querySelectorAll("#codex-grid button[data-codex-id]")].map(node => node.dataset.codexId).sort();
  function change(selector, value, type = "change") {
    const node = d.querySelector(selector);
    assert.ok(node, `${selector} exists`);
    node.value = value;
    node.dispatchEvent(new w.Event(type,{bubbles:true}));
  }
  return {w,d,api,calls,pending,storage,errors,effects,card,action,visibleIds,change,signOut(){signedIn=false;}};
}

test("codex contains only implemented heroes and shard-only heroes never enter the formation roster", async context => {
  const h = harness(context);
  h.api.openCodex("c03");
  assert.equal(h.d.body.dataset.page,"page-codex");
  assert.equal(h.api.GIRLS.length,28);
  assert.equal(h.visibleIds().length,28);
  assert.deepEqual(h.visibleIds(),copy(h.api.GIRLS.map(girl => girl.id).sort()));
  assert(!h.card("c029") && !h.card("c29") && !h.card("c142"));
  assert.deepEqual(Object.keys(h.api.CODEX_LORE).sort(),h.visibleIds());
  for (const [id,status,owned,shards] of [["c01","owned",true,0],["c02","missing",false,0],["c03","shards",false,79],["c11","ready",false,h.api.RARITY.SR.shardToUnlock]]) {
    const girl = h.api.GIRLS.find(entry => entry.id === id);
    assert.deepEqual(copy(h.api.codexOwnership(girl)),{owned,shards,need:h.api.RARITY[girl.rarity].shardToUnlock,status});
    assert.equal(h.card(id).dataset.status,status);
    assert.equal(h.card(id).dataset.owned,String(owned));
  }
  h.api.nav("page-squad");
  assert.deepEqual([...h.d.querySelectorAll("#owned-list .card")].map(node => node.dataset.id),["c01"]);
  for (const id of ["c02","c03","c11"]) {
    assert.equal(h.d.querySelector(`#owned-list [data-id="${id}"]`),null);
    await h.api.toggleSquad(id);
  }
  assert.deepEqual(h.calls,[]);
  h.api.openGirl("c03");
  assert.equal(h.d.body.dataset.page,"page-codex");
  assert.equal(h.api.currentCodex().id,"c03");
  assert.match(h.d.querySelector("#codex-detail").textContent,/79\s*\/\s*80/);
  assert.equal(h.action("workshop"),null);
  assert.equal(h.action("squad"),null);
  assert.deepEqual(h.storage,[],"acquisition never reads the standalone atlas's browser state");
  assert.deepEqual(h.errors,[]);
});

test("missing and insufficient shards cannot submit synthesis, including calls outside the disabled button", async context => {
  const h = harness(context);
  for (const id of ["c02","c03"]) {
    h.api.openCodex(id);
    assert.ok(!h.action("unlock") || h.action("unlock").disabled);
    await h.api.unlockCodexHero(id);
  }
  await h.api.unlockCodexHero("c142");
  assert.deepEqual(h.calls,[]);
  assert.equal(h.api.save.girls.c03.shards,79);
  assert.equal(h.api.save.girls.c03.star,0);
  assert.deepEqual(h.api.save.squad,["c01"]);
});

test("synthesis submits only intent once and waits for the server before exposing a hero to formation", async context => {
  const h = harness(context), need = h.api.RARITY.SR.shardToUnlock;
  h.api.openCodex("c11");
  assert.equal(h.action("unlock").disabled,false);
  h.action("unlock").focus();
  const before = copy(h.api.save), request = h.api.unlockCodexHero("c11");
  await h.api.unlockCodexHero("c11");
  assert.deepEqual(h.calls,[["shard_unlock",{girl_id:"c11"}]]);
  assert.deepEqual(copy(h.api.save),before);
  assert.equal(h.card("c11").dataset.status,"ready");
  assert.ok(h.action("unlock").disabled);
  assert.ok(h.d.querySelector("#codex-detail").contains(h.d.activeElement),"pending synthesis keeps keyboard focus in the dossier");
  h.api.renderSquad();
  assert.equal(h.d.querySelector('#owned-list [data-id="c11"]'),null);
  const confirmed = copy(before);
  confirmed.girls.c11 = {star:h.api.RARITY.SR.baseStar,level:1,exp:0,shards:before.girls.c11.shards-need};
  h.api.setSave(confirmed);
  h.pending.shift()({drops:["剑华已解锁"]});
  await request; await tick();
  assert.equal(h.card("c11").dataset.status,"owned");
  assert.equal(h.card("c11").dataset.owned,"true");
  assert.equal(h.action("unlock"),null);
  h.api.nav("page-squad");
  assert.ok(h.d.querySelector('#owned-list [data-id="c11"] .quick-toggle'));
  assert.deepEqual(copy(h.api.save.squad),["c01"],"unlocking does not silently deploy the new hero");
  assert.deepEqual(h.storage,[]);
  assert.deepEqual(h.errors,[]);
});

test("an unconfirmed synthesis retains shards and the unowned state and permits an explicit retry", async context => {
  const h = harness(context);
  h.api.openCodex("c11");
  const before = copy(h.api.save), request = h.api.unlockCodexHero("c11");
  h.pending.shift()(null);
  await request;
  assert.deepEqual(copy(h.api.save),before);
  assert.equal(h.card("c11").dataset.status,"ready");
  assert.equal(h.action("unlock").disabled,false);
  assert.equal(h.api.currentCodex().busy,false);
  const retry = h.api.unlockCodexHero("c11");
  assert.equal(h.calls.length,2);
  h.pending.shift()(null);
  await retry;
  h.api.nav("page-squad");
  assert.equal(h.d.querySelector('#owned-list [data-id="c11"]'),null);
});

test("a late synthesis receipt cannot change another account's selection, ownership or shards", async context => {
  for (const boundary of ["epoch","owner"]) await context.test(boundary,async context => {
    const h = harness(context);
    h.api.openCodex("c11");
    const oldState = h.api.currentCodex(), request = h.api.unlockCodexHero("c11");
    if (boundary === "epoch") h.api.Store.sessionEpoch++;
    else h.api.Store.authUserId++;
    const newSave = copy(h.api.save);
    newSave.girls.c11.shards = 2;
    h.api.setSave(newSave);
    h.api.openCodex("c03");
    const state = h.api.currentCodex();
    assert.notEqual(state,oldState);
    assert.equal(state.busy,false);
    const markup = h.d.querySelector("#codex-root").innerHTML;
    h.pending.shift()({drops:["旧账号合成成功"]});
    await request;
    assert.equal(h.api.currentCodex(),state);
    assert.equal(state.id,"c03");
    assert.equal(h.api.save.girls.c11.star,0);
    assert.equal(h.api.save.girls.c11.shards,2);
    assert.equal(h.d.querySelector("#codex-root").innerHTML,markup);
    assert.deepEqual(h.errors,[]);
  });
});

test("search and acquisition filters find real heroes while dossiers show the supplied reference skills and stories", async context => {
  const h = harness(context);
  h.api.openCodex("c01");
  h.change('#codex-search input,input#codex-search',"寻宝女王","input");
  await tick();
  assert.deepEqual(h.visibleIds(),["c03"]);
  h.change('#codex-search input,input#codex-search',"","input");
  h.change("#codex-status","ready");
  await tick();
  assert.deepEqual(h.visibleIds(),["c11"]);
  h.change("#codex-status","owned");
  assert.deepEqual(h.visibleIds(),["c01"]);
  h.change("#codex-status","all");
  h.d.querySelector('[data-codex-rarity="SSR"]').click();
  assert.deepEqual(h.visibleIds(),copy(h.api.GIRLS.filter(girl=>girl.rarity==="SSR").map(girl=>girl.id).sort()));
  h.change("#codex-role","support");
  assert.deepEqual(h.visibleIds(),copy(h.api.GIRLS.filter(girl=>girl.rarity==="SSR"&&girl.role==="support").map(girl=>girl.id).sort()));
  h.api.openCodex("c01");
  const lore = h.api.CODEX_LORE.c01;
  h.d.querySelector('[data-codex-tab="skills"]').click();
  let detail = h.d.querySelector("#codex-detail").textContent;
  assert.match(detail,/把本领，写成星光/);
  for (const skill of lore.skills) assert.ok(detail.includes(skill.name) && detail.includes(prose(skill)));
  h.d.querySelector('[data-codex-tab="story"]').click();
  detail = h.d.querySelector("#codex-detail").textContent;
  assert.ok(detail.includes(h.api.CODEX_LORE.c01.biography));
  assert.deepEqual(h.calls,[]);
  assert.deepEqual(h.storage,[]);
  assert.deepEqual(h.errors,[]);
});

test("all 28 implemented heroes retain the reference skill rules, evolution prose and star resonance", context => {
  const h = harness(context), before = copy(h.api.save);
  let skills = 0, resonances = 0;
  for (const hero of h.api.GIRLS) {
    const lore = h.api.CODEX_LORE[hero.id];
    h.api.openCodex(hero.id);
    h.d.querySelector('[data-codex-tab="skills"]').click();
    const panel = h.d.querySelector("#codex-tab-content");
    assert.match(panel.textContent,/把本领，写成星光/);
    const separateBasic = lore.basic && !lore.skills.some(skill => skill.id === lore.basic.id);
    const entries = [...(separateBasic ? [["basic",lore.basic]] : []),...lore.skills.map((skill,index) => [String(index),skill])];
    assert.equal(panel.querySelectorAll("[data-codex-preview]").length,entries.length,`${hero.id} has no duplicated basic attack`);
    for (const [index,skill] of entries) {
      const button = panel.querySelector(`[data-codex-preview="${index}"]`);
      assert.ok(button,`${hero.id} exposes a preview for ${skill.name}`);
      const card = button.closest("article");
      assert.ok(card,`${hero.id} skill uses its own card`);
      for (const field of [skill.name,skill.type,skill.target,prose(skill),prose(skill.upgrade),prose(skill.visual)].filter(Boolean)) includesProse(card,field,`${hero.id} preserves ${skill.name} reference text`);
      if (Number(skill.rage) > 0) assert.match(card.textContent,new RegExp(`${skill.rage}\\s*怒气`));
      if (Number(skill.cooldown) > 0) assert.match(card.textContent,new RegExp(`冷却\\s*${skill.cooldown}\\s*次行动`));
      if (prose(skill).length > 280) {
        const rules = card.querySelector("details");
        assert.ok(rules,`${hero.id} long rules can be expanded`);
        assert.equal(rules.open,false);
        includesProse(rules,prose(skill),`${hero.id} keeps the complete settlement rules`);
      }
      skills++;
    }
    assert.match(panel.textContent,/星阶共鸣/);
    assert.equal(panel.querySelectorAll("[data-star-milestone]").length,lore.highStars.length);
    for (const star of lore.highStars) {
      const card = panel.querySelector(`[data-star-milestone="${star.star ?? star.stars}"]`);
      assert.ok(card);
      includesProse(card,star.name);
      includesProse(card,star);
      resonances++;
    }
    assert.match(panel.textContent,/应对与边界/);
    includesProse(panel,lore.counter,`${hero.id} keeps its limitations`);
    assert.match(panel.textContent,/星霜演化.*尚未接入实战结算/s,"reference proposals remain identified as display-only design");
  }
  assert.equal(skills,56);
  assert.equal(resonances,112);
  assert.deepEqual(copy(h.api.save),before);
  assert.deepEqual(h.calls,[]);
  assert.deepEqual(h.storage,[]);
  assert.deepEqual(h.errors,[]);
});

test("all reference dossiers show six original stats, the full life and event, folded appearance and four abilities", context => {
  const h = harness(context), before = copy(h.api.save);
  let abilities = 0;
  for (const hero of h.api.GIRLS) {
    const lore = h.api.CODEX_LORE[hero.id];
    h.api.openCodex(hero.id);
    h.d.querySelector('[data-codex-tab="story"]').click();
    const panel = h.d.querySelector("#codex-tab-content");
    for (const heading of ["此身有来处","生平","那一次选择","四项本领"]) assert.ok([...panel.querySelectorAll("h3")].some(node => node.textContent === heading),`${hero.id} has ${heading}`);
    for (const [key,label] of [["hp","生命"],["speed","速度"],["atk","攻击"],["def","防御"],["spAtk","特攻"],["spDef","特防"]]) {
      const node = [...panel.querySelectorAll("label")].find(node => node.textContent === label);
      assert.ok(node,`${hero.id} has the ${label} stat`);
      assert.equal(node.parentElement.querySelector("strong").textContent,String(lore.stats[key] ?? "—"),`${hero.id} preserves the reference ${label}, including absent special stats`);
    }
    for (const field of [lore.statNote,lore.biography,lore.event,lore.origin]) includesProse(panel,field,`${hero.id} preserves the reference dossier`);
    const appearance = [...panel.querySelectorAll("details")].find(node => node.querySelector("summary")?.textContent === "形貌与随身物件");
    assert.ok(appearance,`${hero.id} appearance is independently expandable`);
    assert.equal(appearance.open,false);
    includesProse(appearance,lore.appearance);
    assert.equal(lore.abilities.length,4);
    for (const ability of lore.abilities) {
      const heading = [...panel.querySelectorAll("h4")].find(node => node.textContent === ability.name);
      assert.ok(heading,`${hero.id} retains ${ability.name}`);
      includesProse(heading.closest("article"),ability.description || ability.text);
      abilities++;
    }
  }
  assert.equal(abilities,112);
  assert.deepEqual(copy(h.api.save),before);
  assert.deepEqual(h.calls,[]);
  assert.deepEqual(h.storage,[]);
  assert.deepEqual(h.errors,[]);
});

test("the world chronicle opens from the atlas and returns focus without adding heroes or changing acquisition", context => {
  const h = harness(context);
  h.api.openCodex("c03");
  h.d.querySelector('[data-codex-tab="sources"]').click();
  const opener = h.d.querySelector('#codex-tab-content [data-codex-action="world"]');
  assert.ok(opener);
  assert.match(opener.textContent,/读一页百骸洲纪事/);
  const before = copy(h.api.save), ids = h.visibleIds();
  opener.focus(); opener.click();
  const dialog = h.d.querySelector("#codex-world-dialog");
  assert.ok(dialog?.open);
  assert.ok(dialog.contains(h.d.activeElement),"opening the chronicle moves keyboard focus inside");
  for (const passage of ["葬星之野","百骸洲纪事","坠后 317 年","一条街上天以前","炉京盖在活兽赤母的背上。","钉址","驮址","潮口","群星的来处","初见群星","旧火与潮路","无主之地","远岸来信","同途异人","认出一份活儿，不等于认出了做活的人。"])
    includesProse(dialog,passage);
  dialog.querySelector("[data-codex-close]").click();
  assert.ok(!h.d.querySelector("#codex-world-dialog[open]"));
  assert.equal(h.d.activeElement,opener);
  assert.deepEqual(h.visibleIds(),ids,"later chronicle chapters never add unimplemented cards to the roster");
  assert.equal(h.card("c03").dataset.status,"shards");
  assert.deepEqual(copy(h.api.save),before);
  assert.deepEqual(h.calls,[]);
  assert.deepEqual(h.storage,[]);
  assert.deepEqual(h.errors,[]);
});

test("skill previews use reference skills and support pause, speed, replay and close without battle mutations", context => {
  const h = harness(context);
  h.api.openCodex("c03");
  h.d.querySelector('[data-codex-tab="skills"]').click();
  const opener = h.d.querySelector('[data-codex-preview="1"]'), before = copy(h.api.save);
  opener.focus(); opener.click();
  const dialog = h.d.querySelector("#codex-preview-dialog"), lore = h.api.CODEX_LORE.c03, skill = lore.skills[1];
  assert.ok(dialog?.open);
  assert.ok(dialog.contains(h.d.activeElement));
  includesProse(dialog,skill.name);
  includesProse(dialog,skill.visual);
  let played = h.effects.filter(call => call.method === "play");
  assert.equal(played.length,1);
  assert.equal(played[0].canvas,h.d.querySelector("#codex-effect-canvas"));
  assert.equal(played[0].options.caption,h.d.querySelector("#codex-effect-caption"));
  assert.equal(played[0].options.card.id,"c03");
  assert.equal(played[0].options.card.name,lore.name);
  assert.equal(played[0].options.skill,skill);
  assert.equal(played[0].options.speed,1);
  const pause = dialog.querySelector('[data-codex-preview-control="pause"]');
  pause.click();
  assert.equal(pause.getAttribute("aria-pressed"),"true");
  assert.match(pause.textContent,/继续/);
  pause.click();
  assert.equal(pause.getAttribute("aria-pressed"),"false");
  assert.match(pause.textContent,/暂停/);
  h.change('[data-codex-preview-speed]',"1.5");
  assert.deepEqual(h.effects.filter(call => call.method === "speed").map(call => call.speed),[1.5]);
  dialog.querySelector('[data-codex-preview-control="replay"]').click();
  played = h.effects.filter(call => call.method === "play");
  assert.equal(played.length,2);
  assert.equal(played[1].options.speed,1.5);
  assert.equal(played[1].options.skill,skill);
  played[1].options.onEnd();
  assert.equal(pause.disabled,true);
  assert.match(pause.textContent,/已播放/);
  dialog.querySelector('[data-codex-preview-control="replay"]').click();
  assert.equal(pause.disabled,false);
  const stopped = h.effects.filter(call => call.method === "stop").length;
  dialog.querySelector("[data-codex-close]").click();
  assert.ok(!h.d.querySelector("#codex-preview-dialog[open]"));
  assert.ok(h.effects.filter(call => call.method === "stop").length > stopped);
  assert.equal(h.d.activeElement,opener);
  assert.equal(h.card("c03").dataset.status,"shards");
  assert.deepEqual(copy(h.api.save),before);
  assert.deepEqual(h.calls,[]);
  assert.deepEqual(h.storage,[]);
  assert.deepEqual(h.errors,[]);
});

test("preview cancellation and navigation stop animation without focusing a stale account or page", async context => {
  for (const boundary of ["cancel","page","epoch","owner"]) await context.test(boundary,context => {
    const h = harness(context);
    h.api.openCodex("c01");
    const opener = h.d.querySelector('[data-codex-preview="0"]');
    opener.focus(); opener.click();
    const dialog = h.d.querySelector("#codex-preview-dialog"), stopped = h.effects.filter(call => call.method === "stop").length;
    if (boundary === "page") h.api.nav("page-squad");
    else if (boundary === "cancel") {
      const event = new h.w.Event("cancel",{cancelable:true});
      dialog.dispatchEvent(event);
      if (!event.defaultPrevented && dialog.open) dialog.close();
    } else {
      if (boundary === "epoch") h.api.Store.sessionEpoch++;
      else h.api.Store.authUserId++;
      dialog.querySelector("[data-codex-close]").click();
    }
    assert.ok(!h.d.querySelector("#codex-preview-dialog[open]"));
    assert.ok(h.effects.filter(call => call.method === "stop").length > stopped);
    if (boundary === "cancel") assert.equal(h.d.activeElement,opener);
    else assert.notEqual(h.d.activeElement,opener);
    assert.deepEqual(h.calls,[]);
    assert.deepEqual(h.storage,[]);
    assert.deepEqual(h.errors,[]);
  });
});

test("switching to the world chronicle closes a running preview and keeps only one atlas dialog open", context => {
  const h = harness(context);
  h.api.openCodex("c01");
  h.d.querySelector('[data-codex-preview="0"]').click();
  assert.ok(h.d.querySelector("#codex-preview-dialog[open]"));
  const stopped = h.effects.filter(call => call.method === "stop").length;
  h.api.openCodexWorld();
  assert.ok(!h.d.querySelector("#codex-preview-dialog[open]"));
  assert.ok(h.d.querySelector("#codex-world-dialog[open]"));
  assert.ok(h.effects.filter(call => call.method === "stop").length > stopped);
  assert.equal(h.d.querySelectorAll("dialog[open]").length,1);
  h.api.closeCodexDialogs(false);
  assert.equal(h.d.querySelectorAll("dialog[open]").length,0);
  assert.deepEqual(h.calls,[]);
  assert.deepEqual(h.storage,[]);
  assert.deepEqual(h.errors,[]);
});

test("same-account synchronization refreshes shards while preserving browsing and exposes failures in the roster", async context => {
  const h = harness(context);
  h.api.openCodex("c03");
  h.change("input#codex-search","女王","input");
  h.change("#codex-status","shards");
  h.d.querySelector('[data-codex-tab="story"]').click();
  const state = h.api.currentCodex(), detail = h.d.querySelector("#codex-detail");
  detail.scrollTop = 137;
  let finish, reads = 0;
  h.api.Store.refresh = () => {
    reads++;
    return new Promise(resolve => { finish = resolve; });
  };
  h.d.querySelector("#codex-sync").click();
  h.d.querySelector("#codex-sync").click();
  assert.equal(reads,1,"synchronization cannot overlap");
  assert.equal(state.syncing,true);
  assert.equal(h.api.save.girls.c03.shards,79);
  assert.equal(h.d.querySelector("#codex-status-message").hidden,false);
  assert.match(h.d.querySelector("#codex-status-message").textContent,/正在同步/);
  const confirmed = copy(h.api.save);
  confirmed.girls.c03.shards = 80;
  h.api.setSave(confirmed);
  finish({save:copy(confirmed)});
  await tick();
  assert.equal(h.api.currentCodex(),state);
  assert.equal(state.syncing,false);
  assert.equal(state.query,"女王");
  assert.equal(state.status,"shards");
  assert.equal(state.id,"c03");
  assert.equal(state.tab,"story");
  assert.equal(detail.scrollTop,137);
  assert.equal(h.d.querySelector("input#codex-search").value,"女王");
  assert.equal(h.d.querySelector("#codex-status").value,"shards");
  assert.equal(h.card("c03").dataset.status,"ready");
  assert.match(detail.textContent,/80\s*\/\s*80/);
  assert.equal(h.d.querySelector("#codex-status-message").hidden,true);
  h.action("roster").click();
  assert.equal(state.mobileDetail,false);
  h.api.Store.refresh = async () => { reads++; return null; };
  h.d.querySelector("#codex-sync").click();
  await tick();
  const message = h.d.querySelector("#codex-status-message");
  assert.equal(message.hidden,false);
  assert.equal(message.getAttribute("role"),"status");
  assert.match(message.textContent,/无法同步|同步失败/);
  assert.ok(h.d.querySelector(".cx-roster").contains(message),"sync errors remain visible when mobile details are closed");
  assert.equal(state.mobileDetail,false);
  assert.equal(h.api.save.girls.c03.shards,80);
  assert.equal(detail.scrollTop,137);
  assert.equal(h.d.querySelector("#codex-sync").disabled,false);
  assert.deepEqual(h.calls,[]);
  assert.deepEqual(h.storage,[]);
  assert.deepEqual(h.errors,[]);
});

test("previous and next enter filtered results at the correct boundary when the current hero is outside the filter", context => {
  const h = harness(context);
  h.api.openCodex("c01");
  h.d.querySelector('[data-codex-rarity="SSR"]').click();
  const matches = [...h.d.querySelectorAll("#codex-grid button[data-codex-id]")].map(node=>node.dataset.codexId);
  assert.ok(matches.length > 2);
  assert.ok(!matches.includes("c01"));
  assert.equal(h.api.currentCodex().id,"c01");
  h.action("previous").click();
  assert.equal(h.api.currentCodex().id,matches.at(-1));
  h.api.selectCodexHero("c01");
  assert.equal(h.api.currentCodex().rarity,"SSR");
  h.action("next").click();
  assert.equal(h.api.currentCodex().id,matches[0]);
  h.action("previous").click();
  assert.equal(h.api.currentCodex().id,matches.at(-1),"previous wraps from the first filtered hero to the last");
  h.action("next").click();
  assert.equal(h.api.currentCodex().id,matches[0],"next wraps from the last filtered hero to the first");
  assert.deepEqual(h.calls,[]);
  assert.deepEqual(h.errors,[]);
});

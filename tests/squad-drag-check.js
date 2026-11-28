const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const { JSDOM } = require("jsdom");

const root = path.resolve(__dirname, "..");
const read = file => fs.readFileSync(path.join(root, file), "utf8");
const clone = value => JSON.parse(JSON.stringify(value));
const ids = ["c08", "c06", "c01", "c12", "c02", "c20", "c21"];
const flush = async () => { for (let i = 0; i < 12; i++) await Promise.resolve(); };

function harness(context, count = 6) {
  const dom = new JSDOM(read("index.html"), {runScripts:"outside-only", pretendToBeVisual:true});
  context.after(() => dom.window.close());
  const w = dom.window, calls = [], notices = [], errors = [];
  w.document.body.dataset.page = "page-squad";
  w.addEventListener("error", event => errors.push(event.error || event.message));
  w.Store = {sessionEpoch:1, ready:() => true, profile:{}, mutate(action, fields) {
    return new Promise(resolve => calls.push({action, fields:clone(fields), resolve}));
  }};
  w.__notices = notices;
  w.requireGame = () => w.Store.ready();
  w.eval(["js/data.js", "js/ui.js", "js/progression.js"].map(read).join("\n;\n") + `
    toast = message => __notices.push(message); renderTopbar = () => {};
    window.api = {renderSquad, selectSquadSlot, toggleSquad, unslot,
      get save() { return SAVE; }, setSave(value) { SAVE = value; },
      get selected() { return squadSelectedId; }};
  `);
  const api = w.api;
  api.setSave({squad:ids.slice(0,count), girls:Object.fromEntries(ids.map(id => [id,{star:1,level:1,shards:0,exp:0}])), gold:100, mats:{}});
  api.renderSquad();
  const slot = index => w.document.querySelector(`#squad-slots [data-squad-slot="${index}"]`);
  const pick = index => slot(index).querySelector(".squad-pick");
  function native(target, type, data = {}) {
    const event = new w.Event(type, {bubbles:true,cancelable:true});
    Object.assign(event, {clientX:10,clientY:10,dataTransfer:{setData(){},...data}});
    target.dispatchEvent(event);
    return event;
  }
  function pointer(target, type, x, y, extra = {}) {
    const event = new w.MouseEvent(type,{bubbles:true,cancelable:true,clientX:x,clientY:y,button:0});
    Object.defineProperties(event, {pointerType:{value:extra.pointerType || "touch"}, pointerId:{value:extra.pointerId || 7}, isPrimary:{value:extra.isPrimary !== false}});
    target.dispatchEvent(event);
    return event;
  }
  function accept(call, squad = call.fields.squad) {
    api.setSave({...api.save,squad:clone(squad)});
    call.resolve({save:clone(api.save)});
  }
  return {w,api,calls,notices,errors,slot,pick,native,pointer,accept};
}

test("desktop dragging swaps exact slots, deduplicates requests and waits for the authoritative response", async context => {
  const h = harness(context), original = clone(h.api.save.squad);
  assert.equal(h.pick(0).draggable, true);
  assert.equal(h.pick(0).querySelector("img").draggable, false);
  h.native(h.pick(0), "dragstart");
  assert.ok(h.slot(0).classList.contains("slot-dragging"));
  const over = h.native(h.pick(5).querySelector(".tn-name"), "dragover");
  assert.equal(over.defaultPrevented, true);
  assert.ok(h.slot(5).classList.contains("slot-drop-target"));
  h.native(h.pick(5), "drop");
  assert.deepEqual(h.calls.map(call => ({action:call.action,...call.fields})), [{action:"squad",squad:[ids[5],...ids.slice(1,5),ids[0]]}]);
  assert.deepEqual(clone(h.api.save.squad), original);
  assert.equal(h.w.document.getElementById("squad-slots").getAttribute("aria-busy"), "true");
  assert.equal(h.native(h.pick(1), "dragstart").defaultPrevented, true);
  await h.api.toggleSquad(ids[6]);
  assert.equal(h.calls.length, 1);
  const serverSquad = [ids[5],...ids.slice(1,5),ids[0]];
  h.accept(h.calls[0], serverSquad); await flush();
  assert.deepEqual(clone(h.api.save.squad), serverSquad);
  assert.equal(h.pick(0).dataset.squadId, ids[5]);
  assert.equal(h.pick(0).disabled, false);
  assert.equal(h.w.document.querySelectorAll(".slot-dragging,.slot-drop-target").length, 0);
  assert.deepEqual(h.errors, []);
});

test("empty slots move a member to the compact roster tail without duplicate IDs or untrusted drag payloads", async context => {
  const h = harness(context, 3);
  h.native(h.pick(0), "dragstart");
  h.native(h.slot(5), "drop", {getData:() => "c21"});
  assert.deepEqual(h.calls[0].fields, {squad:[ids[1],ids[2],ids[0]]});
  assert.equal(new Set(h.calls[0].fields.squad).size, 3);
  h.accept(h.calls[0]); await flush();
  h.native(h.pick(2), "dragstart");
  h.native(h.slot(5), "drop");
  assert.equal(h.calls.length, 1, "already-last card does not send a no-op request");
  h.native(h.slot(0), "drop", {getData:() => "c21"});
  assert.equal(h.calls.length, 1, "an external payload cannot add or replace cards");
});

test("desktop cancellation, changed roster and a new session cannot apply stale drops", async context => {
  const h = harness(context);
  h.native(h.pick(0), "dragstart"); h.native(h.pick(0), "drop");
  assert.equal(h.calls.length, 0);
  h.native(h.pick(0), "dragstart"); h.native(h.pick(0), "dragend"); h.native(h.pick(1), "drop");
  assert.equal(h.calls.length, 0);
  h.native(h.pick(0), "dragstart");
  h.api.setSave({...h.api.save,squad:[...ids.slice(1,6),ids[0]]});
  h.native(h.pick(1), "drop");
  assert.equal(h.calls.length, 0);
  h.api.renderSquad();
  h.native(h.pick(0), "dragstart"); h.w.Store.sessionEpoch++;
  h.native(h.pick(1), "drop");
  assert.equal(h.calls.length, 0);
  assert.equal(h.w.document.querySelectorAll(".slot-dragging,.slot-drop-target").length, 0);
});

test("touch dragging shows a following card and target, submits once, and suppresses the synthetic click", async context => {
  const h = harness(context), source = h.pick(0);
  h.slot(0).getBoundingClientRect = () => ({left:0,top:0,width:100,height:130});
  h.w.document.elementFromPoint = () => h.pick(4).querySelector(".tn-name");
  h.pointer(source, "pointerdown", 20, 20);
  h.pointer(h.w.document, "pointermove", 24, 23);
  assert.equal(h.w.document.querySelector(".squad-drag-ghost"), null, "tiny movement remains a tap");
  const moved = h.pointer(h.w.document, "pointermove", 240, 240);
  assert.equal(moved.defaultPrevented, true);
  const ghost = h.w.document.querySelector(".squad-drag-ghost");
  assert.ok(ghost);
  assert.equal(ghost.getAttribute("aria-hidden"), "true");
  assert.ok(h.slot(4).classList.contains("slot-drop-target"));
  h.pointer(h.w.document, "pointerup", 240, 240);
  assert.equal(h.w.document.querySelector(".squad-drag-ghost"), null);
  assert.deepEqual(h.calls[0].fields, {squad:[ids[4],...ids.slice(1,4),ids[0],ids[5]]});
  const click = new h.w.MouseEvent("click", {bubbles:true,cancelable:true});
  source.dispatchEvent(click);
  assert.equal(click.defaultPrevented, true);
  assert.equal(h.api.selected, null);
  h.pointer(h.w.document, "pointerup", 240, 240);
  assert.equal(h.calls.length, 1);
  h.calls[0].resolve(null); await flush();
  assert.deepEqual(clone(h.api.save.squad), ids.slice(0,6));
  assert.equal(h.pick(0).disabled, false);
  assert.deepEqual(h.errors, []);
});

test("touch tap preserves click selection while cancel, outside drop and secondary fingers never write", async context => {
  const h = harness(context);
  h.w.document.elementFromPoint = () => null;
  h.pointer(h.pick(0), "pointerdown", 10, 10);
  h.pointer(h.w.document, "pointerup", 10, 10);
  await h.api.selectSquadSlot(0);
  assert.equal(h.api.selected, ids[0]);
  assert.equal(h.calls.length, 0);
  h.pointer(h.pick(1), "pointerdown", 10, 10, {isPrimary:false});
  h.pointer(h.w.document, "pointermove", 100, 100);
  assert.equal(h.w.document.querySelector(".squad-drag-ghost"), null);
  h.pointer(h.pick(1), "pointerdown", 10, 10);
  h.pointer(h.w.document, "pointermove", 100, 100);
  h.pointer(h.w.document, "pointercancel", 100, 100);
  assert.equal(h.w.document.querySelector(".squad-drag-ghost"), null);
  h.pointer(h.pick(1), "pointerdown", 10, 10, {pointerType:"pen"});
  h.pointer(h.w.document, "pointermove", 100, 100, {pointerType:"pen"});
  h.pointer(h.w.document, "pointerup", 100, 100, {pointerType:"pen"});
  assert.equal(h.calls.length, 0);
  assert.equal(h.w.document.querySelector(".squad-drag-ghost"), null);
});

test("failed swaps can retry and late server responses cannot redraw another account's selection", async context => {
  const h = harness(context);
  h.native(h.pick(0), "dragstart"); h.native(h.pick(1), "drop");
  h.calls[0].resolve(null); await flush();
  assert.equal(h.pick(0).dataset.squadId, ids[0]);
  h.native(h.pick(0), "dragstart"); h.native(h.pick(1), "drop");
  assert.equal(h.calls.length, 2);
  h.w.Store.sessionEpoch++;
  h.api.setSave({...h.api.save,squad:ids.slice(2,5)});
  h.api.renderSquad();
  await h.api.selectSquadSlot(1);
  assert.equal(h.api.selected, ids[3]);
  h.calls[1].resolve({save:{squad:[ids[1],ids[0]]}}); await flush();
  assert.equal(h.api.selected, ids[3]);
  assert.deepEqual(clone(h.api.save.squad), ids.slice(2,5));
  assert.equal(h.pick(0).dataset.squadId, ids[2]);
});

test("clicking a selected member and then an empty slot uses the same server-owned move", async context => {
  const h = harness(context,3);
  await h.api.selectSquadSlot(0);
  const pending = h.api.selectSquadSlot(5);
  assert.deepEqual(h.calls[0].fields,{squad:[ids[1],ids[2],ids[0]]});
  assert.deepEqual(clone(h.api.save.squad),ids.slice(0,3));
  h.accept(h.calls[0]); await pending;
  assert.equal(h.api.selected,null);
});

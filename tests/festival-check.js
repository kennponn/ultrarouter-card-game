const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const { JSDOM } = require("jsdom");
const root = path.join(__dirname, "..");
const events = JSON.parse(fs.readFileSync(path.join(root,"backend/model/gacha_festival.json")));

function festivalHarness() {
  const dom = new JSDOM('<body data-page="page-festival"><div id="festival-body"></div></body>', {runScripts:"outside-only"});
  const window = dom.window;
  window.Store = {profile:{festival:{events,week:"2026-W48",album:{3:{forge_ore:10}}}},sessionEpoch:3,ready:() => true,serverNow:() => Date.parse("2026-11-23T15:59:59Z")};
  window.SAVE = {daily:{date:"2026-11-23"},festival:{week:"2026-W48",day:0,stamps:{},tries:{},claimed:{}}};
  window.esc = value => String(value).replaceAll("<", "&lt;");
  window.renderTopbar = () => {};
  window.requireGame = () => window.Store.ready();
  window.nav = page => { window.document.body.dataset.page = page; window.renderFestival(); };
  window.eval(fs.readFileSync(path.join(root,"js/reward-chests.js"), "utf8"));
  window.eval(fs.readFileSync(path.join(root,"js/festival.js"), "utf8"));
  window.renderFestival();
  return dom;
}

test("daily activity shortcut restores today after previewing another day without submitting a proposal", () => {
  const dom = festivalHarness(), {window} = dom, doc = window.document, calls = [];
  try {
    window.Store.mutate = (...args) => { calls.push(args); };
    doc.querySelector('[data-day="6"]').click();
    assert.equal(doc.querySelectorAll("form[data-fest-id]").length,0);
    window.openFestivalToday();
    assert.equal(doc.querySelector('[data-day="0"]').getAttribute("aria-pressed"),"true");
    assert.equal(doc.querySelectorAll("form[data-fest-id]").length,2);
    assert.ok(doc.activeElement.matches("form[data-fest-id] input, form[data-fest-id] select"));
    assert.deepEqual(calls,[]);
  } finally { window.close(); }
});

test("seven ports preview all fourteen fixtures but only today's events accept a proposal", () => {
  const dom = festivalHarness(), doc = dom.window.document;
  try {
    assert.equal(doc.querySelectorAll("[data-day]").length, 7);
    assert.equal(doc.querySelectorAll("form[data-fest-id]").length, 2);
    const names = [...doc.querySelectorAll(".fest-event h3")].map(node => node.textContent);
    for (let index = 1; index < 7; index++) {
      doc.querySelector(`[data-day="${index}"]`).click();
      assert.equal(doc.querySelectorAll(".fest-event form").length, 0);
      names.push(...[...doc.querySelectorAll(".fest-event h3")].map(node => node.textContent));
    }
    assert.equal(new Set(names).size, 14); assert.equal(doc.querySelector("[data-album]").disabled, true);
  } finally { dom.window.close(); }
});

test("port forms submit the dated proposal only and display the server award", async () => {
  const dom = festivalHarness(), window = dom.window, doc = window.document;
  try {
    let sent;
    window.Store.mutate = async (action, fields) => {
      sent = JSON.parse(JSON.stringify({action,...fields}));
      window.SAVE.festival.stamps.relief = true;
      return {save:window.SAVE,drops:["斥候札记 ×2"]};
    };
    const form = doc.querySelector('[data-fest-id="relief"]');
    assert.ok(form.querySelector("button").disabled);
    form.querySelector('input[value="0"]').click(); form.querySelector('input[value="1"]').click();
    assert.equal(form.querySelector("button").disabled, false);
    assert.match(form.querySelector("output").textContent, /重 7 · 值 15/);
    form.requestSubmit(); await new Promise(resolve => setImmediate(resolve));
    assert.deepEqual(sent,{action:"festival_play",item_id:"2026-11-23:relief",count:1,choice:"01"});
    assert.match(doc.querySelector(".fest-notice").textContent,/斥候札记/);
    assert.equal(doc.querySelector('[data-fest-id="relief"]'), null);
    window.Store.profile.festival.week = "2026-W49"; window.renderFestival();
    assert.equal(doc.querySelector("form"),null); assert.match(doc.querySelector("#festival-body").textContent,/同步/);
  } finally { dom.window.close(); }
});

test("permanent desks open a supported workshop for the current party without a write", () => {
  const dom = festivalHarness(), window = dom.window, doc = window.document;
  try {
    const permanent = doc.createElement("div"); permanent.id = "festival-permanent"; doc.body.append(permanent);
    window.workshopRules = () => ({}); window.workshopTabKeys = () => ["cases", "dispatch"];
    window.SAVE.squad = ["c01"]; window.SAVE.girls = {c01:{star:2}};
    let opened; window.openWorkshop = (...args) => { opened = args; };
    window.renderFestival();
    assert.equal(permanent.querySelectorAll("button").length, 2);
    permanent.querySelector('[data-desk="dispatch"]').click();
    assert.deepEqual(opened,["c01","dispatch"]); assert.equal(permanent.querySelector('[data-desk="vigil"]'),null);
  } finally { dom.window.close(); }
});

test("an open proposal crossing Beijing midnight offers the new rotation without spending an attempt", async () => {
  const dom = festivalHarness(), window = dom.window, doc = window.document;
  try {
    const form = doc.querySelector('form[data-fest-id="relief"]');
    let writes = 0;
    window.Store.mutate = async () => { writes++; };
    window.Store.serverNow = () => Date.parse("2026-11-23T16:00:00Z");
    form.querySelector('input[value="0"]').click(); form.querySelector('input[value="1"]').click();
    form.requestSubmit();
    assert.equal(writes,0); assert.equal(doc.querySelector("form"),null); assert.match(doc.querySelector("[role=status]").textContent,/待同步/);
    window.Store.refresh = async () => { window.SAVE.daily.date = "2026-11-24"; window.SAVE.festival.day = 1; };
    await window.refreshFestival();
    assert.equal(doc.querySelector("[data-day='1']").getAttribute("aria-pressed"),"true");
    assert.equal(doc.querySelectorAll("form[data-fest-id]").length,2); assert.equal(writes,0);
  } finally { dom.window.close(); }
});

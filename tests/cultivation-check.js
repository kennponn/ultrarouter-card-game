const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const { JSDOM } = require("jsdom");

const root = path.join(__dirname, "..");
const read = file => fs.readFileSync(path.join(root, file), "utf8");
const rules = JSON.parse(read("backend/model/gacha_cultivation.json"));
const catalog = JSON.parse(read("backend/model/gacha_catalog.json"));
const equipment = JSON.parse(read("backend/model/gacha_equipment.json"));
const atlas = JSON.parse(read("backend/model/gacha_items.json"));
const carry = JSON.parse(read("backend/model/gacha_carry.json"));
const training = JSON.parse(read("backend/model/gacha_training.json"));
const plain = value => JSON.parse(JSON.stringify(value));
const owned = (star, level = 60) => ({ star, level, exp: 0, shards: 0 });

function workshop(context) {
  const dom = new JSDOM(read("index.html"), { runScripts: "outside-only" });
  context.after(() => dom.window.close());
  const state = { squad: ["c01"], girls: { c01: owned(5) }, mats: {}, sigils: {}, daily: {}, cleared: {}, stars: {} };
  const scripts = ["config", "store", "data", "ui", "progression", "atelier", "training", "carry", "workshop", "battle"].map(name => read(`js/${name}.js`));
  dom.window.eval(scripts.join("\n;\n") + `
    function requireGame() { return Store.ready(); }
    window.workshop = { Store, finalStats, artifactBonuses, individualBonuses, growthStarMax, growthStarCost, growthStarStep, shardReserve,
      openGirl, starUp, cultivateGirl, collectionBond, collectionBonds, codexBonuses, starMilestones, renderSquad, renderBag, unlockShardGirl, GIRLS,
      renderGrowth, selectGrowthGirl, openGrowthGear, renderStage, pickChapter, pickStage, boundGearBonuses, atelierIcon, atelierGearQuote, selectAtelierGear, actAtelierGear,
      recyclableShards, confirmShardDecompose, decomposeShards, exchangeLegacyDust, battleDropLedger,
      openWorkshop, selectWorkshopTab, selectWorkshopSlot, selectWorkshopItem, workshopSlotAction, selectPotentialMode, workshopIVAction, workshopSquadAction,
      renderGacha, selectDrawPool, drawArtifacts, convertArtifactTicket, selectWorkshopBag, openArtifactDraw, returnFromArtifactDraw,
      carryQuote, selectCarryObject, carryWorkshopAction, openCarryFromBag,
      setState(state) { SAVE = state; } };
  `);
  const view = dom.window.workshop;
  view.setState(state);
  view.Store.ready = () => true;
  view.Store.profile = { cultivation: rules, equipment, items: atlas, carry, training };
  return { state, view, document: dom.window.document };
}

test("catalog covers all 28 cards, retains v5 bonds and supports seven payable advanced stars", () => {
  const ids = catalog.GIRLS.map(girl => girl.id).sort();
  assert.equal(ids.length, 28);
  assert.deepEqual(Object.keys(rules.weapons).sort(), ids);
  assert.deepEqual(rules.bonds.flatMap(bond => bond.members).sort(), ids);
  assert.equal(new Set(Object.values(rules.weapons).map(weapon => weapon.name)).size, ids.length);
  for (const bond of rules.bonds) {
    assert.equal(bond.members.length, 2);
    assert.ok(bond.name && Object.hasOwn(rules.codex_flat, bond.stat));
  }
  const limits = { R: 2, SR: 4, SSR: 6 }, edges = new Set();
  for (const bond of rules.fellowships) {
    assert.equal(bond.members.length, 2);
    const [first, second] = [...bond.members].sort();
    assert.ok(first !== second && ids.includes(first) && ids.includes(second));
    assert.ok(!edges.has(`${first}:${second}`));
    edges.add(`${first}:${second}`);
    assert.ok(bond.name && Object.hasOwn(rules.codex_flat, bond.stat));
    assert.ok((bond.advanced || rules.bond_advanced) > (bond.base || rules.bond_base));
  }
  assert.equal(edges.size, 56);
  for (const { id, rarity } of catalog.GIRLS) assert.equal(rules.fellowships.filter(pair => pair.members.includes(id)).length, limits[rarity]);
  assert.deepEqual([2,3].map(size => rules.crews.filter(bond => bond.members.length === size).length), [2,36]);
  assert.equal(new Set(rules.crews.map(bond => [...bond.members].sort().join(":"))).size, rules.crews.length);
  for (const {members} of rules.crews) assert.ok(members.length === new Set(members).size && members.every(id => ids.includes(id)));
  for (const { id, rarity } of catalog.GIRLS) assert.equal(rules.crews.filter(bond => bond.members.includes(id)).length, limits[rarity]);
  for (const role of new Set(catalog.GIRLS.map(girl => girl.role))) {
    assert.deepEqual(rules.milestones[role].map(step => step.star), [3, 6, 9, 12]);
    for (const step of rules.milestones[role]) assert.ok(step.name && Number.isSafeInteger(step.value) && step.value > 0);
  }
  const count = catalog.GROWTH.starMax - 5;
  assert.equal(count, 7);
  for (const list of [...Object.values(rules.advanced_shards), rules.advanced_awakening, rules.advanced_mat_multiplier]) {
    assert.equal(list.length, count);
    assert.ok(list.every((value, index) => Number.isSafeInteger(value) && value > 0 && (!index || value >= list[index - 1])));
  }
  assert.deepEqual(rules.advanced_shards.SSR, Array.from({ length: count }, (_, index) => catalog.GROWTH.starCost[index + 6]));
  assert.equal(rules.potential_stars.at(-1), 12);
  assert.equal(rules.potential_mats.length, rules.potential_stars.length);
  assert.equal(rules.weapon_cores.length, rules.weapon_awakening_gates.length);
  assert.equal(rules.weapon_cores.length, rules.weapon_level_gates.length);
  assert.deepEqual(rules.individual.stats.map(({ stat }) => stat), ["hp", "atk", "def", "spd", "cr", "cd"]);
  assert.equal(rules.individual.max_value, 31);
  assert.ok(rules.individual.stats.every(({ step }) => Number.isSafeInteger(step) && step > 0));
  const artifacts = rules.artifacts;
  assert.equal(artifacts.gem_sockets.length, 9);
  assert.equal(artifacts.chip_stars.length, 6);
  assert.equal(rules.potential_stars.length, 6);
  for (const [sets, thresholds] of [[artifacts.gem_sets, [3, 6, 9]], [artifacts.chip_sets, [2, 4, 6]]]) {
    for (const set of Object.values(sets)) {
      assert.deepEqual(set.tiers.map(tier => tier.count), thresholds);
      for (const effect of [...set.tiers, ...(set.base ? [set.base] : [])]) {
        assert.ok(["hp", "atk", "def", "spd", "cr", "cd"].includes(effect.stat));
        assert.ok(Number.isInteger(effect.value) && effect.value > 0);
      }
    }
  }
});

test("collection and partner bonuses use ownership, not the four deployed slots", context => {
  const { state, view } = workshop(context);
  const mine = state.girls.c01, girl = catalog.GIRLS.find(entry => entry.id === "c01");
  mine.star = 6;
  state.squad = [];
  const attack = girl.base.atk * (1 + 59 * catalog.GROWTH.levelStatGrow.atk) * (1 + 5 * catalog.GROWTH.starStatGrow.atk);
  state.girls.c11 = { ...owned(0, 0), shards: 250 };
  assert.deepEqual(plain(view.codexBonuses()), { hp: 0, atk: 3, def: 0 });
  assert.equal(view.collectionBond("c01").percent, 0);
  assert.equal(view.finalStats("c01", true).atk, Math.round(attack) + 3);
  state.girls.c11 = owned(2, 1);
  assert.equal(view.collectionBond("c01").percent, 0, "two members do not activate a three-card crew");
  assert.equal(view.finalStats("c01").atk, Math.round(attack) + 6);
  state.girls.c04 = owned(5, 1);
  assert.equal(view.collectionBond("c01").percent, 4);
  assert.equal(view.finalStats("c01").atk, Math.round(attack * 1.04) + 9);
  state.girls.c11.star = 6;
  assert.equal(view.collectionBond("c01").percent, 4, "the least advanced member still gates the crew");
  state.girls.c04.star = 6;
  state.girls.c08 = owned(1, 1);
  Object.assign(mine, { weapon_level: 10, weapon_awake: 2, potential: 3, weapon_rune: "edge" });
  const expected = Math.round(attack * 1.2 * 1.07 * 1.03 * 1.05) + 9;
  assert.equal(view.finalStats("c01", true).atk, expected);
  assert.equal(view.collectionBond("c11").percent, 7);
  for (const deployed of [[], ["c01"], ["c08"]]) {
    state.squad = deployed;
    assert.equal(view.finalStats("c01", true).atk, expected);
    assert.deepEqual(plain(view.codexBonuses()), { hp: 18, atk: 9, def: 0 });
  }
});

test("fellowships add before the per-stat cap; all four role milestones describe live effects", context => {
  const { view, state, document } = workshop(context);
  const hero = catalog.GIRLS.find(girl => girl.id === "c04");
  state.squad = [];
  state.girls = { c04: owned(12) };
  for (const bond of view.collectionBonds(hero.id)) {
    for (const id of bond.spec.members.filter(member => member !== hero.id)) state.girls[id] = owned(6, 40);
  }
  const bonuses = view.collectionBonds(hero.id);
  assert.equal(bonuses.length, 6);
  assert.ok(bonuses.filter(bond => bond.spec.stat === "atk").reduce((total, bond) => total + bond.percent, 0) > rules.bond_cap);
  const rawAtk = hero.base.atk * (1 + 59 * catalog.GROWTH.levelStatGrow.atk) * (1 + 11 * catalog.GROWTH.starStatGrow.atk);
  assert.equal(view.finalStats(hero.id).atk, Math.round(rawAtk * 1.30) + view.codexBonuses().atk);
  state.girls.c04.star = 9;
  view.openWorkshop("c04");
  const steps = view.starMilestones("c04");
  assert.deepEqual(plain(steps.map(step => step.active)), [true, true, true, false]);
  assert.deepEqual(plain(steps.map(step => step.effect)), ["rage", "skill", "leech", "rage_gain"]);
  for (const step of steps) {
    const node = document.querySelector(`#modal-box [data-star-milestone="${step.star}"]`);
    assert.ok(node.textContent.includes(step.name) && node.textContent.includes(step.description));
  }
  view.openGirl("c28");
  assert.equal(document.querySelectorAll("#modal-box [data-star-milestone]").length, 4, "even an unowned card previews its route");
  assert.equal(document.querySelectorAll('#modal-box button[onclick="closeModal()"].gbtn').length, 1);
});

test("the item guide uses seven independent quality tiers and covers actual equipment and draw pools", () => {
  const tiers = Object.entries(atlas.qualities).sort(([, first], [, second]) => first.rank - second.rank);
  assert.deepEqual(tiers.map(([id]) => id), ["white", "green", "blue", "purple", "gold", "orange", "red"]);
  assert.equal(new Set(tiers.map(([, tier]) => tier.color.toLowerCase())).size, 7);
  for (const [, { name, emblem, color }] of tiers) assert.ok(name && emblem && /^#[a-f\d]{6}$/i.test(color));
  const realIds = [...Object.keys(equipment.materials), ...equipment.slots.map(slot => `gear_${slot.id}`), ...Object.keys(rules.artifacts.gem_sets), "gem_ticket", "chip_ticket", "forge_ore", "training_manual", "weapon_core", "shard_dust", "mat_r", "mat_sr", "mat_ssr"];
  for (const family of Object.keys(rules.artifacts.chip_sets)) for (const index of rules.artifacts.chip_stars.keys()) realIds.push(`${family}_${index + 1}`);
  for (const id of realIds) assert.ok(atlas.items[id], `${id} has a guide entry`);
  for (const [id, item] of Object.entries(atlas.items)) {
    assert.ok(atlas.qualities[item.quality] && atlas.categories[item.category], id);
    assert.ok(item.name && item.icon && item.source && item.use && item.effect, id);
  }
  assert.deepEqual(Object.keys(equipment.drops.campaign).sort(), ["gear_plate", "gear_thread"]);
  for (const rewards of [equipment.drops.campaign_boss, equipment.drops.rift, equipment.drops.nightmare]) assert.ok(rewards.gear_rivet > 0);
});

test("the bag shows real forge stock and complete recipes, with seven distinct quality emblems and five gem cuts", context => {
  const { view, state, document } = workshop(context);
  state.mats = { gear_plate: 4, gear_thread: 1, gear_rivet: 0, shard_dust: 9, shard_dust_sr: 13 };
  view.renderBag();
  for (const id of ["gear_plate", "gear_thread", "gear_rivet"]) {
    const item = atlas.items[id], card = document.querySelector(`[data-bag-material="${id}"]`);
    assert.ok(card.textContent.includes(`持有 ${state.mats[id]}`));
    assert.equal(card.querySelector("[data-rarity-quality]").dataset.rarityQuality, item.quality);
    for (const key of ["source", "use", "effect"]) assert.ok(card.textContent.includes(item[key]));
  }
  view.selectWorkshopBag("materials", "stars");
  assert.equal(document.querySelector('[data-dust-exchange="shard_dust"]').disabled, true);
  assert.equal(document.querySelector('[data-dust-exchange="shard_dust_sr"]').disabled, false);
  const icon = id => {
    const box = document.createElement("div");
    box.innerHTML = view.atelierIcon(id);
    return box.firstElementChild;
  };
  const emblems = new Set();
  for (const [quality, grade] of Object.entries(atlas.qualities)) {
    const [id] = Object.entries(atlas.items).find(([, item]) => item.quality === quality);
    const mark = icon(id);
    assert.ok(mark.getAttribute("aria-label").includes(grade.name));
    emblems.add(mark.querySelector(".at-emblem path").getAttribute("d"));
  }
  assert.equal(emblems.size, 7);
  const shapes = Object.keys(rules.artifacts.gem_sets).map(id => icon(id).querySelector("svg").innerHTML);
  assert.equal(new Set(shapes).size, 5, "element is shown by the cut, not by the quality frame");
});

test("an old weapon is not strengthened twice and gear resonance only counts eligible stars on its wearer", context => {
  const { state, view } = workshop(context), hero = state.girls.c01, base = catalog.GIRLS.find(girl => girl.id === "c01").base;
  hero.star = 12;
  hero.weapon_level = 10;
  const rawAtk = base.atk * (1 + 59 * catalog.GROWTH.levelStatGrow.atk) * (1 + 11 * catalog.GROWTH.starStatGrow.atk);
  const empty = { hp: 0, atk: 0, def: 0, spd: 0, cr: 0, cd: 0 };
  assert.deepEqual(plain(view.boundGearBonuses("c01", hero)), empty);
  assert.equal(view.finalStats("c01").atk, Math.round(rawAtk * 1.20) + 3);
  hero.equipment = Object.fromEntries(equipment.slots.map(slot => [slot.id, slot.id === "weapon" ? { star: 5 } : { level: 10, star: 5 }]));
  assert.deepEqual(plain(view.boundGearBonuses("c01", hero)), { hp: 23, atk: 34, def: 34, spd: 20, cr: 4, cd: 10 });
  state.girls.c08 = { ...owned(12), equipment: structuredClone(hero.equipment) };
  hero.chips = ["edge_1"];
  hero.iv_values = [0, 25, 0, 0, 0, 0];
  assert.equal(view.finalStats("c01").atk, Math.round(rawAtk * 1.40 * 1.20) + view.codexBonuses().atk + equipment.refinement.chips.edge.base);
  hero.equipment.feet.star = 2;
  assert.deepEqual(plain(view.boundGearBonuses("c01", hero)), { hp: 23, atk: 34, def: 34, spd: 12, cr: 0, cd: 0 });
  hero.equipment.body.star = 2;
  assert.deepEqual(plain(view.boundGearBonuses("c01", hero)), { hp: 23, atk: 30, def: 24, spd: 12, cr: 0, cd: 0 });
  delete view.Store.profile.equipment;
  assert.deepEqual(plain(view.boundGearBonuses("c01", hero)), empty);
});

test("stage guides quote real gear materials by tier and settlement decorates only reported item quantities", context => {
  const { view, document, state } = workshop(context);
  state.cleared = Object.fromEntries(catalog.STAGES.map(stage => [stage.id, true]));
  for (const [chapter, id, kind] of [[0, "s1", "campaign"], [1, "s3", "campaign_boss"], [4, "r1", "rift"], [5, "n1", "nightmare"]]) {
    view.pickChapter(chapter);
    view.pickStage(id);
    const drops = [...document.querySelectorAll("#stage-detail [data-gear-drop]")].map(node => [node.dataset.gearDrop, node.textContent]);
    assert.deepEqual(Object.fromEntries(drops), Object.fromEntries(Object.entries(equipment.drops[kind]).map(([key, count]) => [key, `${equipment.materials[key].name} ×${count}`])));
    const mastery = [...document.querySelectorAll("#stage-detail [data-mastery-drop]")];
    assert.deepEqual(Object.fromEntries(mastery.map(node => [node.dataset.masteryDrop, node.textContent])), Object.fromEntries(Object.entries({ ...training.drops[kind], route_badge:carry.drops[kind] }).map(([key, count]) => [key, `${atlas.items[key].name} ×${count}`])));
    assert.ok(mastery.every(node => node.dataset.rewardQuality === atlas.items[node.dataset.masteryDrop].quality));
  }
  const ledger = document.createElement("ul"), before = structuredClone(state.mats);
  ledger.innerHTML = view.battleDropLedger(["辉星铆钉 x2", "金币 x123", "<未识别回执>"]);
  assert.equal(ledger.querySelectorAll("li").length, 3);
  assert.equal(ledger.querySelector("[data-rarity-quality]").dataset.rarityQuality, atlas.items.gear_rivet.quality);
  assert.ok(ledger.textContent.includes("×2") && ledger.lastElementChild.textContent === "<未识别回执>");
  assert.equal(ledger.lastElementChild.childElementCount, 0);
  assert.deepEqual(state.mats, before);
  delete view.Store.profile.items;
  ledger.innerHTML = view.battleDropLedger(["辉星铆钉 x2"]);
  assert.equal(ledger.querySelector("svg"), null);
  assert.equal(ledger.textContent, "辉星铆钉 x2");
});

test("gear auto quotes the maximum payable steps for the selected slot; receipts own every mutation", async context => {
  const { state, view, document } = workshop(context), hero = state.girls.c01, calls = [];
  state.mats = { gear_thread: 12, gear_plate: 0, gear_rivet: 2 };
  const before = structuredClone(state);
  view.Store.mutate = async (action, fields) => { calls.push([action, plain(fields)]); return null; };
  view.openWorkshop("c01", "weapon");
  const pieces = [...document.querySelectorAll("#modal-box [data-atelier-gear]")];
  assert.deepEqual(pieces.map(node => node.dataset.atelierGear).sort(), ["body", "feet", "hands", "head", "weapon"]);
  assert.equal(new Set(pieces.map(node => node.querySelector("svg")?.innerHTML)).size, 5);
  for (const piece of pieces) assert.ok((piece.textContent.trim() || piece.getAttribute("aria-label")) && piece.querySelector("[data-rarity-quality]"));
  const button = action => document.querySelector(`#modal-box [data-gear-action="${action}"]`);
  view.selectAtelierGear("head");
  const quote = view.atelierGearQuote("c01", "head");
  assert.equal(quote.autoLevels, 3);
  assert.equal(quote.autoCost, 12);
  assert.ok(quote.canAuto && !quote.canStar);
  assert.equal(button("gear_auto").disabled, false);
  assert.equal(button("gear_star").disabled, true);
  await view.actAtelierGear("gear_auto");
  await view.actAtelierGear("gear_upgrade");
  await view.actAtelierGear("gear_star");
  view.selectAtelierGear("body");
  await view.actAtelierGear("gear_auto");
  assert.deepEqual(state, before);
  hero.equipment = { head: { level: 2 } };
  hero.star = 1;
  view.selectAtelierGear("head");
  assert.equal(button("gear_star").disabled, true);
  hero.star = 2;
  view.selectAtelierGear("head");
  assert.equal(button("gear_star").disabled, false);
  await view.actAtelierGear("gear_star");
  assert.deepEqual(calls, ["gear_auto", "gear_upgrade", "gear_star"].map(action => [action, { girl_id: "c01", item_id: "head" }]));
  assert.equal(state.mats.gear_rivet, 2);
  hero.equipment.head.level = 10;
  view.selectAtelierGear("head");
  assert.equal(button("gear_auto").disabled, true);
});

test("growth uses a compact owned-card selector and deep-links to the exact gear slot", context => {
  const { view, state, document } = workshop(context);
  state.girls.c04 = { ...owned(12), awakening: 3, branch: "focus" };
  state.girls.c11 = { ...owned(0, 0), shards: 100 };
  state.progression = { days: [], weeks: [], permits: 0, marks: 0, trial_wins: {}, dual: {}, mainline_claims: [], reserve_squad: [] };
  view.Store.profile.progression = { rules: { awakening_costs: [80, 120, 200] }, branches: {} };
  view.renderGrowth();
  const dossier = () => document.querySelector("#growth-body [data-growth-dossier]");
  assert.equal(dossier().dataset.growthDossier, "c01");
  assert.deepEqual([...dossier().querySelectorAll("#growth-hero option")].map(node => node.value), ["c01", "c04"]);
  assert.equal(dossier().querySelectorAll("[data-growth-gear]").length, 5);
  const daily = document.getElementById("growth-contracts");
  if (daily) assert.ok(dossier().compareDocumentPosition(daily) & 4, "the selected wearer comes before chores");
  assert.equal(dossier().querySelector('.at-roster'),null);
  view.selectGrowthGirl("c04");
  assert.equal(dossier().dataset.growthDossier, "c04");
  assert.ok(dossier().querySelector('[data-star-prestige="crown"]'));
  assert.ok(dossier().querySelector('[data-open-workshop="potential"]'));
  view.openGrowthGear("c04", "hands");
  const modal = document.querySelector("#modal-box .hero-workshop");
  assert.equal(modal.dataset.cultivationCard, "c04");
  assert.equal(modal.querySelector('[data-atelier-gear="hands"]').getAttribute("aria-pressed"), "true");
  assert.deepEqual(state.squad, ["c01"]);
  delete view.Store.profile.equipment;
  view.renderGrowth();
  assert.equal(dossier(), null, "a pre-gear host does not advertise unsupported gear");
  view.openWorkshop("c01", "weapon");
  assert.ok(document.querySelector('[data-cultivation-action="weapon_upgrade"]'));
  assert.equal(document.querySelector("[data-gear-action]"), null);
});

test("recycling reserves unlock or next-star shards, leaves remainders and cannot buy another card's star", async context => {
  const { view, state, document } = workshop(context), calls = [];
  view.Store.mutate = async (action, fields) => { calls.push([action, plain(fields)]); return null; };
  for (const [id, rate] of [["c01", 10], ["c11", 5], ["c04", 2]]) {
    const girl = view.GIRLS.find(entry => entry.id === id), hero = state.girls[id] = owned(0, 0);
    const reserved = view.shardReserve(girl, hero);
    hero.shards = reserved + 3 * rate - 1;
    assert.equal(view.recyclableShards(girl, hero), 2 * rate);
    await view.decomposeShards(id, rate + 1);
    await view.decomposeShards(id, 3 * rate);
    view.confirmShardDecompose(id, 2 * rate);
    assert.match(document.querySelector("#modal-box").textContent, /获得 2 枚星锻矿/);
    await view.decomposeShards(id, 2 * rate);
    assert.equal(hero.shards, reserved + 3 * rate - 1);
  }
  Object.assign(state.girls.c01, { star: 5, level: 50, awakening: 1, shards: 99 });
  state.mats = { mat_r: 100, shard_dust: 90, shard_dust_r: 99, shard_dust_ssr: 9, forge_ore: 9999 };
  const before = structuredClone(state);
  await view.starUp("c01");
  await view.exchangeLegacyDust("forge_ore");
  await view.exchangeLegacyDust("shard_dust_ssr");
  await view.exchangeLegacyDust("shard_dust");
  assert.deepEqual(calls, [["shard_decompose", { girl_id: "c01", count: 20 }], ["shard_decompose", { girl_id: "c11", count: 10 }], ["shard_decompose", { girl_id: "c04", count: 4 }], ["salvage_dust", { item_id: "shard_dust" }]]);
  assert.deepEqual(state, before);
});

test("unlocking an undeployed partner immediately refreshes the squad collection and opens the new card", async context => {
  const { view, state, document } = workshop(context);
  document.body.dataset.page = "page-squad";
  state.girls.c04 = owned(4, 20);
  state.girls.c11 = { ...owned(0, 0), shards: catalog.RARITY.SR.shardToUnlock };
  view.renderSquad();
  assert.ok(document.querySelector('#owned-list [data-id="c11"]').classList.contains("card-locked"));
  view.Store.mutate = async (action, fields) => {
    assert.equal(action, "shard_unlock");
    assert.deepEqual(plain(fields), { girl_id: "c11" });
    state.girls.c11 = owned(2, 1);
    return { drops: ["已解锁"] };
  };
  await view.unlockShardGirl("c11");
  assert.deepEqual(state.squad, ["c01"]);
  assert.equal(document.querySelector('#owned-list [data-id="c11"]').classList.contains("card-locked"), false);
  assert.equal(document.querySelector("#modal-mask:not(.hidden) .hero-workshop").dataset.cultivationCard, "c11");
  assert.equal(view.collectionBonds("c11").filter(bond => bond.percent > 0).length, 1);
  assert.equal(document.querySelectorAll("#team-stats [data-album-bond]").length, 0);
  assert.ok(document.querySelectorAll("#team-stats [data-card-bond]").length <= 6);
  view.selectWorkshopTab("bonds");
  const bonds = document.querySelectorAll("#modal-box [data-card-bond]");
  assert.equal(bonds.length, 4);
  assert.equal(document.querySelectorAll("#modal-box [data-album-bond]").length, 0, "other cards' bonds do not flood this dossier");
  assert.ok([...bonds].some(node => node.textContent.includes("炉边誓约") && node.textContent.includes("4%") && node.querySelectorAll(".collection-partner").length === 3));
});

test("carry recipes, grade, stock and worn items remain distinct; actions submit only intent", async context => {
  const { view, state, document } = workshop(context), calls = [];
  view.Store.mutate = async (action, fields) => { calls.push([action, plain(fields)]); return null; };
  state.carry_inventory = { keepsake_lens:1 };
  state.mats = { route_badge:8, forge_ore:3 };
  state.girls.c01.carry = "keepsake_banner";
  view.openWorkshop("c01", "carry");
  view.selectCarryObject("keepsake_lens");
  const action = key => document.querySelector(`#modal-box [data-carry-action="carry_${key}"]`);
  assert.equal(document.querySelectorAll("[data-carry-object]").length, 6);
  for (const entry of document.querySelectorAll("[data-carry-object]")) assert.equal(entry.querySelector("[data-rarity-quality]").dataset.rarityQuality, atlas.items[entry.dataset.carryObject].quality);
  assert.ok(action("craft").disabled && !action("equip").disabled && !action("remove").disabled);
  await view.carryWorkshopAction("carry_craft");
  state.mats.forge_ore++;
  const before = structuredClone(state);
  await view.carryWorkshopAction("carry_craft");
  await view.carryWorkshopAction("carry_equip");
  await view.carryWorkshopAction("carry_remove");
  assert.deepEqual(calls, [["carry_craft",{item_id:"keepsake_lens"}],["carry_equip",{girl_id:"c01",item_id:"keepsake_lens"}],["carry_remove",{girl_id:"c01"}]]);
  assert.deepEqual(state,before);
  view.renderBag();
  view.selectWorkshopBag("carry");
  assert.match(document.querySelector('[data-bag-item="keepsake_lens"]').textContent,/×1/);
  view.openCarryFromBag("keepsake_reed");
  assert.equal(document.querySelector('[data-carry-object="keepsake_reed"]').getAttribute("aria-pressed"), "true");
  delete view.Store.profile.carry;
  view.openWorkshop("c01", "overview");
  assert.equal(document.querySelector('[data-workshop-tab="carry"]'), null);
});

test("carry set tiers require different objects in the squad, unlike ownership crews", context => {
  const { view, state, document } = workshop(context);
  state.girls.c06 = { ...owned(2), carry:"keepsake_lens" };
  state.girls.c08 = { ...owned(2), carry:"keepsake_log" };
  state.girls.c12 = { ...owned(2), carry:"keepsake_reed" };
  state.squad = ["c06","c08"];
  const baseline = view.finalStats("c01");
  state.girls.c01.carry = "keepsake_lens";
  assert.ok(view.finalStats("c01").atk > baseline.atk);
  view.openWorkshop("c01","carry");
  assert.equal(document.querySelectorAll("[data-carry-tier].is-active").length, 0);
  state.squad.push("c01");
  view.selectCarryObject("keepsake_lens");
  assert.deepEqual([...document.querySelectorAll("[data-carry-tier].is-active")].map(entry => entry.dataset.carryTier), ["2"]);
  state.squad.push("c12");
  view.selectCarryObject("keepsake_lens");
  assert.equal(document.querySelectorAll("[data-carry-tier].is-active").length, 2);
  view.selectCarryObject("keepsake_banner");
  assert.equal(document.querySelectorAll("[data-carry-tier].is-active").length, 0);
  for (const id of Object.keys(carry.items)) {
    const entry = carry.items[id];
    assert.ok(atlas.items[id]?.quality && Object.keys(entry.recipe).every(mat => atlas.items[mat]));
    assert.ok(entry.recipe.route_badge > 0 && carry.sets[entry.set].tiers.length === 2);
  }
});

test("legacy workshop hosts keep their own fellowship catalog", context => {
  const {view, state} = workshop(context);
  const legacy = { ...rules }; delete legacy.crews;
  view.Store.profile.cultivation = legacy;
  state.girls.c11 = owned(2,1);
  assert.equal(view.collectionBond("c01").spec.name, "薪火相传");
  assert.equal(view.collectionBond("c01").percent, 4);
  delete legacy.fellowships;
  assert.equal(view.collectionBonds("c01").length, 1);
});

test("12-star quotes protect next-star shards and the last potential node", context => {
  const { state, view, document } = workshop(context);
  const girl = view.GIRLS.find(entry => entry.id === "c01"), card = state.girls.c01;
  const button = action => document.querySelector(`#modal-box [data-cultivation-action="${action}"]`);
  Object.assign(card, { star: 11, shards: 359, awakening: 3, branch: "focus", potential: 5 });
  state.mats.mat_r = catalog.MAT_STAR_COST.R * 8;
  state.mats.shard_dust_sr = 1000;
  assert.equal(view.growthStarMax(), 12);
  assert.equal(view.shardReserve(girl, card), 360);
  assert.equal(view.growthStarStep(girl, card).ready, false);
  state.mats.shard_dust_r = 1;
  state.mats.shard_dust = 500;
  view.openGirl("c01");
  assert.equal(button("star").disabled, true);
  assert.match(document.querySelector(".cultivation-star").textContent, /359\s*\/\s*360/);
  card.shards++;
  view.openGirl("c01");
  assert.equal(button("star").disabled, false);
  assert.match(button("star").textContent, /12/);
  assert.equal(document.querySelectorAll(".star-grades [data-star-index]").length, 12);
  assert.equal(document.querySelectorAll('.star-grades .is-lit').length, 11);
  assert.ok(document.querySelector('[data-star-prestige="crimson"]'));
  view.selectWorkshopTab("potential");
  assert.equal(button("potential_unlock").disabled, true);
  card.star = 12;
  view.openGirl("c01");
  assert.equal(button("star").disabled, true);
  assert.equal(document.querySelectorAll(".star-grades .is-lit").length, 12);
  assert.ok(document.querySelector('[data-star-prestige="crown"]'));
  assert.deepEqual([...document.querySelectorAll("#modal-box [data-star-milestone]")].map(node => Number(node.dataset.starMilestone)), [3, 6, 9, 12]);
  assert.equal(view.shardReserve(girl, card), 0);
  view.selectWorkshopTab("potential");
  assert.equal(button("potential_unlock").disabled, false);
  delete view.Store.profile.cultivation;
  delete view.Store.profile.equipment;
  delete view.Store.profile.items;
  card.star = 5;
  view.openGirl("c01");
  assert.equal(view.growthStarMax(), 5);
  assert.equal(button("star").disabled, true);
  assert.equal(document.querySelectorAll(".star-grades [data-star-index]").length, 5);
  assert.equal(button("weapon_upgrade"), null);
});

test("growth controls send only intents and a rejected or missing receipt never spends locally", async context => {
  const { state, view, document } = workshop(context);
  Object.assign(state.girls.c01, { level: 50, star: 5, awakening: 1, branch: "focus", weapon_level: 5, weapon_rune: "ward", shards: 150 });
  state.mats = { forge_ore: 8, weapon_core: 3, training_manual: 1, mat_r: 100 };
  const calls = [];
  view.Store.mutate = async (action, fields) => { calls.push([action, plain(fields)]); return null; };
  view.openWorkshop("c01", "weapon");
  assert.equal(document.querySelector('[data-choice="ward"]').disabled, true);
  const before = structuredClone(state);
  for (const action of ["weapon_upgrade", "weapon_awaken", "potential_unlock", "use_manual"]) await view.cultivateGirl(action, "c01");
  await view.cultivateGirl("weapon_engrave", "c01", "edge");
  await view.cultivateGirl("weapon_engrave", "c01", "ward");
  await view.cultivateGirl("weapon_engrave", "c01", "unknown");
  await view.cultivateGirl("use_manual", "c04");
  await view.starUp("c01");
  assert.deepEqual(calls, [
    ["weapon_upgrade", { girl_id: "c01" }], ["weapon_awaken", { girl_id: "c01" }],
    ["potential_unlock", { girl_id: "c01" }], ["use_manual", { girl_id: "c01" }],
    ["weapon_engrave", { girl_id: "c01", choice: "edge" }], ["star", { girl_id: "c01" }],
  ]);
  assert.deepEqual(state, before);
  state.girls.c01.level = 60;
  view.openGirl("c01");
  assert.equal(document.querySelector('[data-cultivation-action="use_manual"]').disabled, false);
  state.girls.c01.level = catalog.GROWTH.maxLevel;
  view.openGirl("c01");
  assert.equal(document.querySelector('[data-cultivation-action="use_manual"]').disabled, true);
});

test("artifact sets use equipped pieces on one card, accumulate thresholds and survive position locks", context => {
  const { state, view } = workshop(context);
  const hero = state.girls.c01;
  Object.assign(hero, { star: 12, sockets: 9, gems: Array(9).fill("ruby"), chips: Array.from({ length: 6 }, (_, index) => `edge_${index + 1}`) });
  state.gem_inventory = { ruby: 300 };
  state.chip_inventory = { edge_6: 300 };
  state.girls.c08 = { ...owned(12), chips: [...hero.chips], gems: [...hero.gems] };
  assert.deepEqual(plain(view.artifactBonuses(hero)), { hp: 0, atk: 16, def: 0, spd: 0, cr: 6, cd: 24 });
  const full = view.finalStats("c01");
  hero.chip_locks = 63;
  assert.deepEqual(plain(view.finalStats("c01")), plain(full));
  hero.gems[8] = "wind";
  hero.chips[5] = "tempo_6";
  assert.deepEqual(plain(view.artifactBonuses(hero)), { hp: 0, atk: 15, def: 0, spd: 1, cr: 6, cd: 0 });
  const mixed = view.finalStats("c01");
  assert.ok(mixed.atk < full.atk);
  assert.equal(mixed.spd, full.spd + 4 + equipment.refinement.gems.wind.base);
  assert.equal(mixed.cd, full.cd - 24);
  hero.chips = [];
  hero.gems = ["ruby", "ruby", ""];
  assert.deepEqual(plain(view.artifactBonuses(hero)), { hp: 0, atk: 0, def: 0, spd: 0, cr: 0, cd: 0 });
});

test("individual values apply in full steps on the same stat layer as the equipped sets", context => {
  const { view, state } = workshop(context);
  const hero = state.girls.c01, base = catalog.GIRLS.find(girl => girl.id === "c01").base;
  const unrolled = view.finalStats("c01");
  hero.iv_values = [4, 4, 4, 7, 7, 3];
  assert.deepEqual(plain(view.finalStats("c01")), plain(unrolled));
  hero.iv_values = [29, 31, 4, 8, 24, 31];
  hero.chips = ["edge_1"];
  assert.deepEqual(plain(view.individualBonuses(hero)), { hp: 5, atk: 6, def: 0, spd: 1, cr: 3, cd: 7 });
  const statAt = stat => base[stat] * (1 + (hero.level - 1) * catalog.GROWTH.levelStatGrow[stat]) * (1 + (hero.star - 1) * catalog.GROWTH.starStatGrow[stat]);
  const rolled = view.finalStats("c01");
  assert.deepEqual(plain(rolled), { ...plain(unrolled), hp: Math.round(statAt("hp") * 1.05), atk: Math.round(statAt("atk") * 1.07) + 3 + equipment.refinement.chips.edge.base, spd: unrolled.spd + 1, cr: base.cr + 3, cd: base.cd + 7 });
  hero.iv_locks = 63;
  state.squad = [];
  assert.deepEqual(plain(view.finalStats("c01", true)), plain(rolled));
});

test("the six-axis reroll prices locks in tokens and requires gold and solvent", async context => {
  const { view, state, document } = workshop(context), hero = state.girls.c01, calls = [];
  const roll = () => document.querySelector('[data-workshop-action="iv_roll"]');
  const attackLock = () => document.querySelector('[data-iv-lock="atk"]');
  let receipt = null;
  view.Store.mutate = async (action, fields) => { calls.push([action, plain(fields)]); return receipt; };
  state.mats.iv_solvent = 1; state.gold = rules.individual.gold; state.token = 0;
  view.openWorkshop("c01", "potential");
  view.selectPotentialMode("individual");
  assert.equal(document.querySelectorAll("[data-iv-lock]:disabled").length, 6);
  const emptyPoints = document.querySelector("[data-iv-polygon]").getAttribute("points");
  assert.equal(roll().disabled, false);
  await view.workshopIVAction("iv_lock", "atk");
  await view.workshopIVAction("iv_roll");
  assert.equal(hero.iv_values, undefined);
  hero.iv_values = [5, 31, 10, 0, 2, 8];
  hero.iv_locks = 2;
  view.selectPotentialMode("individual");
  assert.notEqual(document.querySelector("[data-iv-polygon]").getAttribute("points"), emptyPoints);
  assert.equal(attackLock().getAttribute("aria-pressed"), "true");
  assert.equal(roll().disabled, true);
  await view.workshopIVAction("iv_roll");
  await view.workshopIVAction("iv_lock", "atk");
  await view.workshopIVAction("iv_lock", "unknown");
  assert.equal(hero.iv_locks, 2);
  state.token = rules.individual.lock_tokens[1];
  receipt = { drops: ["洗练已确认"] };
  view.selectPotentialMode("individual");
  await view.workshopIVAction("iv_roll");
  assert.ok(attackLock(), "the receipt must not send the player back to the node page");
  assert.deepEqual(hero.iv_values, [5, 31, 10, 0, 2, 8]);
  assert.equal(state.mats.iv_solvent, 1);
  hero.iv_locks = 63;
  state.token = 1000;
  view.selectPotentialMode("individual");
  assert.equal(roll().disabled, true);
  await view.workshopIVAction("iv_roll");
  assert.deepEqual(calls, [["iv_roll", { girl_id: "c01" }], ["iv_lock", { girl_id: "c01", choice: "atk", enabled: false }], ["iv_roll", { girl_id: "c01" }]]);
});

test("nine sockets, six protected chip slots and the six-node potential dial submit constrained intents", async context => {
  const { state, view, document } = workshop(context);
  const hero = state.girls.c01, calls = [];
  Object.assign(hero, { star: 12, sockets: 3, gems: ["ruby", "ruby", ""], chips: ["edge_1"], chip_locks: 1 });
  state.chip_inventory = { aegis_1: 4, tempo_2: 1 };
  state.gem_inventory = { ruby: 2 };
  state.gold = 2000;
  state.mats.mat_r = 100;
  view.Store.mutate = async (action, fields) => { calls.push([action, plain(fields)]); return null; };
  const buttons = () => document.querySelectorAll(".ws-dial [data-workshop-slot]");
  const control = action => document.querySelector(`[data-workshop-action="${action}"]`);
  view.openWorkshop("c01", "gems");
  assert.equal(buttons().length, 9);
  view.selectWorkshopSlot(4);
  assert.equal(control("gem_socket").disabled, false);
  await view.workshopSlotAction("gem_socket");
  view.selectWorkshopSlot(9);
  assert.equal(control("gem_socket").disabled, true);
  view.selectWorkshopTab("chips");
  assert.equal(buttons().length, 6);
  view.selectWorkshopItem("aegis_1");
  assert.equal(control("chip_equip").disabled, true);
  assert.equal(control("chip_remove").disabled, true);
  await view.workshopSlotAction("chip_equip");
  await view.workshopSlotAction("chip_lock");
  view.selectWorkshopSlot(2);
  view.selectWorkshopItem("tempo_2");
  await view.workshopSlotAction("chip_equip");
  view.selectWorkshopTab("potential");
  assert.equal(buttons().length, 6);
  assert.equal(document.querySelectorAll(".dial-potential svg polygon").length, 2);
  assert.deepEqual(calls, [
    ["gem_socket", { girl_id: "c01", count: 4 }],
    ["chip_lock", { girl_id: "c01", count: 1, enabled: false }],
    ["chip_equip", { girl_id: "c01", count: 2, item_id: "tempo_2" }],
  ]);
  assert.equal(hero.sockets, 3);
  assert.equal(hero.chip_locks, 1);
  assert.deepEqual(hero.chips, ["edge_1"]);
});

test("the workshop squad shortcut waits for its receipt without closing a different card", async context => {
  const { view, state, document } = workshop(context), calls = [];
  state.squad = ["c08", "c06", "c12", "c11"];
  for (const id of state.squad) state.girls[id] = owned(1, 1);
  const button = () => document.querySelector('[data-workshop-action="squad"]');
  let complete;
  view.Store.mutate = (action, fields) => {
    calls.push([action, plain(fields)]);
    return new Promise(resolve => { complete = resolve; });
  };
  view.openWorkshop("c01");
  assert.equal(button().disabled, true);
  await view.workshopSquadAction();
  assert.deepEqual(calls, []);
  state.squad = ["c08"];
  view.openWorkshop("c01");
  const pending = view.workshopSquadAction();
  assert.equal(button().disabled, true);
  await view.workshopSquadAction();
  assert.deepEqual(state.squad, ["c08"]);
  view.openWorkshop("c12");
  state.squad = ["c08", "c01"];
  complete({});
  await pending;
  assert.deepEqual(calls, [["squad", { squad: ["c08", "c01"] }]]);
  assert.equal(document.querySelector(".hero-workshop").dataset.cultivationCard, "c12");
  assert.equal(button().disabled, false);
});

test("an item-pool return restores the wearer and numbered socket, not a different account or pool", context => {
  const { view, state, document } = workshop(context);
  document.body.dataset.page = "page-squad";
  state.gem_inventory = { sapphire: 4 };
  view.openWorkshop("c01", "gems");
  view.selectWorkshopSlot(5);
  view.selectWorkshopItem("sapphire");
  view.openArtifactDraw("gems");
  const back = () => document.querySelector("[data-draw-return]");
  assert.ok(back());
  view.returnFromArtifactDraw();
  assert.equal(document.querySelector("#modal-mask:not(.hidden) .hero-workshop").dataset.cultivationCard, "c01");
  assert.equal(document.querySelector('[data-workshop-slot="5"]').getAttribute("aria-pressed"), "true");
  assert.equal(document.querySelector('[data-workshop-item="sapphire"]').getAttribute("aria-pressed"), "true");
  view.openArtifactDraw("gems");
  view.selectDrawPool("chips");
  assert.equal(back(), null);
  view.openWorkshop("c01", "chips");
  view.openArtifactDraw("chips");
  view.Store.sessionEpoch++;
  view.selectDrawPool("chips");
  assert.equal(back(), null);
});

test("separate ticket pools expose odds, enforce budgets and show only the returned receipt", async context => {
  const { state, view, document } = workshop(context), calls = [];
  document.body.dataset.page = "page-gacha";
  state.mats = { gem_ticket: 1, chip_ticket: 10, forge_ore: 8 };
  state.daily = {date:"2026-11-28",artifact_free:{gem_ticket:true,chip_ticket:true}};
  view.Store.serverNow = () => Date.parse("2026-11-28T10:00:00Z");
  view.Store.mutate = async (action, fields) => { calls.push([action, plain(fields)]); return { drops: ["回执 <b>已收录</b>"] }; };
  view.selectDrawPool("gems");
  assert.equal(document.querySelectorAll(".ws-artifact-grid > div").length, 5);
  assert.match(document.querySelector("#artifact-draw").textContent, /1\/5/);
  assert.equal(document.querySelector('[data-artifact-draw="10"]').disabled, true);
  await view.drawArtifacts(10);
  await view.drawArtifacts(1);
  assert.equal(document.querySelector(".ws-draw-receipt li").textContent, "回执 <b>已收录</b>");
  assert.equal(document.querySelector(".ws-draw-receipt li b"), null);
  view.selectDrawPool("chips");
  assert.match(document.querySelector("#artifact-draw").textContent, /1\/18/);
  assert.equal(document.querySelectorAll(".ws-artifact-grid > div").length, 18);
  assert.equal(document.querySelector(".ws-draw-receipt li"), null);
  await view.drawArtifacts(10);
  await view.convertArtifactTicket();
  assert.deepEqual(calls, [["draw_gems", { count: 1, choice:"ticket" }], ["draw_chips", { count: 10, choice:"ticket" }], ["convert_tickets", { item_id: "chip_ticket" }]]);
  assert.deepEqual(state.mats, { gem_ticket: 1, chip_ticket: 10, forge_ore: 8 });
  view.renderBag();
  assert.equal(document.querySelectorAll("[data-bag-tab]").length, 6);
  view.selectWorkshopBag("materials", "tickets");
  assert.match(document.getElementById("bag-body").textContent, /宝石召唤券/);
  assert.match(document.getElementById("bag-body").textContent, /芯片召唤券/);
});

test("gem and chip daily gifts are independent; paid draws use only the chosen currency", async context => {
  const {state,view,document} = workshop(context), calls = [];
  state.daily = {date:"2026-11-28"}; state.token = 8; state.mats = {gem_ticket:1};
  view.Store.serverNow = () => Date.parse("2026-11-28T15:59:50Z");
  view.Store.mutate = async (action,fields) => { calls.push([action,plain(fields)]); return {drops:["本次收取 1 件"]}; };
  document.body.dataset.page = "page-gacha";
  view.selectDrawPool("gems");
  assert.equal(document.querySelector('[data-artifact-payment="free"]').disabled,false);
  assert.equal(document.querySelector('[data-artifact-draw="1"]').disabled,true);
  assert.equal(document.querySelector('[data-artifact-token="1"]').disabled,true);
  await view.drawArtifacts(1); await view.drawArtifacts(10); await view.drawArtifacts(1,"token"); await view.drawArtifacts(10,"token"); await view.drawArtifacts(10,"free"); await view.drawArtifacts(1,"free");
  assert.deepEqual(calls,[["draw_gems",{count:1,choice:"free",item_id:"2026-11-28"}]]);
  assert.equal(state.daily.artifact_free,undefined);
  state.daily.artifact_free={gem_ticket:true}; view.renderGacha();
  assert.equal(document.querySelector('[data-artifact-payment="free"]').disabled,true);
  assert.equal(document.querySelector('[data-artifact-token="10"]').disabled,true);
  await view.drawArtifacts(1,"free"); await view.drawArtifacts(10,"token"); await view.drawArtifacts(1,"token"); await view.drawArtifacts(1);
  assert.deepEqual(calls.slice(1),[["draw_gems",{count:1,choice:"token"}],["draw_gems",{count:1,choice:"ticket"}]]);
  view.selectDrawPool("chips"); await view.drawArtifacts(1,"token"); await view.drawArtifacts(1,"free");
  assert.deepEqual(calls.at(-1),["draw_chips",{count:1,choice:"free",item_id:"2026-11-28"}]);
  view.Store.serverNow = () => Date.parse("2026-11-28T16:00:00Z"); view.renderGacha();
  assert.match(document.querySelector('[data-artifact-payment="free"]').textContent,/同步/);
  await view.drawArtifacts(1,"free"); await view.drawArtifacts(1,"token"); assert.equal(calls.length,4);
});

const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");
const test = require("node:test");

const root = path.resolve(__dirname, "..");
const read = name => fs.readFileSync(path.join(root, name), "utf8");
const json = name => JSON.parse(read(name));
const plain = value => JSON.parse(JSON.stringify(value));
const catalog = json("backend/model/gacha_catalog.json");
const context = vm.createContext({});
vm.runInContext(read("js/data.js") + ";globalThis.units = { GIRLS, ENEMY_TPL, UNIT_ASSET_ALIASES, unitAsset };", context);

test("card-owned definitions agree with both runtime catalogs and own their artwork", () => {
  const groups = {}, aliases = {}, identities = new Set();
  for (const [group, filename] of Object.entries({ cards: "card.json", enemies: "enemy.json", bosses: "boss.json" })) {
    const directory = `content/${group}`;
    const { schemaVersion, ids } = json(`${directory}/index.json`);
    assert.equal(schemaVersion, 1);
    groups[group] = ids.map(id => {
      assert(!identities.has(id), `duplicate ID: ${id}`);
      identities.add(id);
      const unit = json(`${directory}/${id}/${filename}`);
      assert.equal(unit.id, id);
      for (const image of [unit.img, unit.battleImg, ...Object.values(unit.legacyImages || {})]) {
        assert(image.startsWith(`${directory}/${id}/assets/`), image);
        assert(fs.statSync(path.join(root, image)).isFile(), image);
      }
      for (const [old, image] of Object.entries(unit.legacyImages || {})) {
        assert.equal(aliases[old], undefined, `duplicate alias: ${old}`);
        aliases[old] = image;
      }
      delete unit.legacyImages;
      return unit;
    });
    assert.deepEqual(fs.readdirSync(path.join(root, directory), { withFileTypes: true }).filter(entry => entry.isDirectory()).map(entry => entry.name).sort(), [...ids].sort());
  }
  const enemies = [...groups.enemies, ...groups.bosses];
  assert.deepEqual(catalog.GIRLS, groups.cards);
  assert.deepEqual(catalog.ENEMY_TPL, enemies);
  assert.deepEqual(plain(context.units.GIRLS), groups.cards);
  assert.deepEqual(plain(context.units.ENEMY_TPL), enemies);
  assert.deepEqual(plain(context.units.UNIT_ASSET_ALIASES), aliases);
  assert(groups.enemies.every(enemy => enemy.boss === false));
  assert(groups.bosses.every(enemy => enemy.boss === true));
});

test("historical reports resolve old portraits and transparent art after relocation", () => {
  for (const [old, image] of Object.entries(context.units.UNIT_ASSET_ALIASES)) {
    assert.equal(context.units.unitAsset(old), image);
    assert.equal(context.units.unitAsset(image), image);
    if (!old.endsWith("-cut.webp")) {
      const battle = context.units.unitAsset(old).replace(/\.webp$/, "-cut.webp");
      assert(fs.existsSync(path.join(root, battle)), battle);
    }
  }
  assert.equal(context.units.unitAsset("assets/d01-volcano.webp"), "assets/d01-volcano.webp");
  assert.equal(context.units.unitAsset("assets/enemy-boss-cut.webp"), "content/bosses/abyss-lord/assets/legacy-cut.webp");
  for (const group of ["cards", "enemies", "bosses"]) assert(!fs.existsSync(path.join(root, group)), "unit directories belong under content/");
});

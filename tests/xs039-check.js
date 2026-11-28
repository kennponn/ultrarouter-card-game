const test=require("node:test");
const assert=require("node:assert/strict");
const fs=require("node:fs");
const path=require("node:path");
const vm=require("node:vm");
const root=path.resolve(__dirname,"..");
const read=file=>fs.readFileSync(path.join(root,file),"utf8");

function player() {
  const context=vm.createContext({window:{},document:{addEventListener(){}},console});
  vm.runInContext(read("js/battle.js")+`
    let draws=0, logs=[];
    updateUnit=()=>{};renderBattle=()=>{draws++};renderBattleTimeline=()=>{};batLog=text=>logs.push(text);
    globalThis.api={mergeBattleEvent,get draws(){return draws},logs};
  `,context);
  const battle={allies:[{uid:"a0",slot:0,hp:80},{uid:"a1",slot:1,hp:20}],waves:[[{uid:"e0",slot:0,hp:100}]],waveIdx:0};
  return {api:context.api,battle};
}

test("server swaps replay atomically, including skipped/reloaded events, without rewriting report input",()=>{
  const {api,battle}=player();
  const event={round:2,units:[{uid:"a0",position:1,hp:80},{uid:"a1",position:0,hp:60}]},before=JSON.stringify(event);
  api.mergeBattleEvent(battle,event);
  assert.deepEqual(battle.allies.map(u=>u.slot),[1,0]);assert.equal(battle.allies[1].hp,60);assert.equal(api.draws,1);
  api.mergeBattleEvent(battle,event);assert.equal(api.draws,1);assert.equal(JSON.stringify(event),before);
  api.mergeBattleEvent(battle,{round:3,units:[{uid:"a0",hp:70}]});assert.equal(battle.allies[0].slot,1);
});

test("invalid partial swaps are rejected before any state changes",()=>{
  const {api,battle}=player(),before=JSON.stringify(battle);
  assert.throws(()=>api.mergeBattleEvent(battle,{units:[{uid:"a0",position:1,hp:1}]}),/Invalid server formation/);
  assert.equal(JSON.stringify(battle),before);assert.equal(api.draws,0);
});

test("imported art, explicit rotations, and exclusive acquisition are present in the shipped catalog",()=>{
  const card=JSON.parse(read("content/cards/xs039/card.json"));
  assert.deepEqual(card.sources,["world"]);assert.equal(card.initialStars,5);assert.equal(card.base.spAtk,78);
  assert.equal(card.skills["xs039-skill-2"].name,"空壳留震·三榫共鸣");
  for(const asset of [card.img,card.battleImg,...["skill","xs039-skill-2"].flatMap(skill=>["cast","impact"].map(phase=>`content/cards/xs039/assets/fx/${skill}/${phase}.webp`))]) assert.ok(fs.statSync(path.join(root,asset)).size>0);
  const rotation=JSON.parse(read("backend/model/gacha_world.json")).rotations.at(-1);
  assert.equal(rotation.starts,"2026-11-30");assert.equal(rotation.heroes[0],"xs039");assert.equal(new Set(rotation.heroes).size,rotation.heroes.length);
});

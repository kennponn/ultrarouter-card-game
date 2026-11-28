/* =====================================================
 * data.js —— 展示用图鉴与规则；账号进度只从服务器读取
 * ===================================================== */

/* ---------- 稀有度配置 ---------- */
const RARITY = {
  R:   { name: "R",   color: "#6d7f9a", shardToUnlock: 20,  baseStar: 1 },
  SR:  { name: "SR",  color: "#4a7dff", shardToUnlock: 50,  baseStar: 2 },
  SSR: { name: "SSR", color: "#d9a94a", shardToUnlock: 80,  baseStar: 3 },
};

/* ---------- 元素 ---------- */
const ELEM = {
  fire:  { name: "焰", color: "#e05555" },
  water: { name: "水", color: "#4a7dff" },
  plant: { name: "木", color: "#4fbf7f" },
  star:  { name: "星", color: "#9a6bff" },
  dark:  { name: "暗", color: "#8a93a8" },
  time:  { name: "时", color: "#3aa8a0" },
};

/* ---------- 28 位少女 ----------
 * role: tank坦克 / sdps单体 / adps群攻 / heal治疗 / support辅助
 * dupGain: 重复获得转为碎片数
 * matBonus: 材料掉落加成（进入伤害公式的 (1 + 材料掉落加成) 与掉落数量）
 * skills: { normal普攻, skill怒气技能(满怒100释放) }
 */
// BEGIN GENERATED GIRLS — run python scripts/sync-catalog.py
const GIRLS = [
  {
    "id": "c01",
    "sources": [
      "crew",
      "summon",
      "world",
      "voyage",
      "growth"
    ],
    "name": "灯里",
    "rarity": "R",
    "elem": "fire",
    "role": "adps",
    "atkType": "magic",
    "img": "content/cards/c01/assets/portrait.webp",
    "base": {
      "hp": 820,
      "atk": 168,
      "def": 62,
      "spd": 102,
      "cr": 5,
      "cd": 150,
      "ehr": 0,
      "res": 10
    },
    "matBonus": 0,
    "dupGain": 8,
    "skills": {
      "normal": {
        "name": "火苗拍击",
        "mult": 1,
        "target": "enemy",
        "desc": "对单体造成100%攻击伤害"
      },
      "skill": {
        "name": "火焰乱舞",
        "mult": 1.6,
        "target": "allEnemies",
        "desc": "对全体造成160%攻击伤害"
      }
    },
    "desc": "喜欢念书的火系见习魔法师，怒气技为全体火焰。",
    "battleImg": "content/cards/c01/assets/portrait-cut.webp",
    "fxColor": "#ff8a5c"
  },
  {
    "id": "c02",
    "name": "小满",
    "rarity": "R",
    "elem": "plant",
    "role": "support",
    "atkType": "ranged",
    "img": "content/cards/c02/assets/portrait.webp",
    "base": {
      "hp": 900,
      "atk": 120,
      "def": 70,
      "spd": 98,
      "cr": 5,
      "cd": 150,
      "ehr": 0,
      "res": 15
    },
    "matBonus": 0.05,
    "dupGain": 8,
    "skills": {
      "normal": {
        "name": "篮击",
        "mult": 1,
        "target": "enemy",
        "desc": "对单体造成100%攻击伤害"
      },
      "skill": {
        "name": "丰收祝福",
        "mult": 0,
        "target": "team",
        "healMult": 0.8,
        "desc": "治疗全队（80%攻击），并提升本次副本材料掉落"
      }
    },
    "desc": "菜园少女，上阵时副本材料掉落+5%。",
    "battleImg": "content/cards/c02/assets/portrait-cut.webp",
    "fxColor": "#9adf6e"
  },
  {
    "id": "c06",
    "sources": [
      "crew",
      "summon",
      "world",
      "voyage",
      "growth"
    ],
    "name": "铃羽",
    "rarity": "R",
    "elem": "plant",
    "role": "sdps",
    "atkType": "ranged",
    "img": "content/cards/c06/assets/portrait.webp",
    "base": {
      "hp": 780,
      "atk": 185,
      "def": 55,
      "spd": 108,
      "cr": 8,
      "cd": 150,
      "ehr": 0,
      "res": 10
    },
    "matBonus": 0,
    "dupGain": 8,
    "skills": {
      "normal": {
        "name": "疾风箭",
        "mult": 1,
        "target": "enemy",
        "desc": "对单体造成100%攻击伤害"
      },
      "skill": {
        "name": "贯穿之矢",
        "mult": 3.2,
        "target": "enemy",
        "desc": "对单体造成320%攻击伤害"
      }
    },
    "desc": "草原弓手，怒气技对单体造成高额伤害。",
    "battleImg": "content/cards/c06/assets/portrait-cut.webp",
    "fxColor": "#a5e07a"
  },
  {
    "id": "c08",
    "sources": [
      "crew",
      "summon",
      "world",
      "voyage",
      "growth"
    ],
    "name": "小椛",
    "rarity": "R",
    "elem": "plant",
    "role": "tank",
    "atkType": "melee",
    "img": "content/cards/c08/assets/portrait.webp",
    "base": {
      "hp": 1450,
      "atk": 105,
      "def": 105,
      "spd": 92,
      "cr": 5,
      "cd": 150,
      "ehr": 0,
      "res": 20
    },
    "matBonus": 0,
    "dupGain": 8,
    "skills": {
      "normal": {
        "name": "盾击",
        "mult": 0.9,
        "target": "enemy",
        "desc": "对单体造成90%攻击伤害"
      },
      "skill": {
        "name": "樱花铁壁",
        "mult": 0,
        "target": "self",
        "shieldMult": 2.2,
        "taunt": true,
        "desc": "获得相当于220%攻击的护盾并嘲讽全体敌人1回合"
      }
    },
    "desc": "举着樱花木盾的少女，队伍前排。",
    "battleImg": "content/cards/c08/assets/portrait-cut.webp",
    "fxColor": "#ffb0c8"
  },
  {
    "id": "c12",
    "sources": [
      "crew",
      "summon",
      "world",
      "voyage",
      "growth"
    ],
    "name": "苺",
    "rarity": "R",
    "elem": "plant",
    "role": "heal",
    "atkType": "ranged",
    "img": "content/cards/c12/assets/portrait.webp",
    "base": {
      "hp": 880,
      "atk": 128,
      "def": 60,
      "spd": 100,
      "cr": 5,
      "cd": 150,
      "ehr": 0,
      "res": 15
    },
    "matBonus": 0,
    "dupGain": 8,
    "skills": {
      "normal": {
        "name": "草药投掷",
        "mult": 0.9,
        "target": "enemy",
        "desc": "对单体造成90%攻击伤害"
      },
      "skill": {
        "name": "生命花束",
        "mult": 0,
        "target": "ally",
        "healMult": 2.6,
        "desc": "治疗当前生命最低的队友（260%攻击）"
      }
    },
    "desc": "庭院草药少女，单体急救。",
    "battleImg": "content/cards/c12/assets/portrait-cut.webp",
    "fxColor": "#8ff0b6"
  },
  {
    "id": "c20",
    "sources": [
      "world",
      "voyage"
    ],
    "name": "暖弦",
    "rarity": "R",
    "elem": "plant",
    "role": "support",
    "atkType": "magic",
    "img": "content/cards/c20/assets/portrait.webp",
    "base": {
      "hp": 910,
      "atk": 135,
      "def": 65,
      "spd": 105,
      "cr": 5,
      "cd": 150,
      "ehr": 0,
      "res": 15
    },
    "matBonus": 0,
    "dupGain": 8,
    "skills": {
      "normal": {
        "name": "拨弦轻响",
        "mult": 1,
        "target": "enemy",
        "desc": "对单体造成100%攻击伤害"
      },
      "skill": {
        "name": "风铃旅歌",
        "mult": 0,
        "target": "team",
        "teamAtkUp": 12,
        "buffTurns": 2,
        "desc": "全队攻击提升12%，持续2回合"
      }
    },
    "desc": "风铃旅人，抱着旧琴走过集市与花田，用轻快的歌声鼓舞同伴。",
    "battleImg": "content/cards/c20/assets/portrait-cut.webp",
    "fxColor": "#4fbf7f"
  },
  {
    "id": "c21",
    "name": "柚芽",
    "rarity": "R",
    "elem": "plant",
    "role": "support",
    "atkType": "magic",
    "img": "content/cards/c21/assets/portrait.webp",
    "base": {
      "hp": 920,
      "atk": 140,
      "def": 65,
      "spd": 103,
      "cr": 5,
      "cd": 150,
      "ehr": 0,
      "res": 15
    },
    "matBonus": 0,
    "dupGain": 8,
    "skills": {
      "normal": {
        "name": "药瓶轻投",
        "mult": 0.9,
        "target": "enemy",
        "desc": "对单体造成90%攻击伤害"
      },
      "skill": {
        "name": "晴叶调剂",
        "mult": 0,
        "target": "team",
        "healMult": 1,
        "teamAtkUp": 8,
        "buffTurns": 2,
        "desc": "治疗全队（100%攻击），全队攻击提升8%（2回合）"
      }
    },
    "desc": "花房里的见习药剂师，将晨露与草叶调成清甜药剂，为同伴恢复精神。",
    "battleImg": "content/cards/c21/assets/portrait-cut.webp",
    "fxColor": "#4fbf7f"
  },
  {
    "id": "c22",
    "sources": [
      "world",
      "growth"
    ],
    "name": "榛果",
    "rarity": "R",
    "elem": "plant",
    "role": "tank",
    "atkType": "heavy",
    "img": "content/cards/c22/assets/portrait.webp",
    "base": {
      "hp": 1500,
      "atk": 125,
      "def": 98,
      "spd": 89,
      "cr": 5,
      "cd": 150,
      "ehr": 0,
      "res": 20
    },
    "matBonus": 0,
    "dupGain": 8,
    "skills": {
      "normal": {
        "name": "橡锤重敲",
        "mult": 1,
        "target": "enemy",
        "desc": "对单体造成100%攻击伤害"
      },
      "skill": {
        "name": "坚果堡垒",
        "mult": 0,
        "target": "self",
        "shieldMult": 2.4,
        "taunt": true,
        "selfDefUp": 15,
        "desc": "自身获得240%攻击的护盾，嘲讽全体敌人并提升15%防御1回合"
      }
    },
    "desc": "扛着重锤的林间护卫，走得慢却站得稳，总把最危险的位置留给自己。",
    "battleImg": "content/cards/c22/assets/portrait-cut.webp",
    "fxColor": "#4fbf7f"
  },
  {
    "id": "c11",
    "name": "剑华",
    "rarity": "SR",
    "elem": "fire",
    "role": "sdps",
    "atkType": "melee",
    "img": "content/cards/c11/assets/portrait.webp",
    "base": {
      "hp": 1150,
      "atk": 265,
      "def": 85,
      "spd": 112,
      "cr": 12,
      "cd": 160,
      "ehr": 0,
      "res": 15
    },
    "matBonus": 0,
    "dupGain": 15,
    "skills": {
      "normal": {
        "name": "正拳",
        "mult": 1,
        "target": "enemy",
        "desc": "对单体造成100%攻击伤害"
      },
      "skill": {
        "name": "烈焰连击",
        "mult": 4.2,
        "target": "enemy",
        "hits": 2,
        "desc": "对单体造成两段210%攻击伤害"
      }
    },
    "desc": "拳法少女，单体爆发。",
    "battleImg": "content/cards/c11/assets/portrait-cut.webp",
    "fxColor": "#ff9c5c"
  },
  {
    "id": "c05",
    "sources": [
      "summon",
      "voyage",
      "growth"
    ],
    "name": "霜音",
    "rarity": "SR",
    "elem": "water",
    "role": "adps",
    "atkType": "magic",
    "img": "content/cards/c05/assets/portrait.webp",
    "base": {
      "hp": 1080,
      "atk": 240,
      "def": 78,
      "spd": 104,
      "cr": 8,
      "cd": 150,
      "ehr": 20,
      "res": 15
    },
    "matBonus": 0,
    "dupGain": 15,
    "skills": {
      "normal": {
        "name": "霜刃",
        "mult": 1,
        "target": "enemy",
        "desc": "对单体造成100%攻击伤害"
      },
      "skill": {
        "name": "凛冬低语",
        "mult": 1.8,
        "target": "allEnemies",
        "slow": 25,
        "slowTurns": 2,
        "desc": "对全体造成180%攻击伤害，并使目标速度降低25%，持续2回合"
      }
    },
    "desc": "雪街的冰系少女，群体减速。",
    "battleImg": "content/cards/c05/assets/portrait-cut.webp",
    "fxColor": "#a8e0ff"
  },
  {
    "id": "c13",
    "sources": [
      "summon",
      "voyage",
      "growth"
    ],
    "name": "圣愈",
    "rarity": "SR",
    "elem": "water",
    "role": "heal",
    "atkType": "magic",
    "img": "content/cards/c13/assets/portrait.webp",
    "base": {
      "hp": 1250,
      "atk": 210,
      "def": 90,
      "spd": 98,
      "cr": 5,
      "cd": 150,
      "ehr": 10,
      "res": 25
    },
    "matBonus": 0,
    "dupGain": 15,
    "skills": {
      "normal": {
        "name": "圣光弹",
        "mult": 0.9,
        "target": "enemy",
        "desc": "对单体造成90%攻击伤害"
      },
      "skill": {
        "name": "圣泉祷言",
        "mult": 0,
        "target": "team",
        "healMult": 1.6,
        "desc": "治疗全队（160%攻击）并解除1个负面效果"
      }
    },
    "desc": "教堂修女，群体治疗。",
    "battleImg": "content/cards/c13/assets/portrait-cut.webp",
    "fxColor": "#8fd0ff"
  },
  {
    "id": "c10",
    "name": "绯破",
    "rarity": "SR",
    "elem": "dark",
    "role": "sdps",
    "atkType": "melee",
    "img": "content/cards/c10/assets/portrait.webp",
    "base": {
      "hp": 1120,
      "atk": 258,
      "def": 82,
      "spd": 110,
      "cr": 10,
      "cd": 155,
      "ehr": 15,
      "res": 12
    },
    "matBonus": 0,
    "dupGain": 15,
    "skills": {
      "normal": {
        "name": "居合",
        "mult": 1,
        "target": "enemy",
        "desc": "对单体造成100%攻击伤害"
      },
      "skill": {
        "name": "破甲太刀",
        "mult": 3,
        "target": "enemy",
        "defBreak": 30,
        "defBreakTurns": 3,
        "desc": "对单体造成300%攻击伤害，并使其防御降低30%，持续3回合"
      }
    },
    "desc": "雨夜刀客，破甲削减防御。",
    "battleImg": "content/cards/c10/assets/portrait-cut.webp",
    "fxColor": "#b06eff"
  },
  {
    "id": "c14",
    "name": "缚锁",
    "rarity": "SR",
    "elem": "dark",
    "role": "support",
    "atkType": "ranged",
    "img": "content/cards/c14/assets/portrait.webp",
    "base": {
      "hp": 1100,
      "atk": 220,
      "def": 84,
      "spd": 106,
      "cr": 6,
      "cd": 150,
      "ehr": 35,
      "res": 15
    },
    "matBonus": 0,
    "dupGain": 15,
    "skills": {
      "normal": {
        "name": "锁镰",
        "mult": 1,
        "target": "enemy",
        "desc": "对单体造成100%攻击伤害"
      },
      "skill": {
        "name": "禁锢之链",
        "mult": 1.2,
        "target": "enemy",
        "stunChance": 0.6,
        "stunTurns": 1,
        "desc": "对单体造成120%攻击伤害，60%概率使其眩晕1回合（受效果命中影响）"
      }
    },
    "desc": "锁链使，控制敌方关键单位。",
    "battleImg": "content/cards/c14/assets/portrait-cut.webp",
    "fxColor": "#9a7aff"
  },
  {
    "id": "c17",
    "sources": [
      "voyage",
      "growth"
    ],
    "name": "澄汐",
    "rarity": "SR",
    "elem": "water",
    "role": "heal",
    "atkType": "magic",
    "img": "content/cards/c17/assets/portrait.webp",
    "base": {
      "hp": 1200,
      "atk": 205,
      "def": 88,
      "spd": 101,
      "cr": 5,
      "cd": 150,
      "ehr": 0,
      "res": 25
    },
    "matBonus": 0,
    "dupGain": 15,
    "skills": {
      "normal": {
        "name": "清波",
        "mult": 0.9,
        "target": "enemy",
        "desc": "对单体造成90%攻击伤害"
      },
      "skill": {
        "name": "归港潮声",
        "mult": 0,
        "target": "team",
        "healMult": 1.2,
        "shieldMult": 0.8,
        "desc": "治疗全队（120%攻击），并给予相当于80%攻击的护盾"
      }
    },
    "desc": "潮泉守望者，将港湾的潮声收进瓷碗，为远行归来的伙伴疗伤。",
    "battleImg": "content/cards/c17/assets/portrait-cut.webp",
    "fxColor": "#4a7dff"
  },
  {
    "id": "c18",
    "name": "朱笺",
    "rarity": "SR",
    "elem": "dark",
    "role": "support",
    "atkType": "magic",
    "img": "content/cards/c18/assets/portrait.webp",
    "base": {
      "hp": 1180,
      "atk": 215,
      "def": 94,
      "spd": 103,
      "cr": 5,
      "cd": 150,
      "ehr": 0,
      "res": 25
    },
    "matBonus": 0,
    "dupGain": 15,
    "skills": {
      "normal": {
        "name": "飞符",
        "mult": 1,
        "target": "enemy",
        "desc": "对单体造成100%攻击伤害"
      },
      "skill": {
        "name": "四方镇印",
        "mult": 0,
        "target": "team",
        "shieldMult": 1.6,
        "teamAtkUp": 10,
        "buffTurns": 2,
        "desc": "全队获得160%攻击的护盾，攻击提升10%（2回合）"
      }
    },
    "desc": "雨廊符师，寡言却细心。她把守护的约定，一枚一枚盖进朱红符印。",
    "battleImg": "content/cards/c18/assets/portrait-cut.webp",
    "fxColor": "#8a93a8"
  },
  {
    "id": "c19",
    "name": "岚翎",
    "rarity": "SR",
    "elem": "plant",
    "role": "sdps",
    "atkType": "melee",
    "img": "content/cards/c19/assets/portrait.webp",
    "base": {
      "hp": 1110,
      "atk": 250,
      "def": 78,
      "spd": 116,
      "cr": 12,
      "cd": 155,
      "ehr": 0,
      "res": 15
    },
    "matBonus": 0,
    "dupGain": 15,
    "skills": {
      "normal": {
        "name": "掠风刺",
        "mult": 1,
        "target": "enemy",
        "desc": "对单体造成100%攻击伤害"
      },
      "skill": {
        "name": "穿林双燕",
        "mult": 4,
        "target": "enemy",
        "hits": 2,
        "ignoreDefPct": 10,
        "desc": "对单体造成两段200%攻击伤害，无视目标10%防御"
      }
    },
    "desc": "追风游哨，踏着溪石穿行山野，长枪与青色围巾总比报信的风先到。",
    "battleImg": "content/cards/c19/assets/portrait-cut.webp",
    "fxColor": "#4fbf7f"
  },
  {
    "id": "c23",
    "sources": [
      "summon",
      "voyage",
      "growth"
    ],
    "name": "夜萤",
    "rarity": "SR",
    "elem": "dark",
    "role": "sdps",
    "atkType": "melee",
    "img": "content/cards/c23/assets/portrait.webp",
    "base": {
      "hp": 1070,
      "atk": 255,
      "def": 75,
      "spd": 119,
      "cr": 16,
      "cd": 160,
      "ehr": 0,
      "res": 15
    },
    "matBonus": 0,
    "dupGain": 15,
    "skills": {
      "normal": {
        "name": "掠影刃",
        "mult": 1,
        "target": "enemy",
        "desc": "对单体造成100%攻击伤害"
      },
      "skill": {
        "name": "双萤逐夜",
        "mult": 4.3,
        "target": "enemy",
        "hits": 2,
        "desc": "对单体造成两段215%攻击伤害"
      }
    },
    "desc": "巡行夜巷的双刃斥候，以萤火为暗号，在敌人察觉前结束交锋。",
    "battleImg": "content/cards/c23/assets/portrait-cut.webp",
    "fxColor": "#8a93a8"
  },
  {
    "id": "c24",
    "name": "雨棠",
    "rarity": "SR",
    "elem": "water",
    "role": "tank",
    "atkType": "melee",
    "img": "content/cards/c24/assets/portrait.webp",
    "base": {
      "hp": 1740,
      "atk": 160,
      "def": 130,
      "spd": 96,
      "cr": 5,
      "cd": 150,
      "ehr": 0,
      "res": 25
    },
    "matBonus": 0,
    "dupGain": 15,
    "skills": {
      "normal": {
        "name": "伞骨横击",
        "mult": 0.9,
        "target": "enemy",
        "desc": "对单体造成90%攻击伤害"
      },
      "skill": {
        "name": "听雨成垒",
        "mult": 0,
        "target": "team",
        "shieldMult": 1.5,
        "taunt": true,
        "selfDefUp": 20,
        "desc": "全队获得150%攻击的护盾，自身嘲讽并提升20%防御1回合"
      }
    },
    "desc": "雨街守伞人，撑开的伞面既能挡住骤雨，也能为伙伴隔开迎面而来的锋芒。",
    "battleImg": "content/cards/c24/assets/portrait-cut.webp",
    "fxColor": "#4a7dff"
  },
  {
    "id": "c25",
    "sources": [
      "growth"
    ],
    "name": "苏兰",
    "rarity": "SR",
    "elem": "plant",
    "role": "support",
    "atkType": "magic",
    "img": "content/cards/c25/assets/portrait.webp",
    "base": {
      "hp": 1160,
      "atk": 215,
      "def": 85,
      "spd": 108,
      "cr": 5,
      "cd": 150,
      "ehr": 30,
      "res": 20
    },
    "matBonus": 0,
    "dupGain": 15,
    "skills": {
      "normal": {
        "name": "兰香轻雾",
        "mult": 1,
        "target": "enemy",
        "desc": "对单体造成100%攻击伤害"
      },
      "skill": {
        "name": "蚀甲幽香",
        "mult": 2.4,
        "target": "enemy",
        "defBreak": 35,
        "defBreakTurns": 2,
        "desc": "对单体造成240%攻击伤害，并使其防御降低35%，持续2回合"
      }
    },
    "desc": "温室调香师，能从百种花香中辨出破绽，让幽微的香气渗入敌人的护甲。",
    "battleImg": "content/cards/c25/assets/portrait-cut.webp",
    "fxColor": "#4fbf7f"
  },
  {
    "id": "c26",
    "name": "铃绡",
    "rarity": "SR",
    "elem": "water",
    "role": "support",
    "atkType": "magic",
    "img": "content/cards/c26/assets/portrait.webp",
    "base": {
      "hp": 1160,
      "atk": 200,
      "def": 82,
      "spd": 114,
      "cr": 5,
      "cd": 150,
      "ehr": 0,
      "res": 20
    },
    "matBonus": 0,
    "dupGain": 15,
    "skills": {
      "normal": {
        "name": "绡带回旋",
        "mult": 1,
        "target": "enemy",
        "desc": "对单体造成100%攻击伤害"
      },
      "skill": {
        "name": "流铃踏歌",
        "mult": 0,
        "target": "team",
        "healMult": 0.7,
        "teamAtkUp": 18,
        "buffTurns": 2,
        "desc": "治疗全队（70%攻击），全队攻击提升18%（2回合）"
      }
    },
    "desc": "水榭舞者，步摇与绡带随歌声轻摆，用清亮铃音唤回同伴的斗志。",
    "battleImg": "content/cards/c26/assets/portrait-cut.webp",
    "fxColor": "#4a7dff"
  },
  {
    "id": "c04",
    "sources": [
      "summon",
      "starter",
      "voyage",
      "growth"
    ],
    "name": "龙音",
    "rarity": "SSR",
    "elem": "fire",
    "role": "sdps",
    "atkType": "melee",
    "img": "content/cards/c04/assets/portrait.webp",
    "base": {
      "hp": 1500,
      "atk": 380,
      "def": 110,
      "spd": 116,
      "cr": 15,
      "cd": 170,
      "ehr": 0,
      "res": 20
    },
    "matBonus": 0,
    "dupGain": 25,
    "skills": {
      "normal": {
        "name": "龙枪突刺",
        "mult": 1,
        "target": "enemy",
        "desc": "对单体造成100%攻击伤害"
      },
      "skill": {
        "name": "龙焰葬送",
        "mult": 5.5,
        "target": "enemy",
        "ignoreDefPct": 20,
        "desc": "对单体造成550%攻击伤害，无视目标20%防御"
      }
    },
    "desc": "龙角的枪之主人，单体主C。",
    "battleImg": "content/cards/c04/assets/portrait-cut.webp",
    "fxColor": "#ff6e5e"
  },
  {
    "id": "c07",
    "sources": [
      "summon",
      "starter",
      "voyage",
      "growth"
    ],
    "name": "星墙",
    "rarity": "SSR",
    "elem": "star",
    "role": "tank",
    "atkType": "heavy",
    "img": "content/cards/c07/assets/portrait.webp",
    "base": {
      "hp": 2100,
      "atk": 180,
      "def": 160,
      "spd": 95,
      "cr": 5,
      "cd": 150,
      "ehr": 0,
      "res": 30
    },
    "matBonus": 0,
    "dupGain": 25,
    "skills": {
      "normal": {
        "name": "星陨冲撞",
        "mult": 0.9,
        "target": "enemy",
        "desc": "对单体造成90%攻击伤害"
      },
      "skill": {
        "name": "星座壁垒",
        "mult": 0,
        "target": "team",
        "shieldMult": 1.8,
        "taunt": true,
        "selfDefUp": 30,
        "desc": "全队获得180%攻击护盾，自身嘲讽并提升30%防御1回合"
      }
    },
    "desc": "背负星座之盾的守护者。",
    "battleImg": "content/cards/c07/assets/portrait-cut.webp",
    "fxColor": "#ffe27a"
  },
  {
    "id": "c09",
    "name": "时雨",
    "rarity": "SSR",
    "elem": "time",
    "role": "support",
    "atkType": "magic",
    "img": "content/cards/c09/assets/portrait.webp",
    "base": {
      "hp": 1300,
      "atk": 260,
      "def": 95,
      "spd": 118,
      "cr": 8,
      "cd": 150,
      "ehr": 20,
      "res": 25
    },
    "matBonus": 0,
    "dupGain": 25,
    "skills": {
      "normal": {
        "name": "时之振子",
        "mult": 1,
        "target": "enemy",
        "desc": "对单体造成100%攻击伤害"
      },
      "skill": {
        "name": "回溯指针",
        "mult": 0,
        "target": "ally",
        "extraTurn": true,
        "healMult": 1,
        "desc": "使攻击最高的队友立即再次行动，并恢复其（100%攻击）生命"
      }
    },
    "desc": "持有怀表的时之少女，让队友再动。",
    "battleImg": "content/cards/c09/assets/portrait-cut.webp",
    "fxColor": "#c9a2ff"
  },
  {
    "id": "c03",
    "sources": [
      "world"
    ],
    "name": "寻宝女王",
    "rarity": "SSR",
    "elem": "water",
    "role": "support",
    "atkType": "ranged",
    "img": "content/cards/c03/assets/portrait.webp",
    "base": {
      "hp": 1350,
      "atk": 245,
      "def": 100,
      "spd": 112,
      "cr": 10,
      "cd": 155,
      "ehr": 0,
      "res": 20
    },
    "matBonus": 0.15,
    "dupGain": 25,
    "skills": {
      "normal": {
        "name": "提灯敲击",
        "mult": 1,
        "target": "enemy",
        "desc": "对单体造成100%攻击伤害"
      },
      "skill": {
        "name": "宝藏地图",
        "mult": 0,
        "target": "team",
        "teamAtkUp": 20,
        "buffTurns": 2,
        "goldBonus": 0.3,
        "desc": "全队攻击提升20%（2回合），本次副本金币掉落+30%"
      }
    },
    "desc": "探险家女王，上阵时材料掉落+15%。",
    "battleImg": "content/cards/c03/assets/portrait-cut.webp",
    "fxColor": "#6ec6ff"
  },
  {
    "id": "c15",
    "sources": [
      "world"
    ],
    "name": "月璃",
    "rarity": "SSR",
    "elem": "dark",
    "role": "adps",
    "atkType": "magic",
    "img": "content/cards/c15/assets/portrait.webp",
    "base": {
      "hp": 1340,
      "atk": 325,
      "def": 96,
      "spd": 111,
      "cr": 10,
      "cd": 165,
      "ehr": 15,
      "res": 25
    },
    "matBonus": 0,
    "dupGain": 25,
    "skills": {
      "normal": {
        "name": "月弧",
        "mult": 1,
        "target": "enemy",
        "desc": "对单体造成100%攻击伤害"
      },
      "skill": {
        "name": "胧月千灯",
        "mult": 2.6,
        "target": "allEnemies",
        "slow": 20,
        "slowTurns": 2,
        "desc": "对全体造成260%攻击伤害，并使目标速度降低20%，持续2回合"
      }
    },
    "desc": "渡月伞姬，在河灯亮起时踏过古桥。伞影中的月色，能让追逐者忘记脚步。",
    "battleImg": "content/cards/c15/assets/portrait-cut.webp",
    "fxColor": "#8a93a8"
  },
  {
    "id": "c16",
    "name": "织星",
    "rarity": "SSR",
    "elem": "star",
    "role": "support",
    "atkType": "magic",
    "img": "content/cards/c16/assets/portrait.webp",
    "base": {
      "hp": 1380,
      "atk": 255,
      "def": 102,
      "spd": 113,
      "cr": 5,
      "cd": 150,
      "ehr": 0,
      "res": 30
    },
    "matBonus": 0,
    "dupGain": 25,
    "skills": {
      "normal": {
        "name": "蝶线",
        "mult": 1,
        "target": "enemy",
        "desc": "对单体造成100%攻击伤害"
      },
      "skill": {
        "name": "天幕织梦",
        "mult": 0,
        "target": "team",
        "shieldMult": 1.2,
        "teamAtkUp": 25,
        "buffTurns": 2,
        "desc": "全队获得120%攻击的护盾，攻击提升25%（2回合）"
      }
    },
    "desc": "星蝶偶师，在旧观星台修补木偶。她牵动细线，让失落的星光重新照向同伴。",
    "battleImg": "content/cards/c16/assets/portrait-cut.webp",
    "fxColor": "#9a6bff"
  },
  {
    "id": "c27",
    "sources": [
      "world"
    ],
    "name": "雪翎",
    "rarity": "SSR",
    "elem": "star",
    "role": "sdps",
    "atkType": "ranged",
    "img": "content/cards/c27/assets/portrait.webp",
    "base": {
      "hp": 1360,
      "atk": 350,
      "def": 95,
      "spd": 121,
      "cr": 18,
      "cd": 170,
      "ehr": 0,
      "res": 20
    },
    "matBonus": 0,
    "dupGain": 25,
    "skills": {
      "normal": {
        "name": "白羽矢",
        "mult": 1,
        "target": "enemy",
        "desc": "对单体造成100%攻击伤害"
      },
      "skill": {
        "name": "破晓穿云",
        "mult": 4.8,
        "target": "enemy",
        "ignoreDefPct": 30,
        "desc": "对单体造成480%攻击伤害，无视目标30%防御"
      }
    },
    "desc": "立于雪岭高台的白羽弓手，将黎明收进弓弦，让穿云之箭直抵敌阵要害。",
    "battleImg": "content/cards/c27/assets/portrait-cut.webp",
    "fxColor": "#9a6bff"
  },
  {
    "id": "c28",
    "sources": [
      "summon",
      "starter",
      "growth"
    ],
    "name": "海璃",
    "rarity": "SSR",
    "elem": "water",
    "role": "heal",
    "atkType": "magic",
    "img": "content/cards/c28/assets/portrait.webp",
    "base": {
      "hp": 1550,
      "atk": 270,
      "def": 108,
      "spd": 103,
      "cr": 5,
      "cd": 150,
      "ehr": 0,
      "res": 35
    },
    "matBonus": 0,
    "dupGain": 25,
    "skills": {
      "normal": {
        "name": "潮珠微光",
        "mult": 0.9,
        "target": "enemy",
        "desc": "对单体造成90%攻击伤害"
      },
      "skill": {
        "name": "沧海安魂",
        "mult": 0,
        "target": "team",
        "healMult": 2.2,
        "shieldMult": 0.6,
        "desc": "治疗全队（220%攻击），并给予相当于60%攻击的护盾"
      }
    },
    "desc": "聆听海潮的疗愈者，在夜色中吟唱归航的曲调，以温柔潮光守住全队的生命。",
    "battleImg": "content/cards/c28/assets/portrait-cut.webp",
    "fxColor": "#4a7dff"
  },
  {
    "id": "xs039",
    "name": "解弦川",
    "rarity": "SSR",
    "statProfile": "atlas-initial",
    "initialStars": 5,
    "elem": "star",
    "role": "support",
    "atkType": "magic",
    "fxColor": "#ddbf79",
    "desc": "解弦川是悬天台修吊床的匠人，睡在平地上反倒总梦见自己下坠。他靠对调吊屋的位置替富客换窗景赚过第一桶金，下面的教书人从此教不了早起的孩子看日出，教师的哑童却仍常睡在他铺里。黎梭音要搭一台高空戏，图上落下的人从此得欠她一世。解弦川收了预支却不肯动工，图和欠据至今都留着。阮冬眠曾替他的运索队守住最后一个伤员的位置。",
    "base": {
      "hp": 736,
      "atk": 46,
      "def": 59,
      "spd": 69,
      "spAtk": 78,
      "spDef": 87,
      "cr": 5,
      "cd": 150,
      "ehr": 0,
      "res": 0
    },
    "skills": {
      "normal": {
        "name": "解弦川·普攻",
        "desc": "造成 100% 特攻 特殊伤害，获得 25 怒气。",
        "mult": 1,
        "target": "enemy",
        "rageGain": 25
      },
      "skill": {
        "name": "对榫换位",
        "desc": "【共用费用】消耗1亮榫点，冷却3次行动；无点数时本技能不可用，自动模式按正常优先级改用必杀或普攻。出手前选择一种对榫，不能同时获得两组收益。\n【救援】交换生命比例最低与当前生命最高的两名不同友方格号，前者恢复150%特攻，并净化1个减益，优先硬控制、混乱、重伤；只有一名友方时只治疗净化。已锁定的攻击不改目标，捆绑者不能被交换但仍可接受治疗。\n【切开】对敌方当前攻击最高者造成160%特攻，植入1枚脉冲，倒计时为其接下来2次行动开始；每次在周期伤害后减1，归零时爆破。每人最多2枚、每名解弦川最多4枚，超限只造成切开伤害，不刷新旧脉冲。\n【自动选择】有友方被硬控／混乱，或生命比例不高于45%时救援，否则切开；并列均取最小格号。",
        "mult": 1.6,
        "target": "joinery",
        "healMult": 1.5,
        "cooldown": 3
      },
      "xs039-skill-2": {
        "name": "空壳留震·三榫共鸣",
        "desc": "【穿刺】消耗100怒气，对常规主目标所在列各造成210%特攻，无视25%特防；主目标已有捆绑或重压时只对其追加40%特攻。伤害与追加共同受一次技能无效阻挡。\n【展开领域】出手前可再支付2亮榫点，建立持续2个大回合的三榫弦场；从本次施放所在回合计时，在第2个回合结束时消失。同名领域只刷新，不叠加，不返还点数；无点数仍可仅使用穿刺。自动模式默认保留1点供衣角榫印，故只有3点时开启领域。\n【领域的代价】领域中解弦川的伤害减免降低10个百分点，最低为0；普通护盾仍有效。解弦川被异能压制或阵亡时领域立即结束；硬控制不删除领域，但会阻止他行动后的调度。领域不延长硬控制，不增加额外行动。",
        "mult": 2.1,
        "target": "column",
        "ignoreSpDef": 0.25
      }
    },
    "img": "content/cards/xs039/assets/portrait.webp",
    "battleImg": "content/cards/xs039/assets/portrait-cut.webp",
    "combatStatus": "configured",
    "art": {
      "status": "delivered",
      "standard": "starfrost-art-v1",
      "note": "用户交付解弦川原创卡面与全身战斗立绘，均为1024×1536；WebP转换保留原始Alpha。四张1254×1254特效图已登记并按可见主体适配；初始5星战斗机制与主动演出已接入。未宣称已完成统一脚底落点验收。",
      "appearanceVersion": 2,
      "palette": "灰绿与暖茶棕为主，乌黑银白发色、骨白和哑铜点缀",
      "importedAt": "2026-11-28",
      "deliverySource": "user-provided",
      "assets": {
        "portrait": {
          "sourceFile": "gpt-image-2.6_消耗1积分_【主题配色】 灰绿与暖茶棕为主，乌黑银白发色、骨_1795854099098-y0gkpwh7cld_01.png",
          "sourceSha256": "b83225070c1cc8038fbaa0c9a3cf597648c3c860e1c28401704a72d4ebe2c095",
          "file": "portrait.webp",
          "width": 1024,
          "height": 1536,
          "bytes": 405412,
          "alpha": [
            255,
            255
          ]
        },
        "battle": {
          "sourceFile": "gpt-image-2.6_消耗1积分_【主题配色】 灰绿与暖茶棕为主，乌黑银白发色、骨_1795855001838-mdnpx8fcmj_01.png",
          "sourceSha256": "c1812b52dffaccead8796ad90f07cec30cac26fb41d49336da872195e0011fe8",
          "file": "portrait-cut.webp",
          "width": 1024,
          "height": 1536,
          "bytes": 226380,
          "alpha": [
            0,
            254
          ]
        }
      }
    },
    "atlas": {
      "id": "xs039",
      "number": 39,
      "name": "解弦川",
      "title": "对榫匠",
      "rarity": "SSR",
      "role": "support",
      "roleLabel": "支援",
      "element": "star",
      "elementLabel": "星",
      "color": "#ddbf79",
      "group": "旧火与潮路",
      "faction": "悬天台吊床街",
      "origin": "手艺（吊床匠）＋器物（三枚骨亮榫）",
      "address": "钉址·悬天台吊床街（吊屋本身属驮址）",
      "biography": "解弦川是悬天台修吊床的匠人，睡在平地上反倒总梦见自己下坠。他靠对调吊屋的位置替富客换窗景赚过第一桶金，下面的教书人从此教不了早起的孩子看日出，教师的哑童却仍常睡在他铺里。黎梭音要搭一台高空戏，图上落下的人从此得欠她一世。解弦川收了预支却不肯动工，图和欠据至今都留着。阮冬眠曾替他的运索队守住最后一个伤员的位置。",
      "appearance": "解弦川是一位清隽利落的对榫少女，乌黑齐肩发在耳前留着一绺银白，半束发尾以短皮绳扎起，冷暖分明得像新木边的一线刨痕。她的灰金眼睛微微上挑，眉细而平，淡色唇边有一点专注时才显出的倔强。米白高领衬衣外穿灰绿皮坎肩，茶色腰封贴着三只骨白榫袋，深色裙裤与交叉绑腿收住轻巧身形。她把一枚光滑骨榫夹在修长指间，另一手握细绳尺，垂眼时银白发丝恰好拂过指背；手上动作毫不拖沓，抬眼却先看同伴是否站稳，锋利里藏着周全。",
      "event": "工头卷走全队工资逃向吊桥，桥另一头正好裂开，哑童还睡在悬空的床里。解弦川只剩一榫，钉工头脚下能截下钱，钉床脚则要拿自己换孩子。他换了。孩子站到完整的走道，工头带着铁匣逃了，他摔到下层台阶。断层间临时只通一只信篮，他留在下层修路。信篮里既有孩子画的新吊床，也有整队匠人追问欠薪的信，他没拿前者去挡后者。",
      "abilities": [
        {
          "name": "对榫换位",
          "description": "他从腰里取一枚亮榫，把近处两处还托着东西的承重点对准钉下，两边绳影一晃，床上的人能与另一处的空筐互换，筐照样有分量。他不能指着远处一块空地送人过去，也不能换走被牢牢捆死的一方，承点已塌，更没有另一头可接。"
        },
        {
          "name": "空壳留震",
          "description": "亮榫还可以往硬壳里送。他指尖压下去，壳面先微微一颤，两次动作过去，真正的震响才在送入时的原处由内往外翻开。已留在壳里等回响的那一枚，不能同时拿去把吊床上的人换回来。"
        },
        {
          "name": "三榫调弦",
          "description": "他拨开亮榫之间的细弦，所顾始终至多三榫。每次拉紧只能择其一：让最早一道留震提前发作，或替同伴把卡在半道的动作理顺。他看似没有一根弦乱跳，其实心神全摊在各个受力端，此时有人来袭，他本人最难照应。"
        },
        {
          "name": "衣角接坠",
          "description": "有人可能坠下，他把最后一枚亮榫预先藏在那人的衣角。致命一坠到来前，衣角猛被提住，能替对方接走这一下，另一头却得有地方实实在在承力，他手头也得真有余榫。一处都没预留，人跌进谷底，再拍衣角也叫不活。"
        }
      ],
      "related": "黎梭音、阮冬眠、哑童",
      "skills": [
        {
          "id": "xs039-skill-1",
          "name": "对榫换位",
          "type": "小技能",
          "target": "两名存活友方／当前攻击最高敌人",
          "text": "【共用费用】消耗1亮榫点，冷却3次行动；无点数时本技能不可用，自动模式按正常优先级改用必杀或普攻。出手前选择一种对榫，不能同时获得两组收益。\n【救援】交换生命比例最低与当前生命最高的两名不同友方格号，前者恢复150%特攻，并净化1个减益，优先硬控制、混乱、重伤；只有一名友方时只治疗净化。已锁定的攻击不改目标，捆绑者不能被交换但仍可接受治疗。\n【切开】对敌方当前攻击最高者造成160%特攻，植入1枚脉冲，倒计时为其接下来2次行动开始；每次在周期伤害后减1，归零时爆破。每人最多2枚、每名解弦川最多4枚，超限只造成切开伤害，不刷新旧脉冲。\n【自动选择】有友方被硬控／混乱，或生命比例不高于45%时救援，否则切开；并列均取最小格号。",
          "cooldown": 3,
          "rage": 0,
          "upgrade": "对榫护角：仅救援分支实际成功换位后，被救援者在下次本人行动前受到的第一段直接伤害降低5%。每大回合最多触发一次；治疗净化但未换位时不触发，不返还亮榫点。",
          "visual": "一枚骨亮榫钉入承重点，两块绳影先交换座位，随后才是人影；选择留震时，榫光藏入壳内，第二次动作后才从内里绽开。",
          "tags": [
            "异能",
            "器物"
          ],
          "narrativeAbility": "对榫换位",
          "enhancementStatus": "proposal"
        },
        {
          "id": "xs039-skill-2",
          "name": "空壳留震·三榫共鸣",
          "type": "必杀",
          "target": "敌方一列／三榫弦场",
          "text": "【穿刺】消耗100怒气，对常规主目标所在列各造成210%特攻，无视25%特防；主目标已有捆绑或重压时只对其追加40%特攻。伤害与追加共同受一次技能无效阻挡。\n【展开领域】出手前可再支付2亮榫点，建立持续2个大回合的三榫弦场；从本次施放所在回合计时，在第2个回合结束时消失。同名领域只刷新，不叠加，不返还点数；无点数仍可仅使用穿刺。自动模式默认保留1点供衣角榫印，故只有3点时开启领域。\n【领域的代价】领域中解弦川的伤害减免降低10个百分点，最低为0；普通护盾仍有效。解弦川被异能压制或阵亡时领域立即结束；硬控制不删除领域，但会阻止他行动后的调度。领域不延长硬控制，不增加额外行动。",
          "cooldown": 0,
          "rage": 100,
          "upgrade": "",
          "visual": "三枚骨亮榫各燃起一颗苍青星点，细弦穿过目标一列，沿壳内同时震开；额外付出榫点才会展开完整调弦场，持弦者自己的影子此时最薄。",
          "tags": [
            "异能",
            "器物"
          ],
          "narrativeAbility": "空壳留震"
        },
        {
          "id": "xs039-skill-3",
          "name": "三榫调弦",
          "type": "被动",
          "target": "脉冲目标／双方小技能冷却",
          "text": "【亮榫点】入场2点，上限3；本人完成普攻后恢复1点，每大回合最多1点。追击与跳过行动不补点；异能压制时不能生成或使用点数，已有点数保留。\n【延迟脉冲】倒计时归零或携带者阵亡时，原生脉冲对该格仍存活的原目标造成100%特攻，并向同排左右邻格各溅射30%特攻；使用植入时特攻快照。随后向一个没有脉冲的存活敌人植入衍生脉冲，优先生命比例最低者，最多1次裂变。衍生脉冲倒计时2次行动、伤害70%特攻、无溅射且不再裂变。全部受每名解弦川4枚上限约束，净化移除不爆破；阵亡者不重复承伤。解弦川被压制或阵亡后已有脉冲仍计时爆破，但不再裂变。\n【弦场调度】领域存在且解弦川未被硬控时，每次实际行动结束，为另一名小技能剩余冷却最长的友方减少1次冷却，同时令当前攻击最高敌人小技能冷却增加1次，范围0—5；无敌阻止敌方冷却增加，瞬时冷却变化不能被净化倒退。再推进最早植入的一枚脉冲1拍。每大回合整组只触发1次，推进引发的爆破也不能再次调度；多枚同拍按植入顺序处理。",
          "cooldown": 0,
          "rage": 0,
          "upgrade": "",
          "visual": "细弦标出每一枚留震还剩的节拍，最早植入的一点先亮；调弦只在允许的那一拍拨动一次，熄灭的榫光绝不会凭空补回。",
          "tags": [
            "异能",
            "延时"
          ],
          "narrativeAbility": "三榫调弦"
        },
        {
          "id": "xs039-skill-4",
          "name": "衣角接坠",
          "type": "被动",
          "target": "自身与攻击最高的另一名友方",
          "text": "【预存衣角榫印】入场给自己及攻击最高的另一名友方各1枚衣角榫印，记录当时生命与怒气；同一人全场只能获得一次，不可驱散。衣角榫印不是复活，目标已消耗过任何致命救助资格时不再生效。\n【救助费用】衣角榫印持有者遭致命伤时，若解弦川存活、未被异能压制且有1亮榫点，消耗1点与衣角榫印，阻止该次致命伤，把生命设为记录生命的50%（不超过其当前最大生命40%），怒气取当前与记录的较大值且不超过100。净化2个减益，优先硬控制和混乱，并减伤20%持续1次本人行动。该生命设定不受重伤影响；同一多段技能后续伤害继续结算。点数不足时正常阵亡。\n【术后回访】己方每累计新获得3个可净化减益，净化当前生命比例最低友方1个减益，每大回合至多1次；刷新旧状态、多层同时添加和被免疫的状态各不多计。解弦川被硬控或异能压制时不计数不触发；解弦川阵亡后未用衣角榫印立即消失。",
          "cooldown": 0,
          "rage": 0,
          "upgrade": "",
          "visual": "预藏衣角的亮榫在致命坠落前骤然绷紧，青色承力线把尚未断绝的人接回；已经落空的衣角安静垂下，不召回死者。",
          "tags": [
            "异能",
            "致命保护"
          ],
          "narrativeAbility": "衣角接坠"
        }
      ],
      "basic": {
        "id": "xs039-basic",
        "name": "解弦川·普攻",
        "type": "普攻",
        "target": "常规单体",
        "text": "造成 100% 特攻 特殊伤害，获得 25 怒气。",
        "cooldown": 0,
        "rage": 0,
        "upgrade": "",
        "visual": "发动换位时最古怪的不是人腾空：绳下两块阴影先换了座，半眨眼以后，伸手的人才发觉掌中握到的不是床绳，而是对侧的栏杆。",
        "tags": [
          "意志"
        ],
        "narrativeAbility": "对榫换位"
      },
      "stats": {
        "hp": 736,
        "speed": 69,
        "atk": 46,
        "def": 59,
        "spAtk": 78,
        "spDef": 87
      },
      "statNote": "原独立图鉴卡：1级、初始5星、未突破六维，不含羁绊、装备和培养；此标尺与28张原星霜少女卡独立。",
      "highStars": [
        {
          "star": 8,
          "name": "衣角榫印接续",
          "text": "第3回合开始以后，第一次本人行动开始时可消耗1亮榫点，把一枚尚未触发的衣角榫印转给从未持有衣角榫印、未用过救助资格的当前攻击最高友方，排除解弦川。默认保留原衣角榫印，玩家战前可选转移；旧持有者失去衣角榫印，新持有者按转移时生命与怒气记录，每场最多一次，不增殖衣角榫印。"
        },
        {
          "star": 11,
          "name": "对榫连锁",
          "text": "一次衣角榫印成功救助后，下一次对榫换位可改为敌方交换：支付原技能1亮榫点，交换敌方最高攻击者与其同列另一存活本体，各减少10怒气；不同时治疗或植入脉冲，每场最多一次。目标有捆绑、被吸引或固定站位时不能交换；不可交换的首领仅削1格韧性，正常控制保护可阻挡。自动模式仍按基础救援／切开，不主动花这次机会。"
        }
      ],
      "bonds": [
        {
          "targetId": "xs029",
          "targetIds": [
            "xs029"
          ],
          "name": "解弦川 × 贺紬",
          "type": "战术共鸣",
          "text": "沿用对应卡的收藏加成；曾经获取本卡与羁绊伙伴即永久激活；无需同时上阵，之后分解、下阵或阵亡均不失效。 此战术关系不自动等于本作中的血亲、恋情或同场经历。",
          "bonus": "生命 +17%",
          "percent": 17,
          "stat": "hp"
        },
        {
          "targetId": "xs141",
          "targetIds": [
            "xs141"
          ],
          "name": "解弦川 × 黎梭音",
          "type": "战术共鸣",
          "text": "沿用对应卡的收藏加成；曾经获取本卡与羁绊伙伴即永久激活；无需同时上阵，之后分解、下阵或阵亡均不失效。 此战术关系不自动等于本作中的血亲、恋情或同场经历。",
          "bonus": "特攻 +14%",
          "percent": 14,
          "stat": "spatk"
        },
        {
          "targetId": "xs042",
          "targetIds": [
            "xs042"
          ],
          "name": "解弦川 × 祁炊娘",
          "type": "战术共鸣",
          "text": "沿用对应卡的收藏加成；曾经获取本卡与羁绊伙伴即永久激活；无需同时上阵，之后分解、下阵或阵亡均不失效。 此战术关系不自动等于本作中的血亲、恋情或同场经历。",
          "bonus": "特防 +14%",
          "percent": 14,
          "stat": "spdef"
        },
        {
          "targetId": "xs138",
          "targetIds": [
            "xs138"
          ],
          "name": "解弦川 × 晏向隅",
          "type": "战术共鸣",
          "text": "沿用对应卡的收藏加成；曾经获取本卡与羁绊伙伴即永久激活；无需同时上阵，之后分解、下阵或阵亡均不失效。 此战术关系不自动等于本作中的血亲、恋情或同场经历。",
          "bonus": "速度 +6%",
          "percent": 6,
          "stat": "speed"
        },
        {
          "targetId": "xs142",
          "targetIds": [
            "xs142"
          ],
          "name": "解弦川 × 叶刻湾",
          "type": "战术共鸣",
          "text": "沿用对应卡的收藏加成；曾经获取本卡与羁绊伙伴即永久激活；无需同时上阵，之后分解、下阵或阵亡均不失效。 此战术关系不自动等于本作中的血亲、恋情或同场经历。",
          "bonus": "防御 +11%",
          "percent": 11,
          "stat": "def"
        }
      ],
      "storyLinks": [
        {
          "targetId": "xs133",
          "name": "阮冬眠",
          "type": "生平原句",
          "text": "阮冬眠曾替他的运索队守住最后一个伤员的位置。"
        },
        {
          "targetId": "xs141",
          "name": "黎梭音",
          "type": "生平原句",
          "text": "黎梭音要搭一台高空戏，图上落下的人从此得欠她一世。"
        }
      ],
      "acquisition": [
        {
          "name": "强者世界",
          "text": "仅在强者世界轮换卡池中获得同卡碎片；集齐80枚后在图鉴主动合成。"
        }
      ],
      "source": {
        "originalId": "law",
        "originalName": "特拉法尔加·罗",
        "roster": "独立图鉴对应卡",
        "storyFile": "02-旧火与潮路.md",
        "storyAnchor": "039-解弦川对榫匠",
        "image": "assets/characters/law.jpg",
        "face": "assets/characters/law-face.jpg",
        "statBasis": "atlas.design.initialStats",
        "originalSkillNames": [
          "手术空间·屠宰场",
          "麻醉空间·穿刺波动",
          "手术空间·脉冲与调度",
          "不老手术·刻印救助"
        ],
        "originalSkillCount": 4,
        "originalCounter": "沉默延误屠宰场与麻醉空间；净化脉冲时不引爆；逼罗花完手术点后再集火刻印目标。果实压制暂停点数与救助、关闭领域，击败罗会清掉未用刻印。多段后续伤害仍可击败刚获救者。"
      },
      "counter": "他不能指着远处一块空地送人过去，也不能换走被牢牢捆死的一方，承点已塌，更没有另一头可接。已留在壳里等回响的那一枚，不能同时拿去把吊床上的人换回来。",
      "mechanic": {
        "title": "承重弦场 · 对榫台上的整片战场",
        "complexity": "专精",
        "resource": "亮榫点：初始2／上限3；普攻补1。脉冲：每人最多2枚，每名解弦川最多4枚。",
        "summary": "救援和进攻共用亮榫点；保留点数能保住衣角榫印，投入领域则抢走对手的技能窗口。",
        "loop": [
          "普攻补点，提前保留衣角榫印所需的1点。",
          "对榫换位选择换位救援，或切开敌人埋下两拍脉冲。",
          "穿壳弦场用2点开启三榫弦场，调度冷却、加速脉冲。",
          "领域结束后回到普攻蓄点，保护与爆发不能持续全开。"
        ],
        "synergy": "屏障与钻石先替解弦川争取行动；捆绑／重压延缓敌方出手，让脉冲与穿壳弦场在同一窗口兑现。换位也能为乐斜明重新安排下一次对角。",
        "counterplay": "沉默阻止开弦场；净化脉冲阻断传播；集中攻击解弦川或逼其提前花掉亮榫点，让衣角榫印失去救助费用。",
        "highlights": [
          "救援／切开双分支",
          "可净化的延迟脉冲",
          "冷却调度与有限衣角榫印"
        ]
      },
      "limitations": [
        {
          "name": "对榫换位",
          "text": "他不能指着远处一块空地送人过去，也不能换走被牢牢捆死的一方，承点已塌，更没有另一头可接。"
        },
        {
          "name": "空壳留震",
          "text": "已留在壳里等回响的那一枚，不能同时拿去把吊床上的人换回来。"
        },
        {
          "name": "三榫调弦",
          "text": "他看似没有一根弦乱跳，其实心神全摊在各个受力端，此时有人来袭，他本人最难照应。"
        },
        {
          "name": "衣角接坠",
          "text": "一处都没预留，人跌进谷底，再拍衣角也叫不活。"
        }
      ],
      "designNote": "技能数值继承对应原卡；星霜演绎为施放表现。标为「融合优化」的条目是图鉴设计提案，尚未接入战斗执行。",
      "gender": "女",
      "visualAge": "青年少女",
      "palette": "灰绿与暖茶棕为主，乌黑银白发色、骨白和哑铜点缀",
      "implementationNote": "当前实装初始5星的普攻、对榫换位、三榫共鸣与两项被动；星霜演化和8／11星专属规则仍为设计预览。"
    },
    "combatProfile": "joinery",
    "matBonus": 0,
    "sources": [
      "world"
    ],
    "dupGain": 80
  }
];
// END GENERATED GIRLS

function cardSources(girl) { return girl.sources || ["summon"]; }
function cardSourceDescription(girl) {
  const labels = {summon:"常驻召唤",world:girl.rarity === "SSR" ? "强者世界轮值碎片（召唤、积分箱、排名奖）" : "强者世界 R 整卡",voyage:"副本碎片",growth:"成长许愿与周常自选碎片",starter:"首核心自选",crew:"初始队伍"};
  return cardSources(girl).map(key => labels[key]).join("、");
}

/* ---------- 突破材料 ----------
 * 三档材料按关卡产出，升星时与碎片一起消耗（按少女稀有度对应档位）。
 */
const MATS = {
  mat_r:   { name: "微光结晶", color: "#93a6c2", from: "林中圣所 · 溪谷哨站 · 夜潮栈道", use: "R 升星" },
  mat_sr:  { name: "元素结晶", color: "#5c8bff", from: "冰晶殿堂 · 天空遗迹 · 潮汐神殿 · 逐星长阶", use: "SR 升星" },
  mat_ssr: { name: "星屑",     color: "#ffcf6e", from: "熔岩要塞 · 深渊回廊 · 月影王庭 · 观星禁域 · 星穹圣座", use: "SSR 升星" },
  shard_dust: { name: "共鸣尘", color: "#bd82e8", from: "旧版库存", use: "历史材料，用途以当前服务器规则为准" },
};
const MAT_BY_RARITY = { R: "mat_r", SR: "mat_sr", SSR: "mat_ssr" };
/* 升到下一星所需材料数（按稀有度分档） */
const MAT_STAR_COST = { R: 20, SR: 30, SSR: 50 };

const CULTIVATION_ITEMS = {
  forge_ore: { name: "星锻矿", from: "主线、裂隙通关及碎片回收", use: "专武强化、铭刻与宝石芯片养成；可兑换宝石券或芯片券" },
  weapon_core: { name: "觉醒棱芯", from: "通关有首领的主线或裂隙", use: "专武觉醒" },
  training_manual: { name: "历练手记", from: "通关有首领的主线或裂隙", use: "单人经验" },
};
const ARTIFACT_TICKETS = {
  gem_ticket: { name: "宝石召唤券", color: "#b16a74", from: "主线首领、棱镜打捞湾、50分日常宝箱或星锻矿兑换等", use: "宝石召唤池 · 一券抽一颗" },
  chip_ticket: { name: "芯片召唤券", color: "#557ca1", from: "裂隙、棱镜打捞湾、75分日常宝箱或星锻矿兑换等", use: "芯片召唤池 · 一券抽一枚" },
};
const CULTIVATION_STATS = { hp: "生命", atk: "攻击", def: "防御", spd: "速度" };

/* ---------- 共鸣宝石 ---------- */
const GEMS = {
  ruby:     { name: "赤曜石", color: "#ef7373", effect: "攻击 +5%" },
  jade:     { name: "生命翡翠", color: "#67c98b", effect: "生命 +5%" },
  sapphire: { name: "坚壁蓝宝石", color: "#69a8f5", effect: "防御 +5%" },
  wind:     { name: "风灵晶", color: "#6dd6d0", effect: "速度 +3" },
  amethyst: { name: "紫晶", color: "#b78af2", effect: "暴击 +3%" },
};
const GEM_SOCKET_COST = [
  { gold: 200, mat: 5 },
  { gold: 600, mat: 12 },
  { gold: 1500, mat: 25 },
];

const TALENTS = {
  power:   { name: "破军", effect: "每阶攻击 +8%、技能伤害 +5%，生命 -4%", color: "#e77c70", style: "爆发流", tradeoff: "伤害高，但更怕反弹与持续伤害" },
  guardian:{ name: "磐石", effect: "每阶生命 +5%、防御 +10%；受到生命伤害时按防御反击，攻击 -4%", color: "#66b99a", style: "防反流", tradeoff: "越能站住越有输出，主动攻击较弱" },
  swift:   { name: "追风", effect: "每阶速度 +4、暴击 +3%、普攻怒气 +15，抵抗 -2", color: "#6aa9dd", style: "先手怒技流", tradeoff: "满星配时序灵印，可更频繁释放怒气技能" },
  siphon:  { name: "噬血", effect: "每阶攻击 +4%、生命伤害吸血 +5%，防御 -3%", color: "#c26d79", style: "吸血流", tradeoff: "打护盾不吸血，侵蚀会压低恢复" },
  blight:  { name: "蚀骨", effect: "攻击有概率附加两回合侵蚀；每阶最大生命伤害 +1%、命中 +4，攻击 -6%", color: "#8aa96d", style: "侵蚀流", tradeoff: "持续伤害无视护盾、不叠层，伤害受自身攻击上限约束" },
  oracle:  { name: "灵契", effect: "每阶生命 +4%、抵抗 +4、治疗和护盾效果 +15%，攻击 -2%", color: "#82b7c7", style: "护盾续航流", tradeoff: "保护和恢复更强，直接伤害略低" },
};

const SIGIL_FAMILIES = {
  ember:  { name: "烈焰灵印", color: "#dc7865", effect: "技能伤害 +12%", mat: "炎心精华", from: "灼焰地宫", element: "fire" },
  tide:   { name: "潮汐灵印", color: "#6dadd5", effect: "治疗与护盾效果 +15%", mat: "潮汐精华", from: "潮壁圣殿", element: "water" },
  grove:  { name: "森息灵印", color: "#85b679", effect: "攻击有50%概率附加两回合侵蚀，伤害受自身攻击约束", mat: "森息精华", from: "复苏秘林", element: "plant" },
  astral: { name: "星辉灵印", color: "#cfb567", effect: "暴击伤害 +20%", mat: "星辉精华", from: "折光星殿", element: "star" },
  umbra:  { name: "夜蚀灵印", color: "#b28198", effect: "造成生命伤害时吸血8%", mat: "夜蚀精华", from: "毒雾墓城", element: "dark" },
  chrono: { name: "时序灵印", color: "#75b9ae", effect: "初始怒气 +45、普攻额外怒气 +8", mat: "时序精华", from: "封时钟塔", element: "time" },
};
const SIGIL_STATS = {
  atk: { name: "攻击", unit: "%" }, hp: { name: "生命", unit: "%" }, def: { name: "防御", unit: "%" },
  spd: { name: "速度", unit: "" }, cr: { name: "暴击", unit: "%" }, cd: { name: "暴伤", unit: "%" },
  ehr: { name: "效果命中", unit: "" }, res: { name: "效果抵抗", unit: "" },
};
const SIGIL_QUALITY = { 1: "普通", 2: "稀有", 3: "史诗" };
const SIGIL_DUST = { 1: 2, 2: 4, 3: 8 };
for (const [id, family] of Object.entries(SIGIL_FAMILIES)) {
  MATS["essence_" + id] = { name: family.mat, color: family.color, from: family.from + " · 对应噩梦裂隙", use: "40精华 + 15灵印粉尘 + 800金币打造本系史诗灵印" };
}
MATS.sigil_dust = { name: "灵印粉尘", color: "#bca27b", from: "分解未装备的灵印", use: "用于打造史诗灵印" };
const DUNGEON_MECHANICS = {
  fury: "灼怒 · 回合越久敌人攻击越高", barrier: "结界 · 初始护盾，每三回合补盾",
  regrowth: "复苏 · 侵蚀可以压制恢复", reflection: "映伤 · 反弹12%生命伤害",
  venom: "毒雾 · 侵蚀会压低治疗与吸血", silence: "封怒 · 偶数回合削减怒气",
};

const GACHA_RUN_MODES = {
  expedition: { id: "expedition", name: "星穹远征", type: "周常爬塔", waves: 1, img: "content/bosses/abyss-lord/assets/portrait.webp", starRound: 12, cost: 13 },
  weekly_boss: { id: "weekly_boss", name: "每周元素首领", type: "周常首领", waves: 1, img: "content/bosses/abyss-lord/assets/portrait.webp", starRound: 12, cost: 20 },
};

/* ---------- 章节分组 ----------
 * 关卡节点在副本页按章节展示；章节本身不加额外解锁条件，
 * 仍沿用每个关卡的 unlockStage 串联。
 */
const CHAPTERS = [
  { id: "ch1", name: "雾森边境", sub: "第一章", stages: ["s1", "s5", "s2"],
    note: "从林中圣所出发，穿过溪谷哨站，抵达冰晶殿堂。" },
  { id: "ch2", name: "火与遗迹", sub: "第二章", stages: ["s4", "s3", "s6"],
    note: "天空遗迹通向熔岩要塞，尽头是深渊回廊的主宰。" },
  { id: "ch3", name: "潮汐与月影", sub: "第三章", stages: ["s7", "s8", "s9"],
    note: "离开深渊，循着海灯走过夜潮栈道，在月影王庭迎战守门者。" },
  { id: "ch4", name: "星穹终境", sub: "第四章", stages: ["s10", "s11", "s12"],
    note: "沿逐星长阶登上云端，穿越观星禁域，挑战星穹圣座的最终试炼。" },
{
  "id": "ch5",
  "name": "六系裂隙",
  "sub": "第五章",
  "stages": [
    "r1",
    "r2",
    "r3",
    "r4",
    "r5",
    "r6"
  ],
  "note": "六种敌人机制与六套专属掉落，按你的流派选择目标。通关深渊回廊后开放。"
},

{
  "id": "ch6",
  "name": "噩梦回响",
  "sub": "第六章",
  "stages": [
    "n1",
    "n2",
    "n3",
    "n4",
    "n5",
    "n6"
  ],
  "note": "逐个通关对应普通裂隙后开启。高阶灵印与更多精华，需要有针对性的阵容。"
},
];

/* ---------- 24 个副本 ----------
 * cost: 单次出战消耗体力
 * enemyLv: 敌人等级（决定防御与生命成长）
 * tokenRange: [最小, 最大] 单次掉落范围（没有每日上限）
 * unlockStage: 解锁条件（通关某副本序号）
 * starRound: 三星中的「速通」回合上限（超过只能拿两星）
 */
// BEGIN GENERATED STAGES — run python scripts/sync-catalog.py
const STAGES = [
  {
    "id": "s1",
    "name": "林中圣所",
    "type": "普通本",
    "img": "assets/d04-forest.webp",
    "cost": 2,
    "enemyLv": 10,
    "waves": 1,
    "enemyCount": 3,
    "starRound": 4,
    "tokenRange": [
      1,
      1
    ],
    "gold": [
      28,
      48
    ],
    "exp": 40,
    "shardR": 0.2,
    "shardSR": 0.04,
    "shardSSR": 0.008,
    "mat": [
      2,
      4
    ],
    "matKey": "mat_r",
    "unlockStage": null,
    "desc": "新手练级地，掉落基础材料。",
    "lootFamily": "grove"
  },
  {
    "id": "s5",
    "name": "溪谷哨站",
    "type": "普通本",
    "img": "assets/d04-forest.webp",
    "cost": 3,
    "enemyLv": 18,
    "waves": 2,
    "enemyCount": 2,
    "starRound": 5,
    "tokenRange": [
      1,
      1
    ],
    "gold": [
      42,
      72
    ],
    "exp": 60,
    "shardR": 0.22,
    "shardSR": 0.06,
    "shardSSR": 0.012,
    "mat": [
      3,
      5
    ],
    "matKey": "mat_r",
    "unlockStage": "s1",
    "desc": "林间哨站，两波敌人。练级与攒碎片的稳定来源。",
    "lootFamily": "grove"
  },
  {
    "id": "s2",
    "name": "冰晶殿堂",
    "type": "精英本",
    "img": "assets/d03-ice.webp",
    "cost": 5,
    "enemyLv": 25,
    "waves": 2,
    "enemyCount": 3,
    "starRound": 7,
    "tokenRange": [
      1,
      2
    ],
    "gold": [
      70,
      120
    ],
    "exp": 90,
    "shardR": 0.25,
    "shardSR": 0.1,
    "shardSSR": 0.02,
    "mat": [
      4,
      7
    ],
    "matKey": "mat_sr",
    "unlockStage": "s5",
    "desc": "第一章终点。碎片掉率更高，产出元素结晶。",
    "lootFamily": "tide"
  },
  {
    "id": "s4",
    "name": "天空遗迹",
    "type": "活动本",
    "img": "assets/d02-sky.webp",
    "cost": 6,
    "enemyLv": 35,
    "waves": 2,
    "enemyCount": 4,
    "starRound": 7,
    "tokenRange": [
      1,
      3
    ],
    "gold": [
      84,
      144
    ],
    "exp": 120,
    "shardR": 0.28,
    "shardSR": 0.12,
    "shardSSR": 0.03,
    "mat": [
      5,
      10
    ],
    "matKey": "mat_sr",
    "unlockStage": "s2",
    "desc": "限时活动本。代币浮动范围最大。",
    "lootFamily": "astral"
  },
  {
    "id": "s3",
    "name": "熔岩要塞",
    "type": "Boss本",
    "img": "assets/d01-volcano.webp",
    "cost": 9,
    "enemyLv": 45,
    "waves": 3,
    "enemyCount": 2,
    "hasBoss": true,
    "starRound": 10,
    "tokenRange": [
      2,
      5
    ],
    "gold": [
      126,
      216
    ],
    "exp": 180,
    "shardR": 0.3,
    "shardSR": 0.16,
    "shardSSR": 0.045,
    "mat": [
      7,
      12
    ],
    "matKey": "mat_ssr",
    "unlockStage": "s4",
    "desc": "最终波为Boss战，产出星屑与高阶碎片。",
    "lootFamily": "ember"
  },
  {
    "id": "s6",
    "name": "深渊回廊",
    "type": "Boss本",
    "img": "assets/d01-volcano.webp",
    "cost": 11,
    "enemyLv": 55,
    "waves": 3,
    "enemyCount": 2,
    "hasBoss": true,
    "starRound": 11,
    "tokenRange": [
      3,
      6
    ],
    "gold": [
      154,
      264
    ],
    "exp": 220,
    "shardR": 0.32,
    "shardSR": 0.18,
    "shardSSR": 0.05,
    "mat": [
      9,
      15
    ],
    "matKey": "mat_ssr",
    "unlockStage": "s3",
    "desc": "第二章终点。三星需要压到 11 回合内，通关后开启潮汐与月影。",
    "lootFamily": "umbra"
  },
  {
    "id": "s7",
    "name": "夜潮栈道",
    "type": "精英本",
    "img": "assets/d05-tide.webp",
    "cost": 10,
    "enemyLv": 58,
    "waves": 2,
    "enemyCount": 3,
    "starRound": 8,
    "tokenRange": [
      3,
      5
    ],
    "gold": [
      140,
      240
    ],
    "exp": 230,
    "shardR": 0.34,
    "shardSR": 0.18,
    "shardSSR": 0.05,
    "mat": [
      12,
      18
    ],
    "matKey": "mat_r",
    "unlockStage": "s6",
    "desc": "海灯照亮夜潮中的栈道。两波精英守卫，适合补充微光结晶。",
    "lootFamily": "tide"
  },
  {
    "id": "s8",
    "name": "潮汐神殿",
    "type": "精英本",
    "img": "assets/d05-tide.webp",
    "cost": 12,
    "enemyLv": 62,
    "waves": 3,
    "enemyCount": 3,
    "starRound": 11,
    "tokenRange": [
      3,
      6
    ],
    "gold": [
      168,
      288
    ],
    "exp": 260,
    "shardR": 0.34,
    "shardSR": 0.2,
    "shardSSR": 0.055,
    "mat": [
      10,
      16
    ],
    "matKey": "mat_sr",
    "unlockStage": "s7",
    "desc": "潮水退去，神殿现身。连续三波守卫，考验全队的持续作战能力。",
    "lootFamily": "tide"
  },
  {
    "id": "s9",
    "name": "月影王庭",
    "type": "Boss本",
    "img": "assets/d05-tide.webp",
    "cost": 14,
    "enemyLv": 66,
    "waves": 3,
    "enemyCount": 3,
    "hasBoss": true,
    "starRound": 12,
    "tokenRange": [
      4,
      7
    ],
    "gold": [
      196,
      336
    ],
    "exp": 300,
    "shardR": 0.35,
    "shardSR": 0.22,
    "shardSSR": 0.06,
    "mat": [
      12,
      18
    ],
    "matKey": "mat_ssr",
    "unlockStage": "s8",
    "desc": "第三章终点。月色之下迎战Boss，通关后开启星穹终境。",
    "lootFamily": "umbra"
  },
  {
    "id": "s10",
    "name": "逐星长阶",
    "type": "精英本",
    "img": "assets/d06-observatory.webp",
    "cost": 14,
    "enemyLv": 70,
    "waves": 3,
    "enemyCount": 3,
    "starRound": 11,
    "tokenRange": [
      4,
      7
    ],
    "gold": [
      196,
      336
    ],
    "exp": 320,
    "shardR": 0.35,
    "shardSR": 0.23,
    "shardSSR": 0.065,
    "mat": [
      14,
      20
    ],
    "matKey": "mat_sr",
    "unlockStage": "s9",
    "desc": "踏上云端星阶，突破三波守卫。高阶元素结晶的稳定来源。",
    "lootFamily": "astral"
  },
  {
    "id": "s11",
    "name": "观星禁域",
    "type": "精英本",
    "img": "assets/d06-observatory.webp",
    "cost": 16,
    "enemyLv": 75,
    "waves": 3,
    "enemyCount": 4,
    "starRound": 13,
    "tokenRange": [
      5,
      8
    ],
    "gold": [
      224,
      384
    ],
    "exp": 350,
    "shardR": 0.36,
    "shardSR": 0.24,
    "shardSSR": 0.07,
    "mat": [
      14,
      21
    ],
    "matKey": "mat_ssr",
    "unlockStage": "s10",
    "desc": "观星台的禁制尚未散去。每波四名敌人，群体伤害与治疗尤为关键。",
    "lootFamily": "chrono"
  },
  {
    "id": "s12",
    "name": "星穹圣座",
    "type": "Boss本",
    "img": "assets/d06-observatory.webp",
    "cost": 18,
    "enemyLv": 80,
    "waves": 4,
    "enemyCount": 3,
    "hasBoss": true,
    "starRound": 15,
    "tokenRange": [
      5,
      10
    ],
    "gold": [
      252,
      432
    ],
    "exp": 400,
    "shardR": 0.38,
    "shardSR": 0.26,
    "shardSSR": 0.08,
    "mat": [
      16,
      24
    ],
    "matKey": "mat_ssr",
    "unlockStage": "s11",
    "desc": "当前最高难度。四波连续试炼，最终波迎战Boss；三星需在15回合内完成。",
    "lootFamily": "astral"
  },
  {
    "id": "r1",
    "name": "灼焰地宫",
    "type": "元素裂隙",
    "img": "assets/d01-volcano.webp",
    "cost": 12,
    "enemyLv": 84,
    "waves": 2,
    "enemyCount": 3,
    "starRound": 12,
    "exp": 280,
    "hasBoss": true,
    "tokenRange": [
      4,
      7
    ],
    "gold": [
      252,
      384
    ],
    "mat": [
      9,
      15
    ],
    "shardR": 0,
    "shardSR": 0.2,
    "shardSSR": 0.08,
    "matKey": "mat_sr",
    "unlockStage": "s6",
    "lootFamily": "ember",
    "enemyElem": "fire",
    "mechanic": "fury",
    "hpScale": 2.4,
    "atkScale": 1.25,
    "defScale": 1.15,
    "enemyRes": 15,
    "relicTier": 1,
    "relicChance": 0.45,
    "essence": [
      2,
      4
    ],
    "desc": "灼怒：敌人随回合提高攻击，拖得越久越危险；适合爆发与先手控制。 专属产出本系精华与灵印，不发放站点额度。"
  },
  {
    "id": "r2",
    "name": "潮壁圣殿",
    "type": "元素裂隙",
    "img": "assets/d05-tide.webp",
    "cost": 12,
    "enemyLv": 86,
    "waves": 2,
    "enemyCount": 3,
    "starRound": 12,
    "exp": 280,
    "hasBoss": true,
    "tokenRange": [
      4,
      7
    ],
    "gold": [
      252,
      384
    ],
    "mat": [
      9,
      15
    ],
    "shardR": 0,
    "shardSR": 0.2,
    "shardSSR": 0.08,
    "matKey": "mat_sr",
    "unlockStage": "s6",
    "lootFamily": "tide",
    "enemyElem": "water",
    "mechanic": "barrier",
    "hpScale": 2.4,
    "atkScale": 1.25,
    "defScale": 1.15,
    "enemyRes": 15,
    "relicTier": 1,
    "relicChance": 0.45,
    "essence": [
      2,
      4
    ],
    "desc": "结界：敌人自带护盾，每三回合补充护盾；侵蚀可越过护盾，吸血不会从护盾获取恢复。 专属产出本系精华与灵印，不发放站点额度。"
  },
  {
    "id": "r3",
    "name": "复苏秘林",
    "type": "元素裂隙",
    "img": "assets/d04-forest.webp",
    "cost": 12,
    "enemyLv": 88,
    "waves": 2,
    "enemyCount": 3,
    "starRound": 12,
    "exp": 280,
    "hasBoss": true,
    "tokenRange": [
      4,
      7
    ],
    "gold": [
      252,
      384
    ],
    "mat": [
      9,
      15
    ],
    "shardR": 0,
    "shardSR": 0.2,
    "shardSSR": 0.08,
    "matKey": "mat_sr",
    "unlockStage": "s6",
    "lootFamily": "grove",
    "enemyElem": "plant",
    "mechanic": "regrowth",
    "hpScale": 2.4,
    "atkScale": 1.25,
    "defScale": 1.15,
    "enemyRes": 15,
    "relicTier": 1,
    "relicChance": 0.45,
    "essence": [
      2,
      4
    ],
    "desc": "复苏：敌人每次行动恢复8%最大生命；侵蚀会把复苏压制到四分之一。 专属产出本系精华与灵印，不发放站点额度。"
  },
  {
    "id": "r4",
    "name": "折光星殿",
    "type": "元素裂隙",
    "img": "assets/d02-sky.webp",
    "cost": 12,
    "enemyLv": 90,
    "waves": 2,
    "enemyCount": 3,
    "starRound": 12,
    "exp": 280,
    "hasBoss": true,
    "tokenRange": [
      4,
      7
    ],
    "gold": [
      252,
      384
    ],
    "mat": [
      9,
      15
    ],
    "shardR": 0,
    "shardSR": 0.2,
    "shardSSR": 0.08,
    "matKey": "mat_sr",
    "unlockStage": "s6",
    "lootFamily": "astral",
    "enemyElem": "star",
    "mechanic": "reflection",
    "hpScale": 2.4,
    "atkScale": 1.25,
    "defScale": 1.15,
    "enemyRes": 15,
    "relicTier": 1,
    "relicChance": 0.45,
    "essence": [
      2,
      4
    ],
    "desc": "映伤：生命伤害的12%反弹给攻击者，堆纯攻击会更脆；需要护盾、吸血或治疗。 专属产出本系精华与灵印，不发放站点额度。"
  },
  {
    "id": "r5",
    "name": "毒雾墓城",
    "type": "元素裂隙",
    "img": "assets/d01-volcano.webp",
    "cost": 12,
    "enemyLv": 92,
    "waves": 2,
    "enemyCount": 3,
    "starRound": 12,
    "exp": 280,
    "hasBoss": true,
    "tokenRange": [
      4,
      7
    ],
    "gold": [
      252,
      384
    ],
    "mat": [
      9,
      15
    ],
    "shardR": 0,
    "shardSR": 0.2,
    "shardSSR": 0.08,
    "matKey": "mat_sr",
    "unlockStage": "s6",
    "lootFamily": "umbra",
    "enemyElem": "dark",
    "mechanic": "venom",
    "hpScale": 2.4,
    "atkScale": 1.25,
    "defScale": 1.15,
    "enemyRes": 15,
    "relicTier": 1,
    "relicChance": 0.45,
    "essence": [
      2,
      4
    ],
    "desc": "毒雾：敌人攻击附带侵蚀，侵蚀期间治疗与吸血减半；抵抗与净化有实战价值。 专属产出本系精华与灵印，不发放站点额度。"
  },
  {
    "id": "r6",
    "name": "封时钟塔",
    "type": "元素裂隙",
    "img": "assets/d06-observatory.webp",
    "cost": 12,
    "enemyLv": 94,
    "waves": 2,
    "enemyCount": 3,
    "starRound": 12,
    "exp": 280,
    "hasBoss": true,
    "tokenRange": [
      4,
      7
    ],
    "gold": [
      252,
      384
    ],
    "mat": [
      9,
      15
    ],
    "shardR": 0,
    "shardSR": 0.2,
    "shardSSR": 0.08,
    "matKey": "mat_sr",
    "unlockStage": "s6",
    "lootFamily": "chrono",
    "enemyElem": "time",
    "mechanic": "silence",
    "hpScale": 2.4,
    "atkScale": 1.25,
    "defScale": 1.15,
    "enemyRes": 15,
    "relicTier": 1,
    "relicChance": 0.45,
    "essence": [
      2,
      4
    ],
    "desc": "封怒：偶数回合敌人命中后削减15怒气；追风、时序灵印或不依赖技能的反击更合适。 专属产出本系精华与灵印，不发放站点额度。"
  },
  {
    "id": "n1",
    "name": "灼焰地宫·噩梦",
    "type": "噩梦裂隙",
    "img": "assets/d01-volcano.webp",
    "cost": 24,
    "enemyLv": 113,
    "waves": 3,
    "enemyCount": 3,
    "starRound": 18,
    "exp": 650,
    "hasBoss": true,
    "tokenRange": [
      11,
      15
    ],
    "gold": [
      672,
      960
    ],
    "mat": [
      18,
      28
    ],
    "shardR": 0,
    "shardSR": 0.28,
    "shardSSR": 0.16,
    "matKey": "mat_ssr",
    "unlockStage": "r1",
    "lootFamily": "ember",
    "enemyElem": "fire",
    "mechanic": "fury",
    "hpScale": 6.4,
    "atkScale": 2.0,
    "defScale": 1.5,
    "enemyRes": 35,
    "relicTier": 2,
    "relicChance": 0.75,
    "essence": [
      6,
      10
    ],
    "desc": "灼怒：敌人随回合提高攻击，拖得越久越危险；适合爆发与先手控制。 专属产出本系精华与灵印，不发放站点额度。"
  },
  {
    "id": "n2",
    "name": "潮壁圣殿·噩梦",
    "type": "噩梦裂隙",
    "img": "assets/d05-tide.webp",
    "cost": 24,
    "enemyLv": 116,
    "waves": 3,
    "enemyCount": 3,
    "starRound": 18,
    "exp": 650,
    "hasBoss": true,
    "tokenRange": [
      11,
      15
    ],
    "gold": [
      672,
      960
    ],
    "mat": [
      18,
      28
    ],
    "shardR": 0,
    "shardSR": 0.28,
    "shardSSR": 0.16,
    "matKey": "mat_ssr",
    "unlockStage": "r2",
    "lootFamily": "tide",
    "enemyElem": "water",
    "mechanic": "barrier",
    "hpScale": 6.4,
    "atkScale": 2.0,
    "defScale": 1.5,
    "enemyRes": 35,
    "relicTier": 2,
    "relicChance": 0.75,
    "essence": [
      6,
      10
    ],
    "desc": "结界：敌人自带护盾，每三回合补充护盾；侵蚀可越过护盾，吸血不会从护盾获取恢复。 专属产出本系精华与灵印，不发放站点额度。"
  },
  {
    "id": "n3",
    "name": "复苏秘林·噩梦",
    "type": "噩梦裂隙",
    "img": "assets/d04-forest.webp",
    "cost": 24,
    "enemyLv": 119,
    "waves": 3,
    "enemyCount": 3,
    "starRound": 18,
    "exp": 650,
    "hasBoss": true,
    "tokenRange": [
      11,
      15
    ],
    "gold": [
      672,
      960
    ],
    "mat": [
      18,
      28
    ],
    "shardR": 0,
    "shardSR": 0.28,
    "shardSSR": 0.16,
    "matKey": "mat_ssr",
    "unlockStage": "r3",
    "lootFamily": "grove",
    "enemyElem": "plant",
    "mechanic": "regrowth",
    "hpScale": 6.4,
    "atkScale": 2.0,
    "defScale": 1.5,
    "enemyRes": 35,
    "relicTier": 2,
    "relicChance": 0.75,
    "essence": [
      6,
      10
    ],
    "desc": "复苏：敌人每次行动恢复8%最大生命；侵蚀会把复苏压制到四分之一。 专属产出本系精华与灵印，不发放站点额度。"
  },
  {
    "id": "n4",
    "name": "折光星殿·噩梦",
    "type": "噩梦裂隙",
    "img": "assets/d02-sky.webp",
    "cost": 24,
    "enemyLv": 122,
    "waves": 3,
    "enemyCount": 3,
    "starRound": 18,
    "exp": 650,
    "hasBoss": true,
    "tokenRange": [
      11,
      15
    ],
    "gold": [
      672,
      960
    ],
    "mat": [
      18,
      28
    ],
    "shardR": 0,
    "shardSR": 0.28,
    "shardSSR": 0.16,
    "matKey": "mat_ssr",
    "unlockStage": "r4",
    "lootFamily": "astral",
    "enemyElem": "star",
    "mechanic": "reflection",
    "hpScale": 6.4,
    "atkScale": 2.0,
    "defScale": 1.5,
    "enemyRes": 35,
    "relicTier": 2,
    "relicChance": 0.75,
    "essence": [
      6,
      10
    ],
    "desc": "映伤：生命伤害的12%反弹给攻击者，堆纯攻击会更脆；需要护盾、吸血或治疗。 专属产出本系精华与灵印，不发放站点额度。"
  },
  {
    "id": "n5",
    "name": "毒雾墓城·噩梦",
    "type": "噩梦裂隙",
    "img": "assets/d01-volcano.webp",
    "cost": 24,
    "enemyLv": 125,
    "waves": 3,
    "enemyCount": 3,
    "starRound": 18,
    "exp": 650,
    "hasBoss": true,
    "tokenRange": [
      11,
      15
    ],
    "gold": [
      672,
      960
    ],
    "mat": [
      18,
      28
    ],
    "shardR": 0,
    "shardSR": 0.28,
    "shardSSR": 0.16,
    "matKey": "mat_ssr",
    "unlockStage": "r5",
    "lootFamily": "umbra",
    "enemyElem": "dark",
    "mechanic": "venom",
    "hpScale": 6.4,
    "atkScale": 2.0,
    "defScale": 1.5,
    "enemyRes": 35,
    "relicTier": 2,
    "relicChance": 0.75,
    "essence": [
      6,
      10
    ],
    "desc": "毒雾：敌人攻击附带侵蚀，侵蚀期间治疗与吸血减半；抵抗与净化有实战价值。 专属产出本系精华与灵印，不发放站点额度。"
  },
  {
    "id": "n6",
    "name": "封时钟塔·噩梦",
    "type": "噩梦裂隙",
    "img": "assets/d06-observatory.webp",
    "cost": 24,
    "enemyLv": 128,
    "waves": 3,
    "enemyCount": 3,
    "starRound": 18,
    "exp": 650,
    "hasBoss": true,
    "tokenRange": [
      11,
      15
    ],
    "gold": [
      672,
      960
    ],
    "mat": [
      18,
      28
    ],
    "shardR": 0,
    "shardSR": 0.28,
    "shardSSR": 0.16,
    "matKey": "mat_ssr",
    "unlockStage": "r6",
    "lootFamily": "chrono",
    "enemyElem": "time",
    "mechanic": "silence",
    "hpScale": 6.4,
    "atkScale": 2.0,
    "defScale": 1.5,
    "enemyRes": 35,
    "relicTier": 2,
    "relicChance": 0.75,
    "essence": [
      6,
      10
    ],
    "desc": "封怒：偶数回合敌人命中后削减15怒气；追风、时序灵印或不依赖技能的反击更合适。 专属产出本系精华与灵印，不发放站点额度。"
  }
];
// END GENERATED STAGES

/* ---------- 抽卡配置 ---------- */
const GACHA = {
  cost1: 150,      // 单抽金币
  cost10: 1400,    // 十连金币（省100）
  weightR: 0.795,  // R 概率
  weightSR: 0.18,  // SR 概率
  weightSSR: 0.025,// SSR 概率（基础）
  pitySSR: 90,     // 90 抽硬保底
  tenPitySR: 10,   // 十连必出SR
};

/* ---------- 养成配置 ---------- */
const GROWTH = {
  expPerLevel: 100,       // 每级经验需求（线性，可调）
  maxLevel: 150,
  starCost: { 1: 20, 2: 40, 3: 80, 4: 120, 5: 200, 6: 240, 7: 300, 8: 360, 9: 430, 10: 510, 11: 600, 12: 700 },  // 升到对应星级所需碎片
  starMax: 12,
  levelStatGrow: { hp: 0.06, atk: 0.06, def: 0.05, spd: 0.005 },  // 每级成长率
  starStatGrow: { hp: 0.10, atk: 0.10, def: 0.08, spd: 0.01 },    // 每星成长率
};

/* ---------- 战斗公式参数 ---------- */
const FORMULA = {
  defK: 300, defLv: 10,   // 1 - DEF / (DEF + 300 + 10*等级)
  ragePerNormal: 50,      // 普攻获得怒气
  rageMax: 100,
  critBase: 1.0,          // 暴击倍率 = 1 + CD/100
};

/* ---------- 敌人模板（按副本等级生成数值） ---------- */
// BEGIN GENERATED ENEMY_TPL — run python scripts/sync-catalog.py
const ENEMY_TPL = [
  {
    "name": "魔化狼",
    "img": "content/enemies/wolf/assets/portrait.webp",
    "elem": "plant",
    "atkType": "melee",
    "hpM": 0.9,
    "atkM": 1,
    "defM": 0.8,
    "spd": 100,
    "boss": false,
    "id": "wolf",
    "battleImg": "content/enemies/wolf/assets/portrait-cut.webp"
  },
  {
    "name": "石像卫兵",
    "img": "content/enemies/golem/assets/portrait.webp",
    "elem": "star",
    "atkType": "heavy",
    "hpM": 1.2,
    "atkM": 0.8,
    "defM": 1.3,
    "spd": 90,
    "boss": false,
    "id": "golem",
    "battleImg": "content/enemies/golem/assets/portrait-cut.webp"
  },
  {
    "name": "暗影法师",
    "img": "content/enemies/mage/assets/portrait.webp",
    "elem": "dark",
    "atkType": "magic",
    "hpM": 0.8,
    "atkM": 1.2,
    "defM": 0.7,
    "spd": 105,
    "boss": false,
    "id": "mage",
    "battleImg": "content/enemies/mage/assets/portrait-cut.webp"
  },
  {
    "name": "深渊主宰",
    "img": "content/bosses/abyss-lord/assets/portrait.webp",
    "elem": "time",
    "atkType": "magic",
    "hpM": 3.2,
    "atkM": 1.5,
    "defM": 1.5,
    "spd": 110,
    "boss": true,
    "id": "abyss-lord",
    "battleImg": "content/bosses/abyss-lord/assets/portrait-cut.webp"
  }
];
// END GENERATED ENEMY_TPL

// BEGIN GENERATED UNIT_ASSET_ALIASES — run python scripts/sync-catalog.py
const UNIT_ASSET_ALIASES = {
  "assets/c01-dengli.webp": "content/cards/c01/assets/portrait.webp",
  "assets/c01-dengli-cut.webp": "content/cards/c01/assets/portrait-cut.webp",
  "assets/c02-xiaoman.webp": "content/cards/c02/assets/portrait.webp",
  "assets/c02-xiaoman-cut.webp": "content/cards/c02/assets/portrait-cut.webp",
  "assets/c06-lingyu.webp": "content/cards/c06/assets/portrait.webp",
  "assets/c06-lingyu-cut.webp": "content/cards/c06/assets/portrait-cut.webp",
  "assets/c08-xiaohua.webp": "content/cards/c08/assets/portrait.webp",
  "assets/c08-xiaohua-cut.webp": "content/cards/c08/assets/portrait-cut.webp",
  "assets/c12-mei.webp": "content/cards/c12/assets/portrait.webp",
  "assets/c12-mei-cut.webp": "content/cards/c12/assets/portrait-cut.webp",
  "assets/c20-nuanxian.webp": "content/cards/c20/assets/portrait.webp",
  "assets/c20-nuanxian-cut.webp": "content/cards/c20/assets/portrait-cut.webp",
  "assets/c21-youya.webp": "content/cards/c21/assets/portrait.webp",
  "assets/c21-youya-cut.webp": "content/cards/c21/assets/portrait-cut.webp",
  "assets/c22-zhenguo.webp": "content/cards/c22/assets/portrait.webp",
  "assets/c22-zhenguo-cut.webp": "content/cards/c22/assets/portrait-cut.webp",
  "assets/c11-jianhua.webp": "content/cards/c11/assets/portrait.webp",
  "assets/c11-jianhua-cut.webp": "content/cards/c11/assets/portrait-cut.webp",
  "assets/c05-shuangyin.webp": "content/cards/c05/assets/portrait.webp",
  "assets/c05-shuangyin-cut.webp": "content/cards/c05/assets/portrait-cut.webp",
  "assets/c13-shengyu.webp": "content/cards/c13/assets/portrait.webp",
  "assets/c13-shengyu-cut.webp": "content/cards/c13/assets/portrait-cut.webp",
  "assets/c10-feipo.webp": "content/cards/c10/assets/portrait.webp",
  "assets/c10-feipo-cut.webp": "content/cards/c10/assets/portrait-cut.webp",
  "assets/c14-fusuo.webp": "content/cards/c14/assets/portrait.webp",
  "assets/c14-fusuo-cut.webp": "content/cards/c14/assets/portrait-cut.webp",
  "assets/c17-chengxi.webp": "content/cards/c17/assets/portrait.webp",
  "assets/c17-chengxi-cut.webp": "content/cards/c17/assets/portrait-cut.webp",
  "assets/c18-zhujian.webp": "content/cards/c18/assets/portrait.webp",
  "assets/c18-zhujian-cut.webp": "content/cards/c18/assets/portrait-cut.webp",
  "assets/c19-lanling.webp": "content/cards/c19/assets/portrait.webp",
  "assets/c19-lanling-cut.webp": "content/cards/c19/assets/portrait-cut.webp",
  "assets/c23-yeying.webp": "content/cards/c23/assets/portrait.webp",
  "assets/c23-yeying-cut.webp": "content/cards/c23/assets/portrait-cut.webp",
  "assets/c24-yutang.webp": "content/cards/c24/assets/portrait.webp",
  "assets/c24-yutang-cut.webp": "content/cards/c24/assets/portrait-cut.webp",
  "assets/c25-sulan.webp": "content/cards/c25/assets/portrait.webp",
  "assets/c25-sulan-cut.webp": "content/cards/c25/assets/portrait-cut.webp",
  "assets/c26-lingxiao.webp": "content/cards/c26/assets/portrait.webp",
  "assets/c26-lingxiao-cut.webp": "content/cards/c26/assets/portrait-cut.webp",
  "assets/c04-longyin.webp": "content/cards/c04/assets/portrait.webp",
  "assets/c04-longyin-cut.webp": "content/cards/c04/assets/portrait-cut.webp",
  "assets/c07-xingqiang.webp": "content/cards/c07/assets/portrait.webp",
  "assets/c07-xingqiang-cut.webp": "content/cards/c07/assets/portrait-cut.webp",
  "assets/c09-shiyu.webp": "content/cards/c09/assets/portrait.webp",
  "assets/c09-shiyu-cut.webp": "content/cards/c09/assets/portrait-cut.webp",
  "assets/c03-queen.webp": "content/cards/c03/assets/portrait.webp",
  "assets/c03-queen-cut.webp": "content/cards/c03/assets/portrait-cut.webp",
  "assets/c15-yueli.webp": "content/cards/c15/assets/portrait.webp",
  "assets/c15-yueli-cut.webp": "content/cards/c15/assets/portrait-cut.webp",
  "assets/c16-zhixing.webp": "content/cards/c16/assets/portrait.webp",
  "assets/c16-zhixing-cut.webp": "content/cards/c16/assets/portrait-cut.webp",
  "assets/c27-xueling.webp": "content/cards/c27/assets/portrait.webp",
  "assets/c27-xueling-cut.webp": "content/cards/c27/assets/portrait-cut.webp",
  "assets/c28-haili.webp": "content/cards/c28/assets/portrait.webp",
  "assets/c28-haili-cut.webp": "content/cards/c28/assets/portrait-cut.webp",
  "assets/gen/enemy-wolf.webp": "content/enemies/wolf/assets/portrait.webp",
  "assets/gen/enemy-wolf-cut.webp": "content/enemies/wolf/assets/portrait-cut.webp",
  "assets/enemy-wolf-cut.webp": "content/enemies/wolf/assets/portrait-cut.webp",
  "assets/gen/enemy-golem.webp": "content/enemies/golem/assets/portrait.webp",
  "assets/gen/enemy-golem-cut.webp": "content/enemies/golem/assets/portrait-cut.webp",
  "assets/enemy-golem-cut.webp": "content/enemies/golem/assets/portrait-cut.webp",
  "assets/gen/enemy-mage.webp": "content/enemies/mage/assets/portrait.webp",
  "assets/gen/enemy-mage-cut.webp": "content/enemies/mage/assets/portrait-cut.webp",
  "assets/enemy-mage-cut.webp": "content/enemies/mage/assets/portrait-cut.webp",
  "assets/gen/enemy-boss.webp": "content/bosses/abyss-lord/assets/portrait.webp",
  "assets/gen/enemy-boss-cut.webp": "content/bosses/abyss-lord/assets/portrait-cut.webp",
  "assets/enemy-boss-cut.webp": "content/bosses/abyss-lord/assets/legacy-cut.webp"
};
// END GENERATED UNIT_ASSET_ALIASES

// Resolve persisted battle-report image paths after the card folder migration.
function unitAsset(src) { return UNIT_ASSET_ALIASES[src] || src; }

// 只用于绘制当前账号的服务器快照，不提供客户端存档上传接口。
let SAVE = null;

function cultivationRules() { return Store.profile?.cultivation || null; }

function individualBonuses(own) {
  const rules = cultivationRules()?.individual, stats = { hp: 0, atk: 0, def: 0, spd: 0, cr: 0, cd: 0 };
  if (rules && own?.star > 0 && own.iv_values?.length === rules.stats.length) {
    rules.stats.forEach(({ stat, step }, index) => { stats[stat] = Math.floor(own.iv_values[index] / step); });
  }
  return stats;
}

function artifactIdentity(key) {
  const grades = Store.profile?.artifact_grades;
  if (grades) {
    if (typeof key !== "string" || key.length > 40) return null;
    const match = /^([a-z]+(?:_[1-6])?)(?:~([a-z]+))?(?:@([1-9]\d?)|!([1-9]\d{0,2})\.([1-9]\d?))?$/.exec(key);
    if (!match) return null;
    const base = match[1], [family, position] = base.split("_"), chip = !!position;
    if (chip ? !Object.hasOwn(cultivationRules()?.artifacts?.chip_sets || {},family) : !Object.hasOwn(GEMS,base)) return null;
    const standard = chip ? grades.chip_default : grades.gem_default, quality = match[2] || standard;
    const grade = grades.qualities.findIndex(option => option.id === quality), legacy = Number(match[3] || 0);
    const level = legacy ? legacy+1 : Number(match[4] || 1), rank = legacy ? Math.max(1,legacy) : Number(match[5] || 1);
    if (grade < 0 || legacy > 12 || level > grades.level_max || rank > grades.rank_max || (rank > 1 && level < grades.rank_levels[rank-2] && (quality !== standard || rank > Math.min(12,level-1)))) return null;
    const item = {base,family,chip,standard,quality,grade,level,rank};
    return artifactProgressKey(item) === key ? item : null;
  }
  const limit = Store.profile?.equipment?.refinement?.costs.length || 0;
  const match = /^([a-z]+(?:_[1-6])?)(?:@([1-9]\d?))?$/.exec(key || "");
  return match && Number(match[2] || 0) <= limit ? { base:match[1], level:Number(match[2] || 0) } : null;
}

function artifactProgressKey({base,quality,standard,level,rank}) {
  const id = quality === standard ? base : `${base}~${quality}`;
  if (level === 1 && rank === 1) return id;
  const legacy = level-1;
  return quality === standard && legacy >= 1 && legacy <= 12 && rank === Math.max(1,legacy) ? `${id}@${legacy}` : `${id}!${level}.${rank}`;
}

function artifactGradeStats(identity, spec) {
  const grades = Store.profile.artifact_grades, family = grades.families[identity.family], growth = grades.growth;
  const early = Math.min(identity.level-1,growth.early_steps), late = identity.level-1-early;
  const value = family.base[identity.grade]+(Math.floor(early/spec.every)+Math.floor(late/(growth.late_every*spec.every)))*spec.step;
  return {baseValue:family.base[identity.grade],value,extraHp:late*growth.late_hp};
}

function artifactGradeEffects(item) {
  const grades = Store.profile.artifact_grades, family = grades.families[item.family];
  return [{...family.rank_effect, value: Math.max(0,item.rank-1), active:item.rank>1, gate:"每阶"},
    ...item.spec.effects.map(effect=>({...effect,active:item.rank>=effect.level,gate:`${effect.level}阶`})),
    ...family.rare.map((effect,index)=>({...effect,active:item.grade>=index+4,gate:grades.qualities[index+4].name}))];
}

function artifactPrices(tab, item) {
  const grades = Store.profile.artifact_grades, rules = Store.profile.equipment.refinement, cost = grades.level_cost, step = item.level-1;
  const material = tab === "gems" ? rules.gem_mat : rules.chip_mat, ticket = tab === "gems" ? "gem_ticket" : "chip_ticket";
  return {level:item.level >= grades.level_max ? null : {[material]:cost.base+cost.linear*step+Math.floor(step*step/cost.square_divisor),forge_ore:cost.ore_base+Math.floor(step/cost.ore_every)},
    rank:item.rank >= grades.rank_max ? null : {[Store.profile.equipment.star_mat]:grades.rank_rivets[item.rank-1],[ticket]:grades.rank_tickets[item.rank-1]}, rankLevel:grades.rank_levels[item.rank-1]};
}

function refinementItem(tab, key) {
  const identity = artifactIdentity(key), rules = Store.profile?.equipment?.refinement;
  if (!identity || !rules) return null;
  const family = tab === "gems" ? identity.base : identity.base.split("_")[0], spec = rules[tab]?.[family];
  if (identity.quality) return spec && identity.chip === (tab === "chips") ? {...identity,spec,...artifactGradeStats(identity,spec)} : null;
  return spec ? { ...identity, family, spec, value:spec.base + Math.floor(identity.level / spec.every) * spec.step } : null;
}

function equippedRefinement(own) {
  const stats = { hp:0, atk:0, def:0, spd:0, cr:0, cd:0 };
  if (own?.star) for (const tab of ["gems","chips"]) for (const key of own[tab] || []) {
    const item = refinementItem(tab,key);
    if (item) {
      stats[item.spec.stat] += item.value-(item.quality && item.base === "wind" ? 3 : 0);
      stats.hp += item.extraHp || 0;
    }
  }
  return stats;
}

function artifactBonuses(own) {
  const rules = cultivationRules()?.artifacts;
  const stats = { hp: 0, atk: 0, def: 0, spd: 0, cr: 0, cd: 0 };
  if (!rules || !own?.star) return stats;
  const gemCounts = {}, chipCounts = {};
  for (const key of own.gems || []) {
    const gem = artifactIdentity(key)?.base;
    if (rules.gem_sets[gem]) gemCounts[gem] = (gemCounts[gem] || 0) + 1;
  }
  for (const [index, chip] of (own.chips || []).entries()) {
    const base = artifactIdentity(chip)?.base, family = base?.split("_")[0];
    if (rules.chip_sets[family] && base === `${family}_${index + 1}`) chipCounts[family] = (chipCounts[family] || 0) + 1;
  }
  for (const [counts, sets] of [[gemCounts, rules.gem_sets], [chipCounts, rules.chip_sets]]) {
    for (const [key, count] of Object.entries(counts)) {
      const set = sets[key];
      if (set.base) stats[set.base.stat] += count * set.base.value;
      for (const tier of set.tiers) if (count >= tier.count) stats[tier.stat] += tier.value;
    }
  }
  return stats;
}

function collectionBonds(id) {
  const rules = cultivationRules();
  return (rules?.crews || rules?.fellowships || rules?.bonds || []).filter(bond => bond.members.includes(id)).map(bond => {
    const spec = { ...bond, base: bond.base || rules.bond_base, advanced: bond.advanced || rules.bond_advanced };
    const stars = spec.members.map(member => SAVE.girls[member]?.star || 0);
    const owned = stars.filter(star => star > 0).length;
    const percent = owned !== stars.length ? 0 : Math.min(...stars) >= rules.bond_star ? spec.advanced : spec.base;
    return { spec, owned, percent };
  });
}

function collectionBond(id) { return collectionBonds(id)[0] || null; }

function boundGearBonuses(id, own) {
  const rules = Store.profile?.equipment, stats = { hp: 0, atk: 0, def: 0, spd: 0, cr: 0, cd: 0 };
  if (!rules || !own?.star) return stats;
  for (const slot of rules.slots) {
    const piece = own.equipment?.[slot.id], level = slot.id === "weapon" ? own.weapon_level : piece?.level;
    const stat = slot.stat === "signature" ? cultivationRules()?.weapons[id]?.stat : slot.stat;
    if (Object.hasOwn(stats, stat)) stats[stat] += (level || 0) * slot.per_level + (piece?.star || 0) * slot.per_star;
  }
  for (const tier of rules.set_tiers) {
    if (rules.slots.filter(slot => (own.equipment?.[slot.id]?.star || 0) >= tier.star).length >= tier.pieces) {
      for (const [stat, value] of Object.entries(tier.stats)) stats[stat] += value;
    }
  }
  return stats;
}

function starMilestones(id) {
  const girl = GIRLS.find(entry => entry.id === id), star = SAVE?.girls[id]?.star || 0;
  const describe = {
    rage: value => `开战时初始怒气 +${value}`,
    rage_gain: value => `每次普攻额外回复 ${value} 怒气`,
    shield: value => `开战时获得自身最大生命 ${value}% 的护盾`,
    skill: value => `怒气技的伤害倍率 +${value}%`,
    heal: value => `主动治疗与技能护盾效果 +${value}%`,
    counter: value => `自身受到生命伤害且存活时，反震自身防御 ${value}% 的伤害`,
    leech: value => `命中造成生命伤害时，回复实际伤害 ${value}% 的生命`,
  };
  return (cultivationRules()?.milestones?.[girl?.role] || []).map(step => ({ ...step, description: describe[step.effect](step.value), active: star >= step.star }));
}

function codexBonuses() {
  const rules = cultivationRules(), flat = { hp: 0, atk: 0, def: 0 };
  if (rules) for (const [id, own] of Object.entries(SAVE.girls)) {
    const stat = rules.weapons[id]?.stat;
    if (own.star > 0 && Object.hasOwn(flat, stat)) flat[stat] += rules.codex_flat[stat];
  }
  return flat;
}

/* ---------- 角色最终属性计算 ----------
 * finalStats(girlId) -> 含等级/星级成长
 */
function cardLevelCost(level) {
  if (level >= GROWTH.maxLevel) return 0;
  const rules = Store.profile?.training?.experience;
  const veteran = Math.max(0,level-(rules?.veteran_level ?? 60)+1), master = Math.max(0,level-(rules?.master_level ?? 120)+1);
  return Math.max(1,level)*GROWTH.expPerLevel + veteran*veteran*(rules?.veteran_growth ?? 10) + master*master*(rules?.master_growth ?? 25);
}

function cardLevelLimit() {
  const level = SAVE?.captain?.level;
  return Number.isSafeInteger(level) && level > 0 ? Math.min(GROWTH.maxLevel, level) : GROWTH.maxLevel;
}

function finalStats(girlId, includeBonds = false) {
  const g = GIRLS.find(x => x.id === girlId);
  const own = SAVE.girls[girlId];
  const lv = own && own.star > 0 ? Math.max(1,own.level) : 1;
  const st = own && own.star > 0 ? own.star : g.initialStars || RARITY[g.rarity].baseStar;
  const starGrowth = Math.max(0, st - (g.initialStars || 1));
  const b = g.base, gr = GROWTH.levelStatGrow, sr = GROWTH.starStatGrow;
  const gemCounts = {};
  for (const key of own?.gems || []) {
    const gem = artifactIdentity(key)?.base;
    if (gem) gemCounts[gem] = (gemCounts[gem] || 0) + 1;
  }
  let hp = b.hp * (1 + gr.hp * (lv-1)) * (1 + sr.hp * starGrowth) * (1 + 0.05 * (gemCounts.jade || 0));
  let atk = b.atk * (1 + gr.atk * (lv-1)) * (1 + sr.atk * starGrowth) * (1 + 0.05 * (gemCounts.ruby || 0));
  let def = b.def * (1 + gr.def * (lv-1)) * (1 + sr.def * starGrowth) * (1 + 0.05 * (gemCounts.sapphire || 0));
  let spd = b.spd * (1 + gr.spd * (lv-1)) * (1 + sr.spd * starGrowth) + 3 * (gemCounts.wind || 0);
  let cr = b.cr + 3 * (gemCounts.amethyst || 0);
  let cd = b.cd, ehr = b.ehr, res = b.res;
  const artifacts = artifactBonuses(own), individual = individualBonuses(own), gear = boundGearBonuses(girlId, own);
  const carried = own?.star > 0 && Store.profile?.carry?.items?.[own.carry];
  const combined = stat => artifacts[stat] + individual[stat] + gear[stat] + (carried?.stat === stat ? carried.value + (typeof carryLevel === "function" ? carryLevel(own.carry_xp || 0) - 1 : 0) : 0);
  hp *= 1 + combined("hp") / 100; atk *= 1 + combined("atk") / 100; def *= 1 + combined("def") / 100;
  spd += combined("spd"); cr += combined("cr"); cd += combined("cd");
  if (own?.training && typeof trainingBonuses === "function" && Store.profile?.training) {
    const {percent,flat} = trainingBonuses(own), factor = 1 + percent/100;
    hp = hp*factor + flat.hp; atk = atk*factor + flat.atk; def = def*factor + flat.def; spd += flat.spd; res += flat.res;
  }
  const sigil = own?.sigil && SAVE.sigils?.[own.sigil];
  if (sigil && SIGIL_FAMILIES[sigil.family]) {
    const a = sigil.affixes || {};
    hp *= 1 + (a.hp || 0) / 100;
    atk *= 1 + (a.atk || 0) / 100;
    def *= 1 + (a.def || 0) / 100;
    spd += a.spd || 0; cr += a.cr || 0; cd += a.cd || 0;
    ehr += a.ehr || 0; res += a.res || 0;
    if (sigil.family === "astral") cd += 20;
  }
  const talentRank = Math.min(3, Math.max(0, (own?.star || 0) - 2));
  if (own?.talent === "power") { atk *= 1 + 0.08 * talentRank; hp *= 1 - 0.04 * talentRank; }
  if (own?.talent === "guardian") { hp *= 1 + 0.05 * talentRank; def *= 1 + 0.10 * talentRank; atk *= 1 - 0.04 * talentRank; }
  if (own?.talent === "swift") { spd += 4 * talentRank; cr += 3 * talentRank; res -= 2 * talentRank; }
  if (own?.talent === "siphon") { atk *= 1 + 0.04 * talentRank; def *= 1 - 0.03 * talentRank; }
  if (own?.talent === "blight") { atk *= 1 - 0.06 * talentRank; ehr += 4 * talentRank; }
  if (own?.talent === "oracle") { hp *= 1 + 0.04 * talentRank; res += 4 * talentRank; atk *= 1 - 0.02 * talentRank; }
  if (includeBonds && SAVE.squad.includes(girlId)) {
    const team = SAVE.squad.map(id => GIRLS.find(girl => girl.id === id)).filter(Boolean);
    if (team.filter(girl => girl.elem === g.elem).length >= 2) atk *= 1.06;
    if (team.filter(girl => girl.role === g.role).length >= 3) hp *= 1.07;
    if (new Set(team.map(girl => girl.elem)).size >= 4) def *= 1.05;
  }
  const rules = cultivationRules();
  if (rules && own?.star > 0) {
    const stats = { hp, atk, def, spd }, weapon = rules.weapons[girlId];
    if (weapon) stats[weapon.stat] *= 1 + (own.weapon_level || 0) * rules.weapon_per_level / 100;
    stats.hp *= 1 + (own.weapon_awake || 0) * rules.weapon_vitality_per_awake / 100;
    const bondTotals = { hp: 0, atk: 0, def: 0 };
    for (const { spec, percent } of collectionBonds(girlId)) bondTotals[spec.stat] += percent;
    for (const [stat, percent] of Object.entries(bondTotals)) stats[stat] *= 1 + Math.min(rules.bond_cap || 30, percent) / 100;
    const potential = 1 + (own.potential || 0) * rules.potential_per_node / 100;
    for (const stat of ["hp", "atk", "def"]) stats[stat] *= potential;
    const engraving = rules.engravings[own.weapon_rune];
    if (engraving?.stat === "spd") stats.spd += engraving.value;
    else if (engraving) stats[engraving.stat] *= 1 + engraving.value / 100;
    const flat = codexBonuses();
    hp = stats.hp + flat.hp; atk = stats.atk + flat.atk; def = stats.def + flat.def; spd = stats.spd;
  }
  const flatRefinement = equippedRefinement(own);
  hp += flatRefinement.hp; atk += flatRefinement.atk; def += flatRefinement.def; spd += flatRefinement.spd;
  cr += flatRefinement.cr; cd += flatRefinement.cd;
  return {
    hp: Math.round(hp), atk: Math.round(atk), def: Math.round(def), spd: Math.round(spd),
    ...(b.spAtk ? {spAtk:Math.round(b.spAtk*atk/Math.max(1,b.atk))} : {}),
    ...(b.spDef ? {spDef:Math.round(b.spDef*def/Math.max(1,b.def))} : {}),
    cr: Math.min(cr, 100), cd, ehr: Math.min(100, ehr), res: Math.min(100, Math.max(0, res)),
  };
}

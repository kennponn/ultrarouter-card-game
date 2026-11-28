# 卡牌与敌方单位

游戏内容集中在 `content/`。每个单位的规则、图鉴信息和图片归同一个目录所有；根目录 `assets/` 只保留公共场景与界面素材。

新增或重绘角色前先按 [美术交付规范](ART-SPEC.md) 准备素材：当前每个单位必需卡面 `portrait.webp` 与透明战斗立绘 `portrait-cut.webp` 两张；规范同时说明尺寸、脚底落点、朝向、Boss 要求，以及尚需代码扩展的头像、技能特写与动作包。

```text
content/
  cards/index.json                  # 28 张角色卡的图鉴顺序
  cards/c01/card.json               # 灯里：属性、技能、介绍、图片与特效色
  cards/c01/assets/portrait.webp    # 图鉴/卡面
  cards/c01/assets/portrait-cut.webp # 战斗立绘
  enemies/index.json                # 普通敌人顺序
  enemies/wolf/enemy.json
  enemies/wolf/assets/...
  bosses/index.json                 # 首领顺序
  bosses/abyss-lord/boss.json
  bosses/abyss-lord/assets/...
```

修改角色时只编辑对应的 `card.json`；普通敌人与首领分别编辑 `enemy.json`、`boss.json`。角色 ID 保持稳定，显示名称可以修改。`img`、`battleImg` 必须指向本单位的 `assets/` 内已有 WebP；`fxColor` 是角色演出配色。原有技能、数值和图片保持不变，首领的旧版立绘另存为 `assets/legacy-cut.webp`。

维护步骤：

1. 修改已有单位，或新增一个 ID 目录，将 ID 加入所属 `index.json` 的 `ids` 列表。
2. 在项目根目录执行 `python scripts/sync-catalog.py`（或 `npm run catalog:sync`）。
3. 检查生成的 `js/data.js` 与 `backend/model/gacha_catalog.json` 差异，再执行 `python scripts/sync-catalog.py --check`。提交源目录和生成文件。

`GIRLS`、`ENEMY_TPL`、`UNIT_ASSET_ALIASES` 是生成区块，禁止手工维护。其余关卡与系统配置暂沿用原目录。客户端目录用于显示；正式玩法、账户与奖励仍由后端的嵌入目录和服务器结算决定，浏览器不能上传规则覆盖它们。

卡的 `sources` 是获取白名单，省略即只进常驻召唤（`summon`）；填写后不隐含常驻。`world` 指强者世界（SSR 轮值碎片或 R 整卡），`voyage`、副本碎片，`growth`、许愿及周常自选碎片，`starter`、首核心自选，`crew`、初始队伍。碎片不自动开卡，可在图鉴用同卡 R 20 / SR 50 / SSR 80 枚合成；既有角色和已持有的碎片保留。

`index.json` 明确图鉴顺序，不能依赖文件系统排序。现有后端普通敌人循环使用前 3 个模板，并使用第 4 个模板作为首领；新增敌人类型还需同步后端选取规则，不应只追加索引就假定已投放到关卡。

`legacyImages` 保存旧战报的图片路径映射。生成器将其汇总为 `UNIT_ASSET_ALIASES`；播放旧战报时通过 `unitAsset()` 解析，不需要在根 `assets/` 保留重复图片。`SOURCE.lock.json` 是原始导入快照，保留旧路径与原始哈希用于追溯；本次移植前的内容备份与逐文件哈希记录保存在本地 `.local/port-backup/content/`。

预览只公开图片与生成后的前端文件，`card.json` 等维护源不作为游戏 API。发布白名单只允许内容配置、索引、这份说明和 WebP，不包含宿主服务、凭据或本地 mock 数据。

# 星霜少女更新公告｜强者世界、人物图鉴与每日任务

强者世界换成参考活动页的角色展示、排行和宝箱布局；人物图鉴收录当前游戏的 28 位角色，集中展示技能、档案、获取途径和碎片合成；每日任务的「前往」按钮直接打开能完成任务的页面，操作完成后根据回包更新进度。

本次基于[六人战斗与公会远征 PR #7](https://github.com/kennponn/ultrarouter-card-game/pull/7)及[周限定召唤与公会奖励 PR #8](https://github.com/kennponn/ultrarouter-card-game/pull/8)，增量起点为 `e6be00f`。提交时这两个 PR 尚未合并，因此向上游 `main` 提交时会包含它们的改动。[本次界面增量对比](https://github.com/1810551251/ultrarouter-card-game/compare/e6be00f...feat/codex-daily-world-ui)可用于单独审阅这一轮。上一版公告继续保留。

配图均为本地浏览器加载实际项目界面、使用虚构账号与模拟回包的截图。截图来自各功能验收阶段，强者世界使用虚构开放周，不能作为线上账号或真实数据库结算的证明。15 张截图全部放在[独立截图仓库](https://github.com/1810551251/ultrarouter-card-game-screenshots/tree/d38cfd58b70efdc071219357d553db46db706117/codex-daily-world-ui-20261128)，本文使用固定提交链接；游戏仓库仅保存文档和[图片索引](screenshots.json)。

## 1. 强者世界：角色、排行与积分宝箱

按参考项目的活动页重排背景、标题、抽取按钮、榜单徽章和宝箱：左边显示本期角色、碎片与单抽／十连，右边切换积分排行与排名奖励，下方集中显示积分进度和可领宝箱。规则、奖池预览、往期领奖和刷新榜单均有独立入口，手机可滚动查看完整内容。

![强者世界桌面：左侧角色与召唤，右侧积分榜，下方五档积分箱](https://raw.githubusercontent.com/1810551251/ultrarouter-card-game-screenshots/d38cfd58b70efdc071219357d553db46db706117/codex-daily-world-ui-20261128/world-desktop.png)

图中 3 分箱已经开启，8 分和 16 分箱可领取，30 分和 50 分箱尚未达到；状态、奖励和本期积分同时显示。角色继续使用本项目立绘。抽取、扣券、加分及领奖沿用已有服务端接口，以确认回包刷新页面。

<img src="https://raw.githubusercontent.com/1810551251/ultrarouter-card-game-screenshots/d38cfd58b70efdc071219357d553db46db706117/codex-daily-world-ui-20261128/world-phone-awards.png" alt="390 像素手机：强者世界排名奖励页签" width="320">

手机实际切换排行与奖励、打开规则与奖池预览、领取往期奖励，并进行了单抽、十连和当期积分箱领取。另检查了 844×390 横屏时的滚动与按钮可达性。

## 2. 人物图鉴：填满两侧的群星档案

新增底部「图鉴」入口。背景改为与游戏主题一致的米白纸色、深蓝文字和金色细线；宽屏左侧人物名录、右侧角色详情独立滚动，1920 和 2560 宽度下两边各保留 12 像素边距。手机按名录与详情切换，底部导航保持可用。

![1920×1080 图鉴：人物名录与角色详情扩展到两侧](https://raw.githubusercontent.com/1810551251/ultrarouter-card-game-screenshots/d38cfd58b70efdc071219357d553db46db706117/codex-daily-world-ui-20261128/codex-wide.png)

搜索支持姓名、技能和地域，筛选包括品质、定位及获取状态。仅收录当前游戏已有定义的 28 位英雄；参考文档中尚未加入游戏的角色不会变成可选卡牌。图中测试账号拥有 6 位角色、另有 1 位碎片足够合成，未获得的卡面以灰色显示。

### 卡牌技能与人物档案

对照参考图鉴逐位补全 28 人的 56 项技能、112 条星阶描述、112 项人物能力，以及传记、事迹、外观、来历与反差描述。人物档案保留参考六维字段，尚无数值的特攻、特防显示「—」。这些是图鉴资料，不用参考设定覆盖账号的实战属性。

![卡牌技能：招式、星霜演化、施放意象与星阶描述](https://raw.githubusercontent.com/1810551251/ultrarouter-card-game-screenshots/d38cfd58b70efdc071219357d553db46db706117/codex-daily-world-ui-20261128/codex-skills.png)

![人物档案：参考六维和人物传记，继续向下滚动可看外观与能力](https://raw.githubusercontent.com/1810551251/ultrarouter-card-game-screenshots/d38cfd58b70efdc071219357d553db46db706117/codex-daily-world-ui-20261128/codex-dossier.png)

技能演示支持暂停、继续、倍速和重播；关闭、按 Escape、切页或退出账号会停止演出。页面明确标注「星霜演化」仍是参考图鉴的融合设计，尚未接入实战结算。人物羁绊页还可点击相关角色档案，例如从龙音跳到星墙。

### 获取、碎片与「读一页」

碎片保留在图鉴。尚未获得整卡的角色，即使已有碎片，也不会出现在编队候选中。R／SR／SSR 分别需要同卡 20／50／80 片，玩家在图鉴主动合成，等待服务端确认拥有后，才会进入编队；已有角色的剩余碎片继续用于培养。

![收录与获取：角色来源、碎片合成说明和读一页百骸洲纪事入口](https://raw.githubusercontent.com/1810551251/ultrarouter-card-game-screenshots/d38cfd58b70efdc071219357d553db46db706117/codex-daily-world-ui-20261128/codex-sources.png)

「收录与获取」列出实际可用的获取渠道，并补回「读一页百骸洲纪事」。阅读弹窗保留参考中的世界介绍和五章索引；索引是世界观文字，不代表增加了对应章节的英雄。

| 手机技能演示 | 手机世界观阅读 |
| --- | --- |
| <img src="https://raw.githubusercontent.com/1810551251/ultrarouter-card-game-screenshots/d38cfd58b70efdc071219357d553db46db706117/codex-daily-world-ui-20261128/codex-phone-preview.png" alt="技能演示暂停状态，可重播、继续和调速" width="300"> | <img src="https://raw.githubusercontent.com/1810551251/ultrarouter-card-game-screenshots/d38cfd58b70efdc071219357d553db46db706117/codex-daily-world-ui-20261128/codex-phone-chronicle.png" alt="读一页百骸洲纪事的阅读弹窗" width="300"> |

合成回归覆盖无碎片、碎片不足、79／80 片、足量合成、失败重试、重复点击，以及切换账号后旧回包抵达。未确认成功时不扣本地碎片，也不提前把英雄加入编队。

## 3. 每日任务：点到目的地，做完能回看

任务入口按当前已拥有角色、培养上限与开放关卡选择目的地。「去强化」直接打开角色工坊的装备页；「学技能」直接打开可学角色的技能页。只点击前往不会自动消费或完成任务，实际操作仍需要玩家确认，任务进度读取服务端回包。

| 每日任务 | 导航与实际操作 | 本地浏览器验收结果 |
| --- | --- | --- |
| 港口点名 | 抵达母港后，从日志领取 | 1/1，领取后同步仍为已领 |
| 三程巡航 | 打开已开放且消耗体力的剧情关卡；实测扫荡三次 | 3/3，可领取 |
| 甲板晨练 | 进入经验训练并定位操作入口，确认后完成战斗 | 1/1，可领取 |
| 向星许愿 | 打开成长页并定位定向许愿 | 1/1，可领取 |
| 整装备航 | 选择已拥有、可培养的角色与资源路线；实测读经验书 | 1/1，可领取 |
| 试剑竞技场 | 直接进入排名挑战，实际完成五场并返回 | 5/5，胜负均计入 |
| 锤火不息 | 打开可强化角色的装备部位，实测一键强化 | 5/5，可领取 |
| 勤学五式 | 跳过已练满角色，进入可学的技能路线，实测自动学习 | 5/5，可领取 |
| 今日航章 | 活动日历回到今天，进入今日题目并提交 | 1/1，可领取 |

装备、技能都练满时，入口转到消耗体力的实战复盘：验收账号实际完成五场后，装备和技能达到 5/5，整装备航在首场后达到 1/1。未获得的图鉴角色不会被用来判断是否毕业。若工坊配置尚未同步，入口回到成长页并提示同步。

| 手机每日任务列表 | 「去强化」的装备目的地 | 技能任务的学习目的地 |
| --- | --- | --- |
| <img src="https://raw.githubusercontent.com/1810551251/ultrarouter-card-game-screenshots/d38cfd58b70efdc071219357d553db46db706117/codex-daily-world-ui-20261128/daily-phone-tasks.png" alt="每日任务进度、奖励与前往按钮" width="260"> | <img src="https://raw.githubusercontent.com/1810551251/ultrarouter-card-game-screenshots/d38cfd58b70efdc071219357d553db46db706117/codex-daily-world-ui-20261128/daily-phone-equipment.png" alt="从每日任务直达角色工坊的可强化装备" width="260"> | <img src="https://raw.githubusercontent.com/1810551251/ultrarouter-card-game-screenshots/d38cfd58b70efdc071219357d553db46db706117/codex-daily-world-ui-20261128/daily-phone-skill.png" alt="从每日任务直达技能学习页面" width="260"> |

上述九项任务分阶段完成并逐项领取、同步回读；手机另外实际完成了装备和技能任务。跨天、切换账号或退出后，旧任务入口与迟到回包不会继续作用于新会话。

## 4. 宝箱统一展示：进度、奖励、领取状态

每日 25／50／75／100 分宝箱采用强者世界同类箱体：未达成显示差额，可领取显示按钮，领取后显示开箱状态。桌面并排展示，手机横向滑动查看。奖励名称和数量保留在卡片中。

![每日任务 100/100：四档宝箱均领取，并同步确认已领取状态](https://raw.githubusercontent.com/1810551251/ultrarouter-card-game-screenshots/d38cfd58b70efdc071219357d553db46db706117/codex-daily-world-ui-20261128/daily-all-claimed.png)

港口活动周印册、公会港务里程和远征星箱也复用这套展示。领取继续调用各自的服务端动作，奖励入包后刷新页面，不由界面直接修改奖励数量。

| 手机每日积分箱 | 港口活动周印册 | 公会港务里程 |
| --- | --- | --- |
| <img src="https://raw.githubusercontent.com/1810551251/ultrarouter-card-game-screenshots/d38cfd58b70efdc071219357d553db46db706117/codex-daily-world-ui-20261128/daily-phone-chests.png" alt="每日宝箱显示距离下一档的积分与奖励，支持横向滑动" width="260"> | <img src="https://raw.githubusercontent.com/1810551251/ultrarouter-card-game-screenshots/d38cfd58b70efdc071219357d553db46db706117/codex-daily-world-ui-20261128/festival-phone-chests.png" alt="活动周印册的宝箱与领取条件" width="260"> | <img src="https://raw.githubusercontent.com/1810551251/ultrarouter-card-game-screenshots/d38cfd58b70efdc071219357d553db46db706117/codex-daily-world-ui-20261128/guild-phone-chests.png" alt="公会港务里程的宝箱与参与条件" width="260"> |

## 5. 验证与接入范围

- 浏览器覆盖 390×844 手机、1440×1000 桌面，以及图鉴的 1920×1080、2560×1440 宽屏；强者世界另测 844×390 横屏。检查滚动、弹窗、按钮命中、图片加载和页面错误。
- 图鉴逐张核对 28 位角色的技能与档案；演示的暂停、重播、调速、关闭和阅读入口均实际点击。每日九项任务完成、领取、同步回读及四档宝箱领取使用本地模拟回包验收；战斗演出与排名冷却在测试夹具中缩短。
- 提交前完整前端回归 `npm test` 通过：100 项界面断言、244 项 Node 测试／子测试；目录同步检查与 20 项 Python 测试通过。`python scripts/audit-release.py` 扫描 351 个白名单文件通过，保留 4 处按哈希审核的样例匹配，本次未加载私有标记清单。复验命令和记录见 [VALIDATION.md](../../VALIDATION.md)。
- 本次相对 `e6be00f` 不修改后端结算模型；仍依赖前置 PR 的游戏接口。静态 HTML／CSS／JS 和运行素材可单独发布，无需为本次界面增量重建后端镜像。本次未部署。
- 本地未运行 Go 或三数据库集成测试；浏览器模拟回包不能替代真实 Go、数据库和生产接入验证。上游 PR 的 CI 结果以 GitHub 实际记录为准。

截图 PNG 仅在独立图床。游戏新增的 `assets/world/` 是界面运行需要的 40 个 WebP 控件与背景，已记录来源、尺寸和哈希，详情见 [ASSETS.md](../../ASSETS.md) 与 [assets/PROVENANCE.json](../../assets/PROVENANCE.json)。来源记录不等于独立的美术授权声明。

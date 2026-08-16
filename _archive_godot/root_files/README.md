# 三火算术练习 · 单机 MVP

这是一个 Godot 4.7.1 的手机竖屏原型：玩家看到 4 个数字，每个数字只能使用一次，通过 `+ - × ÷` 和点击合成得到 24。

## 运行

1. 推荐使用项目内 D 盘便携版 Godot 4.7.1，路径为 `tools/Godot4.7.1Portable/Godot_v4.7.1-stable_win64_sc.exe`。后续新增导出模板和编辑器缓存也统一放在 D 盘。
2. 运行项目，入口场景是 `scenes/MainMenu.tscn`；从首页进入闯关、每日、无尽和好友对战。
3. 也可以在 PowerShell 中执行：

```powershell
& "C:\Users\Administrator\AppData\Local\Microsoft\WinGet\Packages\GodotEngine.GodotEngine_Microsoft.Winget.Source_8wekyb3d8bbwe\Godot_v4.7.1-stable_win64_console.exe" --path "D:\微信小游戏"
```

也可以直接运行项目内脚本：

```powershell
& "D:\微信小游戏\tools\run_godot_d.ps1" -Editor
```

## Web 预览导出

项目已加入 `Web` 导出预设，用于检查竖屏布局、触摸点击、音频和资源加载：

```powershell
& "D:\微信小游戏\tools\Godot4.7.1Portable\Godot_v4.7.1-stable_win64_sc.exe" --path "D:\微信小游戏" --export-release "Web" "D:\微信小游戏\exports\web\24dian_challenge.html"
```

如果提示缺少 Web export template，需要在 D 盘便携版 Godot 编辑器的“编辑器设置 → 导出 → 管理导出模板”中安装对应 4.7.1 模板。模板应安装到 `D:\微信小游戏\tools\Godot4.7.1Portable\editor_data\export_templates`。Web 导出只用于兼容性预览，不能直接替代微信小游戏最终包。

## 当前实现

- 首页先进入“闯关模式”，再进入关卡选择界面；每页 20 关，共 200 个关卡、10 个章节，可用上一页/下一页切换
- 首页提供无尽模式：答对继续，答错、非法运算或超时立即结束；题目由本地智能出题器无限生成
- 无尽模式每 3 题提高难度，会持续缩短答题时间、减少可行解数量，后段加入 10～13 的数字，并记录最高分、最高答题数和最高连击
- 无尽模式支持连续答题倍率、5/10/15 题里程碑、个人纪录追赶提示；每日无尽金币奖励上限为 120 金币
- 每道无尽题都必须经过 `PuzzleGenerator.solve` 验证，确认有解、目标为 24、满足整数规则后才会显示；同一局会尽量避免数字组合重复
- 当前“AI”是离线本地难度导演和候选生成算法，不依赖网络；未来接入真实大模型时，模型只能提供候选题或难度建议，不能绕过程序验题
- 普通关 3 题，每 5 关设置 1 个 5 题挑战关，当前已扩充到 200 关，适合碎片时间持续体验
- 首页提供每日三题挑战，按日期固定题目并轮换 8 种规则：禁用除法、不可撤销、进阶数字、必须减法、必须乘法、禁用乘法、禁用加法、快速出手
- 每日挑战完成后获得金币，连续 3 天/7 天有额外奖励；奖励领取按日期防重复
- 首页提供主题商店，当前有黑板绿、深海蓝、落日橙三套外观，皮肤只改变配色
- 首次进入闯关模式会显示三步操作教学
- 合并成功有轻量反馈动画
- 程序穷举并验证题目，保存至少一种解法
- 先点击第一个数字，再点击运算符，最后点击第二个数字完成合成
- 只允许整数中间结果，自动拦截除数为 0
- 撤销、重置本题、有限提示、计时、计分、连击
- 失败和通关结算、1～3 星、本地最高分与解锁进度
- 单机与未来联机共用 `MatchData` 数据外形
- 广告服务只保留占位接口，不接真实广告 SDK
- 当前原型用 `user://twenty_four_progress.json` 保存解锁关卡、每关星级/最高分和最近进入的关卡
- 项目开启桌面鼠标模拟触摸，方便用电脑提前检查微信端点击体验；手机端使用原生触摸事件
- 存档还保存金币、已拥有/当前装备皮肤、每日挑战奖励领取记录、登录奖励、每周任务、挑战统计和章节音乐设置；旧版本存档会自动补齐字段
- 无尽模式的成绩保存在同一个本地存档中；正式上架微信小游戏时应迁移到云开发/服务端

## 文件结构

```text
project.godot        项目配置与启动入口
mvp.tscn             24 点 MVP 场景
mvp.gd               UI、点击流程与单机局内状态
core/puzzle_generator.gd  题目生成、穷举求解与难度筛选
core/daily_challenge.gd   每日三题、固定日期和 8 种特殊规则
core/endless_mode.gd      无尽模式难度曲线和题目配置
core/skin_catalog.gd      主题皮肤配置
core/level_catalog.gd     200 关、10 章节配置
core/match_data.gd        单机/联机共用数据结构
services/save_service.gd 本地存档
services/reward_service.gd 金币奖励、登录奖励与防重复领取
services/ad_service.gd    广告占位接口
services/task_service.gd  每日/每周任务与周期刷新
services/player_stats.gd  玩家挑战记录
services/share_service.gd 好友房间和战绩分享适配层
tests/verify_mvp.gd       无界面自动验证脚本
tests/verify_endless.gd   无尽模式专项验证脚本
tests/verify_advanced_features.gd 登录奖励、每周任务、每日规则、统计和分享验证
tests/verify_web_preparation.gd 竖屏、拉伸和触摸入口验证
```

## 验证

```powershell
& "C:\Users\Administrator\AppData\Local\Microsoft\WinGet\Packages\GodotEngine.GodotEngine_Microsoft.Winget.Source_8wekyb3d8bbwe\Godot_v4.7.1-stable_win64_console.exe" --headless --path "D:\微信小游戏" --script res://tests/verify_mvp.gd
```

## 已知风险

Godot 4.7.1 可以稳定运行桌面/Web 原型，但微信小游戏不是 Godot 的官方一键导出目标。后续需要单独验证 Web 导出、小游戏文件系统、触摸输入、音频和包体限制；题目与业务逻辑已拆分，便于迁移到微信小游戏适配层或其他前端运行时。

正式上架微信小游戏时，建议使用微信登录态对应的服务端/云开发数据库保存进度。`user://` 或小游戏本地存储只能作为离线缓存，换手机、清缓存或重新安装后可能丢失，不能作为唯一进度来源。

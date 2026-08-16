# 前端模块说明

这是原生微信小游戏 Canvas 前端。`game.js` 是平台入口，`src/app.js` 只保留兼容导出，真正的应用控制器位于 `src/app/game_app.js`。

## 目录职责

| 目录 | 职责 |
| --- | --- |
| `src/app/` | 应用生命周期、运行恢复和公共应用工具 |
| `src/config/` | 画布、存档键、版本等稳定配置 |
| `src/core/` | 题目生成、验证、关卡、每日题、无尽题、对战数据和皮肤目录 |
| `src/input/` | 触摸坐标换算、命中检测和拖动区域处理 |
| `src/modes/` | 闯关、每日、无尽、好友对战的模式编排；`campaign_progress.js` 负责严格的逐关解锁 |
| `src/services/` | 存档、音频、广告、平台、后端、分享、排行榜和奖励服务 |
| `src/ui/` | 主题、页面布局和 Canvas 页面绘制路由 |

## 页面绘制边界

- `src/ui/screen_renderer.js` 负责每帧绘制顺序：背景、页面、弹窗、连接提示、反馈和点击特效。
- `src/ui/page_layout.js` 负责安全区、页面顶部、弹窗居中、游戏卡片和底部区域坐标。
- `src/app/game_app.js` 继续作为状态编排器，保留各页面的实际绘制函数和业务回调，避免本轮整理改变玩法。

## 修改规则

1. 题目和规则只改 `src/core/`，不要在 UI 里临时生成题目。
2. 存档只通过 `src/services/storage.js`，不要在页面里直接写 `wx.setStorageSync`。
3. 新页面先注册到 `src/ui/screen_renderer.js`，再补触摸命中区域。
4. 新模式优先放入 `src/modes/`，网络请求放入 `src/services/api_client.js`。
5. 后端和 `_archive_godot` 不属于本前端目录，本轮整理不触碰它们。

闯关进度规则：第 1 关默认开放；第 N 关只有在第 N-1 关存档中存在完成记录后才开放。`unlocked_level` 仍保留用于兼容旧存档和分页，但不再单独作为解锁权限。

## 整理后的检查命令

在本目录执行：

```powershell
Get-ChildItem src,tools -Recurse -Filter *.js | ForEach-Object { node --check $_.FullName }
node tools/module_audit.js
node tools/smoke_test.js
node tools/layout_audit.js
node tools/question_quality_audit.js
node tools/friend_match_audit.js
node tools/matchmaking_audit.js
node tools/stability_audit.js
node tools/prelaunch_audit.js
```

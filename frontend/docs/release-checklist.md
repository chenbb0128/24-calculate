# 三火算术练习｜微信小游戏发布清单

## 微信开发者工具

1. 导入目录：`D:\微信小游戏\frontend\wechat_game`。
2. 确认 AppID 为 `wx1e7ac815548c561c`，项目类型为“微信小游戏”。
3. 确认 request 合法域名已经配置：`https://calc-api.pdurl.cn`。
4. 使用正式项目配置编译：合法域名校验开启、代码压缩开启、源码映射关闭。
5. 编译后检查 Console 没有请求 `127.0.0.1`、`localhost` 或局域网地址。

## 真机冒烟

- 冷启动并完成真实微信登录；
- 首页能正常显示服务端 bootstrap 的用户和进度；
- 闯关、每日挑战、无尽模式均能创建服务端 run；
- 完成后结果页显示“服务端已校验”；
- 网络失败时显示重试提示，不增加金币、解锁、任务或排行榜成绩；
- 好友房创建、加入、准备、开始、进度同步和结算各测试一次；
- 账号 A 和账号 B 的本地设置、缓存和进度不串；
- 切后台、恢复、弱网和重复点击不导致重复奖励。

## 发布阻塞项

- `frontend/wechat_game/src/services/platform.js` 中的广告位目前仍是占位 ID，替换真实广告位前不要宣称广告功能已上线。
- 隐私协议、用户协议、备案和平台类目材料必须与实际上载版本一致。
- 生产数据库备份必须先完成一次恢复演练。

## 自动检查

```powershell
cd D:\微信小游戏\frontend\wechat_game
node tools/prelaunch_audit.js
node tools/prelaunch_audit.js --strict
node tools/account_storage_audit.js
node tools/smoke_test.js
node tools/friend_match_audit.js
node tools/matchmaking_audit.js
node tools/production_probe.js

cd D:\微信小游戏\backend
D:\bin\go.exe test ./...
D:\bin\go.exe vet ./...
D:\bin\go.exe build ./...
```

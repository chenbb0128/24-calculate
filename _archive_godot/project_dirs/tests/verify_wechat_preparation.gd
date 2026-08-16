extends SceneTree

const PlatformAdapterScript = preload("res://services/platform_adapter.gd")
const ShareServiceScript = preload("res://services/share_service.gd")


func _init() -> void:
	var adapter := PlatformAdapterScript.new()
	var contract: Dictionary = PlatformAdapterScript.integration_contract()
	assert(str(contract["wx_storage_key"]) == "twenty_four_progress", "微信存档键名错误")
	assert(adapter.runtime_name() == "godot_prototype", "原型运行时标识错误")
	assert(not adapter.is_wechat_runtime(), "Godot 原型不能误判为微信运行时")
	var fallback := {"coins": 12, "levels": {"0": {"stars": 3}}}
	var loaded := adapter.load_progress_cache(fallback)
	loaded["coins"] = 99
	assert(int(fallback["coins"]) == 12, "平台缓存接口应返回独立副本")
	assert(adapter.save_progress_cache(fallback), "平台存档接口应接受字典")
	var cloud_payload: Dictionary = adapter.build_cloud_progress_payload({"version": 9, "unlocked_level": 20, "coins": 12, "owned_skins": ["classic"], "equipped_skin": "classic"}, "player-a")
	assert(int(cloud_payload["contract_version"]) == 1, "云存档协议版本错误")
	assert(not cloud_payload["progress"].has("audio"), "音频设置不应进入云端主进度")
	var merged: Dictionary = adapter.merge_cloud_progress({"unlocked_level": 4, "coins": 20, "owned_skins": ["classic"], "equipped_skin": "classic"}, {"progress": {"unlocked_level": 20, "coins": 12, "owned_skins": ["classic", "ocean"], "equipped_skin": "ocean"}})
	assert(int(merged["unlocked_level"]) == 20 and int(merged["coins"]) == 20, "云存档合并策略错误")
	assert(str(merged["equipped_skin"]) == "ocean", "云端已拥有皮肤应可恢复装备")
	var ad_request: Dictionary = adapter.build_ad_reward_request("hint", "session-1", 2)
	assert(bool(ad_request["complete_only"]) and not bool(ad_request["client_grants_reward"]), "广告奖励必须等待服务端/SDK完成回调")
	var match := {"match_id": "friend-A7K2", "room_seed": 20260813, "puzzles": [{"puzzle_id": "F-Q1"}, {"puzzle_id": "F-Q2"}]}
	var submission: Dictionary = adapter.build_match_submission_payload(match, {"outcome": "win", "player_score": 560}, "player-a")
	assert(submission["puzzle_ids"].size() == 2 and bool(submission["client_score_is_provisional"]), "好友对战提交协议错误")
	var share: Dictionary = adapter.share_contract(ShareServiceScript.create_friend_room_payload({"room_code": "A7K2", "room_seed": 20260813}))
	assert(str(share["api"]) == "wx.shareAppMessage", "分享接口契约错误")
	assert(bool(share["requires_user_action"]), "分享必须由用户主动触发")
	var leaderboard: Dictionary = adapter.leaderboard_contract("campaign", 560, {"level": 20})
	assert(not bool(leaderboard["client_is_authoritative"]), "排行榜不能信任客户端分数")
	print("WeChat preparation verification passed: storage, ad/share and server boundary contracts")
	quit()

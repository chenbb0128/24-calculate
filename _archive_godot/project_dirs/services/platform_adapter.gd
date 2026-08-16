class_name PlatformAdapter
extends RefCounted

## 平台边界层：Godot 原型中使用本地模拟，迁移微信小游戏时只替换这里。
## 题目、计分和玩法不依赖微信 API，避免把 wx 调用散落到界面脚本中。

const RUNTIME := "godot_prototype"
const WECHAT_RUNTIME := "wechat_minigame"
const PROGRESS_STORAGE_KEY := "twenty_four_progress"
const DATA_CONTRACT_VERSION := 1


static func integration_contract() -> Dictionary:
	# 这是 Godot 原型与微信小游戏之间的稳定边界。微信端只需要在对应方法
	# 内接入 wx API，玩法、题目、计分和 UI 不需要知道平台细节。
	return {
		"runtime": RUNTIME,
		"data_contract_version": DATA_CONTRACT_VERSION,
		"progress_storage_key": PROGRESS_STORAGE_KEY,
		"wx_storage_key": PROGRESS_STORAGE_KEY,
		"required_apis": [
			"wx.getStorageSync / wx.setStorageSync",
			"wx.createRewardedVideoAd",
			"wx.shareAppMessage",
			"wx.cloud.callFunction or server HTTPS API",
		],
		"server_owned": ["leaderboards", "friend_match_result", "anti_cheat"],
		"local_cache_owned": ["audio", "equipped_skin", "tutorial_seen"],
		"sync_fields": ["version", "unlocked_level", "levels", "coins", "owned_skins", "equipped_skin", "daily", "endless", "player_stats"],
	}


func runtime_name() -> String:
	return RUNTIME


func is_wechat_runtime() -> bool:
	return false


func load_progress_cache(fallback: Dictionary) -> Dictionary:
	# Godot 版由 SaveService 负责真正的 user:// 文件读写；微信版在这里映射
	# 到 wx.getStorageSync(PROGRESS_STORAGE_KEY)。返回深拷贝，避免平台层修改 UI 引用。
	return fallback.duplicate(true)


func save_progress_cache(progress: Dictionary) -> bool:
	# 微信版：wx.setStorageSync(PROGRESS_STORAGE_KEY, progress)。
	return progress is Dictionary


func build_cloud_progress_payload(progress: Dictionary, player_id: String = "") -> Dictionary:
	# 只同步影响进度和经济的数据；UI、临时弹窗和本地音量不进入云端主记录。
	return {
		"contract_version": DATA_CONTRACT_VERSION,
		"player_id": player_id,
		"progress": _pick_cloud_progress(progress),
		"updated_at_ms": Time.get_unix_time_from_system() * 1000,
	}


func merge_cloud_progress(local_progress: Dictionary, cloud_payload: Dictionary) -> Dictionary:
	# 云端数据在微信版应经过服务端鉴权；Godot 这里仅实现可测试的字段合并规则。
	var cloud_progress: Dictionary = cloud_payload.get("progress", {})
	if cloud_progress.is_empty():
		return local_progress.duplicate(true)
	var merged := local_progress.duplicate(true)
	for key in ["unlocked_level", "coins"]:
		merged[key] = maxi(int(merged.get(key, 0)), int(cloud_progress.get(key, 0)))
	for key in ["levels", "owned_skins", "daily", "endless", "player_stats"]:
		if cloud_progress.has(key):
			merged[key] = cloud_progress[key].duplicate(true) if cloud_progress[key] is Dictionary else cloud_progress[key]
	if cloud_progress.has("equipped_skin") and str(cloud_progress["equipped_skin"]) in merged.get("owned_skins", []):
		merged["equipped_skin"] = str(cloud_progress["equipped_skin"])
	return merged


func build_ad_reward_request(reward_type: String, session_id: String, attempt: int = 1) -> Dictionary:
	return {
		"contract_version": DATA_CONTRACT_VERSION,
		"reward_type": reward_type,
		"session_id": session_id,
		"attempt": maxi(1, attempt),
		"complete_only": true,
		"client_grants_reward": false,
	}


func build_match_submission_payload(match: Dictionary, result: Dictionary, player_id: String = "") -> Dictionary:
	return {
		"contract_version": DATA_CONTRACT_VERSION,
		"player_id": player_id,
		"match_id": str(match.get("match_id", match.get("room_id", ""))),
		"room_seed": int(match.get("room_seed", 0)),
		"puzzle_ids": _puzzle_ids(match.get("puzzles", [])),
		"result": result.duplicate(true),
		"client_score_is_provisional": true,
		"submit_to": "server_or_cloud_function",
	}


func show_rewarded_ad(_reward_type: String) -> bool:
	# 微信版在这里接 wx.createRewardedVideoAd 的 onClose 回调。
	return true


func show_rewarded_ad_async_contract() -> Dictionary:
	return {
		"preload": "createRewardedVideoAd({adUnitId})",
		"on_close": "仅在 res.isEnded == true 时发放奖励",
		"fallback": "广告加载失败时不扣除次数、不阻断核心玩法",
	}


func share(_payload: Dictionary) -> bool:
	# 微信版在这里接 wx.shareAppMessage；桌面原型视为成功。
	return true


func share_contract(payload: Dictionary) -> Dictionary:
	var result := payload.duplicate(true)
	result["api"] = "wx.shareAppMessage"
	result["requires_user_action"] = true
	return result


func submit_score(_mode: String, _score: int, _metadata: Dictionary = {}) -> bool:
	# 微信版可替换为云开发/服务端提交，Godot 原型继续走本地排行榜。
	return true


func leaderboard_contract(mode: String, score: int, metadata: Dictionary = {}) -> Dictionary:
	return {
		"mode": mode,
		"score": maxi(0, score),
		"metadata": metadata.duplicate(true),
		"submit_to": "server_or_cloud_function",
		"client_is_authoritative": false,
	}


func _pick_cloud_progress(progress: Dictionary) -> Dictionary:
	var result: Dictionary = {}
	for key in ["version", "unlocked_level", "last_level", "levels", "coins", "owned_skins", "equipped_skin", "daily", "endless", "player_stats"]:
		if progress.has(key):
			result[key] = progress[key].duplicate(true) if progress[key] is Dictionary or progress[key] is Array else progress[key]
	return result


func _puzzle_ids(puzzles: Array) -> Array[String]:
	var ids: Array[String] = []
	for puzzle in puzzles:
		if puzzle is Dictionary:
			ids.append(str(puzzle.get("puzzle_id", "")))
	return ids

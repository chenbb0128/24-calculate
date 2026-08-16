class_name AdService
extends RefCounted

## 低打扰广告接口：当前使用“模拟激励广告”，不接入真实 SDK。
## 微信版只需替换 show_rewarded 的实现，界面和奖励限制不变。

const DAILY_REWARDED_LIMIT := 3
const PlatformAdapterScript = preload("res://services/platform_adapter.gd")

signal reward_completed(reward_type: String)

var date_key := ""
var rewarded_used_today := 0
var platform_adapter: RefCounted


func _init() -> void:
	platform_adapter = PlatformAdapterScript.new()


func configure(saved: Dictionary, current_date_key: String) -> void:
	date_key = current_date_key
	if str(saved.get("date", "")) == current_date_key:
		rewarded_used_today = clampi(int(saved.get("rewarded_used", 0)), 0, DAILY_REWARDED_LIMIT)
	else:
		rewarded_used_today = 0


func is_available() -> bool:
	return rewarded_used_today < DAILY_REWARDED_LIMIT


func show_rewarded(reward_type: String) -> bool:
	if not is_available():
		return false
	if platform_adapter and not platform_adapter.show_rewarded_ad(reward_type):
		return false
	# 原型中立即完成，正式微信版在这里等待激励广告 onClose 回调。
	rewarded_used_today += 1
	reward_completed.emit(reward_type)
	return true


func usage() -> Dictionary:
	return {
		"date": date_key,
		"rewarded_used": rewarded_used_today,
		"daily_limit": DAILY_REWARDED_LIMIT,
		"remaining": maxi(0, DAILY_REWARDED_LIMIT - rewarded_used_today),
	}


func show_interstitial() -> bool:
	# 暂不启用插屏，避免打断思考和答题流程。
	return false

extends SceneTree

const SkinCatalogScript = preload("res://core/skin_catalog.gd")
const RewardServiceScript = preload("res://services/reward_service.gd")


func _init() -> void:
	var locked_progress := {"coins": 999, "unlocked_level": 5, "levels": {}}
	var ocean_locked := SkinCatalogScript.unlock_status("ocean", locked_progress)
	assert(not bool(ocean_locked["unlocked"]), "深海蓝应有解锁关卡门槛")
	var ready_progress := {"coins": 999, "unlocked_level": 10, "levels": {}}
	var ocean_ready := SkinCatalogScript.unlock_status("ocean", ready_progress)
	assert(bool(ocean_ready["unlocked"]), "推进到第 10 关后应满足深海蓝条件")
	var sunset_locked := SkinCatalogScript.unlock_status("sunset", ready_progress)
	assert(not bool(sunset_locked["unlocked"]), "落日橙应有星星门槛")
	var star_progress := {"coins": 0, "unlocked_level": 25, "levels": {}}
	for index in range(12):
		star_progress["levels"][str(index)] = {"stars": 1}
	assert(bool(SkinCatalogScript.unlock_status("sunset", star_progress)["unlocked"]), "满足关卡和星星后应解锁落日橙兑换资格")
	var daily_progress := {"coins": 0, "daily": {"streak": 0, "reward_claimed": {}}}
	var daily_reward := RewardServiceScript.claim_daily_reward(daily_progress, "2026-08-13", 100, 3, false)
	assert(int(daily_reward["coins"]) == 50, "每日基础奖励应适度降低")
	var endless_progress := {"coins": 0, "endless": {"best_score": 0, "best_questions": 0, "best_combo": 0}}
	var endless_reward := RewardServiceScript.claim_endless_reward(endless_progress, 500, 10, 5, "2026-08-13")
	assert(int(endless_reward["coins"]) <= 90, "单局无尽奖励不应过高")
	assert(int(endless_reward["daily_cap"]) == 120, "无尽每日奖励上限错误")
	print("Economy verification passed: reduced rewards, skin gates and daily cap")
	quit()

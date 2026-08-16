extends SceneTree

const AchievementServiceScript = preload("res://services/achievement_service.gd")
const SaveServiceScript = preload("res://services/save_service.gd")


func _init() -> void:
	var progress := {"coins": 0}
	AchievementServiceScript.ensure_progress(progress)
	assert(progress.has("achievements"), "成就存档字段应自动补齐")
	assert(AchievementServiceScript.all().size() == 11, "首批成就数量错误")
	var first := AchievementServiceScript.unlock(progress, "first_clear")
	assert(str(first["title"]) == "迈出第一步", "成就标题错误")
	assert(int(first["reward"]) == 30, "成就奖励错误")
	assert(int(progress["coins"]) == 30, "成就金币没有发放")
	var repeat := AchievementServiceScript.unlock(progress, "first_clear")
	assert(repeat.is_empty(), "同一成就不能重复解锁")
	assert(int(progress["coins"]) == 30, "重复解锁不应重复发金币")
	var combo_unlocks := AchievementServiceScript.unlock_many(progress, ["combo_5", "combo_10"])
	assert(combo_unlocks.size() == 2, "批量解锁数量错误")
	assert(AchievementServiceScript.unlocked_count(progress) == 3, "已解锁数量错误")
	var next := AchievementServiceScript.next_hint(progress)
	assert(not next.is_empty(), "未完成全部成就时应有下一个目标")
	assert(not AchievementServiceScript.format_unlocks(combo_unlocks).is_empty(), "成就提示文本不能为空")
	var migrated := SaveServiceScript._migrate_for_test({"coins": 7, "tasks": {}})
	assert(migrated.has("achievements"), "旧存档迁移应补齐成就字段")
	assert(migrated["achievements"].has("unlocked"), "迁移后的成就应有解锁记录")
	print("Achievement verification passed: unlock, one-time rewards, migration and formatting")
	quit()

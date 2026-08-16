extends SceneTree

const LevelCatalogScript = preload("res://core/level_catalog.gd")
const RewardServiceScript = preload("res://services/reward_service.gd")
const PlatformAdapterScript = preload("res://services/platform_adapter.gd")
const SaveServiceScript = preload("res://services/save_service.gd")


func _init() -> void:
	var levels: Array = LevelCatalogScript.all()
	assert(LevelCatalogScript.chapter_count() == 10, "应有 10 个章节")
	assert(str(levels[0]["chapter_name"]) == "基础星球", "第 1 章名称错误")
	assert(str(levels[20]["chapter_name"]) == "括号秘境", "第 2 章名称错误")
	assert(int(levels[20]["chapter_index"]) == 1, "第 21 关应进入第 2 章")
	assert(str(levels[160]["chapter_name"]) == "极光天台", "第 161 关章节名称错误")
	assert(str(levels[199]["chapter_name"]) == "终极方程式", "第 200 关章节名称错误")

	var progress := {"coins": 0, "levels": {}, "milestones": {"first_clear": false, "three_star": false, "chapters": {}}}
	var first := RewardServiceScript.claim_campaign_bonus(progress, 0, 3, 0, false, true)
	assert(int(first["coins"]) == 20, "首次通关三星里程碑应奖励 20 金币")
	var repeated := RewardServiceScript.claim_campaign_bonus(progress, 0, 3, 0, false, false)
	assert(int(repeated["coins"]) == 0, "重复通关不应重复领取里程碑奖励")
	var chapter := RewardServiceScript.claim_campaign_bonus(progress, 19, 3, 0, true, true)
	assert(int(chapter["coins"]) == 20, "章节完成应奖励 20 金币")
	assert(int(progress["coins"]) == 40, "里程碑奖励应写入金币")

	var migrated := SaveServiceScript._migrate_for_test({"version": 7, "coins": 0})
	assert(migrated.has("milestones"), "旧存档应补全里程碑字段")
	var adapter := PlatformAdapterScript.new()
	assert(adapter.runtime_name() == "godot_prototype", "Godot 原型平台标识错误")
	assert(adapter.show_rewarded_ad("hint"), "原型激励广告接口应可用")
	assert(adapter.share({"room_code": "123456"}), "原型分享接口应可用")
	print("Progression verification passed: chapters, milestone rewards, migration and platform adapter")
	quit()

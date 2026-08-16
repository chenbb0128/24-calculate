extends SceneTree

const AdServiceScript = preload("res://services/ad_service.gd")
const RewardServiceScript = preload("res://services/reward_service.gd")
const SaveServiceScript = preload("res://services/save_service.gd")


func _init() -> void:
	var service := AdServiceScript.new()
	service.configure({}, "2026-08-13")
	assert(service.is_available(), "新的一天应可以领取广告奖励")
	assert(service.show_rewarded("hint"), "第一次激励广告应成功")
	assert(service.show_rewarded("continue"), "第二次激励广告应成功")
	assert(service.show_rewarded("coins"), "第三次激励广告应成功")
	assert(not service.is_available(), "每日 3 次后应达到上限")
	assert(int(service.usage()["remaining"]) == 0, "广告剩余次数应为 0")
	var hint_service := AdServiceScript.new()
	hint_service.configure({}, "2026-08-13")
	assert(hint_service.show_rewarded("hint"), "提示广告奖励应可用")
	assert(hint_service.show_rewarded("undo"), "撤销广告奖励应可用")
	assert(str(hint_service.usage()["remaining"]) == "1", "提示和撤销应共用每日广告额度")
	service.configure(service.usage(), "2026-08-14")
	assert(service.is_available(), "跨天应重置广告次数")
	var progress := {"coins": 10}
	assert(RewardServiceScript.claim_ad_coin_bonus(progress, 25) == 25, "广告金币奖励计算错误")
	assert(int(progress["coins"]) == 35, "广告金币没有正确写入存档")
	var old_progress := {"version": 5, "coins": 10}
	var migrated := SaveServiceScript._migrate_for_test(old_progress)
	assert(migrated.has("ads"), "旧存档应补全广告记录")
	assert(migrated["ads"].has("rewarded_used"), "广告记录字段缺失")
	print("Ads verification passed: voluntary rewards, daily cap, rollover and migration")
	quit()

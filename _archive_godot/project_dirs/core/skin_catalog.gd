class_name SkinCatalog
extends RefCounted

## 外观皮肤只改变颜色和氛围，不改变时间、提示或题目难度。

static func all() -> Array:
	return [
		{
			"id": "classic",
			"name": "星空玻璃",
			"description": "深蓝紫背景与青绿色操作高光",
			"price": 0,
			"theme": {
				"bg": "#1e1b4b", "surface": "#25215b", "surface_2": "#312e81",
				"card": "#ffffff", "card_text": "#ffffff", "accent": "#fbbf24",
				"blue": "#22d3ee", "gold": "#fbbf24", "text": "#ffffff", "muted": "#a5b4fc",
			},
		},
		{
			"id": "ocean",
			"name": "深海蓝",
			"description": "安静、清晰，适合夜间挑战",
			"price": 360,
			"min_level": 10,
			"requirement_text": "解锁第 10 关后可兑换",
			"theme": {
				"bg": "#102b4f", "surface": "#19436c", "surface_2": "#245b87",
				"card": "#e9f6ff", "card_text": "#123f65", "accent": "#75e3ff",
				"blue": "#3188b8", "gold": "#e2bd62", "text": "#f3fbff", "muted": "#b5d4e6",
			},
		},
		{
			"id": "sunset",
			"name": "落日橙",
			"description": "热烈的冲刺挑战氛围",
			"price": 720,
			"min_level": 25,
			"min_stars": 12,
			"requirement_text": "解锁第 25 关并累计 12 颗星",
			"theme": {
				"bg": "#542b27", "surface": "#763d2f", "surface_2": "#985039",
				"card": "#fff0d9", "card_text": "#683727", "accent": "#ffd06b",
				"blue": "#c56549", "gold": "#f1bd5b", "text": "#fff8ee", "muted": "#e8c4aa",
			},
		},
	]


static func get_skin(skin_id: String) -> Dictionary:
	for skin in all():
		if str(skin["id"]) == skin_id:
			return skin
	return all()[0]


static func theme_colors(skin_id: String) -> Dictionary:
	return get_skin(skin_id).get("theme", {}).duplicate(true)


static func unlock_status(skin_id: String, progress: Dictionary) -> Dictionary:
	var skin := get_skin(skin_id)
	var min_level := int(skin.get("min_level", 0))
	var unlocked_level := int(progress.get("unlocked_level", 0))
	if unlocked_level < min_level:
		return {"unlocked": false, "reason": "解锁第 %d 关后可兑换" % min_level}
	var min_stars := int(skin.get("min_stars", 0))
	if min_stars > 0:
		var total_stars := 0
		var levels: Dictionary = progress.get("levels", {})
		for record in levels.values():
			if record is Dictionary:
				total_stars += int(record.get("stars", 0))
		if total_stars < min_stars:
			return {"unlocked": false, "reason": "还需累计 %d 颗星（当前 %d）" % [min_stars, total_stars]}
	return {"unlocked": true, "reason": ""}

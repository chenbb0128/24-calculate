class_name LevelCatalog
extends RefCounted


const CHAPTERS := [
	{
		"name": "基础星球",
		"subtitle": "先熟悉三步操作，稳稳算出 24",
		"goal": "整数与明显解法",
		"color": "#5ecdf2",
		"tint": "#152d68",
	},
	{
		"name": "括号秘境",
		"subtitle": "学会安排顺序，让括号帮你取胜",
		"goal": "括号与多步计算",
		"color": "#c69bff",
		"tint": "#34205e",
	},
	{
		"name": "连击跑道",
		"subtitle": "越快越高分，保持你的连胜节奏",
		"goal": "速度与连击",
		"color": "#ff8ea8",
		"tint": "#5a244f",
	},
	{
		"name": "困难星门",
		"subtitle": "数字范围扩大，解法更加珍贵",
		"goal": "高难度与少解",
		"color": "#ffc775",
		"tint": "#5b3c28",
	},
	{
		"name": "大师终点站",
		"subtitle": "完成全部章节，成为 24 点大师",
		"goal": "综合挑战",
		"color": "#9beab8",
		"tint": "#1c4e4a",
	},
	{
		"name": "星云实验室",
		"subtitle": "在变化的数字里寻找稳定路线",
		"goal": "多解筛选与节奏",
		"color": "#8de8ff",
		"tint": "#123f62",
	},
	{
		"name": "彩虹迷宫",
		"subtitle": "每一步都可能打开新的出口",
		"goal": "顺序判断与组合",
		"color": "#ffb6e1",
		"tint": "#60294e",
	},
	{
		"name": "时空回廊",
		"subtitle": "在有限时间里完成更少见的解法",
		"goal": "速度与少解",
		"color": "#b8c7ff",
		"tint": "#303a76",
	},
	{
		"name": "极光天台",
		"subtitle": "挑战更大的数字和更紧的节奏",
		"goal": "进阶数字与连击",
		"color": "#a8f0d0",
		"tint": "#1c5a55",
	},
	{
		"name": "终极方程式",
		"subtitle": "完成 200 关，成为真正的 24 点大师",
		"goal": "全规则综合挑战",
		"color": "#ffd58a",
		"tint": "#684323",
	},
]


static func chapter(index: int) -> Dictionary:
	return CHAPTERS[clampi(index, 0, CHAPTERS.size() - 1)].duplicate(true)


static func chapter_for_level(level_index: int) -> Dictionary:
	var chapter_index := clampi(int(level_index / 20), 0, CHAPTERS.size() - 1)
	var result := chapter(chapter_index)
	result["index"] = chapter_index
	result["level_start"] = chapter_index * 20 + 1
	result["level_end"] = mini((chapter_index + 1) * 20, CHAPTERS.size() * 20)
	return result


static func chapter_count() -> int:
	return CHAPTERS.size()


static func all() -> Array:
	var levels: Array = []
	for index in range(CHAPTERS.size() * 20):
		var chapter_index := int(index / 20)
		var chapter_level := index % 20
		var chapter := chapter_for_level(index)
		var is_challenge := index % 5 == 4
		var config := {
			"title": "第 %d 关" % (index + 1),
			"question_count": 5 if is_challenge else 3,
			"time_limit": maxf(35.0, (95.0 if is_challenge else 62.0) - float(chapter_level * 1.5) - float(chapter_index * 3)),
			"target_score": (520 if is_challenge else 270) + chapter_level * 28 + chapter_index * 80,
			"target_combo": 3 + int(chapter_level / 3) + chapter_index,
			"is_challenge": is_challenge,
			"allow_hint": chapter_level < 16,
			"hint_count": 2 if chapter_level < 5 else 1,
			"min_digit": 1,
			"max_digit": 9 if chapter_index == 0 and chapter_level < 15 else 13,
			"min_solutions": 2 if chapter_index == 0 and chapter_level < 5 else 1,
			"max_solutions": 999999 if chapter_index == 0 and chapter_level < 15 else 6,
			"chapter_index": chapter_index,
			"chapter_name": chapter["name"],
			"chapter_subtitle": chapter["subtitle"],
			"chapter_goal": chapter["goal"],
			"chapter_color": chapter["color"],
			"chapter_tint": chapter["tint"],
			"chapter_level": chapter_level + 1,
		}
		if is_challenge:
			config.title = "挑战关"
		elif chapter_index == 0 and chapter_level < 5:
			config.title = "基础整数"
		elif chapter_index == 0 and chapter_level < 10:
			config.title = "括号进阶"
		elif chapter_index == 0 and chapter_level < 15:
			config.title = "连击冲刺"
		elif chapter_index == 0:
			config.title = "困难挑战"
		elif chapter_index == 1:
			config.title = "速度训练"
		elif chapter_index == 2:
			config.title = "混合运算"
		elif chapter_index == 3:
			config.title = "极限数字"
		elif chapter_index == 4:
			config.title = "大师挑战"
		elif chapter_index == 5:
			config.title = "实验室试炼"
		elif chapter_index == 6:
			config.title = "迷宫路线"
		elif chapter_index == 7:
			config.title = "时空竞速"
		elif chapter_index == 8:
			config.title = "极光冲刺"
		else:
			config.title = "终极方程式"
		levels.append(config)
	return levels

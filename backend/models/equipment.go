package models

import (
	"encoding/json"
	"sort"
)

// EquipmentSet 封装设备集合，提供语义化查询方法。
// 存储格式: JSON array string，如 `["projector","smartboard","aircon"]`
// 空字符串 = 无设备。
type EquipmentSet struct {
	items map[string]bool
}

// ParseEquipment 从 JSON string 解析为 EquipmentSet。
// 解析失败返回空集合（不报错，降级处理）。
func ParseEquipment(jsonStr string) EquipmentSet {
	if jsonStr == "" {
		return EquipmentSet{}
	}
	var arr []string
	if err := json.Unmarshal([]byte(jsonStr), &arr); err != nil {
		return EquipmentSet{}
	}
	s := EquipmentSet{items: make(map[string]bool, len(arr))}
	for _, e := range arr {
		s.items[e] = true
	}
	return s
}

// Has 检查是否包含某个设备。
func (s EquipmentSet) Has(item string) bool {
	return s.items[item]
}

// ContainsAll 检查是否包含所有所需设备。
func (s EquipmentSet) ContainsAll(required EquipmentSet) bool {
	for item := range required.items {
		if !s.items[item] {
			return false
		}
	}
	return true
}

// Items 返回设备列表（按字典序排序，保证确定性）。
func (s EquipmentSet) Items() []string {
	out := make([]string, 0, len(s.items))
	for item := range s.items {
		out = append(out, item)
	}
	sort.Strings(out)
	return out
}

// IsEmpty 检查是否为空集。
func (s EquipmentSet) IsEmpty() bool {
	return len(s.items) == 0
}

// ToJSON 序列化回 JSON string（按字典序排序，保证确定性）。
func (s EquipmentSet) ToJSON() string {
	if len(s.items) == 0 {
		return ""
	}
	arr := s.Items()
	b, _ := json.Marshal(arr)
	return string(b)
}

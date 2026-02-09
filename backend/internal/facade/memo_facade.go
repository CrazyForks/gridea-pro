package facade

import (
	"context"
	"encoding/json"
	"fmt"
	"gridea-pro/backend/internal/domain"
	"gridea-pro/backend/internal/service"
	"regexp"
	"sort"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	gonanoid "github.com/matoous/go-nanoid/v2"
)

// MemoFacade wraps MemoService
type MemoFacade struct {
	internal *service.MemoService
}

func NewMemoFacade(s *service.MemoService) *MemoFacade {
	return &MemoFacade{internal: s}
}

func (f *MemoFacade) LoadMemos() ([]domain.Memo, error) {
	memos, err := f.internal.LoadMemos(context.TODO())
	if err != nil {
		return nil, err
	}

	// 按创建时间倒序排列
	sort.Slice(memos, func(i, j int) bool {
		return memos[i].CreatedAt > memos[j].CreatedAt
	})

	return memos, nil
}

func (f *MemoFacade) SaveMemos(memos []domain.Memo) error {
	return f.internal.SaveMemos(context.TODO(), memos)
}

// extractTags 从内容中提取 #标签
func extractTags(content string) []string {
	re := regexp.MustCompile(`#([\p{L}\p{N}_]+)`)
	matches := re.FindAllStringSubmatch(content, -1)

	tagSet := make(map[string]bool)
	tags := make([]string, 0)
	for _, match := range matches {
		if len(match) > 1 {
			tag := match[1]
			if !tagSet[tag] {
				tagSet[tag] = true
				tags = append(tags, tag)
			}
		}
	}
	return tags
}

// GetMemoStats 获取闪念统计数据
func (f *MemoFacade) GetMemoStats() (map[string]interface{}, error) {
	memos, err := f.LoadMemos()
	if err != nil {
		return nil, err
	}

	// 统计标签
	tagCount := make(map[string]int)
	for _, memo := range memos {
		for _, tag := range memo.Tags {
			tagCount[tag]++
		}
	}

	// 转换为数组格式
	var tagStats []map[string]interface{}
	for name, count := range tagCount {
		tagStats = append(tagStats, map[string]interface{}{
			"name":  name,
			"count": count,
		})
	}

	// 按数量排序
	sort.Slice(tagStats, func(i, j int) bool {
		return tagStats[i]["count"].(int) > tagStats[j]["count"].(int)
	})

	// 生成热力图数据 (过去365天)
	heatmap := make(map[string]int)
	now := time.Now()
	for i := 0; i < 365; i++ {
		date := now.AddDate(0, 0, -i).Format(domain.DateLayout)
		heatmap[date] = 0
	}

	for _, memo := range memos {
		// 解析时间字符串
		t, err := time.Parse(domain.TimeLayout, memo.CreatedAt)
		if err == nil {
			date := t.Format(domain.DateLayout)
			if _, exists := heatmap[date]; exists {
				heatmap[date]++
			}
		}
	}

	return map[string]interface{}{
		"total":   len(memos),
		"tags":    tagStats,
		"heatmap": heatmap,
	}, nil
}

// RegisterEvents 注册闪念相关事件监听器
func (f *MemoFacade) RegisterEvents(ctx context.Context) {
	registerMemoLoadEvent(ctx, f)
	registerMemoSaveEvent(ctx, f)
	registerMemoDeleteEvent(ctx, f)
	registerMemoUpdateEvent(ctx, f)
	registerMemoRenameTagEvent(ctx, f)
	registerMemoDeleteTagEvent(ctx, f)
}

// registerMemoLoadEvent 注册闪念加载事件
func registerMemoLoadEvent(ctx context.Context, facade *MemoFacade) {
	runtime.EventsOn(ctx, "memo-load", func(data ...interface{}) {
		memos, err := facade.LoadMemos()
		if err != nil {
			runtime.EventsEmit(ctx, "memo-loaded", map[string]interface{}{
				"success": false,
				"memos":   []domain.Memo{},
				"stats":   nil,
			})
			return
		}

		stats, _ := facade.GetMemoStats()

		runtime.EventsEmit(ctx, "memo-loaded", map[string]interface{}{
			"success": true,
			"memos":   memos,
			"stats":   stats,
		})
	})
}

// registerMemoSaveEvent 注册闪念保存事件
func registerMemoSaveEvent(ctx context.Context, facade *MemoFacade) {
	runtime.EventsOn(ctx, "memo-save", func(data ...interface{}) {
		if len(data) == 0 {
			runtime.EventsEmit(ctx, "memo-saved", map[string]interface{}{
				"success": false,
			})
			return
		}

		var content string

		if len(data) > 0 {
			fmt.Printf("Backend received memo-save data: %+v\n", data[0])
			switch v := data[0].(type) {
			case string:
				content = v
			case map[string]interface{}:
				if c, ok := v["content"].(string); ok {
					content = c
				}
			}
		}

		if content == "" {
			runtime.EventsEmit(ctx, "memo-saved", map[string]interface{}{
				"success": false,
			})
			return
		}

		// 生成新闪念
		const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
		id, err := gonanoid.Generate(alphabet, 6)
		if err != nil {
			runtime.EventsEmit(ctx, "memo-saved", map[string]interface{}{
				"success": false,
			})
			return
		}

		now := time.Now().Format(domain.TimeLayout)
		newMemo := domain.Memo{
			ID:        id,
			Content:   content,
			Tags:      extractTags(content),
			Images:    []string{},
			CreatedAt: now,
			UpdatedAt: now,
		}

		// 加载现有闪念
		memos, err := facade.LoadMemos()
		if err != nil {
			memos = []domain.Memo{}
		}

		// 添加新闪念到开头
		memos = append([]domain.Memo{newMemo}, memos...)

		// 保存
		if err := facade.SaveMemos(memos); err != nil {
			runtime.EventsEmit(ctx, "memo-saved", map[string]interface{}{
				"success": false,
			})
			return
		}

		stats, _ := facade.GetMemoStats()

		runtime.EventsEmit(ctx, "memo-saved", map[string]interface{}{
			"success": true,
			"memo":    newMemo,
			"memos":   memos,
			"stats":   stats,
		})
		fmt.Printf("闪念保存成功: %s\n", id)
	})
}

// registerMemoDeleteEvent 注册闪念删除事件
func registerMemoDeleteEvent(ctx context.Context, facade *MemoFacade) {
	runtime.EventsOn(ctx, "memo-delete", func(data ...interface{}) {
		if len(data) == 0 {
			runtime.EventsEmit(ctx, "memo-deleted", map[string]interface{}{
				"success": false,
			})
			return
		}

		memoID, ok := data[0].(string)
		if !ok {
			runtime.EventsEmit(ctx, "memo-deleted", map[string]interface{}{
				"success": false,
			})
			return
		}

		// 加载现有闪念
		memos, err := facade.LoadMemos()
		if err != nil {
			memos = []domain.Memo{}
		}

		// 过滤掉要删除的闪念
		filtered := make([]domain.Memo, 0)
		for _, memo := range memos {
			if memo.ID != memoID {
				filtered = append(filtered, memo)
			}
		}

		// 保存
		if err := facade.SaveMemos(filtered); err != nil {
			runtime.EventsEmit(ctx, "memo-deleted", map[string]interface{}{
				"success": false,
			})
			return
		}

		stats, _ := facade.GetMemoStats()

		runtime.EventsEmit(ctx, "memo-deleted", map[string]interface{}{
			"success": true,
			"memos":   filtered,
			"stats":   stats,
		})
		fmt.Printf("闪念删除成功: %s\n", memoID)
	})
}

// registerMemoUpdateEvent 注册闪念更新事件
func registerMemoUpdateEvent(ctx context.Context, facade *MemoFacade) {
	runtime.EventsOn(ctx, "memo-update", func(data ...interface{}) {
		if len(data) == 0 {
			runtime.EventsEmit(ctx, "memo-updated", map[string]interface{}{
				"success": false,
			})
			return
		}

		// 解析更新数据
		memoMap, ok := data[0].(map[string]interface{})
		if !ok {
			runtime.EventsEmit(ctx, "memo-updated", map[string]interface{}{
				"success": false,
			})
			return
		}

		jsonBytes, err := json.Marshal(memoMap)
		if err != nil {
			runtime.EventsEmit(ctx, "memo-updated", map[string]interface{}{
				"success": false,
			})
			return
		}

		var updatedMemo domain.Memo
		if err := json.Unmarshal(jsonBytes, &updatedMemo); err != nil {
			runtime.EventsEmit(ctx, "memo-updated", map[string]interface{}{
				"success": false,
			})
			return
		}

		// 更新标签和时间
		updatedMemo.Tags = extractTags(updatedMemo.Content)
		updatedMemo.UpdatedAt = time.Now().Format(domain.TimeLayout)

		// 加载现有闪念
		memos, err := facade.LoadMemos()
		if err != nil {
			memos = []domain.Memo{}
		}

		// 查找并更新
		found := false
		for i := range memos {
			if memos[i].ID == updatedMemo.ID {
				memos[i].Content = updatedMemo.Content
				memos[i].Tags = updatedMemo.Tags
				memos[i].UpdatedAt = updatedMemo.UpdatedAt
				found = true
				break
			}
		}

		if !found {
			runtime.EventsEmit(ctx, "memo-updated", map[string]interface{}{
				"success": false,
			})
			return
		}

		// 保存
		if err := facade.SaveMemos(memos); err != nil {
			runtime.EventsEmit(ctx, "memo-updated", map[string]interface{}{
				"success": false,
			})
			return
		}

		stats, _ := facade.GetMemoStats()

		runtime.EventsEmit(ctx, "memo-updated", map[string]interface{}{
			"success": true,
			"memo":    updatedMemo,
			"memos":   memos,
			"stats":   stats,
		})
		fmt.Printf("闪念更新成功: %s\n", updatedMemo.ID)
	})
}

// RenameTag 重命名标签
func (f *MemoFacade) RenameTag(oldName, newName string) error {
	memos, err := f.LoadMemos()
	if err != nil {
		return err
	}

	// 预编译正则，确保只匹配完整的标签
	// 匹配 #oldName 后面非标签字符（如空格、标点、换行）或结尾
	// re := regexp.MustCompile(`#` + regexp.QuoteMeta(oldName) + `([^\p{L}\p{N}_]|$)`)
	// 上面的正则在替换主要用 ReplaceAllString

	count := 0
	updatedMemos := make([]domain.Memo, 0)

	// 为了安全起见，我们遍历所有 memo，检查 Tags 列表
	for i := range memos {
		hasTag := false
		for _, t := range memos[i].Tags {
			if t == oldName {
				hasTag = true
				break
			}
		}

		if hasTag {
			// 执行替换
			// 使用正则替换：#oldName -> #newName
			// 注意：这里简单起见，我们假设标签形式规范。
			// 更严谨的做法是正则替换，保留后续的分隔符
			re := regexp.MustCompile(`#` + regexp.QuoteMeta(oldName) + `([^\p{L}\p{N}_]|$)`)
			memos[i].Content = re.ReplaceAllString(memos[i].Content, "#"+newName+"$1")

			// 重新提取标签和更新时间
			memos[i].Tags = extractTags(memos[i].Content)
			memos[i].UpdatedAt = time.Now().Format(domain.TimeLayout)
			count++
		}
		updatedMemos = append(updatedMemos, memos[i])
	}

	if count > 0 {
		return f.SaveMemos(updatedMemos)
	}
	return nil
}

// DeleteTag 删除标签
func (f *MemoFacade) DeleteTag(tagName string) error {
	memos, err := f.LoadMemos()
	if err != nil {
		return err
	}

	count := 0
	updatedMemos := make([]domain.Memo, 0)

	for i := range memos {
		hasTag := false
		for _, t := range memos[i].Tags {
			if t == tagName {
				hasTag = true
				break
			}
		}

		if hasTag {
			// 执行删除
			// #tagName -> "" (或者保留后面的分隔符)
			// 如果标签前后都有空格，删除标签后可能会有两个空格，这里暂时不处理那么细致，只删除标签本身和标签符号
			re := regexp.MustCompile(`#` + regexp.QuoteMeta(tagName) + `([^\p{L}\p{N}_]|$)`)
			// 替换为 tagName$1 (移除#但保留tagName和后续字符)
			memos[i].Content = re.ReplaceAllString(memos[i].Content, tagName+"$1")

			// 重新提取标签和更新时间
			memos[i].Tags = extractTags(memos[i].Content)
			memos[i].UpdatedAt = time.Now().Format(domain.TimeLayout)
			count++
		}
		updatedMemos = append(updatedMemos, memos[i])
	}

	if count > 0 {
		return f.SaveMemos(updatedMemos)
	}
	return nil
}

// registerMemoRenameTagEvent 注册重命名标签事件
func registerMemoRenameTagEvent(ctx context.Context, facade *MemoFacade) {
	runtime.EventsOn(ctx, "memo-rename-tag", func(data ...interface{}) {
		if len(data) < 2 {
			return
		}
		oldName, ok1 := data[0].(string)
		newName, ok2 := data[1].(string)
		if !ok1 || !ok2 {
			return
		}

		err := facade.RenameTag(oldName, newName)
		success := err == nil

		memos, _ := facade.LoadMemos()
		stats, _ := facade.GetMemoStats()

		runtime.EventsEmit(ctx, "memo-renamed-tag", map[string]interface{}{
			"success": success,
			"memos":   memos,
			"stats":   stats,
		})
	})
}

// registerMemoDeleteTagEvent 注册删除标签事件
func registerMemoDeleteTagEvent(ctx context.Context, facade *MemoFacade) {
	runtime.EventsOn(ctx, "memo-delete-tag", func(data ...interface{}) {
		if len(data) < 1 {
			return
		}
		tagName, ok := data[0].(string)
		if !ok {
			return
		}

		err := facade.DeleteTag(tagName)
		success := err == nil

		memos, _ := facade.LoadMemos()
		stats, _ := facade.GetMemoStats()

		runtime.EventsEmit(ctx, "memo-deleted-tag", map[string]interface{}{
			"success": success,
			"memos":   memos,
			"stats":   stats,
		})
	})
}

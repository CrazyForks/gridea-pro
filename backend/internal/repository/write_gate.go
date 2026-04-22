package repository

import (
	"sync"
	"time"
)

// WriteGate 记录最近由 app 自身发起的文件写入，供 ResourceWatcher 过滤"保存 → fsnotify →
// 再次触发渲染"的自激循环。
//
// 工作机制：
//   - `WriteFileAtomic` 在 rename 前调用 `MarkSelfWrite(path)`，给该路径打上一个短 TTL 的时间戳
//   - 观察者（`ResourceWatcher`）在处理 fsnotify 事件前调用 `IsSelfWrite(path)`；
//     命中 + 未过期 → 跳过这次事件（认为是 app 自己刚写的）
//   - 命中后 entry 立刻清掉，避免后续外部编辑器的改动被误吞
//
// TTL 取得足够覆盖"写 tmp → rename → fsnotify 投递到 user space"的正常链路，
// 但又小到让 2s 之后的外部改动可以正常触发。实测 macOS FSEvents 大约 50-300ms 到达，
// Linux inotify 更快，2 秒已经是很宽的窗口。
//
// 只对 app 内部的 `WriteFileAtomic` 生效 —— 外部编辑器（Typora / VSCode / `git checkout`）
// 直接写文件不会经过本模块，watcher 仍能正常把这种"真实的用户意图"转成渲染事件。
type WriteGate struct {
	mu    sync.Mutex
	paths map[string]time.Time
}

// DefaultWriteGate 进程级共享实例。直接使用，不需要手动实例化。
var DefaultWriteGate = &WriteGate{paths: map[string]time.Time{}}

// defaultWriteGateTTL 是 MarkSelfWrite 打上的过期时间。超过 TTL 后 IsSelfWrite 会返回 false。
const defaultWriteGateTTL = 2 * time.Second

// MarkSelfWrite 给 path 打上"刚由 app 写入"的标记，TTL 窗口内的 fsnotify 事件会被跳过。
// 重复调用会刷新时间戳。
func (g *WriteGate) MarkSelfWrite(path string) {
	g.mu.Lock()
	g.paths[path] = time.Now()
	g.mu.Unlock()
}

// IsSelfWrite 在 TTL 窗口内匹配到过 path 时返回 true，并顺便清掉该条目（每次 mark 对应一次吞掉）。
// 命中即消费，避免残留标记误杀后续合法事件。
func (g *WriteGate) IsSelfWrite(path string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	t, ok := g.paths[path]
	if !ok {
		return false
	}
	delete(g.paths, path)
	return time.Since(t) < defaultWriteGateTTL
}

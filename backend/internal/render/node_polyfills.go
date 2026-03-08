package render

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/dop251/goja"
)

// SetupNodePolyfills 注入 Node.js 核心模块模拟
// baseDir: 模拟的 process.cwd()，通常是主题目录
func SetupNodePolyfills(vm *goja.Runtime, baseDir string) {
	// 确保 baseDir 是绝对路径且干净的
	if !filepath.IsAbs(baseDir) {
		if abs, err := filepath.Abs(baseDir); err == nil {
			baseDir = abs
		}
	}
	baseDir = filepath.Clean(baseDir)

	// Helper to resolve paths relative to baseDir if they are relative
	// And enforce that they do not escape baseDir
	resolvePath := func(p string) (string, error) {
		// 1. Resolve to absolute path
		var target string
		if filepath.IsAbs(p) {
			target = filepath.Clean(p)
		} else {
			target = filepath.Join(baseDir, p)
		}

		// 2. Security Check: Prevent Path Traversal
		// Ensure the target path is strictly within baseDir
		// We check if it equals baseDir OR starts with baseDir + Separator
		if target != baseDir && !strings.HasPrefix(target, baseDir+string(os.PathSeparator)) {
			return "", fmt.Errorf("access denied: path escapes theme directory: %s", p)
		}

		return target, nil
	}

	// --- 1. Process Module ---
	processObj := vm.NewObject()
	processObj.Set("cwd", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(baseDir)
	})
	processObj.Set("platform", runtime.GOOS)
	processObj.Set("env", vm.NewObject())
	processObj.Set("argv", []string{})
	processObj.Set("version", "v14.0.0") // Mock version
	vm.Set("process", processObj)

	// --- 2. Console Module ---
	consoleObj := vm.NewObject()
	consoleObj.Set("log", func(call goja.FunctionCall) goja.Value {
		args := make([]interface{}, len(call.Arguments))
		for i, arg := range call.Arguments {
			args[i] = arg.Export()
		}
		slog.Info(fmt.Sprint(args...), "source", "js")
		return goja.Undefined()
	})
	consoleObj.Set("error", func(call goja.FunctionCall) goja.Value {
		args := make([]interface{}, len(call.Arguments))
		for i, arg := range call.Arguments {
			args[i] = arg.Export()
		}
		slog.Error(fmt.Sprint(args...), "source", "js")
		return goja.Undefined()
	})
	consoleObj.Set("warn", func(call goja.FunctionCall) goja.Value {
		args := make([]interface{}, len(call.Arguments))
		for i, arg := range call.Arguments {
			args[i] = arg.Export()
		}
		slog.Warn(fmt.Sprint(args...), "source", "js")
		return goja.Undefined()
	})
	vm.Set("console", consoleObj)

	// --- 3. Path Module ---
	pathObj := vm.NewObject()
	pathObj.Set("join", func(call goja.FunctionCall) goja.Value {
		parts := make([]string, len(call.Arguments))
		for i, arg := range call.Arguments {
			parts[i] = arg.String()
		}
		return vm.ToValue(filepath.Join(parts...))
	})
	pathObj.Set("resolve", func(call goja.FunctionCall) goja.Value {
		// Node.js path.resolve resolves right-to-left
		resolved := ""
		for i := len(call.Arguments) - 1; i >= 0; i-- {
			p := call.Argument(i).String()
			if p == "" {
				continue
			}
			if resolved == "" {
				resolved = p
			} else {
				resolved = filepath.Join(p, resolved)
			}
			if filepath.IsAbs(resolved) {
				return vm.ToValue(filepath.Clean(resolved))
			}
		}
		// If still relative, resolve against baseDir (CWD)
		return vm.ToValue(filepath.Join(baseDir, resolved))
	})
	pathObj.Set("dirname", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(filepath.Dir(call.Argument(0).String()))
	})
	pathObj.Set("basename", func(call goja.FunctionCall) goja.Value {
		path := call.Argument(0).String()
		ext := ""
		if len(call.Arguments) > 1 {
			ext = call.Argument(1).String()
		}
		base := filepath.Base(path)
		if ext != "" && strings.HasSuffix(base, ext) {
			return vm.ToValue(base[:len(base)-len(ext)])
		}
		return vm.ToValue(base)
	})
	pathObj.Set("extname", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(filepath.Ext(call.Argument(0).String()))
	})
	pathObj.Set("isAbsolute", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(filepath.IsAbs(call.Argument(0).String()))
	})
	pathObj.Set("sep", string(filepath.Separator))
	pathObj.Set("delimiter", string(os.PathListSeparator))
	vm.Set("path", pathObj)

	// --- 4. FS Module ---
	fsObj := vm.NewObject()

	fsObj.Set("existsSync", func(call goja.FunctionCall) goja.Value {
		path := call.Argument(0).String()
		safePath, err := resolvePath(path)
		if err != nil {
			return vm.ToValue(false)
		}
		_, err = os.Stat(safePath)
		return vm.ToValue(err == nil)
	})

	fsObj.Set("readFileSync", func(call goja.FunctionCall) goja.Value {
		path := call.Argument(0).String()
		safePath, err := resolvePath(path)
		if err != nil {
			panic(vm.ToValue(fmt.Sprintf("EACCES: permission denied, open '%s'", path)))
		}

		data, err := os.ReadFile(safePath)
		if err != nil {
			panic(vm.ToValue(fmt.Sprintf("ENOENT: no such file or directory, open '%s'", path)))
		}
		// Encoding handling: EJS usually requests 'utf8'. We just always return string for simplicity in templates.
		return vm.ToValue(string(data))
	})

	fsObj.Set("statSync", func(call goja.FunctionCall) goja.Value {
		path := call.Argument(0).String()
		safePath, err := resolvePath(path)
		if err != nil {
			panic(vm.ToValue(fmt.Sprintf("EACCES: permission denied, stat '%s'", path)))
		}

		info, err := os.Stat(safePath)
		if err != nil {
			panic(vm.ToValue(fmt.Sprintf("ENOENT: no such file or directory, stat '%s'", path)))
		}

		stat := vm.NewObject()
		stat.Set("isFile", func(call goja.FunctionCall) goja.Value { return vm.ToValue(!info.IsDir()) })
		stat.Set("isDirectory", func(call goja.FunctionCall) goja.Value { return vm.ToValue(info.IsDir()) })
		return stat
	})

	fsObj.Set("lstatSync", func(call goja.FunctionCall) goja.Value {
		path := call.Argument(0).String()
		safePath, err := resolvePath(path)
		if err != nil {
			panic(vm.ToValue(fmt.Sprintf("EACCES: permission denied, lstat '%s'", path)))
		}

		info, err := os.Lstat(safePath)
		if err != nil {
			panic(vm.ToValue(fmt.Sprintf("ENOENT: no such file or directory, lstat '%s'", path)))
		}
		stat := vm.NewObject()
		stat.Set("isFile", func(call goja.FunctionCall) goja.Value { return vm.ToValue(!info.IsDir()) })
		stat.Set("isDirectory", func(call goja.FunctionCall) goja.Value { return vm.ToValue(info.IsDir()) })
		return stat
	})

	// realpathSync
	fsObj.Set("realpathSync", func(call goja.FunctionCall) goja.Value {
		path := call.Argument(0).String()
		safePath, err := resolvePath(path)
		if err != nil {
			// If denied, potentially return strict error or original path?
			// Panic is safer to stop execution on security violation
			panic(vm.ToValue(fmt.Sprintf("EACCES: permission denied, realpath '%s'", path)))
		}

		resolved, err := filepath.EvalSymlinks(safePath)
		if err != nil {
			return vm.ToValue(safePath)
		}
		// Re-clean and check in case symlink points outside
		resolved = filepath.Clean(resolved)
		if !strings.HasPrefix(resolved, baseDir) {
			panic(vm.ToValue(fmt.Sprintf("EACCES: symlink targets outside theme directory: '%s'", resolved)))
		}

		return vm.ToValue(resolved)
	})

	vm.Set("fs", fsObj)

	// --- 5. Global Require ---
	vm.Set("require", func(call goja.FunctionCall) goja.Value {
		moduleName := call.Argument(0).String()
		switch moduleName {
		case "fs":
			return fsObj
		case "path":
			return pathObj
		case "process":
			return processObj
		case "console":
			return consoleObj
		default:
			return goja.Undefined()
		}
	})
}

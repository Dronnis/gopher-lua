package lua

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

/* load lib {{{ */

var loLoaders = []LGFunction{loLoaderPreload, loLoaderLua}

func loGetPath(env string, defpath string) string {
	path := os.Getenv(env)
	if len(path) == 0 {
		path = defpath
	}
	path = strings.Replace(path, ";;", ";"+defpath+";", -1)
	if os.PathSeparator != '/' {
		dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
		if err != nil {
			panic(err)
		}
		path = strings.Replace(path, "!", dir, -1)
	}
	return path
}

func loFindFile(L *LState, name, pname string) (string, string) {
	name = strings.Replace(name, ".", string(os.PathSeparator), -1)
	lv := L.GetField(L.GetField(L.Get(EnvironIndex), "package"), pname)
	path, ok := lv.(LString)
	if !ok {
		L.RaiseError("package.%s must be a string", pname)
	}
	messages := []string{}
	for _, pattern := range strings.Split(string(path), ";") {
		luapath := strings.Replace(pattern, "?", name, -1)
		if _, err := os.Stat(luapath); err == nil {
			return luapath, ""
		} else {
			messages = append(messages, err.Error())
		}
	}
	return "", strings.Join(messages, "\n\t")
}

func OpenPackage(L *LState) int {
	packagemod := L.RegisterModule(LoadLibName, loFuncs)

	L.SetField(packagemod, "preload", L.NewTable())

	searchers := L.CreateTable(len(loLoaders), 0)
	for i, loader := range loLoaders {
		L.RawSetInt(searchers, i+1, L.NewFunction(loader))
	}
	L.SetField(packagemod, "searchers", searchers)
	L.SetField(L.Get(RegistryIndex), "_SEARCHERS", searchers)

	loaded := L.NewTable()
	L.SetField(packagemod, "loaded", loaded)
	L.SetField(L.Get(RegistryIndex), "_LOADED", loaded)

	// Lua 5.3: package module should be in package.loaded['package']
	L.SetField(loaded, "package", packagemod)

	L.SetField(packagemod, "path", LString(loGetPath(LuaPath, LuaPathDefault)))
	L.SetField(packagemod, "cpath", emptyLString)

	L.SetField(packagemod, "config", LString(LuaDirSep+"\n"+LuaPathSep+
		"\n"+LuaPathMark+"\n"+LuaExecDir+"\n"+LuaIgMark+"\n"))

	L.Push(packagemod)
	return 1
}

var loFuncs = map[string]LGFunction{
	"loadlib":    loLoadLib,
	"seeall":     loSeeAll,
	"searchpath": loSearchPath,
}

func loLoaderPreload(L *LState) int {
	name := L.CheckString(1)
	preload := L.GetField(L.GetField(L.Get(EnvironIndex), "package"), "preload")
	if _, ok := preload.(*LTable); !ok {
		L.RaiseError("package.preload must be a table")
	}
	lv := L.GetField(preload, name)
	if lv == LNil {
		L.Push(LString(fmt.Sprintf("no field package.preload['%s']", name)))
		return 1
	}
	L.Push(lv)
	return 1
}

func loLoaderLua(L *LState) int {
	name := L.CheckString(1)
	path, msg := loFindFile(L, name, "path")
	if len(path) == 0 {
		L.Push(LString(msg))
		return 1
	}
	fn, err1 := L.LoadFile(path)
	if err1 != nil {
		L.RaiseError(err1.Error())
	}
	L.Push(fn)
	L.Push(LString(path)) // Return the file path as second value (Lua 5.3 compatibility)
	return 2
}

func loLoadLib(L *LState) int {
	L.RaiseError("loadlib is not supported")
	return 0
}

func loSeeAll(L *LState) int {
	mod := L.CheckTable(1)
	mt := L.GetMetatable(mod)
	if mt == LNil {
		mt = L.CreateTable(0, 1)
		L.SetMetatable(mod, mt)
	}
	L.SetField(mt, "__index", L.Get(GlobalsIndex))
	return 0
}

// package.searchpath(name, path[, sep[, rep]])
// Searches for the given name in the given path.
func loSearchPath(L *LState) int {
	name := L.CheckString(1)
	path := L.CheckString(2)
	sep := L.OptString(3, ".")
	rep := L.OptString(4, string(os.PathSeparator))

	// Replace separator in name with the replacement character
	name = strings.Replace(name, sep, rep, -1)

	messages := []string{}
	for _, pattern := range strings.Split(string(path), ";") {
		luapath := strings.Replace(pattern, "?", name, -1)
		if _, err := os.Stat(luapath); err == nil {
			L.Push(LString(luapath))
			return 1
		} else {
			messages = append(messages, err.Error())
		}
	}

	// Return nil and error message
	L.Push(LNil)
	L.Push(LString(strings.Join(messages, "\n\t")))
	return 2
}

/* }}} */

//

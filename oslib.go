package lua

import (
	"os"
	"strings"
	"time"
)

var startedAt time.Time

func init() {
	startedAt = time.Now()
}

// Locale categories (Lua 5.3 compatible)
const (
	LocaleCategoryAll      = "all"
	LocaleCategoryCollate  = "collate"
	LocaleCategoryCtype    = "ctype"
	LocaleCategoryMonetary = "monetary"
	LocaleCategoryNumeric  = "numeric"
	LocaleCategoryTime     = "time"
)

// currentLocales stores the current locale settings for each category
var currentLocales = map[string]string{
	LocaleCategoryAll:      "C",
	LocaleCategoryCollate:  "C",
	LocaleCategoryCtype:    "C",
	LocaleCategoryMonetary: "C",
	LocaleCategoryNumeric:  "C",
	LocaleCategoryTime:     "C",
}

// isValidLocale checks if a locale name is potentially valid
// Since Go doesn't have full locale support, we accept any non-empty string
func isValidLocale(locale string) bool {
	// Accept any non-empty locale string
	// Common locale names: C, POSIX, en_US, en_US.UTF-8, ru_RU, ru_RU.UTF-8, etc.
	if locale == "" {
		return false
	}
	// Basic validation: locale names typically contain letters, numbers, underscores, dots, and hyphens
	for _, c := range locale {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '.' || c == '-') {
			return false
		}
	}
	return true
}

// getCategory returns the category string from the argument
func getCategory(L *LState, index int) string {
	if L.GetTop() < index {
		return LocaleCategoryAll
	}
	cat := L.CheckString(index)
	switch strings.ToLower(cat) {
	case "all":
		return LocaleCategoryAll
	case "collate":
		return LocaleCategoryCollate
	case "ctype":
		return LocaleCategoryCtype
	case "monetary":
		return LocaleCategoryMonetary
	case "numeric":
		return LocaleCategoryNumeric
	case "time":
		return LocaleCategoryTime
	default:
		// Invalid category
		return ""
	}
}

// setLocaleForCategory sets the locale for a specific category
// In Go, we can't actually change locale, so we just store it
func setLocaleForCategory(category, locale string) string {
	// If setting "all", update all categories
	if category == LocaleCategoryAll {
		oldLocale := currentLocales[LocaleCategoryAll]
		for cat := range currentLocales {
			currentLocales[cat] = locale
		}
		return oldLocale
	}

	// Setting a specific category
	oldLocale := currentLocales[category]
	currentLocales[category] = locale
	return oldLocale
}

func getIntField(L *LState, tb *LTable, key string, v int) int {
	ret := tb.RawGetString(key)

	switch lv := ret.(type) {
	case LNumber:
		return int(lv.Int64())
	case LString:
		slv := string(lv)
		slv = strings.TrimLeft(slv, " ")
		if strings.HasPrefix(slv, "0") && !strings.HasPrefix(slv, "0x") && !strings.HasPrefix(slv, "0X") {
			// Standard lua interpreter only support decimal and hexadecimal
			slv = strings.TrimLeft(slv, "0")
			if slv == "" {
				return 0
			}
		}
		if num, err := parseNumber(slv); err == nil {
			return int(num.Int64())
		}
	default:
		return v
	}

	return v
}

func getBoolField(L *LState, tb *LTable, key string, v bool) bool {
	ret := tb.RawGetString(key)
	if lb, ok := ret.(LBool); ok {
		return bool(lb)
	}
	return v
}

func OpenOs(L *LState) int {
	osmod := L.RegisterModule(OsLibName, osFuncs)
	L.Push(osmod)
	return 1
}

var osFuncs = map[string]LGFunction{
	"clock":     osClock,
	"difftime":  osDiffTime,
	"execute":   osExecute,
	"exit":      osExit,
	"date":      osDate,
	"getenv":    osGetEnv,
	"remove":    osRemove,
	"rename":    osRename,
	"setenv":    osSetEnv,
	"setlocale": osSetLocale,
	"time":      osTime,
	"tmpname":   osTmpname,
}

func osClock(L *LState) int {
	L.Push(LNumberFloat(float64(time.Now().Sub(startedAt)) / float64(time.Second)))
	return 1
}

func osDiffTime(L *LState) int {
	L.Push(LNumberInt(L.CheckInt64(1) - L.CheckInt64(2)))
	return 1
}

func osExecute(L *LState) int {
	// Lua 5.3: os.execute() without arguments returns true if shell is available
	if L.GetTop() == 0 {
		// On Windows, we assume cmd.exe is always available
		// On Unix-like systems, /bin/sh is assumed available
		L.Push(LTrue)
		return 1
	}
	
	var procAttr os.ProcAttr
	procAttr.Files = []*os.File{os.Stdin, os.Stdout, os.Stderr}
	cmd, args := popenArgs(L.CheckString(1))
	args = append([]string{cmd}, args...)
	process, err := os.StartProcess(cmd, args, &procAttr)
	if err != nil {
		L.Push(LFalse)
		L.Push(LString("error running command"))
		L.Push(LNumberInt(1))
		return 3
	}

	ps, err := process.Wait()
	if err != nil || !ps.Success() {
		L.Push(LFalse)
		L.Push(LString("command failed"))
		L.Push(LNumberInt(1))
		return 3
	}
	L.Push(LTrue)
	return 1
}

func osExit(L *LState) int {
	L.Close()
	os.Exit(L.OptInt(1, 0))
	return 1
}

func osDate(L *LState) int {
	t := time.Now()
	isUTC := false
	cfmt := "%c"
	if L.GetTop() >= 1 {
		cfmt = L.CheckString(1)
		if strings.HasPrefix(cfmt, "!") {
			cfmt = strings.TrimLeft(cfmt, "!")
			isUTC = true
		}
		if L.GetTop() >= 2 {
			t = time.Unix(L.CheckInt64(2), 0)
		}
		if isUTC {
			t = t.UTC()
		}
		if strings.HasPrefix(cfmt, "*t") {
			ret := L.NewTable()
			ret.RawSetString("year", LNumberInt(int64(t.Year())))
			ret.RawSetString("month", LNumberInt(int64(t.Month())))
			ret.RawSetString("day", LNumberInt(int64(t.Day())))
			ret.RawSetString("hour", LNumberInt(int64(t.Hour())))
			ret.RawSetString("min", LNumberInt(int64(t.Minute())))
			ret.RawSetString("sec", LNumberInt(int64(t.Second())))
			ret.RawSetString("wday", LNumberInt(int64(t.Weekday()+1)))
			// TODO yday & dst
			ret.RawSetString("yday", LNumberInt(0))
			ret.RawSetString("isdst", LFalse)
			L.Push(ret)
			return 1
		}
	}
	L.Push(LString(strftime(t, cfmt)))
	return 1
}

func osGetEnv(L *LState) int {
	v := os.Getenv(L.CheckString(1))
	if len(v) == 0 {
		L.Push(LNil)
	} else {
		L.Push(LString(v))
	}
	return 1
}

func osRemove(L *LState) int {
	err := os.Remove(L.CheckString(1))
	if err != nil {
		L.Push(LNil)
		L.Push(LString(err.Error()))
		return 2
	} else {
		L.Push(LTrue)
		return 1
	}
}

func osRename(L *LState) int {
	err := os.Rename(L.CheckString(1), L.CheckString(2))
	if err != nil {
		L.Push(LNil)
		L.Push(LString(err.Error()))
		return 2
	} else {
		L.Push(LTrue)
		return 1
	}
}

// os.setlocale([locale [, category]])
// Lua 5.3 compatible implementation
func osSetLocale(L *LState) int {
	// Get locale argument (default: empty string = query current locale)
	locale := L.OptString(1, "")

	// Get category argument (default: "all")
	category := getCategory(L, 2)
	if category == "" {
		// Invalid category
		L.ArgError(2, "invalid category")
		return 0
	}

	// If locale is empty string, return current locale for the category
	if locale == "" {
		if category == LocaleCategoryAll {
			// Return the "all" locale
			L.Push(LString(currentLocales[LocaleCategoryAll]))
		} else {
			L.Push(LString(currentLocales[category]))
		}
		return 1
	}

	// Validate locale name
	if !isValidLocale(locale) {
		L.Push(LNil)
		L.Push(LString("invalid locale name"))
		return 2
	}

	// Set the locale for the specified category
	oldLocale := setLocaleForCategory(category, locale)

	// Return the previous locale for the category
	L.Push(LString(oldLocale))
	return 1
}

func osSetEnv(L *LState) int {
	err := os.Setenv(L.CheckString(1), L.CheckString(2))
	if err != nil {
		L.Push(LNil)
		L.Push(LString(err.Error()))
		return 2
	} else {
		L.Push(LTrue)
		return 1
	}
}

func osTime(L *LState) int {
	if L.GetTop() == 0 {
		L.Push(LNumberInt(time.Now().Unix()))
	} else {
		lv := L.CheckAny(1)
		if lv == LNil {
			L.Push(LNumberInt(time.Now().Unix()))
		} else {
			tbl, ok := lv.(*LTable)
			if !ok {
				L.TypeError(1, LTTable)
			}
			sec := getIntField(L, tbl, "sec", 0)
			min := getIntField(L, tbl, "min", 0)
			hour := getIntField(L, tbl, "hour", 12)
			day := getIntField(L, tbl, "day", -1)
			month := getIntField(L, tbl, "month", -1)
			year := getIntField(L, tbl, "year", -1)
			isdst := getBoolField(L, tbl, "isdst", false)
			t := time.Date(year, time.Month(month), day, hour, min, sec, 0, time.Local)
			// TODO dst
			if false {
				print(isdst)
			}
			L.Push(LNumberInt(t.Unix()))
		}
	}
	return 1
}

func osTmpname(L *LState) int {
	file, err := os.CreateTemp("", "")
	if err != nil {
		L.RaiseError("unable to generate a unique filename")
	}
	file.Close()
	os.Remove(file.Name()) // ignore errors
	L.Push(LString(file.Name()))
	return 1
}

//

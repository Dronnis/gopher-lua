package lua

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yuin/gopher-lua/parse"
)

// Increased memory limit for constructs.lua short-circuit test
const maxMemory = 1024

var gluaTests []string = []string{
	"base.lua",
	//	"coroutine.lua",
	//	"db.lua",
	"issues.lua",
	"os.lua",
	"table.lua",
	"vm.lua",
	// "strings.lua",
	// "goto.lua",
}

var luaTests []string = []string{

	//"all.lua",
	"attrib.lua",
	"big.lua",
	"bitwise.lua",
	"bwcoercion.lua",
	"calls.lua",
	"closure.lua",
	"constructs.lua",
	"coroutine.lua",
	"db.lua",
	"errors.lua",
	"events.lua",
	"files.lua",
	//	"gc.lua",
	"goto.lua",
	//	"heavy.lua",
	"literals.lua",
	"locals.lua",
	"main.lua",
	"math.lua",
	"nextvar.lua",
	"pm.lua",
	"sort.lua",
	"strings.lua",
	"tpack.lua",
	"utf8.lua",
	"vararg.lua",
	"verybig.lua",
}

func testScriptCompile(t *testing.T, script string) {
	file, err := os.Open(script)
	if err != nil {
		t.Fatal(err)
		return
	}
	chunk, err2 := parse.Parse(file, script)
	if err2 != nil {
		t.Fatal(err2)
		return
	}
	parse.Dump(chunk)
	proto, err3 := Compile(chunk, script)
	if err3 != nil {
		t.Fatal(err3)
		return
	}
	nop := func(s string) {}
	nop(proto.String())
}

func testScriptDir(t *testing.T, tests []string, directory string) {
	// Get absolute path to ensure we change to the correct directory
	// even when go test runs from a temp directory
	absDir, err := filepath.Abs(directory)
	if err != nil {
		t.Fatal(err)
	}

	// Save current directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	// Change to the test directory
	if err := os.Chdir(absDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir) // Restore original directory

	for _, script := range tests {
		fmt.Printf("testing %s/%s\n", directory, script)
		testScriptCompile(t, script)
		L := NewState(Options{
			RegistrySize:        1024 * 20,
			CallStackSize:       1024,
			IncludeGoStackTrace: true,
		})
		L.SetMx(maxMemory)
		L.OpenLibs()
		// Register T module for Lua 5.3 tests
		// Call OpenTest to get the T table and set it as global
		L.Push(L.NewFunction(OpenTest))
		if err := L.PCall(0, 1, nil); err != nil {
			t.Fatal(err)
		}
		L.SetGlobal("T", L.Get(-1))
		L.Pop(1)

		// Set _soft mode for constructs.lua to reduce test combinations
		if script == "constructs.lua" {
			L.SetGlobal("_soft", LTrue)
		}

		// Set _port mode for main.lua to skip platform-specific shell tests
		// main.lua tests the stand-alone interpreter by running shell commands
		if script == "main.lua" {
			L.SetGlobal("_port", LTrue)
		}

		// Set arg and _ARG tables for Lua 5.3 compatibility (used by main.lua and other tests)
		// Lua 5.3: arg table has:
		//   arg[0] = program name
		//   arg[1], arg[2], ... = script arguments
		//   arg[-1], arg[-2], ... = interpreter options
		argtb := L.NewTable()
		argtb.RawSetInt(0, LString("lua"))   // Program name at index 0
		argtb.RawSetInt(-1, LString(script)) // Script name at -1 (like lua command)
		L.SetGlobal("arg", argtb)
		L.SetGlobal("_ARG", argtb)

		// Force GC before running memory-intensive tests
		if script == "constructs.lua" || script == "heavy.lua" || script == "verybig.lua" || script == "strings.lua" {
			runtime.GC()
		}
		L.SetMx(maxMemory)

		if err := L.DoFile(script); err != nil {
			t.Error(err)
		}
		L.Close()

		// Force GC after running memory-intensive tests
		if script == "constructs.lua" || script == "heavy.lua" || script == "verybig.lua" || script == "strings.lua" {
			runtime.GC()
		}
	}
}

var numActiveUserDatas int32 = 0

type finalizerStub struct{ x byte }

func allocFinalizerUserData(L *LState) int {
	ud := L.NewUserData()
	atomic.AddInt32(&numActiveUserDatas, 1)
	a := finalizerStub{}
	ud.Value = &a
	runtime.SetFinalizer(&a, func(aa *finalizerStub) {
		atomic.AddInt32(&numActiveUserDatas, -1)
	})
	L.Push(ud)
	return 1
}

func sleep(L *LState) int {
	time.Sleep(time.Duration(L.CheckInt(1)) * time.Millisecond)
	return 0
}

func countFinalizers(L *LState) int {
	L.Push(LNumberInt(int64(numActiveUserDatas)))
	return 1
}

// TestLocalVarFree verifies that tables and user user datas which are no longer referenced by the lua script are
// correctly gc-ed. There was a bug in gopher lua where local vars were not being gc-ed in all circumstances.
func TestLocalVarFree(t *testing.T) {
	s := `
		function Test(a, b, c)
			local a = { v = allocFinalizer() }
			local b = { v = allocFinalizer() }
			return a
		end
		Test(1,2,3)
		for i = 1, 100 do
			collectgarbage()
			if countFinalizers() == 0 then
				return
			end
			sleep(100)
		end
		error("user datas not finalized after 100 gcs")
`
	L := NewState()
	L.SetGlobal("allocFinalizer", L.NewFunction(allocFinalizerUserData))
	L.SetGlobal("sleep", L.NewFunction(sleep))
	L.SetGlobal("countFinalizers", L.NewFunction(countFinalizers))
	defer L.Close()
	if err := L.DoString(s); err != nil {
		t.Error(err)
	}
}

func TestGlua(t *testing.T) {
	testScriptDir(t, gluaTests, "_glua-tests")
}

func TestLua(t *testing.T) {
	testScriptDir(t, luaTests, "_lua5.3-tests")
}

func TestUnaryMinusAfterPower(t *testing.T) {
	// Test expression: 2.0^-2 == 1/4 and -2^- -2 == - - -4
	s := `
		local a = 2.0^-2
		local b = 1/4
		assert(a == b, "2.0^-2 should equal 1/4")
		
		local c = -2^- -2
		local d = - - -4
		assert(c == d, "-2^- -2 should equal - - -4")
	`
	L := NewState()
	defer L.Close()
	if err := L.DoString(s); err != nil {
		t.Error(err)
	}
}

func TestMergingLoadNilBug(t *testing.T) {
	// there was a bug where a multiple load nils were being incorrectly merged, and the following code exposed it
	s := `
    function test()
        local a = 0
        local b = 1
        local c = 2
        local d = 3
        local e = 4		-- reg 4
        local f = 5
        local g = 6
        local h = 7

        if e == 4 then
            e = nil		-- should clear reg 4, but clears regs 4-8 by mistake
        end
        if f == nil then
            error("bad f")
        end
        if g == nil then
            error("bad g")
        end
        if h == nil then
            error("bad h")
        end
    end

    test()
`

	L := NewState()
	defer L.Close()
	if err := L.DoString(s); err != nil {
		t.Error(err)
	}
}

func TestMergingLoadNil(t *testing.T) {
	// multiple nil assignments to consecutive registers should be merged
	s := `
		function test()
			local a = 0
			local b = 1
			local c = 2

			-- this should generate just one LOADNIL byte code instruction
			a = nil
			b = nil
			c = nil

			print(a,b,c)
		end

		test()`

	chunk, err := parse.Parse(strings.NewReader(s), "test")
	if err != nil {
		t.Fatal(err)
	}

	compiled, err := Compile(chunk, "test")
	if err != nil {
		t.Fatal(err)
	}

	if len(compiled.FunctionPrototypes) != 1 {
		t.Fatal("expected 1 function prototype")
	}

	// there should be exactly 1 LOADNIL instruction in the byte code generated for the above
	// anymore, and the LOADNIL merging is not working correctly
	count := 0
	for _, instr := range compiled.FunctionPrototypes[0].Code {
		if opGetOpCode(instr) == OP_LOADNIL {
			count++
		}
	}

	if count != 1 {
		t.Fatalf("expected 1 LOADNIL instruction, found %d", count)
	}
}

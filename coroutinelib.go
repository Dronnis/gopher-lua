package lua

func OpenCoroutine(L *LState) int {
	// TODO: Tie module name to contents of linit.go?
	mod := L.RegisterModule(CoroutineLibName, coFuncs)
	L.Push(mod)
	return 1
}

var coFuncs = map[string]LGFunction{
	"create":      coCreate,
	"isyieldable": coIsYieldable,
	"resume":      coResume,
	"running":     coRunning,
	"status":      coStatus,
	"wrap":        coWrap,
	"yield":       coYield,
}

func coCreate(L *LState) int {
	fn := L.CheckFunction(1)
	newthread, _ := L.NewThread()
	base := 0
	newthread.stack.Push(callFrame{
		Fn:         fn,
		Pc:         0,
		Base:       base,
		LocalBase:  base + 1,
		ReturnBase: base,
		NArgs:      0,
		NRet:       MultRet,
		Parent:     nil,
		TailCall:   0,
	})
	L.Push(newthread)
	return 1
}

// coroutine.isyieldable([thread])
// Returns true if the given thread can yield.
// In Lua 5.3+, a thread can yield if it's not the main thread and not dead.
func coIsYieldable(L *LState) int {
	var th *LState
	if L.GetTop() == 0 {
		// No argument: check current thread
		th = L
	} else {
		th = L.CheckThread(1)
	}

	// The main thread cannot yield
	if th == L.G.MainThread {
		L.Push(LFalse)
		return 1
	}

	// Dead threads cannot yield
	if th.Dead {
		L.Push(LFalse)
		return 1
	}

	// A suspended/resumed thread can always yield
	// (unlike the running main thread)
	L.Push(LTrue)
	return 1
}

func coYield(L *LState) int {
	// Check if we're in the main thread or not in a coroutine context
	if L.Parent == nil {
		L.raiseError(1, "can not yield from outside of a coroutine")
		return 0
	}
	// Check if we're trying to yield across a C boundary
	if L.nCcalls > 0 {
		L.raiseError(1, "attempt to yield across metamethod/c-call boundary")
		return 0
	}
	return -1
}

func coResume(L *LState) int {
	th := L.CheckThread(1)
	if L.G.CurrentThread == th {
		msg := "can not resume a running thread"
		if th.wrapped {
			L.RaiseError(msg)
			return 0
		}
		L.Push(LFalse)
		L.Push(LString(msg))
		return 2
	}
	if th.Dead {
		msg := "can not resume a dead thread"
		if th.wrapped {
			L.RaiseError(msg)
			return 0
		}
		L.Push(LFalse)
		L.Push(LString(msg))
		return 2
	}
	th.Parent = L
	L.G.CurrentThread = th
	if !th.isStarted() {
		cf := th.stack.Last()
		th.currentFrame = cf
		th.SetTop(0)
		nargs := L.GetTop() - 1
		L.XMoveTo(th, nargs)
		cf.NArgs = nargs
		th.initCallFrame(cf)
		th.Panic = panicWithoutTraceback
	} else {
		nargs := L.GetTop() - 1
		L.XMoveTo(th, nargs)
	}
	top := L.GetTop()
	threadRun(th)
	return L.GetTop() - top
}

func coRunning(L *LState) int {
	if L.G.MainThread == L {
		L.Push(L.G.MainThread)
		L.Push(LTrue)
		return 2
	}
	L.Push(L.G.CurrentThread)
	L.Push(LFalse)
	return 2
}

func coStatus(L *LState) int {
	L.Push(LString(L.Status(L.CheckThread(1))))
	return 1
}

func wrapaux(L *LState) int {
	L.Insert(L.ToThread(UpvalueIndex(1)), 1)
	return coResume(L)
}

func coWrap(L *LState) int {
	coCreate(L)
	L.CheckThread(L.GetTop()).wrapped = true
	v := L.Get(L.GetTop())
	L.Pop(1)
	L.Push(L.NewClosure(wrapaux, v))
	return 1
}

//

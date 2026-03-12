package lua

import (
	"fmt"
)

/*
  GopherLua VM opcodes - Lua 5.3 compatible.
  
  Lua 5.3 instruction format (32-bit):
  
  +---------------------------------------------+
  |0-5(6bits)|6-13(8bit)|14-22(9bit)|23-31(9bit)|
  |==========+==========+===========+===========|
  |  opcode  |    A     |     C     |    B      |  ABC type
  |----------+----------+-----------+-----------|
  |  opcode  |    A     |      Bx(unsigned)     |  ABx type
  |----------+----------+-----------+-----------|
  |  opcode  |    A     |      sBx(signed)      |  ASbx type
  |----------+----------+-----------+-----------|
  |  opcode  |    A                     | Ax   |  Ax type
  +---------------------------------------------+
*/

const opInvalidInstruction = ^uint32(0)

const (
	opSizeCode = 6
	opSizeA    = 8
	opSizeB    = 9
	opSizeC    = 9
	opSizeBx   = 18
	opSizeSbx  = 18
	opSizeAx   = 26
)

const (
	opMaxArgsA  = (1 << opSizeA) - 1
	opMaxArgsB  = (1 << opSizeB) - 1
	opMaxArgsC  = (1 << opSizeC) - 1
	opMaxArgBx  = (1 << opSizeBx) - 1
	opMaxArgSbx = opMaxArgBx >> 1
	opMaxArgAx  = (1 << opSizeAx) - 1
)

// Opcodes Lua 5.3
const (
	OP_MOVE      int = iota /* A B     R(A) := R(B) */
	OP_MOVEN                /* A B C   R(A) := R(B); followed by C MOVE ops */
	OP_LOADK                /* A Bx    R(A) := Kst(Bx) */
	OP_LOADKX               /* A       R(A) := Kst(extra arg) */
	OP_LOADBOOL             /* A B C   R(A) := (Bool)B; if (C) pc++ */
	OP_LOADNIL              /* A B     R(A) := ... := R(B) := nil */
	OP_GETUPVAL             /* A B     R(A) := UpValue[B] */
	OP_SETUPVAL             /* A B     UpValue[B] := R(A) */

	OP_GETTABUP  /* A B C   R(A) := UpValue[B][RK(C)] */
	OP_GETTABLE  /* A B C   R(A) := R(B)[RK(C)] */
	OP_GETTABLEKS /* A B C  R(A) := R(B)[RK(C)] ; RK(C) is constant string */

	OP_SETTABUP  /* A B C   UpValue[A][RK(B)] := RK(C) */
	OP_SETTABLE  /* A B C   R(A)[RK(B)] := RK(C) */
	OP_SETTABLEKS /* A B C  R(A)[RK(B)] := RK(C) ; RK(B) is constant string */

	OP_NEWTABLE /* A B C   R(A) := {} (size = BC) */

	OP_SELF /* A B C   R(A+1) := R(B); R(A) := R(B)[RK(C)] */

	OP_ADD   /* A B C   R(A) := RK(B) + RK(C) */
	OP_SUB   /* A B C   R(A) := RK(B) - RK(C) */
	OP_MUL   /* A B C   R(A) := RK(B) * RK(C) */
	OP_MOD   /* A B C   R(A) := RK(B) % RK(C) */
	OP_POW   /* A B C   R(A) := RK(B) ^ RK(C) */
	OP_DIV   /* A B C   R(A) := RK(B) / RK(C) */
	OP_IDIV  /* A B C   R(A) := RK(B) // RK(C) */

	OP_BAND /* A B C   R(A) := RK(B) & RK(C) */
	OP_BOR  /* A B C   R(A) := RK(B) | RK(C) */
	OP_BXOR /* A B C   R(A) := RK(B) ~ RK(C) */
	OP_SHL  /* A B C   R(A) := RK(B) << RK(C) */
	OP_SHR  /* A B C   R(A) := RK(B) >> RK(C) */
	OP_BNOT /* A B     R(A) := ~R(B) */
	OP_UNM  /* A B     R(A) := -R(B) */
	OP_NOT  /* A B     R(A) := not R(B) */
	OP_LEN  /* A B     R(A) := #R(B) */

	OP_CONCAT /* A B C   R(A) := R(B).. ... ..R(C) */

	OP_JMP /* A sBx   pc+=sBx; if (A) close upvalues */

	OP_EQ      /* A B C   if ((RK(B) == RK(C)) ~= A) then pc++ */
	OP_LT      /* A B C   if ((RK(B) <  RK(C)) ~= A) then pc++ */
	OP_LE      /* A B C   if ((RK(B) <= RK(C)) ~= A) then pc++ */

	OP_TEST    /* A C     if not (R(A) <=> C) then pc++ */
	OP_TESTSET /* A B C   if (R(B) <=> C) then R(A) := R(B) else pc++ */

	OP_CALL     /* A B C   R(A) ... R(A+C-2) := R(A)(R(A+1) ... R(A+B-1)) */
	OP_TAILCALL /* A B C   return R(A)(R(A+1) ... R(A+B-1)) */
	OP_RETURN   /* A B     return R(A) ... R(A+B-2) */

	OP_FORLOOP  /* A sBx   R(A)+=R(A+2); if R(A) <?= R(A+1) then { pc+=sBx; R(A+3)=R(A) } */
	OP_FORPREP  /* A sBx   R(A)-=R(A+2); pc+=sBx */
	OP_TFORCALL /* A C     R(A+3) ... R(A+2+C) := R(A)(R(A+1) R(A+2)) */
	OP_TFORLOOP /* A C     if R(A+3) ~= nil then R(A+2)=R(A+3) else pc++ */

	OP_SETLIST /* A B C   R(A)[(C-1)*FPF+i] := R(A+i) 1 <= i <= B */

	OP_CLOSE   /* A       close all variables in the stack up to (>=) R(A) */
	OP_CLOSURE /* A Bx    R(A) := closure(KPROTO[Bx]) */

	OP_VARARG /* A B     R(A) ... R(A+B-1) = vararg */

	OP_EXTRAARG /* Ax      extra (larger) argument for previous opcode */

	OP_NOP /* NOP */
)

const opCodeMax = OP_NOP

// Argument modes
type opArgMode int

const (
	opArgModeN opArgMode = iota // No argument
	opArgModeU                  // Unused
	opArgModeR                  // Register
	opArgModeK                  // K (constant)
)

// Instruction types
type opType int

const (
	opTypeABC = iota
	opTypeABx
	opTypeASbx
	opTypeAx
)

// Opcode properties
type opProp struct {
	Name     string
	IsTest   bool
	SetRegA  bool
	ModeArgB opArgMode
	ModeArgC opArgMode
	Type     opType
}

var opProps = []opProp{
	opProp{"MOVE", false, true, opArgModeR, opArgModeN, opTypeABC},
	opProp{"MOVEN", false, true, opArgModeR, opArgModeN, opTypeABC},
	opProp{"LOADK", false, true, opArgModeK, opArgModeN, opTypeABx},
	opProp{"LOADKX", false, true, opArgModeN, opArgModeN, opTypeABC},
	opProp{"LOADBOOL", false, true, opArgModeU, opArgModeU, opTypeABC},
	opProp{"LOADNIL", false, true, opArgModeR, opArgModeN, opTypeABC},
	opProp{"GETUPVAL", false, true, opArgModeU, opArgModeN, opTypeABC},
	opProp{"SETUPVAL", false, false, opArgModeU, opArgModeN, opTypeABC},
	opProp{"GETTABUP", false, true, opArgModeR, opArgModeK, opTypeABC},
	opProp{"GETTABLE", false, true, opArgModeR, opArgModeK, opTypeABC},
	opProp{"GETTABLEKS", false, true, opArgModeR, opArgModeK, opTypeABC},
	opProp{"SETTABUP", false, false, opArgModeK, opArgModeK, opTypeABC},
	opProp{"SETTABLE", false, false, opArgModeK, opArgModeK, opTypeABC},
	opProp{"SETTABLEKS", false, false, opArgModeK, opArgModeK, opTypeABC},
	opProp{"NEWTABLE", false, true, opArgModeU, opArgModeU, opTypeABC},
	opProp{"SELF", false, true, opArgModeR, opArgModeK, opTypeABC},
	opProp{"ADD", false, true, opArgModeK, opArgModeK, opTypeABC},
	opProp{"SUB", false, true, opArgModeK, opArgModeK, opTypeABC},
	opProp{"MUL", false, true, opArgModeK, opArgModeK, opTypeABC},
	opProp{"MOD", false, true, opArgModeK, opArgModeK, opTypeABC},
	opProp{"POW", false, true, opArgModeK, opArgModeK, opTypeABC},
	opProp{"DIV", false, true, opArgModeK, opArgModeK, opTypeABC},
	opProp{"IDIV", false, true, opArgModeK, opArgModeK, opTypeABC},
	opProp{"BAND", false, true, opArgModeK, opArgModeK, opTypeABC},
	opProp{"BOR", false, true, opArgModeK, opArgModeK, opTypeABC},
	opProp{"BXOR", false, true, opArgModeK, opArgModeK, opTypeABC},
	opProp{"SHL", false, true, opArgModeK, opArgModeK, opTypeABC},
	opProp{"SHR", false, true, opArgModeK, opArgModeK, opTypeABC},
	opProp{"BNOT", false, true, opArgModeR, opArgModeN, opTypeABC},
	opProp{"UNM", false, true, opArgModeR, opArgModeN, opTypeABC},
	opProp{"NOT", false, true, opArgModeR, opArgModeN, opTypeABC},
	opProp{"LEN", false, true, opArgModeR, opArgModeN, opTypeABC},
	opProp{"CONCAT", false, true, opArgModeR, opArgModeR, opTypeABC},
	opProp{"JMP", false, false, opArgModeR, opArgModeN, opTypeASbx},
	opProp{"EQ", true, false, opArgModeK, opArgModeK, opTypeABC},
	opProp{"LT", true, false, opArgModeK, opArgModeK, opTypeABC},
	opProp{"LE", true, false, opArgModeK, opArgModeK, opTypeABC},
	opProp{"TEST", true, true, opArgModeR, opArgModeU, opTypeABC},
	opProp{"TESTSET", true, true, opArgModeR, opArgModeU, opTypeABC},
	opProp{"CALL", false, true, opArgModeU, opArgModeU, opTypeABC},
	opProp{"TAILCALL", false, true, opArgModeU, opArgModeU, opTypeABC},
	opProp{"RETURN", false, false, opArgModeU, opArgModeN, opTypeABC},
	opProp{"FORLOOP", false, true, opArgModeR, opArgModeN, opTypeASbx},
	opProp{"FORPREP", false, true, opArgModeR, opArgModeN, opTypeASbx},
	opProp{"TFORCALL", false, true, opArgModeN, opArgModeU, opTypeABC},
	opProp{"TFORLOOP", true, false, opArgModeN, opArgModeU, opTypeABC},
	opProp{"SETLIST", false, false, opArgModeU, opArgModeU, opTypeABC},
	opProp{"CLOSE", false, false, opArgModeN, opArgModeN, opTypeABC},
	opProp{"CLOSURE", false, true, opArgModeU, opArgModeN, opTypeABx},
	opProp{"VARARG", false, true, opArgModeU, opArgModeN, opTypeABC},
	opProp{"EXTRAARG", false, false, opArgModeN, opArgModeN, opTypeAx},
	opProp{"NOP", false, false, opArgModeR, opArgModeN, opTypeASbx},
}

// Instruction encoding/decoding
func opGetOpCode(inst uint32) int {
	return int(inst >> 26)
}

func opSetOpCode(inst *uint32, opcode int) {
	*inst = (*inst & 0x3ffffff) | uint32(opcode<<26)
}

func opGetArgA(inst uint32) int {
	return int(inst>>18) & 0xff
}

func opSetArgA(inst *uint32, arg int) {
	*inst = (*inst & 0xfc03ffff) | uint32((arg&0xff)<<18)
}

func opGetArgB(inst uint32) int {
	return int(inst & 0x1ff)
}

func opSetArgB(inst *uint32, arg int) {
	*inst = (*inst & 0xfffffe00) | uint32(arg&0x1ff)
}

func opGetArgC(inst uint32) int {
	return int(inst>>9) & 0x1ff
}

func opSetArgC(inst *uint32, arg int) {
	*inst = (*inst & 0xfffc01ff) | uint32((arg&0x1ff)<<9)
}

func opGetArgBx(inst uint32) int {
	return int(inst & 0x3ffff)
}

func opSetArgBx(inst *uint32, arg int) {
	*inst = (*inst & 0xfffc0000) | uint32(arg&0x3ffff)
}

func opGetArgSbx(inst uint32) int {
	return opGetArgBx(inst) - opMaxArgSbx
}

func opSetArgSbx(inst *uint32, arg int) {
	opSetArgBx(inst, arg+opMaxArgSbx)
}

func opGetArgAx(inst uint32) int {
	return int(inst & 0x3ffffff)
}

func opSetArgAx(inst *uint32, arg int) {
	*inst = (*inst & 0xfc000000) | uint32(arg&0x3ffffff)
}

// Instruction creation
func opCreateABC(op int, a int, b int, c int) uint32 {
	var inst uint32 = 0
	opSetOpCode(&inst, op)
	opSetArgA(&inst, a)
	opSetArgB(&inst, b)
	opSetArgC(&inst, c)
	return inst
}

func opCreateABx(op int, a int, bx int) uint32 {
	var inst uint32 = 0
	opSetOpCode(&inst, op)
	opSetArgA(&inst, a)
	opSetArgBx(&inst, bx)
	return inst
}

func opCreateASbx(op int, a int, sbx int) uint32 {
	var inst uint32 = 0
	opSetOpCode(&inst, op)
	opSetArgA(&inst, a)
	opSetArgSbx(&inst, sbx)
	return inst
}

func opCreateAx(op int, a int, ax int) uint32 {
	var inst uint32 = 0
	opSetOpCode(&inst, op)
	opSetArgA(&inst, a)
	opSetArgAx(&inst, ax)
	return inst
}

// RK mode constants
const opBitRk = 1 << (opSizeB - 1)
const opMaxIndexRk = opBitRk - 1

func opIsK(value int) bool {
	return (value & opBitRk) != 0
}

func opIndexK(value int) int {
	return value & ^opBitRk
}

func opRkAsk(value int) int {
	return value | opBitRk
}

// String representation
func opToString(inst uint32) string {
	op := opGetOpCode(inst)
	if op > opCodeMax {
		return ""
	}
	prop := &opProps[op]

	arga := opGetArgA(inst)
	argb := opGetArgB(inst)
	argc := opGetArgC(inst)
	argbx := opGetArgBx(inst)
	argsbx := opGetArgSbx(inst)
	argax := opGetArgAx(inst)

	var buf string
	switch prop.Type {
	case opTypeABC:
		buf = fmt.Sprintf("%-10s |  %3d, %3d, %3d", prop.Name, arga, argb, argc)
	case opTypeABx:
		buf = fmt.Sprintf("%-10s |  %3d, %5d", prop.Name, arga, argbx)
	case opTypeASbx:
		buf = fmt.Sprintf("%-10s |  %3d, %5d", prop.Name, arga, argsbx)
	case opTypeAx:
		buf = fmt.Sprintf("%-10s |  %3d, %d", prop.Name, arga, argax)
	}

	// Add semantic description
	switch op {
	case OP_MOVE:
		buf += fmt.Sprintf("; R(%v) := R(%v)", arga, argb)
	case OP_MOVEN:
		buf += fmt.Sprintf("; R(%v) := R(%v); followed by %v MOVE ops", arga, argb, argc)
	case OP_LOADK:
		buf += fmt.Sprintf("; R(%v) := Kst(%v)", arga, argbx)
	case OP_LOADKX:
		buf += fmt.Sprintf("; R(%v) := Kst(extra arg)", arga)
	case OP_LOADBOOL:
		buf += fmt.Sprintf("; R(%v) := (Bool)%v; if (%v) pc++", arga, argb, argc)
	case OP_LOADNIL:
		buf += fmt.Sprintf("; R(%v) := ... := R(%v) := nil", arga, argb)
	case OP_GETUPVAL:
		buf += fmt.Sprintf("; R(%v) := UpValue[%v]", arga, argb)
	case OP_SETUPVAL:
		buf += fmt.Sprintf("; UpValue[%v] := R(%v)", argb, arga)
	case OP_GETTABUP:
		buf += fmt.Sprintf("; R(%v) := UpValue[%v][RK(%v)]", arga, argb, argc)
	case OP_GETTABLE:
		buf += fmt.Sprintf("; R(%v) := R(%v)[RK(%v)]", arga, argb, argc)
	case OP_GETTABLEKS:
		buf += fmt.Sprintf("; R(%v) := R(%v)[RK(%v)] ; RK(%v) is constant string", arga, argb, argc, argc)
	case OP_SETTABUP:
		buf += fmt.Sprintf("; UpValue[%v][RK(%v)] := RK(%v)", arga, argb, argc)
	case OP_SETTABLE:
		buf += fmt.Sprintf("; R(%v)[RK(%v)] := RK(%v)", arga, argb, argc)
	case OP_SETTABLEKS:
		buf += fmt.Sprintf("; R(%v)[RK(%v)] := RK(%v) ; RK(%v) is constant string", arga, argb, argc, argb)
	case OP_NEWTABLE:
		buf += fmt.Sprintf("; R(%v) := {} (size = BC)", arga)
	case OP_SELF:
		buf += fmt.Sprintf("; R(%v+1) := R(%v); R(%v) := R(%v)[RK(%v)]", arga, argb, arga, argb, argc)
	case OP_ADD:
		buf += fmt.Sprintf("; R(%v) := RK(%v) + RK(%v)", arga, argb, argc)
	case OP_SUB:
		buf += fmt.Sprintf("; R(%v) := RK(%v) - RK(%v)", arga, argb, argc)
	case OP_MUL:
		buf += fmt.Sprintf("; R(%v) := RK(%v) * RK(%v)", arga, argb, argc)
	case OP_MOD:
		buf += fmt.Sprintf("; R(%v) := RK(%v) %% RK(%v)", arga, argb, argc)
	case OP_POW:
		buf += fmt.Sprintf("; R(%v) := RK(%v) ^ RK(%v)", arga, argb, argc)
	case OP_DIV:
		buf += fmt.Sprintf("; R(%v) := RK(%v) / RK(%v)", arga, argb, argc)
	case OP_IDIV:
		buf += fmt.Sprintf("; R(%v) := RK(%v) // RK(%v)", arga, argb, argc)
	case OP_BAND:
		buf += fmt.Sprintf("; R(%v) := RK(%v) & RK(%v)", arga, argb, argc)
	case OP_BOR:
		buf += fmt.Sprintf("; R(%v) := RK(%v) | RK(%v)", arga, argb, argc)
	case OP_BXOR:
		buf += fmt.Sprintf("; R(%v) := RK(%v) ~ RK(%v)", arga, argb, argc)
	case OP_SHL:
		buf += fmt.Sprintf("; R(%v) := RK(%v) << RK(%v)", arga, argb, argc)
	case OP_SHR:
		buf += fmt.Sprintf("; R(%v) := RK(%v) >> RK(%v)", arga, argb, argc)
	case OP_BNOT:
		buf += fmt.Sprintf("; R(%v) := ~R(%v)", arga, argb)
	case OP_UNM:
		buf += fmt.Sprintf("; R(%v) := -R(%v)", arga, argb)
	case OP_NOT:
		buf += fmt.Sprintf("; R(%v) := not R(%v)", arga, argb)
	case OP_LEN:
		buf += fmt.Sprintf("; R(%v) := #R(%v)", arga, argb)
	case OP_CONCAT:
		buf += fmt.Sprintf("; R(%v) := R(%v).. ... ..R(%v)", arga, argb, argc)
	case OP_JMP:
		buf += fmt.Sprintf("; pc+=%v; if (%v) close upvalues", argsbx, arga)
	case OP_EQ:
		buf += fmt.Sprintf("; if ((RK(%v) == RK(%v)) ~= %v) then pc++", argb, argc, arga)
	case OP_LT:
		buf += fmt.Sprintf("; if ((RK(%v) <  RK(%v)) ~= %v) then pc++", argb, argc, arga)
	case OP_LE:
		buf += fmt.Sprintf("; if ((RK(%v) <= RK(%v)) ~= %v) then pc++", argb, argc, arga)
	case OP_TEST:
		buf += fmt.Sprintf("; if not (R(%v) <=> %v) then pc++", arga, argc)
	case OP_TESTSET:
		buf += fmt.Sprintf("; if (R(%v) <=> %v) then R(%v) := R(%v) else pc++", argb, argc, arga, argb)
	case OP_CALL:
		buf += fmt.Sprintf("; R(%v) ... R(%v+%v-2) := R(%v)(R(%v+1) ... R(%v+%v-1))", arga, arga, argc, arga, arga, arga, argb)
	case OP_TAILCALL:
		buf += fmt.Sprintf("; return R(%v)(R(%v+1) ... R(%v+%v-1))", arga, arga, arga, argb)
	case OP_RETURN:
		buf += fmt.Sprintf("; return R(%v) ... R(%v+%v-2)", arga, arga, argb)
	case OP_FORLOOP:
		buf += fmt.Sprintf("; R(%v)+=R(%v+2); if R(%v) <?= R(%v+1) then { pc+=%v; R(%v+3)=R(%v) }", arga, arga, arga, arga, argsbx, arga, arga)
	case OP_FORPREP:
		buf += fmt.Sprintf("; R(%v)-=R(%v+2); pc+=%v", arga, arga, argsbx)
	case OP_TFORCALL:
		buf += fmt.Sprintf("; R(%v+3) ... R(%v+2+%v) := R(%v)(R(%v+1) R(%v+2))", arga, arga, argc, arga, arga, arga)
	case OP_TFORLOOP:
		buf += fmt.Sprintf("; if R(%v+3) ~= nil then R(%v+2)=R(%v+3) else pc++", arga, arga, arga)
	case OP_SETLIST:
		buf += fmt.Sprintf("; R(%v)[(%v-1)*FPF+i] := R(%v+i) 1 <= i <= %v", arga, argc, arga, argb)
	case OP_CLOSE:
		buf += fmt.Sprintf("; close upvalues >= R(%v)", arga)
	case OP_CLOSURE:
		buf += fmt.Sprintf("; R(%v) := closure(KPROTO[%v])", arga, argbx)
	case OP_VARARG:
		buf += fmt.Sprintf("; R(%v) ... R(%v+%v-1) = vararg", arga, arga, argb)
	case OP_EXTRAARG:
		buf += fmt.Sprintf("; extra arg = %v", argax)
	case OP_NOP:
		// nothing
	}
	return buf
}

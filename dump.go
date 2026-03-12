package lua

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
)

// Lua 5.3 bytecode header constants
const (
	luaSignature = "\x1bLua"
	luaVersion53 = 0x53
	luaFormat    = 0
	luaData      = 4 // endianness (4 = little endian)
	luaIntSize   = 8
	luaSizeTSize = 8
	luaInstructionSize = 4
	luaNumberSize = 8
	luaNumberFormat = 0 // float
)

// dumpProto serializes a FunctionProto to binary format (Lua 5.3 compatible)
func dumpProto(proto *FunctionProto, strip bool) []byte {
	buf := &bytes.Buffer{}

	// Write header
	buf.WriteString(luaSignature)
	buf.WriteByte(luaVersion53)
	buf.WriteByte(luaFormat)
	buf.WriteByte(luaData)
	buf.WriteByte(luaIntSize)
	buf.WriteByte(luaSizeTSize)
	buf.WriteByte(luaInstructionSize)
	buf.WriteByte(luaNumberSize)
	buf.WriteByte(luaNumberFormat)

	// Write function prototype
	writeFunction(buf, proto, strip)

	return buf.Bytes()
}

func writeFunction(buf *bytes.Buffer, proto *FunctionProto, strip bool) {
	writeString(buf, proto.SourceName)
	writeInt32(buf, int32(proto.LineDefined))
	writeInt32(buf, int32(proto.LastLineDefined))
	buf.WriteByte(proto.NumUpvalues)
	buf.WriteByte(proto.NumParameters)
	buf.WriteByte(proto.IsVarArg)
	buf.WriteByte(proto.NumUsedRegisters)
	writeCode(buf, proto.Code)
	writeConstants(buf, proto.Constants)
	writePrototypes(buf, proto.FunctionPrototypes)
	if !strip {
		writeDebugInfo(buf, proto)
	} else {
		writeInt32(buf, 0)
		buf.WriteByte(0)
		buf.WriteByte(0)
	}
}

func writeString(buf *bytes.Buffer, s string) {
	data := []byte(s)
	writeSizeT(buf, uint64(len(data)+1))
	buf.Write(data)
	buf.WriteByte(0)
}

func writeInt32(buf *bytes.Buffer, v int32) {
	binary.Write(buf, binary.LittleEndian, v)
}

func writeSizeT(buf *bytes.Buffer, v uint64) {
	binary.Write(buf, binary.LittleEndian, v)
}

func writeCode(buf *bytes.Buffer, code []uint32) {
	writeInt32(buf, int32(len(code)))
	for _, inst := range code {
		writeInt32(buf, int32(inst))
	}
}

func writeConstants(buf *bytes.Buffer, constants []LValue) {
	writeInt32(buf, int32(len(constants)))
	for _, c := range constants {
		writeConstant(buf, c)
	}
}

func writeConstant(buf *bytes.Buffer, c LValue) {
	switch c.(type) {
	case *LNilType:
		buf.WriteByte(0)
	case LBool:
		v := c.(LBool)
		buf.WriteByte(1)
		if v {
			buf.WriteByte(1)
		} else {
			buf.WriteByte(0)
		}
	case LNumber:
		v := c.(LNumber)
		buf.WriteByte(3)
		if v.IsInteger() {
			writeInt64(buf, v.Int64())
		} else {
			writeFloat64(buf, v.Float64())
		}
	case LString:
		v := c.(LString)
		buf.WriteByte(4)
		writeString(buf, string(v))
	default:
		buf.WriteByte(0)
	}
}

func writeInt64(buf *bytes.Buffer, v int64) {
	binary.Write(buf, binary.LittleEndian, v)
}

func writeFloat64(buf *bytes.Buffer, v float64) {
	binary.Write(buf, binary.LittleEndian, v)
}

func writePrototypes(buf *bytes.Buffer, protos []*FunctionProto) {
	writeInt32(buf, int32(len(protos)))
	for _, p := range protos {
		writeFunction(buf, p, false)
	}
}

func writeDebugInfo(buf *bytes.Buffer, proto *FunctionProto) {
	writeInt32(buf, int32(len(proto.DbgSourcePositions)))
	for _, pos := range proto.DbgSourcePositions {
		writeInt32(buf, int32(pos))
	}
	buf.WriteByte(byte(len(proto.DbgLocals)))
	for _, local := range proto.DbgLocals {
		writeString(buf, local.Name)
		writeInt32(buf, int32(local.StartPc))
		writeInt32(buf, int32(local.EndPc))
	}
	buf.WriteByte(byte(len(proto.DbgUpvalues)))
	for _, name := range proto.DbgUpvalues {
		writeString(buf, name)
	}
}

// undumpProto deserializes binary data to FunctionProto
func undumpProto(L *LState, data []byte) (*FunctionProto, error) {
	buf := bytes.NewReader(data)
	signature := make([]byte, 4)
	if _, err := io.ReadFull(buf, signature); err != nil {
		return nil, errors.New("invalid chunk: cannot read header")
	}
	if string(signature) != luaSignature {
		return nil, errors.New("not a binary chunk")
	}
	version, _ := buf.ReadByte()
	if version != luaVersion53 {
		return nil, errors.New("version mismatch")
	}
	format, _ := buf.ReadByte()
	if format != luaFormat {
		return nil, errors.New("format mismatch")
	}
	endian, _ := buf.ReadByte()
	intSize, _ := buf.ReadByte()
	sizeTSize, _ := buf.ReadByte()
	instSize, _ := buf.ReadByte()
	numberSize, _ := buf.ReadByte()
	numberFormat, _ := buf.ReadByte()
	if intSize != 4 && intSize != 8 {
		return nil, errors.New("unsupported int size")
	}
	if instSize != 4 {
		return nil, errors.New("unsupported instruction size")
	}
	if numberSize != 8 {
		return nil, errors.New("unsupported number size")
	}
	proto, err := readFunction(L, buf, endian, intSize, sizeTSize, numberSize, numberFormat)
	if err != nil {
		return nil, err
	}
	return proto, nil
}

func readFunction(L *LState, buf *bytes.Reader, endian, intSize, sizeTSize, numberSize, numberFormat byte) (*FunctionProto, error) {
	proto := &FunctionProto{}
	var err error
	proto.SourceName, err = readString(buf)
	if err != nil {
		return nil, err
	}
	proto.LineDefined, err = readInt32(buf)
	if err != nil {
		return nil, err
	}
	proto.LastLineDefined, err = readInt32(buf)
	if err != nil {
		return nil, err
	}
	proto.NumUpvalues, err = buf.ReadByte()
	if err != nil {
		return nil, err
	}
	proto.NumParameters, err = buf.ReadByte()
	if err != nil {
		return nil, err
	}
	proto.IsVarArg, err = buf.ReadByte()
	if err != nil {
		return nil, err
	}
	proto.NumUsedRegisters, err = buf.ReadByte()
	if err != nil {
		return nil, err
	}
	proto.Code, err = readCode(buf)
	if err != nil {
		return nil, err
	}
	proto.Constants, err = readConstants(L, buf)
	if err != nil {
		return nil, err
	}
	proto.FunctionPrototypes, err = readPrototypes(L, buf)
	if err != nil {
		return nil, err
	}
	err = readDebugInfo(buf, proto)
	if err != nil {
		return nil, err
	}
	proto.stringConstants = make([]string, 0)
	for _, c := range proto.Constants {
		if s, ok := c.(LString); ok {
			proto.stringConstants = append(proto.stringConstants, string(s))
		}
	}
	return proto, nil
}

func readString(buf *bytes.Reader) (string, error) {
	size, err := readSizeT(buf)
	if err != nil {
		return "", err
	}
	if size == 0 {
		return "", nil
	}
	data := make([]byte, size)
	if _, err := io.ReadFull(buf, data); err != nil {
		return "", err
	}
	if data[len(data)-1] == 0 {
		data = data[:len(data)-1]
	}
	return string(data), nil
}

func readInt32(buf *bytes.Reader) (int, error) {
	var v int32
	err := binary.Read(buf, binary.LittleEndian, &v)
	return int(v), err
}

func readSizeT(buf *bytes.Reader) (uint64, error) {
	var v uint64
	err := binary.Read(buf, binary.LittleEndian, &v)
	return v, err
}

func readCode(buf *bytes.Reader) ([]uint32, error) {
	size, err := readInt32(buf)
	if err != nil {
		return nil, err
	}
	code := make([]uint32, size)
	for i := 0; i < int(size); i++ {
		v, err := readInt32(buf)
		if err != nil {
			return nil, err
		}
		code[i] = uint32(v)
	}
	return code, nil
}

func readConstants(L *LState, buf *bytes.Reader) ([]LValue, error) {
	size, err := readInt32(buf)
	if err != nil {
		return nil, err
	}
	constants := make([]LValue, size)
	for i := 0; i < int(size); i++ {
		c, err := readConstant(L, buf)
		if err != nil {
			return nil, err
		}
		constants[i] = c
	}
	return constants, nil
}

func readConstant(L *LState, buf *bytes.Reader) (LValue, error) {
	tag, err := buf.ReadByte()
	if err != nil {
		return nil, err
	}
	switch tag {
	case 0:
		return LNil, nil
	case 1:
		v, err := buf.ReadByte()
		if err != nil {
			return nil, err
		}
		return LBool(v != 0), nil
	case 3:
		var v int64
		err := binary.Read(buf, binary.LittleEndian, &v)
		if err != nil {
			return nil, err
		}
		return LNumberInt(v), nil
	case 4:
		s, err := readString(buf)
		if err != nil {
			return nil, err
		}
		return LString(s), nil
	default:
		return LNil, nil
	}
}

func readPrototypes(L *LState, buf *bytes.Reader) ([]*FunctionProto, error) {
	size, err := readInt32(buf)
	if err != nil {
		return nil, err
	}
	protos := make([]*FunctionProto, size)
	for i := 0; i < int(size); i++ {
		p, err := readFunction(L, buf, 4, 4, 8, 8, 0)
		if err != nil {
			return nil, err
		}
		protos[i] = p
	}
	return protos, nil
}

func readDebugInfo(buf *bytes.Reader, proto *FunctionProto) error {
	size, err := readInt32(buf)
	if err != nil {
		return err
	}
	proto.DbgSourcePositions = make([]int, size)
	for i := 0; i < int(size); i++ {
		pos, err := readInt32(buf)
		if err != nil {
			return err
		}
		proto.DbgSourcePositions[i] = pos
	}
	numLocals, err := buf.ReadByte()
	if err != nil {
		return err
	}
	proto.DbgLocals = make([]*DbgLocalInfo, numLocals)
	for i := 0; i < int(numLocals); i++ {
		name, err := readString(buf)
		if err != nil {
			return err
		}
		startPc, err := readInt32(buf)
		if err != nil {
			return err
		}
		endPc, err := readInt32(buf)
		if err != nil {
			return err
		}
		proto.DbgLocals[i] = &DbgLocalInfo{
			Name:    name,
			StartPc: startPc,
			EndPc:   endPc,
		}
	}
	numUpvalues, err := buf.ReadByte()
	if err != nil {
		return err
	}
	proto.DbgUpvalues = make([]string, numUpvalues)
	for i := 0; i < int(numUpvalues); i++ {
		name, err := readString(buf)
		if err != nil {
			return err
		}
		proto.DbgUpvalues[i] = name
	}
	return nil
}

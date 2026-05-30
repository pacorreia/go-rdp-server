package guacd

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Instruction represents a Guacamole protocol instruction.
type Instruction struct {
	Opcode string
	Args   []string
}

// Encode converts an instruction into Guacamole wire format.
func (i *Instruction) Encode() string {
	parts := make([]string, 0, len(i.Args)+1)
	parts = append(parts, encodeElement(i.Opcode))
	for _, arg := range i.Args {
		parts = append(parts, encodeElement(arg))
	}
	return strings.Join(parts, ",") + ";"
}

func encodeElement(value string) string {
	return fmt.Sprintf("%d.%s", len(value), value)
}

// Decode parses a single Guacamole instruction from wire format.
func Decode(raw string) (*Instruction, error) {
	if raw == "" {
		return nil, errors.New("empty instruction")
	}
	instr, _, err := decodeSingle(raw)
	return instr, err
}

// DecodeAll parses all Guacamole instructions from a wire-format string.
// The input may contain one or more concatenated complete instructions.
func DecodeAll(raw string) ([]*Instruction, error) {
	var instructions []*Instruction
	for len(raw) > 0 {
		instr, tail, err := decodeSingle(raw)
		if err != nil {
			return nil, err
		}
		instructions = append(instructions, instr)
		raw = tail
	}
	return instructions, nil
}

// decodeSingle parses the first instruction in raw and returns the instruction
// together with any remaining input after the terminating ';'.
func decodeSingle(raw string) (*Instruction, string, error) {
	if raw == "" {
		return nil, "", errors.New("empty instruction")
	}

	values := make([]string, 0, 8)
	idx := 0
	for idx < len(raw) {
		dot := strings.IndexByte(raw[idx:], '.')
		if dot <= 0 {
			return nil, "", errors.New("invalid element length")
		}
		dot += idx

		length, err := strconv.Atoi(raw[idx:dot])
		if err != nil || length < 0 {
			return nil, "", fmt.Errorf("invalid element length %q", raw[idx:dot])
		}

		start := dot + 1
		end := start + length
		if end > len(raw) {
			return nil, "", errors.New("element length exceeds payload")
		}

		values = append(values, raw[start:end])
		if end >= len(raw) {
			return nil, "", errors.New("missing delimiter")
		}

		switch raw[end] {
		case ',':
			idx = end + 1
		case ';':
			if len(values) == 0 || values[0] == "" {
				return nil, "", errors.New("missing opcode")
			}
			return &Instruction{Opcode: values[0], Args: values[1:]}, raw[end+1:], nil
		default:
			return nil, "", fmt.Errorf("invalid delimiter %q", raw[end])
		}
	}

	return nil, "", errors.New("instruction not terminated")
}

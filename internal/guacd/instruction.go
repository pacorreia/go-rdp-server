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
	if raw[len(raw)-1] != ';' {
		return nil, errors.New("instruction missing terminator")
	}

	values := make([]string, 0, 8)
	for idx := 0; idx < len(raw); {
		dot := strings.IndexByte(raw[idx:], '.')
		if dot <= 0 {
			return nil, errors.New("invalid element length")
		}
		dot += idx

		length, err := strconv.Atoi(raw[idx:dot])
		if err != nil || length < 0 {
			return nil, fmt.Errorf("invalid element length %q", raw[idx:dot])
		}

		start := dot + 1
		end := start + length
		if end > len(raw) {
			return nil, errors.New("element length exceeds payload")
		}

		values = append(values, raw[start:end])
		if end >= len(raw) {
			return nil, errors.New("missing delimiter")
		}

		switch raw[end] {
		case ',':
			idx = end + 1
		case ';':
			idx = len(raw)
		default:
			return nil, fmt.Errorf("invalid delimiter %q", raw[end])
		}
	}

	if len(values) == 0 || values[0] == "" {
		return nil, errors.New("missing opcode")
	}

	return &Instruction{Opcode: values[0], Args: values[1:]}, nil
}

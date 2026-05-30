package guacd

import "testing"

func TestInstructionEncodeDecodeRoundTrip(t *testing.T) {
	instruction := &Instruction{
		Opcode: "connect",
		Args:   []string{"hostname", "127.0.0.1", "username", "user"},
	}

	encoded := instruction.Encode()
	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	if decoded.Opcode != instruction.Opcode {
		t.Fatalf("opcode mismatch: got %q want %q", decoded.Opcode, instruction.Opcode)
	}
	if len(decoded.Args) != len(instruction.Args) {
		t.Fatalf("arg length mismatch: got %d want %d", len(decoded.Args), len(instruction.Args))
	}
	for idx := range decoded.Args {
		if decoded.Args[idx] != instruction.Args[idx] {
			t.Fatalf("arg[%d] mismatch: got %q want %q", idx, decoded.Args[idx], instruction.Args[idx])
		}
	}
}

func TestDecodeInvalidInstruction(t *testing.T) {
	if _, err := Decode("7.connect,4.host"); err == nil {
		t.Fatal("expected decode error for unterminated instruction")
	}
}

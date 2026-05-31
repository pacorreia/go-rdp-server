// Package display provides a high-level interface for an RDP display session.
// It abstracts the underlying RDP library, delivering bitmap tile updates on a
// channel and accepting keyboard/mouse input via method calls.
package display

// Tile represents a single bitmap update received from the RDP server.
// Data holds the tile image encoded as JPEG.
type Tile struct {
	X, Y, W, H int
	Data        []byte // JPEG-encoded image bytes
}

// RDPSession abstracts an active RDP display connection.
// All methods are safe for concurrent use after Connect returns.
type RDPSession interface {
	// Tiles returns a channel on which bitmap tile updates are delivered.
	// The channel is closed when the session ends.
	Tiles() <-chan Tile

	// KeyDown sends a key-press event using the given PS/2 scancode.
	KeyDown(scancode int)
	// KeyUp sends a key-release event using the given PS/2 scancode.
	KeyUp(scancode int)

	// MouseMove sends a mouse movement event.
	MouseMove(x, y int)
	// MouseDown sends a mouse button press. button: 0=left, 1=middle, 2=right.
	MouseDown(button, x, y int)
	// MouseUp sends a mouse button release. button: 0=left, 1=middle, 2=right.
	MouseUp(button, x, y int)
	// MouseWheel sends a scroll-wheel event.
	// delta is positive for scroll-up, negative for scroll-down.
	MouseWheel(delta int)

	// Close terminates the RDP session.
	Close()
}

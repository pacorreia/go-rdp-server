module github.com/pacorreia/go-rdp-server

go 1.26.3

require (
	github.com/google/uuid v1.6.0
	github.com/gorilla/websocket v1.5.3
	github.com/nakagami/grdp v0.8.6
	golang.org/x/sys v0.38.0
)

require (
	github.com/lunixbochs/struc v0.0.0-20200707160740-784aaebc1d40 // indirect
	golang.org/x/crypto v0.45.0 // indirect
)

replace github.com/nakagami/grdp v0.8.6 => ./_grdp // patched: PUNPCKLWD/PUNPCKHWD/PSRLD/PSLLD renamed to Go Plan 9 mnemonics

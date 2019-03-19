package shared

import (
	"encoding/binary"
	"log"
	"net"

	"github.com/google/uuid"
)

// echo -n -e "\x0\x0\x0\x017d4448409dc011d1b2455ffdce74fad2\x00\x00\x00\x00\x00\x00\x00\x10\x0\x05abcde" | nc 127.0.0.1 3405

// SetupMessage is the first message sent from the sender to the receiver
type SetupMessage struct {
	NumFiles uint32
	Files    *[]NCFile
}

// FromStream populates the SetupMessage with data from the given stream
func (s *SetupMessage) FromStream(conn *net.Conn) {
	nfb := make([]byte, 4)

	n, err := (*conn).Read(nfb)
	if err != nil {
		log.Fatalf("net-copy/shared: could not read from connection: %v", err)
	}
	if n != 4 {
		log.Fatalf("net-copy/shared: expected 4 bytes but got %d", n)

	}
	(*s).NumFiles = binary.BigEndian.Uint32(nfb)

	nf := make([]NCFile, (*s).NumFiles)
	(*s).Files = &nf

	fidbuff := make([]byte, 32)
	fsbuff := make([]byte, 8)
	nsbuff := make([]byte, 2)
	for i := uint32(0); i < s.NumFiles; i++ {
		n, err := (*conn).Read(fidbuff)
		if err != nil {
			log.Fatalf("net-copy/shared: could not read from connection: %v", err)
		}

		if n != 32 {
			log.Fatalf("net-copy/shared: expected 32 bytes but got %d", n)
		}

		fileID, err := uuid.ParseBytes(fidbuff)
		if err != nil {
			log.Fatalf("net-copy/shared: could not parse ID for NCFile: %v", err)
		}

		// ======================

		n, err = (*conn).Read(fsbuff)
		if err != nil {
			log.Fatalf("net-copy/shared: could not read from connection: %v", err)
		}

		if n != 8 {
			log.Fatalf("net-copy/shared: expected 8 bytes but got %d", n)
		}

		fileSize := binary.BigEndian.Uint64(fsbuff)

		// ======================

		n, err = (*conn).Read(nsbuff)
		if err != nil {
			log.Fatalf("net-copy/shared: could not read from connection: %v", err)
		}

		if n != 2 {
			log.Fatalf("net-copy/shared: expected 2 bytes but got %d", n)
		}

		fileNameSize := binary.BigEndian.Uint16(nsbuff)

		// ======================

		nbuff := make([]byte, fileNameSize)
		n, err = (*conn).Read(nbuff)
		if err != nil {
			log.Fatalf("net-copy/shared: could not read from connection: %v", err)
		}

		if uint16(n) != fileNameSize {
			log.Fatalf("net-copy/shared: expected %d bytes but got %d", fileNameSize, n)
		}

		fileName := string(nbuff)

		(*(*s).Files)[i] = NCFile{
			ID:       fileID,
			FileSize: fileSize,
			NameSize: fileNameSize,
			Name:     fileName,
		}
	}
}

// NCFile describes a file to be sent/received
type NCFile struct {
	ID       uuid.UUID
	FileSize uint64
	NameSize uint16
	Name     string
}

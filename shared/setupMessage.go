package shared

import (
	"github.com/google/uuid"
)

// echo -n -e "\x0\x0\x0\x017d4448409dc011d1b2455ffdce74fad2\x00\x00\x00\x00\x00\x00\x00\x10\x0\x05abcde" | nc 127.0.0.1 3405

// SetupMessage is the first message sent from the sender to the receiver
type SetupMessage struct {
	NumFiles uint32
	Files    *[]NCFile
}

// NCFile describes a file to be sent/received
type NCFile struct {
	ID       uuid.UUID
	FileSize uint64
	Name     string
}

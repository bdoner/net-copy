package ncproto

import (
	"fmt"
	"path/filepath"

	"github.com/google/uuid"
)

// MessageType determine how to handle the incoming message
type MessageType uint8

const (
	// MsgConfig is the initial message being sent from the
	// client to the server.
	MsgConfig MessageType = 0x0

	// MsgFile denotes that the Data contains raw file bytes
	MsgFile MessageType = 0x1

	// MsgClose tells the server that everything is done and it can close the connection
	MsgClose MessageType = 0x2
)

// Config holds configuration for both sender and receiver
type Config struct {
	Hostname         string
	Port             uint16
	WorkingDirectory string
	Threads          uint16
	ConnectionID     uuid.UUID
	ReadBufferSize   uint32
}

// Merge two Config's
// The calling struct is the resulting struct
func (c *Config) Merge(conf Config) {
	if conf.Threads < c.Threads {
		c.Threads = conf.Threads
	}
	c.ConnectionID = conf.ConnectionID
	c.ReadBufferSize = conf.ReadBufferSize
}

// File describes a file to be sent/received
type File struct {
	ID           uuid.UUID
	ConnectionID uuid.UUID
	FileSize     int64
	Name         string
	RelativePath string
}

// FileChunk is the actual file data being sent
type FileChunk struct {
	ID           uuid.UUID
	ConnectionID uuid.UUID
	Data         []byte
}

// FullPath returns the absolute path of where a file should be located on disk according to a given config
func (f *File) FullPath(c *Config) string {
	return filepath.Join(c.WorkingDirectory, f.RelativePath, f.Name)
}

// ConnectionClose closes the connection when sent from client to server
type ConnectionClose struct {
	ConnectionID uuid.UUID
}

// PrettySize returns a human readable file size
func (f *File) PrettySize() string {
	ffs := float64(f.FileSize)
	if 1000000000 < f.FileSize {
		return fmt.Sprintf("%.3fGB", ffs/1000000000.0)
	} else if 1000000 < f.FileSize {
		return fmt.Sprintf("%.3fMB", ffs/1000000.0)
	} else if 1000 < f.FileSize {
		return fmt.Sprintf("%.3fKB", ffs/1000.0)
	}
	return fmt.Sprintf("%.0fB", ffs)

}

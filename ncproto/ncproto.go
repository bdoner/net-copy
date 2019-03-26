package ncproto

import (
	"fmt"
	"io"
	"math"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// MessageType determine how to handle the incoming message
type MessageType uint8

// IMessageType is the interface type that is sent using
// gob encoding from client to server
type IMessageType interface{}

const (
	// MsgConfig is the initial message being sent from the
	// client to the server.
	MsgConfig MessageType = 0x0

	// MsgFile is meta data about the coming file
	MsgFile MessageType = 0x1

	// MsgFileChunk tells the message is of type FileChunk
	MsgFileChunk MessageType = 0x2

	// MsgConnectionClose tells the server that everything is done and it can close the connection
	MsgConnectionClose MessageType = 0x3
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
	c.ConnectionID = conf.ConnectionID
	c.ReadBufferSize = conf.ReadBufferSize
}

// File describes a file to be sent/received
type File struct {
	ID             uuid.UUID
	ConnectionID   uuid.UUID
	FileSize       int64
	Name           string
	RelativePath   string
	FileDescriptor io.WriteCloser
	ChunkQueue     chan FileChunk
	Complete       chan bool
}

// FileChunk is the actual file data being sent
type FileChunk struct {
	ID           uuid.UUID
	ConnectionID uuid.UUID
	Data         []byte
	Seq          int
}

// FileComplete is sent when all chunks have been transfered
type FileComplete struct {
	ID           uuid.UUID
	ConnectionID uuid.UUID
}

// FullFilePath returns the absolute path of where a file should be located on disk according to a given config
func (f *File) FullFilePath(c *Config) string {
	return filepath.Join(c.WorkingDirectory, f.RelativePath, f.Name)
}

// RelativeFilePath gives the path relative to the WorkingDirectory
func (f *File) RelativeFilePath(c *Config) string {
	return filepath.Join(f.RelativePath, f.Name)
}

// GetProgress returns the progress of a file transfer as an ascii bar and a number from 0-100
func (f *File) GetProgress(count, width int, conf *Config) (string, int) {
	cm := int(math.Max(float64(count), 1))
	csm := int(math.Max(float64(f.FileSize/int64(conf.ReadBufferSize)), 1))
	progress := int((float64(cm) / float64(csm)) * 100.0)
	prog := int((float64(progress) / 100.0) * float64(width))
	bar := fmt.Sprintf("%s%s>", strings.Repeat("#", prog), strings.Repeat(" ", width-prog))
	return bar, progress
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

package session

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	schema "github.com/ryanreadbooks/tokkibot/llm/schema"
)

// baseLog contains common fields and methods for log files
type baseLog struct {
	workspace string
	filename  string
	channel   string
	chatId    string
	f         *os.File
}

// closeFile closes the underlying file handle
func (b *baseLog) closeFile() {
	if b.f != nil {
		_ = b.f.Close()
	}
}

// fullLogFileName returns the full path to the log file
// ~/sessions/channel/chatid/filename
func (b *baseLog) fullLogFileName(root string) string {
	return filepath.Join(root, b.channel, b.chatId, b.filename)
}

// writeLine writes a single log item to the file
func (b *baseLog) writeLine(item *LogItem) error {
	if b.f == nil {
		return fmt.Errorf("file not opened")
	}

	str, err := json.Marshal(item)
	if err != nil {
		return err
	}

	str = append(str, '\n')
	_, err = b.f.Write(str)
	return err
}

// initFile initializes the log file with the given flags
func (b *baseLog) initFile(workspace string, flags int) error {
	path := b.fullLogFileName(workspace)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	f, err := os.OpenFile(path, flags, 0644)
	if err != nil {
		return err
	}

	b.f = f
	return nil
}

// createLogItem creates a new log item with the given role and message
func (b *baseLog) createLogItem(role schema.Role, msg *schema.MessageParam) LogItem {
	return LogItem{
		Id:      newLogItemId(),
		Role:    role,
		Created: time.Now().Unix(),
		Message: msg,
	}
}

// readLogItems reads all log items from the given file path using json.Decoder
func readLogItems(path string) ([]LogItem, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to open session file: %w", err)
	}
	defer f.Close()

	// Check if file is empty
	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if stat.Size() == 0 {
		return nil, nil
	}

	items := make([]LogItem, 0, 128)
	decoder := json.NewDecoder(f)
	for {
		var item LogItem
		if err := decoder.Decode(&item); err == io.EOF {
			break
		} else if err != nil {
			continue
		}
		items = append(items, item)
	}

	return items, nil
}

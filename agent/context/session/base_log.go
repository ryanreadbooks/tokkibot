package session

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/ryanreadbooks/tokkibot/llm/schema/param"
)

var regMediaRef = regexp.MustCompile(`\[image\]\((@medias/[^)]+)\)`)

type baseLog struct {
	filename string
	channel  string
	chatId   string
	f        *os.File
}

func (b *baseLog) closeFile() {
	if b.f != nil {
		_ = b.f.Close()
	}
}

func (b *baseLog) logFilePath(root string) string {
	return filepath.Join(root, b.channel, b.chatId, b.filename)
}

func (b *baseLog) writeLine(item *LogItem) error {
	if b.f == nil {
		return fmt.Errorf("file not opened")
	}

	data, err := json.Marshal(item)
	if err != nil {
		return err
	}

	data = append(data, '\n')
	_, err = b.f.Write(data)
	return err
}

func (b *baseLog) initFile(workspace string, flags int) error {
	path := b.logFilePath(workspace)
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

func (b *baseLog) newLogItem(role param.Role, msg *param.Message) LogItem {
	return LogItem{
		Id:      NewLogItemId(),
		Role:    role,
		Created: time.Now().Unix(),
		Message: msg,
	}
}

func readLogItems(path string) ([]LogItem, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to open session file: %w", err)
	}
	defer f.Close()

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

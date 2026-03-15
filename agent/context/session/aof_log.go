package session

import "os"

// AOFLog is an append-only log for the complete conversation history.
type AOFLog struct {
	baseLog
}

func (s *AOFLog) AddLogItem(item LogItem) error {
	return s.writeLine(&item)
}

func (s *AOFLog) RetrieveLogItems(workspace string) ([]LogItem, error) {
	return readLogItems(s.logFilePath(workspace))
}

func (s *AOFLog) initFile(workspace string) error {
	return s.baseLog.initFile(workspace, os.O_CREATE|os.O_APPEND|os.O_RDWR)
}

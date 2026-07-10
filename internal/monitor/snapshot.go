package monitor

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// Snapshot is a case-insensitive set of process image names.
type Snapshot map[string]struct{}

func (s Snapshot) Has(process string) bool {
	_, ok := s[strings.ToLower(strings.TrimSpace(process))]
	return ok
}

// SnapshotProvider is the low-frequency fallback for the event watcher. One
// invocation returns every running process, so its cost does not grow with the
// number of configured games.
type SnapshotProvider interface {
	Running(context.Context) (Snapshot, error)
}

type TasklistSnapshot struct{}

func (TasklistSnapshot) Running(ctx context.Context) (Snapshot, error) {
	command := exec.CommandContext(ctx, "tasklist.exe", "/FO", "CSV", "/NH")
	prepareHiddenProcess(command)
	result, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("读取进程快照: %w", err)
	}
	return parseTasklistCSV(string(result))
}

func parseTasklistCSV(content string) (Snapshot, error) {
	reader := csv.NewReader(strings.NewReader(content))
	reader.FieldsPerRecord = -1
	snapshot := make(Snapshot)
	for {
		record, err := reader.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("解析 tasklist 输出: %w", err)
		}
		if len(record) == 0 || strings.TrimSpace(record[0]) == "" {
			continue
		}
		snapshot[strings.ToLower(strings.TrimSpace(record[0]))] = struct{}{}
	}
	return snapshot, nil
}

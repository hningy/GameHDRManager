package discovery

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/huang/game-hdr-manager/internal/domain"
)

// GameFromCandidate creates a safe, editable default. The candidate is never
// silently added to user configuration; the UI presents it for confirmation.
func GameFromCandidate(candidate Candidate) (domain.Game, error) {
	bytes := make([]byte, 12)
	if _, err := rand.Read(bytes); err != nil {
		return domain.Game{}, fmt.Errorf("生成游戏 ID: %w", err)
	}
	return domain.Game{ID: hex.EncodeToString(bytes), Name: candidate.Name, Enabled: true, Source: candidate.Source,
		Launch: candidate.Launch, Processes: candidate.Processes, InstallPath: candidate.InstallPath,
		HDR: domain.HDRRule{EnableBeforeLaunch: true, RestoreOnExit: true}, ExitConfirmSeconds: 15}, nil
}

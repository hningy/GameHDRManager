package launcher

import (
	"reflect"
	"testing"

	"github.com/huang/game-hdr-manager/internal/domain"
)

type recordedStarter struct {
	name string
	args []string
}

func (s *recordedStarter) Start(name string, args ...string) error {
	s.name, s.args = name, args
	return nil
}

func TestLaunchSteamURI(t *testing.T) {
	recorded := &recordedStarter{}
	if err := New(recorded).Launch(domain.LaunchConfig{Type: domain.LaunchTypeSteamURI, Value: "steam://rungameid/1091500"}); err != nil {
		t.Fatal(err)
	}
	if recorded.name != "rundll32.exe" || !reflect.DeepEqual(recorded.args, []string{"url.dll,FileProtocolHandler", "steam://rungameid/1091500"}) {
		t.Fatalf("unexpected command: %q %#v", recorded.name, recorded.args)
	}
}

func TestLaunchRejectsInvalidURI(t *testing.T) {
	if err := New(&recordedStarter{}).Launch(domain.LaunchConfig{Type: domain.LaunchTypeSteamURI, Value: "https://example.com"}); err == nil {
		t.Fatal("invalid Steam URI should fail")
	}
}

package loader

import (
	"io/fs"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testcontentWith is ../testcontent with one file's JSON array given an
// extra entry -- the copy-paste a builder makes.
func testcontentWith(t *testing.T, file, entry string) fs.FS {
	t.Helper()
	m := fstest.MapFS{}
	require.NoError(t, fs.WalkDir(os.DirFS("../testcontent"), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(os.DirFS("../testcontent"), p)
		m[p] = &fstest.MapFile{Data: b}
		return err
	}))
	body := strings.TrimSpace(string(m[file].Data))
	require.True(t, strings.HasSuffix(body, "]"), file)
	m[file] = &fstest.MapFile{Data: []byte(strings.TrimSuffix(body, "]") + "," + entry + "]")}
	return m
}

// A second entry with an id already in the zone would quietly take the
// first one's place; it's refused.
func TestLoadContent_duplicateIds(t *testing.T) {
	for file, entry := range map[string]string{
		"world/wrathrock/rooms.json":   `{"id": "temple_square", "name": "Copy", "description": "A copy."}`,
		"world/wrathrock/objects.json": `{"id": "rope", "name": "lantern", "category": "OTHER", "short_description": "a lantern", "description_on_ground": "A lantern."}`,
		"world/wrathrock/mobs.json":    `{"id": "rabbit", "name": "wolf", "short_description": "A wolf.", "description_in_room": "A wolf.", "max_health": 10, "ac": 10}`,
	} {
		_, err := LoadContent(testcontentWith(t, file, entry))
		assert.ErrorContains(t, err, "two ", file)
	}
}

// A followPath walks two rooms or more, all of them real.
func TestLoadContent_pathsAreWalkable(t *testing.T) {
	for want, path := range map[string]string{
		"at least two rooms": `["temple_square"]`,
		"nothing has":        `["temple_square", "nowhere"]`,
	} {
		mob := `{"id": "pacer", "name": "pacer", "short_description": "A pacer.", "description_in_room": "A pacer.", "max_health": 10, "ac": 10,
			"wandering_definition": {"can_wander": true, "check_frequency_seconds": 1, "check_percentage": 100, "wander_style": "followPath", "path": ` + path + `}}`
		_, err := LoadContent(testcontentWith(t, "world/wrathrock/mobs.json", mob))
		assert.ErrorContains(t, err, want)
	}
}

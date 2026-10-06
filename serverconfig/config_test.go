package serverconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeConfig(t *testing.T, yaml string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "app.yaml")
	require.NoError(t, os.WriteFile(path, []byte(yaml), 0o600))
	return path
}

const minimal = `
contentPath: ./content
telnet:
  host: 0.0.0.0
  port: 4000
mongo:
  uri: mongodb://from-the-file
  database: watchmud
`

func TestLoad_mongoUriFromTheEnvironment(t *testing.T) {
	t.Setenv("WATCHMUD_MONGO_URI", "mongodb://user:secret@mongo:27017/watchmud")

	cfg, err := Load(writeConfig(t, minimal))
	require.NoError(t, err)
	assert.Equal(t, "mongodb://user:secret@mongo:27017/watchmud", cfg.Mongo.Uri)
	assert.Equal(t, "watchmud", cfg.Mongo.Database, "the rest still comes from the file")
}

func TestLoad_mongoUriFromTheFile(t *testing.T) {
	t.Setenv("WATCHMUD_MONGO_URI", "")

	cfg, err := Load(writeConfig(t, minimal))
	require.NoError(t, err)
	assert.Equal(t, "mongodb://from-the-file", cfg.Mongo.Uri)
}

// The deploy config has to load, or the container won't start.
func TestLoad_theDeployConfig(t *testing.T) {
	cfg, err := Load("../deploy/app.yaml")
	require.NoError(t, err)
	assert.Equal(t, "0.0.0.0", cfg.Telnet.Host, "reachable from outside the container")
	assert.Empty(t, cfg.Log.File, "stdout; docker rotates it")
	assert.Empty(t, cfg.Mongo.Uri, "the uri carries the password: it comes from the environment")
	assert.Equal(t, 4443, cfg.TLS.Port)
	assert.Equal(t, "/app/certs/fullchain.pem", cfg.TLS.Cert, "where compose.yaml mounts deploy/certs")
	assert.Equal(t, "127.0.0.1", cfg.Health.Host, "the healthcheck runs inside the container; nothing outside needs it")
	assert.NotZero(t, cfg.Health.Port, "compose.yaml's healthcheck asks it")
}

func TestLoad_tlsIsOptional(t *testing.T) {
	cfg, err := Load(writeConfig(t, minimal))
	require.NoError(t, err)
	assert.Zero(t, cfg.TLS.Port, "no tls: block, no TLS")
}

func TestLoad_tlsNeedsCertAndKey(t *testing.T) {
	_, err := Load(writeConfig(t, minimal+`
tls:
  port: 4443
  cert: /certs/fullchain.pem
`))
	assert.ErrorContains(t, err, "tls")

	cfg, err := Load(writeConfig(t, minimal+`
tls:
  port: 4443
  cert: /certs/fullchain.pem
  key: /certs/privkey.pem
`))
	require.NoError(t, err)
	assert.Equal(t, 4443, cfg.TLS.Port)
	assert.Equal(t, "/certs/privkey.pem", cfg.TLS.Key)
}

func TestLoad_healthIsOptional(t *testing.T) {
	cfg, err := Load(writeConfig(t, minimal))
	require.NoError(t, err)
	assert.Zero(t, cfg.Health.Port, "no health: block, no health port")
}

func TestLoad_healthDefaultsToLoopback(t *testing.T) {
	cfg, err := Load(writeConfig(t, minimal+`
health:
  port: 4080
`))
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1", cfg.Health.Host)
}

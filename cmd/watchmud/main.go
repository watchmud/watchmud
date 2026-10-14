package main

import (
	"context"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/watchmud/watchmud/dice"
	"github.com/watchmud/watchmud/loader"
	"github.com/watchmud/watchmud/logging"
	"github.com/watchmud/watchmud/memstore"
	"github.com/watchmud/watchmud/mongostore"
	"github.com/watchmud/watchmud/player"
	"github.com/watchmud/watchmud/report"
	"github.com/watchmud/watchmud/server"
	"github.com/watchmud/watchmud/serverconfig"
	"github.com/watchmud/watchmud/telnet"
	"github.com/watchmud/watchmud/world"
	"github.com/watchmud/watchmud/writebehind"
)

// version is what's running: a release tag (v1.2.3), a branch and commit, or
// "dev" for a build that wasn't stamped. Set at build time by the Dockerfile:
// -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "watchmud: %v\n", err)
		os.Exit(1)
	}
}

// Run holds everything main used to do, so that deferred cleanup actually
// happens. Nothing below here calls os.Exit.
func run() error {
	configPath := flag.String("config", "app.local.yaml", "location of the server configuration file")
	contentPath := flag.String("content", "", "override location of the content files")
	healthcheck := flag.Bool("healthcheck", false, "ask the running server's health port whether it is ticking, and exit 0 if so")
	flag.Parse()

	// load and verify the serverconfig.Config from YAML
	cfg, err := serverconfig.Load(*configPath)
	if err != nil {
		return fmt.Errorf("server config: %w", err)
	}

	// an explicitly passed flag overrides the config file. Defaults live in
	// the config struct, not in the flag definition.
	if *contentPath != "" {
		cfg.ContentPath = *contentPath
	}

	// before logging, so a probe every 30s doesn't write a startup banner
	if *healthcheck {
		return probeHealth(cfg)
	}

	closeLog, err := logging.Initialize(cfg.Log.File, cfg.Log.Level)
	if err != nil {
		return fmt.Errorf("initializing logging: %w", err)
	}
	defer closeLog()
	log.Info().Str("version", version).Msg("WatchMUD starting")

	d, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("main: %w", err)
	}
	log.Info().Msgf("Current Directory: %s", d)
	log.Info().Msgf("Configuration Path: %s", *configPath)
	log.Info().Msgf("Content Path: %s", cfg.ContentPath)

	// canceled on SIGINT or SIGTERM. A second signal kills the process
	//outright, which is what you want if shutdown hangs.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// load the content: rules and world files
	contentDir := os.DirFS(cfg.ContentPath)
	content, err := loader.LoadContent(contentDir)
	if err != nil {
		return fmt.Errorf("loading content: %w", err)
	}

	// persistence
	store, closeStore, err := openStore(ctx, cfg)
	if err != nil {
		return fmt.Errorf("persistence: %w", err)
	}
	defer closeStore()
	saver := writebehind.New(store)
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		if err := saver.Close(closeCtx); err != nil {
			log.Error().Err(err).Msg("flushing player saves")
		}
	}()

	// randomness
	var seed [32]byte
	if _, err := rand.Read(seed[:]); err != nil {
		return fmt.Errorf("generating random seed: %w", err)
	}
	roller := dice.New(seed)

	// build the world and the game server
	w, err := world.New(content, saver, roller)
	if err != nil {
		return fmt.Errorf("loading world: %w", err)
	}
	// reports go to mongo when there is one, each on a goroutine of its own:
	// the world mustn't wait on a database, and losing a report to an error
	// is a log line, not an outage
	if ms, ok := store.(*mongostore.Store); ok {
		w.SetReportFiler(func(r report.Report) {
			go func() {
				if err := ms.File(r); err != nil {
					log.Error().Err(err).Msg("filing a report")
				}
			}()
		})
	}
	gameServer := server.New(w, content.Catalog, saver)

	// The listeners run beside the game; if they fail -- a port in use, a
	// certificate that won't load -- the server stops with that error rather
	// than run on with nobody able to connect, looking healthy.
	ctx, stopForListener := context.WithCancelCause(ctx)
	defer stopForListener(nil)
	go func() {
		opts := telnet.Options{Addr: fmt.Sprintf("%s:%d", cfg.Telnet.Host, cfg.Telnet.Port)}
		if cfg.TLS.Port != 0 {
			opts.TLSAddr = fmt.Sprintf("%s:%d", cfg.Telnet.Host, cfg.TLS.Port)
			opts.TLSPort = cfg.TLS.Port
			opts.CertFile, opts.KeyFile = cfg.TLS.Cert, cfg.TLS.Key
		}
		if cfg.MSSP.Enabled {
			opts.MSSP = &telnet.MSSP{
				Hostname: cfg.MSSP.Hostname, Website: cfg.MSSP.Website, Contact: cfg.MSSP.Contact,
				Port: cfg.Telnet.Port, TLSPort: cfg.TLS.Port,
				Players: gameServer.Playing, Started: gameServer.Started(),
			}
		}
		if err := telnet.Listen(ctx, opts, gameServer, content.Catalog); err != nil {
			log.Error().Err(err).Msg("telnet listener")
			stopForListener(err)
		}
	}()

	if cfg.Health.Port != 0 {
		if err := serveHealth(ctx, cfg, gameServer); err != nil {
			return fmt.Errorf("health: %w", err)
		}
	}

	// run the game server
	runErr := gameServer.Run(ctx)
	if cause := context.Cause(ctx); cause != nil && !errors.Is(cause, context.Canceled) {
		return fmt.Errorf("listener: %w", cause)
	}
	if runErr != nil && !errors.Is(runErr, context.Canceled) {
		return fmt.Errorf("game server: %w", runErr)
	}

	return nil
}

// healthMaxAge is how long the loop can go without a tick before /healthz
// says so. A tick is a second; this is room for a slow pulse or a long GC,
// and short enough that a wedged world is noticed within a minute.
const healthMaxAge = 30 * time.Second

// serveHealth listens on the health port and answers /healthz until ctx ends.
// The listen happens here, so a port in use fails startup like the telnet
// port does; serving is on its own goroutine, and reads only an atomic.
func serveHealth(ctx context.Context, cfg *serverconfig.Config, gs *server.GameServer) error {
	addr := net.JoinHostPort(cfg.Health.Host, fmt.Sprint(cfg.Health.Port))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.Handle("GET /healthz", gs.HealthHandler(healthMaxAge))
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		_ = srv.Close()
	}()
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error().Err(err).Msg("health server")
		}
	}()
	log.Info().Str("addr", addr).Msg("health: /healthz")
	return nil
}

// probeHealth is `watchmud -healthcheck`: what compose.yaml's healthcheck
// runs, since the image is distroless and has no curl. It reads the same
// config the server did to find the port.
func probeHealth(cfg *serverconfig.Config) error {
	if cfg.Health.Port == 0 {
		return errors.New("healthcheck: no health.port in the config")
	}
	host := cfg.Health.Host
	if host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://" + net.JoinHostPort(host, fmt.Sprint(cfg.Health.Port)) + "/healthz")
	if err != nil {
		return fmt.Errorf("healthcheck: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	fmt.Print(string(body))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("healthcheck: %s", resp.Status)
	}
	return nil
}

// openStore picks where characters are kept, and returns the cleanup to run on
// the way out.
//
// A configured mongo that can't be reached is a hard startup failure rather
// than a fallback to memory: a server that comes up anyway looks healthy right
// until it has quietly thrown away an evening of play. No uri at all is a
// different thing -- that is someone who asked for a throwaway server, and
// they get one, loudly.
func openStore(ctx context.Context, cfg *serverconfig.Config) (player.Store, func(), error) {
	if cfg.Mongo.Uri == "" {
		log.Warn().Msg("no mongo.uri configured: using the in-memory store, nothing will survive a restart")
		return memstore.New(), func() {}, nil
	}

	store, err := mongostore.New(ctx, cfg.Mongo.Uri, cfg.Mongo.Database)
	if err != nil {
		return nil, nil, err
	}
	log.Info().Str("uri", mongostore.RedactURI(cfg.Mongo.Uri)).Str("database", cfg.Mongo.Database).Msg("persistence: mongo")

	return store, func() {
		// ctx is cancelled by the signal that got us here, so the disconnect
		// needs a deadline of its own or it has none at all.
		closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if err := store.Close(closeCtx); err != nil {
			log.Error().Err(err).Msg("closing the player store")
		}
	}, nil
}

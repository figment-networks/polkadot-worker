package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/figment-networks/polkadot-worker/api"
	"github.com/figment-networks/polkadot-worker/api/scale"
	"github.com/figment-networks/polkadot-worker/cmd/polkadot-live/config"
	"github.com/figment-networks/polkadot-worker/cmd/polkadot-live/logger"
	"github.com/figment-networks/polkadot-worker/indexer"
	thttp "github.com/figment-networks/polkadot-worker/transport/http"

	"github.com/figment-networks/indexing-engine/health"
	"github.com/figment-networks/indexing-engine/metrics"
	"github.com/figment-networks/indexing-engine/metrics/prometheusmetrics"

	"go.uber.org/zap"
)

type flags struct {
	configPath string
}

var configFlags = flags{}

func init() {
	flag.StringVar(&configFlags.configPath, "config", "", "path to config.json file")
	flag.Parse()
}

// Start runs polkadot-worker
func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := getConfig(configFlags.configPath)
	if err != nil {
		log.Fatalf("error initializing config [ERR: %v]", err.Error())
	}

	if cfg.RollbarServerRoot == "" {
		cfg.RollbarServerRoot = "github.com/figment-networks/polkadot-worker"
	}
	rcfg := &logger.RollbarConfig{
		AppEnv:             cfg.AppEnv,
		RollbarAccessToken: cfg.RollbarAccessToken,
		RollbarServerRoot:  cfg.RollbarServerRoot,
		Version:            config.GitSHA,
		ChainIDs:           []string{cfg.ChainID},
	}

	if cfg.AppEnv == "development" || cfg.AppEnv == "local" {
		logger.Init("console", "debug", []string{"stderr"}, rcfg)
	} else {
		logger.Init("json", "info", []string{"stderr"}, rcfg)
	}

	logger.Info(config.IdentityString())
	defer logger.Sync()

	// Initialize metrics
	prom := prometheusmetrics.New()
	err = metrics.AddEngine(prom)
	if err != nil {
		logger.Error(err)
		return
	}
	err = metrics.Hotload(prom.Name())
	if err != nil {
		logger.Error(err)
	}

	connApi := api.NewConn(logger.GetLogger())

	polkaNodes := strings.Split(cfg.PolkadotNodeAddrs, ",")
	for _, address := range polkaNodes {
		go connApi.Run(ctx, address)
	}

	ds := scale.NewDecodeStorage()
	specName := cfg.ChainID
	if specName == "mainnet" {
		specName = "polkadot"
	}
	if err := ds.Init(specName); err != nil {
		log.Fatal("Error creating decode storage", zap.Error(err))
	}

	client := indexer.NewClient(logger.GetLogger(), nil, 0, 0, "", "", connApi, ds)
	connector := thttp.NewConnector(client, logger.GetLogger())

	mux := http.NewServeMux()

	connector.AttachToHandler(mux)
	mux.Handle("/metrics", metrics.Handler())

	monitor := &health.Monitor{}
	go monitor.RunChecks(ctx, cfg.HealthCheckInterval)
	monitor.AttachHttp(mux)

	attachProfiling(mux)

	handleHTTP(logger.GetLogger(), *cfg, mux)
}

func getConfig(path string) (cfg *config.Config, err error) {
	cfg = &config.Config{}
	if path != "" {
		if err := config.FromFile(path, cfg); err != nil {
			return nil, err
		}
	}

	if cfg.PolkadotNodeAddrs != "" {
		return cfg, nil
	}

	if err := config.FromEnv(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func handleHTTP(l *zap.Logger, cfg config.Config, mux *http.ServeMux) {
	s := &http.Server{
		Addr:         cfg.Address + ":" + cfg.HTTPPort,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	l.Info("[HTTP] Listening on", zap.String("address", cfg.Address), zap.String("port", cfg.HTTPPort))
	if err := s.ListenAndServe(); err != nil {
		l.Error("[GRPC] Error while listening ", zap.String("address", cfg.Address), zap.String("port", cfg.HTTPPort), zap.Error(err))
	}
}

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/figment-networks/polkadot-worker/api"
	"github.com/figment-networks/polkadot-worker/cmd/polkadot-worker/config"
	"github.com/figment-networks/polkadot-worker/cmd/polkadot-worker/logger"
	"github.com/figment-networks/polkadot-worker/indexer"
	"github.com/figment-networks/polkadot-worker/proxy"
	"golang.org/x/time/rate"

	"github.com/figment-networks/indexer-manager/worker/connectivity"
	grpcIndexer "github.com/figment-networks/indexer-manager/worker/transport/grpc"
	grpcProtoIndexer "github.com/figment-networks/indexer-manager/worker/transport/grpc/indexer"
	"github.com/figment-networks/indexing-engine/health"
	"github.com/figment-networks/indexing-engine/metrics"
	"github.com/figment-networks/indexing-engine/metrics/prometheusmetrics"
	"github.com/figment-networks/polkadothub-proxy/grpc/account/accountpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/block/blockpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/chain/chainpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/decode/decodepb"
	"github.com/figment-networks/polkadothub-proxy/grpc/event/eventpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/transaction/transactionpb"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc"
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

	registerWorker(ctx, logger.GetLogger(), cfg)

	grpcConn, err := grpc.DialContext(
		ctx,
		cfg.PolkadotProxyAddr,
		grpc.WithInsecure(),
		grpc.WithDefaultCallOptions(grpc.WaitForReady(true)),
	)
	if err != nil {
		log.Fatal("Error while creating connection to polkadot-proxy", zap.Error(err))

	}
	defer grpcConn.Close()

	connApi := api.NewConn(logger.GetLogger())
	go connApi.Run(ctx, "0.0.0.0:9944")
	indexerClient := createIndexerClient(ctx, logger.GetLogger(), cfg, grpcConn, connApi)
	go serveGRPC(logger.GetLogger(), *cfg, indexerClient)

	mux := http.NewServeMux()
	mux.Handle("/metrics", metrics.Handler())

	monitor := &health.Monitor{}
	go monitor.RunChecks(ctx, cfg.HealthCheckInterval)
	monitor.AttachHttp(mux)

	handleHTTP(logger.GetLogger(), *cfg, mux)
}

func getConfig(path string) (cfg *config.Config, err error) {
	cfg = &config.Config{}
	if path != "" {
		if err := config.FromFile(path, cfg); err != nil {
			return nil, err
		}
	}

	if cfg.PolkadotProxyAddr != "" {
		return cfg, nil
	}

	if err := config.FromEnv(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func createIndexerClient(ctx context.Context, log *zap.Logger, cfg *config.Config, conn *grpc.ClientConn, connApi *api.Conn) *indexer.Client {
	rateLimiter := rate.NewLimiter(rate.Limit(cfg.ReqPerSecond), cfg.ReqPerSecond)

	proxyClient := proxy.NewClient(
		log,
		rateLimiter,
		accountpb.NewAccountServiceClient(conn),
		blockpb.NewBlockServiceClient(conn),
		chainpb.NewChainServiceClient(conn),
		eventpb.NewEventServiceClient(conn),
		transactionpb.NewTransactionServiceClient(conn),
		decodepb.NewDecodeServiceClient(conn),
	)

	return indexer.NewClient(log, proxyClient, cfg.Exp, uint64(cfg.MaximumHeightsToGet), cfg.ChainID, cfg.Currency, connApi)
}

func registerWorker(ctx context.Context, l *zap.Logger, cfg *config.Config) {
	workerRunID, err := uuid.NewRandom()
	if err != nil {
		l.Error("Error while creating new random id for polkadot-worker", zap.Error(err))
		return
	}

	managers := strings.Split(cfg.Managers, ",")
	hostname := cfg.Hostname
	if hostname == "" {
		hostname = cfg.Address
	}

	logger.Info(fmt.Sprintf("Self-hostname (%s) is %s:%s ", workerRunID.String(), hostname, cfg.Port))

	c := connectivity.NewWorkerConnections(workerRunID.String(), hostname+":"+cfg.Port, cfg.Network, cfg.ChainID, "0.0.1")
	for _, m := range managers {
		c.AddManager(m + "/client_ping")
	}

	logger.Info(fmt.Sprintf("Connecting to managers (%s)", strings.Join(managers, ",")))

	go c.Run(ctx, l, cfg.ManagerInterval)
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

func serveGRPC(l *zap.Logger, cfg config.Config, client *indexer.Client) {
	defer l.Sync()
	ctx := context.Background()
	grpcServer := grpc.NewServer()
	indexer := grpcIndexer.NewIndexerServer(ctx, client, l)

	grpcProtoIndexer.RegisterIndexerServiceServer(grpcServer, indexer)

	l.Info("[GRPC] Listening on", zap.String("address", cfg.Address), zap.String("port", cfg.Port))
	lis, err := net.Listen("tcp", cfg.Address+":"+cfg.Port)
	if err != nil {
		l.Error("[GRPC] Error while listening ", zap.String("address", cfg.Address), zap.String("port", cfg.Port), zap.Error(err))
		return
	}

	if err := grpcServer.Serve(lis); err != nil {
		l.Error("[GRPC] Error while serving ", zap.String("address", cfg.Address), zap.String("port", cfg.Port), zap.Error(err))
	}
}

package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/figment-networks/polkadot-worker/config"
	"github.com/figment-networks/polkadot-worker/indexer"
	"github.com/figment-networks/polkadot-worker/proxy"

	"github.com/figment-networks/indexer-manager/worker/connectivity"
	grpcIndexer "github.com/figment-networks/indexer-manager/worker/transport/grpc"
	grpcProtoIndexer "github.com/figment-networks/indexer-manager/worker/transport/grpc/indexer"
	"github.com/figment-networks/indexing-engine/metrics"
	"github.com/figment-networks/indexing-engine/metrics/prometheusmetrics"
	"github.com/figment-networks/polkadothub-proxy/grpc/block/blockpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/event/eventpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/transaction/transactionpb"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
)

// Start runs polkadot-worker
func Start() {
	cfg := getConfig()
	mainCtx := context.Background()
	log, logSync := getLogger(cfg.Worker.LogLevel)
	defer logSync()

	indexerClient, closeProxyConnection := createIndexerClient(mainCtx, log, &cfg)
	defer closeProxyConnection()

	registerWorker(mainCtx, log, &cfg)

	grpcServer := grpc.NewServer()
	indexer := grpcIndexer.NewIndexerServer(mainCtx, indexerClient, log.Desugar())

	grpcProtoIndexer.RegisterIndexerServiceServer(grpcServer, indexer)

	lis, err := net.Listen("tcp", "0.0.0.0"+cfg.ProxyPort)
	if err != nil {
		log.Errorf("Error while listening on %s port", cfg.ProxyPort, zap.Error(err))
		return
	}

	go createMetricsHandler(&cfg, log)

	log.Infof("Polkadot-worker listetning on port %s", cfg.ProxyPort)

	grpcServer.Serve(lis)
}

func getConfig() (cfg config.Config) {
	file, err := ioutil.ReadFile("./../../config.json")
	if err != nil {
		fmt.Printf("Error while getting config file: %s\n", err.Error())
		os.Exit(1)
	}

	if err := json.Unmarshal(file, &cfg); err != nil {
		fmt.Printf("Error while unmarshalling config file to struct: %s", err.Error())
		os.Exit(1)
	}

	return
}

func getLogger(logLevel string) (*zap.SugaredLogger, func() error) {
	lvl := zap.NewAtomicLevel()
	if logLevel == "debug" {
		lvl.SetLevel(zapcore.DebugLevel)
	}

	cfg := zap.Config{
		Encoding:    "json",
		Level:       lvl,
		OutputPaths: []string{"stderr"},
		EncoderConfig: zapcore.EncoderConfig{
			MessageKey:   "message",
			LevelKey:     "level",
			EncodeLevel:  zapcore.CapitalLevelEncoder,
			TimeKey:      "time",
			EncodeTime:   zapcore.RFC3339TimeEncoder,
			CallerKey:    "caller",
			EncodeCaller: zapcore.ShortCallerEncoder,
		},
	}

	logger, err := cfg.Build()
	if err != nil {
		panic(fmt.Sprintf("Could not create a new logger: %s", err.Error()))
	}

	return logger.Sugar(), logger.Sync
}

func createIndexerClient(ctx context.Context, log *zap.SugaredLogger, cfg *config.Config) (*indexer.Client, func() error) {
	conn, err := grpc.DialContext(
		ctx,
		cfg.PolkadotClientBaseURL,
		grpc.WithInsecure(),
		grpc.WithDefaultCallOptions(grpc.WaitForReady(true)),
	)
	if err != nil {
		log.Fatalf("Error while creating connection with polkadot-proxy: %s", err.Error())
	}

	proxyClient := proxy.NewClient(
		log,
		blockpb.NewBlockServiceClient(conn),
		eventpb.NewEventServiceClient(conn),
		transactionpb.NewTransactionServiceClient(conn),
	)

	return indexer.NewClient(
		cfg.Worker.ChainID,
		log,
		cfg.IndexerManager.Page,
		proxyClient,
	), conn.Close
}

func registerWorker(ctx context.Context, log *zap.SugaredLogger, cfg *config.Config) {
	workerRunID, err := uuid.NewRandom()
	if err != nil {
		log.Errorf("Error while creating new random id for polkadot-worker: %s", err.Error())
	}

	workerAddress := cfg.Worker.Address.Host + cfg.Worker.Address.Port

	c := connectivity.NewWorkerConnections(workerRunID.String(), workerAddress, cfg.Worker.Network, cfg.Worker.ChainID, "0.0.1")

	c.AddManager(cfg.IndexerManager.BaseURL + "/client_ping")

	go c.Run(ctx, log.Desugar(), 10*time.Second)
}

func createMetricsHandler(cfg *Config, log *zap.SugaredLogger) {
	prom := prometheusmetrics.New()
	if err := metrics.AddEngine(prom); err != nil {
		log.Errorf("Error wile adding prometheus metrics engine", zap.Error(err))
	}
	if err := metrics.Hotload(prom.Name()); err != nil {
		log.Errorf("Error wile loading prometheus metrics engine", zap.Error(err))
	}

	mux := http.NewServeMux()

	mux.Handle("/metrics", metrics.Handler())

	s := &http.Server{
		Addr:         cfg.Host + cfg.WorkerPort,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Infof("Handling metrics on port %s", cfg.WorkerPort)
	if err := s.ListenAndServe(); err != nil {
		log.Error("Error while listening on %s port", cfg.WorkerPort, zap.Error(err))
	}
}

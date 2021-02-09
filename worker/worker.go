package worker

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/figment-networks/polkadot-worker/worker/indexer"
	"github.com/figment-networks/polkadot-worker/worker/proxy"

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
	"gopkg.in/yaml.v2"
)

// Start runs polkadot-worker
func Start() {
	cfg := getConfig()
	mainCtx := context.Background()
	log, logSync := getLogger(cfg.LogLevel)
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

func getConfig() (cfg Config) {
	file, err := ioutil.ReadFile("config.yml")
	if err != nil {
		fmt.Printf("Error while getting config file: %s\n", err.Error())
		os.Exit(1)
	}

	if err := yaml.Unmarshal(file, &cfg); err != nil {
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

func createIndexerClient(ctx context.Context, log *zap.SugaredLogger, cfg *Config) (*indexer.Client, func() error) {
	conn, err := grpc.DialContext(
		ctx,
		cfg.Proxy.Client.URL,
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
		cfg.ChainID,
		log,
		cfg.Indexer.Client.Page,
		proxyClient,
	), conn.Close
}

func registerWorker(ctx context.Context, log *zap.SugaredLogger, cfg *Config) {
	workerRunID, err := uuid.NewRandom()
	if err != nil {
		log.Errorf("Error while creating new random id for polkadot-worker: %s", err.Error())
	}

	workerAddress := cfg.Host + cfg.ProxyPort

	c := connectivity.NewWorkerConnections(workerRunID.String(), workerAddress, cfg.Network, cfg.ChainID, "0.0.1")

	c.AddManager(cfg.Indexer.Manager.Address + "/client_ping")

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

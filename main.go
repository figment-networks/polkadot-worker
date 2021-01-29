package main

import (
	"context"
	"fmt"
	"io/ioutil"

	"github.com/figment-networks/polkadot-worker/utils"
	"github.com/figment-networks/polkadot-worker/worker"
	"github.com/figment-networks/polkadot-worker/worker/mapper"

	"github.com/figment-networks/polkadothub-proxy/grpc/block/blockpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/event/eventpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/transaction/transactionpb"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v3"
)

func main() {
	mainCtx := context.Background()

	logger, err := zap.NewProduction()
	if err != nil {
		panic(fmt.Sprintf("Could not create a new logger: %s", err.Error()))
	}
	defer logger.Sync()
	log := logger.Sugar()

	config := getConfig(log)

	conn, err := grpc.DialContext(
		mainCtx,
		config.Client.Grcp.URL,
		grpc.WithInsecure(),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(config.Client.Grcp.MaxMsgSize),
		),
	)
	if err != nil {
		log.Fatalf("Error while creating connection with polkadot-proxy: %s", err.Error())
	}

	client := worker.Client{
		Log:     log,
		GrcpCli: conn,

		BlockClient:       blockpb.NewBlockServiceClient(conn),
		EventClient:       eventpb.NewEventServiceClient(conn),
		TransactionClient: transactionpb.NewTransactionServiceClient(conn),
	}

	blockRes, err := client.GetBlockByHeight(3537654)
	if err != nil {
		log.Fatal(err.Error())
	}

	fmt.Println(blockRes)

	transactionRes, err := client.GetTransactionByHeight(3537654)
	if err != nil {
		log.Fatal(err.Error())
	}

	fmt.Println(transactionRes)

	eventRes, err := client.GetEventByHeight(3537654)
	if err != nil {
		log.Fatal(err.Error())
	}

	fmt.Println(eventRes)

	if _, err = mapper.TransactionMapper(blockRes, eventRes, transactionRes); err != nil {
		log.Fatal(err.Error())
	}

}

func getConfig(log *zap.SugaredLogger) (cfg utils.Config) {
	file, err := ioutil.ReadFile("config.yml")
	if err != nil {
		log.Fatalf("Error while getting config file: %s", err.Error())
	}

	if err := yaml.Unmarshal(file, &cfg); err != nil {
		log.Fatalf("Error while unmarshalling config file to struct: %s", err.Error())
	}

	return
}

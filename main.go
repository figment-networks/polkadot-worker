package main

import (
	"context"
	"fmt"
	"io/ioutil"

	"github.com/figment-networks/polkadot-worker/client"
	"github.com/figment-networks/polkadot-worker/utils"
	"github.com/figment-networks/polkadothub-proxy/grpc/block/blockpb"

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

	client := client.Client{
		Log:         log,
		GrcpCli:     conn,
		BlockClient: blockpb.NewBlockServiceClient(conn),
	}

	res, err := client.GetBlockByHeight(590)
	if err != nil {
		log.Fatal(err.Error())
	}

	fmt.Println(res)

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

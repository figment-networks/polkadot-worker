package main

import (
	"context"
	"io/ioutil"
	"log"
	"os"

	"github.com/figment-networks/polkadot-worker/client"
	"github.com/figment-networks/polkadot-worker/utils"
	"github.com/figment-networks/polkadothub-proxy/grpc/block/blockpb"

	"google.golang.org/grpc"
	"gopkg.in/yaml.v3"
)

func main() {
	mainCtx := context.Background()

	log := log.New(nil, "[polkadot-worker]", log.Ldate|log.Ltime|log.Lshortfile)
	log.SetOutput(os.Stdout)

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

	log.Println(res)

}

func getConfig(log *log.Logger) (cfg utils.Config) {
	file, err := ioutil.ReadFile("config.yml")
	if err != nil {
		log.Fatalf("Error while getting config file: %s", err.Error())
	}

	if err := yaml.Unmarshal(file, &cfg); err != nil {
		log.Fatalf("Error while unmarshalling config file to struct: %s", err.Error())
	}

	return
}

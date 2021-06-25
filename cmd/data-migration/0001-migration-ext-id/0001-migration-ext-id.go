package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/figment-networks/polkadot-worker/api"
	"github.com/figment-networks/polkadot-worker/api/scale"
	"github.com/figment-networks/polkadot-worker/cmd/polkadot-live/logger"
	"github.com/figment-networks/polkadot-worker/indexer"

	"github.com/figment-networks/indexing-engine/structs"
	"github.com/figment-networks/indexing-engine/worker/store/params"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"go.uber.org/zap"
)

var (
	network       string
	chain         string
	dbpath        string
	from          uint64
	to            uint64
	polkadotAddrs string
)

func init() {
	flag.StringVar(&network, "network", "", "Network  name")
	flag.StringVar(&chain, "chain", "", "Chain  name")
	flag.StringVar(&dbpath, "db", "", "Database connection string")
	flag.Uint64Var(&from, "from", 0, "Starting height")
	flag.Uint64Var(&to, "to", 0, "End height")

	flag.StringVar(&polkadotAddrs, "polkadotAddrs", "", "Polkadot node ws addresses separated by coma")
	flag.Parse()
}

func main() {
	ctx := context.Background()
	logger.Init("json", "info", []string{"stderr"}, nil)
	zlog := logger.GetLogger()
	defer zlog.Sync()
	// connect to database
	logger.Info("[DB] Connecting to database...")
	db, err := sql.Open("postgres", dbpath)
	if err != nil {
		logger.Error(err)
		return
	}
	if err := db.PingContext(ctx); err != nil {
		logger.Error(err)
		return
	}

	connApi := api.NewConn(logger.GetLogger())
	polkaNodes := strings.Split(polkadotAddrs, ",")
	for _, address := range polkaNodes {
		go connApi.Run(ctx, address, time.Second*30)
	}

	ds := scale.NewDecodeStorage()
	specName := chain
	if specName == "mainnet" {
		specName = "polkadot"
	}
	if err := ds.Init(specName); err != nil {
		log.Fatal("Error creating decode storage", zap.Error(err))
	}

	client := indexer.NewClient(logger.GetLogger(), nil, ds, nil, connApi, 0, 0, chain, "", "")

	logger.Info("[DB] Ping successfull...")
	defer db.Close()
	if err := getTransactions(ctx, zlog, client, db, connApi, ds, network, chain, from, to); err != nil {
		logger.Error(err)
		return
	}
}

func getTransactions(ctx context.Context, zLog *zap.Logger, c *indexer.Client, db *sql.DB, serverConn *api.Conn, ds *scale.DecodeStorage, network, chain string, from, to uint64) error {

	for i := from; i < to+1; i++ {
		if err := getTransaction(ctx, zLog, c, db, serverConn, ds, network, chain, i); err != nil && err != sql.ErrNoRows {
			return err
		}
		log.Printf("updated  - %d ", i)
	}
	return nil
}

func getTransaction(ctx context.Context, zLog *zap.Logger, c *indexer.Client, db *sql.DB, serverConn *api.Conn, ds *scale.DecodeStorage, network, chain string, height uint64) error {

	rows, err := db.QueryContext(ctx, "SELECT id, height, hash, data FROM public.transaction_events WHERE network = $1 AND chain_id = $2 AND height = $3  ORDER BY height ASC", []interface{}{network, chain, height}...)
	switch {
	case err == sql.ErrNoRows:
		return params.ErrNotFound
	case err != nil:
		return fmt.Errorf("query error: %w", err)
	default:
	}

	defer rows.Close()

	initTxs := []structs.Transaction{}
	for rows.Next() {
		var id uuid.UUID
		var events structs.TransactionEvents
		var height uint64
		var hash string

		if err := rows.Scan(&id, &height, &hash, &events); err != nil {
			return err
		}
		initTxs = append(initTxs, structs.Transaction{
			ID:     id,
			Height: height,
			Hash:   hash,
			Events: events,
		})
	}

	if len(initTxs) == 0 {
		return nil
	}

	if len(initTxs) == 1 {
		ev := initTxs[0].Events
		ev[0].ID = fmt.Sprintf("%d-0", height)
		evts, err := json.Marshal(ev)
		if err != nil {
			return err
		}
		log.Println("singular update - ", evts)
		if _, err := db.ExecContext(ctx, "UPDATE public.transaction_events SET data = $1 WHERE id = $2", evts, initTxs[0].ID); err != nil {
			return err
		}
	}

	if len(initTxs) > 1 {
		sort.Slice(initTxs, func(i, j int) bool {
			ev1 := strings.Split(initTxs[i].Events[0].Sub[0].ID, "-")
			ev2 := strings.Split(initTxs[j].Events[0].Sub[0].ID, "-")
			val1, err1 := strconv.ParseInt(ev1[1], 10, 64)
			val2, err2 := strconv.ParseInt(ev2[1], 10, 64)
			if err1 != nil || err2 != nil {
				return false
			}
			return val1 < val2
		})
	}

	ch := make(chan api.Response, 20)
	defer close(ch)

	blH, _, gpBLH, err := indexer.GetBlockHashes(height, serverConn, c.Cache, ch)
	if err != nil {
		return fmt.Errorf("error unmarshaling block data: %w", err)
	}

	prm := &scale.PolkaRuntimeVersion{}
	block := &scale.PolkaBlock{}
	serverConn.Send(ch, 1234, "state_getRuntimeVersion", []interface{}{blH})
	serverConn.Send(ch, 2345, "chain_getBlock", []interface{}{blH})

	var got int8
RuntimeVersionLoop:
	for {
		select {
		case resp := <-ch:
			if resp.Error != nil {
				return fmt.Errorf("response from ws is wrong: %s ", resp.Error)
			}
			if len(resp.Result) == 0 {
				return fmt.Errorf("response from ws is empty")
			}
			switch resp.Type {
			case "state_getRuntimeVersion":
				if err = json.Unmarshal(resp.Result, prm); err != nil {
					return errors.New("wrong data returned state_getRuntimeVersion")
				}
			case "chain_getBlock":
				if err = json.Unmarshal(resp.Result, block); err != nil {
					return errors.New("wrong data returned state_getRuntimeVersion")
				}
			default:
				return errors.New("wrong data returned")
			}
			got++
			if got == 2 {
				break RuntimeVersionLoop
			}
		case <-time.After(time.Second * 30):
			return errors.New("timeout on state_getRuntimeVersion after 30 seconds")
		}
	}
	meta, err := c.GetMetadata(serverConn, ch, gpBLH, prm.SpecName, uint(prm.SpecVersion))
	if err != nil {
		return fmt.Errorf("error while getting metadata: %w", err)
	}

	txs, err := indexer.GetTransactionsForHeight(ds, block, meta, int(prm.SpecVersion))
	if err != nil {
		return fmt.Errorf("error getTransactionsForHeight: %w", err)
	}

	// pair transactions with ids
	var found bool
	for k, t := range initTxs {
		found = false
	TXS_LOOP:
		for extIndex, rawTx := range txs {
			if t.Hash[2:] == rawTx.ExtrinsicHash {
				found = true
				for in, ev := range t.Events {
					ev.ID = fmt.Sprintf("%d-%d", height, extIndex)
					t.Events[in] = ev
				}
				initTxs[k] = t
				break TXS_LOOP
			}
		}
		if !found {
			if len(txs) >= k+1 {
				candidateTx := txs[k]
				for in, ev := range t.Events {
					if strings.ToLower(candidateTx.CallModule.Name) == ev.Module && ev.Kind == "Extrinsic" {
						found = true
						ev.ID = fmt.Sprintf("%d-%d", height, k)
						t.Events[in] = ev
					}
				}
				initTxs[k] = t
			}
		}

		if !found {
			return errors.New("fatal error on height " + strconv.FormatUint(height, 10))
		}
	}
	for _, iTx := range initTxs {
		evts, err := json.Marshal(iTx.Events)
		if err != nil {
			return err
		}
		if _, err := db.ExecContext(ctx, "UPDATE public.transaction_events SET data = $1 WHERE id = $2", evts, iTx.ID); err != nil {
			return err
		}
	}

	return nil
}

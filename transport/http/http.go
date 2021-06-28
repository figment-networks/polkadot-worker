package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/figment-networks/indexing-engine/metrics"
	"github.com/figment-networks/polkadot-worker/api/scale"

	"go.uber.org/zap"
)

var (
	getAccountDuration *metrics.GroupObserver
)

type retrieveClienter interface {
	GetAccount(ctx context.Context, logger *zap.Logger, height uint64, accountID string) (pai scale.PolkaAccountInfo, err error)
}

// Connector is main HTTP connector for manager
type Connector struct {
	cli    retrieveClienter
	logger *zap.Logger
}

// NewConnector is  Connector constructor
func NewConnector(cli retrieveClienter, logger *zap.Logger) *Connector {
	getAccountDuration = endpointDuration.WithLabels("getAccount")
	return &Connector{cli, logger}
}

// AttachToHandler attaches handlers to http server's mux
func (c *Connector) AttachToHandler(mux *http.ServeMux) {
	mux.HandleFunc("/account", c.GetAccount)
	mux.HandleFunc("/account/", c.GetAccount)

}

// GetAccount is http handler for GetTransactions method
func (c *Connector) GetAccount(w http.ResponseWriter, req *http.Request) {
	timer := metrics.NewTimer(getAccountDuration)
	defer timer.ObserveDuration()
	var (
		intHeight uint64
		err       error
	)
	enc := json.NewEncoder(w)
	height := req.URL.Query().Get("height")
	if height != "" {
		intHeight, err = strconv.ParseUint(height, 10, 64)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			enc.Encode(ServiceError{Msg: "Invalid height param: " + err.Error()})
			return
		}
	}

	s := strings.Split(strings.Replace(req.URL.Path, "/account", "", 1), "/")
	if len(s) < 2 || s[1] == "" {
		w.WriteHeader(http.StatusBadRequest)
		enc.Encode(ServiceError{Msg: "empty address parameter "})
		return
	}

	ac, err := c.cli.GetAccount(req.Context(), c.logger, intHeight, s[len(s)-1])
	if err != nil {
		c.logger.Error("Error processing account request", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		enc.Encode(ServiceError{Msg: "Error processing account request"})
		return
	}

	err = enc.Encode(AccountView{
		Nonce:      int64(ac.Nonce),
		Free:       ac.Data.Free,
		Reserved:   ac.Data.Reserved,
		MiscFrozen: ac.Data.MiscFrozen,
		FeeFrozen:  ac.Data.FeeFrozen,
	})

	if err != nil {
		c.logger.Error("Error processing account request", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		enc.Encode(ServiceError{Msg: "Error processing account request"})
		return
	}
}

// ServiceError structure as formated error
type ServiceError struct {
	Status int         `json:"status"`
	Msg    interface{} `json:"error"`
}

func (ve ServiceError) Error() string {
	return fmt.Sprintf("Bad Request: %s", ve.Msg)
}

type AccountView struct {
	// Nonce passed
	Nonce int64 `json:"nonce"`
	// Free balance of account
	Free string `json:"free"`
	// Reserved balance of account
	Reserved string `json:"reserved"`
	// MiscFrozen balance of account
	MiscFrozen string `json:"misc_frozen"`
	// FeeFrozen balance of account
	FeeFrozen string `json:"fee_frozen"`
}

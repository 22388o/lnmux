package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/bottlepay/lnmux"
	"github.com/bottlepay/lnmux/common"
	"github.com/bottlepay/lnmux/lnd"
	"github.com/bottlepay/lnmux/persistence"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/urfave/cli/v2"
)

var runCommand = &cli.Command{
	Name:   "run",
	Action: runAction,
}

func runAction(c *cli.Context) error {
	cfg, err := loadConfig(c.String("config"))
	if err != nil {
		return err
	}

	// Setup persistence.
	db, err := persistence.NewPostgresPersister(&persistence.PostgresOptions{
		DSN:    cfg.DSN,
		Logger: log,
	})
	if err != nil {
		return err
	}

	// Parse lnd connection info from the configuration.
	lnds, err := initLndClients(&cfg.Lnd)
	if err != nil {
		return err
	}

	// Get identity key so that incoming htlcs can be decoded.
	identityKey, err := cfg.GetIdentityKey()
	if err != nil {
		return err
	}

	keyRing := lnmux.NewKeyRing(identityKey)

	activeNetParams := &chaincfg.RegressionNetParams

	wrappedDb := &dbWrapper{
		db: db,
	}

	// Instantiate and start the multiplexer.
	mux, err := lnmux.New(
		&lnmux.MuxConfig{
			KeyRing:         keyRing,
			ActiveNetParams: activeNetParams,
			Lnd:             lnds,
			ChannelDb:       wrappedDb,
			Logger:          log,
		})
	if err != nil {
		return err
	}

	if err := mux.Start(); err != nil {
		return err
	}

	// Wait for break and terminate.
	log.Infof("Press ctrl-c to exit")
	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-sigint
	log.Infof("Exiting")

	if err := mux.Stop(); err != nil {
		return err
	}

	return nil
}

func initLndClients(cfg *LndConfig) ([]lnd.LndClient, error) {
	var (
		nodes []lnd.LndClient
	)

	network, err := network(cfg.Network)
	if err != nil {
		return nil, err
	}

	seenPubKeys := make(map[common.PubKey]struct{})
	for _, node := range cfg.Nodes {
		pubkey, err := common.NewPubKeyFromStr(node.PubKey)
		if err != nil {
			return nil, fmt.Errorf("cannot parse pubkey %v: %v", node.PubKey, err)
		}

		lnd, err := lnd.NewLndClient(lnd.Config{
			TlsCertPath:  node.TlsCertPath,
			MacaroonPath: node.MacaroonPath,
			LndUrl:       node.LndUrl,
			Logger:       log,
			PubKey:       pubkey,
			Network:      network,
			Timeout:      cfg.Timeout,
		})
		if err != nil {
			return nil, err
		}
		pubKey := lnd.PubKey()
		if _, exists := seenPubKeys[pubKey]; exists {
			return nil, fmt.Errorf("duplicate lnd node: %v", pubKey)
		}
		seenPubKeys[pubKey] = struct{}{}

		nodes = append(nodes, lnd)
	}

	return nodes, nil
}

func network(network string) (*chaincfg.Params, error) {
	switch network {
	case chaincfg.MainNetParams.Name:
		return &chaincfg.MainNetParams, nil
	case chaincfg.TestNet3Params.Name:
	case "testnet":
		return &chaincfg.TestNet3Params, nil
	case chaincfg.RegressionNetParams.Name:
		return &chaincfg.RegressionNetParams, nil
	case chaincfg.SimNetParams.Name:
		return &chaincfg.SimNetParams, nil
	}

	return nil, fmt.Errorf("unsupported network %v", network)
}
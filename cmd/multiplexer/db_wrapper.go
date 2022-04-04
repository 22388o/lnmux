package main

import (
	"github.com/bottlepay/lnmux"
	"github.com/bottlepay/lnmux/persistence"
	"github.com/lightningnetwork/lnd/lntypes"
)

// dbWrapper wraps a persistence instance to make it compatible with the multiplexer interface.
type dbWrapper struct {
	db *persistence.PostgresPersister
}

func (d *dbWrapper) Get(hash lntypes.Hash) (*lnmux.InvoiceCreationData,
	map[lnmux.CircuitKey]int64, error) {

	invoice, htlcs, err := d.db.Get(hash)
	if err != nil {
		return nil, nil, err
	}

	return &invoice.InvoiceCreationData.InvoiceCreationData, htlcs, nil
}

func (d *dbWrapper) Settle(hash lntypes.Hash,
	htlcs map[lnmux.CircuitKey]int64) error {

	return d.db.Settle(hash, htlcs)
}
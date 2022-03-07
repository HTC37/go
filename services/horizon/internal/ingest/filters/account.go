package filters

import (
	"context"
	"encoding/json"

	"github.com/stellar/go/ingest"
	"github.com/stellar/go/services/horizon/internal/db2/history"
	"github.com/stellar/go/services/horizon/internal/ingest/processors"
	"github.com/stellar/go/support/errors"
)

type AccountFilterRules struct {
	CanonicalWhitelist []string `json:"account_whitelist"`
}

type accountFilter struct {
	whitelistedAccountsSet map[string]struct{}
	lastModified           int64
	enabled                bool
}

type AccountFilter interface {
	processors.LedgerTransactionFilterer
	RefreshAccountFilter(filterConfig *history.FilterConfig) error
}

func NewAccountFilter() AccountFilter {
	return &accountFilter{
		whitelistedAccountsSet: map[string]struct{}{},
	}
}

// TODO:(fons) this code should probably be generic for all filters
func (filter *accountFilter) RefreshAccountFilter(filterConfig *history.FilterConfig) error {
	// only need to re-initialize the filter config state(rules) if it's cached version(in  memory)
	// is older than the incoming config version based on lastModified epoch timestamp
	if filterConfig.LastModified > filter.lastModified {
		logger.Infof("New Account Filter config detected, reloading new config %v ", *filterConfig)
		var accountFilterRules AccountFilterRules
		if err := json.Unmarshal([]byte(filterConfig.Rules), &accountFilterRules); err != nil {
			return errors.Wrap(err, "unable to serialize account filter rules")
		}

		filter.enabled = filterConfig.Enabled
		filter.whitelistedAccountsSet = listToMap(accountFilterRules.CanonicalWhitelist)
		filter.lastModified = filterConfig.LastModified
	}

	return nil
}

func (f *accountFilter) FilterTransaction(ctx context.Context, transaction ingest.LedgerTransaction) (bool, error) {
	// filtering is disabled if the whitelist is empty for now, as that is the only filter rule
	if len(f.whitelistedAccountsSet) == 0 || !f.enabled {
		return true, nil
	}

	// TODO: what is the sequence used for?
	participants, err := processors.ParticipantsForTransaction(0, transaction)
	if err != nil {
		return false, err
	}

	// NOTE: this assumes that the participant list has a small memory footprint
	//       otherwise, we should be doing the filtering on the DB side
	for _, p := range participants {
		if _, ok := f.whitelistedAccountsSet[p.Address()]; ok {
			return true, nil
		}
	}
	return false, nil
}
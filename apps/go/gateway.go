package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const maxAccountIDLength = 64

type account struct {
	ID           string `json:"id"`
	Currency     string `json:"currency"`
	BalanceMinor *int64 `json:"balanceMinor"`
}

type transactionList struct {
	Transactions []struct {
		AmountMinor *int64 `json:"amountMinor"`
	} `json:"transactions"`
}

var errIncompleteResponse = errors.New("downstream response is missing required fields")

func (a account) validate() error {
	if a.ID == "" || a.Currency == "" || a.BalanceMinor == nil {
		return errIncompleteResponse
	}
	return nil
}

func (l transactionList) validate() error {
	if l.Transactions == nil {
		return errIncompleteResponse
	}
	for _, tx := range l.Transactions {
		if tx.AmountMinor == nil {
			return errIncompleteResponse
		}
	}
	return nil
}

func (r rateTable) validate() error {
	if r.Rates == nil {
		return errIncompleteResponse
	}
	return nil
}

type validator interface{ validate() error }

type rateTable struct {
	Rates map[string]float64 `json:"rates"`
}

type aggregate struct {
	AccountID       string `json:"accountId"`
	Currency        string `json:"currency"`
	BalanceMinor    int64  `json:"balanceMinor"`
	Transactions    int    `json:"transactions"`
	SpentMinor      int64  `json:"spentMinor"`
	ReceivedMinor   int64  `json:"receivedMinor"`
	BalanceEurMinor int64  `json:"balanceEurMinor"`
}

type gateway struct {
	client *http.Client
	base   string
}

func newGateway(base string) *gateway {
	transport := &http.Transport{MaxIdleConnsPerHost: 4096, IdleConnTimeout: 90 * time.Second}
	return &gateway{client: &http.Client{Transport: transport, Timeout: 5 * time.Second}, base: base}
}

func (g *gateway) aggregate(ctx context.Context, accountID string) (aggregate, error) {
	var (
		acct  account
		txs   transactionList
		rates rateTable
		errs  [3]error
		calls sync.WaitGroup
	)
	calls.Go(func() { errs[0] = g.get(ctx, "/accounts/"+accountID, &acct) })
	calls.Go(func() { errs[1] = g.get(ctx, "/accounts/"+accountID+"/transactions", &txs) })
	calls.Go(func() { errs[2] = g.get(ctx, "/rates", &rates) })
	calls.Wait()
	if err := errors.Join(errs[:]...); err != nil {
		return aggregate{}, err
	}
	return summarize(acct, txs, rates), nil
}

func (g *gateway) get(ctx context.Context, path string, into validator) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.base+path, nil)
	if err != nil {
		return err
	}
	resp, err := g.client.Do(req)
	if urlErr := (*url.Error)(nil); errors.As(err, &urlErr) {
		return fmt.Errorf("downstream unreachable: %w", urlErr.Err)
	} else if err != nil {
		return err
	}
	defer resp.Body.Close()
	defer io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downstream answered %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(into); err != nil {
		return err
	}
	return into.validate()
}

func summarize(acct account, txs transactionList, rates rateTable) aggregate {
	result := aggregate{
		AccountID:       acct.ID,
		Currency:        acct.Currency,
		BalanceMinor:    *acct.BalanceMinor,
		Transactions:    len(txs.Transactions),
		BalanceEurMinor: int64(math.Round(float64(*acct.BalanceMinor) * rates.Rates["EUR"])),
	}
	for _, tx := range txs.Transactions {
		if amount := *tx.AmountMinor; amount < 0 {
			result.SpentMinor -= amount
		} else {
			result.ReceivedMinor += amount
		}
	}
	return result
}

func validAccountID(id string) bool {
	if id == "" || len(id) > maxAccountIDLength {
		return false
	}
	for _, c := range id {
		if !(c == '-' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z') {
			return false
		}
	}
	return true
}

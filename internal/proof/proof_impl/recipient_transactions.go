package proof_impl

import (
	"context"
	"fmt"
	"github.com/lidofinance/cosmos-query-relayer/internal/proof"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto/merkle"
	"github.com/tendermint/tendermint/rpc/coretypes"
	"github.com/tendermint/tendermint/types"
	"strings"
)

var perPage = 100

const orderBy = ""

// RecipientTransactions gets proofs for query type = 'x/tx/RecipientTransactions'
// (NOTE: there is no such query function in cosmos-sdk)
func (p ProoferImpl) RecipientTransactions(ctx context.Context, queryParams map[string]string) ([]proof.TxValue, error) {
	query := queryFromParams(queryParams)
	page := 1 // NOTE: page index starts from 1

	txs := make([]*coretypes.ResultTx, 0)
	for {
		searchResult, err := p.querier.Client.TxSearch(ctx, query, true, &page, &perPage, orderBy)
		if err != nil {
			return nil, fmt.Errorf("could not query new transactions to proof: %w", err)
		}

		if len(searchResult.Txs) == 0 {
			break
		}

		if page*perPage >= searchResult.TotalCount {
			break
		}

		for _, item := range searchResult.Txs {
			txs = append(txs, item)
		}

		page += 1
	}

	if len(txs) == 0 {
		return []proof.TxValue{}, nil
	}

	result := make([]proof.TxValue, 0, len(txs))
	for _, item := range txs {
		deliveryProof, inclusionProof, err := TxCompletedSuccessfullyProof(ctx, p.querier, item.Height, item.Index)
		if err != nil {
			return nil, fmt.Errorf("could not proof transaction with hash=%s: %w", item.Tx.String(), err)
		}

		txProof := proof.TxValue{
			InclusionProof: item.Proof.Proof,
			DeliveryProof:  *deliveryProof,
			Tx:             inclusionProof,
			Height:         uint64(item.Height),
		}
		result = append(result, txProof)
	}

	return result, nil
}

// TxCompletedSuccessfullyProof returns (deliveryProof, deliveryResult, error) for transaction in block 'blockHeight' with index 'txIndexInBlock'
func TxCompletedSuccessfullyProof(ctx context.Context, querier *proof.Querier, blockHeight int64, txIndexInBlock uint32) (*merkle.Proof, *abci.ResponseDeliverTx, error) {
	results, err := querier.Client.BlockResults(ctx, &blockHeight)

	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch block results for height = %d: %w", blockHeight, err)
	}

	txsResults := results.TxsResults
	abciResults := types.NewResults(txsResults)
	txProof := abciResults.ProveResult(int(txIndexInBlock))
	txResult := txsResults[txIndexInBlock]

	return &txProof, txResult, nil
}

func queryFromParams(params map[string]string) string {
	queryParamsList := make([]string, 0, len(params))
	for key, value := range params {
		queryParamsList = append(queryParamsList, fmt.Sprintf("%s='%s'", key, value))
	}
	return strings.Join(queryParamsList, " AND ")
}

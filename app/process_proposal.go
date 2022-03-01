package app

import (
	"bytes"

	"github.com/celestiaorg/celestia-app/x/payment/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/pkg/da"
	coretypes "github.com/tendermint/tendermint/types"
)

const (
	rejectedPropBlockLog = "Rejected proposal block:"
)

func (app *App) ProcessProposal(req abci.RequestProcessProposal) abci.ResponseProcessProposal {
	// Check for message inclusion:
	//  - each MsgPayForMessage included in a block should have a corresponding message also in the block data
	//  - the commitment in each PFM should match that of its corresponding message
	//  - there should be no unpaid for messages

	// extract the commitments from any MsgPayForMessages in the block
	commitments := make(map[string]struct{})
	for _, rawTx := range req.BlockData.Txs {
		tx, err := MalleatedTxDecoder(app.txConfig.TxDecoder())(rawTx)
		if err != nil {
			continue
		}

		for _, msg := range tx.GetMsgs() {
			if sdk.MsgTypeURL(msg) != types.URLMsgPayforMessage {
				continue
			}

			pfm, ok := msg.(*types.MsgPayForMessage)
			if !ok {
				continue
			}

			commitments[string(pfm.MessageShareCommitment)] = struct{}{}
		}

	}

	// quickly compare the number of PFMs and messages, if they aren't
	// identical, then  we already know this block is invalid
	if len(commitments) != len(req.BlockData.Messages.MessagesList) {
		return abci.ResponseProcessProposal{
			Result: abci.ResponseProcessProposal_REJECT,
		}
	}

	// iterate through all of the messages and ensure that a PFM with the exact
	// commitment exists
	for _, msg := range req.BlockData.Messages.MessagesList {
		commit, err := types.CreateCommitment(req.BlockData.OriginalSquareSize, msg.NamespaceId, msg.Data)
		if err != nil {
			app.Logger().Error(
				rejectedPropBlockLog,
				"reason",
				"failure to create commitment for included message",
				"error",
				err.Error(),
			)
			return abci.ResponseProcessProposal{
				Result: abci.ResponseProcessProposal_REJECT,
			}
		}

		if _, has := commitments[string(commit)]; !has {
			app.Logger().Error(rejectedPropBlockLog, "reason", "missing MsgPayForMessage for included message")
			return abci.ResponseProcessProposal{
				Result: abci.ResponseProcessProposal_REJECT,
			}
		}

	}

	data, err := coretypes.DataFromProto(req.BlockData)
	if err != nil {
		app.Logger().Error(rejectedPropBlockLog, "reason", "failure to unmarshal block data:", err)
		return abci.ResponseProcessProposal{
			Result: abci.ResponseProcessProposal_REJECT,
		}
	}

	shares, _, err := data.ComputeShares(req.BlockData.OriginalSquareSize)
	if err != nil {
		app.Logger().Error(rejectedPropBlockLog, "reason", "failure to compute shares from block data:", err)
		return abci.ResponseProcessProposal{
			Result: abci.ResponseProcessProposal_REJECT,
		}
	}

	eds, err := da.ExtendShares(req.BlockData.OriginalSquareSize, shares.RawShares())
	if err != nil {
		app.Logger().Error(
			rejectedPropBlockLog,
			"reason",
			"failure to erasure the data square",
			"error",
			err.Error(),
		)
		return abci.ResponseProcessProposal{
			Result: abci.ResponseProcessProposal_REJECT,
		}
	}

	dah := da.NewDataAvailabilityHeader(eds)

	if !bytes.Equal(dah.Hash(), req.Header.DataHash) {
		app.Logger().Error(
			rejectedPropBlockLog,
			"reason",
			"proposed data root differs from calculated data root",
		)
		return abci.ResponseProcessProposal{
			Result: abci.ResponseProcessProposal_REJECT,
		}
	}

	return abci.ResponseProcessProposal{
		Result: abci.ResponseProcessProposal_ACCEPT,
	}
}

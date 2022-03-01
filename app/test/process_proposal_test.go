package app_test

import (
	"crypto/rand"
	"testing"

	"github.com/celestiaorg/celestia-app/app"
	"github.com/celestiaorg/celestia-app/testutil"
	"github.com/celestiaorg/celestia-app/x/payment/types"
	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/spm/cosmoscmd"
	abci "github.com/tendermint/tendermint/abci/types"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/pkg/consts"
	"github.com/tendermint/tendermint/pkg/da"
	core "github.com/tendermint/tendermint/proto/tendermint/types"
	coretypes "github.com/tendermint/tendermint/types"
)

func TestMessageInclusionCheck(t *testing.T) {
	signer := testutil.GenerateKeyringSigner(t, testAccName)
	info := signer.GetSignerInfo()

	testApp := testutil.SetupTestApp(t, info.GetAddress())

	encConf := cosmoscmd.MakeEncodingConfig(app.ModuleBasics)

	firstValidPFM, msg1 := genRandMsgPayForMessage(t, signer, 8)
	secondValidPFM, msg2 := genRandMsgPayForMessage(t, signer, 8)

	invalidCommitmentPFM, msg3 := genRandMsgPayForMessage(t, signer, 4)
	invalidCommitmentPFM.MessageShareCommitment = tmrand.Bytes(32)

	// block with all messages included
	validData := core.Data{
		Txs: [][]byte{
			buildTx(t, signer, encConf.TxConfig, firstValidPFM),
			buildTx(t, signer, encConf.TxConfig, secondValidPFM),
		},
		Messages: core.Messages{
			MessagesList: []*core.Message{
				{
					NamespaceId: firstValidPFM.MessageNamespaceId,
					Data:        msg1,
				},
				{
					NamespaceId: secondValidPFM.MessageNamespaceId,
					Data:        msg2,
				},
			},
		},
		OriginalSquareSize: 4,
	}

	// block with a missing message
	missingMessageData := core.Data{
		Txs: [][]byte{
			buildTx(t, signer, encConf.TxConfig, firstValidPFM),
			buildTx(t, signer, encConf.TxConfig, secondValidPFM),
		},
		Messages: core.Messages{
			MessagesList: []*core.Message{
				{
					NamespaceId: firstValidPFM.MessageNamespaceId,
					Data:        msg1,
				},
			},
		},
		OriginalSquareSize: 4,
	}

	// block with all messages included, but the commitment is changed
	invalidData := core.Data{
		Txs: [][]byte{
			buildTx(t, signer, encConf.TxConfig, firstValidPFM),
			buildTx(t, signer, encConf.TxConfig, secondValidPFM),
		},
		Messages: core.Messages{
			MessagesList: []*core.Message{
				{
					NamespaceId: firstValidPFM.MessageNamespaceId,
					Data:        msg1,
				},
				{
					NamespaceId: invalidCommitmentPFM.MessageNamespaceId,
					Data:        msg3,
				},
			},
		},
		OriginalSquareSize: 4,
	}

	// block with all messages included
	extraMessageData := core.Data{
		Txs: [][]byte{
			buildTx(t, signer, encConf.TxConfig, firstValidPFM),
		},
		Messages: core.Messages{
			MessagesList: []*core.Message{
				{
					NamespaceId: firstValidPFM.MessageNamespaceId,
					Data:        msg1,
				},
				{
					NamespaceId: secondValidPFM.MessageNamespaceId,
					Data:        msg2,
				},
			},
		},
		OriginalSquareSize: 4,
	}

	type test struct {
		input          abci.RequestProcessProposal
		expectedResult abci.ResponseProcessProposal_Result
	}

	tests := []test{
		{
			input: abci.RequestProcessProposal{
				BlockData: &validData,
			},
			expectedResult: abci.ResponseProcessProposal_ACCEPT,
		},
		{
			input: abci.RequestProcessProposal{
				BlockData: &missingMessageData,
			},
			expectedResult: abci.ResponseProcessProposal_REJECT,
		},
		{
			input: abci.RequestProcessProposal{
				BlockData: &invalidData,
			},
			expectedResult: abci.ResponseProcessProposal_REJECT,
		},
		{
			input: abci.RequestProcessProposal{
				BlockData: &extraMessageData,
			},
			expectedResult: abci.ResponseProcessProposal_REJECT,
		},
	}

	for _, tt := range tests {
		data, err := coretypes.DataFromProto(tt.input.BlockData)
		require.NoError(t, err)

		shares, _, err := data.ComputeShares(tt.input.BlockData.OriginalSquareSize)
		require.NoError(t, err)

		rawShares := shares.RawShares()

		require.NoError(t, err)
		eds, err := da.ExtendShares(tt.input.BlockData.OriginalSquareSize, rawShares)
		require.NoError(t, err)
		dah := da.NewDataAvailabilityHeader(eds)
		tt.input.Header.DataHash = dah.Hash()
		res := testApp.ProcessProposal(tt.input)
		assert.Equal(t, tt.expectedResult, res.Result)
	}

}

func genRandMsgPayForMessage(t *testing.T, signer *types.KeyringSigner, squareSize uint64) (*types.MsgPayForMessage, []byte) {
	ns := make([]byte, consts.NamespaceSize)
	_, err := rand.Read(ns)
	require.NoError(t, err)

	message := make([]byte, 20)
	_, err = rand.Read(message)
	require.NoError(t, err)

	commit, err := types.CreateCommitment(squareSize, ns, message)
	require.NoError(t, err)

	pfm := types.MsgPayForMessage{
		MessageShareCommitment: commit,
		MessageNamespaceId:     ns,
	}

	return &pfm, message
}

func buildTx(t *testing.T, signer *types.KeyringSigner, txCfg client.TxConfig, msg sdk.Msg) []byte {
	tx, err := signer.BuildSignedTx(signer.NewTxBuilder(), msg)
	require.NoError(t, err)

	rawTx, err := txCfg.TxEncoder()(tx)
	require.NoError(t, err)

	return rawTx
}

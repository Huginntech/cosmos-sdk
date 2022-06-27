package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	"github.com/cosmos/cosmos-sdk/x/staking/keeper"
	"github.com/cosmos/cosmos-sdk/x/staking/testutil"
	"github.com/cosmos/cosmos-sdk/x/staking/types"
)

// addrs, _, validators := createValidators(suite.T(), ctx, app, []int64{9, 8, 7})
// header := tmproto.Header{
// 	ChainID: "HelloChain",
// 	Height:  5,
// }

// // sort a copy of the validators, so that original validators does not
// // have its order changed
// sortedVals := make([]types.Validator, len(validators))
// copy(sortedVals, validators)
// hi := types.NewHistoricalInfo(header, sortedVals, suite.stakingKeeper.PowerReduction(ctx))
// suite.stakingKeeper.SetHistoricalInfo(ctx, 5, &hi)

type KeeperTestSuite struct {
	suite.Suite

	ctx         sdk.Context
	addrs       []sdk.AccAddress
	vals        []types.Validator
	queryClient types.QueryClient
	msgServer   types.MsgServer

	app         *runtime.App
	codec       codec.Codec
	txConfig    client.TxConfig
	legacyAmino *codec.LegacyAmino

	accountKeeper authkeeper.AccountKeeper
	bankKeeper    bankkeeper.Keeper
	stakingKeeper *keeper.Keeper
}

func (suite *KeeperTestSuite) SetupTest() {
	var interfaceRegistry codectypes.InterfaceRegistry

	app, err := simtestutil.Setup(
		testutil.AppConfig,
		&interfaceRegistry,
		&suite.codec,
		&suite.txConfig,
		&suite.legacyAmino,
		&suite.accountKeeper,
		&suite.bankKeeper,
		&suite.stakingKeeper,
	)

	suite.Require().NoError(err)
	suite.app = app
	suite.ctx = app.BaseApp.NewContext(false, tmproto.Header{})

	querier := keeper.Querier{Keeper: suite.stakingKeeper}
	queryHelper := baseapp.NewQueryServerTestHelper(suite.ctx, interfaceRegistry)
	types.RegisterQueryServer(queryHelper, querier)
	suite.queryClient = types.NewQueryClient(queryHelper)

	suite.msgServer = keeper.NewMsgServerImpl(suite.stakingKeeper)
}

func TestKeeperTestSuite(t *testing.T) {
	suite.Run(t, new(KeeperTestSuite))
}

func (suite *KeeperTestSuite) TestParams() {
	expParams := types.DefaultParams()

	// check that the empty keeper loads the default
	resParams := suite.stakingKeeper.GetParams(suite.ctx)
	suite.Require().True(expParams.Equal(resParams))

	// modify a params, save, and retrieve
	expParams.MaxValidators = 777
	suite.stakingKeeper.SetParams(suite.ctx, expParams)
	resParams = suite.stakingKeeper.GetParams(suite.ctx)
	suite.Require().True(expParams.Equal(resParams))
}

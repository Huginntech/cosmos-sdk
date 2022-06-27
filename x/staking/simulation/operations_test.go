package simulation_test

import (
	"math/big"
	"math/rand"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/runtime"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	abci "github.com/tendermint/tendermint/abci/types"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/testutil"
	distributionkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	mintkeeper "github.com/cosmos/cosmos-sdk/x/mint/keeper"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/cosmos/cosmos-sdk/x/staking/keeper"
	"github.com/cosmos/cosmos-sdk/x/staking/simulation"
	"github.com/cosmos/cosmos-sdk/x/staking/teststaking"
	"github.com/cosmos/cosmos-sdk/x/staking/testutil"
	"github.com/cosmos/cosmos-sdk/x/staking/types"
)

type SimTestSuite struct {
	suite.Suite

	r   *rand.Rand
	ctx sdk.Context

	app      *runtime.App
	codec    codec.Codec
	txConfig client.TxConfig

	accountKeeper authkeeper.AccountKeeper
	bankKeeper    bankkeeper.Keeper
	stakingKeeper *keeper.Keeper
	distrKeeper   distributionkeeper.Keeper
	mintKeeper    mintkeeper.Keeper

	accs []simtypes.Account
}

func (suite *SimTestSuite) SetupTest() {
	suite.T().Parallel()

	sdk.DefaultPowerReduction = sdk.NewIntFromBigInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))

	app, err := simtestutil.Setup(
		testutil.AppConfig,
		&suite.codec,
		&suite.txConfig,
		&suite.accountKeeper,
		&suite.bankKeeper,
		&suite.stakingKeeper,
		&suite.mintKeeper,
		&suite.distrKeeper,
	)
	suite.Require().NoError(err)

	suite.app = app
	suite.ctx = app.BaseApp.NewContext(false, tmproto.Header{})

	s := rand.NewSource(100)
	suite.r = rand.New(s)
	suite.accs = simtypes.RandomAccounts(suite.r, 4)

	suite.mintKeeper.SetParams(suite.ctx, minttypes.DefaultParams())
	suite.mintKeeper.SetMinter(suite.ctx, minttypes.DefaultInitialMinter())

	initAmt := suite.stakingKeeper.TokensFromConsensusPower(suite.ctx, 200)
	initCoins := sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, initAmt))

	// add coins to the accounts
	for _, account := range suite.accs {
		acc := suite.accountKeeper.NewAccountWithAddress(suite.ctx, account.Address)
		suite.accountKeeper.SetAccount(suite.ctx, acc)
		suite.Require().NoError(banktestutil.FundAccount(suite.bankKeeper, suite.ctx, account.Address, initCoins))
	}
}

func TestSimTestSuite(t *testing.T) {
	suite.Run(t, new(SimTestSuite))
}

// TestWeightedOperations tests the weights of the operations.
func (suite *SimTestSuite) TestWeightedOperations() {
	suite.ctx.WithChainID("test-chain")

	appParams := make(simtypes.AppParams)

	weightesOps := simulation.WeightedOperations(appParams,
		suite.codec,
		suite.accountKeeper,
		suite.bankKeeper,
		suite.stakingKeeper,
	)

	expected := []struct {
		weight     int
		opMsgRoute string
		opMsgName  string
	}{
		{simtestutil.DefaultWeightMsgCreateValidator, types.ModuleName, types.TypeMsgCreateValidator},
		{simtestutil.DefaultWeightMsgEditValidator, types.ModuleName, types.TypeMsgEditValidator},
		{simtestutil.DefaultWeightMsgDelegate, types.ModuleName, types.TypeMsgDelegate},
		{simtestutil.DefaultWeightMsgUndelegate, types.ModuleName, types.TypeMsgUndelegate},
		{simtestutil.DefaultWeightMsgBeginRedelegate, types.ModuleName, types.TypeMsgBeginRedelegate},
		{simtestutil.DefaultWeightMsgCancelUnbondingDelegation, types.ModuleName, types.TypeMsgCancelUnbondingDelegation},
	}

	for i, w := range weightesOps {
		operationMsg, _, err := w.Op()(suite.r, suite.app.BaseApp, suite.ctx, suite.accs, suite.ctx.ChainID())
		suite.Require().NoError(err)

		// the following checks are very much dependent from the ordering of the output given
		// by WeightedOperations. if the ordering in WeightedOperations changes some tests
		// will fail
		suite.Require().Equal(expected[i].weight, w.Weight(), "weight should be the same")
		suite.Require().Equal(expected[i].opMsgRoute, operationMsg.Route, "route should be the same")
		suite.Require().Equal(expected[i].opMsgName, operationMsg.Name, "operation Msg name should be the same")
	}
}

// TestSimulateMsgCreateValidator tests the normal scenario of a valid message of type TypeMsgCreateValidator.
// Abonormal scenarios, where the message are created by an errors are not tested here.
func (suite *SimTestSuite) TestSimulateMsgCreateValidator() {
	app, ctx, accounts := suite.app, suite.ctx, suite.accs

	// begin a new block
	app.BeginBlock(abci.RequestBeginBlock{Header: tmproto.Header{Height: app.LastBlockHeight() + 1, AppHash: app.LastCommitID().Hash}})

	// execute operation
	op := simulation.SimulateMsgCreateValidator(suite.txConfig, suite.accountKeeper, suite.bankKeeper, suite.stakingKeeper)
	operationMsg, futureOperations, err := op(suite.r, app.BaseApp, ctx, accounts, "")
	suite.Require().NoError(err)

	var msg types.MsgCreateValidator
	types.ModuleCdc.UnmarshalJSON(operationMsg.Msg, &msg)

	suite.Require().True(operationMsg.OK)
	suite.Require().Equal("0.910000000000000000", msg.Commission.MaxChangeRate.String())
	suite.Require().Equal("0.910000000000000000", msg.Commission.MaxRate.String())
	suite.Require().Equal("0.481079303822955765", msg.Commission.Rate.String())
	suite.Require().Equal(types.TypeMsgCreateValidator, msg.Type())
	suite.Require().Equal([]byte{0xa, 0x20, 0xfe, 0x16, 0xda, 0xac, 0x46, 0x9, 0xd7, 0x12, 0x2c, 0x93, 0x31, 0xe1, 0x5b, 0x32, 0xde, 0x31, 0x59, 0xc4, 0x43, 0xc3, 0x9, 0x4b, 0x2e, 0xa4, 0xa6, 0x2d, 0xca, 0x76, 0x38, 0xc7, 0x51, 0x9c}, msg.Pubkey.Value)
	suite.Require().Equal("cosmos1pjdrdhzq6hea7jl9t6nsdp35v2sgg899d66xvh", msg.DelegatorAddress)
	suite.Require().Equal("cosmosvaloper1pjdrdhzq6hea7jl9t6nsdp35v2sgg899gwwnqy", msg.ValidatorAddress)
	suite.Require().Len(futureOperations, 0)
}

// TestSimulateMsgCancelUnbondingDelegation tests the normal scenario of a valid message of type TypeMsgCancelUnbondingDelegation.
// Abonormal scenarios, where the message is
func (suite *SimTestSuite) TestSimulateMsgCancelUnbondingDelegation() {
	app, ctx, accounts := suite.app, suite.ctx, suite.accs

	blockTime := time.Now().UTC()
	ctx = ctx.WithBlockTime(blockTime)

	// setup accounts[0] as validator
	validator0 := getTestingValidator0(suite.T(), suite.stakingKeeper, ctx, accounts)

	// setup delegation
	delTokens := suite.stakingKeeper.TokensFromConsensusPower(ctx, 2)
	validator0, issuedShares := validator0.AddTokensFromDel(delTokens)
	delegator := accounts[1]
	delegation := types.NewDelegation(delegator.Address, validator0.GetOperator(), issuedShares)
	suite.stakingKeeper.SetDelegation(ctx, delegation)
	suite.distrKeeper.SetDelegatorStartingInfo(ctx, validator0.GetOperator(), delegator.Address, distrtypes.NewDelegatorStartingInfo(2, sdk.OneDec(), 200))

	setupValidatorRewards(suite.distrKeeper, ctx, validator0.GetOperator())

	// unbonding delegation
	udb := types.NewUnbondingDelegation(delegator.Address, validator0.GetOperator(), app.LastBlockHeight(), blockTime.Add(2*time.Minute), delTokens)
	suite.stakingKeeper.SetUnbondingDelegation(ctx, udb)
	setupValidatorRewards(suite.distrKeeper, ctx, validator0.GetOperator())

	// begin a new block
	app.BeginBlock(abci.RequestBeginBlock{Header: tmproto.Header{Height: app.LastBlockHeight() + 1, AppHash: app.LastCommitID().Hash, Time: blockTime}})

	// execute operation
	op := simulation.SimulateMsgCancelUnbondingDelegate(suite.txConfig, suite.accountKeeper, suite.bankKeeper, suite.stakingKeeper)
	accounts = []simtypes.Account{accounts[1]}
	operationMsg, futureOperations, err := op(suite.r, app.BaseApp, ctx, accounts, "")
	suite.Require().NoError(err)

	var msg types.MsgCancelUnbondingDelegation
	types.ModuleCdc.UnmarshalJSON(operationMsg.Msg, &msg)

	suite.Require().True(operationMsg.OK)
	suite.Require().Equal(types.TypeMsgCancelUnbondingDelegation, msg.Type())
	suite.Require().Equal(delegator.Address.String(), msg.DelegatorAddress)
	suite.Require().Equal(validator0.GetOperator().String(), msg.ValidatorAddress)
	suite.Require().Len(futureOperations, 0)
}

// TestSimulateMsgEditValidator tests the normal scenario of a valid message of type TypeMsgEditValidator.
// Abonormal scenarios, where the message is created by an errors are not tested here.
func (suite *SimTestSuite) TestSimulateMsgEditValidator() {
	app, ctx, accounts := suite.app, suite.ctx, suite.accs

	blockTime := time.Now().UTC()
	ctx = ctx.WithBlockTime(blockTime)

	// setup accounts[0] as validator
	_ = getTestingValidator0(suite.T(), suite.stakingKeeper, ctx, accounts)

	// begin a new block
	app.BeginBlock(abci.RequestBeginBlock{Header: tmproto.Header{Height: app.LastBlockHeight() + 1, AppHash: app.LastCommitID().Hash, Time: blockTime}})

	// execute operation
	op := simulation.SimulateMsgEditValidator(suite.txConfig, suite.accountKeeper, suite.bankKeeper, suite.stakingKeeper)
	operationMsg, futureOperations, err := op(suite.r, app.BaseApp, ctx, accounts, "")
	suite.Require().NoError(err)

	var msg types.MsgEditValidator
	types.ModuleCdc.UnmarshalJSON(operationMsg.Msg, &msg)

	suite.Require().True(operationMsg.OK)
	suite.Require().Equal("1.000000000000000000", msg.CommissionRate.String())
	suite.Require().Equal("zhXBSyWnLq", msg.Description.Moniker)
	suite.Require().Equal("AYwLJYQzRs", msg.Description.Identity)
	suite.Require().Equal("yfMcIWVPhu", msg.Description.Website)
	suite.Require().Equal("PaVidkRnZv", msg.Description.SecurityContact)
	suite.Require().Equal(types.TypeMsgEditValidator, msg.Type())
	suite.Require().Equal("cosmosvaloper16rr9ks0vjjfq758g9mqlye7zntq378xg9j7lrd", msg.ValidatorAddress)
	suite.Require().Len(futureOperations, 0)
}

// TestSimulateMsgDelegate tests the normal scenario of a valid message of type TypeMsgDelegate.
// Abonormal scenarios, where the message is created by an errors are not tested here.
func (suite *SimTestSuite) TestSimulateMsgDelegate() {
	app, ctx, accounts := suite.app, suite.ctx, suite.accs

	blockTime := time.Now().UTC()
	ctx = ctx.WithBlockTime(blockTime)

	// execute operation
	op := simulation.SimulateMsgDelegate(suite.txConfig, suite.accountKeeper, suite.bankKeeper, suite.stakingKeeper)
	operationMsg, futureOperations, err := op(suite.r, app.BaseApp, ctx, accounts, "")
	suite.Require().NoError(err)

	var msg types.MsgDelegate
	types.ModuleCdc.UnmarshalJSON(operationMsg.Msg, &msg)

	suite.Require().True(operationMsg.OK)
	suite.Require().Equal("cosmos1pjdrdhzq6hea7jl9t6nsdp35v2sgg899d66xvh", msg.DelegatorAddress)
	suite.Require().Equal("182870851461110560141", msg.Amount.Amount.String())
	suite.Require().Equal("stake", msg.Amount.Denom)
	suite.Require().Equal(types.TypeMsgDelegate, msg.Type())
	suite.Require().Equal("cosmosvaloper1kfe9j73l43x6vjdl0zdx77j2azxdd8t5z2tt20", msg.ValidatorAddress)
	suite.Require().Len(futureOperations, 0)
}

// TestSimulateMsgUndelegate tests the normal scenario of a valid message of type TypeMsgUndelegate.
// Abonormal scenarios, where the message is created by an errors are not tested here.
func (suite *SimTestSuite) TestSimulateMsgUndelegate() {
	app, ctx, accounts := suite.app, suite.ctx, suite.accs

	blockTime := time.Now().UTC()
	ctx = ctx.WithBlockTime(blockTime)

	// setup accounts[0] as validator
	validator0 := getTestingValidator0(suite.T(), suite.stakingKeeper, ctx, accounts)

	// setup delegation
	delTokens := suite.stakingKeeper.TokensFromConsensusPower(ctx, 2)
	validator0, issuedShares := validator0.AddTokensFromDel(delTokens)
	delegator := accounts[1]
	delegation := types.NewDelegation(delegator.Address, validator0.GetOperator(), issuedShares)
	suite.stakingKeeper.SetDelegation(ctx, delegation)
	suite.distrKeeper.SetDelegatorStartingInfo(ctx, validator0.GetOperator(), delegator.Address, distrtypes.NewDelegatorStartingInfo(2, sdk.OneDec(), 200))

	setupValidatorRewards(suite.distrKeeper, ctx, validator0.GetOperator())

	// begin a new block
	app.BeginBlock(abci.RequestBeginBlock{Header: tmproto.Header{Height: app.LastBlockHeight() + 1, AppHash: app.LastCommitID().Hash, Time: blockTime}})

	// execute operation
	op := simulation.SimulateMsgUndelegate(suite.txConfig, suite.accountKeeper, suite.bankKeeper, suite.stakingKeeper)
	operationMsg, futureOperations, err := op(suite.r, app.BaseApp, ctx, accounts, "")
	suite.Require().NoError(err)

	var msg types.MsgUndelegate
	types.ModuleCdc.UnmarshalJSON(operationMsg.Msg, &msg)

	suite.Require().True(operationMsg.OK)
	suite.Require().Equal("cosmos1ghekyjucln7y67ntx7cf27m9dpuxxemn4c8g4r", msg.DelegatorAddress)
	suite.Require().Equal("280623462081924937", msg.Amount.Amount.String())
	suite.Require().Equal("stake", msg.Amount.Denom)
	suite.Require().Equal(types.TypeMsgUndelegate, msg.Type())
	suite.Require().Equal("cosmosvaloper1p8wcgrjr4pjju90xg6u9cgq55dxwq8j7epjs3u", msg.ValidatorAddress)
	suite.Require().Len(futureOperations, 0)
}

// TestSimulateMsgBeginRedelegate tests the normal scenario of a valid message of type TypeMsgBeginRedelegate.
// Abonormal scenarios, where the message is created by an errors, are not tested here.
func (suite *SimTestSuite) TestSimulateMsgBeginRedelegate() {
	app, ctx, accounts := suite.app, suite.ctx, suite.accs

	blockTime := time.Now().UTC()
	ctx = ctx.WithBlockTime(blockTime)

	// setup accounts[0] as validator0 and accounts[1] as validator1
	validator0 := getTestingValidator0(suite.T(), suite.stakingKeeper, ctx, accounts)
	validator1 := getTestingValidator1(suite.T(), suite.stakingKeeper, ctx, accounts)

	delTokens := suite.stakingKeeper.TokensFromConsensusPower(ctx, 2)
	validator0, issuedShares := validator0.AddTokensFromDel(delTokens)

	// setup accounts[2] as delegator
	delegator := accounts[2]
	delegation := types.NewDelegation(delegator.Address, validator1.GetOperator(), issuedShares)
	suite.stakingKeeper.SetDelegation(ctx, delegation)
	suite.distrKeeper.SetDelegatorStartingInfo(ctx, validator1.GetOperator(), delegator.Address, distrtypes.NewDelegatorStartingInfo(2, sdk.OneDec(), 200))

	setupValidatorRewards(suite.distrKeeper, ctx, validator0.GetOperator())
	setupValidatorRewards(suite.distrKeeper, ctx, validator1.GetOperator())

	// begin a new block
	app.BeginBlock(abci.RequestBeginBlock{Header: tmproto.Header{Height: app.LastBlockHeight() + 1, AppHash: app.LastCommitID().Hash, Time: blockTime}})

	// execute operation
	op := simulation.SimulateMsgBeginRedelegate(suite.txConfig, suite.accountKeeper, suite.bankKeeper, suite.stakingKeeper)
	operationMsg, futureOperations, err := op(suite.r, app.BaseApp, ctx, accounts, "")
	suite.Require().NoError(err)

	var msg types.MsgBeginRedelegate
	types.ModuleCdc.UnmarshalJSON(operationMsg.Msg, &msg)

	suite.Require().True(operationMsg.OK)
	suite.Require().Equal("cosmos1092v0qgulpejj8y8hs6dmlw82x9gv8f7jfc7jl", msg.DelegatorAddress)
	suite.Require().Equal("1883752832348281252", msg.Amount.Amount.String())
	suite.Require().Equal("stake", msg.Amount.Denom)
	suite.Require().Equal(types.TypeMsgBeginRedelegate, msg.Type())
	suite.Require().Equal("cosmosvaloper1gnkw3uqzflagcqn6ekjwpjanlne928qhruemah", msg.ValidatorDstAddress)
	suite.Require().Equal("cosmosvaloper1kk653svg7ksj9fmu85x9ygj4jzwlyrgs89nnn2", msg.ValidatorSrcAddress)
	suite.Require().Len(futureOperations, 0)
}

func getTestingValidator0(t *testing.T, stakingKeeper *keeper.Keeper, ctx sdk.Context, accounts []simtypes.Account) types.Validator {
	commission0 := types.NewCommission(sdk.ZeroDec(), sdk.OneDec(), sdk.OneDec())
	return getTestingValidator(t, stakingKeeper, ctx, accounts, commission0, 0)
}

func getTestingValidator1(t *testing.T, stakingKeeper *keeper.Keeper, ctx sdk.Context, accounts []simtypes.Account) types.Validator {
	commission1 := types.NewCommission(sdk.ZeroDec(), sdk.ZeroDec(), sdk.ZeroDec())
	return getTestingValidator(t, stakingKeeper, ctx, accounts, commission1, 1)
}

func getTestingValidator(t *testing.T, stakingKeeper *keeper.Keeper, ctx sdk.Context, accounts []simtypes.Account, commission types.Commission, n int) types.Validator {
	account := accounts[n]
	valPubKey := account.PubKey
	valAddr := sdk.ValAddress(account.PubKey.Address().Bytes())
	validator := teststaking.NewValidator(t, valAddr, valPubKey)
	validator, err := validator.SetInitialCommission(commission)
	require.NoError(t, err)

	validator.DelegatorShares = sdk.NewDec(100)
	validator.Tokens = stakingKeeper.TokensFromConsensusPower(ctx, 100)

	stakingKeeper.SetValidator(ctx, validator)

	return validator
}

func setupValidatorRewards(distrKeeper distrkeeper.Keeper, ctx sdk.Context, valAddress sdk.ValAddress) {
	decCoins := sdk.DecCoins{sdk.NewDecCoinFromDec(sdk.DefaultBondDenom, sdk.OneDec())}
	historicalRewards := distrtypes.NewValidatorHistoricalRewards(decCoins, 2)
	distrKeeper.SetValidatorHistoricalRewards(ctx, valAddress, 2, historicalRewards)
	// setup current revards
	currentRewards := distrtypes.NewValidatorCurrentRewards(decCoins, 3)
	distrKeeper.SetValidatorCurrentRewards(ctx, valAddress, currentRewards)
}

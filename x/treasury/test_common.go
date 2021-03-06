package treasury

import (
	"terra/types/assets"
	"terra/types/mock"
	"terra/x/market"
	"terra/x/mint"
	"terra/x/oracle"

	"testing"
	"time"

	"github.com/stretchr/testify/require"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/secp256k1"
	dbm "github.com/tendermint/tendermint/libs/db"
	"github.com/tendermint/tendermint/libs/log"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/store"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/bank"
	"github.com/cosmos/cosmos-sdk/x/params"
)

var (
	pubKeys = []crypto.PubKey{
		secp256k1.GenPrivKey().PubKey(),
		secp256k1.GenPrivKey().PubKey(),
		secp256k1.GenPrivKey().PubKey(),
	}

	addrs = []sdk.AccAddress{
		sdk.AccAddress(pubKeys[0].Address()),
		sdk.AccAddress(pubKeys[1].Address()),
		sdk.AccAddress(pubKeys[2].Address()),
	}

	mLunaAmt = sdk.NewInt(1000).MulRaw(assets.MicroUnit)
)

type testInput struct {
	ctx            sdk.Context
	cdc            *codec.Codec
	bankKeeper     bank.Keeper
	oracleKeeper   oracle.Keeper
	marketKeeper   market.Keeper
	mintKeeper     mint.Keeper
	treasuryKeeper Keeper
}

func newTestCodec() *codec.Codec {
	cdc := codec.New()

	bank.RegisterCodec(cdc)
	RegisterCodec(cdc)
	auth.RegisterCodec(cdc)
	sdk.RegisterCodec(cdc)
	codec.RegisterCrypto(cdc)

	return cdc
}

func createTestInput(t *testing.T) testInput {
	keyAcc := sdk.NewKVStoreKey(auth.StoreKey)
	keyParams := sdk.NewKVStoreKey(params.StoreKey)
	tKeyParams := sdk.NewTransientStoreKey(params.TStoreKey)
	keyMint := sdk.NewKVStoreKey(mint.StoreKey)
	keyOracle := sdk.NewKVStoreKey(oracle.StoreKey)
	keyTreasury := sdk.NewKVStoreKey(StoreKey)

	cdc := newTestCodec()
	db := dbm.NewMemDB()
	ms := store.NewCommitMultiStore(db)
	ctx := sdk.NewContext(ms, abci.Header{Time: time.Now().UTC()}, false, log.NewNopLogger())

	ms.MountStoreWithDB(keyAcc, sdk.StoreTypeIAVL, db)
	ms.MountStoreWithDB(tKeyParams, sdk.StoreTypeTransient, db)
	ms.MountStoreWithDB(keyParams, sdk.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyMint, sdk.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyOracle, sdk.StoreTypeIAVL, db)
	ms.MountStoreWithDB(keyTreasury, sdk.StoreTypeIAVL, db)

	require.NoError(t, ms.LoadLatestVersion())

	paramsKeeper := params.NewKeeper(cdc, keyParams, tKeyParams)
	accKeeper := auth.NewAccountKeeper(
		cdc,
		keyAcc,
		paramsKeeper.Subspace(auth.DefaultParamspace),
		auth.ProtoBaseAccount,
	)

	bankKeeper := bank.NewBaseKeeper(
		accKeeper,
		paramsKeeper.Subspace(bank.DefaultParamspace),
		bank.DefaultCodespace,
	)

	mintKeeper := mint.NewKeeper(cdc, keyMint, bankKeeper, accKeeper)

	valset := mock.NewMockValSet()
	for _, addr := range addrs {
		err2 := mintKeeper.Mint(ctx, addr, sdk.NewCoin(assets.MicroLunaDenom, mLunaAmt))
		require.NoError(t, err2)

		// Add validators
		validator := mock.NewMockValidator(sdk.ValAddress(addr.Bytes()), mLunaAmt)
		valset.Validators = append(valset.Validators, validator)
	}

	oracleKeeper := oracle.NewKeeper(
		cdc,
		keyOracle,
		valset,
		paramsKeeper.Subspace(oracle.DefaultParamspace),
	)

	marketKeeper := market.NewKeeper(oracleKeeper, mintKeeper)

	treasuryKeeper := NewKeeper(
		cdc,
		keyTreasury,
		valset,
		mintKeeper,
		marketKeeper,
		paramsKeeper.Subspace(DefaultParamspace),
	)

	InitGenesis(ctx, treasuryKeeper, DefaultGenesisState())

	return testInput{ctx, cdc, bankKeeper, oracleKeeper, marketKeeper, mintKeeper, treasuryKeeper}
}

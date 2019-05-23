package asset

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// GenesisState - all asset state that must be provided at genesis
type GenesisState struct {
	Params Params  `json:"params"`
	Tokens []Token `json:"tokens"`
}

// NewGenesisState - Create a new genesis state
func NewGenesisState(params Params, tokens []Token) GenesisState {
	return GenesisState{
		Params: params,
		Tokens: tokens,
	}
}

// DefaultGenesisState - Return a default genesis state
func DefaultGenesisState() GenesisState {
	return NewGenesisState(DefaultParams(), []Token{})
}

// InitGenesis - Init store state from genesis data
func InitGenesis(ctx sdk.Context, tk TokenKeeper, data GenesisState) {
	tk.SetParams(ctx, data.Params)
}

// ExportGenesis returns a GenesisState for a given context and keeper
func ExportGenesis(ctx sdk.Context, tk TokenKeeper) GenesisState {
	return NewGenesisState(tk.GetParams(ctx), tk.GetAllTokens(ctx))
}

// ValidateGenesis performs basic validation of asset genesis data returning an
// error for any failed validation criteria.
func ValidateGenesis(data GenesisState) error {
	//TODO:
	return nil
}
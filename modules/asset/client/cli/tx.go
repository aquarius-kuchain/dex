package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/context"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/auth/client/utils"

	"github.com/coinexchain/dex/modules/asset/internal/types"
	dex "github.com/coinexchain/dex/types"
)

var issueTokenFlags = []string{
	flagName,
	flagSymbol,
	flagTotalSupply,
	flagMintable,
	flagBurnable,
	flagAddrForbiddable,
	flagTokenForbiddable,
	flagTokenURL,
	flagTokenDescription,
}

// get the root tx command of this module
func GetTxCmd(cdc *codec.Codec) *cobra.Command {
	assTxCmd := &cobra.Command{
		Use:   types.ModuleName,
		Short: "Asset transactions subcommands",
	}

	assTxCmd.AddCommand(client.PostCommands(
		IssueTokenCmd(types.QuerierRoute, cdc),
		TransferOwnershipCmd(cdc),
		MintTokenCmd(cdc),
		BurnTokenCmd(cdc),
		ForbidTokenCmd(cdc),
		UnForbidTokenCmd(cdc),
		AddTokenWhitelistCmd(cdc),
		RemoveTokenWhitelistCmd(cdc),
		ForbidAddrCmd(cdc),
		UnForbidAddrCmd(cdc),
		ModifyTokenURLCmd(cdc),
		ModifyTokenDescriptionCmd(cdc),
	)...)

	return assTxCmd
}

// IssueTokenCmd will create a issue token tx and sign.
func IssueTokenCmd(queryRoute string, cdc *codec.Codec) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "issue-token",
		Short: "Create and sign a issue-token tx",
		Long: strings.TrimSpace(
			`Create and sign a issue-token tx, broadcast to nodes.

Example:
$ cetcli tx asset issue-token --name="ABC Token" \
	--symbol="abc" \
	--total-supply=2100000000000000 \
	--mintable=false \
	--burnable=true \
	--addr-forbiddable=false \
	--token-forbiddable=false \
	--url="www.abc.org" \
	--description="token abc is a example token" \
	--from mykey
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			cliCtx := context.NewCLIContext().WithCodec(cdc)
			tokenOwner := cliCtx.GetFromAddress()
			msg, err := parseIssueFlags(tokenOwner)
			if err != nil {
				return err
			}

			if err = msg.ValidateBasic(); err != nil {
				return err
			}

			bz, err := cdc.MarshalJSON(types.NewQueryAssetParams(msg.Symbol))
			if err != nil {
				return err
			}
			route := fmt.Sprintf("custom/%s/%s", queryRoute, types.QueryToken)
			if res, _, _ := cliCtx.QueryWithData(route, bz); res != nil {
				return fmt.Errorf("token symbol already exists，please query tokens and issue another symbol")
			}

			// ensure account has enough coins
			account, err := auth.NewAccountRetriever(cliCtx).GetAccount(tokenOwner)
			if err != nil {
				return err
			}
			issueFee := dex.NewCetCoins(types.IssueTokenFee)
			if len(msg.Symbol) == types.RareSymbolLength {
				issueFee = dex.NewCetCoins(types.IssueRareTokenFee)
			}
			if !account.GetCoins().IsAllGTE(issueFee) {
				return fmt.Errorf("address %s doesn't have enough cet to issue token", tokenOwner)
			}

			// build and sign the transaction, then broadcast to Tendermint
			txBldr := auth.NewTxBuilderFromCLI().WithTxEncoder(utils.GetTxEncoder(cdc))
			return utils.GenerateOrBroadcastMsgs(cliCtx, txBldr, []sdk.Msg{msg})
		},
	}

	cmd.Flags().String(flagName, "", "issue token name is limited to 32 unicode characters")
	cmd.Flags().String(flagSymbol, "", "issue token symbol is limited to [a-z][a-z0-9]{1,7}")
	cmd.Flags().Int64(flagTotalSupply, 0, "the total supply for token can have a maximum of "+
		"8 digits of decimal and is boosted by 1e8 in order to store as int64. "+
		"The amount before boosting should not exceed 90 billion.")
	cmd.Flags().Bool(flagMintable, false, "whether the token could be minted")
	cmd.Flags().Bool(flagBurnable, true, "whether the token could be burned")
	cmd.Flags().Bool(flagAddrForbiddable, false, "whether the token holder address can be forbidden by token owner")
	cmd.Flags().Bool(flagTokenForbiddable, false, "whether the token can be forbidden")
	cmd.Flags().String(flagTokenURL, "", "url of token website")
	cmd.Flags().String(flagTokenDescription, "", "description of token info")

	for _, flag := range issueTokenFlags {
		_ = cmd.MarkFlagRequired(flag)
	}

	return cmd
}

var transferOwnershipFlags = []string{
	flagSymbol,
	flagNewOwner,
}

// TransferOwnershipCmd will create a transfer token  owner tx and sign.
func TransferOwnershipCmd(cdc *codec.Codec) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "transfer-ownership",
		Short: "Create and sign a transfer-ownership tx",
		Long: strings.TrimSpace(
			`Create and sign a transfer-ownership tx, broadcast to nodes.

Example:
$ cetcli tx asset transfer-ownership --symbol="abc" \
	--new-owner=newkey \
	--from mykey
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			cliCtx := context.NewCLIContext().WithCodec(cdc)
			originalOwner := cliCtx.GetFromAddress()
			msg, err := parseTransferOwnershipFlags(originalOwner)
			if err != nil {
				return err
			}

			if err = msg.ValidateBasic(); err != nil {
				return err
			}

			if _, err = auth.NewAccountRetriever(cliCtx).GetAccount(originalOwner); err != nil {
				return err
			}

			// build and sign the transaction, then broadcast to Tendermint
			txBldr := auth.NewTxBuilderFromCLI().WithTxEncoder(utils.GetTxEncoder(cdc))
			return utils.GenerateOrBroadcastMsgs(cliCtx, txBldr, []sdk.Msg{msg})
		},
	}

	cmd.Flags().String(flagSymbol, "", "which token`s ownership be transferred")
	cmd.Flags().String(flagNewOwner, "", "who do you want to transfer to ?")

	_ = cmd.MarkFlagRequired(client.FlagFrom)
	for _, flag := range transferOwnershipFlags {
		_ = cmd.MarkFlagRequired(flag)
	}

	return cmd
}

var mintTokenFlags = []string{
	flagSymbol,
	flagAmount,
}

// MintTokenCmd will create a mint token tx and sign.
func MintTokenCmd(cdc *codec.Codec) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mint-token",
		Short: "Create and sign a mint token tx",
		Long: strings.TrimSpace(
			`Create and sign a mint token tx, broadcast to nodes.

Example:
$ cetcli tx asset mint-token --symbol="abc" \
	--amount=10000000000000000 \
	--from mykey
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			cliCtx := context.NewCLIContext().WithCodec(cdc)
			owner := cliCtx.GetFromAddress()
			msg, err := parseMintTokenFlags(owner)
			if err != nil {
				return err
			}

			if err = msg.ValidateBasic(); err != nil {
				return err
			}

			if _, err = auth.NewAccountRetriever(cliCtx).GetAccount(owner); err != nil {
				return err
			}

			// build and sign the transaction, then broadcast to Tendermint
			txBldr := auth.NewTxBuilderFromCLI().WithTxEncoder(utils.GetTxEncoder(cdc))
			return utils.GenerateOrBroadcastMsgs(cliCtx, txBldr, []sdk.Msg{msg})
		},
	}

	cmd.Flags().String(flagSymbol, "", "which token will be minted")
	cmd.Flags().String(flagAmount, "", "the amount of mint")

	_ = cmd.MarkFlagRequired(client.FlagFrom)
	for _, flag := range mintTokenFlags {
		_ = cmd.MarkFlagRequired(flag)
	}

	return cmd
}

var burnTokenFlags = []string{
	flagSymbol,
	flagAmount,
}

// BurnTokenCmd will create a burn token tx and sign.
func BurnTokenCmd(cdc *codec.Codec) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "burn-token",
		Short: "Create and sign a burn token tx",
		Long: strings.TrimSpace(
			`Create and sign a burn token tx, broadcast to nodes.

Example:
$ cetcli tx asset burn-token --symbol="abc" \
	--amount=10000000000000000 \
	--from mykey
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			cliCtx := context.NewCLIContext().WithCodec(cdc)
			owner := cliCtx.GetFromAddress()
			msg, err := parseBurnTokenFlags(owner)
			if err != nil {
				return err
			}

			if err = msg.ValidateBasic(); err != nil {
				return err
			}

			if _, err = auth.NewAccountRetriever(cliCtx).GetAccount(owner); err != nil {
				return err
			}

			// build and sign the transaction, then broadcast to Tendermint
			txBldr := auth.NewTxBuilderFromCLI().WithTxEncoder(utils.GetTxEncoder(cdc))
			return utils.GenerateOrBroadcastMsgs(cliCtx, txBldr, []sdk.Msg{msg})
		},
	}

	cmd.Flags().String(flagSymbol, "", "which token will be burned")
	cmd.Flags().String(flagAmount, "", "the amount of burn")

	_ = cmd.MarkFlagRequired(client.FlagFrom)
	for _, flag := range burnTokenFlags {
		_ = cmd.MarkFlagRequired(flag)
	}

	return cmd
}

var symbolFlags = []string{
	flagSymbol,
}

// ForbidTokenCmd will create a Forbid token tx and sign.
func ForbidTokenCmd(cdc *codec.Codec) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "forbid-token",
		Short: "Create and sign a forbid token tx",
		Long: strings.TrimSpace(
			`Create and sign a forbid token tx, broadcast to nodes.

Example:
$ cetcli tx asset forbid-token --symbol="abc" \
	--from mykey
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			cliCtx := context.NewCLIContext().WithCodec(cdc)
			owner := cliCtx.GetFromAddress()
			msg, err := parseForbidTokenFlags(owner)
			if err != nil {
				return err
			}

			if err = msg.ValidateBasic(); err != nil {
				return err
			}

			if _, err = auth.NewAccountRetriever(cliCtx).GetAccount(owner); err != nil {
				return err
			}

			// build and sign the transaction, then broadcast to Tendermint
			txBldr := auth.NewTxBuilderFromCLI().WithTxEncoder(utils.GetTxEncoder(cdc))
			return utils.GenerateOrBroadcastMsgs(cliCtx, txBldr, []sdk.Msg{msg})
		},
	}

	cmd.Flags().String(flagSymbol, "", "which token will be forbidden")

	_ = cmd.MarkFlagRequired(client.FlagFrom)
	for _, flag := range symbolFlags {
		_ = cmd.MarkFlagRequired(flag)
	}

	return cmd
}

// UnForbidTokenCmd will create a UnForbid token tx and sign.
func UnForbidTokenCmd(cdc *codec.Codec) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unforbid-token",
		Short: "Create and sign a unforbid token tx",
		Long: strings.TrimSpace(
			`Create and sign a unforbid token tx, broadcast to nodes.

Example:
$ cetcli tx asset unforbid-token --symbol="abc" \
	--from mykey
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			cliCtx := context.NewCLIContext().WithCodec(cdc)
			owner := cliCtx.GetFromAddress()
			msg, err := parseUnForbidTokenFlags(owner)
			if err != nil {
				return err
			}

			if err = msg.ValidateBasic(); err != nil {
				return err
			}

			if _, err = auth.NewAccountRetriever(cliCtx).GetAccount(owner); err != nil {
				return err
			}

			// build and sign the transaction, then broadcast to Tendermint
			txBldr := auth.NewTxBuilderFromCLI().WithTxEncoder(utils.GetTxEncoder(cdc))
			return utils.GenerateOrBroadcastMsgs(cliCtx, txBldr, []sdk.Msg{msg})
		},
	}

	cmd.Flags().String(flagSymbol, "", "which token will be un forbidden")

	_ = cmd.MarkFlagRequired(client.FlagFrom)
	for _, flag := range symbolFlags {
		_ = cmd.MarkFlagRequired(flag)
	}

	return cmd
}

var whitelistFlags = []string{
	flagSymbol,
	flagWhitelist,
}

// AddTokenWhitelistCmd will create a add token whitelist tx and sign.
func AddTokenWhitelistCmd(cdc *codec.Codec) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add-whitelist",
		Short: "Create and sign a add-whitelist tx",
		Long: strings.TrimSpace(
			`Create and sign a add-whitelist tx, broadcast to nodes.
				Multiple addresses separated by commas.

Example:
$ cetcli tx asset add-whitelist --symbol="abc" \
	--whitelist=key,key,key \
	--from mykey
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			cliCtx := context.NewCLIContext().WithCodec(cdc)
			owner := cliCtx.GetFromAddress()
			msg, err := parseAddWhitelistFlags(owner)
			if err != nil {
				return err
			}

			if err = msg.ValidateBasic(); err != nil {
				return err
			}

			if _, err = auth.NewAccountRetriever(cliCtx).GetAccount(owner); err != nil {
				return err
			}

			// build and sign the transaction, then broadcast to Tendermint
			txBldr := auth.NewTxBuilderFromCLI().WithTxEncoder(utils.GetTxEncoder(cdc))
			return utils.GenerateOrBroadcastMsgs(cliCtx, txBldr, []sdk.Msg{msg})
		},
	}

	cmd.Flags().String(flagSymbol, "", "which token whitelist be added")
	cmd.Flags().String(flagWhitelist, "", "add token whitelist addresses")

	_ = cmd.MarkFlagRequired(client.FlagFrom)
	for _, flag := range whitelistFlags {
		_ = cmd.MarkFlagRequired(flag)
	}

	return cmd
}

// RemoveTokenWhitelistCmd will create a remove token whitelist tx and sign.
func RemoveTokenWhitelistCmd(cdc *codec.Codec) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove-whitelist",
		Short: "Create and sign a remove-whitelist tx",
		Long: strings.TrimSpace(
			`Create and sign a remove-whitelist tx, broadcast to nodes.
				Multiple addresses separated by commas.

Example:
$ cetcli tx asset remove-whitelist --symbol="abc" \
	--whitelist=key,key,key \
	--from mykey
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			cliCtx := context.NewCLIContext().WithCodec(cdc)
			owner := cliCtx.GetFromAddress()
			msg, err := parseRemoveWhitelistFlags(owner)
			if err != nil {
				return err
			}

			if err = msg.ValidateBasic(); err != nil {
				return err
			}

			if _, err = auth.NewAccountRetriever(cliCtx).GetAccount(owner); err != nil {
				return err
			}

			// build and sign the transaction, then broadcast to Tendermint
			txBldr := auth.NewTxBuilderFromCLI().WithTxEncoder(utils.GetTxEncoder(cdc))
			return utils.GenerateOrBroadcastMsgs(cliCtx, txBldr, []sdk.Msg{msg})
		},
	}

	cmd.Flags().String(flagSymbol, "", "which token whitelist be remove")
	cmd.Flags().String(flagWhitelist, "", "remove token whitelist addresses")

	_ = cmd.MarkFlagRequired(client.FlagFrom)
	for _, flag := range whitelistFlags {
		_ = cmd.MarkFlagRequired(flag)
	}

	return cmd
}

var addressesFlags = []string{
	flagSymbol,
	flagAddresses,
}

// ForbidAddrCmd will create forbid address tx and sign.
func ForbidAddrCmd(cdc *codec.Codec) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "forbid-addr",
		Short: "Create and sign a forbid-addr tx",
		Long: strings.TrimSpace(
			`Create and sign a forbid-addr tx, broadcast to nodes.
				Multiple addresses separated by commas.

Example:
$ cetcli tx asset forbid-addr --symbol="abc" \
	--addresses=key,key,key \
	--from mykey
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			cliCtx := context.NewCLIContext().WithCodec(cdc)
			owner := cliCtx.GetFromAddress()
			msg, err := parseForbidAddrFlags(owner)
			if err != nil {
				return err
			}

			if err = msg.ValidateBasic(); err != nil {
				return err
			}

			if _, err = auth.NewAccountRetriever(cliCtx).GetAccount(owner); err != nil {
				return err
			}

			// build and sign the transaction, then broadcast to Tendermint
			txBldr := auth.NewTxBuilderFromCLI().WithTxEncoder(utils.GetTxEncoder(cdc))
			return utils.GenerateOrBroadcastMsgs(cliCtx, txBldr, []sdk.Msg{msg})
		},
	}

	cmd.Flags().String(flagSymbol, "", "which token address be forbidden")
	cmd.Flags().String(flagAddresses, "", "forbid addresses")

	_ = cmd.MarkFlagRequired(client.FlagFrom)
	for _, flag := range addressesFlags {
		_ = cmd.MarkFlagRequired(flag)
	}

	return cmd
}

// UnForbidAddrCmd will create unforbid address tx and sign.
func UnForbidAddrCmd(cdc *codec.Codec) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unforbid-addr",
		Short: "Create and sign a unforbid-addr tx",
		Long: strings.TrimSpace(
			`Create and sign a unforbid-addr tx, broadcast to nodes.
				Multiple addresses separated by commas.

Example:
$ cetcli tx asset unforbid-addr --symbol="abc" \
	--addresses=key,key,key \
	--from mykey
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			cliCtx := context.NewCLIContext().WithCodec(cdc)
			owner := cliCtx.GetFromAddress()
			msg, err := parseUnForbidAddrFlags(owner)
			if err != nil {
				return err
			}

			if err = msg.ValidateBasic(); err != nil {
				return err
			}

			if _, err = auth.NewAccountRetriever(cliCtx).GetAccount(owner); err != nil {
				return err
			}

			// build and sign the transaction, then broadcast to Tendermint
			txBldr := auth.NewTxBuilderFromCLI().WithTxEncoder(utils.GetTxEncoder(cdc))
			return utils.GenerateOrBroadcastMsgs(cliCtx, txBldr, []sdk.Msg{msg})
		},
	}

	cmd.Flags().String(flagSymbol, "", "which token address be un-forbidden")
	cmd.Flags().String(flagAddresses, "", "unforbid addresses")

	_ = cmd.MarkFlagRequired(client.FlagFrom)
	for _, flag := range addressesFlags {
		_ = cmd.MarkFlagRequired(flag)
	}

	return cmd
}

var modifyTokenURLFlags = []string{
	flagSymbol,
	flagTokenURL,
}

// ModifyTokenURLCmd will create a modify token url tx and sign.
func ModifyTokenURLCmd(cdc *codec.Codec) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "modify-token-url",
		Short: "Modify token url",
		Long: strings.TrimSpace(
			`Create and sign a modify token url msg, broadcast to nodes.

Example:
$ cetcli tx asset modify-token-url --symbol="abc" \
	--url="www.abc.com" \
	--from mykey
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			cliCtx := context.NewCLIContext().WithCodec(cdc)
			owner := cliCtx.GetFromAddress()
			msg, err := parseModifyTokenURLFlags(owner)
			if err != nil {
				return err
			}

			if err = msg.ValidateBasic(); err != nil {
				return err
			}

			if _, err = auth.NewAccountRetriever(cliCtx).GetAccount(owner); err != nil {
				return err
			}

			// build and sign the transaction, then broadcast to Tendermint
			txBldr := auth.NewTxBuilderFromCLI().WithTxEncoder(utils.GetTxEncoder(cdc))
			return utils.GenerateOrBroadcastMsgs(cliCtx, txBldr, []sdk.Msg{msg})
		},
	}

	cmd.Flags().String(flagSymbol, "", "which token will be modify")
	cmd.Flags().String(flagTokenURL, "", "the url of token")

	_ = cmd.MarkFlagRequired(client.FlagFrom)
	for _, flag := range modifyTokenURLFlags {
		_ = cmd.MarkFlagRequired(flag)
	}

	return cmd
}

var modifyTokenDescriptionFlags = []string{
	flagSymbol,
	flagTokenDescription,
}

// ModifyTokenDescriptionCmd will create a modify token description tx and sign.
func ModifyTokenDescriptionCmd(cdc *codec.Codec) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "modify-token-description",
		Short: "Modify token description",
		Long: strings.TrimSpace(
			`Create and sign a modify token description msg, broadcast to nodes.

Example:
$ cetcli tx asset modify-token-description --symbol="abc" \
	--description="abc example description" \
	--from mykey
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			cliCtx := context.NewCLIContext().WithCodec(cdc)
			owner := cliCtx.GetFromAddress()
			msg, err := parseModifyTokenDescriptionFlags(owner)
			if err != nil {
				return err
			}

			if err = msg.ValidateBasic(); err != nil {
				return err
			}

			if _, err = auth.NewAccountRetriever(cliCtx).GetAccount(owner); err != nil {
				return err
			}

			// build and sign the transaction, then broadcast to Tendermint
			txBldr := auth.NewTxBuilderFromCLI().WithTxEncoder(utils.GetTxEncoder(cdc))
			return utils.GenerateOrBroadcastMsgs(cliCtx, txBldr, []sdk.Msg{msg})
		},
	}

	cmd.Flags().String(flagSymbol, "", "which token will be modify")
	cmd.Flags().String(flagTokenDescription, "", "the description of token")

	_ = cmd.MarkFlagRequired(client.FlagFrom)
	for _, flag := range modifyTokenDescriptionFlags {
		_ = cmd.MarkFlagRequired(flag)
	}

	return cmd
}

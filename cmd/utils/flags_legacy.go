// Copyright 2020 The go-ethereum Authors
// This file is part of go-ethereum.
//
// go-ethereum is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// go-ethereum is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with go-ethereum. If not, see <http://www.gnu.org/licenses/>.

package utils

import (
	"fmt"

	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/internal/flags"
	"github.com/urfave/cli/v2"
)

var ShowDeprecated = &cli.Command{
	Action:      showDeprecated,
	Name:        "show-deprecated-flags",
	Usage:       "Show flags that have been deprecated",
	ArgsUsage:   " ",
	Category:    "MISCELLANEOUS COMMANDS",
	Description: "Show flags that have been deprecated and will soon be removed",
}

var DeprecatedFlags = []cli.Flag{
	LegacyMinerGasTargetFlag,
	NoUSBFlag,
	CacheTrieJournalFlag,
	CacheTrieRejournalFlag,
	TxLookupLimitFlag,
}

var (
	// (Deprecated May 2020, shown in aliased flags section)
	NoUSBFlag = &cli.BoolFlag{
		Name:  "nousb",
		Usage: "Disables monitoring for and managing USB hardware wallets (deprecated)",
	}
	// (Deprecated July 2021, shown in aliased flags section)
	LegacyMinerGasTargetFlag = &cli.Uint64Flag{
		Name:  "miner.gastarget",
		Usage: "Target gas floor for mined blocks (deprecated)",
		Value: ethconfig.Defaults.Miner.GasFloor,
	}
	// (Deprecated Oct 2024, shown in aliased flags section)
	CacheTrieJournalFlag = &cli.StringFlag{
		Name:     "cache.trie.journal",
		Usage:    "Disk journal directory for trie cache to survive node restarts",
		Category: flags.PerfCategory,
	}
	CacheTrieRejournalFlag = &cli.DurationFlag{
		Name:     "cache.trie.rejournal",
		Usage:    "Time interval to regenerate the trie cache journal",
		Category: flags.PerfCategory,
	}
	// Deprecated Nov 2024
	TxLookupLimitFlag = &cli.Uint64Flag{
		Name:     "txlookuplimit",
		Usage:    "Number of recent blocks to maintain transactions index for (default = about one year, 0 = entire chain) (deprecated, use history.transactions instead)",
		Value:    ethconfig.Defaults.TransactionHistory,
		Category: flags.DeprecatedCategory,
	}
)

// showDeprecated displays deprecated flags that will be soon removed from the codebase.
func showDeprecated(*cli.Context) error {
	fmt.Println("--------------------------------------------------------------------")
	fmt.Println("The following flags are deprecated and will be removed in the future!")
	fmt.Println("--------------------------------------------------------------------")
	fmt.Println()
	for _, flag := range DeprecatedFlags {
		fmt.Println(flag.String())
	}
	fmt.Println()
	return nil
}

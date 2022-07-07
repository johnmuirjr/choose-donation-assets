package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/johnmuirjr/go-knapsack"
	"github.com/shopspring/decimal"
	"os"
)

var (
	donation       = flag.String("donation", "1000.00", "donation amount")
	maximizeLosses = flag.Bool("maximize-losses", false, "maximize capital losses instead of capital gains")
	quoteDecimals  = flag.Bool("quote-decimals", false, "print decimal values as JSON strings")
)

type LotJSON struct {
	AssetName string          `json:"assetName"`
	Date      string          `json:"date"`
	Shares    uint64          `json:"shares"`
	ShareCost decimal.Decimal `json:"shareCost"`
}

type Input struct {
	AssetSharePrices map[string]decimal.Decimal `json:"assetSharePrices"`
	Lots             []LotJSON                  `json:"lots"`
}

func (i *Input) UnitCapitalGains(lot *LotJSON) decimal.Decimal {
	return i.AssetSharePrices[lot.AssetName].Sub(lot.ShareCost)
}

type Lot struct {
	json   *LotJSON
	shares uint64
	cost   uint64
}

type NormalizedLots struct {
	lots     []Lot
	donation uint64

	// minimum exponent from AssetSharePrices
	sharePriceExponent int32

	// AssetSharePrices converted to integers
	// after shifting by -sharePriceExponent
	// (to make the knapsack algorithm work)
	sharePrices map[string]uint64
}

func NewNormalizedLots(input *Input, donation string) (nl NormalizedLots, err error) {
	donationDecimal := decimal.RequireFromString(donation)
	nl.sharePriceExponent = donationDecimal.Exponent()
	for _, lot := range input.Lots {
		if lot.ShareCost.Exponent() < nl.sharePriceExponent {
			nl.sharePriceExponent = lot.ShareCost.Exponent()
		}
		if _, ok := input.AssetSharePrices[lot.AssetName]; !ok {
			err = fmt.Errorf(`lot has an assetName that does not appear in assetSharePrices: %s`, lot.AssetName)
			return
		}
	}
	for _, value := range input.AssetSharePrices {
		if value.Exponent() < nl.sharePriceExponent {
			nl.sharePriceExponent = value.Exponent()
		}
	}

	nl.donation = uint64(donationDecimal.Shift(-nl.sharePriceExponent).IntPart())
	nl.lots = make([]Lot, len(input.Lots))
	for m := range input.Lots {
		nl.lots[m] = Lot{
			json:   &input.Lots[m],
			shares: input.Lots[m].Shares,
			cost:   uint64(input.Lots[m].ShareCost.Shift(-nl.sharePriceExponent).IntPart())}
	}
	nl.sharePrices = make(map[string]uint64, len(input.AssetSharePrices))
	for name, value := range input.AssetSharePrices {
		nl.sharePrices[name] = uint64(value.Shift(-nl.sharePriceExponent).IntPart())
	}
	return
}

func (na *NormalizedLots) UnitCapitalGains(lot *Lot) int64 {
	return int64(na.sharePrices[lot.json.AssetName]) - int64(lot.cost)
}

func (nl *NormalizedLots) FilterLotsInPlace() {
	length := len(nl.lots)
	filter := func(lot *Lot) bool {
		if *maximizeLosses {
			return nl.UnitCapitalGains(lot) < 0
		}
		return nl.UnitCapitalGains(lot) > 0
	}
	for m := 0; m < length; {
		if filter(&nl.lots[m]) && nl.lots[m].shares > 0 && nl.sharePrices[nl.lots[m].json.AssetName] <= nl.donation {
			m++
		} else {
			length--
			nl.lots[m] = nl.lots[length]
		}
	}
	nl.lots = nl.lots[:length]
}

func (nl *NormalizedLots) GetTotalPrice() (totalPrice uint64) {
	for _, lot := range nl.lots {
		totalPrice += nl.sharePrices[lot.json.AssetName] * lot.shares
	}
	return
}

func ExpandLots(unexpanded []Lot) (expanded []Lot) {
	numShares := uint64(0)
	for _, lot := range unexpanded {
		numShares += lot.shares
	}
	expanded = make([]Lot, numShares)[:0]
	for _, lot := range unexpanded {
		for n := uint64(0); n < lot.shares; n++ {
			expanded = append(expanded, lot)
		}
	}
	return
}

func DeduplicateLots(lots []Lot) (deduplicated []Lot) {
	deduplicated = make([]Lot, len(lots))[:0]
	var prev *LotJSON
	for m, lot := range lots {
		if prev != nil && lot.json == prev {
			deduplicated[len(deduplicated)-1].shares++
			continue
		}
		prev = lots[m].json
		d := lots[m]
		d.shares = 1
		deduplicated = append(deduplicated, d)
	}
	return
}

func printUseMessage() {
	fmt.Fprintf(os.Stderr,
		`choose-donation-assets reads a set of asset prices and lots
from standard input and calculates which lots you should donate
to maximize capital gains tax savings (or, optionally,
which you should sell before donating to maximize capital losses).

The United States of America's Internal Revenue Service (IRS)
allows most taxpayers to deduct the full value of donated capital gain property
(shares of stock, bonds, ETFs, mutual funds, cryptocurrencies, and so on)
from their gross income, thus reducing their tax liability.
Two special rules apply:

1. If the donated assets were owned for more than a year
   and they have capital gains, the donors pay no taxes
   on the capital gains.
2. If donors sell assets that have capital losses and donate the cash proceeds,
   the donors can deduct the cash donations from their gross income
   and deduct the losses from their capital gains (if any).
   If capital losses exceed capital gains during a particular tax year,
   the donors can usually deduct up to $3,000 of losses
   from their gross income.

This tool helps calculate which assets you should donate to charity
given one of these two goals and a donation amount.
The goal is to encourage more charitable giving
and save you taxes in the long run.

Standard input MUST be a JSON object with the following structure:

- assetSharePrices :: object -- a set of current share (per-unit) prices
  for assets, where each key is the case-sensitive name of an asset
  and the value is the current share (per-unit) price of that asset,
  which can be a number or a numeric string
- lots :: array -- a list of asset lots, each of which is an object
  with the following fields:
    - assetName :: string -- the asset's case-sensitive name,
      which must match a key in assetSharePrices above
    - date :: string -- the date the asset was acquired
      (used for identifying this lot, so it can be any value
      that helps you easily identify it)
    - shares :: int -- the positive number of shares of this asset
      in this lot
    - shareCost :: number|numericString -- the share (per-unit) cost
      of the asset in this lot (the price of the asset
      when you purchased it in this lot), which can be a number
      or a numeric string

The program prints the results to standard output,
which is a JSON object with the following structure:

- donation :: object -- the lots you should donate,
  which have the same structure as the lots objects
  from standard input (but note that the number of shares
  you should donate in each lot may differ from those you inputted)
- assetSharePrices :: object -- the same assetSharePrices from standard input
- totalValue :: number|numericString -- the total value (total price)
  of the assets in the donation
- totalCapitalGains :: number|numericString -- the total capital gains
  (or losses if negative) contained in the donation

The program will not exceed the specified donation amount;
therefore, if you are comfortable donating slightly more
than your target donation amount, try various larger ones
until you find a donation that satisfies you.

The core algorithm runs in O(s*d) time and takes O(s*d) space,
where s is the total number of asset shares and d is the donation amount.

Options:

`)
	flag.PrintDefaults()
}

func main() {
	flag.Usage = printUseMessage
	flag.Parse()
	if !*quoteDecimals {
		decimal.MarshalJSONWithoutQuotes = true
	}

	// Parse assets from standard input.
	var input Input
	if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
		fmt.Fprintf(os.Stderr, "error decoding input JSON: %v\n", err)
		os.Exit(2)
	}
	normalizedLots, err := NewNormalizedLots(&input, *donation)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(2)
	}
	normalizedLots.FilterLotsInPlace()

	// Calculate the optimal donation.
	var donationLots []Lot
	if normalizedLots.GetTotalPrice() <= normalizedLots.donation {
		donationLots = normalizedLots.lots
	} else {
		lots := ExpandLots(normalizedLots.lots)
		getValue := func(a *Lot) int64 {
			multiplier := int64(1)
			if *maximizeLosses {
				multiplier = int64(-1)
			}
			return multiplier * normalizedLots.UnitCapitalGains(a)
		}
		donationLots = knapsack.Get01Solution(normalizedLots.donation, lots, func(lot *Lot) uint64 { return normalizedLots.sharePrices[lot.json.AssetName] }, getValue)
		donationLots = DeduplicateLots(donationLots)
	}

	// Print the optimal donation.
	outputLots := make([]LotJSON, len(donationLots))
	for m, lot := range donationLots {
		outputLots[m] = *lot.json
		outputLots[m].Shares = lot.shares
	}
	type Output struct {
		Lots              []LotJSON                  `json:"donation"`
		AssetSharePrices  map[string]decimal.Decimal `json:"assetSharePrices"`
		TotalValue        decimal.Decimal            `json:"totalValue"`
		TotalCapitalGains decimal.Decimal            `json:"totalCapitalGains"`
	}
	output := Output{Lots: outputLots, AssetSharePrices: input.AssetSharePrices}
	for _, asset := range output.Lots {
		shares := decimal.NewFromInt(int64(asset.Shares))
		output.TotalValue = output.TotalValue.Add(input.AssetSharePrices[asset.AssetName].Mul(shares))
		cg := input.UnitCapitalGains(&asset).Mul(shares)
		output.TotalCapitalGains = output.TotalCapitalGains.Add(cg)
	}
	json.NewEncoder(os.Stdout).Encode(output)
}

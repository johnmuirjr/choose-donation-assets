# Charitable Donation Calculator for Capital Gains Assets

This [Golang] program reads a portfolio of capital gains assets
(stocks, bonds, ETFs, mutual funds, cryptocurrencies, and so on)
and current prices as JSON from standard input and,
given a desired charitable donation amount,
calculates the assets you should donate
to maximize capital gains tax savings.
(Optionally, you can make it calculate the donation
that will maximize capital losses.)

This program requires Go 1.18 or later.

## Building

Run the standard build command:

```sh
go build
```

which will generate the `choose-donation-assets` executable.

## Use

Run `choose-donation-assets --help` to see a long explanation
of the input and ouptut formats and a list of options.

### Examples

Suppose you have this `input.json` file:

```json
{
	"assetSharePrices": {
		"VTI": 100.22,
		"BND": 12.35
	},
	"lots": [
		{"assetName":"VTI", "date":"2019-01-02", "shares":13, "shareCost":50.55},
		{"assetName":"VTI", "date":"2019-02-02", "shares":11, "shareCost":55.55},
		{"assetName":"VTI", "date":"2019-03-02", "shares":9, "shareCost":120.22},
		{"assetName":"BND", "date":"2019-02-03", "shares":50, "shareCost":10.00}
	]
}
```

Here are some example program runs (using [jq(1)] for pretty printing).

For a donation amount of 100
(the currency is the same as the asset costs and prices):

```
% choose-donation-assets -donation 100 <input.json | jq -S .
{
  "assetSharePrices": {
    "BND": 12.35,
    "VTI": 100.22
  },
  "donation": [
    {
      "assetName": "BND",
      "date": "2019-02-03",
      "shareCost": 10,
      "shares": 8
    }
  ],
  "totalCapitalGains": 18.8,
  "totalValue": 98.8
}
```

For a donation of 200:

```
% choose-donation-assets -donation 200 <input.json | jq -S .
{
  "assetSharePrices": {
    "BND": 12.35,
    "VTI": 100.22
  },
  "donation": [
    {
      "assetName": "VTI",
      "date": "2019-01-02",
      "shareCost": 50.55,
      "shares": 1
    },
    {
      "assetName": "BND",
      "date": "2019-02-03",
      "shareCost": 10,
      "shares": 8
    }
  ],
  "totalCapitalGains": 68.47,
  "totalValue": 199.02
}
```

For a donation of 10, which the program cannot satisfy:

```
% choose-donation-assets -donation 10 <input.json | jq -S .
{
  "assetSharePrices": {
    "BND": 12.35,
    "VTI": 100.22
  },
  "donation": [],
  "totalCapitalGains": 0,
  "totalValue": 0
}
```

[Golang]: https://go.dev
[jq(1)]: https://stedolan.github.io/jq/

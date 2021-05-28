/*
Copyright © 2021 Bhargav SNV bhargavsnv100@gmail.com

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package allcoin

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Gituser143/cryptgo/pkg/api"
	changePercentDurationPackage "github.com/Gituser143/cryptgo/pkg/display/changePercentageDuration"
	"github.com/Gituser143/cryptgo/pkg/display/coin"
	c "github.com/Gituser143/cryptgo/pkg/display/currency"
	"github.com/Gituser143/cryptgo/pkg/display/portfolio"
	"github.com/Gituser143/cryptgo/pkg/utils"
	"github.com/Gituser143/cryptgo/pkg/widgets"
	ui "github.com/gizak/termui/v3"
	"golang.org/x/sync/errgroup"
)

const (
	UP_ARROW   = "▲"
	DOWN_ARROW = "▼"
)

// DisplayAllCoins displays the main page with top coin prices, favourites and
// general coin asset data
func DisplayAllCoins(ctx context.Context, dataChannel chan api.AssetData, sendData *bool) error {

	// Initialise UI
	if err := ui.Init(); err != nil {
		return fmt.Errorf("failed to initialise termui: %v", err)
	}
	defer ui.Close()

	// Variables for currency
	currency := "USD $"
	currencyVal := 1.0

	changePercentageDuration := "24h"
	selectChangePercentageDuration := false
	changePercentageDurationWidget := changePercentDurationPackage.NewChangePercentageDurationPage()

	selectCurrency := false
	currencyWidget := c.NewCurrencyPage()

	// variables for sorting CoinTable
	coinSortIdx := -1
	coinSortAsc := false
	coinHeader := []string{
		"Rank",
		"Symbol",
		fmt.Sprintf("Price (%s)", currency),
		fmt.Sprintf("Change %%(%s)", changePercentageDuration),
		"Supply / MaxSupply",
	}

	// variables for sorting FavouritesTable
	favSortIdx := -1
	favSortAsc := false
	favHeader := []string{
		"Symbol",
		fmt.Sprintf("Price (%s)", currency),
	}

	previousKey := ""

	// coinIDs tracks symbol: id
	coinIDs, _ := api.GetTopNCoinSymbolToIDMap(200)

	// Initalise page and set selected table
	myPage := NewAllCoinPage()
	selectedTable := myPage.CoinTable

	// Initialise favourites and portfolio
	portfolioMap := utils.GetPortfolio()
	favourites := utils.GetFavourites()
	defer utils.SaveMetadata(favourites, currency, portfolioMap)

	// Initialise Help Menu
	help := widgets.NewHelpMenu()
	help.SelectHelpMenu("ALL")
	helpSelected := false

	// Initiliase Portfolio Table
	portfolioTable := portfolio.NewPortfolioPage()
	portfolioSelected := false

	// Pause function to pause sending and receiving of data
	pause := func() {
		*sendData = !(*sendData)
	}

	// UpdateUI to refresh UI
	updateUI := func() {
		// Get Terminal Dimensions
		w, h := ui.TerminalDimensions()
		myPage.Grid.SetRect(0, 0, w, h)

		// Clear UI
		ui.Clear()

		// Render required widgets
		if helpSelected {
			help.Resize(w, h)
			ui.Render(help)
		} else if portfolioSelected {
			portfolioTable.Resize(w, h)
			ui.Render(portfolioTable)
		} else if selectCurrency {
			currencyWidget.Resize(w, h)
			ui.Render(currencyWidget)
		} else if selectChangePercentageDuration {
			changePercentageDurationWidget.Resize(w, h)
			ui.Render(changePercentageDurationWidget)
		} else {
			ui.Render(myPage.Grid)
		}
	}

	// Render Empty UI
	updateUI()

	// Create Channel to get keyboard events
	uiEvents := ui.PollEvents()

	// Create ticker to periodically refresh UI
	t := time.NewTicker(time.Duration(1) * time.Second)
	tick := t.C

	for {
		select {
		case <-ctx.Done(): // Context cancelled, exit
			return ctx.Err()

		case e := <-uiEvents: // keyboard events
			switch e.ID {
			case "q", "<C-c>": // q or Ctrl-C to quit
				return fmt.Errorf("UI Closed")

			case "<Resize>":
				updateUI()

			case "p":
				pause()

			case "?":
				helpSelected = !helpSelected
				updateUI()

			case "f":
				if !helpSelected && !selectCurrency && !portfolioSelected {
					selectedTable.ShowCursor = false
					selectedTable = myPage.FavouritesTable
				}

			case "F":
				if !helpSelected && !selectCurrency && !portfolioSelected {
					selectedTable.ShowCursor = false
					selectedTable = myPage.CoinTable
				}

			case "c":
				if !helpSelected && !selectChangePercentageDuration && !portfolioSelected {
					selectedTable.ShowCursor = false
					selectChangePercentageDuration = false
					selectCurrency = true
					selectedTable.ShowCursor = true
					currencyWidget.UpdateRows()
					updateUI()
				}

			case "C":
				if !helpSelected && !selectChangePercentageDuration && !portfolioSelected {
					selectedTable.ShowCursor = false
					selectChangePercentageDuration = false
					selectCurrency = true
					selectedTable.ShowCursor = true
					currencyWidget.UpdateAll()
					updateUI()
				}
			case "%":
				if !helpSelected && !selectCurrency && !portfolioSelected {
					selectedTable.ShowCursor = false
					selectChangePercentageDuration = true
					selectCurrency = false
					selectedTable.ShowCursor = true
					updateUI()
				}

			case "P":
				if !helpSelected && !selectCurrency && !selectChangePercentageDuration {
					portfolioTable.UpdateRows(portfolioMap, currency, currencyVal)
					portfolioSelected = !portfolioSelected
					updateUI()
				}
			}
			if helpSelected {
				switch e.ID {
				case "?":
					updateUI()
				case "<Escape>":
					helpSelected = false
					updateUI()
				case "j", "<Down>":
					help.List.ScrollDown()
					ui.Render(help)
				case "k", "<Up>":
					help.List.ScrollUp()
					ui.Render(help)
				}
			} else if portfolioSelected {
				switch e.ID {
				case "<Escape>":
					portfolioSelected = false
					updateUI()
				case "j", "<Down>":
					portfolioTable.ScrollDown()
					ui.Render(portfolioTable)
				case "k", "<Up>":
					portfolioTable.ScrollUp()
					ui.Render(portfolioTable)
				case "e": // Add/Edit coin in portfolio
					id := ""
					symbol := ""

					// Get ID and symbol
					if portfolioTable.SelectedRow < len(portfolioTable.Rows) {
						row := portfolioTable.Rows[portfolioTable.SelectedRow]
						symbol = row[1]
					}

					id = coinIDs[symbol]

					if id != "" {
						inputStr := widgets.DrawEdit(uiEvents, symbol)
						amt, err := strconv.ParseFloat(inputStr, 64)

						if err == nil {
							if amt > 0 {
								portfolioMap[id] = amt
							} else {
								delete(portfolioMap, id)
							}
						}
					}

					portfolioTable.UpdateRows(portfolioMap, currency, currencyVal)
				}
			} else if selectCurrency {
				switch e.ID {
				case "j", "<Down>":
					currencyWidget.ScrollDown()
				case "k", "<Up>":
					currencyWidget.ScrollUp()
				case "<C-d>":
					currencyWidget.ScrollHalfPageDown()
				case "<C-u>":
					currencyWidget.ScrollHalfPageUp()
				case "<C-f>":
					currencyWidget.ScrollPageDown()
				case "<C-b>":
					currencyWidget.ScrollPageUp()
				case "g":
					if previousKey == "g" {
						currencyWidget.ScrollTop()
					}
				case "<Home>":
					currencyWidget.ScrollTop()
				case "G", "<End>":
					currencyWidget.ScrollBottom()
				case "<Enter>":
					var err error

					// Update Currency
					if currencyWidget.SelectedRow < len(currencyWidget.Rows) {
						row := currencyWidget.Rows[currencyWidget.SelectedRow]

						// Get currency and rate
						currency = fmt.Sprintf("%s %s", row[0], row[1])
						currencyVal, err = strconv.ParseFloat(row[3], 64)
						if err != nil {
							currencyVal = 0
							currency = "USD $"
						}

						// Update currency fields
						coinHeader[2] = fmt.Sprintf("Price (%s)", currency)
						favHeader[1] = fmt.Sprintf("Price (%s)", currency)
					}

					selectCurrency = false

				case "<Escape>":
					selectCurrency = false
				}
				if selectCurrency {
					ui.Render(currencyWidget)
				}
			} else if selectChangePercentageDuration {
				switch e.ID {
				case "j", "<Down>":
					changePercentageDurationWidget.ScrollDown()
				case "k", "<Up>":
					changePercentageDurationWidget.ScrollUp()
				case "<C-d>":
					changePercentageDurationWidget.ScrollHalfPageDown()
				case "<C-u>":
					changePercentageDurationWidget.ScrollHalfPageUp()
				case "<C-f>":
					changePercentageDurationWidget.ScrollPageDown()
				case "<C-b>":
					changePercentageDurationWidget.ScrollPageUp()
				case "g":
					if previousKey == "g" {
						changePercentageDurationWidget.ScrollTop()
					}
				case "<Home>":
					changePercentageDurationWidget.ScrollTop()
				case "G", "<End>":
					changePercentageDurationWidget.ScrollBottom()
				case "<Enter>":

					if changePercentageDurationWidget.SelectedRow < len(changePercentageDurationWidget.Rows) {
						row := changePercentageDurationWidget.Rows[changePercentageDurationWidget.SelectedRow]

						changePercentageDuration = changePercentDurationPackage.DurationMap[row[0]]

						coinHeader[3] = fmt.Sprintf("Change %%(%s)", changePercentageDuration)
					}

					selectChangePercentageDuration = false

				case "<Escape>":
					selectChangePercentageDuration = false
				}
				if selectChangePercentageDuration {
					ui.Render(changePercentageDurationWidget)
				}
			} else if selectedTable != nil {
				selectedTable.ShowCursor = true

				switch e.ID {
				case "j", "<Down>":
					selectedTable.ScrollDown()
				case "k", "<Up>":
					selectedTable.ScrollUp()
				case "<C-d>":
					selectedTable.ScrollHalfPageDown()
				case "<C-u>":
					selectedTable.ScrollHalfPageUp()
				case "<C-f>":
					selectedTable.ScrollPageDown()
				case "<C-b>":
					selectedTable.ScrollPageUp()
				case "g":
					if previousKey == "g" {
						selectedTable.ScrollTop()
					}
				case "<Home>":
					selectedTable.ScrollTop()
				case "G", "<End>":
					selectedTable.ScrollBottom()

				case "e": // Add/Edit coin in portfolio
					id := ""
					symbol := ""

					// Get ID and symbol
					if selectedTable == myPage.CoinTable {
						if myPage.CoinTable.SelectedRow < len(myPage.CoinTable.Rows) {
							row := myPage.CoinTable.Rows[myPage.CoinTable.SelectedRow]
							symbol = row[1]
						}
					} else {
						if myPage.FavouritesTable.SelectedRow < len(myPage.FavouritesTable.Rows) {
							row := myPage.FavouritesTable.Rows[myPage.FavouritesTable.SelectedRow]
							symbol = row[0]
						}
					}
					id = coinIDs[symbol]

					if id != "" {

						inputStr := widgets.DrawEdit(uiEvents, symbol)

						amt, err := strconv.ParseFloat(inputStr, 64)
						if err == nil {
							if amt > 0 {
								portfolioMap[id] = amt
							} else {
								delete(portfolioMap, id)
							}
						}
					}

					ui.Clear()
					updateUI()

				case "s": // Add coin to favourites
					id := ""
					symbol := ""

					// Get ID and symbol
					if selectedTable == myPage.CoinTable {
						if myPage.CoinTable.SelectedRow < len(myPage.CoinTable.Rows) {
							row := myPage.CoinTable.Rows[myPage.CoinTable.SelectedRow]
							symbol = row[1]
						}
					} else {
						if myPage.FavouritesTable.SelectedRow < len(myPage.FavouritesTable.Rows) {
							row := myPage.FavouritesTable.Rows[myPage.FavouritesTable.SelectedRow]
							symbol = row[0]
						}
					}
					id = coinIDs[symbol]
					favourites[id] = true

				case "S": // Remove coin from favourites
					id := ""
					symbol := ""

					// Get ID and symbol
					if selectedTable == myPage.CoinTable {
						if myPage.CoinTable.SelectedRow < len(myPage.CoinTable.Rows) {
							row := myPage.CoinTable.Rows[myPage.CoinTable.SelectedRow]
							symbol = row[1]
						}
					} else {
						if myPage.FavouritesTable.SelectedRow < len(myPage.FavouritesTable.Rows) {
							row := myPage.FavouritesTable.Rows[myPage.FavouritesTable.SelectedRow]
							symbol = row[0]
						}
					}
					id = coinIDs[symbol]

					delete(favourites, id)

				case "<Enter>": // Serve per coin details
					// pause UI and data send
					pause()

					id := ""
					symbol := ""

					// Get ID and symbol
					if selectedTable == myPage.CoinTable {
						if myPage.CoinTable.SelectedRow < len(myPage.CoinTable.Rows) {
							row := myPage.CoinTable.Rows[myPage.CoinTable.SelectedRow]
							symbol = row[1]
						}
					} else {
						if myPage.FavouritesTable.SelectedRow < len(myPage.FavouritesTable.Rows) {
							row := myPage.FavouritesTable.Rows[myPage.FavouritesTable.SelectedRow]
							symbol = row[0]
						}
					}
					id = coinIDs[symbol]

					if id != "" {
						// Create new errorgroup for coin page
						eg, coinCtx := errgroup.WithContext(ctx)
						coinDataChannel := make(chan api.CoinData)
						coinPriceChannel := make(chan string)
						intervalChannel := make(chan string)

						// Clear UI
						ui.Clear()

						// Serve Coin Price History
						eg.Go(func() error {
							err := api.GetCoinHistory(
								coinCtx,
								id,
								intervalChannel,
								coinDataChannel,
							)
							return err
						})

						// Serve Coin Asset data
						eg.Go(func() error {
							err := api.GetCoinAsset(coinCtx, id, coinDataChannel)
							return err
						})

						// Serve favourie coin prices
						eg.Go(func() error {
							err := api.GetFavouritePrices(coinCtx,
								favourites,
								coinDataChannel,
							)
							return err
						})

						// Serve Live price of coin
						eg.Go(func() error {
							err := api.GetLivePrice(coinCtx, id, coinPriceChannel)
							return err
						})

						// Serve Visuals for coin
						eg.Go(func() error {
							err := coin.DisplayCoin(
								coinCtx,
								id,
								coinIDs,
								intervalChannel,
								coinDataChannel,
								coinPriceChannel,
								uiEvents,
							)
							return err
						})

						if err := eg.Wait(); err != nil {
							if err.Error() != "UI Closed" {
								// Unpause
								pause()
								return err
							}
						}

					}
					// unpause data send and receive
					pause()
					updateUI()
				}

				if selectedTable == myPage.CoinTable {
					switch e.ID {
					// Sort Ascending
					case "1", "2", "3", "4":
						idx, _ := strconv.Atoi(e.ID)
						coinSortIdx = idx - 1
						myPage.CoinTable.Header = append([]string{}, coinHeader...)
						myPage.CoinTable.Header[coinSortIdx] = coinHeader[coinSortIdx] + " " + UP_ARROW
						coinSortAsc = true
						utils.SortData(myPage.CoinTable.Rows, coinSortIdx, coinSortAsc, "COINS")

					// Sort Descending
					case "<F1>", "<F2>", "<F3>", "<F4>":
						myPage.CoinTable.Header = append([]string{}, coinHeader...)
						idx, _ := strconv.Atoi(e.ID[2:3])
						coinSortIdx = idx - 1
						myPage.CoinTable.Header[coinSortIdx] = coinHeader[coinSortIdx] + " " + DOWN_ARROW
						coinSortAsc = false
						utils.SortData(myPage.CoinTable.Rows, coinSortIdx, coinSortAsc, "COINS")

					}
				} else if selectedTable == myPage.FavouritesTable {
					switch e.ID {
					// Sort Ascending
					case "1", "2":
						idx, _ := strconv.Atoi(e.ID)
						favSortIdx = idx - 1
						myPage.FavouritesTable.Header = append([]string{}, favHeader...)
						myPage.FavouritesTable.Header[favSortIdx] = favHeader[favSortIdx] + " " + UP_ARROW
						favSortAsc = true
						utils.SortData(myPage.FavouritesTable.Rows, favSortIdx, favSortAsc, "FAVOURITES")

					// Sort Descending
					case "<F1>", "<F2>":
						myPage.FavouritesTable.Header = append([]string{}, favHeader...)
						idx, _ := strconv.Atoi(e.ID[2:3])
						favSortIdx = idx - 1
						myPage.FavouritesTable.Header[favSortIdx] = favHeader[favSortIdx] + " " + DOWN_ARROW
						favSortAsc = false
						utils.SortData(myPage.FavouritesTable.Rows, favSortIdx, favSortAsc, "FAVOURITES")
					}
				}

				ui.Render(myPage.Grid)
				if previousKey == "g" {
					previousKey = ""
				} else {
					previousKey = e.ID
				}
			}

		case data := <-dataChannel:
			if data.IsTopCoinData {
				// Update Top Coin data
				for i, v := range data.TopCoinData {
					// Set title to coin name
					myPage.TopCoinGraphs[i].Title = " " + data.TopCoins[i] + " "

					// Update value graphs
					myPage.TopCoinGraphs[i].Data["Value"] = v

					// Set value, max & min values
					myPage.TopCoinGraphs[i].Labels["Value"] = fmt.Sprintf("%.2f %s", v[len(v)-1]/currencyVal, currency)
					myPage.TopCoinGraphs[i].Labels["Max"] = fmt.Sprintf("%.2f %s", utils.MaxFloat64(v...)/currencyVal, currency)
					myPage.TopCoinGraphs[i].Labels["Min"] = fmt.Sprintf("%.2f %s", utils.MinFloat64(v...)/currencyVal, currency)
				}
			} else {
				rows := [][]string{}
				favouritesData := [][]string{}

				// Update currency headers
				myPage.CoinTable.Header[2] = fmt.Sprintf("Price (%s)", currency)
				myPage.CoinTable.Header[3] = fmt.Sprintf("Change %%(%s)", changePercentageDuration)
				myPage.FavouritesTable.Header[1] = fmt.Sprintf("Price (%s)", currency)

				// Iterate over coin assets
				for _, val := range data.AllCoinData {
					// Get coin price
					price := fmt.Sprintf("%.2f", val.CurrentPrice/currencyVal)

					// Get change %
					change := "NA"
					percentageChange := api.GetPercentageChangeForDuration(val, changePercentageDuration)
					if percentageChange < 0 {
						change = fmt.Sprintf("%s %.2f", DOWN_ARROW, -1*percentageChange)
					} else {
						change = fmt.Sprintf("%s %.2f", UP_ARROW, percentageChange)
					}

					units := ""
					var supplyVals []float64
					supplyData := ""

					if val.CirculatingSupply != 0.00 && val.TotalSupply != 0.00 {
						supplyVals, units = utils.RoundValues(val.CirculatingSupply, val.TotalSupply)
						supplyData = fmt.Sprintf("%.2f%s / %.2f%s", supplyVals[0], units, supplyVals[1], units)
					} else {
						if val.CirculatingSupply == 0.00 {
							supplyVals, units = utils.RoundValues(val.CirculatingSupply, val.TotalSupply)
							supplyData = fmt.Sprintf("NA / %.2f%s", supplyVals[1], units)
						} else {
							supplyVals, units = utils.RoundValues(val.CirculatingSupply, val.TotalSupply)
							supplyData = fmt.Sprintf("%.2f%s / NA", supplyVals[0], units)
						}
					}

					rank := fmt.Sprintf("%d", val.MarketCapRank)

					// Aggregate data
					rows = append(rows, []string{
						rank,
						strings.ToUpper(val.Symbol),
						price,
						change,
						supplyData,
					})

					// Aggregate favourite data
					if _, ok := favourites[val.ID]; ok {
						favouritesData = append(favouritesData, []string{
							strings.ToUpper(val.Symbol),
							price,
						})
					}
				}

				myPage.CoinTable.Rows = rows
				myPage.FavouritesTable.Rows = favouritesData

				// Sort CoinTable data
				if coinSortIdx != -1 {
					utils.SortData(myPage.CoinTable.Rows, coinSortIdx, coinSortAsc, "COINS")

					if coinSortAsc {
						myPage.CoinTable.Header[coinSortIdx] = coinHeader[coinSortIdx] + " " + UP_ARROW
					} else {
						myPage.CoinTable.Header[coinSortIdx] = coinHeader[coinSortIdx] + " " + DOWN_ARROW
					}
				}

				// Sort FavouritesTable Data
				if favSortIdx != -1 {
					utils.SortData(myPage.FavouritesTable.Rows, favSortIdx, favSortAsc, "FAVOURITES")

					if favSortAsc {
						myPage.FavouritesTable.Header[favSortIdx] = favHeader[favSortIdx] + " " + UP_ARROW
					} else {
						myPage.FavouritesTable.Header[favSortIdx] = favHeader[favSortIdx] + " " + DOWN_ARROW
					}
				}
			}

		case <-tick: // Refresh UI
			if *sendData {
				updateUI()
			}
		}

	}

}

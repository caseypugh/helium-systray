package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/getlantern/systray"
)

const appSettingsPath = "/Documents/helium-systray.json"

type appSettings struct {
	RefreshMinutes int    `json:"refresh_minutes"`
	AccountAddress string `json:"account_address"`
}

type hotspotMenuItem struct {
	MenuItem *systray.MenuItem
	Status   *systray.MenuItem
	Scale    *systray.MenuItem
	R24H     *systray.MenuItem
	R7D      *systray.MenuItem
	R30D     *systray.MenuItem
}

func main() {
	systray.Run(onReady, onExit)
}

func onReady() {
	// load config file
	appSettings := loadAppSettings(appSettingsPath)
	fmt.Printf("Config loaded: %+v", appSettings)

	// set loading status
	systray.SetTitle("Calculating HNT summary...")
	systray.SetTooltip("HNT summary for your Helium hotspots in past 24 hours")

	// setup initial config values
	cfg := newConfig(appSettings)

	// get initial list of hotspots
	hotspotsResp, err := getAccountHotspots(cfg.AccountAddress)
	if err != nil {
		systray.SetTitle("Error fetching hotspots")
		fmt.Println(err)
	}

	// populate hotspot data and menu items
	for _, hs := range hotspotsResp.Data {
		cfg.HsMap[hs.Name] = hs
		cfg.HsMenuItems = append(cfg.HsMenuItems, newHotspotMenuItem())
	}

	// set flag for skipping first refresh
	cfg.SkipHotspotRefresh = true

	// add quit button at the end because order matters
	systray.AddSeparator()
	pref := systray.AddMenuItem("Preferences...", "Adjust preferences")
	displayHNT := pref.AddSubMenuItem("display rewards in HNT", "display hotspot rewards in HNT")
	displayDollars := pref.AddSubMenuItem("display rewards in dollars", "display hotspot rewards in dollars")
	mQuit := systray.AddMenuItem("Quit", "Quits this app")

	// data refresh routine
	go func() {
		for {
			// clear previous sort/total data
			cfg.ClearPreviousData()

			// get new price
			priceResp, err := getPrice()
			if err != nil {
				systray.SetTitle("Error fetching HNT price")
				fmt.Println(err)
			}
			cfg.Price = priceResp.Data.Price

			// update hotspots data
			if !cfg.SkipHotspotRefresh {
				hotspotsResp, err := getAccountHotspots(appSettings.AccountAddress)
				if err != nil {
					systray.SetTitle("Error fetching hotspots")
					fmt.Println(err)
				}

				for _, hs := range hotspotsResp.Data {
					cfg.HsMap[hs.Name] = hs
				}

				// TODO: reconcile menu items here
			}

			// get rewards for each hotspot
			for name, hs := range cfg.HsMap {
				// track rewards
				rewardsResp, _ := getHotspotRewards(hs.Address)
				cfg.HsRewards[name] = rewardsResp.Data

				// track sorting order and today's reward
				reward := cfg.RewardOn(name, 0)
				cfg.HsSort = append(cfg.HsSort, sortOrder{Name: name, Reward: reward})
				cfg.Total += reward
			}

			cfg.SortHotspotsByReward()
			cfg.UpdateView()
			time.Sleep(time.Duration(cfg.RefreshMinutes) * time.Minute)
		}
	}()

	// click handling routine
	go func() {
		for {
			select {
			case <-displayHNT.ClickedCh:
				cfg.ConvertToDollars = false
				cfg.UpdateView()
			case <-displayDollars.ClickedCh:
				cfg.ConvertToDollars = true
				cfg.UpdateView()
			case <-mQuit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()
}

func onExit() {
	// no-op
}

func loadAppSettings(path string) appSettings {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalln(err)
	}

	file, err := os.Open(homeDir + path)
	if err != nil {
		log.Fatalln(err)
	}
	defer file.Close()

	rawSettings, err := ioutil.ReadAll(file)
	if err != nil {
		log.Fatalln(err)
	}

	var as appSettings
	err = json.Unmarshal(rawSettings, &as)
	if err != nil {
		log.Fatalln(err)
	}
	return as
}

func newHotspotMenuItem() hotspotMenuItem {
	item := systray.AddMenuItem("", "")
	return hotspotMenuItem{
		MenuItem: item,
		Status:   item.AddSubMenuItem("", ""),
		Scale:    item.AddSubMenuItem("", ""),
		R24H:     item.AddSubMenuItem("", ""),
		R7D:      item.AddSubMenuItem("", ""),
		R30D:     item.AddSubMenuItem("", ""),
	}
}

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sort"
	"time"

	"github.com/getlantern/systray"
)

const configFileName = "/Documents/helium-systray.json"

type config struct {
	RefreshMinutes int    `json:"refresh_minutes"`
	AccountAddress string `json:"account_address"`
}

type hotspotMenuItem struct {
	MenuItem *systray.MenuItem
}

type sortOrder struct {
	Name   string
	Reward float64
}

func main() {
	systray.Run(onReady, onExit)
}

func onReady() {
	// load config file
	config := loadConfig(configFileName)
	fmt.Printf("Config loaded: %+v", config)

	// set loading status
	systray.SetTitle("Calculating HNT summary...")
	systray.SetTooltip("HNT summary for your Helium hotspots in past 24 hours")

	// setup initial values
	var (
		total              float64 // total rewards to be displayed in the menu
		skipHotspotRefresh bool
		hsMap              = make(map[string]hotspot)  // map of hotspots
		hsRewards          = make(map[string][]reward) // 60 day reward data of hotspots
		hsMenuItems        = []hotspotMenuItem{}       // slice of view rows
		hsSort             = []sortOrder{}             // sorting order
	)

	// Get initial list of hotspots
	skipHotspotRefresh = true
	hotspotsResp, err := getAccountHotspots(config.AccountAddress)
	if err != nil {
		systray.SetTitle("Error fetching hotspots")
		fmt.Println(err)
	}

	for _, hs := range hotspotsResp.Data {
		hsMap[hs.Name] = hs
		hsMenuItems = append(hsMenuItems, newHotspotMenuItem())
	}

	// add quit button at the end because order matters
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "Quits this app")

	// data refresh routine
	go func() {
		for {
			// update hotspots data
			if !skipHotspotRefresh {
				hotspotsResp, err := getAccountHotspots(config.AccountAddress)
				if err != nil {
					systray.SetTitle("Error fetching hotspots")
					fmt.Println(err)
				}

				for _, hs := range hotspotsResp.Data {
					hsMap[hs.Name] = hs
				}

				// TODO: reconcile menu items here
			}

			// get rewards for each hotspot
			for name, hs := range hsMap {
				rewardsResp, _ := getHotspotRewards(hs.Address)

				hsRewards[name] = rewardsResp.Data                             // track rewards
				reward := hsRewards[name][0].Total                             // grab hs reward total for 24 hours
				hsSort = append(hsSort, sortOrder{Name: name, Reward: reward}) // track sorting order
				total += reward                                                // track total
			}

			// sort the hotspots by rewards
			sort.SliceStable(hsSort, func(a, b int) bool {
				return hsSort[a].Reward > hsSort[b].Reward
			})

			// update menu items for each ordered hotspots
			for i, order := range hsSort {
				hsStatus := hsMap[order.Name].Status.Online
				hsMenuItems[i].MenuItem.SetTitle(
					fmt.Sprintf(
						"%s %s - %s",
						statusToEmoji(hsStatus),
						rewardSummary(hsRewards[order.Name]),
						order.Name,
					),
				)
			}

			// update title with total
			systray.SetTitle(fmt.Sprintf("HNT earned: %s", floatToString(total)))

			// reset values
			total = 0.0
			skipHotspotRefresh = false

			// sleep until next refresh
			time.Sleep(time.Duration(config.RefreshMinutes) * time.Minute)
		}
	}()

	// click handling routine
	go func() {
		for {
			select {
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

func newHotspotMenuItem() hotspotMenuItem {
	return hotspotMenuItem{
		MenuItem: systray.AddMenuItem("", ""),
	}
}

func loadConfig(fileName string) config {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalln(err)
	}

	configFile, err := os.Open(homeDir + fileName)
	if err != nil {
		log.Fatalln(err)
	}
	defer configFile.Close()

	rawConfig, err := ioutil.ReadAll(configFile)
	if err != nil {
		log.Fatalln(err)
	}

	var cfg config
	err = json.Unmarshal(rawConfig, &cfg)
	if err != nil {
		log.Fatalln(err)
	}

	return cfg
}

func floatToString(val float64) string {
	return fmt.Sprintf("%.2f", val)
}

func rewardSummary(summary []reward) string {
	diff := summary[0].Total - summary[1].Total
	return fmt.Sprintf("%s %s", diffEmoji(diff), floatToString(summary[0].Total))
}

func diffEmoji(diff float64) string {
	if diff > 0 {
		return "⬆"
	}
	return "⬇"
}

func statusToEmoji(status string) string {
	if status == "online" {
		return "🟢"
	}
	return "🔴"
}
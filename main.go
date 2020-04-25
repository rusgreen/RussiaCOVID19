package main

import (
	"encoding/json"
	"fmt"
	"github.com/rusgreen/whdisco/wh"
	"io/ioutil"
	"net/http"
	"sort"
	"strings"
	"time"
)

const discordChannelId = "ID"                                                                                      // ID канала Discord Webhook для отправки информационных уведомлений
const discordToken = "токен"                                                                                       //	токен Discord Webhook для отправки информационных уведомлений
const discordWhUrl = "https://discordapp.com/api/webhooks/" + discordChannelId + "/" + discordToken                // url Discord Webhook для отправки информационных уведомлений
const errorDiscordChannelId = "ID"                                                                                 // ID канала Discord Webhook для отправки уведомлений об ошибке
const errorDiscordToken = "токен"                                                                                  //	токен Discord Webhook для отправки уведомлений об ошибке
const errorDiscordWhUrl = "https://discordapp.com/api/webhooks/" + errorDiscordChannelId + "/" + errorDiscordToken // url Discord Webhook для отправки уведомлений об ошибке
const url = "https://yandex.ru/maps/covid19"                                                                       //	url источника
const pauseBetweenMesages = 1                                                                                      // пауза между сообщениями

type Items struct {
	Name      string      `json:"name"`
	Cases     int         `json:"cases"`
	Deaths    int         `json:"deaths"`
	Cured     int         `json:"cured"`
	Ru        bool        `json:"ru"`
	Histogram interface{} `json:"histogram"`
	Number    int
}

var err error
var response *http.Response
var inform []uint8

func main() {
	var previous []*Items
	for {
		for {
			response, err = http.Get(url)
			if err != nil {
				sendErrorWebhooks(err)
			} else {
				inform, err = ioutil.ReadAll(response.Body)
				if err != nil {
					sendErrorWebhooks(err)
				} else if response != nil && inform != nil {
					break
				}
			}
			time.Sleep(30 * time.Second)
		}
		worldStartIdx := strings.Index(string(inform), "[{\"coordinates")
		worldEndIdx := strings.LastIndex(string(inform), ",\"histogram\":[{\"val")
		if worldStartIdx <= 0 || worldEndIdx <= 0 {
			sendRangeBoundsErrorWebhooks()
		}
		world := inform[worldStartIdx:worldEndIdx]
		// десериализация информации по всем старанам мира
		var worldData []*Items
		err = json.Unmarshal(world, &worldData)
		if err != nil {
			sendErrorWebhooks(err)
		} else {
			// делаем выборку информации по регионам России
			var regions []*Items
			for _, ru := range worldData {
				if ru.Ru == false {
					continue
				}
				regions = append(regions, ru)
			}
			current := regions
			// создаём срез всех случаев заболевания
			sumAllCases := make([]int, 0)
			for _, currentCases := range current {
				sumAllCases = append(sumAllCases, currentCases.Cases)
			}
			// создаём срез всех случаев гибели
			sumAllDeaths := make([]int, 0)
			for _, currentDeaths := range current {
				sumAllDeaths = append(sumAllDeaths, currentDeaths.Deaths)
			}
			// создаём срез всех случаев выздоровления
			sumAllCured := make([]int, 0)
			for _, currentCured := range current {
				sumAllCured = append(sumAllCured, currentCured.Cured)
			}
			// сортируем по количеству случаев заболевания
			sort.SliceStable(current, func(i, j int) bool {
				return current[i].Cases > current[j].Cases
			})
			// добавляем нумерацию
			for i, v := range current {
				v.Number = i + 1
			}
			// сортируем по алфавиту
			sort.SliceStable(current, func(i, j int) bool {
				return current[i].Name < current[j].Name
			})
			// определяем разницу между текущим и предыдущим запросом
			diffResult := difference(current, previous)
			// собираем необходимую информацию и отправляем её посредством Discord Webhook
			BuildAndSendWebhooks(diffResult, previous, sumAllCases, sumAllDeaths, sumAllCured)
			previous = current
		}
		time.Sleep(5 * time.Minute)
	}
}

func difference(slice1 []*Items, slice2 []*Items) []*Items {
	var diff []*Items
	for _, s1 := range slice1 {
		changed := false
		for _, s2 := range slice2 {
			if s1.Name == s2.Name && (s1.Cases != s2.Cases || s1.Deaths != s2.Deaths || s1.Cured != s2.Cured) {
				changed = true
				break
			}
		}
		if changed {
			diff = append(diff, s1)
		}
	}
	return diff
}

func BuildAndSendWebhooks(diffResult []*Items, previous []*Items, sumAllCases []int, sumAllDeaths []int, sumAllCured []int) {
	webhook := wh.NewDiscordWebhook(discordWhUrl)
	webhook.SetStatusYellow()
	if len(diffResult) > 0 {
		sliceOfDescriptions := make([]string, 0)
		sumChangeCases := make([]int, 0)
		sumChangeDeaths := make([]int, 0)
		sumChangeRecovered := make([]int, 0)
		for _, diffSlice := range diffResult {
			for _, previousSlice := range previous {
				if diffSlice.Name == previousSlice.Name {
					changeCases := diffSlice.Cases - previousSlice.Cases
					changeDeaths := diffSlice.Deaths - previousSlice.Deaths
					changeRecovered := diffSlice.Cured - previousSlice.Cured
					var deltaCases string
					var deltaDeaths string
					var deltaRecovered string
					if changeCases > 0 {
						deltaCases = fmt.Sprintf(" (+%v)", changeCases)
					} else if changeCases < 0 {
						deltaCases = fmt.Sprintf(" (%v)", changeCases)
					}
					if changeDeaths > 0 {
						deltaDeaths = fmt.Sprintf(" (+%v)", changeDeaths)
					} else if changeDeaths < 0 {
						deltaDeaths = fmt.Sprintf(" (%v)", changeDeaths)
					}
					if changeRecovered > 0 {
						deltaRecovered = fmt.Sprintf(" (+%v)", changeRecovered)
					} else if changeRecovered < 0 {
						deltaRecovered = fmt.Sprintf(" (%v)", changeRecovered)
					}
					description := fmt.Sprintf("**%s** №%v\n Заражений: %v"+deltaCases+"\n Заражённых сейчас: %v\n Погибших: %v"+deltaDeaths+"\n Выздоровевших: %v"+deltaRecovered+"\n\n", diffSlice.Name, diffSlice.Number, diffSlice.Cases, diffSlice.Cases-diffSlice.Deaths-diffSlice.Cured, diffSlice.Deaths, diffSlice.Cured)
					sliceOfDescriptions = append(sliceOfDescriptions, description)
					sumChangeCases = append(sumChangeCases, changeCases)
					sumChangeDeaths = append(sumChangeDeaths, changeDeaths)
					sumChangeRecovered = append(sumChangeRecovered, changeRecovered)
				}
			}
		}
		descriptionForSumAllCases := fmt.Sprintf("Всего в России зафиксировано **%v** cлучаев заболевания,\n", sumSlicesItem(sumAllCases))                    // количество всех случаев заболевания
		descriptionForSumAllDeaths := fmt.Sprintf("погибло **%v**", sumSlicesItem(sumAllDeaths))                                                              // количество всех случаев гибели
		descriptionForSumAllCured := fmt.Sprintf(", выздоровело **%v**.\n\n", sumSlicesItem(sumAllCured))                                                     // количество всех случаев выздоровления
		descriptionForSumChangeCases := fmt.Sprintf("За последние сутки в России зафиксировано **%v** cлучаев заболевания,\n", sumSlicesItem(sumChangeCases)) // разница в количестве случаев заболевания между текущим и предыдущим запросом
		descriptionForSumChangeDeaths := fmt.Sprintf("погибло **%v**", sumSlicesItem(sumChangeDeaths))                                                        // разница в количестве случаев гибели между текущим и предыдущим запросом
		descriptionForSumChangeRecovered := fmt.Sprintf(", выздоровело **%v**.", sumSlicesItem(sumChangeRecovered))                                           // разница в количестве случаев выздоровления между текущим и предыдущим запросом
		allDescriptionsForSum := descriptionForSumAllCases + descriptionForSumAllDeaths + descriptionForSumAllCured + descriptionForSumChangeCases + descriptionForSumChangeDeaths + descriptionForSumChangeRecovered

		switch {
		case len(diffResult) > 0 && len(diffResult) <= 17:
			firstMessage := strings.Join(sliceOfDescriptions, "")
			webhook.SetDescription(fmt.Sprintln(firstMessage))
			err = webhook.Send()
			if err != nil {
				fmt.Println(err)
			}
		case len(diffResult) > 17 && len(diffResult) <= 34:
			firstMessage := strings.Join(sliceOfDescriptions[:17], "")
			secondMessage := strings.Join(sliceOfDescriptions[17:], "")
			webhook.SetDescription(fmt.Sprintln(firstMessage))
			err = webhook.Send()
			if err != nil {
				fmt.Println(err)
			}
			time.Sleep(pauseBetweenMesages * time.Second)
			webhook.SetDescription(fmt.Sprintln(secondMessage))
			err = webhook.Send()
			if err != nil {
				fmt.Println(err)
			}
		case len(diffResult) > 34 && len(diffResult) <= 51:
			firstMessage := strings.Join(sliceOfDescriptions[:17], "")
			secondMessage := strings.Join(sliceOfDescriptions[17:34], "")
			thirdMessage := strings.Join(sliceOfDescriptions[34:], "")
			webhook.SetDescription(fmt.Sprintln(firstMessage))
			err = webhook.Send()
			if err != nil {
				fmt.Println(err)
			}
			time.Sleep(pauseBetweenMesages * time.Second)
			webhook.SetDescription(fmt.Sprintln(secondMessage))
			err = webhook.Send()
			if err != nil {
				fmt.Println(err)
			}
			time.Sleep(pauseBetweenMesages * time.Second)
			webhook.SetDescription(fmt.Sprintln(thirdMessage))
			err = webhook.Send()
			if err != nil {
				fmt.Println(err)
			}
		case len(diffResult) > 51 && len(diffResult) <= 68:
			firstMessage := strings.Join(sliceOfDescriptions[:17], "")
			secondMessage := strings.Join(sliceOfDescriptions[17:34], "")
			thirdMessage := strings.Join(sliceOfDescriptions[34:51], "")
			fourthMessage := strings.Join(sliceOfDescriptions[51:], "")
			webhook.SetDescription(fmt.Sprintln(firstMessage))
			err = webhook.Send()
			if err != nil {
				fmt.Println(err)
			}
			time.Sleep(pauseBetweenMesages * time.Second)
			webhook.SetDescription(fmt.Sprintln(secondMessage))
			err = webhook.Send()
			if err != nil {
				fmt.Println(err)
			}
			time.Sleep(pauseBetweenMesages * time.Second)
			webhook.SetDescription(fmt.Sprintln(thirdMessage))
			err = webhook.Send()
			if err != nil {
				fmt.Println(err)
			}
			time.Sleep(pauseBetweenMesages * time.Second)
			webhook.SetDescription(fmt.Sprintln(fourthMessage))
			err = webhook.Send()
			if err != nil {
				fmt.Println(err)
			}
		case len(diffResult) > 68 && len(diffResult) <= 85:
			firstMessage := strings.Join(sliceOfDescriptions[:17], "")
			secondMessage := strings.Join(sliceOfDescriptions[17:34], "")
			thirdMessage := strings.Join(sliceOfDescriptions[34:51], "")
			fourthMessage := strings.Join(sliceOfDescriptions[51:68], "")
			fiveMessage := strings.Join(sliceOfDescriptions[68:], "")
			webhook.SetDescription(fmt.Sprintln(firstMessage))
			err = webhook.Send()
			if err != nil {
				fmt.Println(err)
			}
			time.Sleep(pauseBetweenMesages * time.Second)
			webhook.SetDescription(fmt.Sprintln(secondMessage))
			err = webhook.Send()
			if err != nil {
				fmt.Println(err)
			}
			time.Sleep(pauseBetweenMesages * time.Second)
			webhook.SetDescription(fmt.Sprintln(thirdMessage))
			err = webhook.Send()
			if err != nil {
				fmt.Println(err)
			}
			time.Sleep(pauseBetweenMesages * time.Second)
			webhook.SetDescription(fmt.Sprintln(fourthMessage))
			err = webhook.Send()
			if err != nil {
				fmt.Println(err)
			}
			time.Sleep(pauseBetweenMesages * time.Second)
			webhook.SetDescription(fmt.Sprintln(fiveMessage))
			err = webhook.Send()
			if err != nil {
				fmt.Println(err)
			}
		}
		time.Sleep(pauseBetweenMesages * time.Second)
		webhook.SetDescription(fmt.Sprintln(allDescriptionsForSum))
		err = webhook.Send()
		if err != nil {
			fmt.Println(err)
		}
	}
}

func sumSlicesItem(x []int) int {
	totalx := 0
	for _, valuex := range x {
		totalx += valuex
	}
	return totalx
}

func sendRangeBoundsErrorWebhooks() {
	webhook := wh.NewDiscordWebhook(errorDiscordWhUrl)
	webhook.SetStatusRed()
	webhook.SetDescription("Структура " + url + " изменилась. Необходимо внести изменения.")
	err = webhook.Send()
	if err != nil {
		fmt.Println(err)
	}
}

func sendErrorWebhooks(err error) {
	webhook := wh.NewDiscordWebhook(errorDiscordWhUrl)
	webhook.SetStatusRed()
	webhook.SetDescription("Источник: " + url + "\nОшибка: " + err.Error())
	err = webhook.Send()
	if err != nil {
		fmt.Println(err)
	}
}

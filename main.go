package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/akamensky/argparse"
	"github.com/joho/godotenv"
	"github.com/malaow3/trunk"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
)

func main() {
	trunk.InitLogger()

	parser := argparse.NewParser("clearhistory", "Delete history from funimation")
	show := parser.String("s", "show", &argparse.Options{Required: false, Help: "show to delete"})
	number := parser.Int("n", "number", &argparse.Options{
		Required: false,
		Help:     "number of items to delete",
		Validate: func(args []string) error {
			intval, err := strconv.Atoi(args[0])
			if err != nil {
				return errors.New("number must be an integer")
			}
			if intval < 1 {
				return errors.New("value must be greater than or equal to 1")
			}
			return nil
		},
	})
	all := parser.Flag("a", "all", &argparse.Options{Required: false, Help: "delete all history"})

	err := parser.Parse(os.Args)
	if err != nil {
		log.Error(parser.Usage(err))
		return
	}

	if *show == "" && *number == 0 && !*all {
		log.Error("show, number, or flag must be used")
		return
	}

	argsCalled := 0
	if *show != "" {
		argsCalled++
	}
	if *number != 0 {
		argsCalled++
	}
	if *all {
		argsCalled++
	}
	if argsCalled > 1 {
		log.Error("only one of show, number, or flag can be used")
		return
	}

	if err := godotenv.Load(); err != nil {
		log.Info("No .env file found")
	}

	username := os.Getenv("FUNUSERNAME")
	password := os.Getenv("FUNPASSWORD")

	if username == "" || password == "" {
		log.Error("username and password must be set as environment variables -- .env file is preferable")
		return
	}

	token := getToken(username, password)
	if token == "" {
		log.Error("Could not get token")
		return
	}
	log.Info("TOKEN: " + token)

	var historyItems []string
	if *show != "" {
		historyItems = getHistory(*show, 0, token, false)
	}
	if *number > 0 {
		historyItems = getHistory("", *number, token, false)
	}
	if *all {
		historyItems = getHistory("", 0, token, true)
	}

	deleteItems(historyItems, token)

}

func deleteItems(historyIDs []string, token string) {
	for _, historyID := range historyIDs {
		url := "https://prod-api-funimationnow.dadcdigital.com/api/source/funimation/history/" + historyID + "/"
		method := "DELETE"

		client := &http.Client{}
		req, err := http.NewRequest(method, url, nil)
		trunk.CheckErr(err)

		req.Header.Add("Authorization", "Token "+token)

		res, err := client.Do(req)
		trunk.CheckErr(err)

		body, err := ioutil.ReadAll(res.Body)
		trunk.CheckErr(err)

		res.Body.Close()
		if res.StatusCode != http.StatusOK {
			log.Error("Error deleting item: " + string(body))
		} else {
			log.Info("Item deleted: " + historyID)
		}
	}
}

func getHistory(show string, number int, token string, all bool) []string {
	offset := 0
	url := "https://prod-api-funimationnow.dadcdigital.com/api/source/funimation/history/?return_all=true&offset=" + strconv.Itoa(offset) + "&limit=25"
	method := "GET"

	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)
	trunk.CheckErr(err)

	req.Header.Add("Authorization", "Token "+token)

	res, err := client.Do(req)
	trunk.CheckErr(err)

	body, err := ioutil.ReadAll(res.Body)
	trunk.CheckErr(err)

	history := []gjson.Result{}
	res.Body.Close()
	items := gjson.Get(string(body), "items").Array()
	for len(items) > 0 {
		history = append(history, items...)
		offset += 25
		url := "https://prod-api-funimationnow.dadcdigital.com/api/source/funimation/history/?return_all=true&offset=" + strconv.Itoa(offset) + "&limit=25"
		method := "GET"

		client := &http.Client{}
		req, err := http.NewRequest(method, url, nil)
		trunk.CheckErr(err)

		req.Header.Add("Authorization", "Token "+token)

		res, err := client.Do(req)
		trunk.CheckErr(err)

		body, err := ioutil.ReadAll(res.Body)
		trunk.CheckErr(err)

		res.Body.Close()
		items = gjson.Get(string(body), "items").Array()
	}

	historyIDs := []string{}

	if all {
		for _, item := range history {
			historyIDs = append(historyIDs, item.Get("external_ver_id").String())
		}
		return historyIDs
	}

	if number > 0 {
		for i := 0; i < number; i++ {
			historyIDs = append(historyIDs, history[i].Get("external_ver_id").String())
		}
		return historyIDs
	}

	if show != "" {
		for _, item := range history {
			if strings.Contains(item.Get("show_title").String(), show) {
				historyIDs = append(historyIDs, item.Get("external_ver_id").String())
			}
		}
		return historyIDs
	}

	return nil

}

// getToken gets a token from the login API.
func getToken(username, password string) string {
	urlString := "https://prod-api-funimationnow.dadcdigital.com/api/auth/login/"
	method := "POST"

	payload := strings.NewReader("username=" + url.QueryEscape(username) + "&password=" + url.QueryEscape(password))

	client := &http.Client{}
	req, err := http.NewRequest(method, urlString, payload)
	trunk.CheckErr(err)

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	res, err := client.Do(req)
	trunk.CheckErr(err)
	body, err := ioutil.ReadAll(res.Body)
	trunk.CheckErr(err)

	res.Body.Close()

	type jsonResponse struct {
		Token string `json:"token"`
	}

	var response jsonResponse
	err = json.Unmarshal(body, &response)
	trunk.CheckErr(err)

	return response.Token
}

package bybit

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/spf13/viper"
)

const (
	RECV_WINDOW_MILLISECOND = "3000"
)

func (api *Api) post(endpoint string, params map[string]any) (body []byte, err error) {
	// timestamp
	ts := time.Now().UnixNano() / int64(time.Millisecond)

	// body
	jsonData, err := json.Marshal(params)
	if err != nil {
		return
	}

	// signature
	hmac256 := hmac.New(sha256.New, []byte(viper.GetString("BYBIT_API_SECRET")))
	_, err = hmac256.Write([]byte(strconv.FormatInt(ts, 10) + viper.GetString("BYBIT_API_KEY") + RECV_WINDOW_MILLISECOND + string(jsonData)))
	if err != nil {
		return
	}
	signature := hex.EncodeToString(hmac256.Sum(nil))

	// url
	url := viper.GetString("BYBIT_API_HOST") + endpoint

	// make request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-BAPI-API-KEY", viper.GetString("BYBIT_API_KEY"))
	req.Header.Set("X-BAPI-SIGN", signature)
	req.Header.Set("X-BAPI-TIMESTAMP", strconv.FormatInt(ts, 10))
	req.Header.Set("X-BAPI-RECV-WINDOW", RECV_WINDOW_MILLISECOND)

	// Send the request using an http.Client
	resp, err := api.Client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	// Reading the response body
	body, err = io.ReadAll(resp.Body)
	return
}

func (api *Api) get(endpoint string, params map[string]string) (body []byte, err error) {
	// timestamp
	ts := time.Now().UnixNano() / int64(time.Millisecond)

	// Prepare URL parameters
	query := url.Values{}
	for k, v := range params {
		query.Add(k, v)
	}

	// signature
	hmac256 := hmac.New(sha256.New, []byte(viper.GetString("BYBIT_API_SECRET")))
	_, err = hmac256.Write([]byte(strconv.FormatInt(ts, 10) + viper.GetString("BYBIT_API_KEY") + RECV_WINDOW_MILLISECOND + query.Encode()))
	if err != nil {
		return
	}
	signature := hex.EncodeToString(hmac256.Sum(nil))

	// Construct the full URL with parameters
	url := viper.GetString("BYBIT_API_HOST") + endpoint
	fullURL := url + "?" + query.Encode()

	// Create a new request using http
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		fmt.Printf("Error creating request: %s\n", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-BAPI-API-KEY", viper.GetString("BYBIT_API_KEY"))
	req.Header.Set("X-BAPI-SIGN", signature)
	req.Header.Set("X-BAPI-TIMESTAMP", strconv.FormatInt(ts, 10))
	req.Header.Set("X-BAPI-RECV-WINDOW", RECV_WINDOW_MILLISECOND)

	// Send the request using an http.Client
	resp, err := api.Client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	// Reading the response body
	body, err = io.ReadAll(resp.Body)
	return
}

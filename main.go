package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"fmt"
	"time"

	random "math/rand"

	hdwallet "github.com/miguelmota/go-ethereum-hdwallet"
	"github.com/tyler-smith/go-bip39"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpproxy"
	"github.com/aherve/gopool"
	
)

var headerContentTypeJson = []byte("application/json")

type getBalanceRequestData struct {
	Jsonrpc string    `json:"jsonrpc"`
	Method  string    `json:"method"`
	Params  [2]string `json:"params"`
	Id      int       `json:"id"`
}

type getBalanceResponseData struct {
	Jsonrpc string `json:"jsonrpc"`
	Id      int    `json:"id"`
	Result  string `json:"result"`
}

func generate_pair() (string, string) {
	entropy, _ := bip39.NewEntropy(128)
	mnemonic, _ := bip39.NewMnemonic(entropy)
	wallet, err := hdwallet.NewFromMnemonic(mnemonic)
	if err != nil {
		log.Fatal(err)
	}

	path := hdwallet.MustParseDerivationPath("m/44'/60'/0'/0/0")
	account, err := wallet.Derive(path, false)
	if err != nil {
		log.Fatal(err)
	}
	return mnemonic, account.Address.Hex()
}

func httpConnError(err error) (string, bool) {
	errName := ""
	known := false
	if err == fasthttp.ErrTimeout {
		errName = "timeout"
		known = true
	} else if err == fasthttp.ErrNoFreeConns {
		errName = "conn_limit"
		known = true
	} else if err == fasthttp.ErrConnectionClosed {
		errName = "conn_close"
		known = true
	} else {
		errName = reflect.TypeOf(err).String()
		if errName == "*net.OpError" {
			errName = "timeout"
			known = true
		}
	}
	return errName, known
}

func checkBalance(client *fasthttp.Client, address string, url_rpc string) (string, error) {
	reqTimeout := time.Duration(10000) * time.Millisecond

	requestEntity := &getBalanceRequestData{
		Jsonrpc: "2.0",
		Method:  "eth_getBalance",
		Params:  [2]string{address, "latest"},
		Id:      1,
	}
	reqEntityBytes, _ := json.Marshal(requestEntity)

	responseEntity := &getBalanceResponseData{}

	request := fasthttp.AcquireRequest()
	request.SetRequestURI(url_rpc)
	request.Header.SetMethod(fasthttp.MethodPost)
	request.Header.SetContentTypeBytes(headerContentTypeJson)
	request.SetBodyRaw(reqEntityBytes)
	resp := fasthttp.AcquireResponse()
	err := client.DoTimeout(request, resp, reqTimeout)
	fasthttp.ReleaseRequest(request)
	if err == nil {
		statusCode := resp.StatusCode()
		respBody := resp.Body()
		if statusCode == http.StatusOK {
			err = json.Unmarshal(respBody, responseEntity)
		} else {
			err = errors.New(string(statusCode))
		}
	} else {
		errName, _ := httpConnError(err)
		err = errors.New(errName)
	}
	fasthttp.ReleaseResponse(resp)
	return responseEntity.Result, err
}

func createClient(instance_id int) *fasthttp.Client {
	tor_user := random.Int()
	tor_pass := random.Int()
	tor_port:= 9060+instance_id*2

	readTimeout, _ := time.ParseDuration("10000ms")
	writeTimeout, _ := time.ParseDuration("10000ms")
	maxIdleConnDuration, _ := time.ParseDuration("10s")
	client := &fasthttp.Client{
		ReadTimeout:                   readTimeout,
		WriteTimeout:                  writeTimeout,
		MaxIdleConnDuration:           maxIdleConnDuration,
		NoDefaultUserAgentHeader:      true,
		DisableHeaderNamesNormalizing: true,
		DisablePathNormalizing:        true,
		Dial: fasthttpproxy.FasthttpSocksDialer("socks5://" + strconv.Itoa(tor_user) + ":" + strconv.Itoa(tor_pass) + "@localhost:"+ strconv.Itoa(tor_port)),
	}
	return client
}

func main() {
	VISUAL_LOGS := false
	f, err := os.OpenFile("success.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	f2, err := os.OpenFile("work.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer f2.Close()

	formatString := "| %s | %s | %s | %s\n"

	infoLog := log.New(os.Stdout, "\rINFO    ", log.Ltime)
	successFileLog := log.New(f, "\rSUCCESS ", log.Ltime)
	successLog := log.New(os.Stdout, "SUCCESS ", log.Ltime)
	errorLog := log.New(os.Stdout, "\rERROR   ", log.Ltime)
	workLog := log.New(f2, "PROGRESS\t", log.Ldate|log.Ltime)

	each_file := 50000
	each_terminal := 500
	success := 0
	errors  :=0
	MAX_TOR_INSTANCES := 40
	MAX_TOR_CONNECTIONS:=960
	counter := 0

	workLog.Println("Started")

	var links = []string{"https://eth-mainnet.gateway.pokt.network/v1/5f3453978e354ab992c4da79",
	"https://main-rpc.linkpool.io/","https://cloudflare-eth.com/","https://api.mycryptoapi.com/eth","https://ethereumnodelight.app.runonflux.io"}
	var total_links=len(links)
	
	start_time := time.Now()
	pool := gopool.NewPool(MAX_TOR_CONNECTIONS)
	for i := 0; true; i++ {
		pool.Add(1)
		go func() {
			defer pool.Done()
			prefix := "ETH"
			link := links[i%total_links]
			client := createClient(i%MAX_TOR_INSTANCES)

			// reuse good connection until death
			for {
				mnemonic, public := generate_pair()
				result, err := checkBalance(client, public, link)
				counter+=1
				if err == nil {
					if result != "0x0" && result != "" {
						successLog.Printf(formatString, prefix, public, result, mnemonic)
						successFileLog.Printf(formatString, prefix, public, result, mnemonic)
						success += 1
					} else {
						if result != "0x0" {
							errors+=1
						}
						if VISUAL_LOGS {
							infoLog.Printf(formatString, prefix, public, result, mnemonic)
						}
						
					}
				} else {
					// end connection on first error
					errors+=1
					if VISUAL_LOGS {
						errorLog.Printf(formatString, prefix, public, err.Error(), mnemonic)
					}
					break
				}
				if counter%each_terminal==0{
					current_time := time.Now()
					duration := current_time.Sub(start_time)
					duration_seconds := current_time.Sub(start_time).Seconds()
					duration_time:=time.Time{}.Add(duration).Format("15:04:05")
					speed := float32(counter)/float32(duration_seconds)
					error_rate:=float32(errors)/float32(counter)*100
					fmt.Printf("\rTotal: %d    Used ips: %d    Success: %d    Errors: %d    Error rate: %.2f%%    Speed: %f r/s    Uptime: %s",counter,i,success,errors,error_rate,speed,duration_time)
				
				}
				if counter%each_file == 0 {
					current_time := time.Now()
					duration := current_time.Sub(start_time)
					duration_seconds := current_time.Sub(start_time).Seconds()
					duration_time:=time.Time{}.Add(duration).Format("15:04:05")
					speed := float32(counter)/float32(duration_seconds)
					error_rate:=float32(errors)/float32(counter)*100
					workLog.Printf("Total: %d    Used ips: %d    Success: %d    Errors: %d    Error rate: %.2f%%    Speed: %f r/s    Uptime: %s",counter,i,success,errors,error_rate,speed,duration_time)
				}
			}
			

		}()
	}
	pool.Wait()
}

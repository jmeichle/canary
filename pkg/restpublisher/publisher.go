package restpublisher

import (
    "fmt"
    "log"
    "os"
    "strconv"
    "time"
    "encoding/json"
    "net/http"
    "bytes"
    "github.com/canaryio/canary/pkg/sensor"
)

// Publisher implements canary.Publisher, and is our
// gateway for delivering canary.Measurement data to STDOUT.
type Publisher struct{
    ResultChan chan RestApiResult
    Config config
}

type config struct{
    RestApiEndpoint string
    RestApiBufferCount int
}

// New returns a pointer to a new Publsher.
func New() *Publisher {
    return &Publisher{}
}

type RestApiResult struct{
    RequestTimestamp   string `json:"request_timestamp"`
    Url  string `json:"url"`
    StatusCode int `json:"status_code"`
    RequestTime float64 `json:"request_time"`
    Passed bool `json:"passed"`
    ErrorMessage string `json:"plugin_output"`
}


func (p *Publisher) RestPublisher(c chan RestApiResult) {
    var activeindex int
    activeindex = 0
    var arr = make([]RestApiResult,p.Config.RestApiBufferCount)
    for {
        rest_result := <-c
        arr[activeindex] = rest_result
        if activeindex == (p.Config.RestApiBufferCount-1) {
            fmt.Println("Posting last " + strconv.Itoa(p.Config.RestApiBufferCount) + " results to " + p.Config.RestApiEndpoint)
            activeindex = 0

            jsonStr, err := json.Marshal(arr)
            if err != nil {
                fmt.Printf("ERROR")
            }

            req, err := http.NewRequest("POST", p.Config.RestApiEndpoint, bytes.NewBuffer(jsonStr))
            // req.Header.Set("X-Custom-Header", "myvalue")
            req.Header.Set("Content-Type", "application/json")

            client := &http.Client{}
            resp, err := client.Do(req)
            if err != nil {
                panic(err)
            }
            defer resp.Body.Close()

        } else {
            activeindex++
        }
        time.Sleep(time.Second)
    }
}

func (p *Publisher) GetConfig() (c config, err error) {
    rest_url := os.Getenv("REST_ENDPOINT_URL")
    if rest_url == "" {
        err = fmt.Errorf("REST_ENDPOINT_URL not defined in ENV")
    }
    if err != nil {
        log.Fatal(err)
    }
    c.RestApiEndpoint = rest_url

    rest_buffer_size := os.Getenv("REST_BUFFER_SIZE")
    if rest_buffer_size == "" {
        err = fmt.Errorf("REST_BUFFER_SIZE is not defined in ENV")
    }
    if err != nil {
        log.Fatal(err)
    }
    rest_buffer_size_int, err := strconv.Atoi(rest_buffer_size)
    if err != nil {
        log.Fatal(err)
    }
    c.RestApiBufferCount = rest_buffer_size_int
    return
}

func (p *Publisher) Setup() {
    the_config, err := p.GetConfig()
    if err != nil {
        log.Fatal(err)
    }
    p.Config = the_config

    // buffered output channel.
    p.ResultChan = make(chan RestApiResult)
    // start the rest_pusher method listening on this channel
    go p.RestPublisher(p.ResultChan)
}

// Publish takes a canary.Measurement and emits data to STDOUT.
func (p *Publisher) Publish(m sensor.Measurement) (err error) {
    duration := m.Sample.T2.Sub(m.Sample.T1).Seconds() * 1000

    isOK := true
    if m.Error != nil {
        isOK = false
    }

    errMessage := ``
    if m.Error != nil {
        errMessage = fmt.Sprintf("'%s'", m.Error)
    }

    rest_result := RestApiResult{RequestTimestamp:m.Sample.T2.Format(time.RFC3339), Url:m.Target.URL, StatusCode:m.Sample.StatusCode, RequestTime:duration, Passed:isOK, ErrorMessage: errMessage}
    p.ResultChan <- rest_result

    return
}
package canarykinesispublisher

import (
  "fmt"
  "time"

  "log"
  "os"
  "strconv"

  "github.com/aws/aws-sdk-go/service/kinesis"
  "github.com/aws/aws-sdk-go/aws"
  "github.com/aws/aws-sdk-go/aws/session"

  "github.com/canaryio/canary/pkg/sensor"
)

const MAX_BUFFER = 100

type Publisher struct {
  streamName      string
  flushInterval   time.Duration
  buffer          []*sensor.Measurement
  measurementChan chan *sensor.Measurement
  bufferIndex      int
  kinesisService  *kinesis.Kinesis

}

func New(streamName string, kinesisRegion *string, interval time.Duration) *Publisher {
  svc := kinesis.New(session.New(), &aws.Config{Region: kinesisRegion})
  p := &Publisher{
    buffer:           make([]*sensor.Measurement, MAX_BUFFER),
    measurementChan:  make(chan *sensor.Measurement),
    streamName:       streamName,
    flushInterval:    interval,
    bufferIndex:      0,
    kinesisService:   svc,
  }
  go p.run()
  return p
}

func NewFromEnv() (*Publisher, error) {
  streamName := os.Getenv("KINESIS_STREAM_NAME")
  if streamName == "" {
    return nil, fmt.Errorf("KINESIS_STREAM_NAME not set in ENV")
  }

  sInterval := os.Getenv("KINESIS_PUBLISHER_INTERVAL")
  if sInterval == "" {
    sInterval = "10"
  }

  kRegion := os.Getenv("KINESIS_PUBLISHER_REGION")
  if kRegion == "" {
    kRegion = "us-east-1"
  }

  interval, err := strconv.Atoi(sInterval)
  if err != nil {
    return nil, fmt.Errorf("KINESIS_PUBLISHER_INTERVAL %s not parsable as an int", sInterval)
  }

  return New(streamName, aws.String(kRegion), time.Duration(interval)*time.Second), nil
}

func (p *Publisher) Publish(m sensor.Measurement) error {
  p.measurementChan <- &m
  return nil
}

func (p *Publisher) run() {
  t := time.NewTicker(p.flushInterval)

  for {
    select {
    case <-t.C:
      // flush when our ticker fires
      if err := p.flush(); err != nil {
        log.Print(err)
      }
    case m := <-p.measurementChan:
      p.buffer[p.bufferIndex] = m
      p.bufferIndex++

      // flush if we've exceeded the bounds of our buffer
      if p.bufferIndex >= MAX_BUFFER {
        if err := p.flush(); err != nil {
          log.Print(err)
        }
      }
    }
  }
}

func (p *Publisher) flush() error {
  defer func() { p.bufferIndex = 0 }()

  measurements := p.buffer[:p.bufferIndex]

  for _, measurement := range measurements {
    errMessage := ``
    if measurement.Error != nil {
      errMessage = fmt.Sprintf("'%s'", measurement.Error)
    }
    payload := fmt.Sprintf(
      "%s %s %d %f %t %d %s\n",
      measurement.Sample.TimeEnd.Format(time.RFC3339),
      measurement.Target.URL,
      measurement.Sample.StatusCode,
      measurement.Sample.TimeEnd.Sub(measurement.Sample.TimeStart).Seconds()*1000,
      measurement.IsOK,
      measurement.StateCount,
      errMessage,
    )

    fmt.Printf(payload)


    params := &kinesis.PutRecordInput{
      Data:                      []byte(payload),                 // Required
      PartitionKey:              aws.String("PartitionKey"),      // Required
      StreamName:                &p.streamName,                    // Required
      // ExplicitHashKey:           aws.String("HashKey"),        // @todo figure out if we want to set this
      // SequenceNumberForOrdering: aws.String("SequenceNumber"), // @todo figure out if we want to set this
    }
    put_resp, err := p.kinesisService.PutRecord(params)
    if err != nil {
      fmt.Println(err.Error())
      return err
    } else {
      fmt.Printf("%s : %s %s\n", measurement.Target.URL, *put_resp.ShardId, *put_resp.SequenceNumber)  
    }
  }


  return nil
}

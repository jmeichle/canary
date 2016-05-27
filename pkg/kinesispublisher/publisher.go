package canarykinesispublisher

import (
  "fmt"
  "time"
  "github.com/aws/aws-sdk-go/service/kinesis"
  "github.com/aws/aws-sdk-go/aws"
  "github.com/aws/aws-sdk-go/aws/session"

  "github.com/canaryio/canary/pkg/sensor"
)

// Publisher implements canary.Publisher, and is our
// gateway for delivering canary.Measurement data to AWS Kinesis
type Publisher struct{}

// New returns a pointer to a new Publsher.
func New() *Publisher {
  return &Publisher{}
}

// Publish takes a canary.Measurement and emits data to STDOUT.
func (p *Publisher) Publish(m sensor.Measurement) (err error) {
  errMessage := ``
  if m.Error != nil {
    errMessage = fmt.Sprintf("'%s'", m.Error)
  }

  payload := fmt.Sprintf(
    "%s %s %d %f %t %d %s\n",
    m.Sample.TimeEnd.Format(time.RFC3339),
    m.Target.URL,
    m.Sample.StatusCode,
    m.Sample.TimeEnd.Sub(m.Sample.TimeStart).Seconds()*1000,
    m.IsOK,
    m.StateCount,
    errMessage,
  )

  svc := kinesis.New(session.New(), &aws.Config{Region: aws.String("us-east-1")})
  params := &kinesis.PutRecordInput{
    Data:                      []byte(payload),                 // Required
    PartitionKey:              aws.String("PartitionKey"),      // Required
    StreamName:                aws.String("jmeichle"),          // Required
    // ExplicitHashKey:           aws.String("HashKey"),        // @todo figure out if we want to set this
    // SequenceNumberForOrdering: aws.String("SequenceNumber"), // @todo figure out if we want to set this
  }
  put_resp, err := svc.PutRecord(params)

  fmt.Printf("%s : %s %s\n", m.Target.URL, *put_resp.ShardId, *put_resp.SequenceNumber)

  if err != nil {
    // Print the error, cast err to awserr.Error to get the Code and
    // Message from an error.
    fmt.Println(err.Error())
    return
  }

  return
}

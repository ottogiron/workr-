package rabbit

import (
	"fmt"
	"strconv"
	"time"

	"github.com/go-redis/redis"
	"github.com/ottogiron/metricsworker/worker"
	"github.com/streadway/amqp"
)

var _ worker.Worker = (*DistinctNameWorker)(nil)

const (
	collectionName = "counters"
	idCounter      = "distinctName:id"
)

//DistinctNameWorker implementation of distinctname worker
type DistinctNameWorker struct {
	rclient *redis.Client
}

//NewDistincNameWorker returns a new instance of a distinctName worker
func NewDistincNameWorker(client *redis.Client) *DistinctNameWorker {
	return &DistinctNameWorker{client}
}

//Execute executes a  DistinctNameWorker  task
func (w *DistinctNameWorker) Execute(task interface{}) error {
	delivery, ok := task.(amqp.Delivery)

	if !ok {
		return fmt.Errorf("Task should be a rabbit delivery %v", task)
	}

	countMetric, err := worker.UnmarshallCountMetricToMapInterface(delivery.Body)

	if err != nil {
		return fmt.Errorf("Failed to unmarshall rabbit delivery body (%s) %s ", string(delivery.Body), err)
	}
	eventName := countMetric["metric"].(string)
	id := w.rclient.Incr(idCounter + ":" + eventName)

	if id.Err() != nil {
		return fmt.Errorf("Failed to create metric id %s", id.Err())
	}

	eventID := eventName + ":" + strconv.FormatInt(id.Val(), 10)

	p := w.rclient.Pipeline()

	p.HMSet(eventID, countMetric)

	nowTimestamp := time.Now().UTC().Unix()
	p.ZAdd("events", redis.Z{
		Score:  float64(nowTimestamp),
		Member: eventID,
	})

	_, err = p.Exec()

	if err != nil {
		return fmt.Errorf("Failed to store event in redis %s %v", err, countMetric)
	}
	return nil
}

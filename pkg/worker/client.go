package worker

import (
	"fmt"

	"github.com/danvixent/asynqmon"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/mujhtech/dagryn/pkg/encrypt"
)

var (
	ErrTaskNotFound  = fmt.Errorf("asynq: %w", asynq.ErrTaskNotFound)
	ErrQueueNotFound = fmt.Errorf("asynq: %w", asynq.ErrQueueNotFound)
)

type Client struct {
	redisConnOpt asynq.RedisConnOpt
	client       *asynq.Client
	inspector    *asynq.Inspector
	cfb          encrypt.Encrypt
}

func NewClient(opts asynq.RedisConnOpt, cfb encrypt.Encrypt) *Client {

	return &Client{
		redisConnOpt: opts,
		client:       asynq.NewClient(opts),
		inspector:    asynq.NewInspector(opts),
		cfb:          cfb,
	}
}

func (c *Client) Enqueue(queue QueueName, job TaskName, payload *ClientPayload) error {

	id := uuid.New().String()

	q := string(queue)

	data, err := c.cfb.Encrypt(payload.Data)

	if err != nil {
		return err
	}

	t := asynq.NewTask(string(job), []byte(data), asynq.Queue(q), asynq.TaskID(id), asynq.ProcessIn(payload.Delay))

	_, err = c.inspector.GetTaskInfo(q, id)
	if err != nil {

		message := err.Error()
		if ErrQueueNotFound.Error() == message || ErrTaskNotFound.Error() == message {
			_, err := c.client.Enqueue(t, nil)
			return err
		}

		return err
	}

	// Delete the task if it already exists
	err = c.inspector.DeleteTask(q, id)

	if err != nil {
		return err
	}

	// Enqueue the task
	if _, err := c.client.Enqueue(t, nil); err != nil {
		return err
	}

	return nil
}

// EnqueueRaw enqueues a job with raw data bytes (satisfies handlers.JobEnqueuer).
func (c *Client) EnqueueRaw(queue, taskName string, data []byte) error {
	return c.Enqueue(QueueName(queue), TaskName(taskName), &ClientPayload{Data: data})
}

type Formatter struct {
	cfb encrypt.Encrypt
}

func (c *Client) Monitor() *asynqmon.HTTPHandler {
	h := asynqmon.New(asynqmon.Options{
		RootPath:     "/queue/monitoring",
		RedisConnOpt: c.redisConnOpt,
		PayloadFormatter: Formatter{
			cfb: c.cfb,
		},
		ResultFormatter: Formatter{
			cfb: c.cfb,
		},
	})
	return h
}

func (q *Client) Inspector() *asynq.Inspector {
	return q.inspector
}

func (f Formatter) FormatPayload(_ string, payload []byte) string {
	data, err := f.cfb.Decrypt(string(payload))

	if err != nil {
		return ""
	}

	// bytes, _ := json.Marshal(data)
	// return string(bytes)
	return data
}

func (f Formatter) FormatResult(_ string, payload []byte) string {
	data, err := f.cfb.Decrypt(string(payload))

	if err != nil {
		return ""
	}

	// bytes, _ := json.Marshal(data)
	// return string(bytes)
	return data
}

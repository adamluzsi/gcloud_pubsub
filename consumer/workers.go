package consumer

import (
	"log"
	"sync"
	"time"

	"github.com/adamluzsi/GoogleCloudPubsub/client"

	"cloud.google.com/go/pubsub"
	"google.golang.org/api/iterator"
)

func (c *consumer) subscriptionWorker() {
	defer c.wg.Done()

	var subscriptionWaitGroup sync.WaitGroup

initLoop:
	for {
		select {
		case <-c.ctx.Done():
			break initLoop

		default:

			pub, err := client.New()

			if err != nil {
				log.Println("pullWorker: pubsub client creation failed!")
				time.Sleep(5 * time.Second)
				continue initLoop
			}

			sub := pub.Subscription(c.subscriptionName)

			for index := 0; index < c.workersCount; index++ {
				subscriptionWaitGroup.Add(1)
				go c.subscribe(sub, &subscriptionWaitGroup)
			}

			subscriptionWaitGroup.Wait()

		}
	}

	subscriptionWaitGroup.Wait()
}

func (c *consumer) subscribe(sub *pubsub.Subscription, w *sync.WaitGroup) {
	defer w.Done()

messageProcessingLoop:
	for {
		select {
		case <-c.ctx.Done():
			break messageProcessingLoop

		default:
			err := c.processMessages(sub)

			if err == iterator.Done {
				break messageProcessingLoop
			}

			if err != nil {
				log.Println(err)
			}

		}
	}
}

func (c *consumer) processMessages(sub *pubsub.Subscription) error {
	handler := c.handlerConstructor()

	it, err := sub.Pull(c.ctx, pubsub.MaxExtension(c.maxExtension), pubsub.MaxPrefetch(c.batchSize))

	if err != nil {
		return err
	}

	defer it.Stop()

	messages := make([]*pubsub.Message, 0, c.batchSize)

	for index := 0; index < c.batchSize; index++ {

		msg, err := it.Next()

		if err == iterator.Done {
			break
		}

		if err != nil {
			return err
		}

		messages = append(messages, msg)
		err = handler.HandleMessage(newMessageWrapper(msg))

		if err != nil {
			msg.Done(false)
		}

	}

	err = handler.Finish()

	if err != nil {
		for _, m := range messages {
			m.Done(false)
		}
	}

	return nil
}

// Package bus is the Sūtra bus client: one durable topic exchange (nexus.events),
// a durable queue per service, manual acks, prefetch 32, a dead-letter exchange,
// publisher confirms and reconnect with backoff. Publishing while disconnected
// buffers in memory, so a service never blocks on the broker.
package bus

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"nexus/shared/envelope"
)

const (
	Exchange    = "nexus.events"
	DeadLetter  = "nexus.dlx"
	deadQueue   = "nexus.dead"
	outboxLimit = 4096
)

type Handler func(ctx context.Context, env envelope.Envelope) error

type sub struct {
	queue      string
	keys       []string
	handler    Handler
	includeOwn bool
}

type Bus struct {
	url     string
	service string
	log     *slog.Logger

	mu    sync.Mutex
	subs  []sub
	out   chan envelope.Envelope
	ready atomic.Bool
	wake  chan struct{}
}

// Connect starts the connection manager in the background and returns at once.
func Connect(ctx context.Context, url, service string, log *slog.Logger) *Bus {
	b := &Bus{url: url, service: service, log: log.With("component", "bus"), out: make(chan envelope.Envelope, outboxLimit), wake: make(chan struct{}, 1)}
	go b.run(ctx)
	return b
}

// IncludeOwn makes a subscription deliver envelopes this service published itself.
type Option func(*sub)

func IncludeOwn() Option { return func(s *sub) { s.includeOwn = true } }

// Subscribe registers a queue bound to the routing keys. Call before or after Connect;
// consumers (re)start on every successful connection.
func (b *Bus) Subscribe(queue string, keys []string, h Handler, opts ...Option) {
	s := sub{queue: queue, keys: keys, handler: h}
	for _, o := range opts {
		o(&s)
	}
	b.mu.Lock()
	b.subs = append(b.subs, s)
	b.mu.Unlock()
	select {
	case b.wake <- struct{}{}:
	default:
	}
}

func (b *Bus) Ready() bool { return b.ready.Load() }

// Check is a readiness probe: the broker connection must be up.
func (b *Bus) Check(context.Context) error {
	if !b.Ready() {
		return errors.New("rabbitmq not connected")
	}
	return nil
}

// Publish queues the envelope for the exchange. It never blocks; when the buffer is full the oldest is dropped.
func (b *Bus) Publish(_ context.Context, env envelope.Envelope) error {
	if env.ID == "" || env.Timestamp == "" {
		env.Fill(b.service)
	}
	if env.Origin == "" {
		env.Origin = b.service
	}
	for {
		select {
		case b.out <- env:
			return nil
		default:
			select {
			case <-b.out:
				b.log.Warn("outbox full, dropped oldest envelope")
			default:
			}
		}
	}
}

func (b *Bus) run(ctx context.Context) {
	backoff := time.Second
	for ctx.Err() == nil {
		err := b.session(ctx)
		b.ready.Store(false)
		if ctx.Err() != nil {
			return
		}
		b.log.Warn("bus session ended", "err", err, "retry_in", backoff.String())
		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return
		}
		if backoff < 15*time.Second {
			backoff *= 2
		}
	}
}

func (b *Bus) session(ctx context.Context) error {
	conn, err := amqp.DialConfig(b.url, amqp.Config{Heartbeat: 10 * time.Second, Dial: amqp.DefaultDial(5 * time.Second)})
	if err != nil {
		return err
	}
	defer conn.Close()
	closed := conn.NotifyClose(make(chan *amqp.Error, 1))

	pubCh, err := conn.Channel()
	if err != nil {
		return err
	}
	if err := declareTopology(pubCh); err != nil {
		return err
	}
	if err := pubCh.Confirm(false); err != nil {
		return err
	}

	b.mu.Lock()
	subs := append([]sub(nil), b.subs...)
	b.mu.Unlock()
	started := map[string]bool{}
	startSubs := func(list []sub) error {
		for _, s := range list {
			if started[s.queue] {
				continue
			}
			if err := b.consume(ctx, conn, s); err != nil {
				return err
			}
			started[s.queue] = true
		}
		return nil
	}
	if err := startSubs(subs); err != nil {
		return err
	}

	b.ready.Store(true)
	b.log.Info("bus connected")
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case e := <-closed:
			if e == nil {
				return errors.New("connection closed")
			}
			return e
		case <-b.wake: // a late Subscribe call
			b.mu.Lock()
			late := append([]sub(nil), b.subs...)
			b.mu.Unlock()
			if err := startSubs(late); err != nil {
				return err
			}
		case env := <-b.out:
			if err := b.publish(ctx, pubCh, env); err != nil {
				// put it back so the next session retries it
				select {
				case b.out <- env:
				default:
				}
				return err
			}
		}
	}
}

func declareTopology(ch *amqp.Channel) error {
	if err := ch.ExchangeDeclare(Exchange, "topic", true, false, false, false, nil); err != nil {
		return err
	}
	if err := ch.ExchangeDeclare(DeadLetter, "fanout", true, false, false, false, nil); err != nil {
		return err
	}
	if _, err := ch.QueueDeclare(deadQueue, true, false, false, false, nil); err != nil {
		return err
	}
	return ch.QueueBind(deadQueue, "", DeadLetter, false, nil)
}

func (b *Bus) publish(ctx context.Context, ch *amqp.Channel, env envelope.Envelope) error {
	body, err := json.Marshal(env)
	if err != nil {
		b.log.Error("drop unmarshalable envelope", "err", err)
		return nil
	}
	pctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	dc, err := ch.PublishWithDeferredConfirmWithContext(pctx, Exchange, env.Key(), false, false, amqp.Publishing{
		ContentType: "application/json", DeliveryMode: amqp.Persistent, MessageId: env.ID, Timestamp: time.Now(), Body: body,
	})
	if err != nil {
		return err
	}
	ok, err := dc.WaitContext(pctx)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("publish not confirmed")
	}
	return nil
}

func (b *Bus) consume(ctx context.Context, conn *amqp.Connection, s sub) error {
	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	if err := declareTopology(ch); err != nil {
		return err
	}
	if _, err := ch.QueueDeclare(s.queue, true, false, false, false, amqp.Table{"x-dead-letter-exchange": DeadLetter}); err != nil {
		return err
	}
	for _, k := range s.keys {
		if err := ch.QueueBind(s.queue, k, Exchange, false, nil); err != nil {
			return err
		}
	}
	if err := ch.Qos(32, 0, false); err != nil {
		return err
	}
	msgs, err := ch.Consume(s.queue, b.service+"."+s.queue, false, false, false, false, nil)
	if err != nil {
		return err
	}
	go func() {
		for d := range msgs {
			var env envelope.Envelope
			if err := json.Unmarshal(d.Body, &env); err != nil {
				b.log.Warn("bad envelope, dead-lettering", "err", err)
				_ = d.Nack(false, false)
				continue
			}
			if !s.includeOwn && env.Origin == b.service {
				_ = d.Ack(false)
				continue
			}
			if err := b.safeHandle(ctx, s.handler, env); err != nil {
				b.log.Error("handler failed, dead-lettering", "key", env.Key(), "err", err)
				_ = d.Nack(false, false)
				continue
			}
			_ = d.Ack(false)
		}
	}()
	return nil
}

func (b *Bus) safeHandle(ctx context.Context, h Handler, env envelope.Envelope) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.New("handler panic")
			b.log.Error("handler panic", "key", env.Key(), "recover", r)
		}
	}()
	hctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return h(hctx, env)
}

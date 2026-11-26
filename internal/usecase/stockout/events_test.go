package stockout

import (
	"context"
	"testing"
	"time"

	domainAuth "chawy-erp-api/internal/domain/auth"
	domainStockOut "chawy-erp-api/internal/domain/stockout"

	"github.com/stretchr/testify/require"
)

func TestBrokerFansOutAndUnsubscribes(t *testing.T) {
	b := NewBroker()
	a, stopA := b.Subscribe()
	c, stopC := b.Subscribe()
	require.Equal(t, 2, b.Subscribers())
	b.Publish(Event{Type: "batch", BatchID: 1})
	require.Equal(t, uint(1), (<-a).BatchID)
	require.Equal(t, uint(1), (<-c).BatchID)
	stopA()
	stopA() // twice is safe
	require.Equal(t, 1, b.Subscribers())
	for i := 0; i < 50; i++ { // a slow stream never blocks the publisher
		b.Publish(Event{Type: "batch"})
	}
	stopC()
	require.Equal(t, 0, b.Subscribers())
}

type recordNotifier struct{ got []domainStockOut.Batch }

func (r *recordNotifier) NotifyReview(b domainStockOut.Batch) { r.got = append(r.got, b) }

func TestChangesArePublishedAndShortCountsNotifyTheOwner(t *testing.T) {
	e := newEnv()
	broker := NewBroker()
	note := &recordNotifier{}
	e.uc.broker, e.uc.notifier = broker, note
	events, stop := broker.Subscribe()
	defer stop()

	res := e.count(t, "ok", packer, CountLine{skuTuna, 1})
	ev := <-events
	require.Equal(t, "batch", ev.Type)
	require.Equal(t, res.Batch.ID, ev.BatchID)
	require.Equal(t, "2026-11-26", ev.Date)
	require.Empty(t, note.got)

	e.count(t, "ok", packer, CountLine{skuTuna, 1}) // replay: no event
	select {
	case got := <-events:
		t.Fatalf("unexpected event %+v", got)
	default:
	}

	e.stock.rows[skuChicken].Quantity, e.stock.rows[skuChicken].AvailableQty = 0, 0
	short := e.count(t, "short", packer, CountLine{skuChicken, 2})
	<-events
	require.Len(t, note.got, 1)
	require.Equal(t, short.Batch.BatchNo, note.got[0].BatchNo)

	_, err := e.uc.VoidBatch(context.Background(), res.Batch.ID, owner)
	require.NoError(t, err)
	require.Equal(t, "void", (<-events).Type)
}

func TestOwnVoidWindowMatchesTheUndoToast(t *testing.T) {
	e := newEnv()
	res := e.count(t, "c1", packer, CountLine{skuTuna, 1})
	e.clock = e.clock.Add(11 * time.Second)
	_, err := e.uc.VoidBatch(context.Background(), res.Batch.ID, packer)
	require.Error(t, err)
}

type fakeUsers struct{ users []*domainAuth.User }

func (f fakeUsers) FindAll(context.Context) ([]*domainAuth.User, error) { return f.users, nil }

type fakeSender struct{ to []string }

func (f *fakeSender) SendNotification(to, subject, body string) error {
	f.to = append(f.to, to)
	return nil
}

func TestEmailNotifierWritesToActiveOwnersOnly(t *testing.T) {
	sender := &fakeSender{}
	n := NewEmailNotifier(fakeUsers{users: []*domainAuth.User{
		{ID: 1, Email: "owner@chawy", Role: "owner", IsActive: true},
		{ID: 2, Email: "old-owner@chawy", Role: "owner", IsActive: false},
		{ID: 3, Email: "pack@chawy", Role: "warehouse", IsActive: true},
	}}, sender)
	n.async = func(f func()) { f() }
	n.NotifyReview(domainStockOut.Batch{BatchNo: "SO-000009", UserName: "pack@chawy",
		Lines: []domainStockOut.Line{{Name: "ไก่ <script>", Qty: 3, Deducted: 1, Short: 2}}})
	require.Equal(t, []string{"owner@chawy"}, sender.to)

	subject, body := reviewMessage(domainStockOut.Batch{BatchNo: "SO-1", Lines: []domainStockOut.Line{{Name: "<b>x</b>", Short: 1}}})
	require.Contains(t, subject, "SO-1")
	require.Contains(t, body, "&lt;b&gt;x&lt;/b&gt;")
}

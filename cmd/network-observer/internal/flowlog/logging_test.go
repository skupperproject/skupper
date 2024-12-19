package flowlog

import (
	"context"
	"log/slog"
	"testing"

	"github.com/skupperproject/skupper/cmd/network-observer/internal/collector"
	"github.com/skupperproject/skupper/pkg/vanflow"
	"gotest.tools/v3/assert"
)

func TestSampling(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("lowest priority allow wins", func(t *testing.T) {
		var ct int
		messageCounter := func(string, ...any) {
			ct++
		}
		handler := New(ctx, messageCounter, []Rule{ // Lowest priority wins
			{
				Priority: 2,
				Match:    NewRecordTypeSet(vanflow.SiteRecord{}),
				Strategy: doNotSample,
			},
			{
				Priority: 1,
				Match:    NewRecordTypeSet(vanflow.SiteRecord{}),
				Strategy: Unlimited(),
			},
			{
				Priority: 5,
				Match:    NewRecordTypeSet(vanflow.SiteRecord{}),
				Strategy: doNotSample,
			},
		})

		for x := 0; x < 1_000; x++ {
			handler(vanflow.RecordMessage{Records: []vanflow.Record{
				vanflow.SiteRecord{},
				vanflow.SiteRecord{},
				vanflow.ProcessRecord{},
			}})
		}
		assert.Equal(t, ct, 2_000)
	})

	t.Run("rate limt zero with burst", func(t *testing.T) {
		ct := 0
		messageCounter := func(string, ...any) {
			ct++
		}
		handler := New(ctx, messageCounter, []Rule{ // zero rate with bursts
			{
				Priority: 1,
				Match:    NewRecordTypeSet(vanflow.SiteRecord{}),
				Strategy: RateLimited(0.0, 16),
			},
		})

		for x := 0; x < 1_000; x++ {
			handler(vanflow.RecordMessage{Records: []vanflow.Record{
				vanflow.SiteRecord{},
				vanflow.SiteRecord{},
				vanflow.ProcessRecord{},
			}})
		}
		assert.Equal(t, ct, 16)
	})

	t.Run("hash based", func(t *testing.T) {
		var (
			MagicPercentage  = 0.1
			MagicMatchingIDs = []string{
				"test::7",
				"test::3e2",
				"test::317",
				"test::3ad",
			}
			MagicSkippedIDs = []string{
				"test::d",
				"test::3e3",
				"test::318",
				"test::3af",
			}
		)

		var (
			ctTport int
			ctApp   int
		)
		check := func(subject string, _ ...any) {
			switch subject {
			case vanflow.TransportBiflowRecord{}.GetTypeMeta().String():
				ctTport++
			case vanflow.AppBiflowRecord{}.GetTypeMeta().String():
				ctApp++
			default:
				t.Errorf("unexpected type: %s", subject)
			}
		}
		handler := New(ctx, check, []Rule{
			{
				Priority: 1,
				Match:    NewRecordTypeSetAll(),
				Strategy: TransportFlowHash(MagicPercentage, Unlimited()),
			},
		})

		for _, tportID := range MagicMatchingIDs {
			handler(vanflow.RecordMessage{Records: []vanflow.Record{
				vanflow.SiteRecord{BaseRecord: vanflow.NewBase(tportID)},
				vanflow.ProcessRecord{BaseRecord: vanflow.NewBase(tportID)},
				vanflow.TransportBiflowRecord{BaseRecord: vanflow.NewBase(tportID)},
				vanflow.AppBiflowRecord{BaseRecord: vanflow.NewBase(tportID), Parent: &tportID},
			}})
		}
		for _, tportID := range MagicSkippedIDs {
			handler(vanflow.RecordMessage{Records: []vanflow.Record{
				vanflow.SiteRecord{BaseRecord: vanflow.NewBase(tportID)},
				vanflow.ProcessRecord{BaseRecord: vanflow.NewBase(tportID)},
				vanflow.TransportBiflowRecord{BaseRecord: vanflow.NewBase(tportID)},
				vanflow.AppBiflowRecord{BaseRecord: vanflow.NewBase(tportID), Parent: &tportID},
			}})
		}
		assert.Equal(t, ctTport, ctApp)
		assert.Equal(t, ctTport, len(MagicMatchingIDs))

		ctTport, ctApp = 0, 0
		handler = New(ctx, check, []Rule{
			{
				Priority: 1,
				Match:    NewRecordTypeSetAll(),
				Strategy: TransportFlowHash(MagicPercentage, doNotSample),
			},
		})
		handler(vanflow.RecordMessage{Records: []vanflow.Record{
			vanflow.TransportBiflowRecord{BaseRecord: vanflow.NewBase(MagicMatchingIDs[0])},
			vanflow.AppBiflowRecord{BaseRecord: vanflow.NewBase(MagicMatchingIDs[1])},
		}})
		assert.Equal(t, ctTport, ctApp)
		assert.Equal(t, ctTport, 0)
	})

	t.Run("all types", func(t *testing.T) {
		ct := 0
		messageCounter := func(string, ...any) {
			ct++
		}
		handler := New(ctx, messageCounter, []Rule{ // zero rate with bursts
			{
				Priority: 1,
				Match:    NewRecordTypeSetAll(),
				Strategy: Unlimited(),
			},
		})

		for x := 0; x < 10; x++ {

			handler(vanflow.RecordMessage{Records: []vanflow.Record{
				vanflow.SiteRecord{},
				vanflow.SiteRecord{},
				vanflow.FlowRecord{},
				vanflow.AppBiflowRecord{},
				collector.AddressRecord{},
				vanflow.ProcessRecord{},
			}})
		}
		assert.Equal(t, ct, 60)
	})
	t.Run("log report empty", func(t *testing.T) {
		type message struct {
			Msg  string
			Args []any
		}
		messages := []message{}

		recordMessages := func(msg string, args ...any) {
			messages = append(messages, message{Msg: msg, Args: args})
		}
		handler := &handler{
			logFn: recordMessages,
			rules: []Rule{
				{
					Priority: 1,
					Match:    NewRecordTypeSet(vanflow.SiteRecord{}),
					Strategy: Unlimited(),
				},
			},
		}
		handler.handle(vanflow.RecordMessage{
			Records: []vanflow.Record{
				vanflow.SiteRecord{},
			},
		})
		handler.handle(vanflow.RecordMessage{})
		assert.Equal(t, len(messages), 1)
		messages = messages[:0]
		handler.logReport()
		assert.Equal(t, len(messages), 0)
	})
	t.Run("log report", func(t *testing.T) {
		type message struct {
			Msg  string
			Args []any
		}
		messages := []message{}

		recordMessages := func(msg string, args ...any) {
			messages = append(messages, message{Msg: msg, Args: args})
		}
		handler := &handler{
			logFn: recordMessages,
			rules: []Rule{
				{
					Priority: 1,
					Match:    NewRecordTypeSet(vanflow.SiteRecord{}),
					Strategy: RateLimited(0.0, 16),
				},
			},
		}
		for x := 0; x < 1_000; x++ {
			handler.handle(vanflow.RecordMessage{Records: []vanflow.Record{
				vanflow.SiteRecord{},
				vanflow.SiteRecord{},
				vanflow.ProcessRecord{},
			}})
		}
		assert.Equal(t, len(messages), 16)
		messages = messages[:0]
		handler.logReport()
		assert.Equal(t, len(messages), 1)
		assert.DeepEqual(t, messages[0].Args, []any{
			slog.Int("flow/v1/SiteRecord", 2_000-16),
		})
	})

}

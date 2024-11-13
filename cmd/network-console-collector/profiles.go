package main

import (
	"github.com/skupperproject/skupper/cmd/network-console-collector/internal/flowlog"
	"github.com/skupperproject/skupper/pkg/vanflow"
)

var (
	// loggingProfileMinimal logs 1 vanflow event per second (with bursts up to 32)
	// reduces Link Record noise to 1 every ~20s.
	// excludes network flow records
	loggingProfileMinimal = []flowlog.Rule{
		{
			Priority: 5,
			Match: flowlog.NewRecordTypeSet(
				vanflow.SiteRecord{}, vanflow.RouterRecord{},
				vanflow.ProcessRecord{}, vanflow.ConnectorRecord{},
				vanflow.LinkRecord{}, vanflow.ListenerRecord{},
				vanflow.RouterAccessRecord{}, vanflow.LogRecord{},
			),
			Strategy: flowlog.RateLimited(1.0, 32),
		}, {
			Priority: 1,
			Match:    flowlog.NewRecordTypeSet(vanflow.LinkRecord{}),
			Strategy: flowlog.RateLimited(0.05, 32),
		},
	}
	// loggingProfileModerate is similar to minimal but doubles rate and burst
	// limits. Also samples 1 in every 10 network flows up to 2 events per second.
	loggingProfileModerate = []flowlog.Rule{
		{
			Priority: 5,
			Match:    flowlog.NewRecordTypeSetAll(),
			Strategy: flowlog.RateLimited(2.0, 64),
		}, {
			Priority: 1,
			Match:    flowlog.NewRecordTypeSet(vanflow.LinkRecord{}),
			Strategy: flowlog.RateLimited(0.1, 64),
		}, {
			Priority: 1,
			Match: flowlog.NewRecordTypeSet(
				vanflow.AppBiflowRecord{},
				vanflow.TransportBiflowRecord{},
			),
			Strategy: flowlog.TransportFlowHash(0.1, flowlog.RateLimited(2.0, 64)),
		},
	}
	// loggingProfileAll logs all vanflow events.
	loggingProfileAll = []flowlog.Rule{
		{
			Match:    flowlog.NewRecordTypeSetAll(),
			Strategy: flowlog.Unlimited(),
		},
	}
)
